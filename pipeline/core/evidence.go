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

package core

import (
	"strings"
	"sync"
)

// Evidence is the set of input values added to a Flow Data instance, stored as
// case-insensitive key-value pairs. The caller adds values before processing,
// and a Flow Element outside a parallel group may add values while it
// processes. Later Flow Elements, and the caller once processing is complete,
// read them.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/evidence.md
type Evidence interface {
	// Get returns the value for a key (case-insensitive) and whether it was present.
	Get(key string) (any, bool)
	// AsMap returns a copy of all evidence keyed by normalized (lower-case) name.
	AsMap() map[string]any
	// Keys returns all (normalized) evidence keys. Order is unspecified.
	Keys() []string
}

// evidence is the default implementation. Flow Data writes to it, and the
// lock lets Flow Elements in a parallel group read it at the same time.
type evidence struct {
	mu   sync.RWMutex
	data map[string]any
}

func newEvidence() *evidence { return &evidence{data: make(map[string]any)} }

// put stores value under key, replacing any value held under the same key
// when the case is ignored.
func (e *evidence) put(key string, value any) {
	e.mu.Lock()
	e.data[lower(key)] = value
	e.mu.Unlock()
}

func (e *evidence) Get(key string) (any, bool) {
	e.mu.RLock()
	v, ok := e.data[lower(key)]
	e.mu.RUnlock()
	return v, ok
}

func (e *evidence) AsMap() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string]any, len(e.data))
	for k, v := range e.data {
		out[k] = v
	}
	return out
}

func (e *evidence) Keys() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	keys := make([]string, 0, len(e.data))
	for k := range e.data {
		keys = append(keys, k)
	}
	return keys
}

// Evidence key prefixes, in order from highest to lowest precedence.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/evidence.md
const (
	PrefixQuery    = "query"
	PrefixHeader   = "header"
	PrefixCookie   = "cookie"
	PrefixServer   = "server"
	PrefixFiftyOne = "fiftyone"
	PrefixLocation = "location"
)

// prefixPrecedence maps known prefixes to their precedence rank (lower wins).
var prefixPrecedence = map[string]int{
	PrefixQuery:    0,
	PrefixHeader:   1,
	PrefixCookie:   2,
	PrefixServer:   3,
	PrefixFiftyOne: 4,
	PrefixLocation: 5,
}

// SplitEvidenceKey splits a key into its prefix and field (suffix) on the first
// '.'. If there is no '.', the whole key is returned as the field and the prefix
// is empty.
func SplitEvidenceKey(key string) (prefix, field string) {
	if i := strings.IndexByte(key, '.'); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

// ComparePrefixes returns a negative, zero, or positive number reporting whether
// prefix a has higher (negative), equal, or lower (positive) precedence than b.
// Known prefixes rank ahead of unknown ones; unknown prefixes are ordered
// alphabetically so that conflicts always resolve deterministically.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/evidence.md
func ComparePrefixes(a, b string) int {
	a, b = lower(a), lower(b)
	ra, aKnown := prefixPrecedence[a]
	rb, bKnown := prefixPrecedence[b]
	switch {
	case aKnown && bKnown:
		return ra - rb
	case aKnown && !bKnown:
		return -1
	case !aKnown && bKnown:
		return 1
	default:
		return strings.Compare(a, b)
	}
}
