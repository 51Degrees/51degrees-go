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

// The transaction the engine keeps for each page, by the page's session id,
// and how a sealed Human Confidence result is redeemed with it.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#the-transaction

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// page is a Pipeline holding an engine that keeps transactions, against cs.
type page struct {
	t  *testing.T
	cs *cloudServer
	p  core.Pipeline
}

func newPage(t *testing.T, cs *cloudServer, license string, setup ...func(*Builder)) *page {
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
	e.KeepHumanConfidenceTransactions()
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return &page{t, cs, p}
}

// request processes a request with evidence, in pairs of key and value, and
// gives its Cloud Request Data. The form sent is cs.lastForm.
func (pg *page) request(evidence ...string) *CloudRequestData {
	pg.t.Helper()
	fd := pg.p.CreateFlowData()
	for i := 0; i+1 < len(evidence); i += 2 {
		fd.AddEvidence(evidence[i], evidence[i+1])
	}
	if err := processAndRead(fd); err != nil {
		pg.t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		pg.t.Fatalf("get: %v", err)
	}
	return data.(*CloudRequestData)
}

func TestAPageHasOneTransactionOnEveryRequest(t *testing.T) {
	pg := newPage(t, &cloudServer{jsonBody: `{}`}, "test-license")
	pg.request(SessionIDEvidence, "page-a")
	first := pg.cs.lastForm.Get("transaction")
	pg.request(SessionIDEvidence, "page-a")
	second := pg.cs.lastForm.Get("transaction")
	pg.request(SessionIDEvidence, "page-b")
	other := pg.cs.lastForm.Get("transaction")
	if first == "" || first != second {
		t.Errorf("the page's requests carried %q and %q, want one transaction", first, second)
	}
	if other == "" || other == first {
		t.Errorf("another page carried %q, want a transaction of its own", other)
	}
}

// The transaction is the one the engine kept, never one from the request,
// and the request's transaction and challenge, under any prefix, are left
// out.
func TestTheKeptTransactionIsTheOnlyOne(t *testing.T) {
	pg := newPage(t, &cloudServer{jsonBody: `{}`}, "test-license")
	pg.request(SessionIDEvidence, "page-a")
	kept := pg.cs.lastForm.Get("transaction")
	pg.request(SessionIDEvidence, "page-a", "query.transaction", "from-the-page",
		"cookie.Challenge", "from-a-cookie")
	if got := pg.cs.lastForm["transaction"]; len(got) != 1 || got[0] != kept {
		t.Errorf("the call held transaction %v, want only %q", got, kept)
	}
	for field := range pg.cs.lastForm {
		if strings.EqualFold(field, "challenge") {
			t.Errorf("the call held %s", field)
		}
	}
}

func TestASealedResultRedeemsWithItsPagesTransactionOnce(t *testing.T) {
	pg := newPage(t, &cloudServer{jsonBody: `{}`}, "test-license")
	pg.request(SessionIDEvidence, "page-a")
	kept := pg.cs.lastForm.Get("transaction")
	data := pg.request(SessionIDEvidence, "page-a", HumanConfidenceEvidence, "sealed",
		"query.transaction", "from-the-page")
	if got := pg.cs.lastForm["transaction"]; len(got) != 1 || got[0] != kept {
		t.Errorf("the redeem held transaction %v, want only %q", got, kept)
	}
	if pg.cs.lastForm.Get("humanconfidence") != "sealed" || !data.RedeemsHumanConfidence() {
		t.Error("the sealed result was not redeemed")
	}
	// The transaction was used, so a second sealed result from the page goes
	// with none, and the answer decides it.
	pg.request(SessionIDEvidence, "page-a", HumanConfidenceEvidence, "sealed-again")
	if pg.cs.lastForm.Has("transaction") {
		t.Errorf("a used transaction was sent again, %q", pg.cs.lastForm.Get("transaction"))
	}
	// The page's next request is given a new transaction.
	pg.request(SessionIDEvidence, "page-a")
	if next := pg.cs.lastForm.Get("transaction"); next == "" || next == kept {
		t.Errorf("the page's next request carried %q", next)
	}
}

// A sealed result with no transaction kept for its request is sent with
// none, as the cloud service redeems one made with none, while
// RedeemWithoutTransaction is true.
func TestASealedResultWithNoKeptTransactionIsSentWithNone(t *testing.T) {
	if !RedeemWithoutTransaction {
		t.Skip("a sealed result with no kept transaction is not sent")
	}
	pg := newPage(t, &cloudServer{jsonBody: `{}`}, "test-license")
	for name, evidence := range map[string][]string{
		"no session id":             {HumanConfidenceEvidence, "sealed"},
		"nothing kept for the page": {SessionIDEvidence, "never-seen", HumanConfidenceEvidence, "sealed"},
	} {
		data := pg.request(append(evidence, "query.transaction", "from-the-page")...)
		if pg.cs.lastForm.Get("humanconfidence") != "sealed" || !data.RedeemsHumanConfidence() {
			t.Errorf("%s: the sealed result was not sent to be redeemed", name)
		}
		if pg.cs.lastForm.Has("transaction") {
			t.Errorf("%s: the call held transaction %q", name, pg.cs.lastForm.Get("transaction"))
		}
	}
}

