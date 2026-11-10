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

package requestengine

// These tests follow the specification's rules for the credentials, the
// Requested Properties and the Properties the cloud service did not return.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/cloud-request-engine.md#credentials

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// testLicenseKey is the License Key the tests use, which no error or log
// entry may hold in full.
const testLicenseKey = "TESTLICENSEKEY0123456789"

// testResourceKey is the Resource Key the tests use.
const testResourceKey = "test-resource-key"

// recordedRequest is what the stand-in cloud received in one request.
type recordedRequest struct {
	method string
	query  url.Values
	form   url.Values
}

// credentialCloud is a stand-in cloud service that records the last request
// to each endpoint, and counts them.
type credentialCloud struct {
	// The answers, set before the engine is built. A status of 0 means 200.
	propertiesStatus int
	propertiesBody   string
	jsonStatus       int
	jsonBody         string

	mu       sync.Mutex
	requests map[string]recordedRequest
	counts   map[string]int
}

// newCredentialCloud returns a stand-in whose device product has ismobile
// and iscrawler, and which answers a data request with both.
func newCredentialCloud() *credentialCloud {
	return &credentialCloud{
		propertiesBody: `{"Products":{"device":{"DataTier":"CloudV4",` +
			`"Properties":[{"Name":"ismobile","Type":"Bool","Category":"Device"},` +
			`{"Name":"iscrawler","Type":"Bool","Category":"Device"}]}}}`,
		jsonBody: `{"device":{"ismobile":true,"iscrawler":false}}`,
		requests: map[string]recordedRequest{},
		counts:   map[string]int{},
	}
}

func (c *credentialCloud) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		path := strings.TrimPrefix(r.URL.Path, "/")
		c.mu.Lock()
		c.requests[path] = recordedRequest{r.Method, r.URL.Query(), r.PostForm}
		c.counts[path]++
		c.mu.Unlock()
		answer := func(status int, body string) {
			if status != 0 {
				w.WriteHeader(status)
			}
			io.WriteString(w, body)
		}
		switch path {
		case "accessibleproperties":
			answer(c.propertiesStatus, c.propertiesBody)
		case "evidencekeys":
			answer(0, `["header.user-agent","query.user-agent",`+
				`"query.values","query.license","query.resource"]`)
		case "json":
			answer(c.jsonStatus, c.jsonBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// last returns the last request to path and how many path received.
func (c *credentialCloud) last(path string) (recordedRequest, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests[path], c.counts[path]
}

// total returns how many requests the stand-in received.
func (c *credentialCloud) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, count := range c.counts {
		n += count
	}
	return n
}

// credentialBuilder is a builder for the stand-in at srv, logging to log.
func credentialBuilder(srv *httptest.Server, log *logRecorder) *Builder {
	return NewBuilder().
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		SetLogger(slog.New(log))
}

// processWith runs one request through p with the Evidence given.
func processWith(t *testing.T, p core.Pipeline, evidence map[string]string) error {
	t.Helper()
	fd := p.CreateFlowData()
	for key, value := range evidence {
		fd.AddEvidence(key, value)
	}
	return fd.Process()
}

// recordText returns a log record's message and the values of its
// attributes, as one string.
func recordText(r slog.Record) string {
	parts := []string{r.Message}
	r.Attrs(func(a slog.Attr) bool {
		parts = append(parts, a.Value.String())
		return true
	})
	return strings.Join(parts, " ")
}

// Every row of the specification's table of credentials. The five valid
// rows build, and the two invalid ones fail the build with a
// configuration error that names the fix, before any request is made. A
// value that is empty or only whitespace counts as absent.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/cloud-request-engine.md#accepted-combinations
func TestCredentialCombinations(t *testing.T) {
	properties := []string{"device.ismobile"}
	cases := []struct {
		name       string
		resource   string
		license    string
		properties []string
		// refusal is text the error must hold, or "" when the row is valid.
		refusal string
	}{
		{"Resource Key", testResourceKey, "", nil, ""},
		{"Resource Key and License Key", testResourceKey, testLicenseKey, nil, ""},
		{"License Key and Properties", "", testLicenseKey, properties, ""},
		{"License Key alone", "", testLicenseKey, nil, "SetRequestedProperties"},
		{"License Key and blank Properties", "", testLicenseKey,
			[]string{" ", ""}, "SetRequestedProperties"},
		{"Resource Key and Properties", testResourceKey, "", properties, ""},
		{"both keys and Properties", testResourceKey, testLicenseKey,
			properties, ""},
		{"no credential", "", "", nil, "SetResourceKey or SetLicenseKey"},
		{"no credential and Properties", "", "", properties,
			"SetResourceKey or SetLicenseKey"},
		{"blank credentials", " ", "\t", properties,
			"SetResourceKey or SetLicenseKey"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cloud := newCredentialCloud()
			b := credentialBuilder(cloud.serve(t), &logRecorder{}).
				SetResourceKey(c.resource).
				SetLicenseKey(c.license)
			if c.properties != nil {
				b.SetRequestedProperties(c.properties...)
			}
			e, err := b.Build()
			if c.refusal == "" {
				if err != nil {
					t.Fatalf("a valid combination fails the build: %v", err)
				}
				return
			}
			var configErr *core.ConfigurationError
			if e != nil || !errors.As(err, &configErr) {
				t.Fatalf("build gives %v, %v, want a configuration error", e, err)
			}
			if !strings.Contains(err.Error(), c.refusal) {
				t.Errorf("the error %q does not say %q", err, c.refusal)
			}
			if strings.Contains(err.Error(), testLicenseKey) {
				t.Errorf("the error %q holds the License Key", err)
			}
			if n := cloud.total(); n != 0 {
				t.Errorf("a refused build made %d requests, want none", n)
			}
		})
	}
}

