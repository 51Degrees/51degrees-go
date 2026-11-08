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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/usagesharing"
)

// newSharingServer serves a web pipeline holding a usage sharing element
// that shares nothing, so the test sends nothing.
func newSharingServer(t *testing.T) *httptest.Server {
	t.Helper()
	sharing := usagesharing.NewBuilder().SetSharePercentage(0).
		SetShareUsageURL("http://127.0.0.1:1/").Build()
	p, err := AssemblePipeline(PipelineConfig{}, newHeaderEngine(), sharing)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	srv := httptest.NewServer(NewMiddleware(p).Handler(http.NotFoundHandler()))
	t.Cleanup(func() {
		srv.Close()
		p.Close()
	})
	return srv
}

// A browser revalidates with the ETag it was given in If-None-Match. With
// usage sharing in the Pipeline that header must not change the ETag, or the
// revalidation never gets its 304.
func TestRevalidationGetsNotModifiedWithUsageSharing(t *testing.T) {
	for _, path := range []string{DefaultJavaScriptURL, DefaultJSONURL} {
		t.Run(path, func(t *testing.T) {
			srv := newSharingServer(t)
			send := func(etag string) *http.Response {
				req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
				req.Header.Set("User-Agent", "agent-1")
				req.Header.Set("X-Test", "a header no element accepts")
				if etag != "" {
					req.Header.Set("If-None-Match", etag)
				}
				resp, err := srv.Client().Do(req)
				if err != nil {
					t.Fatalf("request: %v", err)
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				return resp
			}

			first := send("")
			etag := first.Header.Get("ETag")
			if first.StatusCode != http.StatusOK || etag == "" {
				t.Fatalf("first request status = %d, ETag = %q",
					first.StatusCode, etag)
			}
			again := send(etag)
			if again.StatusCode != http.StatusNotModified {
				t.Fatalf("revalidation status = %d, want 304, with the ETag "+
					"%q for the revalidation", again.StatusCode,
					again.Header.Get("ETag"))
			}
		})
	}
}

// The middleware adds the request to the Evidence as server.http-request on
// every request, for the usage sharing element to read.
func TestMiddlewareAddsTheWebRequest(t *testing.T) {
	p := buildWebPipeline(t)
	defer p.Close()

	var got any
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fd, ok := FromContext(r.Context()); ok {
			got, _ = fd.Evidence().Get("server.http-request")
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(NewMiddleware(p).Handler(next))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/page", nil)
	req.Header.Set("X-Test", "seen")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()

	w, ok := got.(core.WebRequest)
	if !ok {
		t.Fatalf("server.http-request = %T, want core.WebRequest", got)
	}
	r := w.Request()
	if r.Header.Get("X-Test") != "seen" || r.URL.Path != "/page" {
		t.Fatalf("server.http-request is not this request: %s %v",
			r.URL.Path, r.Header)
	}
}

// Printing the Evidence the middleware builds must not write out the
// request's headers or cookies, whichever verb prints it.
func TestPrintedEvidenceHoldsNoRequestValues(t *testing.T) {
	p := buildWebPipeline(t)
	defer p.Close()

	var printed []string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fd, ok := FromContext(r.Context()); ok {
			if v, found := fd.Evidence().Get("server.http-request"); found {
				printed = append(printed,
					fmt.Sprintf("%v", v), fmt.Sprintf("%+v", v),
					fmt.Sprintf("%#v", v), fmt.Sprintf("%s", v))
			}
			all := fd.Evidence().AsMap()
			delete(all, "header.x-secret")
			delete(all, "cookie.session")
			printed = append(printed,
				fmt.Sprintf("%v", all["server.http-request"]),
				fmt.Sprintf("%+v", all))
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(NewMiddleware(p).Handler(next))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/page", nil)
	req.Header.Set("X-Secret", "header-secret-value")
	req.Header.Set("Cookie", "session=cookie-secret-value")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	_ = resp.Body.Close()

	if len(printed) == 0 {
		t.Fatal("the handler printed nothing")
	}
	for _, text := range printed {
		if strings.Contains(text, "header-secret-value") ||
			strings.Contains(text, "cookie-secret-value") {
			t.Errorf("printed Evidence holds a request value: %.200s", text)
		}
	}
}