// Without a License Key nothing redeems, so the sealed result is not sent.
func TestNoLicenseKeyRedeemsNothing(t *testing.T) {
	pg := newPage(t, &cloudServer{jsonBody: `{}`}, "")
	pg.request(SessionIDEvidence, "page-a")
	data := pg.request(SessionIDEvidence, "page-a", HumanConfidenceEvidence, "sealed")
	if pg.cs.lastForm.Has("humanconfidence") || data.RedeemsHumanConfidence() {
		t.Error("a sealed result was sent to be redeemed with no License Key")
	}
}

// An engine that keeps no transactions sends a request with no sealed result
// as before, the request's own transaction included.
func TestAnEngineThatKeepsNoTransactionsChangesNothing(t *testing.T) {
	cs := &cloudServer{jsonBody: `{}`}
	sealedRequest(t, cs, nil, map[string]string{
		SessionIDEvidence: "page-a", "query.transaction": "the-pages-own"})
	if got := cs.lastForm.Get("transaction"); got != "the-pages-own" {
		t.Errorf("transaction is %q", got)
	}
}

func TestAnExpiredTransactionIsNotUsed(t *testing.T) {
	now := time.Date(2026, 11, 28, 9, 0, 0, 0, time.UTC)
	pg := newPage(t, &cloudServer{jsonBody: `{}`}, "test-license", func(b *Builder) {
		b.setClock(func() time.Time { return now })
	})
	pg.request(SessionIDEvidence, "page-a")
	kept := pg.cs.lastForm.Get("transaction")
	now = now.Add(TransactionLifetime)
	pg.request(SessionIDEvidence, "page-a", HumanConfidenceEvidence, "sealed")
	if got := pg.cs.lastForm.Get("transaction"); got == kept {
		t.Error("an expired transaction was used")
	}
}

// No error and no log entry holds the sealed result, the transaction or the
// License Key, in whole or in part, even where the cloud service writes them
// in its answer.
func TestNoMessageHoldsTheSealedResultTheTransactionOrTheLicenseKey(t *testing.T) {
	const license, sealed = "license-key-0123456789", "sealed-result-ABCDEFGH"
	cs := &cloudServer{jsonBody: `{}`}
	var log bytes.Buffer
	pg := newPage(t, cs, license, func(b *Builder) {
		b.SetLogger(slog.New(slog.NewTextHandler(&log, nil)))
	})
	pg.request(SessionIDEvidence, "page-a")
	transaction := cs.lastForm.Get("transaction")
	echo := " " + license + " " + sealed + " " + transaction + " "
	secrets := []string{license, sealed, transaction,
		license[len(license)-4:], sealed[len(sealed)-6:], transaction[len(transaction)-6:]}
	check := func(what, text string) {
		t.Helper()
		for _, secret := range secrets {
			if strings.Contains(text, secret) {
				t.Errorf("%s holds %q: %s", what, secret, text)
			}
		}
	}

	cs.jsonStatus = http.StatusInternalServerError
	cs.jsonBody = fmt.Sprintf(`{"warnings":[%q],"detail":%q}`, echo, echo)
	fd := pg.p.CreateFlowData()
	fd.AddEvidence(SessionIDEvidence, "page-a")
	fd.AddEvidence(HumanConfidenceEvidence, sealed)
	err := processAndRead(fd)
	if err == nil {
		t.Fatal("a failed request gave no error")
	}
	check("the error", err.Error())
	check("the log", log.String())

	// Another page, whose transaction the errors list echoes with the rest.
	cs.jsonStatus = 0
	cs.jsonBody = `{}`
	pg.request(SessionIDEvidence, "page-b")
	transaction = cs.lastForm.Get("transaction")
	secrets = append(secrets, transaction, transaction[len(transaction)-6:])
	echo = " " + license + " " + sealed + " " + transaction + " "
	cs.jsonBody = fmt.Sprintf(`{"errors":[%q]}`, echo)
	fd = pg.p.CreateFlowData()
	fd.AddEvidence(SessionIDEvidence, "page-b")
	fd.AddEvidence(HumanConfidenceEvidence, sealed)
	if err := processAndRead(fd); err == nil {
		t.Fatal("an errors list gave no error")
	} else {
		check("the errors list's error", err.Error())
	}
}

// Once as many pages as can be kept have transactions, a new page gets none
// until expired ones make room.
func TestTheKeptTransactionsAreBounded(t *testing.T) {
	var kept transactions
	now := time.Date(2026, 11, 28, 9, 0, 0, 0, time.UTC)
	kept.kept = make(map[string]keptTransaction, maxTransactions)
	for i := 0; i < maxTransactions; i++ {
		kept.kept[fmt.Sprint("page-", i)] = keptTransaction{"t", now.Add(time.Minute)}
	}
	if _, ok := kept.forPage("new", now); ok {
		t.Error("a new page got a transaction past the limit")
	}
	if value, ok := kept.forPage("new", now.Add(2*time.Minute)); !ok || value == "" {
		t.Error("expired transactions did not make room")
	}
}
