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
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/missingproperty"
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
// replace the answer. With stall set, the call answers nothing until the
// client gives up on it, or until the test releases it.
type fakeCloud struct {
	server   *httptest.Server
	requests atomic.Int64
	status   int
	body     string
	passive  string
	stall    bool
	stop     chan struct{}
	release  func()

	mu       sync.Mutex
	redeemed map[string]bool
	forms    []url.Values
	// madeWith is the transaction a sealed result was made with, where a
	// test names one. Such a sealed result redeems only with it.
	madeWith map[string]string
}

func newFakeCloud(t *testing.T) *fakeCloud {
	c := &fakeCloud{
		redeemed: map[string]bool{},
		madeWith: map[string]string{},
		passive:  `{"derived":{"humanconfidence":"High","humanconfidencesource":"Passive"}}`,
		stop:     make(chan struct{}),
	}
	var once sync.Once
	c.release = func() { once.Do(func() { close(c.stop) }) }
	c.server = httptest.NewServer(http.HandlerFunc(c.json))
	t.Cleanup(c.server.Close)
	// Cleanups run last first, so a stalled call is released before the
	// server waits for it to end.
	t.Cleanup(c.release)
	return c
}

func (c *fakeCloud) json(w http.ResponseWriter, r *http.Request) {
	// Discovery is answered as the service does, unless the test sets the
	// answer to every call.
	answered := c.status != 0 || c.body != ""
	switch {
	case answered:
	case strings.HasSuffix(r.URL.Path, "/accessibleproperties"):
		io.WriteString(w, `{"Products":{"device":{"DataTier":"CloudV4",`+
			`"Properties":[{"Name":"ismobile","Type":"Bool","Category":"Device"}]}}}`)
		return
	case strings.HasSuffix(r.URL.Path, "/evidencekeys"):
		io.WriteString(w, `["header.user-agent","`+requestengine.HumanConfidenceEvidence+`"]`)
		return
	}
	c.requests.Add(1)
	r.ParseForm()
	c.mu.Lock()
	c.forms = append(c.forms, r.PostForm)
	c.mu.Unlock()
	if c.stall {
		select {
		case <-r.Context().Done():
			// The client gave up, so there is no one to answer.
			return
		case <-c.stop:
			// Released by the test, so answer as the service would.
		}
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
	io.WriteString(w, `{"derived":{"humanconfidence":"`+c.redeem(sealed, r.PostForm.Get("transaction"))+
		`","humanconfidencesource":"Active"}}`)
}

// redeem gives the Value the service gives for sealed, sent with
// transaction.
func (c *fakeCloud) redeem(sealed, transaction string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	parts := strings.SplitN(sealed, "-", 3)
	if want, named := c.madeWith[sealed]; len(parts) < 3 || parts[0] != "sealed" ||
		(named && want != transaction) {
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

// process processes a request carrying sealed, where it is not empty. A
// sealed result comes from a page the Pipeline's Cloud Request Engine
// issued, unless evidence names the page.
func process(t *testing.T, p core.Pipeline, sealed string, evidence ...string) core.FlowData {
	t.Helper()
	fd := p.CreateFlowData()
	if sealed != "" {
		fd.AddEvidence("query.humanconfidence", sealed)
		if cre, ok := p.GetElement(requestengine.DataKey).(*requestengine.CloudRequestEngine); ok {
			fd.AddEvidence(requestengine.HumanConfidencePageEvidence, cre.IssueHumanConfidencePage().ID)
		}
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
		"the service said HC is not covered": {http.StatusOK, `{"errors":["HumanConfidence is not covered by the License Key"],"derived":{"humanconfidence":"High","humanconfidencesource":"Active"}}`},
		"no derived member":                  {http.StatusOK, `{"device":{"ismobile":true}}`},
		"derived holds no humanconfidence":   {http.StatusOK, `{"derived":{"humanconfidencesource":"Active"}}`},
		"humanconfidence is null":            {http.StatusOK, `{"derived":{"humanconfidence":null}}`},
		"humanconfidence is Unknown":         {http.StatusOK, `{"derived":{"humanconfidence":"Unknown"}}`},
		"humanconfidence is another word":    {http.StatusOK, `{"derived":{"humanconfidence":"high"}}`},
		"humanconfidence is not text":        {http.StatusOK, `{"derived":{"humanconfidence":3}}`},
		"humanconfidence is empty":           {http.StatusOK, `{"derived":{"humanconfidence":""}}`},
		"humanconfidence is a sealed result": {http.StatusOK, `{"derived":{"humanconfidence":"` + sealedHigh + `"}}`},
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

	// The call never answers before the client gives up, and the only limit
	// on the client is the engine's TimeoutSeconds, so the request can only
	// end where that setting applies. It is longer than the engine's default,
	// and a timer never fires early, so ending sooner means the setting was
	// not used.
	t.Run("the request did not complete in time", func(t *testing.T) {
		const timeoutSeconds = 3
		if requestengine.DefaultTimeout >= timeoutSeconds*time.Second {
			t.Fatalf("the test's timeout must be longer than the default %v", requestengine.DefaultTimeout)
		}
		c := newFakeCloud(t)
		c.stall = true
		quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
		element, err := requestengine.Factory(map[string]any{
			"ResourceKey":    testResource,
			"LicenseKey":     testLicense,
			"EndPoint":       c.server.URL + "/",
			"TimeoutSeconds": timeoutSeconds,
		}, quiet)
		if err != nil {
			t.Fatal(err)
		}
		cre := element.(*requestengine.CloudRequestEngine)
		engine := humanconfidence.New(quiet)
		p, err := core.NewPipelineBuilder().SetLogger(quiet).SetSuppressProcessExceptions(true).
			AddFlowElement(cre).AddFlowElement(engine).Build()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { p.Close() })
		fd := p.CreateFlowData()
		fd.AddEvidence("query.humanconfidence", sealedHigh)
		fd.AddEvidence(requestengine.HumanConfidencePageEvidence, cre.IssueHumanConfidencePage().ID)
		done := make(chan error, 1)
		start := time.Now()
		go func() { done <- fd.Process() }()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			if took := time.Since(start); took < timeoutSeconds*time.Second {
				t.Errorf("the request ended after %v, before TimeoutSeconds of %d", took, timeoutSeconds)
			}
		case <-time.After(30 * time.Second):
			// The client did not give up, so TimeoutSeconds did not apply.
			// Let the call answer, as a late answer would, and read what
			// the engine gives.
			c.release()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			t.Errorf("the request was not ended by TimeoutSeconds of %d", timeoutSeconds)
		}
		want(t, fd, engine, "Medium", humanconfidence.Active)
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

	t.Run("no request was made, as discovery failed", func(t *testing.T) {
		c := newFakeCloud(t)
		c.status, c.body = http.StatusServiceUnavailable, `{"error":"down"}`
		p, engine := build(t, c, io.Discard, func(b *requestengine.Builder) {
			b.SetDiscoveryState(nil).SetEndpoint(c.server.URL + "/")
		})
		fd := process(t, p, sealedHigh)
		want(t, fd, engine, "Medium", humanconfidence.Active)
		if len(fd.Errors()) == 0 {
			t.Error("the failure was not recorded as the Cloud Request Engine's error")
		}
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

// Two requests with the same Evidence, sealed result and page id included,
// each go to the service, even with a cache set on the Cloud Request Engine,
// so the second is not served the first one's redeemed Value. The page's
// transaction was used by the first, so the second gives Medium.
func TestASealedRequestIsNotCached(t *testing.T) {
	c := newFakeCloud(t)
	p, engine := build(t, c, io.Discard)
	cre := p.GetElement(requestengine.DataKey).(*requestengine.CloudRequestEngine)
	cre.SetCache(cache.NewLRUCache(100, 1))
	id, transaction := issuePage(t, p)
	c.madeWith[sealedHigh] = transaction
	for i, level := range []string{"High", "Medium"} {
		fd := process(t, p, sealedHigh, "query.humanconfidencepage", id, "header.user-agent", "the same")
		want(t, fd, engine, level, humanconfidence.Active)
		if data, err := fd.Get(requestengine.DataKey); err == nil {
			data.(*requestengine.CloudRequestData).Response()
		}
		if n := c.requests.Load(); n != int64(i+1) {
			t.Errorf("%d requests were made, want %d", n, i+1)
		}
	}
}

// issuePage issues a page from the Pipeline's Cloud Request Engine, and gives
// its id and the transaction its script address carries.
func issuePage(t *testing.T, p core.Pipeline) (string, string) {
	t.Helper()
	cre := p.GetElement(requestengine.DataKey).(*requestengine.CloudRequestEngine)
	page := cre.IssueHumanConfidencePage()
	u, err := url.Parse(page.ScriptURL("https://cloud.example.com/api/v4/", testResource))
	if err != nil {
		t.Fatal(err)
	}
	return page.ID, u.Query().Get("transaction")
}

// The call holds one transaction, the one the package kept for the page,
// where the request's Evidence also holds a transaction and a challenge, and
// the sealed result made with it redeems.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#required-tests
func TestTheCallHoldsOnlyTheKeptTransaction(t *testing.T) {
	c := newFakeCloud(t)
	p, engine := build(t, c, io.Discard)
	id, transaction := issuePage(t, p)
	c.madeWith["sealed-High-a"] = transaction
	fd := process(t, p, "sealed-High-a", "query.humanconfidencepage", id,
		"query.transaction", "from-the-page", "cookie.Challenge", "from-a-cookie")
	want(t, fd, engine, "High", humanconfidence.Active)
	form := c.lastForm()
	if got := form["transaction"]; len(got) != 1 || got[0] != transaction {
		t.Errorf("the call held transaction %v, want only the kept %q", got, transaction)
	}
	for field := range form {
		if strings.EqualFold(field, "challenge") || strings.EqualFold(field, "humanconfidencepage") {
			t.Errorf("the call held %s", field)
		}
	}
}

// Each page redeems with its own transaction, so two pages open at once both
// redeem, and a sealed result never redeems for another page.
func TestTwoPagesEachRedeem(t *testing.T) {
	c := newFakeCloud(t)
	p, engine := build(t, c, io.Discard)
	first, firstTransaction := issuePage(t, p)
	second, secondTransaction := issuePage(t, p)
	c.madeWith["sealed-High-1"] = firstTransaction
	c.madeWith["sealed-Low-2"] = secondTransaction
	c.madeWith["sealed-Low-x"] = firstTransaction
	want(t, process(t, p, "sealed-Low-2", "query.humanconfidencepage", second),
		engine, "Low", humanconfidence.Active)
	want(t, process(t, p, "sealed-High-1", "query.humanconfidencepage", first),
		engine, "High", humanconfidence.Active)
	third, _ := issuePage(t, p)
	want(t, process(t, p, "sealed-Low-x", "query.humanconfidencepage", third),
		engine, "Medium", humanconfidence.Active)
}

// With no transaction kept for the page, the sealed result gives Medium and
// Active, and is not sent: with no page id, a page never issued, or a page
// whose transaction was used.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#where-the-sealed-result-is-not-redeemed
func TestNoKeptTransactionGivesMediumAndSendsNothing(t *testing.T) {
	c := newFakeCloud(t)
	p, engine := build(t, c, io.Discard)
	id, transaction := issuePage(t, p)
	c.madeWith["sealed-High-u"] = transaction
	c.madeWith["sealed-Low-u"] = transaction
	want(t, process(t, p, "sealed-High-u", "query.humanconfidencepage", id),
		engine, "High", humanconfidence.Active)
	for name, evidence := range map[string][]string{
		"the transaction was used": {"query.humanconfidencepage", id},
		"a page never issued":      {"query.humanconfidencepage", "never-issued"},
		"no page id":               {"query.humanconfidencepage", ""},
	} {
		t.Run(name, func(t *testing.T) {
			fd := p.CreateFlowData()
			fd.AddEvidence("query.humanconfidence", "sealed-Low-u")
			fd.AddEvidence(evidence[0], evidence[1])
			if err := fd.Process(); err != nil {
				t.Fatal(err)
			}
			want(t, fd, engine, "Medium", humanconfidence.Active)
			// The request runs on its own, so wait for it before looking at
			// what it sent.
			if data, err := fd.Get(requestengine.DataKey); err == nil {
				data.(*requestengine.CloudRequestData).Response()
			}
			if form := c.lastForm(); form.Has("humanconfidence") || form.Has("transaction") {
				t.Errorf("the call held the sealed result or a transaction, %v", form)
			}
		})
	}
}

// No log entry and no error message holds the transaction, in whole or in
// part, where the cloud service writes it in a failed answer.
func TestNoLogOrErrorHoldsTheTransaction(t *testing.T) {
	c := newFakeCloud(t)
	var log bytes.Buffer
	p, _ := build(t, c, &log)
	id, transaction := issuePage(t, p)
	c.status, c.body = http.StatusInternalServerError, `{"echo":"`+transaction+` `+sealedHigh+`"}`
	fd := p.CreateFlowData()
	fd.AddEvidence("query.humanconfidence", sealedHigh)
	fd.AddEvidence("query.humanconfidencepage", id)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	read(t, fd, humanconfidence.HumanConfidence)
	text := log.String()
	for _, fe := range fd.Errors() {
		text += fe.Err.Error()
	}
	for _, secret := range []string{transaction, transaction[len(transaction)-6:], sealedHigh, "AAAABBBB"} {
		if strings.Contains(text, secret) {
			t.Errorf("a log entry or error holds %q:\n%s", secret, text)
		}
	}
}

// No log entry and no error message holds the sealed result or the License
// Key, whether the request works or fails.
func TestNoLogOrErrorHoldsTheSealedResultOrTheLicenseKey(t *testing.T) {
	for name, fail := range map[string]func(*fakeCloud){
		"redeemed": func(*fakeCloud) {},
		"status 500": func(c *fakeCloud) {
			c.status, c.body = 500, `{"error":"broken `+sealedHigh+` `+testLicense+`"}`
		},
		"errors list": func(c *fakeCloud) {
			c.body = `{"errors":["refused ` + sealedHigh + ` for ` + testLicense + `"],"warnings":["` + testLicense + `"]}`
		},
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

// A Property the answer leaves out, as for a credential that does not carry
// it, reads as no value with the reason Missing Properties gives, and so does
// one whose request failed.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#what-a-consumer-reads
func TestAPropertyTheAnswerLeavesOutGivesTheMissingReason(t *testing.T) {
	c := newFakeCloud(t)
	c.passive = `{"derived":{"humanconfidence":"Medium"},"device":{}}`
	p, _ := build(t, c, io.Discard)
	value := read(t, process(t, p, ""), humanconfidence.HumanConfidenceSource)
	reason := missingproperty.ForPropertyNotInResourceKey("HumanConfidenceSource",
		"derived", "derived", nil).Error()
	if value.HasValue() || value.NoValueMessage() != reason {
		t.Errorf("a Property left out gives %v, %q, want no value and %q",
			value.HasValue(), value.NoValueMessage(), reason)
	}

	c.passive = `{"device":{}}`
	value = read(t, process(t, p, ""), humanconfidence.HumanConfidence)
	reason = missingproperty.ForPropertyNotInResourceKey("HumanConfidence",
		"derived", "derived", nil).Error()
	if value.HasValue() || value.NoValueMessage() != reason {
		t.Errorf("an answer with no derived member gives %q, want %q", value.NoValueMessage(), reason)
	}

	c.status, c.body = http.StatusInternalServerError, `{"error":"broken"}`
	value = read(t, process(t, p, ""), humanconfidence.HumanConfidence)
	reason = missingproperty.ForCloudRequestFailed("HumanConfidence", "derived").Error()
	if value.HasValue() || value.NoValueMessage() != reason {
		t.Errorf("a failed request gives %q, want %q", value.NoValueMessage(), reason)
	}
}

// Without a sealed result, HumanConfidence is the answer's only where it is
// one of the Property's values, High, Medium, Low or Unknown, as given. Any
// other text, a sealed result included, gives no value with the reason Missing
// Properties gives, and no error.
func TestWithNoSealedResultOnlyAValueOfThePropertyIsGiven(t *testing.T) {
	missing := missingproperty.ForPropertyNotInResourceKey("HumanConfidence",
		"derived", "derived", nil).Error()
	for name, raw := range map[string]string{
		"a sealed result": `"` + sealedHigh + `"`,
		"lower case":      `"high"`,
		"empty":           `""`,
		"a number":        `3`,
	} {
		t.Run(name, func(t *testing.T) {
			c := newFakeCloud(t)
			c.passive = `{"derived":{"humanconfidence":` + raw + `,"humanconfidencesource":"Passive"}}`
			p, engine := build(t, c, io.Discard)
			fd := process(t, p, "")
			value := read(t, fd, humanconfidence.HumanConfidence)
			if value.HasValue() || value.NoValueMessage() != missing {
				got, _ := value.Value()
				t.Errorf("HumanConfidence is %q, has a value %v, reason %q, want no value and %q",
					got, value.HasValue(), value.NoValueMessage(), missing)
			}
			for _, fe := range fd.Errors() {
				if fe.Element == engine {
					t.Errorf("the engine added an error: %v", fe.Err)
				}
			}
		})
	}
	for _, level := range []string{"High", "Medium", "Low", "Unknown"} {
		t.Run(level, func(t *testing.T) {
			c := newFakeCloud(t)
			c.passive = `{"derived":{"humanconfidence":"` + level + `","humanconfidencesource":"Passive"}}`
			p, engine := build(t, c, io.Discard)
			want(t, process(t, p, ""), engine, level, "Passive")
		})
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
	filter := humanconfidence.New(nil).EvidenceKeyFilter()
	for _, key := range []string{"query.humanconfidence", "query.humanconfidencepage"} {
		if !filter.Include(key) {
			t.Errorf("the filter leaves out %s, so a Pipeline that filters Evidence loses it", key)
		}
	}
}

// A value named humanconfidence anywhere but query.humanconfidence is not a
// sealed result. It isn't sent, so it can't come back redeemed, and the two
// Properties are those of a request with no sealed result.
func TestAHumanConfidenceValueUnderAnotherKeyIsNotRedeemed(t *testing.T) {
	for _, key := range []string{"cookie.humanconfidence", "header.HumanConfidence", "humanconfidence"} {
		c := newFakeCloud(t)
		p, engine := build(t, c, io.Discard)
		fd := process(t, p, "", key, sealedHigh, "query.transaction", "from-the-page")
		want(t, fd, engine, "High", "Passive")
		if form := c.lastForm(); form.Has("humanconfidence") || form.Has("transaction") {
			t.Errorf("%s: the call held %v", key, form)
		}
	}
}

// A Pipeline with no Cloud Request Engine builds. A request that carries a
// sealed result reads Medium and Active, with no error and no request, and
// any other request reads no value for either Property, with a reason.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#where-the-sealed-result-is-not-redeemed
func TestWithNoCloudRequestEngine(t *testing.T) {
	engine := humanconfidence.New(nil)
	p, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		t.Fatalf("a Pipeline with no Cloud Request Engine did not build: %v", err)
	}
	defer p.Close()

	fd := process(t, p, "made-up-sealed-value")
	want(t, fd, engine, "Medium", humanconfidence.Active)
	if len(fd.Errors()) != 0 {
		t.Errorf("a sealed request gave errors %v", fd.Errors())
	}

	fd = process(t, p, "")
	if len(fd.Errors()) != 0 {
		t.Errorf("a request with no sealed result gave errors %v", fd.Errors())
	}
	for _, name := range []string{humanconfidence.HumanConfidence, humanconfidence.HumanConfidenceSource} {
		value := read(t, fd, name)
		if value.HasValue() || value.NoValueMessage() == "" {
			t.Errorf("%s has a value %v, or no reason, want no value and a reason", name, value.HasValue())
		}
	}
}
