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

// A request that carries a sealed Human Confidence result redeems it through
// the engine's one request, which names derived.HumanConfidence and holds no
// transaction taken from the request.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#redeeming-through-the-json-call

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// sealedRequest builds an engine asking for requested, sends fd's Evidence
// through it to cs, and gives the form the cloud service was sent.
func sealedRequest(t *testing.T, cs *cloudServer, requested []string,
	evidence map[string]string) *CloudRequestEngine {
	t.Helper()
	srv := httptest.NewServer(cs.handler())
	t.Cleanup(srv.Close)
	b := NewBuilder().
		SetResourceKey("test-key").
		SetLicenseKey("test-license").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client())
	if len(requested) > 0 {
		b.SetRequestedProperties(requested...)
	}
	e, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	fd := p.CreateFlowData()
	for key, value := range evidence {
		fd.AddEvidence(key, value)
	}
	// A sealed result comes from a page the engine issued, unless the test
	// names the page itself.
	if _, named := evidence[HumanConfidencePageEvidence]; !named && evidence[HumanConfidenceEvidence] != "" {
		fd.AddEvidence(HumanConfidencePageEvidence, e.IssueHumanConfidencePage().ID)
	}
	if err := processAndRead(fd); err != nil {
		t.Fatalf("process: %v", err)
	}
	return e
}

func TestASealedRequestNamesHumanConfidence(t *testing.T) {
	cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
	sealedRequest(t, cs, []string{"device.IsMobile"},
		map[string]string{HumanConfidenceEvidence: "sealed"})
	if got := cs.lastForm.Get("values"); got != "device.IsMobile,derived.HumanConfidence" {
		t.Errorf("values is %q", got)
	}
	if got := cs.lastForm.Get("humanconfidence"); got != "sealed" {
		t.Errorf("humanconfidence is %q, want the sealed result as it arrived", got)
	}
}

// A list that already names it, in any letter case, is sent as configured.
func TestASealedRequestDoesNotNameHumanConfidenceTwice(t *testing.T) {
	cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
	sealedRequest(t, cs, []string{"device.IsMobile", "DERIVED.humanconfidence"},
		map[string]string{HumanConfidenceEvidence: "sealed"})
	if got := cs.lastForm.Get("values"); got != "device.IsMobile,DERIVED.humanconfidence" {
		t.Errorf("values is %q", got)
	}
}

// With no configured list, a call that redeems still names
// derived.HumanConfidence, as the specification says, because the cloud
// service needs a call with a License Key to name its Properties.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#redeeming-through-the-json-call
func TestASealedRequestWithNoListNamesHumanConfidence(t *testing.T) {
	cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
	sealedRequest(t, cs, nil, map[string]string{HumanConfidenceEvidence: "sealed"})
	if got := cs.lastForm.Get("values"); got != "derived.HumanConfidence" {
		t.Errorf("values is %q, want derived.HumanConfidence", got)
	}
	cs = &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
	sealedRequest(t, cs, nil, map[string]string{"header.user-agent": "ua"})
	if cs.lastForm.Has("values") {
		t.Errorf("a call that redeems nothing named values %q", cs.lastForm.Get("values"))
	}
}

func TestARequestWithNoSealedResultIsUnchanged(t *testing.T) {
	for _, sealed := range []string{"", "absent"} {
		cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
		evidence := map[string]string{"query.transaction": "t1"}
		if sealed == "" {
			evidence[HumanConfidenceEvidence] = ""
		}
		sealedRequest(t, cs, []string{"device.IsMobile"}, evidence)
		if got := cs.lastForm.Get("values"); got != "device.IsMobile" {
			t.Errorf("values is %q", got)
		}
		if got := cs.lastForm.Get("transaction"); got != "t1" {
			t.Errorf("transaction is %q, want it sent as before", got)
		}
	}
}

// The added name is left out of the report of Properties not returned, so
// the report is made for the configured list alone.
func TestTheAddedNameIsNotReported(t *testing.T) {
	// The answer leaves out derived, so a report that counted the added name
	// would name it.
	cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
	var log bytes.Buffer
	pe := newPageEngine(t, cs, "test-license", func(b *Builder) {
		b.SetRequestedProperties("device.IsMobile").
			SetLogger(slog.New(slog.NewTextHandler(&log, nil)))
	})
	page := pe.e.IssueHumanConfidencePage()
	pe.request(HumanConfidenceEvidence, "sealed", HumanConfidencePageEvidence, page.ID)
	if got := cs.lastForm.Get("values"); got != "device.IsMobile,derived.HumanConfidence" {
		t.Fatalf("values is %q", got)
	}
	if strings.Contains(strings.ToLower(log.String()), "derived.humanconfidence") {
		t.Errorf("the report names the added Property: %s", log.String())
	}
	if len(pe.e.requestedProperties) != 1 || pe.e.requestedProperties[0] != "device.IsMobile" {
		t.Errorf("the configured list became %v", pe.e.requestedProperties)
	}
}

// A transaction or challenge in the request, under any prefix and in any
// letter case, is left out of a call that redeems, because only the
// transaction the package kept may go in it.
func TestASealedRequestCarriesNoTransactionFromTheRequest(t *testing.T) {
	cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
	fromRequest := map[string]string{
		HumanConfidenceEvidence: "sealed",
		"query.transaction":     "from-the-page",
		"header.Transaction":    "from-a-header",
		"cookie.CHALLENGE":      "from-a-cookie",
		"challenge":             "with-no-prefix",
	}
	sealedRequest(t, cs, nil, fromRequest)
	for field, values := range cs.lastForm {
		switch lower := strings.ToLower(field); {
		case lower == "challenge":
			t.Errorf("the call holds %s = %q", field, values)
		case lower == "transaction":
			if field != "transaction" || len(values) != 1 {
				t.Errorf("the call holds %s = %q, want one transaction", field, values)
			}
			for _, from := range fromRequest {
				if values[0] == from {
					t.Errorf("the call's transaction %q came from the request", from)
				}
			}
		}
	}
}

func TestCarriesSealedResult(t *testing.T) {
	p, err := core.NewPipelineBuilder().Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for evidence, want := range map[string]bool{
		"":                       false,
		"query.humanconfidence":  true,
		"QUERY.HumanConfidence":  true,
		"header.humanconfidence": false,
		"cookie.humanconfidence": false,
	} {
		fd := p.CreateFlowData()
		if evidence != "" {
			fd.AddEvidence(evidence, "sealed")
		}
		if got := CarriesSealedResult(fd); got != want {
			t.Errorf("%q gives %v, want %v", evidence, got, want)
		}
	}
	fd := p.CreateFlowData()
	fd.AddEvidence(HumanConfidenceEvidence, "")
	if CarriesSealedResult(fd) {
		t.Error("an empty value counts as a sealed result")
	}
}
