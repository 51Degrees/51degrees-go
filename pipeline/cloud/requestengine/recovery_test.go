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

package requestengine

// These tests follow the rule for which failures count towards the
// recovery period.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#recovery-mode

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// recoveryPipeline builds a Pipeline holding an engine that enters its
// recovery period on the first failure that counts, against the stand-in
// cloud, with dataEndpoint replacing the stand-in's when it is set.
func recoveryPipeline(t *testing.T, cs *cloudServer, dataEndpoint string) (*CloudRequestEngine, core.Pipeline) {
	t.Helper()
	srv := httptest.NewServer(cs.handler())
	t.Cleanup(srv.Close)
	b := NewBuilder().
		SetResourceKey("k").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		SetFailuresToEnterRecovery(1)
	if dataEndpoint != "" {
		b.SetDataEndpoint(dataEndpoint)
	}
	e, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build the Pipeline: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return e, p
}

// A status from 400 to 499 other than 429 answers that one request's
// content, so it does not count toward the recovery period, which every
// visitor shares. Each request still reaches the cloud service and gets the
// service's own error.
func TestClientErrorsDoNotCountTowardRecovery(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 408, 413, 422} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			cs := &cloudServer{jsonStatus: status, jsonBody: `{"errors":["refused"]}`}
			e, p := recoveryPipeline(t, cs, "")
			for i := 1; i <= 3; i++ {
				err := p.CreateFlowData().Process()
				var cloudErr *CloudRequestError
				if !errors.As(err, &cloudErr) || cloudErr.StatusCode != status {
					t.Fatalf("request %d gives %v, want the cloud's %d", i, err, status)
				}
			}
			if got := cs.jsonRequests.Load(); got != 3 {
				t.Errorf("the cloud received %d of 3 requests", got)
			}
			if _, recovering := e.inRecovery(); recovering {
				t.Error("the engine is in its recovery period")
			}
		})
	}
}

// A request that did not complete, a status of 500 or more, a 429 or a
// success status whose body could not be used says the cloud service is in
// trouble, so it counts, and the next request is refused as temporarily
// unavailable without reaching the service.
func TestServiceTroubleCountsTowardRecovery(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	cases := []struct {
		name         string
		status       int
		body         string
		dataEndpoint string
	}{
		{"500", http.StatusInternalServerError, `{}`, ""},
		{"503", http.StatusServiceUnavailable, ``, ""},
		{"429", http.StatusTooManyRequests, `{"errors":["slow down"]}`, ""},
		{"200 that is not JSON", http.StatusOK, `not json`, ""},
		{"200 with no body", http.StatusOK, ``, ""},
		{"connection refused", http.StatusOK, `{}`, dead.URL + "/json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cs := &cloudServer{jsonStatus: c.status, jsonBody: c.body}
			_, p := recoveryPipeline(t, cs, c.dataEndpoint)
			first := p.CreateFlowData().Process()
			var unavailable *TemporarilyUnavailableError
			if first == nil || errors.As(first, &unavailable) {
				t.Fatalf("the first request gives %v, want the failure", first)
			}
			sent := cs.jsonRequests.Load()
			if err := p.CreateFlowData().Process(); !errors.As(err, &unavailable) {
				t.Fatalf("the second request gives %v, want "+
					"TemporarilyUnavailableError", err)
			}
			if got := cs.jsonRequests.Load(); got != sent {
				t.Errorf("the second request reached the cloud")
			}
		})
	}
}

// The rule for which failures count, being every status the rule names and
// the edges of the 400 to 499 range.
func TestSignalsServiceTrouble(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"no response", NewCloudRequestError("refused", errors.New("dial")), true},
		{"request not made", errors.New("bad endpoint"), true},
		{"200 body not usable", NewCloudResponseError("not JSON", 200, nil), true},
		{"204", NewCloudResponseError("no data", 204, nil), true},
		{"304", NewCloudResponseError("not modified", 304, nil), false},
		{"400", NewCloudResponseError("bad request", 400, nil), false},
		{"401", NewCloudResponseError("unknown key", 401, nil), false},
		{"403", NewCloudResponseError("forbidden", 403, nil), false},
		{"408", NewCloudResponseError("request timeout", 408, nil), false},
		{"429", NewCloudResponseError("slow down", 429, nil), true},
		{"499", NewCloudResponseError("closed", 499, nil), false},
		{"500", NewCloudResponseError("failed", 500, nil), true},
		{"503", NewCloudResponseError("unavailable", 503, nil), true},
	}
	for _, c := range cases {
		if got := signalsServiceTrouble(c.err); got != c.want {
			t.Errorf("%s: signalsServiceTrouble = %v, want %v", c.name, got, c.want)
		}
	}
}
