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
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
)

// newCloudServer serves a web pipeline holding a Cloud Request Engine whose
// cloud service is a test server that says it uses the User-Agent header.
func newCloudServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"Products":{"device":{"DataTier":"CloudV4",`+
			`"Properties":[{"Name":"ismobile","Type":"Bool","Category":"Device"}]}}}`)
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `["header.User-Agent","query.User-Agent"]`)
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"device":{"ismobile":false}}`)
	})
	cloud := httptest.NewServer(mux)
	t.Cleanup(cloud.Close)

	engine, err := requestengine.NewBuilder().SetResourceKey("test-key").
		SetEndpoint(cloud.URL + "/").SetHTTPClient(cloud.Client()).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, err := AssemblePipeline(PipelineConfig{}, engine)
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
// the Cloud Request Engine in the Pipeline that header must not change the
// ETag, or the revalidation never gets its 304.
func TestCloudRevalidationGetsNotModified(t *testing.T) {
	for _, path := range []string{DefaultJavaScriptURL, DefaultJSONURL} {
		t.Run(path, func(t *testing.T) {
			srv := newCloudServer(t)
			send := func(etag string) *http.Response {
				req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
				req.Header.Set("User-Agent", "agent-1")
				req.Header.Set("X-Test", "a header the cloud does not use")
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
