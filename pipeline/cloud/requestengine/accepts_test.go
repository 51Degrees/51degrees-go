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

// The Cloud Request Engine accepts only the keys the cloud service's
// evidencekeys list names, compared without regard to case, so a header that
// changes no result, such as If-None-Match, stays out of the Pipeline's cache
// keys and the ETag. Before the list has loaded it accepts nothing.
func TestAcceptsOnlyTheCloudsEvidenceKeys(t *testing.T) {
	cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()

	e := buildEngine(t, srv)
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua-header")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	// The test server's evidencekeys list is header.user-agent and
	// query.user-agent.
	for key, want := range map[string]bool{
		"header.user-agent":    true,
		"Query.User-Agent":     true,
		"header.if-none-match": false,
		"header.x-test":        false,
		"cookie.51d_test":      false,
		"query.extra":          false,
		"server.http-request":  false,
	} {
		if got := e.EvidenceKeyFilter().Include(key); got != want {
			t.Errorf("the engine accepts %s = %v, want %v", key, got, want)
		}
		if got := p.EvidenceKeyFilter().Include(key); got != want {
			t.Errorf("the Pipeline accepts %s = %v, want %v", key, got, want)
		}
	}
}
