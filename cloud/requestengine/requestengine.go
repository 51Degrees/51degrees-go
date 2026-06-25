/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence
 * (EUPL) v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */

// Package requestengine implements the Cloud Request Engine, which makes a
// single HTTP call to a remote 51Degrees cloud service per Flow Data and exposes
// the raw JSON response for downstream Cloud Aspect Engines to deserialize.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md
package requestengine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/engines"
)

// DataKey is the Element Data key under which the Cloud Request Engine stores
// its result.
const DataKey = "cloud-response"

// JSONResponseProperty is the Property name holding the raw JSON response body.
const JSONResponseProperty = "json-response"

// Default endpoint configuration.
const (
	DefaultEndpoint      = "https://cloud.51degrees.com/api/v4/"
	DefaultTimeout       = 2 * time.Second
	DefaultFailures      = 10
	DefaultFailureWindow = 100 * time.Second
	DefaultRecovery      = 60 * time.Second
)

// CloudRequestData is the Aspect Data produced by the Cloud Request Engine.
type CloudRequestData struct {
	*engines.BaseAspectData
}

func newCloudRequestData() *CloudRequestData {
	return &CloudRequestData{BaseAspectData: engines.NewBaseAspectData()}
}

// JSONResponse returns the raw JSON response body.
func (d *CloudRequestData) JSONResponse() string {
	if v, err := d.Get(JSONResponseProperty); err == nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// PropertyMeta describes a single Property as reported by the accessible
// properties endpoint.
type PropertyMeta struct {
	// Name is the name of the Property.
	Name string `json:"Name"`
	// Type is the data type of the Property value (for example "String" or "Int32").
	Type string `json:"Type"`
	// Category is the category the Property belongs to.
	Category string `json:"Category"`
	// ItemProperties describes the sub-Properties of a list/collection Property,
	// where each item is itself a set of Properties.
	ItemProperties []PropertyMeta `json:"ItemProperties,omitempty"`
}

// ProductMeta describes the Properties available for a single product (Aspect).
type ProductMeta struct {
	// DataTier is the data tier (for example "Lite" or "Premium") that the
	// supplied Resource Key grants access to for this product.
	DataTier string `json:"DataTier"`
	// Properties are the Properties available for this product.
	Properties []PropertyMeta `json:"Properties"`
}

// accessiblePropertiesResponse is the shape of the accessible properties
// endpoint response.
type accessiblePropertiesResponse struct {
	Products map[string]ProductMeta `json:"Products"`
}

// CloudRequestEngine makes requests to the remote cloud service.
type CloudRequestEngine struct {
	*engines.BaseAspectEngine

	httpClient  *http.Client
	resourceKey string
	origin      string

	dataEndpoint         string
	propertiesEndpoint   string
	evidenceKeysEndpoint string

	// Lazily initialized, failable metadata (see the updated start-up design).
	metaMu           sync.Mutex
	metaLoaded       bool
	publicProperties map[string]ProductMeta
	evidenceKeys     []string

	// Recovery mode state.
	failuresToEnterRecovery int
	failureWindow           time.Duration
	recoveryDuration        time.Duration
	recMu                   sync.Mutex
	failureTimes            []time.Time
	recoveryUntil           time.Time

	now func() time.Time // injectable clock for testing
}

// PublicProperties returns the accessible properties metadata, initializing it
// lazily on first use. It MUST be called within a processing context so that
// failures are subject to SuppressProcessExceptions.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#updated-design
func (e *CloudRequestEngine) PublicProperties() (map[string]ProductMeta, error) {
	if err := e.ensureMetadata(); err != nil {
		return nil, err
	}
	return e.publicProperties, nil
}

// Process makes the cloud request and stores the raw JSON response.
func (e *CloudRequestEngine) Process(fd core.FlowData) error {
	// While recovering, bypass all outbound calls (including the lazy metadata
	// fetch) and signal temporary unavailability, so a struggling server is not
	// hammered and consumer requests do not stall.
	if until, inRecovery := e.inRecovery(); inRecovery {
		return &TemporarilyUnavailableError{RecoveryUntil: until}
	}

	// Lazily initialize metadata within Process scope so that failures honor
	// SuppressProcessExceptions rather than crashing at construction time. A
	// metadata-fetch failure counts toward recovery, just like a data request.
	if err := e.ensureMetadata(); err != nil {
		e.recordFailure()
		return err
	}

	body, err := e.makeDataRequest(fd)
	if err != nil {
		e.recordFailure()
		return err
	}

	data := newCloudRequestData()
	data.Set(JSONResponseProperty, body)
	fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData { return data })
	return nil
}

