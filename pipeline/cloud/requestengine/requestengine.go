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
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
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
	// DefaultRecoveryMultiplier of 1.0 keeps the recovery duration fixed. Set a
	// value greater than 1 to grow it exponentially after consecutive trips.
	DefaultRecoveryMultiplier = 1.0
	// DefaultMaxRecovery of 0 leaves the exponentially grown recovery duration
	// uncapped.
	DefaultMaxRecovery = 0 * time.Second
)

// CloudRequestData is the Aspect Data produced by the Cloud Request Engine.
type CloudRequestData struct {
	*engines.BaseAspectData
	failed bool
}

func newCloudRequestData() *CloudRequestData {
	return &CloudRequestData{BaseAspectData: engines.NewBaseAspectData()}
}

// Failed reports whether the cloud request ran but failed. It lets a downstream
// Cloud Aspect Engine distinguish a failed request from one that never ran (so
// a missing Property can be reported as a cloud-request failure rather than the
// engine not being present).
func (d *CloudRequestData) Failed() bool { return d.failed }

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
	// DelayExecution is true for a JavaScript Property whose code should
	// run only when the page asks for it.
	DelayExecution bool `json:"DelayExecution,omitempty"`
	// EvidenceProperties are the Properties whose JavaScript, when run, gives
	// Evidence for this Property's value.
	EvidenceProperties []string `json:"EvidenceProperties,omitempty"`
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

// DiscoveryState holds what the engine discovers from the cloud service when
// it is built, being the accepted Evidence keys and the accessible
// Properties. Both depend only on the Resource Key, so a host that starts
// often can keep the state from one build, for example as JSON, and give it
// to the next build with Builder.SetDiscoveryState so that build makes no
// request.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#resilience
type DiscoveryState struct {
	// EvidenceKeys are the Evidence keys the cloud service accepts.
	EvidenceKeys []string `json:"EvidenceKeys"`
	// Products are the accessible Properties, keyed by product (Aspect).
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

	// The discovery results, nil until known. Each is set once, when the
	// engine is built or, after a transient failure then, by the first use
	// that fetches it.
	products     atomic.Pointer[map[string]ProductMeta]
	evidenceKeys atomic.Pointer[core.WhitelistFilter]

	// Recovery mode state.
	failuresToEnterRecovery int
	failureWindow           time.Duration
	recoveryDuration        time.Duration
	recoveryMultiplier      float64
	maxRecoveryDuration     time.Duration
	recMu                   sync.Mutex
	failureTimes            []time.Time
	recoveryUntil           time.Time
	// consecutiveRecoveries counts how many times recovery has been tripped in a
	// row without an intervening success, driving the exponential backoff. It is
	// reset by recordSuccess.
	consecutiveRecoveries int

	now func() time.Time // injectable clock for testing
}

// KnownPublicProperties returns the accessible Properties, keyed by product,
// and true once they are known, making no request. It returns false after a
// transient failure at build time until a use has fetched them. It returns a
// copy, so a caller may change it without changing the engine.
func (e *CloudRequestEngine) KnownPublicProperties() (map[string]ProductMeta, bool) {
	if p := e.products.Load(); p != nil {
		return cloneProducts(*p), true
	}
	return nil, false
}

// PublicProperties returns the accessible Properties, keyed by product, which
// are normally fetched when the engine is built. After a transient failure
// then, a call fetches them unless the engine is in its recovery period, so
// call it while processing, where SuppressProcessExceptions covers a failure.
// It returns a copy, so a caller may change it without changing the engine.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#revised-design-eager-discovery-at-build-time-transient-failure-fallback
func (e *CloudRequestEngine) PublicProperties() (map[string]ProductMeta, error) {
	if p := e.products.Load(); p != nil {
		return cloneProducts(*p), nil
	}
	if until, inRecovery := e.inRecovery(); inRecovery {
		return nil, &TemporarilyUnavailableError{RecoveryUntil: until}
	}
	if err := e.loadProducts(); err != nil {
		return nil, err
	}
	return cloneProducts(*e.products.Load()), nil
}

