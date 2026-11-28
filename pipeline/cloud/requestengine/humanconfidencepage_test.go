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

// The page id and transaction the engine issues for each page, and how a
// sealed Human Confidence result is redeemed with the transaction kept for
// its page.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#the-transaction

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// pageEngine is an engine with a License Key against cs, in a Pipeline.
type pageEngine struct {
	t  *testing.T
	cs *cloudServer
	e  *CloudRequestEngine
	p  core.Pipeline
}

func newPageEngine(t *testing.T, cs *cloudServer, license string, setup ...func(*Builder)) *pageEngine {
	t.Helper()
	srv := httptest.NewServer(cs.handler())
	t.Cleanup(srv.Close)
	b := NewBuilder().SetResourceKey("test-key").SetLicenseKey(license).
		SetEndpoint(srv.URL + "/").SetHTTPClient(srv.Client())
	for _, s := range setup {
		s(b)
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
	return &pageEngine{t, cs, e, p}
}

// request processes a request with evidence, in pairs of key and value, and
// gives its Cloud Request Data. The form sent is cs.lastForm.
func (pe *pageEngine) request(evidence ...string) *CloudRequestData {
	pe.t.Helper()
	fd := pe.p.CreateFlowData()
	for i := 0; i+1 < len(evidence); i += 2 {
		fd.AddEvidence(evidence[i], evidence[i+1])
	}
	if err := processAndRead(fd); err != nil {
		pe.t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		pe.t.Fatalf("get: %v", err)
	}
	return data.(*CloudRequestData)
}

// transactionIn gives the transaction a script address carries.
func transactionIn(t *testing.T, address string) string {
	t.Helper()
	u, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("transaction")
}

// Each page is issued its own id and transaction, and the script address
// carries the transaction, never the id.
func TestEachPageIsIssuedItsOwnTransaction(t *testing.T) {
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, "test-license")
	first, second := pe.e.IssueHumanConfidencePage(), pe.e.IssueHumanConfidencePage()
	address := first.ScriptURL("https://cloud.example.com/api/v4", "the resource")
	if !strings.HasPrefix(address, "https://cloud.example.com/api/v4/the%20resource.js?transaction=") {
		t.Errorf("the script address is %q", address)
	}
	transaction := transactionIn(t, address)
	if transaction == "" || transaction == first.ID || strings.Contains(address, first.ID) {
		t.Errorf("the address %q carries the page id, or no transaction", address)
	}
	if first.ID == second.ID ||
		transaction == transactionIn(t, second.ScriptURL("https://cloud.example.com/api/v4/", "r")) {
		t.Error("two pages share an id or a transaction")
	}
	for _, text := range []string{fmt.Sprint(first), fmt.Sprintf("%+v", first), fmt.Sprintf("%#v", first)} {
		if strings.Contains(text, transaction) {
			t.Errorf("printing the page shows its transaction: %s", text)
		}
	}
}

// A sealed result redeems with the transaction kept for its page, which is
// the call's only transaction and is used once. The page id is never sent.
func TestASealedResultRedeemsWithItsPagesTransactionOnce(t *testing.T) {
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, "test-license")
	page := pe.e.IssueHumanConfidencePage()
	kept := transactionIn(t, page.ScriptURL("https://cloud.example.com/api/v4/", "r"))
	data := pe.request(HumanConfidenceEvidence, "sealed", HumanConfidencePageEvidence, page.ID,
		"query.transaction", "from-the-page", "cookie.Challenge", "from-a-cookie")
	form := pe.cs.lastForm
	if got := form["transaction"]; len(got) != 1 || got[0] != kept {
		t.Errorf("the redeem held transaction %v, want only %q", got, kept)
	}
	if form.Get("humanconfidence") != "sealed" || !data.RedeemsHumanConfidence() {
		t.Error("the sealed result was not redeemed")
	}
	for field, values := range form {
		if strings.EqualFold(field, "challenge") || strings.EqualFold(field, "humanconfidencepage") ||
			(len(values) > 0 && values[0] == page.ID) {
			t.Errorf("the call held %s = %q", field, values)
		}
	}
	// The transaction was used, so a second sealed result from the page is
	// not redeemed and not sent.
	data = pe.request(HumanConfidenceEvidence, "sealed-again", HumanConfidencePageEvidence, page.ID)
	if pe.cs.lastForm.Has("humanconfidence") || pe.cs.lastForm.Has("transaction") || data.RedeemsHumanConfidence() {
		t.Errorf("a sealed result was sent with a used transaction, %v", pe.cs.lastForm)
	}
}

