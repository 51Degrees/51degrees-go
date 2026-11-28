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
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md
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
// Its json-response is a lazy property whose task is the HTTP request the
// engine started while it processed. Every read of json-response waits for
// that request.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/cloud-request-engine.md?plain=1#L200-L202
type CloudRequestData struct {
	*engines.BaseAspectData
	failed bool
	// redeems is set where the request sent a sealed Human Confidence result
	// with the transaction kept for its page, so the answer redeems it.
	redeems bool
	// task is the request, nil where the engine failed before starting one.
	task *requestTask
	// fd is the Flow Data the data is held in, so that the first reader to
	// meet a failed request while it is processed can add the error to it.
	fd atomic.Pointer[flowDataRef]
}

// flowDataRef holds a Flow Data, so that it can be kept atomically.
type flowDataRef struct{ core.FlowData }

// requestTask is a request the engine started. done is closed once body or
// err is set, and neither changes after that.
type requestTask struct {
	engine *CloudRequestEngine
	done   chan struct{}
	body   string
	err    error
	// recorded is set by the first reader to meet err.
	recorded atomic.Bool
}

func newCloudRequestData() *CloudRequestData {
	return &CloudRequestData{BaseAspectData: engines.NewBaseAspectData()}
}

// AddedToFlowData keeps the Flow Data the data is held in, then does what
// Aspect Data does.
func (d *CloudRequestData) AddedToFlowData(fd core.FlowData, key string, data core.ElementData) {
	d.fd.CompareAndSwap(nil, &flowDataRef{fd})
	d.BaseAspectData.AddedToFlowData(fd, key, data)
}

// Response waits for the request and gives the raw JSON response body, or
// the error the request failed with. The first reader to meet a failure
// while the Flow Data is processed adds it once to the Flow Data's errors as
// an error of the Cloud Request Engine. A failure met once processing has
// finished reaches only the reader, because nothing can be added then.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/cloud-request-engine.md?plain=1#L364-L382
func (d *CloudRequestData) Response() (string, error) {
	t := d.task
	if t == nil {
		if d.failed {
			return "", core.NewDataError("the cloud request failed")
		}
		v, err := d.BaseAspectData.Get(JSONResponseProperty)
		s, _ := v.(string)
		return s, err
	}
	<-t.done
	if t.err != nil && t.recorded.CompareAndSwap(false, true) {
		if ref := d.fd.Load(); ref != nil && !core.ProcessingFinished(ref.FlowData) {
			ref.AddError(t.err, t.engine)
		}
	}
	return t.body, t.err
}

// Failed reports whether the cloud request ran but failed, waiting for it to
// finish. It lets a downstream Cloud Aspect Engine distinguish a failed
// request from one that never ran (so a missing Property can be reported as
// a cloud-request failure rather than the engine not being present).
func (d *CloudRequestData) Failed() bool {
	if d.failed {
		return true
	}
	if d.task == nil {
		return false
	}
	_, err := d.Response()
	return err != nil
}

// RedeemsHumanConfidence reports whether the request sent the sealed Human
// Confidence result its Flow Data carries, with the transaction the engine
// kept for its page, so that the answer's derived.humanconfidence is the
// redeemed Value. It is false where no sealed result was sent, being where
// the engine holds no transaction for the page or has no License Key.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#where-the-sealed-result-is-not-redeemed
func (d *CloudRequestData) RedeemsHumanConfidence() bool { return d.redeems }

// JSONResponse returns the raw JSON response body, waiting for the request,
// or an empty string where the request failed.
func (d *CloudRequestData) JSONResponse() string {
	s, _ := d.Response()
	return s
}

// Get returns the value of a Property by name. A read of json-response waits
// for the request, and gives its error where it failed.
func (d *CloudRequestData) Get(propertyName string) (any, error) {
	if d.task != nil && strings.EqualFold(propertyName, JSONResponseProperty) {
		s, err := d.Response()
		if err != nil {
			return nil, err
		}
		return s, nil
	}
	return d.BaseAspectData.Get(propertyName)
}

