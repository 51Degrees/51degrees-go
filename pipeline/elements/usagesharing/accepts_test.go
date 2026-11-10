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
// through, a header no element accepts among them, with its client IP address,
// protocol and method. It leaves out the Cookie header, a blocked header, the
// path and the request itself, and a key also in the Evidence keeps the
// Evidence value.
func TestUsageSharingSharesTheWebRequest(t *testing.T) {
	p, e := newSharingPipeline(t, NewBuilder().SetBlockedHTTPHeaders("x-blocked"))

	r := httptest.NewRequest("GET", "/page?51D_q=1&other=2", nil)
	r.Header.Set("User-Agent", "from-request")
	r.Header.Set("X-Test", "shared")
	r.Header.Set("X-Blocked", "not shared")
	r.Header.Set("Cookie", "a=b; 51D_c=3")

	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "from-evidence")
	fd.AddEvidence(httpRequestKey, core.NewWebRequest(r).
		WithClientIP("203.0.113.5").WithProtocol("https"))

	want := map[string]string{
		"header.host":           "example.com",
		"header.user-agent":     "from-evidence",
		"header.x-test":         "shared",
		"header.protocol":       "https",
		"cookie.51d_c":          "3",
		"query.51d_q":           "1",
		"server.client-ip":      "203.0.113.5",
		"server.request-method": "GET",
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

// Whatever the configuration, and even when all Evidence is shared, these
// keys are never shared, matched as the whole key or after a ".".
func TestUsageSharingNeverSharesTheNeverSharedKeys(t *testing.T) {
	configs := map[string]*Builder{
		"default":   NewBuilder(),
		"share all": NewBuilder().SetShareAllEvidence(true),
		"no blocks": NewBuilder().SetBlockedHTTPHeaders(),
		"share all and no blocks": NewBuilder().SetShareAllEvidence(true).
			SetBlockedHTTPHeaders().SetShareAllQueryStringParameters(true),
	}
	never := []string{"id.email", "id.salt", "server.request-query",
		"server.request-path", "header.cookie", "header.Cookie",
		"query.id.email", "query.request-path"}
	for name, b := range configs {
		e := b.Build()
		for _, key := range never {
			if e.isShared(key) {
				t.Errorf("%s: isShared(%q) = true, want false", name, key)
			}
		}
		if e.cfg.shareAllEvidence && !e.isShared("query.valid.email") {
			t.Errorf("%s: isShared(query.valid.email) = false, want true", name)
		}
		e.Close()
	}
}

// With all Evidence shared, the Cookie header is still left out, whether it
// is in the Evidence or in the request, and the cookies starting 51D_ are
// shared as their own cookie Evidence.
func TestUsageSharingNeverSharesTheCookieHeader(t *testing.T) {
	p, e := newSharingPipeline(t, NewBuilder().SetShareAllEvidence(true))

	r := httptest.NewRequest("GET", "/page", nil)
	r.Header.Set("Cookie", "a=secret; 51D_c=3")

	fd := p.CreateFlowData()
	fd.AddEvidenceMap(map[string]any{
		"header.cookie":        "a=secret; 51D_c=3",
		"id.email":             "a@example.com",
		"server.request-query": "x=1",
		"server.request-path":  "/page",
	})
	fd.AddEvidence(httpRequestKey, core.NewWebRequest(r))

	got := e.shareableEvidence(fd)
	for _, key := range []string{"header.cookie", "id.email",
		"server.request-query", "server.request-path"} {
		if _, found := got[key]; found {
			t.Errorf("shared %s", key)
		}
	}
	if got["cookie.51d_c"] != "3" {
		t.Errorf("cookie.51d_c = %q, want 3, shared %v", got["cookie.51d_c"], got)
	}
}

// A session value is shared only when its name starts 51D_.
func TestUsageSharingSharesOnlyTheSessionValuesStarting51D(t *testing.T) {
	e := NewBuilder().Build()
	defer e.Close()
	for key, want := range map[string]bool{
		"session.51D_x": true, "session.51d_x": true, "session.other": false,
	} {
		if got := e.isShared(key); got != want {
			t.Errorf("isShared(%q) = %v, want %v", key, got, want)
		}
	}
}

// The Cookie header is blocked by default, as the specification names it.
func TestUsageSharingBlocksTheCookieHeaderByDefault(t *testing.T) {
	e := NewBuilder().Build()
	defer e.Close()
	if _, ok := e.cfg.blockedHeaders["cookie"]; !ok {
		t.Errorf("default blocked headers = %v, want cookie", e.cfg.blockedHeaders)
	}
}

// A session value in the web request is shared only when its name starts
// 51D_, unless all Evidence is shared, when every session value is.
func TestUsageSharingSharesTheSessionValuesOfTheWebRequest(t *testing.T) {
	for _, shareAll := range []bool{false, true} {
		p, e := newSharingPipeline(t,
			NewBuilder().SetShareAllEvidence(shareAll))
		r := httptest.NewRequest("GET", "/page", nil)
		fd := p.CreateFlowData()
		fd.AddEvidence(httpRequestKey, core.NewWebRequest(r).WithSession(
			map[string]string{"51D_s": "1", "other": "2"}))

		got := e.shareableEvidence(fd)
		if got["session.51d_s"] != "1" {
			t.Errorf("share all %v: session.51d_s = %q, shared %v",
				shareAll, got["session.51d_s"], got)
		}
		if _, found := got["session.other"]; found != shareAll {
			t.Errorf("share all %v: session.other shared = %v", shareAll, found)
		}
	}
}

// No rule removes the Resource Key or the License Key. Under the fiftyone
// prefix, the names the cloud service gives them, they are shared whatever
// the configuration. As query string or form parameters they follow the
// query rule, so they are shared when allow listed, when all query string
// parameters are shared or when all Evidence is shared.
func TestUsageSharingKeepsTheResourceAndLicenseKeys(t *testing.T) {
	fiftyone := []string{"fiftyone.resource-key", "fiftyone.license-key"}
	query := []string{"query.resource", "query.license", "query.licence"}
	for name, c := range map[string]struct {
		b     *Builder
		query bool
	}{
		"default":   {NewBuilder(), false},
		"no blocks": {NewBuilder().SetBlockedHTTPHeaders(), false},
		"allow listed": {NewBuilder().SetIncludedQueryStringParameters(
			"Resource", "License", "Licence"), true},
		"all query": {NewBuilder().SetShareAllQueryStringParameters(true), true},
		"share all": {NewBuilder().SetShareAllEvidence(true), true},
	} {
		e := c.b.Build()
		for _, key := range fiftyone {
			if !e.isShared(key) {
				t.Errorf("%s: isShared(%q) = false, want true", name, key)
			}
		}
		for _, key := range query {
			if got := e.isShared(key); got != c.query {
				t.Errorf("%s: isShared(%q) = %v, want %v",
					name, key, got, c.query)
			}
		}
		e.Close()
	}
}

// The keys are found in the web request under the same rules as in the
// Evidence, so reading the request as well loses neither of them.
func TestUsageSharingSharesTheKeysOfTheWebRequest(t *testing.T) {
	for _, allQuery := range []bool{false, true} {
		p, e := newSharingPipeline(t,
			NewBuilder().SetShareAllQueryStringParameters(allQuery))
		r := httptest.NewRequest("GET", "/page?resource=r&license=l", nil)
		fd := p.CreateFlowData()
		fd.AddEvidence("fiftyone.resource-key", "r")
		fd.AddEvidence("fiftyone.license-key", "l")
		fd.AddEvidence(httpRequestKey, core.NewWebRequest(r))

		got := e.shareableEvidence(fd)
		for _, key := range []string{"fiftyone.resource-key",
			"fiftyone.license-key"} {
			if _, found := got[key]; !found {
				t.Errorf("all query %v: %s not shared, shared %v",
					allQuery, key, got)
			}
		}
		for key, want := range map[string]string{
			"query.resource": "r", "query.license": "l"} {
			if v, found := got[key]; found != allQuery ||
				(found && v != want) {
				t.Errorf("all query %v: %s = %q, found %v, shared %v",
					allQuery, key, v, found, got)
			}
		}
	}
}
