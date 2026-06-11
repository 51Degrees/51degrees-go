package core

import "strings"

// Evidence is the set of input values added to a Flow Data instance, stored as
// case-insensitive key-value pairs. Evidence is immutable once Flow Data
// processing has begun.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/evidence.md
type Evidence interface {
	// Get returns the value for a key (case-insensitive) and whether it was present.
	Get(key string) (any, bool)
	// AsMap returns a copy of all evidence keyed by normalized (lower-case) name.
	AsMap() map[string]any
	// Keys returns all (normalized) evidence keys. Order is unspecified.
	Keys() []string
}

// evidence is the default immutable implementation. It is populated by Flow Data
// before processing and is then only read.
type evidence struct {
	data map[string]any
}

func newEvidence() *evidence { return &evidence{data: make(map[string]any)} }

func (e *evidence) put(key string, value any) { e.data[lower(key)] = value }

func (e *evidence) Get(key string) (any, bool) {
	v, ok := e.data[lower(key)]
	return v, ok
}

func (e *evidence) AsMap() map[string]any {
	out := make(map[string]any, len(e.data))
	for k, v := range e.data {
		out[k] = v
	}
	return out
}

func (e *evidence) Keys() []string {
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