// Process makes the cloud request and stores the raw JSON response.
func (e *CloudRequestEngine) Process(fd core.FlowData) error {
	// While recovering, bypass all outbound calls, discovery included, and
	// signal temporary unavailability, so a struggling server is not hammered
	// and consumer requests do not stall.
	if until, inRecovery := e.inRecovery(); inRecovery {
		return &TemporarilyUnavailableError{RecoveryUntil: until}
	}

	// Discovery normally completed when the engine was built. After a
	// transient failure then, it is retried here, so a failure is a processing
	// error that SuppressProcessExceptions covers.
	if err := e.retryDiscovery(); err != nil {
		e.storeFailed(fd)
		return err
	}

	body, err := e.makeDataRequest(fd)
	if err != nil {
		e.recordFailure(err)
		e.storeFailed(fd)
		return err
	}

	e.recordSuccess()
	data := newCloudRequestData()
	data.Set(JSONResponseProperty, body)
	fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData { return data })
	return nil
}

// storeFailed records that the request ran but failed, so a downstream Cloud
// Aspect Engine can tell this apart from the request engine never running (for
// example under suppressed exceptions, where Process's error is only logged).
func (e *CloudRequestEngine) storeFailed(fd core.FlowData) {
	data := newCloudRequestData()
	data.failed = true
	fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData { return data })
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
		return "", NewCloudRequestError(fmt.Sprintf("error contacting cloud service at '%s'", e.dataEndpoint), redactResourceKey(err))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if err := e.checkResponse(e.dataEndpoint, resp.StatusCode, resp.Header, raw); err != nil {
		return "", err
	}
	if !json.Valid(raw) {
		return "", NewCloudResponseError(
			fmt.Sprintf("failed to parse the response from cloud service at '%s' as JSON",
				e.dataEndpoint), resp.StatusCode, resp.Header)
	}
	e.logWarnings(raw)
	return string(raw), nil
}

// engineFields are the form fields that carry what the engine writes itself,
// licence being the service's other name for license. Evidence that strips to
// one of them is left out, whichever keys the cloud service lists, because a
// second field for the same value would leave which one applied undecided.
var engineFields = map[string]bool{
	"resource": true, "license": true, "licence": true, "values": true}

// buildForm constructs the URL-encoded form, being the resource key and the
// Evidence the cloud service needs, as resolveEvidence gives it.
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