func (e *CloudRequestEngine) makeDataRequest(fd core.FlowData) (string, error) {
	form := e.buildForm(fd)
	req, err := http.NewRequest(http.MethodPost, e.dataEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	e.setCommonHeaders(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", NewCloudRequestError(fmt.Sprintf("error contacting cloud service at '%s'", e.dataEndpoint), err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if err := e.checkResponse(e.dataEndpoint, resp.StatusCode, raw); err != nil {
		return "", err
	}
	e.logWarnings(raw)
	return string(raw), nil
}

// buildForm constructs the URL-encoded form: the resource key plus all Evidence
// values with their prefixes removed. Conflicts between prefixes are resolved
// using the defined Evidence precedence.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#processing
func (e *CloudRequestEngine) buildForm(fd core.FlowData) url.Values {
	form := url.Values{}
	if e.resourceKey != "" {
		form.Set("resource", e.resourceKey)
	}
	for field, value := range e.resolveEvidence(fd) {
		form.Set(field, value)
	}
	return form
}

// resolveEvidence removes prefixes from all Evidence and resolves conflicts
// between prefixes using the defined precedence. Like the .NET reference, every
// Evidence value is forwarded (prefix-stripped) and the server ignores any it
// does not need; the evidence-keys metadata is used to advertise accepted keys,
// not to filter the request body.
func (e *CloudRequestEngine) resolveEvidence(fd core.FlowData) map[string]string {
	type choice struct {
		prefix string
		value  string
	}
	best := map[string]choice{}
	for key, raw := range fd.Evidence().AsMap() {
		prefix, field := core.SplitEvidenceKey(key)
		val := valueToString(raw)
		if cur, ok := best[field]; ok {
			if core.ComparePrefixes(prefix, cur.prefix) < 0 {
				best[field] = choice{prefix, val}
			}
			continue
		}
		best[field] = choice{prefix, val}
	}
	out := make(map[string]string, len(best))
	for field, c := range best {
		out[field] = c.value
	}
	return out
}

func (e *CloudRequestEngine) setCommonHeaders(req *http.Request) {
	if e.origin != "" {
		req.Header.Set("Origin", e.origin)
	}
}

// checkResponse implements the error scenarios required of cloud HTTP requests.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#http-requests
func (e *CloudRequestEngine) checkResponse(url string, status int, raw []byte) error {
	if msgs := extractMessages(raw, "errors"); len(msgs) > 0 {
		return NewCloudRequestError(strings.Join(msgs, "; "), nil)
	}
	if len(raw) == 0 {
		return NewCloudRequestError(fmt.Sprintf("No data in response from cloud service at '%s'", url), nil)
	}
	if status != http.StatusOK {
		return NewCloudRequestError(
			fmt.Sprintf("Cloud service at '%s' returned status code '%d' with content %s",
				url, status, truncate(string(raw), 512)), nil)
	}
	return nil
}

func (e *CloudRequestEngine) logWarnings(raw []byte) {
	for _, w := range extractMessages(raw, "warnings") {
		e.Logger().Warn("cloud service warning", "warning", w)
	}
}

// ensureMetadata fetches the accessible-properties and evidence-keys data once,
// caching it on success. On failure it leaves the engine uninitialized so a
// later call can retry, as required by the spec.
//
// The lock is NOT held across the HTTP calls: concurrent first-use callers may
// each attempt a fetch (publication-only, as in the .NET Lazy<T> design) rather
// than serializing behind one mutex while the network is slow — which is the
// stall the recovery-mode design exists to avoid.
func (e *CloudRequestEngine) ensureMetadata() error {
	e.metaMu.Lock()
	loaded := e.metaLoaded
	e.metaMu.Unlock()
	if loaded {
		return nil
	}

	props, err := e.fetchAccessibleProperties()
	if err != nil {
		return err
	}
	keys, err := e.fetchEvidenceKeys()
	if err != nil {
		return err
	}

	e.metaMu.Lock()
	if !e.metaLoaded {
		e.publicProperties = props
		e.evidenceKeys = keys
		e.metaLoaded = true
	}
	e.metaMu.Unlock()
	return nil
}

func (e *CloudRequestEngine) fetchAccessibleProperties() (map[string]ProductMeta, error) {
	raw, err := e.getEndpoint(e.propertiesEndpoint)
	if err != nil {
		return nil, err
	}
	var parsed accessiblePropertiesResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, NewCloudRequestError("could not parse accessible properties response", err)
	}
	return parsed.Products, nil
}

func (e *CloudRequestEngine) fetchEvidenceKeys() ([]string, error) {
	raw, err := e.getEndpoint(e.evidenceKeysEndpoint)
	if err != nil {
		return nil, err
	}
	var keys []string
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, NewCloudRequestError("could not parse evidence keys response", err)
	}
	return keys, nil
}

func (e *CloudRequestEngine) getEndpoint(endpoint string) ([]byte, error) {
	u := endpoint
	if e.resourceKey != "" {
		if strings.Contains(u, "?") {
			u += "&resource=" + url.QueryEscape(e.resourceKey)
		} else {
			u += "?resource=" + url.QueryEscape(e.resourceKey)
		}
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	e.setCommonHeaders(req)
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, NewCloudRequestError(fmt.Sprintf("error contacting cloud service at '%s'", endpoint), err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if err := e.checkResponse(endpoint, resp.StatusCode, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// --- Recovery mode -------------------------------------------------------

func (e *CloudRequestEngine) inRecovery() (time.Time, bool) {
	if e.recoveryDuration <= 0 {
		return time.Time{}, false
	}
	e.recMu.Lock()
	defer e.recMu.Unlock()
	if e.now().Before(e.recoveryUntil) {
		return e.recoveryUntil, true
	}
	return time.Time{}, false
}

func (e *CloudRequestEngine) recordFailure() {
	if e.recoveryDuration <= 0 {
		return
	}
	now := e.now()
	e.recMu.Lock()
	defer e.recMu.Unlock()
	cutoff := now.Add(-e.failureWindow)
	kept := e.failureTimes[:0]
	for _, t := range e.failureTimes {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	e.failureTimes = kept
	if len(e.failureTimes) >= e.failuresToEnterRecovery {
		e.recoveryUntil = now.Add(e.recoveryDuration)
		e.failureTimes = e.failureTimes[:0]
	}
}

// --- helpers -------------------------------------------------------------

func valueToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case []string:
		return strings.Join(t, ",")
	default:
		return fmt.Sprintf("%v", t)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// extractMessages pulls a top-level array of strings (e.g. "errors" or
// "warnings") from a JSON response, tolerating malformed JSON.
func extractMessages(raw []byte, field string) []string {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	arr, ok := doc[field]
	if !ok {
		return nil
	}
	var msgs []string
	if err := json.Unmarshal(arr, &msgs); err != nil {
		return nil
	}
	return msgs
}
