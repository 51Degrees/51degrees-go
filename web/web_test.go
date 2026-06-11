package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/javascriptbuilder"
	"github.com/51Degrees/pipeline-go/elements/jsonbuilder"
	"github.com/51Degrees/pipeline-go/elements/sequence"
	"github.com/51Degrees/pipeline-go/elements/setheaders"
)

// headerEngine produces a SetHeader Property so the middleware has something to
// put in the response headers.
type headerEngine struct{ *core.BaseElement }

func newHeaderEngine() *headerEngine {
	e := &headerEngine{BaseElement: core.NewBaseElement("device", core.NewWhitelistFilter("header.user-agent"), nil)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: "SetHeaderBrowserAccept-CH", Element: e, Available: true},
	})
	return e
}

func (e *headerEngine) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("device", func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	d.Set("SetHeaderBrowserAccept-CH", "Sec-CH-UA,Sec-CH-UA-Mobile")
	return nil
}

func buildWebPipeline(t *testing.T) core.Pipeline {
	t.Helper()
	p, err := AssemblePipeline(PipelineConfig{}, newHeaderEngine())
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	// Verify required elements present and ordered.
	els := p.FlowElements()
	if els[0].DataKey() != sequence.DataKey {
		t.Fatalf("sequence must be first, got %s", els[0].DataKey())
	}
	if els[len(els)-1].DataKey() != javascriptbuilder.DataKey {
		t.Fatalf("javascript builder must be last, got %s", els[len(els)-1].DataKey())
	}
	if els[len(els)-2].DataKey() != jsonbuilder.DataKey {
		t.Fatalf("json builder must be penultimate, got %s", els[len(els)-2].DataKey())
	}
	return p
}

func TestMiddlewareProcessesAndSetsHeaders(t *testing.T) {
	p := buildWebPipeline(t)
	defer p.Close()
	mw := NewMiddleware(p)

	var sawFlowData bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := FromContext(r.Context()); ok {
			sawFlowData = true
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mw.Handler(next))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("User-Agent", "test-agent")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if !sawFlowData {
		t.Fatal("flow data not attached to request context")
	}
	if got := resp.Header.Get("Accept-CH"); !strings.Contains(got, "Sec-CH-UA") {
		t.Fatalf("Accept-CH response header not set, got %q", got)
	}
}

func TestJavaScriptEndpoint(t *testing.T) {
	p := buildWebPipeline(t)
	defer p.Close()
	mw := NewMiddleware(p)
	srv := httptest.NewServer(mw.Handler(http.NotFoundHandler()))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + DefaultJavaScriptURL)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-javascript" {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(resp.Header.Get("Cache-Control"), "private") {
		t.Fatalf("missing cache-control: %q", resp.Header.Get("Cache-Control"))
	}
	if resp.Header.Get("ETag") == "" {
		t.Fatal("missing ETag")
	}
	if !strings.Contains(string(body), "fiftyoneDegreesManager") {
		t.Fatalf("unexpected JS body")
	}
}

func TestJSONEndpointAndETag(t *testing.T) {
	p := buildWebPipeline(t)
	defer p.Close()
	mw := NewMiddleware(p)
	srv := httptest.NewServer(mw.Handler(http.NotFoundHandler()))
	defer srv.Close()

	// First request gets 200 + ETag.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+DefaultJSONURL, nil)
	req.Header.Set("User-Agent", "agent-1")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	etag := resp.Header.Get("ETag")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || etag == "" {
		t.Fatalf("first request: status=%d etag=%q", resp.StatusCode, etag)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}

	// Second request with matching If-None-Match and same evidence => 304.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+DefaultJSONURL, nil)
	req2.Header.Set("User-Agent", "agent-1")
	req2.Header.Set("If-None-Match", etag)
	resp2, err := srv.Client().Do(req2)
	if err != nil {
		t.Fatalf("request 2: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", resp2.StatusCode)
	}
}

func TestDisableClientSideElements(t *testing.T) {
	p, err := AssemblePipeline(PipelineConfig{
		DisableSequence:          true,
		DisableJSONBuilder:       true,
		DisableJavaScriptBuilder: true,
		DisableSetHeaders:        true,
	}, newHeaderEngine())
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	defer p.Close()
	if len(p.FlowElements()) != 1 {
		t.Fatalf("expected only the supplied element, got %d", len(p.FlowElements()))
	}
}

func TestNoDuplicateWhenPresent(t *testing.T) {
	// Supplying a Sequence Element should prevent a second being added.
	p, err := AssemblePipeline(PipelineConfig{}, sequence.New(nil), newHeaderEngine())
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	defer p.Close()
	count := 0
	for _, e := range p.FlowElements() {
		if e.DataKey() == sequence.DataKey {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one sequence element, got %d", count)
	}
	_ = setheaders.DataKey
}
