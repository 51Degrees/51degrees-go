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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
)

// cloudOnlyHeader is a header only the stand-in cloud accepts as Evidence,
// so it reaches the Vary header through the Cloud Request Engine alone.
const cloudOnlyHeader = "Sec-Ch-Ua-Platform"

// cloudStandIn is a stand-in cloud service whose discovery endpoints answer
// 503 until it is healthy.
type cloudStandIn struct {
	healthy atomic.Bool
}

func (c *cloudStandIn) handler() http.Handler {
	discovery := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !c.healthy.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			io.WriteString(w, body)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", discovery(`{"Products":{}}`))
	mux.HandleFunc("/evidencekeys",
		discovery(`["header.user-agent","header.sec-ch-ua-platform"]`))
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"device":{}}`)
	})
	return mux
}

// cloudWebServer serves the web integration over a Pipeline holding a Cloud
// Request Engine built against the stand-in cloud.
func cloudWebServer(t *testing.T, cloud *cloudStandIn) *httptest.Server {
	t.Helper()
	cloudSrv := httptest.NewServer(cloud.handler())
	t.Cleanup(cloudSrv.Close)
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	cre, err := requestengine.NewBuilder().
		SetResourceKey("k").
		SetEndpoint(cloudSrv.URL + "/").
		SetHTTPClient(cloudSrv.Client()).
		SetLogger(quiet).
		Build()
	if err != nil {
		t.Fatalf("build the Cloud Request Engine: %v", err)
	}
	p, err := AssemblePipeline(PipelineConfig{
		Logger:                    quiet,
		SuppressProcessExceptions: true,
	}, cre)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	srv := httptest.NewServer(NewMiddleware(p).Handler(http.NotFoundHandler()))
	t.Cleanup(srv.Close)
	return srv
}

// jsonVary returns the Vary header of a response from the JSON endpoint.
func jsonVary(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+DefaultJSONURL, nil)
	req.Header.Set("User-Agent", "agent")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	return resp.Header.Get("Vary")
}

// The Cloud Request Engine fetches its Evidence keys when it is built, so the
// first response's Vary header already lists the headers the cloud accepts.
func TestVaryCarriesCloudEvidenceKeysFromBuild(t *testing.T) {
	cloud := &cloudStandIn{}
	cloud.healthy.Store(true)
	srv := cloudWebServer(t, cloud)
	if vary := jsonVary(t, srv); !strings.Contains(vary, cloudOnlyHeader) {
		t.Fatalf("the first response varies on %q, want %s in it", vary,
			cloudOnlyHeader)
	}
}

// After a transient failure at build time the Vary header lacks the cloud's
// headers, and lists them in the response to the request that fetches the
// Evidence keys, because the Pipeline reads the engine's keys rather than
// keeping the empty set it was built with.
func TestVaryFollowsCloudDiscoveryAfterTransientFailure(t *testing.T) {
	cloud := &cloudStandIn{}
	srv := cloudWebServer(t, cloud)
	if vary := jsonVary(t, srv); strings.Contains(vary, cloudOnlyHeader) {
		t.Fatalf("the response varies on %q before the Evidence keys are "+
			"known", vary)
	}
	cloud.healthy.Store(true)
	if vary := jsonVary(t, srv); !strings.Contains(vary, cloudOnlyHeader) {
		t.Fatalf("once the Evidence keys are known the response varies on "+
			"%q, want %s in it", vary, cloudOnlyHeader)
	}
}
