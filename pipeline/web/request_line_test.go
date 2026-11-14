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

// The middleware adds the method, path and query string of the request line
// as Evidence, exactly as the request line carried them, when an element in
// the Pipeline accepts them.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/features/web-integration.md#populating-evidence

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The three request line keys, as the specification names them.
const (
	requestMethodKey = "server.request-method"
	requestPathKey   = "server.request-path"
	requestQueryKey  = "server.request-query"
)

// acceptingEngine accepts the Evidence keys it is given and does nothing else.
type acceptingEngine struct{ *core.BaseElement }

func (e *acceptingEngine) Process(core.FlowData) error { return nil }

// requestEvidence runs r through the middleware of a Pipeline whose only
// element accepts keys, and returns the Evidence the Flow Data held.
func requestEvidence(t *testing.T, r *http.Request, keys ...string) map[string]any {
	t.Helper()
	e := &acceptingEngine{core.NewBaseElement("accepting",
		core.NewWhitelistFilter(keys...), nil)}
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	var got map[string]any
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fd, ok := FromContext(r.Context()); ok {
			got = fd.Evidence().AsMap()
		}
	})
	NewMiddleware(p).Handler(next).ServeHTTP(httptest.NewRecorder(), r)
	if got == nil {
		t.Fatal("the handler was given no Flow Data")
	}
	return got
}

// wantRequestLine fails unless the Evidence holds the three values.
func wantRequestLine(t *testing.T, got map[string]any, method, path, query string) {
	t.Helper()
	for key, want := range map[string]string{
		requestMethodKey: method,
		requestPathKey:   path,
		requestQueryKey:  query,
	} {
		value, ok := got[key]
		if !ok {
			t.Errorf("%s is not in the Evidence, want %q", key, want)
			continue
		}
		if value != want {
			t.Errorf("%s = %#v, want %q", key, value, want)
		}
	}
}

// wantNoRequestLine fails if the Evidence holds any of the three keys.
func wantNoRequestLine(t *testing.T, got map[string]any) {
	t.Helper()
	for _, key := range []string{requestMethodKey, requestPathKey, requestQueryKey} {
		if value, ok := got[key]; ok {
			t.Errorf("%s = %#v, want it left out", key, value)
		}
	}
}

// Nothing is decoded and the order of the query string is kept.
func TestTheRequestLineIsEvidenceExactlyAsSent(t *testing.T) {
	r := httptest.NewRequest("GET", "/a%2Fb/c?x=1&y=%20&x=2", nil)
	got := requestEvidence(t, r, requestMethodKey, requestPathKey, requestQueryKey)
	wantRequestLine(t, got, "GET", "/a%2Fb/c", "x=1&y=%20&x=2")
}

// The query string is there, and empty, when the request has none, so that a
// missing key means the request line was not supplied.
func TestARequestWithNoQueryStringGivesAnEmptyQuery(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	got := requestEvidence(t, r, requestMethodKey, requestPathKey, requestQueryKey)
	wantRequestLine(t, got, "POST", "/", "")
}

// An absolute-form request line, which a client sends to a proxy, gives the
// path after the authority as it was sent, not as Go would escape it again.
func TestAnAbsoluteFormRequestLineGivesThePathAsSent(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.com/a|b/c?q=%7C", nil)
	got := requestEvidence(t, r, requestMethodKey, requestPathKey, requestQueryKey)
	wantRequestLine(t, got, "GET", "/a|b/c", "q=%7C")
}

// A Pipeline with no element that accepts the request line carries none of
// it, so nothing new reaches a cloud request, a cache key or the ETag.
func TestTheRequestLineIsLeftOutWhereNoElementAsksForIt(t *testing.T) {
	r := httptest.NewRequest("GET", "/page?x=1", nil)
	wantNoRequestLine(t, requestEvidence(t, r, "header.user-agent"))
}

// A request made in code rather than read by a server has no request line, so
// none of it is added, and the view gives no path.
func TestARequestWithNoRequestLineGivesNoneOfIt(t *testing.T) {
	r, err := http.NewRequest("GET", "http://example.com/page?x=1", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	wantNoRequestLine(t, requestEvidence(t, r,
		requestMethodKey, requestPathKey, requestQueryKey))
	if path := core.NewWebRequest(r).Path(); path != "" {
		t.Errorf("the path of a request with no request line = %q, want none", path)
	}
}
