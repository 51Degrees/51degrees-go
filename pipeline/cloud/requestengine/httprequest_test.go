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

import (
	"net/http/httptest"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The web integration puts the request itself in the Evidence as
// server.http-request, for usage sharing. The Cloud Request Engine neither
// accepts it, so it stays out of the engine's cache key, nor sends it to the
// cloud service.
func TestTheWebRequestIsNotSentToTheCloud(t *testing.T) {
	cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()

	e := buildEngine(t, srv)
	if e.EvidenceKeyFilter().Include("server.http-request") {
		t.Error("the Cloud Request Engine accepts server.http-request")
	}
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua-header")
	fd.AddEvidence("server.http-request",
		core.NewWebRequest(httptest.NewRequest("GET", "/", nil)))
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	if v, sent := cs.lastForm["http-request"]; sent {
		t.Errorf("the request was sent to the cloud service as %.80q", v)
	}
	if got := cs.lastForm.Get("user-agent"); got != "ua-header" {
		t.Errorf("user-agent sent as %q, want ua-header", got)
	}
}
