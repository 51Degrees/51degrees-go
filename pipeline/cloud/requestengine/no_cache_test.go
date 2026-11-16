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
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// A result that holds a value made for the one request that asked for it,
// such as a 51Did, must not be stored in or served from a cache. This engine
// keeps no cache, so every request reaches the cloud service and each answer
// carries its own value.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/caching.md?plain=1#L137-L149
func TestEachRequestReachesTheCloudServiceEvenWithTheSameEvidence(t *testing.T) {
	const requests = 3
	cs := &cloudServer{
		jsonBody: `{"device":{"ismobile":true},"fodid":"made for one request"}`,
	}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()
	p, err := core.NewPipelineBuilder().AddFlowElement(buildEngine(t, srv)).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	for i := 0; i < requests; i++ {
		fd := p.CreateFlowData()
		fd.AddEvidence("header.user-agent", "the same User-Agent every time")
		if err := fd.Process(); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
		// Each answer reaches the caller, so a test that counted requests
		// could not pass by processing nothing.
		data, err := fd.Get(DataKey)
		if err != nil {
			t.Fatalf("request %d gave no cloud data: %v", i+1, err)
		}
		cloudData, ok := data.(*CloudRequestData)
		if !ok {
			t.Fatalf("request %d gave %T, want a *CloudRequestData", i+1, data)
		}
		if answer := cloudData.JSONResponse(); !strings.Contains(answer, "fodid") {
			t.Fatalf("request %d gave %q, want the answer holding its fodid", i+1, answer)
		}
	}

	if got := cs.jsonRequests.Load(); got != requests {
		t.Errorf("the cloud service was asked %d times for %d requests with "+
			"the same Evidence, want %d, so an answer holding a value made "+
			"for one request was served to another", got, requests, requests)
	}
}