// AsMap returns every Property value, json-response included once the
// request has answered, waiting for it.
func (d *CloudRequestData) AsMap() map[string]any {
	m := d.BaseAspectData.AsMap()
	if d.task != nil {
		if s, err := d.Response(); err == nil {
			m[JSONResponseProperty] = s
		}
	}
	return m
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
	// configured credentials grant access to for this product.
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
// Properties. Both depend only on the credentials, so a host that starts
// often can keep the state from one build, for example as JSON, and give it
// to the next build with Builder.SetDiscoveryState so that build makes no
// request.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#resilience
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
	licenseKey  string
	origin      string

	// requestedProperties are the fully qualified names of the Properties a
	// request asks for, sent as values beside whichever key is set.
	requestedProperties []string
	// notReturnedReported is set once the first answer to a request that
	// asked for Properties has been compared with the request.
	notReturnedReported atomic.Bool

	// pages holds the transaction kept for each page issued a
	// HumanConfidencePage.
	pages pages

	dataEndpoint         string
	propertiesEndpoint   string
	evidenceKeysEndpoint string

	// The discovery results, nil until known. Each is set once, when the
	// engine is built or, after a transient failure then, by the first use
	// that fetches it.
	products     atomic.Pointer[map[string]ProductMeta]
	evidenceKeys atomic.Pointer[core.WhitelistFilter]

	// productsFailed is true from a failed request for the accessible
	// Properties until one succeeds.
	productsFailed atomic.Bool

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

	// tasksMu is held for reading while Process starts a request, and for
	// writing by Close, so that no request starts once Close waits for
	// tasks.
	tasksMu sync.RWMutex
	closed  bool
	tasks   sync.WaitGroup
}

// errEngineClosed is the error Process gives once the engine is closed.
var errEngineClosed = errors.New("the Cloud Request Engine has been closed")

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

// PublicPropertiesRequestFailed reports whether the last request for the
// accessible Properties failed, leaving them unknown. It makes no request.
func (e *CloudRequestEngine) PublicPropertiesRequestFailed() bool {
	return e.products.Load() == nil && e.productsFailed.Load()
}

// PublicProperties returns the accessible Properties, keyed by product, which
// are normally fetched when the engine is built. After a transient failure
// then, a call fetches them unless the engine is in its recovery period, so
// call it while processing, where SuppressProcessExceptions covers a failure.
// It returns a copy, so a caller may change it without changing the engine.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#revised-design-eager-discovery-at-build-time-transient-failure-fallback
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

// Process builds the cloud request from the Evidence, starts it, and adds
// Element Data whose json-response is a lazy property whose task is that
// request, returning without waiting for the response. The task holds only
// the request, and never reads the Flow Data.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/cloud-request-engine.md?plain=1#L364-L382
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

	form, redeems := e.buildForm(fd)
	task, err := e.startRequest(form)
	if err != nil {
		return err
	}
	data := newCloudRequestData()
	data.task = task
	data.redeems = redeems
	fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData { return data })
	return nil
}

// startRequest starts the data request for form on its own goroutine, and
// gives its task, or an error once the engine is closed. The goroutine
// counts the outcome toward the recovery period before the task finishes.
// A panic on the goroutine fails the task with an error naming the panic,
// as a panic in Process fails that Flow Element, rather than end the
// program.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L178-L183
func (e *CloudRequestEngine) startRequest(form url.Values) (*requestTask, error) {
	e.tasksMu.RLock()
	defer e.tasksMu.RUnlock()
	if e.closed {
		return nil, errEngineClosed
	}
	t := &requestTask{engine: e, done: make(chan struct{})}
	secrets := e.secretsIn(form)
	e.tasks.Add(1)
	go func() {
		defer e.tasks.Done()
		defer close(t.done)
		// Deferred last, so it runs before done is closed. The panic's value
		// can hold anything the request held, so the secrets are hidden in it.
		defer func() {
			if r := recover(); r != nil {
				t.body = ""
				t.err = errors.New(hide(fmt.Sprintf("panic processing element '%s': %v",
					e.DataKey(), r), secrets))
			}
		}()
		body, err := e.makeDataRequest(form)
		if err != nil {
			e.recordFailure(err)
			t.err = err
			return
		}
		e.recordSuccess()
		e.reportPropertiesNotReturned(body)
		t.body = body
	}()
	return t, nil
}

// Close waits for every request the engine has started, so that closing the
// engine or its Pipeline waits for every lazy task, and the engine processes
// nothing after it.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L33-L40
func (e *CloudRequestEngine) Close() error {
	e.tasksMu.Lock()
	e.closed = true
	e.tasksMu.Unlock()
	e.tasks.Wait()
	return e.BaseAspectEngine.Close()
}

