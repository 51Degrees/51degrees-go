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

package usagesharing

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// httpRequestKey is the Evidence key the web integration gives the request.
const httpRequestKey = "server.http-request"

// userAgentEngine accepts only the User-Agent header, as a detection engine
// accepts only the Evidence that changes its result.
type userAgentEngine struct{ *core.BaseElement }

func (e *userAgentEngine) Process(core.FlowData) error { return nil }

// newSharingPipeline builds a Pipeline of a userAgentEngine and a usage
// sharing element that shares nothing, so no test sends anything.
func newSharingPipeline(t *testing.T, b *Builder) (core.Pipeline, *Element) {
	t.Helper()
	e := b.SetSharePercentage(0).SetShareUsageURL("http://127.0.0.1:1/").Build()
	engine := &userAgentEngine{BaseElement: core.NewBaseElement("device",
		core.NewWhitelistFilter("header.user-agent"), nil)}
	p, err := core.NewPipelineBuilder().AddFlowElement(engine).
		AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p, e
}

// The usage sharing element accepts no Evidence, so a Pipeline holding it
// accepts only what its other elements accept, and If-None-Match or any
// other header no element uses stays out of cache keys and the ETag.
func TestUsageSharingAcceptsNoEvidence(t *testing.T) {
	p, e := newSharingPipeline(t, NewBuilder())
	for _, key := range []string{"header.if-none-match", "header.x-test",
		"cookie.51d_test", "query.51d_test", "server.client-ip",
		httpRequestKey} {
		if p.EvidenceKeyFilter().Include(key) {
			t.Errorf("the Pipeline accepts %s", key)
		}
		if e.EvidenceKeyFilter().Include(key) {
			t.Errorf("the usage sharing element accepts %s", key)
		}
	}
	if !p.EvidenceKeyFilter().Include("header.user-agent") {
		t.Error("the Pipeline does not accept the engine's header.user-agent")
	}
}

// With the web request in the Evidence, usage sharing shares the request's
// headers, cookies and query string parameters that the sharing rules let
// through, a header no element accepts among them. It leaves out the Cookie
// header, a blocked header and the request itself, and a key also in the
// Evidence keeps the Evidence value.
func TestUsageSharingSharesTheWebRequest(t *testing.T) {
	p, e := newSharingPipeline(t, NewBuilder().SetBlockedHTTPHeaders("x-blocked"))

	r := httptest.NewRequest("GET", "/page?51D_q=1&other=2", nil)
	r.Header.Set("User-Agent", "from-request")
	r.Header.Set("X-Test", "shared")
	r.Header.Set("X-Blocked", "not shared")
	r.Header.Set("Cookie", "a=b; 51D_c=3")

	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "from-evidence")
	fd.AddEvidence(httpRequestKey, r)

	want := map[string]string{
		"header.host":       "example.com",
		"header.user-agent": "from-evidence",
		"header.x-test":     "shared",
		"cookie.51d_c":      "3",
		"query.51d_q":       "1",
	}
	if got := e.shareableEvidence(fd); !reflect.DeepEqual(got, want) {
		t.Errorf("shared %v, want %v", got, want)
	}
}

// Without a web request, usage sharing shares the Evidence values the
// sharing rules let through, as it always has.
func TestUsageSharingSharesEvidenceWithoutAWebRequest(t *testing.T) {
	p, e := newSharingPipeline(t, NewBuilder().SetBlockedHTTPHeaders("x-blocked").
		SetIncludedQueryStringParameters("myparam"))

	fd := p.CreateFlowData()
	fd.AddEvidenceMap(map[string]any{
		"header.user-agent": "agent",
		"header.x-test":     "shared",
		"header.x-blocked":  "not shared",
		"cookie.51d_c":      "3",
		"cookie.other":      "not shared",
		"query.51d_q":       "1",
		"query.myparam":     "4",
		"query.other":       "not shared",
		"server.client-ip":  "203.0.113.5",
	})

	want := map[string]string{
		"header.user-agent": "agent",
		"header.x-test":     "shared",
		"cookie.51d_c":      "3",
		"query.51d_q":       "1",
		"query.myparam":     "4",
		"server.client-ip":  "203.0.113.5",
	}
	if got := e.shareableEvidence(fd); !reflect.DeepEqual(got, want) {
		t.Errorf("shared %v, want %v", got, want)
	}
}
