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
	"crypto/rand"
	"encoding/base64"
	"net/url"
	"strings"
	"sync"
	"time"
)

// A sealed Human Confidence result is bound to the page that asked for it by
// a transaction. The engine issues each page a HumanConfidencePage, being a
// page id and a transaction, and keeps the transaction on the server under
// the id. The page loads the 51Degrees client script from the address the
// HumanConfidencePage gives, which carries the transaction, so the cloud
// service binds the page's sealed result to it. The page sends the sealed
// result back with its page id, and the engine redeems it with the
// transaction it kept for that id, once. The transaction is never taken from
// the request.
//
// The specification does not say how a page's later request is tied to the
// transaction kept for it. Issuing a page id with the script address, and
// reading it back as HumanConfidencePageEvidence, is that tie, and it is all
// in this file.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#the-transaction

// HumanConfidencePageEvidence is the Evidence that names the page a sealed
// result came from, being the ID of the HumanConfidencePage the page was
// given. The page sends it as the parameter humanconfidencepage. It is only
// a handle, and is never sent to the cloud service.
const HumanConfidencePageEvidence = "query.humanconfidencepage"

// TransactionLifetime is how long a page's transaction is kept. A page whose
// sealed result comes back later is not redeemed.
const TransactionLifetime = 10 * time.Minute

// maxTransactions is how many pages' transactions are kept at once. Issuing
// one more drops the oldest.
const maxTransactions = 10_000

// HumanConfidencePage is what a page that asks for Human Confidence needs,
// from IssueHumanConfidencePage. The page loads the client script from
// ScriptURL, and sends ID back as the parameter humanconfidencepage beside
// the sealed result. Printing a page shows only its ID. A struct of your own
// that holds one prints its fields without calling String, so don't log such
// a struct.
type HumanConfidencePage struct {
	// ID names the page. It is not the transaction.
	ID          string
	transaction string
}

// ScriptURL gives the address of the 51Degrees client script for the page,
// on the cloud service at endpoint (the API base, ending in /api/v4/) for
// resourceKey. It carries the page's transaction, which binds the sealed
// result the page gets to it.
func (p HumanConfidencePage) ScriptURL(endpoint, resourceKey string) string {
	return strings.TrimRight(endpoint, "/") + "/" + url.PathEscape(resourceKey) +
		".js?" + url.Values{"transaction": {p.transaction}}.Encode()
}

// String gives the page's ID alone, so a log of the page never holds its
// transaction.
func (p HumanConfidencePage) String() string { return "HumanConfidencePage(" + p.ID + ")" }

// GoString gives the same as String.
func (p HumanConfidencePage) GoString() string { return p.String() }

// IssueHumanConfidencePage issues a new page id and transaction for one page,
// and keeps the transaction until the page's sealed result redeems with it,
// it expires, or newer pages push it out.
func (e *CloudRequestEngine) IssueHumanConfidencePage() HumanConfidencePage {
	return e.pages.issue(e.now())
}

// pages keeps the transaction of each page issued, oldest first.
type pages struct {
	mu    sync.Mutex
	kept  map[string]keptTransaction
	order []string
}

type keptTransaction struct {
	value   string
	expires time.Time
}

func (p *pages) issue(now time.Time) HumanConfidencePage {
	page := HumanConfidencePage{ID: newToken(), transaction: newToken()}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.kept == nil {
		p.kept = map[string]keptTransaction{}
	}
	p.dropOld(now)
	p.kept[page.ID] = keptTransaction{page.transaction, now.Add(TransactionLifetime)}
	p.order = append(p.order, page.ID)
	for len(p.kept) > maxTransactions {
		delete(p.kept, p.order[0])
		p.order = p.order[1:]
	}
	return page
}

// take gives the transaction kept for the page id, and forgets it so it is
// used once. It gives false where none is kept for it.
func (p *pages) take(id string, now time.Time) (string, bool) {
	if id == "" {
		return "", false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dropOld(now)
	kept, ok := p.kept[id]
	delete(p.kept, id)
	return kept.value, ok
}

// dropOld forgets the transactions that have expired, and the ids of pages
// no longer kept, from the oldest.
func (p *pages) dropOld(now time.Time) {
	for len(p.order) > 0 {
		kept, ok := p.kept[p.order[0]]
		if ok && now.Before(kept.expires) {
			return
		}
		delete(p.kept, p.order[0])
		p.order = p.order[1:]
	}
}

// newToken gives a new random value, 18 bytes in URL-safe base64.
func newToken() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand does not fail on the systems Go supports.
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