// storeFailed records that the request ran but failed, so a downstream Cloud
// Aspect Engine can tell this apart from the request engine never running (for
// example under suppressed exceptions, where Process's error is only logged).
func (e *CloudRequestEngine) storeFailed(fd core.FlowData) {
	data := newCloudRequestData()
	data.failed = true
	fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData { return data })
}

func (e *CloudRequestEngine) makeDataRequest(form url.Values) (string, error) {
	raw, resp, err := e.postEndpoint(e.dataEndpoint, form)
	if err != nil {
		return "", err
	}
	if !json.Valid(raw) {
		return "", NewCloudResponseError(
			fmt.Sprintf("failed to parse the response from cloud service at '%s' as JSON",
				e.dataEndpoint), resp.StatusCode, resp.Header)
	}
	return string(raw), nil
}

// postEndpoint posts form to endpoint as URL-encoded form data, so the
// credentials it holds stay out of the URL, and returns the checked answer.
// No message it builds holds the License Key, or the sealed result or the
// transaction the form carries.
func (e *CloudRequestEngine) postEndpoint(endpoint string, form url.Values) ([]byte, *http.Response, error) {
	secrets := e.secretsIn(form)
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	e.setCommonHeaders(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, nil, NewCloudRequestError(fmt.Sprintf("error contacting cloud service at '%s'", endpoint),
			hideInError(redactResourceKey(err), secrets))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	e.logWarnings(raw, secrets)

	if err := e.checkResponse(endpoint, resp.StatusCode, resp.Header, raw, secrets); err != nil {
		return nil, nil, err
	}
	return raw, resp, nil
}

// credentials returns the configured keys as the form fields the cloud
// service reads them from, being resource and license.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#credentials
func (e *CloudRequestEngine) credentials() url.Values {
	form := url.Values{}
	if e.resourceKey != "" {
		form.Set("resource", e.resourceKey)
	}
	if e.licenseKey != "" {
		form.Set("license", e.licenseKey)
	}
	return form
}

// licenseKeys returns the License Key, which no error or log entry may hold
// in full, or none.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/logging.md#license-keys
func (e *CloudRequestEngine) licenseKeys() []string {
	if e.licenseKey == "" {
		return nil
	}
	return []string{e.licenseKey}
}

// secretsIn gives the values no message may hold, in whole or in part,
// being the License Key, and the sealed result and the transaction form
// carries.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#what-a-package-never-exposes
func (e *CloudRequestEngine) secretsIn(form url.Values) []string {
	secrets := e.licenseKeys()
	for _, field := range []string{"humanconfidence", "transaction"} {
		if value := form.Get(field); value != "" {
			secrets = append(secrets, value)
		}
	}
	return secrets
}

// hidden is what every message the engine builds shows in place of its
// License Key, a sealed result or a transaction. It is the same for every
// value, so it gives away neither the value nor its length. It is the
// engine's own, so the masking of data file license keys in pipeline/core,
// which keeps their last four characters, is not changed by it.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#what-a-package-never-exposes
const hidden = "(hidden)"

// hide returns text with each of secrets replaced by hidden, so no part of
// it is shown, nor its length. Each is found as it is written, URL-encoded,
// and as a JSON string may escape it, since an answer the cloud service
// writes can hold it in any of these forms.
func hide(text string, secrets []string) string {
	sorted := slices.Clone(secrets)
	slices.SortFunc(sorted, func(a, b string) int { return len(b) - len(a) })
	for _, secret := range sorted {
		if secret == "" {
			continue
		}
		for _, f := range writtenForms(secret) {
			text = strings.ReplaceAll(text, f, hidden)
		}
	}
	return text
}

// writtenForms gives the ways secret can be written: as it is, URL-encoded,
// and JSON-escaped, with "/" as "\/" or with every character other than a
// letter, a digit, "-", "_", "." or "~" as a \u escape in either case.
func writtenForms(secret string) []string {
	forms := []string{secret}
	add := func(f string) {
		if !slices.Contains(forms, f) {
			forms = append(forms, f)
		}
	}
	add(url.QueryEscape(secret))
	if quoted, err := json.Marshal(secret); err == nil {
		add(string(quoted[1 : len(quoted)-1]))
	}
	add(strings.ReplaceAll(secret, "/", `\/`))
	var upper, lower strings.Builder
	for _, r := range secret {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.', r == '~':
			upper.WriteRune(r)
			lower.WriteRune(r)
		default:
			fmt.Fprintf(&upper, `\u%04X`, r)
			fmt.Fprintf(&lower, `\u%04x`, r)
		}
	}
	add(upper.String())
	add(lower.String())
	return forms
}

