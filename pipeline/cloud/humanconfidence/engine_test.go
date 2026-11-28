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

package humanconfidence_test

// The tests the specification requires of a package that redeems a sealed
// Human Confidence result, against a fake of the cloud service's JSON call
// that answers as the service does.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#required-tests

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/humanconfidence"
	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/derivedproperty"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

const (
	testResource = "test-resource-key"
	testLicense  = "test-license-key-0123456789"
	// sealedHigh redeems to High once, and is replayed after that.
	sealedHigh = "sealed-High-AAAABBBBCCCCDDDD"
)

// fakeCloud stands in for the cloud service's JSON call. A sealed result
// named sealed-<Value>-... redeems to that Value once, and is replayed after
// that. sealed-expired-... and sealed-unconfirmed-... give those statuses,
// and any other sealed result is unreadable. Every answer to a call that
// holds a sealed result is 200, with HumanConfidence Medium on any status but
// success and HumanConfidenceSource Active. status and body, where set,
// replace the answer.
type fakeCloud struct {
	server   *httptest.Server
	requests atomic.Int64
	status   int
	body     string
	passive  string
	delay    time.Duration

	mu       sync.Mutex
	redeemed map[string]bool
	forms    []url.Values
}

func newFakeCloud(t *testing.T) *fakeCloud {
	c := &fakeCloud{
		redeemed: map[string]bool{},
		passive:  `{"derived":{"humanconfidence":"High","humanconfidencesource":"Passive"}}`,
	}
	c.server = httptest.NewServer(http.HandlerFunc(c.json))
	t.Cleanup(c.server.Close)
	return c
}

func (c *fakeCloud) json(w http.ResponseWriter, r *http.Request) {
	c.requests.Add(1)
	r.ParseForm()
	c.mu.Lock()
	c.forms = append(c.forms, r.PostForm)
	c.mu.Unlock()
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	w.Header().Set("Content-Type", "application/json")
	if c.status != 0 || c.body != "" {
		if c.status != 0 {
			w.WriteHeader(c.status)
		}
		io.WriteString(w, c.body)
		return
	}
	sealed := r.PostForm.Get("humanconfidence")
	if sealed == "" {
		io.WriteString(w, c.passive)
		return
	}
	io.WriteString(w, `{"derived":{"humanconfidence":"`+c.redeem(sealed)+
		`","humanconfidencesource":"Active"}}`)
}

// redeem gives the Value the service gives for sealed.
func (c *fakeCloud) redeem(sealed string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	parts := strings.SplitN(sealed, "-", 3)
	if len(parts) < 3 || parts[0] != "sealed" {
		return "Medium" // unreadable
	}
	switch parts[1] {
	case "High", "Medium", "Low":
		if c.redeemed[sealed] {
			return "Medium" // replayed
		}
		c.redeemed[sealed] = true
		return parts[1]
	}
	return "Medium" // expired, unconfirmed
}

// lastForm is the form of the last call.
func (c *fakeCloud) lastForm() url.Values {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.forms) == 0 {
		return nil
	}
	return c.forms[len(c.forms)-1]
}

// setup configures the Cloud Request Engine before it is built.
type setup func(*requestengine.Builder)

// build makes a Pipeline of a Cloud Request Engine and the Human Confidence
// Engine against c, logging to log.
func build(t *testing.T, c *fakeCloud, log io.Writer, setups ...setup) (core.Pipeline, *humanconfidence.Engine) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	b := requestengine.NewBuilder().
		SetLogger(logger).
		SetResourceKey(testResource).
		SetLicenseKey(testLicense).
		SetDataEndpoint(c.server.URL + "/json").
		SetHTTPClient(c.server.Client()).
		SetDiscoveryState(&requestengine.DiscoveryState{
			EvidenceKeys: []string{"header.user-agent", requestengine.HumanConfidenceEvidence},
			Products:     map[string]requestengine.ProductMeta{},
		})
	for _, s := range setups {
		s(b)
	}
	cre, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	engine := humanconfidence.New(logger)
	p, err := core.NewPipelineBuilder().SetLogger(logger).
		SetSuppressProcessExceptions(true).
		AddFlowElement(cre).AddFlowElement(engine).Build()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return p, engine
}

// process processes a request carrying sealed, where it is not empty.
func process(t *testing.T, p core.Pipeline, sealed string, evidence ...string) core.FlowData {
	t.Helper()
	fd := p.CreateFlowData()
	if sealed != "" {
		fd.AddEvidence("query.humanconfidence", sealed)
	}
	for i := 0; i+1 < len(evidence); i += 2 {
		fd.AddEvidence(evidence[i], evidence[i+1])
	}
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	return fd
}

