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

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/aspectengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// acceptCHEngine is a Cloud Aspect Engine that gives each value of its
// section of the cloud response as text.
type acceptCHEngine struct {
	*aspectengine.BaseCloudAspectEngine
}

func (e *acceptCHEngine) Process(fd core.FlowData) error {
	aspect, err := aspectengine.ParseAspect(fd, e.DataKey())
	if err != nil {
		return err
	}
	data := engines.NewBaseAspectData()
	for name := range aspect {
		data.Set(name, aspectengine.PropertyValue[string](aspect, name))
	}
	fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData { return data })
	return nil
}

// acceptCHCloud is a stand-in cloud service whose device product has a
// SetHeader Property for Accept-CH. Its discovery endpoints answer 503 until
// it is healthy.
func acceptCHCloud(t *testing.T, healthy *atomic.Bool) *httptest.Server {
	t.Helper()
	discovery := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !healthy.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			io.WriteString(w, body)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", discovery(`{"Products":{"device":`+
		`{"DataTier":"CloudV4","Properties":[`+
		`{"Name":"SetHeaderBrowserAccept-CH","Type":"String","Category":"Device"}]}}}`))
	mux.HandleFunc("/evidencekeys", discovery(`["header.user-agent"]`))
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"device":{"setheaderbrowseraccept-ch":"Sec-CH-UA,Sec-CH-UA-Mobile"}}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// acceptCHServer serves the web integration over a Pipeline of a Cloud
// Request Engine and an acceptCHEngine against the stand-in cloud.
func acceptCHServer(t *testing.T, healthy *atomic.Bool) *httptest.Server {
	t.Helper()
	cloud := acceptCHCloud(t, healthy)
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	cre, err := requestengine.NewBuilder().
		SetResourceKey("k").
		SetEndpoint(cloud.URL + "/").
		SetHTTPClient(cloud.Client()).
		SetLogger(quiet).
		Build()
	if err != nil {
		t.Fatalf("build the Cloud Request Engine: %v", err)
	}
	engine := &acceptCHEngine{aspectengine.NewBaseCloudAspectEngine("device", quiet)}
	p, err := AssemblePipeline(PipelineConfig{
		Logger:                    quiet,
		SuppressProcessExceptions: true,
	}, cre, engine)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	srv := httptest.NewServer(NewMiddleware(p).Handler(ok))
	t.Cleanup(srv.Close)
	return srv
}

// acceptCH returns the Accept-CH header of a response to a page request.
func acceptCH(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/", nil)
	req.Header.Set("User-Agent", "agent")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()
	return resp.Header.Get("Accept-CH")
}

// A SetHeader Property a cloud product gives sets its response header, from
// the first request after the engine is built.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#consequences-for-consumers-of-the-engine
func TestAcceptCHFromCloudSetHeaderProperty(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	srv := acceptCHServer(t, &healthy)
	if got := acceptCH(t, srv); !strings.Contains(got, "Sec-CH-UA") {
		t.Errorf("Accept-CH is %q, want the cloud's Sec-CH-UA", got)
	}
}

// After a transient failure when the engine is built, the header follows as
// soon as a request has fetched the Properties, because SetHeaders reads the
// Pipeline's Property metadata as it is then rather than as it was built.
func TestAcceptCHFollowsRetriedDiscovery(t *testing.T) {
	var healthy atomic.Bool
	srv := acceptCHServer(t, &healthy)
	if got := acceptCH(t, srv); got != "" {
		t.Fatalf("Accept-CH is %q before the Properties are known", got)
	}
	healthy.Store(true)
	if got := acceptCH(t, srv); !strings.Contains(got, "Sec-CH-UA") {
		t.Errorf("Accept-CH is %q once the Properties are known, want the "+
			"cloud's Sec-CH-UA", got)
	}
}