// hideInError returns err with none of secrets in its message, keeping its
// type where the message holds none.
func hideInError(err error, secrets []string) error {
	if err == nil || len(secrets) == 0 {
		return err
	}
	text := err.Error()
	if masked := hide(text, secrets); masked != text {
		return errors.New(masked)
	}
	return err
}

// reportPropertiesNotReturned compares the Properties a request asked for
// with the answer, the first time an answer arrives, and logs a warning
// naming any the answer left out. With a License Key, the cloud service drops
// a Property the License Key does not cover without naming it when the
// request also asked for one it covers. With a Resource Key alone, a Property
// no licence grants comes back null, so only a name that is not a Property is
// left out. A Property that is present but null counts as answered,
// because its nullreason explains it. Names are compared in any letter case,
// because the answer is in lower case. The answer cannot change while the
// key and the list stay the same, so the comparison is made once.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#properties-the-remote-server-did-not-return
func (e *CloudRequestEngine) reportPropertiesNotReturned(body string) {
	if len(e.requestedProperties) == 0 || e.notReturnedReported.Load() {
		return
	}
	answer, ok := lowerKeys([]byte(body))
	if !ok || !e.notReturnedReported.CompareAndSwap(false, true) {
		return
	}
	var missing []string
	for _, name := range e.requestedProperties {
		section, property, _ := strings.Cut(strings.ToLower(name), ".")
		values, ok := lowerKeys(answer[section])
		if _, answered := values[property]; !ok || !answered {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		reason := "They are not covered by the License Key in use"
		if e.licenseKey == "" {
			reason = "The cloud service has no Properties with those names"
		}
		e.Logger().Warn(fmt.Sprintf("The cloud service did not return the "+
			"requested Properties %s in its answer. %s, so they will be "+
			"missing from every result. The other requested Properties came "+
			"back and are usable.", strings.Join(missing, ", "), reason))
	}
}

// lowerKeys parses raw as a JSON object, with its keys in lower case.
func lowerKeys(raw []byte) (map[string]json.RawMessage, bool) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, false
	}
	out := make(map[string]json.RawMessage, len(object))
	for key, value := range object {
		out[strings.ToLower(key)] = value
	}
	return out, true
}

// engineFields are the form fields that carry what the engine writes itself,
// licence being the service's other name for license. Evidence that strips to
// one of them is left out, whichever keys the cloud service lists, because a
// second field for the same value would leave which one applied undecided.
var engineFields = map[string]bool{
	"resource": true, "license": true, "licence": true, "values": true}

// transactionFields are left out of a request that carries a sealed Human
// Confidence result, under any prefix and in any letter case, because the
// only transaction that call may hold is one the package kept, never one
// taken from the request. challenge is the older name for the same value.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#redeeming-through-the-json-call
var transactionFields = map[string]bool{"transaction": true, "challenge": true}

// HumanConfidenceEvidence is the Evidence that carries a sealed Human
// Confidence result, which the page sends to the publisher's server as the
// parameter humanconfidence.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#when-humanconfidence-arrives-sealed
const HumanConfidenceEvidence = "query.humanconfidence"

// humanConfidenceProperty is the Property a request that carries a sealed
// result must name, so that the answer redeems it.
const humanConfidenceProperty = "derived.HumanConfidence"

// CarriesSealedResult reports whether fd's Evidence holds a sealed Human
// Confidence result, being HumanConfidenceEvidence with a value that is not
// empty.
func CarriesSealedResult(fd core.FlowData) bool {
	value, ok := fd.Evidence().Get(HumanConfidenceEvidence)
	return ok && valueToString(value) != ""
}

// humanConfidenceField and humanConfidencePageField are the names the sealed
// result and the page id have once the prefix is removed.
const (
	humanConfidenceField     = "humanconfidence"
	humanConfidencePageField = "humanconfidencepage"
)

