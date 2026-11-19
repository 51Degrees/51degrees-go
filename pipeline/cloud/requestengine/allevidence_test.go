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

// The request carries every Evidence value apart from the reserved names and
// the web request view, and the keys the cloud service lists still bound the
// engine's Evidence key filter, which is what a cache key is built from.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#processing

import (
	"net/http/httptest"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
)

// The test server's evidencekeys answer lists header.user-agent and
// query.user-agent, so every other key below is one the service does not
// list.
const unlistedHeader = "header.sec-ch-ua-platform"

// Evidence the cloud service does not list is sent all the same, under the
// query, header and cookie prefixes, because what the service uses changes
// over time and knowing about the other Evidence matters to it.
func TestEvidenceTheServiceDoesNotListIsStillSent(t *testing.T) {
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
	fd.AddEvidence(unlistedHeader, "Windows")
	fd.AddEvidence("query.my-own-thing", "alpha")
	fd.AddEvidence("cookie.51d_experiment", "beta")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	for field, want := range map[string]string{
		"user-agent":         "ua-header",
		"sec-ch-ua-platform": "Windows",
		"my-own-thing":       "alpha",
		"51d_experiment":     "beta",
	} {
		if got := cs.lastForm.Get(field); got != want {
			t.Errorf("the request carried %s = %q, want %q", field, got, want)
		}
	}
}

// Evidence with no prefix is sent under its whole key, and ranks as Evidence
// whose prefix is that whole key, so a header of the same name takes
// precedence over it and a key that is itself query takes the precedence of
// query Evidence. This is what .NET does, where KeyHasPrefix compares the
// part of the key before its first separator, or the whole key where it has
// none (CloudRequestEngine.cs lines 865 to 878 in 51degrees-dotnet at a145428).
func TestEvidenceWithNoPrefixIsSentUnderItsWholeKey(t *testing.T) {
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
	fd.AddEvidence("bare", "value")
	fd.AddEvidence("x-thing", "bare")
	fd.AddEvidence("header.x-thing", "header")
	fd.AddEvidence("query", "bare-query")
	fd.AddEvidence("header.query", "header-query")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	for field, want := range map[string]string{
		"bare":    "value",
		"x-thing": "header",
		"query":   "bare-query",
	} {
		if got := cs.lastForm.Get(field); got != want {
			t.Errorf("the request carried %s = %q, want %q", field, got, want)
		}
	}
}

// Evidence whose key strips to a reserved name is left out under any prefix
// and in any letter case, and with no prefix at all, so the request carries
// the configured resource key once and no license, licence or values field
// from the Evidence.
func TestTheReservedNamesAreNeverSentFromEvidence(t *testing.T) {
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
	fd.AddEvidence("query.Resource", "not-the-resource-key")
	fd.AddEvidence("header.License", "not-the-license-key")
	fd.AddEvidence("cookie.LICENCE", "not-the-license-key-either")
	fd.AddEvidence("other.VALUES", "not-the-property-list")
	fd.AddEvidence("RESOURCE", "bare-and-still-reserved")
	fd.AddEvidence("values", "bare-and-still-reserved-too")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	if got := cs.lastForm.Get("resource"); got != "test-key" {
		t.Errorf("the request carried resource = %q, want test-key", got)
	}
	for _, field := range []string{"license", "licence", "values"} {
		if _, sent := cs.lastForm[field]; sent {
			t.Errorf("the request carried the reserved field %s", field)
		}
	}
}

// A value set under the web request view's key is left out of the request in
// any letter case, as the view itself is, because it is a view of the whole
// web request and not a value to send.
func TestTheWebRequestKeyIsNeverSentInAnyLetterCase(t *testing.T) {
	for _, key := range []string{"server.http-request", "Server.HTTP-Request"} {
		cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
		srv := httptest.NewServer(cs.handler())

		e := buildEngine(t, srv)
		p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
		if err != nil {
			srv.Close()
			t.Fatalf("build: %v", err)
		}

		fd := p.CreateFlowData()
		fd.AddEvidence("header.user-agent", "ua-header")
		fd.AddEvidence(key, "not-a-value")
		if err := fd.Process(); err != nil {
			p.Close()
			srv.Close()
			t.Fatalf("process: %v", err)
		}
		if v, sent := cs.lastForm["http-request"]; sent {
			t.Errorf("%s was sent to the cloud service as %.80q", key, v)
		}
		if got := cs.lastForm.Get("user-agent"); got != "ua-header" {
			t.Errorf("user-agent sent as %q, want ua-header", got)
		}
		p.Close()
		srv.Close()
	}
}

// A cache key is built from the Evidence the engine's own filter accepts, and
// that filter still holds the keys the cloud service lists, so Evidence the
// service does not list leaves the key alone and a repeated request still
// finds the entry the first one wrote. The request carries the unlisted value
// all the same.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/caching.md#generation-of-keys
func TestTheCacheKeyStaysOnTheAcceptedEvidence(t *testing.T) {
	cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()

	e := buildEngine(t, srv)
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	first := p.CreateFlowData()
	first.AddEvidence("header.user-agent", "ua-header")
	if err := first.Process(); err != nil {
		t.Fatalf("process the first: %v", err)
	}

	// The same accepted Evidence, with a value the cloud service does not
	// list beside it, which is what a new client hint header or a custom
	// cookie looks like.
	second := p.CreateFlowData()
	second.AddEvidence("header.user-agent", "ua-header")
	second.AddEvidence(unlistedHeader, "Windows")
	if err := second.Process(); err != nil {
		t.Fatalf("process the second: %v", err)
	}

	filter := e.EvidenceKeyFilter()
	if filter.Include(unlistedHeader) {
		t.Fatalf("the engine accepts %s, so the cache key would take it",
			unlistedHeader)
	}
	firstKey := cache.GenerateKey(first, filter)
	secondKey := cache.GenerateKey(second, filter)
	if firstKey != secondKey {
		t.Errorf("the cache key changed from %q to %q, so a repeated "+
			"request would miss the cache", firstKey, secondKey)
	}
	if got := cs.lastForm.Get("sec-ch-ua-platform"); got != "Windows" {
		t.Errorf("the second request carried sec-ch-ua-platform = %q, "+
			"want Windows", got)
	}
}