// With no transaction kept for the page, the sealed result is neither sent
// nor redeemed, and neither is any transaction.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#where-the-sealed-result-is-not-redeemed
func TestASealedResultWithNoKeptTransactionIsNotSent(t *testing.T) {
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, "test-license")
	for name, evidence := range map[string][]string{
		"no page id":              {HumanConfidenceEvidence, "sealed"},
		"a page never issued":     {HumanConfidenceEvidence, "sealed", HumanConfidencePageEvidence, "never-issued"},
		"a page id that is empty": {HumanConfidenceEvidence, "sealed", HumanConfidencePageEvidence, ""},
	} {
		data := pe.request(append(evidence, "query.transaction", "from-the-page")...)
		if pe.cs.lastForm.Has("humanconfidence") || data.RedeemsHumanConfidence() {
			t.Errorf("%s: the sealed result was sent to be redeemed", name)
		}
		if pe.cs.lastForm.Has("transaction") {
			t.Errorf("%s: the call held transaction %q", name, pe.cs.lastForm.Get("transaction"))
		}
	}
}

// Without a License Key nothing redeems, so the sealed result is not sent.
func TestNoLicenseKeyRedeemsNothing(t *testing.T) {
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, "")
	page := pe.e.IssueHumanConfidencePage()
	data := pe.request(HumanConfidenceEvidence, "sealed", HumanConfidencePageEvidence, page.ID)
	if pe.cs.lastForm.Has("humanconfidence") || pe.cs.lastForm.Has("transaction") || data.RedeemsHumanConfidence() {
		t.Error("a sealed result was sent to be redeemed with no License Key")
	}
}

// A request with no sealed result is sent as before, its own transaction
// included, but never the page id.
func TestARequestWithNoSealedResultSendsNoPageID(t *testing.T) {
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, "test-license")
	pe.request(HumanConfidencePageEvidence, "a-page", "query.transaction", "the-pages-own")
	if got := pe.cs.lastForm.Get("transaction"); got != "the-pages-own" {
		t.Errorf("transaction is %q", got)
	}
	if pe.cs.lastForm.Has("humanconfidencepage") {
		t.Error("the page id was sent")
	}
}

func TestAnExpiredTransactionIsNotUsed(t *testing.T) {
	now := time.Date(2026, 11, 28, 9, 0, 0, 0, time.UTC)
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, "test-license", func(b *Builder) {
		b.setClock(func() time.Time { return now })
	})
	page := pe.e.IssueHumanConfidencePage()
	now = now.Add(TransactionLifetime)
	data := pe.request(HumanConfidenceEvidence, "sealed", HumanConfidencePageEvidence, page.ID)
	if pe.cs.lastForm.Has("humanconfidence") || data.RedeemsHumanConfidence() {
		t.Error("a sealed result was redeemed with an expired transaction")
	}
}

// Printing the engine shows neither its License Key nor a transaction it
// keeps, in whole or in part.
func TestPrintingTheEngineShowsNoSecret(t *testing.T) {
	const license = "made-up-license-key-0123456789"
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, license)
	page := pe.e.IssueHumanConfidencePage()
	transaction := transactionIn(t, page.ScriptURL("https://cloud.example.com/api/v4/", "r"))
	for _, text := range []string{fmt.Sprint(pe.e), fmt.Sprintf("%+v", pe.e), fmt.Sprintf("%#v", pe.e)} {
		for _, secret := range []string{license, license[len(license)-4:], transaction, transaction[len(transaction)-6:]} {
			if strings.Contains(text, secret) {
				t.Errorf("printing the engine shows %q: %s", secret, text)
			}
		}
	}
}

// Past the limit, issuing a page drops the oldest page's transaction.
func TestTheOldestTransactionIsDroppedPastTheLimit(t *testing.T) {
	var kept pages
	now := time.Date(2026, 11, 28, 9, 0, 0, 0, time.UTC)
	oldest := kept.issue(now)
	for i := 0; i < maxTransactions; i++ {
		kept.issue(now)
	}
	if _, ok := kept.take(oldest.ID, now); ok {
		t.Error("the oldest page's transaction was kept past the limit")
	}
	if len(kept.kept) != maxTransactions {
		t.Errorf("%d transactions are kept, want %d", len(kept.kept), maxTransactions)
	}
}