// carriesHumanConfidenceElsewhere reports whether fd's Evidence holds a value
// that is not empty under a key other than HumanConfidenceEvidence whose name
// is humanconfidence, in any letter case: under a header, a cookie, server.,
// or no prefix. Only HumanConfidenceEvidence is a sealed result, so such a
// value is never sent, nor is a transaction or challenge from the request,
// which the cloud service could otherwise redeem it with.
func carriesHumanConfidenceElsewhere(fd core.FlowData) bool {
	for key, raw := range fd.Evidence().AsMap() {
		if strings.EqualFold(key, HumanConfidenceEvidence) {
			continue
		}
		if strings.EqualFold(evidenceField(key), humanConfidenceField) && valueToString(raw) != "" {
			return true
		}
	}
	return false
}

// evidenceField gives the name of an Evidence key with its prefix removed,
// being the whole key where it has no prefix.
func evidenceField(key string) string {
	if _, field := core.SplitEvidenceKey(key); field != "" {
		return field
	}
	return key
}

// String names the engine and nothing it holds, so printing the engine, as
// with %v or %+v, never shows its License Key or a transaction it keeps.
func (e *CloudRequestEngine) String() string { return "CloudRequestEngine(" + e.DataKey() + ")" }

// GoString gives the same as String, for %#v.
func (e *CloudRequestEngine) GoString() string { return e.String() }

// HasLicenseKey reports whether the engine was given a License Key, which a
// sealed Human Confidence result needs to be redeemed.
func (e *CloudRequestEngine) HasLicenseKey() bool { return e.licenseKey != "" }

// buildForm constructs the URL-encoded form, being the credentials, the
// Requested Properties as values, comma separated, and the Evidence, as
// resolveEvidence gives it. It reports whether the form redeems a sealed
// Human Confidence result.
//
// A request that carries a sealed result takes the transaction kept for its
// page, so the transaction is used once, and leaves out any transaction or
// challenge in its Evidence. With a kept transaction and a License Key it
// sends the sealed result with that transaction, and names
// derived.HumanConfidence where the configured list is not empty and lacks
// it, even where the configured list is empty, because the cloud service
// needs a call with a License Key to name its Properties. The name is added
// to the form only, so the report of Properties not returned is still made
// for the configured list alone. With no kept transaction, or no License
// Key, the sealed result is not sent and is not redeemed. A value named
// humanconfidence under any other key is never sent, and a request that has
// one, but no sealed result, leaves out its transaction and challenge too.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#processing
// and https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#redeeming-through-the-json-call
func (e *CloudRequestEngine) buildForm(fd core.FlowData) (url.Values, bool) {
	form := e.credentials()
	sealed := CarriesSealedResult(fd)
	transaction, redeems := "", false
	if sealed {
		transaction, redeems = e.pages.take(evidenceText(fd, HumanConfidencePageEvidence), e.now())
		redeems = redeems && e.HasLicenseKey()
	}
	values := e.requestedProperties
	if redeems && !slices.ContainsFunc(values, func(name string) bool {
		return strings.EqualFold(name, humanConfidenceProperty)
	}) {
		values = append(slices.Clip(values), humanConfidenceProperty)
	}
	if len(values) > 0 {
		form.Set("values", strings.Join(values, ","))
	}
	stray := !sealed && carriesHumanConfidenceElsewhere(fd)
	for field, value := range e.resolveEvidence(fd, sealed || stray, sealed || stray, redeems) {
		form.Set(field, value)
	}
	if redeems {
		form.Set("transaction", transaction)
	}
	return form, redeems
}

// evidenceText gives the Evidence under key as text, or empty where there is
// none.
func evidenceText(fd core.FlowData, key string) string {
	if raw, ok := fd.Evidence().Get(key); ok && raw != nil {
		return valueToString(raw)
	}
	return ""
}