// resolveEvidence returns the Evidence whose keys the cloud service accepts,
// with each prefix removed, leaving out any that strips to a field the engine
// writes itself. Conflicts between prefixes are resolved using the defined
// precedence. It returns nothing while the accepted keys are not known, which
// Process does not allow, as it fetches them first.
func (e *CloudRequestEngine) resolveEvidence(fd core.FlowData) map[string]string {
	type choice struct {
		prefix string
		value  string
	}
	accepted := e.evidenceKeys.Load()
	if accepted == nil {
		return nil
	}
	best := map[string]choice{}
	for key, raw := range fd.Evidence().AsMap() {
		if !accepted.Include(key) {
			continue
		}
		prefix, field := core.SplitEvidenceKey(key)
		if engineFields[strings.ToLower(field)] {
			continue
		}
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
func (e *CloudRequestEngine) checkResponse(url string, status int, headers http.Header, raw []byte) error {
	if msgs := extractMessages(raw, "errors"); len(msgs) > 0 {
		return NewCloudResponseError(strings.Join(msgs, "; "), status, headers)
	}
	if len(raw) == 0 {
		return NewCloudResponseError(fmt.Sprintf("No data in response from cloud service at '%s'", url), status, headers)
	}
	if status != http.StatusOK {
		return NewCloudResponseError(
			fmt.Sprintf("Cloud service at '%s' returned status code '%d' with content %s",
				url, status, truncate(string(raw), 512)), status, headers)
	}
	return nil
}

func (e *CloudRequestEngine) logWarnings(raw []byte) {
	for _, w := range extractMessages(raw, "warnings") {
		e.Logger().Warn("cloud service warning", "warning", w)
	}
}

// --- Discovery -----------------------------------------------------------
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#start-up-activity

// discoverAtBuild makes both discovery requests in parallel, so the build
// waits for one timeout at most. A 4xx answer to either is a configuration
// error that no retry can fix, so it fails the build. Any other failure, being
// the service unreachable, a timeout, a 5xx answer or a body that cannot be
// read, is logged as a warning, and the first use retries what is missing.
func (e *CloudRequestEngine) discoverAtBuild() error {
	errs := make([]error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); errs[0] = e.loadProducts() }()
	go func() { defer wg.Done(); errs[1] = e.loadEvidenceKeys() }()
	wg.Wait()
	for _, err := range errs {
		if isDefinitive(err) {
			return core.NewConfigurationError("the cloud service refused a "+
				"discovery request, which no retry with this configuration "+
				"can fix", err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		e.Logger().Warn("the cloud request engine could not complete discovery "+
			"when it was built because of a transient failure, and will retry "+
			"when it is first used", "error", err)
	}
	return nil
}

// isDefinitive reports whether err is a 4xx answer from the cloud service.
func isDefinitive(err error) bool {
	var cre *CloudRequestError
	return errors.As(err, &cre) && cre.StatusCode >= 400 && cre.StatusCode < 500
}

// retryDiscovery fetches whichever discovery result the build could not, and
// makes no request once both are known. The evidence keys are fetched first
// and a failure stops there, so while the service is down each use costs one
// failed request. No lock is held across the requests, so a slow service
// cannot queue uses behind one another, and the first result kept wins.
func (e *CloudRequestEngine) retryDiscovery() error {
	if e.evidenceKeys.Load() == nil {
		if err := e.loadEvidenceKeys(); err != nil {
			return err
		}
	}
	if e.products.Load() == nil {
		if err := e.loadProducts(); err != nil {
			return err
		}
	}
	return nil
}

// loadProducts fetches the accessible Properties and keeps them. A failure
// that says the cloud service is in trouble counts toward the recovery
// period, as it does for any request.
func (e *CloudRequestEngine) loadProducts() error {
	products, err := e.fetchAccessibleProperties()
	if err != nil {
		e.recordFailure(err)
		return err
	}
	e.recordSuccess()
	e.keepProducts(products)
	return nil
}

// loadEvidenceKeys fetches the accepted Evidence keys and keeps them. A
// failure that says the cloud service is in trouble counts toward the
// recovery period, as it does for any request.
func (e *CloudRequestEngine) loadEvidenceKeys() error {
	keys, err := e.fetchEvidenceKeys()
	if err != nil {
		e.recordFailure(err)
		return err
	}
	e.recordSuccess()
	e.evidenceKeys.CompareAndSwap(nil, core.NewWhitelistFilter(keys...))
	return nil
}

// keepProducts stores products unless they are already known. A product list
// the service leaves out is kept as empty, because it is known to be empty.
func (e *CloudRequestEngine) keepProducts(products map[string]ProductMeta) {
	if products == nil {
		products = map[string]ProductMeta{}
	}
	e.products.CompareAndSwap(nil, &products)
}

// useState keeps a copy of a discovery state given to the builder, in place
// of making the discovery requests.
func (e *CloudRequestEngine) useState(s *DiscoveryState) {
	e.keepProducts(cloneProducts(s.Products))
	e.evidenceKeys.CompareAndSwap(nil, core.NewWhitelistFilter(s.EvidenceKeys...))
}

// discoveryState returns a copy of both discovery results, or nil while
// either is not known.
func (e *CloudRequestEngine) discoveryState() *DiscoveryState {
	products, keys := e.products.Load(), e.evidenceKeys.Load()
	if products == nil || keys == nil {
		return nil
	}
	return &DiscoveryState{
		EvidenceKeys: keys.Keys(),
		Products:     cloneProducts(*products),
	}
}

// cloneProducts returns a copy of products that shares no map or slice with
// it, item Properties included. Nil stays nil and empty stays empty.
func cloneProducts(products map[string]ProductMeta) map[string]ProductMeta {
	if products == nil {
		return nil
	}
	out := make(map[string]ProductMeta, len(products))
	for name, product := range products {
		product.Properties = cloneProperties(product.Properties)
		out[name] = product
	}
	return out
}

// cloneProperties returns a copy of properties that shares no slice with it.
func cloneProperties(properties []PropertyMeta) []PropertyMeta {
	if properties == nil {
		return nil
	}
	out := make([]PropertyMeta, len(properties))
	for i, property := range properties {
		property.ItemProperties = cloneProperties(property.ItemProperties)
		property.EvidenceProperties = slices.Clone(property.EvidenceProperties)
		out[i] = property
	}
	return out
}

// acceptedEvidence is the engine's Evidence key filter, holding the keys from
// the evidencekeys endpoint. It accepts nothing until they are known. It
// reads them on each call, so a Pipeline that keeps the filter sees them as
// soon as a retry after a transient failure fetches them.
type acceptedEvidence struct {
	e *CloudRequestEngine
}

// Include reports whether the cloud service accepts the Evidence key.
func (f acceptedEvidence) Include(key string) bool {
	if keys := f.e.evidenceKeys.Load(); keys != nil {
		return keys.Include(key)
	}
	return false
}

// Keys returns the Evidence keys the cloud service accepts, in sorted order,
// or none until they are known.
func (f acceptedEvidence) Keys() []string {
	if keys := f.e.evidenceKeys.Load(); keys != nil {
		return keys.Keys()
	}
	return nil
}

func (e *CloudRequestEngine) fetchAccessibleProperties() (map[string]ProductMeta, error) {
	raw, err := e.getEndpoint(e.propertiesEndpoint, true)
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
	// The evidence-keys endpoint does not take the resource key, avoiding
	// sending the key to an endpoint that ignores it.
	raw, err := e.getEndpoint(e.evidenceKeysEndpoint, false)
	if err != nil {
		return nil, err
	}
	var keys []string
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, NewCloudRequestError("could not parse evidence keys response", err)
	}
	return keys, nil
}

func (e *CloudRequestEngine) getEndpoint(endpoint string, appendResource bool) ([]byte, error) {
	u := endpoint
	if appendResource && e.resourceKey != "" {
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
		// The transport error embeds the request URL, which carries the
		// resource key on this endpoint; redact it before wrapping.
		return nil, NewCloudRequestError(fmt.Sprintf("error contacting cloud service at '%s'", endpoint), redactResourceKey(err))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if err := e.checkResponse(endpoint, resp.StatusCode, resp.Header, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// redactResourceKey removes the resource key from the URL embedded in a
// transport (*url.Error) so it cannot leak into logs or fd.Errors().
func redactResourceKey(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		if u, perr := url.Parse(ue.URL); perr == nil {
			q := u.Query()
			if q.Has("resource") {
				q.Set("resource", "***")
				u.RawQuery = q.Encode()
				ue.URL = u.String()
			}
		}
	}
	return err
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

// recordFailure counts a failed request toward the recovery period when it
// says the cloud service is in trouble.
func (e *CloudRequestEngine) recordFailure(err error) {
	if e.recoveryDuration <= 0 || !signalsServiceTrouble(err) {
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
		e.consecutiveRecoveries++
		e.recoveryUntil = now.Add(e.recoveryDurationFor(e.consecutiveRecoveries))
		e.failureTimes = e.failureTimes[:0]
	}
}

// signalsServiceTrouble reports whether a failed request says the cloud
// service is in trouble, being a request that did not complete, a status of
// 500 or more, a 429, or a success status whose body could not be used. Only
// those count toward the recovery period, because the period stops every
// visitor's requests, and any other status from 400 to 499 answers the
// content of that one request.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#recovery-mode
func signalsServiceTrouble(err error) bool {
	var cloudErr *CloudRequestError
	if !errors.As(err, &cloudErr) {
		return true
	}
	status := cloudErr.StatusCode
	switch {
	case status == 0, status >= 500, status == http.StatusTooManyRequests:
		return true
	default:
		return status >= 200 && status < 300
	}
}

// recoveryDurationFor returns the recovery duration for the nth consecutive
// trip. With the default multiplier of 1 this is always recoveryDuration; with
// a larger multiplier it grows as recoveryDuration * multiplier^(n-1), capped
// at maxRecoveryDuration when that is positive.
func (e *CloudRequestEngine) recoveryDurationFor(n int) time.Duration {
	if n < 1 || e.recoveryMultiplier <= 1 {
		return e.recoveryDuration
	}
	dur := time.Duration(float64(e.recoveryDuration) * math.Pow(e.recoveryMultiplier, float64(n-1)))
	if e.maxRecoveryDuration > 0 && dur > e.maxRecoveryDuration {
		return e.maxRecoveryDuration
	}
	return dur
}

// recordSuccess is called after a request succeeds so the exponential backoff
// resets to the base recovery duration for any future failures.
func (e *CloudRequestEngine) recordSuccess() {
	if e.recoveryDuration <= 0 {
		return
	}
	e.recMu.Lock()
	e.consecutiveRecoveries = 0
	e.recMu.Unlock()
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