// With a License Key alone, the data request carries the License Key as
// license and the Requested Properties as values, comma separated, with no
// resource, all in the body and nothing in the URL. Evidence that strips to
// values or license is still left out.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/cloud-request-engine.md#processing
func TestLicenseKeyAloneSendsLicenseAndValues(t *testing.T) {
	cloud := newCredentialCloud()
	e, err := credentialBuilder(cloud.serve(t), &logRecorder{}).
		SetLicenseKey(testLicenseKey).
		SetRequestedProperties("device.ismobile", " device.iscrawler ").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := processWith(t, pipelineOf(t, e, false), map[string]string{
		"header.user-agent": "ua",
		"query.values":      "device.hardwarevendor",
		"query.license":     "evidence-license",
	}); err != nil {
		t.Fatalf("process: %v", err)
	}
	req, n := cloud.last("json")
	want := url.Values{
		"license":    {testLicenseKey},
		"values":     {"device.ismobile,device.iscrawler"},
		"user-agent": {"ua"},
	}
	if n != 1 || req.method != http.MethodPost ||
		!reflect.DeepEqual(req.form, want) || len(req.query) != 0 {
		t.Errorf("the data request was %d %s with the query %v and the body "+
			"%v, want one POST with no query and the body %v",
			n, req.method, req.query, req.form, want)
	}
}

// With a Resource Key and a License Key, the data request carries both and
// no values.
func TestResourceAndLicenseKeysSendBoth(t *testing.T) {
	cloud := newCredentialCloud()
	e, err := credentialBuilder(cloud.serve(t), &logRecorder{}).
		SetResourceKey(testResourceKey).
		SetLicenseKey(testLicenseKey).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := processWith(t, pipelineOf(t, e, false),
		map[string]string{"header.user-agent": "ua"}); err != nil {
		t.Fatalf("process: %v", err)
	}
	req, _ := cloud.last("json")
	want := url.Values{
		"resource":   {testResourceKey},
		"license":    {testLicenseKey},
		"user-agent": {"ua"},
	}
	if !reflect.DeepEqual(req.form, want) || len(req.query) != 0 {
		t.Errorf("the data request had the query %v and the body %v, want "+
			"no query and the body %v", req.query, req.form, want)
	}
}

// With a Resource Key and Requested Properties, the data request carries the
// Resource Key and the Requested Properties as values, so the cloud service
// adds each one to those the Resource Key carries, as null with a nullreason
// where no licence grants it.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/cloud-request-engine.md#requested-properties
func TestResourceKeyAndPropertiesSendValues(t *testing.T) {
	cloud := newCredentialCloud()
	e, err := credentialBuilder(cloud.serve(t), &logRecorder{}).
		SetResourceKey(testResourceKey).
		SetRequestedProperties("device.hardwarevendor").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := processWith(t, pipelineOf(t, e, false),
		map[string]string{"header.user-agent": "ua"}); err != nil {
		t.Fatalf("process: %v", err)
	}
	req, _ := cloud.last("json")
	want := url.Values{
		"resource":   {testResourceKey},
		"values":     {"device.hardwarevendor"},
		"user-agent": {"ua"},
	}
	if !reflect.DeepEqual(req.form, want) || len(req.query) != 0 {
		t.Errorf("the data request had the query %v and the body %v, want "+
			"no query and the body %v", req.query, req.form, want)
	}
}