// resolveEvidence returns every Evidence value with its prefix removed,
// leaving out the web request, which is not a value to send, the page id
// under any prefix, which is only a handle for this server, any that strips
// to a field the engine writes itself, and any that strips to transaction or
// challenge where dropTransactions is true. Where dropHumanConfidence is
// true, every value named humanconfidence is left out too, under any prefix
// and in any letter case, but for the sealed result itself where
// keepSealed is true.
// The keys the cloud service
// accepts do not limit it, as what the service uses changes and the other
// Evidence helps it improve. Conflicts between prefixes are resolved using
// the defined precedence, so a key with no prefix ranks as a key whose
// prefix is the whole key, giving query the precedence of query Evidence
// and anything else the lowest precedence.
func (e *CloudRequestEngine) resolveEvidence(fd core.FlowData, dropTransactions, dropHumanConfidence, keepSealed bool) map[string]string {
	type choice struct {
		prefix string
		value  string
	}
	best := map[string]choice{}
	for key, raw := range fd.Evidence().AsMap() {
		if strings.EqualFold(key, core.EvidenceHTTPRequest) {
			continue
		}
		prefix, field := core.SplitEvidenceKey(key)
		if prefix == "" {
			prefix = key
		}
		name := strings.ToLower(field)
		if engineFields[name] || name == humanConfidencePageField ||
			(dropTransactions && transactionFields[name]) ||
			(dropHumanConfidence && name == humanConfidenceField &&
				!(keepSealed && strings.EqualFold(key, HumanConfidenceEvidence))) {
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

// maxLoggedContentBytes is how much of a failed response's content its error
// shows.
const maxLoggedContentBytes = 512

// checkResponse implements the error scenarios required of cloud HTTP requests.
// An errors array holding more than one entry gives one error for each, held
// together in a CloudRequestErrors.
// The License Key is masked in each message, as the cloud service may write
// it in its answer.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#http-requests
func (e *CloudRequestEngine) checkResponse(url string, status int, headers http.Header, raw []byte, secrets []string) error {
	if msgs := extractMessages(raw, "errors"); len(msgs) > 0 {
		for i, msg := range msgs {
			msgs[i] = hide(msg, secrets)
		}
		return NewCloudResponseErrors(msgs, status, headers)
	}
	if len(raw) == 0 {
		return NewCloudResponseError(fmt.Sprintf("No data in response from cloud service at '%s'", url), status, headers)
	}
	if status != http.StatusOK {
		// A call that carried a sealed result or a transaction could be
		// answered with a part of either, which can't be found to hide, so
		// the content of its answer is left out.
		if len(secrets) > len(e.licenseKeys()) {
			return NewCloudResponseError(fmt.Sprintf("Cloud service at '%s' returned "+
				"status code '%d'. Its content is left out, as the call carried a "+
				"sealed result or a transaction.", url, status), status, headers)
		}
		// The content is masked before it is cut, so a License Key across
		// the cut is still found.
		content := truncate(hide(string(raw), secrets), maxLoggedContentBytes)
		return NewCloudResponseError(hide(
			fmt.Sprintf("Cloud service at '%s' returned status code '%d' with content %s",
				url, status, content), secrets), status, headers)
	}
	return nil
}

// logWarnings logs each entry of an answer's warnings array as a warning. It
// runs on every answer the engine reads, from discovery and data requests
// alike, with the License Key masked.
func (e *CloudRequestEngine) logWarnings(raw []byte, secrets []string) {
	for _, w := range extractMessages(raw, "warnings") {
		e.Logger().Warn("cloud service warning", "warning", hide(w, secrets))
	}
}

// --- Discovery -----------------------------------------------------------
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#start-up-activity

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
		e.productsFailed.Store(true)
		e.recordFailure(err)
		return err
	}
	e.productsFailed.Store(false)
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

// fetchAccessibleProperties asks for the accessible Properties. A Resource
// Key alone goes in the query of a GET. Whenever a License Key is set, the
// credentials are posted in the body instead, so the License Key stays out
// of the URL and of request logs. For a License Key alone the service answers
// with every Property the License Key entitles.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#discovery-under-a-license-key
func (e *CloudRequestEngine) fetchAccessibleProperties() (map[string]ProductMeta, error) {
	var raw []byte
	var err error
	if e.licenseKey == "" {
		raw, err = e.getEndpoint(e.propertiesEndpoint, true)
	} else {
		raw, _, err = e.postEndpoint(e.propertiesEndpoint, e.credentials())
	}
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
	e.logWarnings(raw, e.licenseKeys())
	if err := e.checkResponse(endpoint, resp.StatusCode, resp.Header, raw, e.licenseKeys()); err != nil {
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
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#recovery-mode
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
