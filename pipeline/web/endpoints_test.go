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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/javascriptbuilder"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/jsonbuilder"
)

// A request whose If-None-Match matches the ETag is answered without the
// Pipeline running, with 304 to GET and HEAD and 412 to any other method
// (web-integration.md, Cache header detail).
func TestMatchingIfNoneMatchIsNotProcessed(t *testing.T) {
	for _, test := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, DefaultJavaScriptURL, http.StatusNotModified},
		{http.MethodHead, DefaultJavaScriptURL, http.StatusNotModified},
		{http.MethodGet, DefaultJSONURL, http.StatusNotModified},
		{http.MethodPost, DefaultJSONURL, http.StatusPreconditionFailed},
	} {
		t.Run(test.method+test.path, func(t *testing.T) {
			srv, engine := newFailingServer(t)

			first := fetch(t, srv, test.method, test.path, "")
			etag := first.Header.Get("ETag")
			if first.StatusCode != http.StatusOK || etag == "" {
				t.Fatalf("first request status = %d, ETag = %q",
					first.StatusCode, etag)
			}
			if got := engine.calls.Load(); got != 1 {
				t.Fatalf("the first request was processed %d times, want 1",
					got)
			}

			second := fetch(t, srv, test.method, test.path, etag)
			if second.StatusCode != test.want {
				t.Errorf("status = %d, want %d", second.StatusCode, test.want)
			}
			if got := engine.calls.Load(); got != 1 {
				t.Errorf("the request whose If-None-Match matches was "+
					"processed, %d requests processed, want 1", got)
			}
		})
	}
}

// Where the application supplies a JavaScript Builder and no JSON Builder,
// the JSON Builder is added immediately before the JavaScript Builder, which
// wraps the JSON it builds (web-integration.md, Pipeline configuration).
func TestJSONBuilderGoesJustBeforeASuppliedJavaScriptBuilder(t *testing.T) {
	p, err := AssemblePipeline(PipelineConfig{}, newHeaderEngine(),
		javascriptbuilder.NewBuilder().SetEndpoint(DefaultJSONURL).Build())
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	defer p.Close()
	keys := []string{}
	for _, e := range p.FlowElements() {
		keys = append(keys, e.DataKey())
	}
	for i, key := range keys {
		if key != javascriptbuilder.DataKey {
			continue
		}
		if i == 0 || keys[i-1] != jsonbuilder.DataKey {
			t.Errorf("the elements are %v, want %s just before %s", keys,
				jsonbuilder.DataKey, javascriptbuilder.DataKey)
		}
		return
	}
	t.Fatalf("the elements are %v, with no %s", keys,
		javascriptbuilder.DataKey)
}

// ipEngine accepts only the client IP address, which is not a header.
type ipEngine struct{ *core.BaseElement }

func (e *ipEngine) Process(core.FlowData) error { return nil }

// Vary names every header in the Pipeline's Evidence key filter, so a
// Pipeline whose filter holds no header gets no Vary header
// (web-integration.md, Cache header detail).
func TestNoVaryWhenTheFilterHoldsNoHeader(t *testing.T) {
	engine := &ipEngine{BaseElement: core.NewBaseElement("location",
		core.NewWhitelistFilter("server.client-ip"), nil)}
	p, err := AssemblePipeline(PipelineConfig{DisableJavaScriptBuilder: true},
		engine)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	defer p.Close()
	srv := httptest.NewServer(NewMiddleware(p).Handler(http.NotFoundHandler()))
	defer srv.Close()

	resp := fetch(t, srv, http.MethodPost, DefaultJSONURL, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Values("Vary"); len(got) != 0 {
		t.Errorf("Vary = %q, want none, as the filter holds no header", got)
	}
}

// The two intercepted paths can be configured, and the default paths are
// then passed to the application (web-integration.md, Intercepted Urls).
func TestEndpointPathsCanBeConfigured(t *testing.T) {
	const jsPath, jsonPath = "/assets/fod.js", "/api/fod/json"
	p, err := AssemblePipeline(PipelineConfig{JSONEndpoint: jsonPath},
		newHeaderEngine())
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	defer p.Close()
	mw := NewMiddleware(p, WithJavaScriptURL(jsPath), WithJSONURL(jsonPath))
	srv := httptest.NewServer(mw.Handler(http.NotFoundHandler()))
	defer srv.Close()

	for _, test := range []struct {
		method, path, contentType string
		status                    int
	}{
		{http.MethodGet, jsPath, "application/x-javascript", http.StatusOK},
		{http.MethodPost, jsonPath, "application/json", http.StatusOK},
		{http.MethodGet, DefaultJavaScriptURL, "", http.StatusNotFound},
		{http.MethodPost, DefaultJSONURL, "", http.StatusNotFound},
	} {
		resp := fetch(t, srv, test.method, test.path, "")
		if resp.StatusCode != test.status {
			t.Errorf("%s %s status = %d, want %d", test.method, test.path,
				resp.StatusCode, test.status)
		}
		if test.contentType == "" {
			continue
		}
		if got := resp.Header.Get("Content-Type"); got != test.contentType {
			t.Errorf("%s %s Content-Type = %q, want %q", test.method,
				test.path, got, test.contentType)
		}
	}
}

// The max-age the endpoints send can be configured (web-integration.md,
// Cache header detail).
func TestMaxAgeCanBeConfigured(t *testing.T) {
	const maxAgeSeconds = 60
	p := buildWebPipeline(t)
	defer p.Close()
	mw := NewMiddleware(p, WithMaxAge(maxAgeSeconds*time.Second))
	srv := httptest.NewServer(mw.Handler(http.NotFoundHandler()))
	defer srv.Close()

	for _, ep := range clientSideEndpoints {
		resp := fetch(t, srv, ep.method, ep.path, "")
		if got, want := resp.Header.Get("Cache-Control"),
			"private, max-age=60"; got != want {
			t.Errorf("%s %s Cache-Control = %q, want %q", ep.method,
				ep.path, got, want)
		}
	}
}

// The JSON endpoint gives the JSON the JavaScript embeds for the same
// Evidence (javascript-builder.md, SetEndpoint).
func TestJSONEndpointGivesTheJSONTheJavaScriptEmbeds(t *testing.T) {
	p := buildWebPipeline(t)
	defer p.Close()
	srv := httptest.NewServer(NewMiddleware(p).Handler(http.NotFoundHandler()))
	defer srv.Close()

	_, js := fetchFrom(t, srv, http.MethodGet, DefaultJavaScriptURL, "", "")
	_, jsonText := fetchFrom(t, srv, http.MethodPost, DefaultJSONURL, "", "")
	if len(jsonText) < len(`{"device":{}}`) {
		t.Fatalf("the JSON endpoint gave %q, want the JSON Builder's JSON",
			jsonText)
	}
	if !strings.Contains(js, jsonText) {
		t.Errorf("the JavaScript does not embed the JSON the JSON endpoint "+
			"gives.\nJSON: %s\nJavaScript: %s", jsonText, js)
	}
}