// No error and no log entry holds the sealed result, the transaction or the
// License Key, in whole or in part, even where the cloud service writes them
// in its answer.
func TestNoMessageHoldsTheSealedResultTheTransactionOrTheLicenseKey(t *testing.T) {
	const license, sealed = "license-key-0123456789", "sealed-result-ABCDEFGH"
	cs := &cloudServer{jsonBody: `{}`}
	var log bytes.Buffer
	pe := newPageEngine(t, cs, license, func(b *Builder) {
		b.SetLogger(slog.New(slog.NewTextHandler(&log, nil)))
	})
	for _, failure := range []struct {
		status int
		body   string
	}{
		{http.StatusInternalServerError, `{"warnings":[%q],"detail":%q}`},
		{http.StatusOK, `{"errors":[%q],"warnings":[%q]}`},
	} {
		page := pe.e.IssueHumanConfidencePage()
		transaction := transactionIn(t, page.ScriptURL("https://cloud.example.com/api/v4/", "r"))
		echo := " " + license + " " + sealed + " " + transaction + " "
		cs.jsonStatus, cs.jsonBody = failure.status, fmt.Sprintf(failure.body, echo, echo)
		fd := pe.p.CreateFlowData()
		fd.AddEvidence(HumanConfidencePageEvidence, page.ID)
		fd.AddEvidence(HumanConfidenceEvidence, sealed)
		err := processAndRead(fd)
		if err == nil {
			t.Fatal("a failed request gave no error")
		}
		text := err.Error() + log.String()
		for _, secret := range []string{license, sealed, transaction,
			license[len(license)-4:], sealed[len(sealed)-6:], transaction[len(transaction)-6:]} {
			if strings.Contains(text, secret) {
				t.Errorf("a message holds %q: %s", secret, text)
			}
		}
	}
}

// heldUnder gives the fields of form whose names are one of names, in any
// letter case.
func heldUnder(form url.Values, names ...string) []string {
	var held []string
	for field := range form {
		for _, name := range names {
			if strings.EqualFold(field, name) {
				held = append(held, field+"="+form.Get(field))
			}
		}
	}
	return held
}

// Only a query.humanconfidence that is not empty is a sealed result. A value
// named humanconfidence under a header, a cookie, server. or no prefix, in
// any letter case, is not sent, and nor is any transaction or challenge from
// the request, so the cloud service can't redeem it with a transaction the
// request chose.
func TestAHumanConfidenceValueUnderAnotherKeyIsNotSent(t *testing.T) {
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, "test-license")
	for _, other := range []string{"header.humanconfidence", "cookie.HumanConfidence",
		"server.humanconfidence", "HUMANCONFIDENCE"} {
		for _, query := range []string{"absent", ""} {
			evidence := []string{other, "made-up-sealed-value",
				"query.transaction", "from-the-page", "header.Transaction", "from-a-header",
				"cookie.challenge", "from-a-cookie"}
			if query != "absent" {
				evidence = append(evidence, HumanConfidenceEvidence, query)
			}
			data := pe.request(evidence...)
			if held := heldUnder(pe.cs.lastForm, "humanconfidence", "transaction", "challenge"); len(held) > 0 {
				t.Errorf("%s with query.humanconfidence %s: the call held %v", other, query, held)
			}
			if data.RedeemsHumanConfidence() {
				t.Errorf("%s: the request redeems", other)
			}
		}
	}
}

// A copy under another key never goes out beside the sealed result, whether
// it redeems or not.
func TestACopyBesideTheSealedResultIsNotSent(t *testing.T) {
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, "test-license")
	page := pe.e.IssueHumanConfidencePage()
	pe.request(HumanConfidenceEvidence, "made-up-sealed-value", HumanConfidencePageEvidence, page.ID,
		"cookie.humanconfidence", "a-cookie-copy")
	if got := heldUnder(pe.cs.lastForm, "humanconfidence"); len(got) != 1 || got[0] != "humanconfidence=made-up-sealed-value" {
		t.Errorf("the redeem held %v, want only the sealed result", got)
	}
	pe.request(HumanConfidenceEvidence, "made-up-sealed-value", HumanConfidencePageEvidence, "never-issued",
		"header.humanconfidence", "a-header-copy")
	if held := heldUnder(pe.cs.lastForm, "humanconfidence", "transaction"); len(held) > 0 {
		t.Errorf("a sealed result that can't redeem let %v go out", held)
	}
}

// An empty value is no sealed result, wherever it is, and changes nothing.
func TestAnEmptyHumanConfidenceChangesNothing(t *testing.T) {
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, "test-license")
	pe.request("header.humanconfidence", "", HumanConfidenceEvidence, "",
		"query.transaction", "from-the-page")
	if got := pe.cs.lastForm.Get("transaction"); got != "from-the-page" {
		t.Errorf("transaction is %q, want it sent as before", got)
	}
}

// The page id is never sent, under any prefix.
func TestThePageIDIsNeverSentUnderAnyPrefix(t *testing.T) {
	pe := newPageEngine(t, &cloudServer{jsonBody: `{}`}, "test-license")
	pe.request("cookie.HumanConfidencePage", "a-page", "header.humanconfidencepage", "a-page",
		"humanconfidencepage", "a-page")
	if held := heldUnder(pe.cs.lastForm, "humanconfidencepage"); len(held) > 0 {
		t.Errorf("the call held %v", held)
	}
}