// With both keys and Requested Properties, the data request carries the
// Resource Key, the License Key and the Requested Properties as values, all
// in the body, so the cloud service answers with the Resource Key's
// Properties plus the requested ones the License Key grants.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/cloud-request-engine.md#license-key
func TestBothKeysAndPropertiesSendValues(t *testing.T) {
	cloud := newCredentialCloud()
	e, err := credentialBuilder(cloud.serve(t), &logRecorder{}).
		SetResourceKey(testResourceKey).
		SetLicenseKey(testLicenseKey).
		SetRequestedProperties("ip.registeredname").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := processWith(t, pipelineOf(t, e, false),
		map[string]string{"header.user-agent": "ua"}); err != nil {
		t.Fatalf("process: %v", err)
	}
	req, _ := cloud.last("json")
	want := url.Values{
		"resource":   {testResourceKey},
		"license":    {testLicenseKey},
		"values":     {"ip.registeredname"},
		"user-agent": {"ua"},
	}
	if !reflect.DeepEqual(req.form, want) || len(req.query) != 0 {
		t.Errorf("the data request had the query %v and the body %v, want "+
			"no query and the body %v", req.query, req.form, want)
	}
}

// With a Resource Key alone, the data request is as it was, carrying the
// Resource Key and the Evidence and no license or values.
func TestResourceKeyAloneRequestIsUnchanged(t *testing.T) {
	cloud := newCredentialCloud()
	e, err := credentialBuilder(cloud.serve(t), &logRecorder{}).
		SetResourceKey(testResourceKey).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := processWith(t, pipelineOf(t, e, false),
		map[string]string{"header.user-agent": "ua"}); err != nil {
		t.Fatalf("process: %v", err)
	}
	req, _ := cloud.last("json")
	want := url.Values{"resource": {testResourceKey}, "user-agent": {"ua"}}
	if req.method != http.MethodPost || !reflect.DeepEqual(req.form, want) {
		t.Errorf("the data request was %s with the body %v, want POST with %v",
			req.method, req.form, want)
	}
}

// Discovery works with each valid set of credentials. A Resource Key alone
// asks for the accessible Properties with a GET carrying it in the query, as
// before. Whenever a License Key is set, the request is a POST carrying the
// credentials in the body, so the License Key stays out of the URL. The
// evidencekeys request carries no credential.
func TestDiscoveryWithEachCredential(t *testing.T) {
	properties := []string{"device.ismobile"}
	cases := []struct {
		name       string
		resource   string
		license    string
		properties []string
		method     string
		query      url.Values
		form       url.Values
	}{
		{"Resource Key", testResourceKey, "", nil, http.MethodGet,
			url.Values{"resource": {testResourceKey}}, url.Values{}},
		{"License Key", "", testLicenseKey, properties, http.MethodPost,
			url.Values{}, url.Values{"license": {testLicenseKey}}},
		{"both keys", testResourceKey, testLicenseKey, nil, http.MethodPost,
			url.Values{}, url.Values{"resource": {testResourceKey},
				"license": {testLicenseKey}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cloud := newCredentialCloud()
			b := credentialBuilder(cloud.serve(t), &logRecorder{}).
				SetResourceKey(c.resource).
				SetLicenseKey(c.license)
			if c.properties != nil {
				b.SetRequestedProperties(c.properties...)
			}
			e, err := b.Build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			req, n := cloud.last("accessibleproperties")
			if n != 1 || req.method != c.method ||
				!reflect.DeepEqual(req.query, c.query) ||
				!reflect.DeepEqual(req.form, c.form) {
				t.Errorf("accessibleproperties got %d requests, the last %s "+
					"with the query %v and the body %v, want one %s with %v "+
					"and %v", n, req.method, req.query, req.form, c.method,
					c.query, c.form)
			}
			keys, n := cloud.last("evidencekeys")
			if n != 1 || keys.method != http.MethodGet ||
				len(keys.query) != 0 || len(keys.form) != 0 {
				t.Errorf("evidencekeys got %d requests, the last %s with the "+
					"query %v and the body %v, want one GET with neither",
					n, keys.method, keys.query, keys.form)
			}
			products, err := e.PublicProperties()
			if err != nil || len(products["device"].Properties) != 2 {
				t.Errorf("PublicProperties gives %v, %v, want the device "+
					"product", products, err)
			}
			wantKeys(t, e.EvidenceKeyFilter(), []string{"header.user-agent",
				"query.license", "query.resource", "query.user-agent",
				"query.values"})
		})
	}
}