// read reads a Property of the derived Element Data, failing the test where
// the read fails.
func read(t *testing.T, fd core.FlowData, name string) engines.AspectPropertyValue[string] {
	t.Helper()
	data, err := core.Get[*derivedproperty.Data](fd)
	if err != nil {
		t.Fatalf("reading derived: %v", err)
	}
	value, err := derivedproperty.GetAs[engines.AspectPropertyValue[string]](data, name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return value
}

// want checks both Properties, and that the engine added no error.
func want(t *testing.T, fd core.FlowData, engine core.FlowElement, level, source string) {
	t.Helper()
	if got, _ := read(t, fd, humanconfidence.HumanConfidence).Value(); got != level {
		t.Errorf("HumanConfidence is %q, want %q", got, level)
	}
	if got, _ := read(t, fd, humanconfidence.HumanConfidenceSource).Value(); got != source {
		t.Errorf("HumanConfidenceSource is %q, want %q", got, source)
	}
	for _, fe := range fd.Errors() {
		if fe.Element == engine {
			t.Errorf("the engine added an error: %v", fe.Err)
		}
	}
}

func TestASealedResultThatRedeemsGivesItsValue(t *testing.T) {
	for _, level := range []string{"High", "Medium", "Low"} {
		c := newFakeCloud(t)
		p, engine := build(t, c, io.Discard)
		fd := process(t, p, "sealed-"+level+"-x")
		want(t, fd, engine, level, humanconfidence.Active)
		if got := c.lastForm().Get("humanconfidence"); got != "sealed-"+level+"-x" {
			t.Errorf("the call held %q, want the sealed result as it arrived", got)
		}
		if got := c.lastForm().Get("license"); got != testLicense {
			t.Error("the call did not hold the configured License Key")
		}
	}
}

// The service answers a sealed result it does not redeem with 200 and
// Medium, for each status that is not success.
func TestEachStatusButSuccessGivesMedium(t *testing.T) {
	c := newFakeCloud(t)
	p, engine := build(t, c, io.Discard)
	process(t, p, sealedHigh)
	for name, sealed := range map[string]string{
		"replayed":    sealedHigh,
		"expired":     "sealed-expired-x",
		"unconfirmed": "sealed-unconfirmed-x",
		"unreadable":  "not a sealed result",
	} {
		t.Run(name, func(t *testing.T) {
			want(t, process(t, p, sealed), engine, "Medium", humanconfidence.Active)
		})
	}
}

// Each case where the sealed result is not redeemed gives Medium and Active,
// and the read adds no error of its own.
func TestEachCaseNotRedeemedGivesMediumAndActive(t *testing.T) {
	answers := map[string]struct {
		status int
		body   string
	}{
		"the request failed":                 {http.StatusInternalServerError, `{"error":"broken"}`},
		"the service said HC is not covered": {http.StatusOK, `{"errors":["HumanConfidence is not covered by the License Key"]}`},
		"no derived member":                  {http.StatusOK, `{"device":{"ismobile":true}}`},
		"derived holds no humanconfidence":   {http.StatusOK, `{"derived":{"humanconfidencesource":"Active"}}`},
		"humanconfidence is null":            {http.StatusOK, `{"derived":{"humanconfidence":null}}`},
		"humanconfidence is Unknown":         {http.StatusOK, `{"derived":{"humanconfidence":"Unknown"}}`},
		"humanconfidence is another word":    {http.StatusOK, `{"derived":{"humanconfidence":"high"}}`},
		"humanconfidence is not text":        {http.StatusOK, `{"derived":{"humanconfidence":3}}`},
		"the answer is not JSON":             {http.StatusOK, `High`},
	}
	for name, answer := range answers {
		t.Run(name, func(t *testing.T) {
			c := newFakeCloud(t)
			c.status, c.body = answer.status, answer.body
			p, engine := build(t, c, io.Discard)
			want(t, process(t, p, sealedHigh), engine, "Medium", humanconfidence.Active)
		})
	}

	t.Run("the request did not complete in time", func(t *testing.T) {
		c := newFakeCloud(t)
		c.delay = 300 * time.Millisecond
		p, engine := build(t, c, io.Discard, func(b *requestengine.Builder) {
			b.SetHTTPClient(&http.Client{Timeout: 50 * time.Millisecond})
		})
		want(t, process(t, p, sealedHigh), engine, "Medium", humanconfidence.Active)
	})

	t.Run("the service could not be reached", func(t *testing.T) {
		c := newFakeCloud(t)
		c.server.Close()
		p, engine := build(t, c, io.Discard)
		want(t, process(t, p, sealedHigh), engine, "Medium", humanconfidence.Active)
	})

	t.Run("no request was made, in a recovery period", func(t *testing.T) {
		c := newFakeCloud(t)
		c.status, c.body = http.StatusServiceUnavailable, `{"error":"down"}`
		p, engine := build(t, c, io.Discard, func(b *requestengine.Builder) {
			b.SetFailuresToEnterRecovery(1).SetRecoveryDuration(time.Hour)
		})
		process(t, p, sealedHigh)
		c.status, c.body = 0, ""
		before := c.requests.Load()
		fd := process(t, p, "sealed-Low-y")
		if c.requests.Load() != before {
			t.Fatal("a request was made in the recovery period")
		}
		want(t, fd, engine, "Medium", humanconfidence.Active)
	})

	t.Run("the Cloud Request Engine has no License Key", func(t *testing.T) {
		c := newFakeCloud(t)
		p, engine := build(t, c, io.Discard, func(b *requestengine.Builder) {
			b.SetLicenseKey("")
		})
		want(t, process(t, p, sealedHigh), engine, "Medium", humanconfidence.Active)
	})

	t.Run("the Pipeline has no Cloud Request Engine", func(t *testing.T) {
		engine := humanconfidence.New(nil)
		p, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()
		want(t, process(t, p, sealedHigh), engine, "Medium", humanconfidence.Active)
	})
}

// A Derived Property Element whose script gives HumanConfidence from the
// request runs before the engine, and the sealed result's Value replaces it.
// Without a sealed result, the script's value is kept.
func TestASealedResultReplacesAValueWorkedOutElsewhere(t *testing.T) {
	c := newFakeCloud(t)
	c.passive = `{"derived":{"humanconfidence":"Low","humanconfidencesource":"Passive"}}`
	cre, err := requestengine.NewBuilder().
		SetResourceKey(testResource).SetLicenseKey(testLicense).
		SetDataEndpoint(c.server.URL + "/json").SetHTTPClient(c.server.Client()).
		SetDiscoveryState(&requestengine.DiscoveryState{EvidenceKeys: []string{}, Products: map[string]requestengine.ProductMeta{}}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	script := &setsHigh{BaseElement: core.NewBaseElement("script", nil, nil)}
	engine := humanconfidence.New(nil)
	p, err := core.NewPipelineBuilder().AddFlowElement(cre).
		AddFlowElement(script).AddFlowElement(engine).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	want(t, process(t, p, "sealed-Low-z"), engine, "Low", humanconfidence.Active)
	want(t, process(t, p, "sealed-Low-z"), engine, "Medium", humanconfidence.Active)
	if got, _ := read(t, process(t, p, ""), humanconfidence.HumanConfidence).Value(); got != "High" {
		t.Errorf("with no sealed result HumanConfidence is %q, want the value given before", got)
	}
}

// setsHigh writes HumanConfidence High into the derived Element Data, as a
// Derived Property Element's script does.
type setsHigh struct{ *core.BaseElement }

func (s *setsHigh) Process(fd core.FlowData) error {
	data := fd.GetOrAdd(derivedproperty.DataKey,
		func(core.FlowData) core.ElementData { return derivedproperty.NewData() })
	data.(*derivedproperty.Data).Set("HumanConfidence", engines.NewAspectPropertyValue("High"))
	return nil
}

func TestTwoReadsMakeOneRequest(t *testing.T) {
	c := newFakeCloud(t)
	p, _ := build(t, c, io.Discard)
	fd := process(t, p, sealedHigh)
	first, _ := read(t, fd, humanconfidence.HumanConfidence).Value()
	second, _ := read(t, fd, humanconfidence.HumanConfidence).Value()
	if first != "High" || second != "High" {
		t.Errorf("the reads gave %q and %q", first, second)
	}
	if n := c.requests.Load(); n != 1 {
		t.Errorf("%d requests were made, want 1", n)
	}
}

// Two requests with the same sealed result each go to the service, so the
// second is not served the first one's Value from a cache.
func TestASealedRequestIsNotCached(t *testing.T) {
	c := newFakeCloud(t)
	p, engine := build(t, c, io.Discard)
	want(t, process(t, p, sealedHigh), engine, "High", humanconfidence.Active)
	want(t, process(t, p, sealedHigh), engine, "Medium", humanconfidence.Active)
	if n := c.requests.Load(); n != 2 {
		t.Errorf("%d requests were made, want 2", n)
	}
}

// The package keeps no transaction yet, so the call holds none, and none
// from the request.
func TestTheCallHoldsNoTransactionFromTheRequest(t *testing.T) {
	c := newFakeCloud(t)
	p, _ := build(t, c, io.Discard)
	process(t, p, sealedHigh, "query.transaction", "from-the-page",
		"cookie.Challenge", "from-a-cookie")
	for field := range c.lastForm() {
		if lower := strings.ToLower(field); lower == "transaction" || lower == "challenge" {
			t.Errorf("the call holds %s", field)
		}
	}
}

// No log entry and no error message holds the sealed result or the License
// Key, whether the request works or fails.
func TestNoLogOrErrorHoldsTheSealedResultOrTheLicenseKey(t *testing.T) {
	for name, fail := range map[string]func(*fakeCloud){
		"redeemed":    func(*fakeCloud) {},
		"status 500":  func(c *fakeCloud) { c.status, c.body = 500, `{"error":"broken"}` },
		"errors list": func(c *fakeCloud) { c.body = `{"errors":["refused"]}` },
		"unreachable": func(c *fakeCloud) { c.server.Close() },
	} {
		t.Run(name, func(t *testing.T) {
			c := newFakeCloud(t)
			var log bytes.Buffer
			p, _ := build(t, c, &log)
			fail(c)
			fd := process(t, p, sealedHigh)
			read(t, fd, humanconfidence.HumanConfidence)
			var text strings.Builder
			text.Write(log.Bytes())
			for _, fe := range fd.Errors() {
				text.WriteString(fe.Err.Error())
			}
			for _, secret := range []string{sealedHigh, "AAAABBBB", testLicense, "0123456789"} {
				if strings.Contains(text.String(), secret) {
					t.Errorf("a log entry or error holds %q:\n%s", secret, text.String())
				}
			}
		})
	}
}

// Without a sealed result the two Properties are the answer's, and a
// credential that does not carry one gives no value and the service's reason.
func TestWithNoSealedResultThePropertiesAreTheAnswers(t *testing.T) {
	c := newFakeCloud(t)
	p, engine := build(t, c, io.Discard)
	want(t, process(t, p, ""), engine, "High", "Passive")
	if form := c.lastForm(); form.Has("humanconfidence") {
		t.Error("a request with no sealed result sent one")
	}

	c.passive = `{"derived":{"humanconfidence":null,` +
		`"humanconfidencenullreason":"Not in this key.",` +
		`"humanconfidencesource":null,` +
		`"humanconfidencesourcenullreason":"Not in this key either."}}`
	fd := process(t, p, "")
	for name, reason := range map[string]string{
		humanconfidence.HumanConfidence:       "Not in this key.",
		humanconfidence.HumanConfidenceSource: "Not in this key either.",
	} {
		value := read(t, fd, name)
		if value.HasValue() || value.NoValueMessage() != reason {
			t.Errorf("%s has a value %v, or the reason %q, want no value and %q",
				name, value.HasValue(), value.NoValueMessage(), reason)
		}
	}
}

// A Property the answer leaves out is not given, so it reads as missing.
func TestAPropertyTheAnswerLeavesOutIsNotGiven(t *testing.T) {
	c := newFakeCloud(t)
	c.passive = `{"derived":{"humanconfidence":"Medium"}}`
	p, _ := build(t, c, io.Discard)
	fd := process(t, p, "")
	data, err := core.Get[*derivedproperty.Data](fd)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := data.Get(humanconfidence.HumanConfidenceSource); err == nil {
		t.Error("HumanConfidenceSource was given though the answer left it out")
	}
}

func TestTheEngineMustFollowTheCloudRequestEngine(t *testing.T) {
	c := newFakeCloud(t)
	cre, err := requestengine.NewBuilder().
		SetResourceKey(testResource).SetDataEndpoint(c.server.URL + "/json").
		SetHTTPClient(c.server.Client()).
		SetDiscoveryState(&requestengine.DiscoveryState{EvidenceKeys: []string{}, Products: map[string]requestengine.ProductMeta{}}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	_, err = core.NewPipelineBuilder().
		AddFlowElement(humanconfidence.New(nil)).AddFlowElement(cre).Build()
	if err == nil {
		t.Error("a Pipeline with the engine before the Cloud Request Engine was built")
	}
}

// The engine names the Evidence it reads, so a cache key or an ETag made from
// the Pipeline's filter changes with the sealed result.
func TestTheEngineNamesTheSealedResultEvidence(t *testing.T) {
	if !humanconfidence.New(nil).EvidenceKeyFilter().Include("query.humanconfidence") {
		t.Error("the filter leaves out query.humanconfidence")
	}
}
