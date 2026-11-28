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

// No error or log entry the engine builds holds the License Key, a sealed
// result or a transaction, in whole or in part, wherever the text comes from.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#what-a-package-never-exposes

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// formPanickingTransport panics on the data request with the whole form it
// was to send, and passes the discovery requests to next.
type formPanickingTransport struct{ next http.RoundTripper }

func (p formPanickingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.HasSuffix(r.URL.Path, "/json") {
		body, _ := io.ReadAll(r.Body)
		panic("could not send " + string(body))
	}
	return p.next.RoundTrip(r)
}

// noneOf fails t where text holds any of secrets.
func noneOf(t *testing.T, what, text string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(text, secret) {
			t.Errorf("%s holds %q: %s", what, secret, text)
		}
	}
}

// A panic whose value holds the form is hidden like any other message.
func TestAPanicHoldingTheFormShowsNoSecret(t *testing.T) {
	const license = "made-up-license-key-0123456789"
	_, srv := newHeldServer(t, http.StatusOK)
	client := srv.Client()
	client.Transport = formPanickingTransport{client.Transport}
	e, err := NewBuilder().SetResourceKey("test-key").SetLicenseKey(license).
		SetEndpoint(srv.URL + "/").SetHTTPClient(client).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	page := e.IssueHumanConfidencePage()
	transaction := transactionIn(t, page.ScriptURL("https://cloud.example.com/api/v4/", "r"))
	fd := p.CreateFlowData()
	fd.AddEvidence(HumanConfidenceEvidence, "made-up-sealed-value")
	fd.AddEvidence(HumanConfidencePageEvidence, page.ID)
	err = processAndRead(fd)
	if err == nil || !strings.Contains(err.Error(), "could not send") {
		t.Fatalf("the panic gave %v", err)
	}
	noneOf(t, "the panic's error", err.Error(), license, license[len(license)-6:],
		"made-up-sealed-value", transaction, transaction[len(transaction)-6:])
}

// An answer that writes the License Key JSON-escaped, as a JSON writer may
// for "+" and "/", still has it hidden.
func TestAJSONEscapedLicenseKeyIsHidden(t *testing.T) {
	const license = "made+up/license+key"
	for _, written := range []string{`made+up\/license+key`, `made+up/license+key`, `made+up\/license+key`} {
		cs := &cloudServer{jsonStatus: http.StatusInternalServerError,
			jsonBody: `{"detail":"the key ` + written + ` is not known"}`}
		pe := newPageEngine(t, cs, license)
		fd := pe.p.CreateFlowData()
		fd.AddEvidence("header.user-agent", "ua")
		err := processAndRead(fd)
		if err == nil {
			t.Fatal("a failed request gave no error")
		}
		noneOf(t, "the error", err.Error(), written, "made", "license", "key\\")
	}
}

// A call that carried a sealed result could be answered with a part of it,
// which can't be found to hide, so the answer's content is left out of the
// error.
func TestAFailedRedeemLeavesTheAnswersContentOut(t *testing.T) {
	cs := &cloudServer{jsonStatus: http.StatusInternalServerError,
		jsonBody: `{"detail":"could not read made-up-sea"}`}
	pe := newPageEngine(t, cs, "made-up-license-key-0123456789")
	page := pe.e.IssueHumanConfidencePage()
	fd := pe.p.CreateFlowData()
	fd.AddEvidence(HumanConfidenceEvidence, "made-up-sealed-value")
	fd.AddEvidence(HumanConfidencePageEvidence, page.ID)
	err := processAndRead(fd)
	if err == nil || !strings.Contains(err.Error(), "'500'") {
		t.Fatalf("the failed redeem gave %v", err)
	}
	noneOf(t, "the error", err.Error(), "made-up-sea", "could not read")
}
