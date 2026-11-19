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

package dd_cloud

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// failingCloud stands in for a cloud service that fails. Where discovery
// fails, every request answers 503, so the Cloud Request Engine fails before
// it starts the data request. Otherwise only the data request fails, with a
// 500.
func failingCloud(t *testing.T, discoveryFails bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch {
			case discoveryFails:
				w.WriteHeader(http.StatusServiceUnavailable)
			case r.URL.Path == "/accessibleproperties":
				w.Write([]byte(`{"Products":{"device":{` +
					`"DataTier":"CloudV4Free","Properties":[` +
					`{"Name":"IsMobile","Type":"Bool","Category":"Device"}]}}}`))
			case r.URL.Path == "/evidencekeys":
				w.Write([]byte(`["header.user-agent"]`))
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
	t.Cleanup(srv.Close)
	return srv
}

// A failed cloud request that the Device Detection Cloud Engine meets while
// the Flow Data is processed is recorded once, as the Cloud Request Engine's
// error, and not again as the Device Detection Cloud Engine's.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/automated-testing.md?plain=1#L159-L162
func TestAFailedCloudRequestIsRecordedOnce(t *testing.T) {
	for _, tc := range []struct {
		name           string
		discoveryFails bool
	}{
		{"the data request fails", false},
		{"discovery fails before the request", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := failingCloud(t, tc.discoveryFails)
			quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
			re, err := requestengine.NewBuilder().
				SetResourceKey("test-resource-key").
				SetEndpoint(srv.URL + "/").
				SetHTTPClient(srv.Client()).
				SetLogger(quiet).
				Build()
			if err != nil {
				t.Fatal(err)
			}
			p, err := core.NewPipelineBuilder().
				SetLogger(quiet).
				SetSuppressProcessExceptions(true).
				AddFlowElement(re).
				AddFlowElement(New(quiet)).
				Build()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { p.Close() })
			fd := p.CreateFlowData()
			fd.AddEvidence("header.user-agent", "Mozilla/5.0")
			_ = fd.Process()
			errs := fd.Errors()
			if len(errs) != 1 || errs[0].Element != core.FlowElement(re) {
				t.Errorf("the errors are %v, want one, the Cloud Request "+
					"Engine's", errs)
			}
		})
	}
}
