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
	"sync"
	"time"
)

// The transaction binds a sealed Human Confidence result to the page that
// asked for it. The engine issues one for each page, keeps it here, sends it
// on every request it makes for that page, and sends it once more to redeem
// the page's sealed result, after which it is forgotten.
//
// A page is known by its session id, the Evidence query.session-id that the
// Sequence Element gives each page and that the client script sends with
// every request the page makes. The specification does not say how a later
// request is tied to the page's transaction, and the session id is what the
// Pipeline already uses to tell one page from another, so the transaction is
// kept under it. It is never taken from the request.
// See https://github.com/51Degrees/specifications/blob/81c1e4df5c7eb923d88ec3b1926d562b0a754ad3/pipeline-specification/features/human-confidence.md#the-transaction

// SessionIDEvidence is the Evidence that names the page a request came from.
// The Sequence Element adds it.
const SessionIDEvidence = "query.session-id"

// TransactionLifetime is how long a page's transaction is kept. A sealed
// result redeems within 10 seconds of being made, and the page asks for it
// after the browser tests run, so this leaves a page plenty of time.
const TransactionLifetime = 30 * time.Minute

// maxTransactions is how many pages' transactions are kept at once. Past it,
// expired ones are dropped, and while it is still reached a new page gets no
// transaction, so its sealed result gives Medium.
const maxTransactions = 100_000

// transactions keeps one transaction for each page.
type transactions struct {
	mu   sync.Mutex
	kept map[string]keptTransaction
}

type keptTransaction struct {
	value   string
	expires time.Time
}

// forPage gives the transaction kept for the page session, issuing a new one
// where there is none or it has expired. It gives false where none can be
// kept.
func (t *transactions) forPage(session string, now time.Time) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if k, ok := t.kept[session]; ok && now.Before(k.expires) {
		return k.value, true
	}
	if t.kept == nil {
		t.kept = map[string]keptTransaction{}
	}
	if len(t.kept) >= maxTransactions {
		for s, k := range t.kept {
			if !now.Before(k.expires) {
				delete(t.kept, s)
			}
		}
		if len(t.kept) >= maxTransactions {
			return "", false
		}
	}
	value, err := newTransaction()
	if err != nil {
		return "", false
	}
	t.kept[session] = keptTransaction{value, now.Add(TransactionLifetime)}
	return value, true
}

// take gives the transaction kept for the page session and forgets it, so it
// redeems one sealed result at most. It gives false where none is kept or it
// has expired.
func (t *transactions) take(session string, now time.Time) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k, ok := t.kept[session]
	delete(t.kept, session)
	if !ok || !now.Before(k.expires) {
		return "", false
	}
	return k.value, true
}

// newTransaction gives a new random value, which no other page shares.
func newTransaction() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
