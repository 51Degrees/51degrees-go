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

package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/javascriptbuilder"
	"github.com/51Degrees/pipeline-go/elements/jsonbuilder"
	"github.com/51Degrees/pipeline-go/elements/sequence"
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
	// The callback endpoint must be baked into the script so the client-side
	// round-trip works (the JSON interception path).
	if !strings.Contains(string(body), DefaultJSONURL) {
		t.Fatalf("served JavaScript is missing the callback URL %q", DefaultJSONURL)
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

func TestCacheControlFormat(t *testing.T) {
	p := buildWebPipeline(t)
	defer p.Close()
	mw := NewMiddleware(p, WithMaxAge(30*time.Minute))
	srv := httptest.NewServer(mw.Handler(http.NotFoundHandler()))
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + DefaultJavaScriptURL)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	// Directives must be comma-separated to be a valid list.
	if got := resp.Header.Get("Cache-Control"); got != "private, max-age=1800" {
		t.Fatalf("Cache-Control = %q, want %q", got, "private, max-age=1800")
	}
}

func TestClientIPDefaultsToPeer(t *testing.T) {
	// With no trusted proxies configured, a client-supplied X-Forwarded-For is
	// ignored and the connection peer address is used.
	mw := NewMiddleware(nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.5:5555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := mw.clientIP(r); got != "203.0.113.5" {
		t.Fatalf("clientIP = %q, want the peer 203.0.113.5", got)
	}
}

func TestClientIPTrustedProxyWalk(t *testing.T) {
	// With the peer in a trusted range, X-Forwarded-For is walked from the
	// right, skipping trusted hops, to the client as seen by the trusted edge.
	mw := NewMiddleware(nil, WithTrustedProxies("10.0.0.0/8"))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.9")
	if got := mw.clientIP(r); got != "1.2.3.4" {
		t.Fatalf("clientIP = %q, want 1.2.3.4", got)
	}

	// An untrusted peer means the header is ignored even if present.
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.RemoteAddr = "198.51.100.7:2222"
	r2.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := mw.clientIP(r2); got != "198.51.100.7" {
		t.Fatalf("clientIP = %q, want the peer 198.51.100.7", got)
	}
}

func TestClientIPIPv6Peer(t *testing.T) {
	mw := NewMiddleware(nil)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "[2001:db8::1]:443"
	if got := mw.clientIP(r); got != "2001:db8::1" {
		t.Fatalf("clientIP = %q, want 2001:db8::1", got)
	}
}

func TestBodyReadableAfterMiddleware(t *testing.T) {
	p := buildWebPipeline(t)
	defer p.Close()
	mw := NewMiddleware(p)

	var gotBody string
	var gotEvidence any
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if fd, ok := FromContext(r.Context()); ok {
			gotEvidence, _ = fd.Evidence().Get("query.foo")
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mw.Handler(next))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader("foo=bar"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	// The downstream handler must still see the full body...
	if gotBody != "foo=bar" {
		t.Fatalf("downstream body = %q, want %q (middleware consumed it)", gotBody, "foo=bar")
	}
	// ...and the form parameter must have been exposed as query evidence.
	if gotEvidence != "bar" {
		t.Fatalf("query.foo evidence = %v, want bar", gotEvidence)
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
}

func TestCORSOriginReflected(t *testing.T) {
	p := buildWebPipeline(t)
	defer p.Close()
	mw := NewMiddleware(p)
	srv := httptest.NewServer(mw.Handler(http.NotFoundHandler()))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+DefaultJavaScriptURL, nil)
	req.Header.Set("Origin", "https://example.com")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the reflected origin", got)
	}

	// The literal "null" origin must not be reflected.
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+DefaultJavaScriptURL, nil)
	req2.Header.Set("Origin", "null")
	resp2, _ := srv.Client().Do(req2)
	defer resp2.Body.Close()
	if got := resp2.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf(`Access-Control-Allow-Origin = %q for a "null" origin, want none`, got)
	}
}