// The License Key never appears in full in an error or a log entry, even
// when the cloud service writes it in its answer. It appears masked, with
// only its last four characters showing.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/features/logging.md#license-keys
func TestLicenseKeyIsMasked(t *testing.T) {
	echo := `{"errors":["The license key ` + testLicenseKey + ` is not valid"]}`
	masked := core.MaskLicenseKey(testLicenseKey)
	check := func(t *testing.T, what, text string) {
		t.Helper()
		if strings.Contains(text, testLicenseKey) {
			t.Errorf("%s %q holds the License Key", what, text)
		}
	}

	t.Run("refused at build", func(t *testing.T) {
		cloud := newCredentialCloud()
		cloud.propertiesStatus = http.StatusUnauthorized
		cloud.propertiesBody = echo
		_, err := credentialBuilder(cloud.serve(t), &logRecorder{}).
			SetLicenseKey(testLicenseKey).
			SetRequestedProperties("device.ismobile").
			Build()
		if err == nil {
			t.Fatal("the build succeeded")
		}
		check(t, "the error", err.Error())
		if !strings.Contains(err.Error(), masked) {
			t.Errorf("the error %q does not show the masked key %q", err, masked)
		}
	})

	t.Run("transient at build", func(t *testing.T) {
		cloud := newCredentialCloud()
		cloud.propertiesStatus = http.StatusServiceUnavailable
		cloud.propertiesBody = "unavailable for " + testLicenseKey
		log := &logRecorder{}
		if _, err := credentialBuilder(cloud.serve(t), log).
			SetLicenseKey(testLicenseKey).
			SetRequestedProperties("device.ismobile").
			Build(); err != nil {
			t.Fatalf("build: %v", err)
		}
		records := log.all()
		if len(records) == 0 {
			t.Fatal("nothing was logged")
		}
		for _, r := range records {
			check(t, "the log entry", recordText(r))
		}
	})

	t.Run("data request", func(t *testing.T) {
		cloud := newCredentialCloud()
		cloud.jsonStatus = http.StatusInternalServerError
		cloud.jsonBody = "failed for " + testLicenseKey
		e, err := credentialBuilder(cloud.serve(t), &logRecorder{}).
			SetLicenseKey(testLicenseKey).
			SetRequestedProperties("device.ismobile").
			Build()
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		err = processWith(t, pipelineOf(t, e, false), nil)
		if err == nil {
			t.Fatal("the request succeeded")
		}
		check(t, "the error", err.Error())
	})

	t.Run("warnings", func(t *testing.T) {
		cloud := newCredentialCloud()
		cloud.jsonBody = `{"device":{"ismobile":true},"warnings":` +
			`["Requests with ` + testLicenseKey + ` are slow"]}`
		log := &logRecorder{}
		e, err := credentialBuilder(cloud.serve(t), log).
			SetLicenseKey(testLicenseKey).
			SetRequestedProperties("device.ismobile").
			Build()
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if err := processWith(t, pipelineOf(t, e, false), nil); err != nil {
			t.Fatalf("process: %v", err)
		}
		records := log.all()
		if len(records) == 0 {
			t.Fatal("the warning was not logged")
		}
		for _, r := range records {
			check(t, "the log entry", recordText(r))
		}
	})
}

// A request that names Properties is compared with the answer, in any
// letter case, and the Properties the answer left out are logged once as
// not covered by the credential, while every request succeeds. A Property
// that is present but null counts as answered.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/cloud-request-engine.md#properties-the-remote-server-did-not-return
func TestPropertiesNotReturnedAreLoggedOnce(t *testing.T) {
	cloud := newCredentialCloud()
	cloud.jsonBody = `{"device":{"ismobile":true,"iscrawler":null,` +
		`"iscrawlernullreason":"Not known"}}`
	log := &logRecorder{}
	e, err := credentialBuilder(cloud.serve(t), log).
		SetLicenseKey(testLicenseKey).
		SetRequestedProperties("Device.IsMobile", "device.HardwareVendor",
			"device.iscrawler", "location.town").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p := pipelineOf(t, e, false)
	for i := 1; i <= 3; i++ {
		if err := processWith(t, p, nil); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	records := log.all()
	if len(records) != 1 {
		for _, r := range records {
			t.Logf("%s %s", r.Level, recordText(r))
		}
		t.Fatalf("logged %d times over 3 requests, want once", len(records))
	}
	text := strings.ToLower(recordText(records[0]))
	if records[0].Level != slog.LevelWarn {
		t.Errorf("logged at %s, want %s", records[0].Level, slog.LevelWarn)
	}
	for _, name := range []string{"device.hardwarevendor", "location.town"} {
		if !strings.Contains(text, name) {
			t.Errorf("the log entry %q does not name %s", text, name)
		}
	}
	for _, name := range []string{"ismobile", "iscrawler"} {
		if strings.Contains(text, name) {
			t.Errorf("the log entry %q names %s, which was answered", text, name)
		}
	}

	answered := newCredentialCloud()
	quiet := &logRecorder{}
	e, err = credentialBuilder(answered.serve(t), quiet).
		SetLicenseKey(testLicenseKey).
		SetRequestedProperties("device.ismobile", "device.iscrawler").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := processWith(t, pipelineOf(t, e, false), nil); err != nil {
		t.Fatalf("process: %v", err)
	}
	if records := quiet.all(); len(records) != 0 {
		t.Errorf("a fully answered request logged %q", recordText(records[0]))
	}
}

// Factory takes the License Key and the Requested Properties from the
// LicenseKey and RequestedProperties parameters, the list as comma separated
// text or as a list.
func TestFactoryTakesLicenseKeyAndRequestedProperties(t *testing.T) {
	for name, list := range map[string]any{
		"text": "device.ismobile, device.iscrawler",
		"list": []any{"device.ismobile", "device.iscrawler"},
	} {
		t.Run(name, func(t *testing.T) {
			cloud := newCredentialCloud()
			srv := cloud.serve(t)
			element, err := Factory(map[string]any{
				"EndPoint":            srv.URL + "/",
				"LicenseKey":          testLicenseKey,
				"RequestedProperties": list,
			}, slog.New(&logRecorder{}))
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			p, err := core.NewPipelineBuilder().AddFlowElement(element).Build()
			if err != nil {
				t.Fatalf("build the Pipeline: %v", err)
			}
			t.Cleanup(func() { p.Close() })
			if err := processWith(t, p, nil); err != nil {
				t.Fatalf("process: %v", err)
			}
			req, _ := cloud.last("json")
			if req.form.Get("license") != testLicenseKey ||
				req.form.Get("values") != "device.ismobile,device.iscrawler" ||
				req.form.Has("resource") {
				t.Errorf("the data request body is %v, want the License Key "+
					"and both Properties", req.form)
			}
		})
	}
}

// A License Key that a failed response's content holds across the point
// where the logged content is cut is still masked, so no part of it is
// shown in the error.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/features/logging.md#license-keys
func TestLicenseKeyAcrossTheCutIsMasked(t *testing.T) {
	cloud := newCredentialCloud()
	cloud.jsonStatus = http.StatusInternalServerError
	cloud.jsonBody = strings.Repeat("x", 512-6) +
		testLicenseKey + " and more"
	e, err := credentialBuilder(cloud.serve(t), &logRecorder{}).
		SetLicenseKey(testLicenseKey).
		SetRequestedProperties("device.ismobile").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	err = processWith(t, pipelineOf(t, e, false), nil)
	if err == nil {
		t.Fatal("a failed response gives no error")
	}
	if strings.Contains(err.Error(), testLicenseKey[:6]) {
		t.Errorf("the error %q shows part of the License Key", err)
	}
}

// Requested Properties given as one comma separated argument are the same
// list as separate arguments, so they are sent as values and none of them
// is reported as not covered when the answer holds them all.
func TestCommaSeparatedPropertiesAreSplit(t *testing.T) {
	cloud := newCredentialCloud()
	log := &logRecorder{}
	e, err := credentialBuilder(cloud.serve(t), log).
		SetLicenseKey(testLicenseKey).
		SetRequestedProperties("device.ismobile, device.iscrawler").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := processWith(t, pipelineOf(t, e, false),
		map[string]string{"header.user-agent": "ua"}); err != nil {
		t.Fatalf("process: %v", err)
	}
	req, _ := cloud.last("json")
	if got := req.form.Get("values"); got != "device.ismobile,device.iscrawler" {
		t.Errorf("values is %q, want device.ismobile,device.iscrawler", got)
	}
	for _, r := range log.all() {
		t.Errorf("logged %s %s, want nothing", r.Level, recordText(r))
	}
}
