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
	"fmt"
	"net/http"
	"net/url"
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

// EvidenceHTTPRequest is the Evidence key under which the web integration
// adds the request, as a WebRequest. No element accepts it, so it never takes
// part in a cache key or an ETag, and the Usage Sharing Element reads the
// request's headers, cookies and query string from it.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/web-integration.md#populating-evidence
const EvidenceHTTPRequest = PrefixServer + ".http-request"

// The Evidence keys of the request line's method, path and query string,
// each exactly as the request line carried it, with no decoding. The web
// integration adds each only when an element in the Pipeline accepts it.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/features/web-integration.md#populating-evidence
const (
	EvidenceRequestMethod = PrefixServer + ".request-method"
	EvidenceRequestPath   = PrefixServer + ".request-path"
	EvidenceRequestQuery  = PrefixServer + ".request-query"
)

// webRequestPlaceholder is what formatting a WebRequest prints.
const webRequestPlaceholder = "[web request]"

// WebRequest is the read-only view of the request added as
// EvidenceHTTPRequest. Its named parts are the headers, cookies, query, session,
// client IP address, protocol, method, path and raw query string. Each part is
// returned as a copy, so changing it never changes the request. Formatting a
// WebRequest with any fmt verb, or calling String, gives a fixed placeholder,
// so printing or logging the Evidence never writes out the request's headers
// and cookies.
type WebRequest struct {
	request  *http.Request
	clientIP string
	protocol string
	session  map[string]string
}

// NewWebRequest returns a view of r.
func NewWebRequest(r *http.Request) WebRequest { return WebRequest{request: r} }

// WithClientIP returns the view with its client IP address set.
func (w WebRequest) WithClientIP(ip string) WebRequest { w.clientIP = ip; return w }

// WithProtocol returns the view with its protocol set.
func (w WebRequest) WithProtocol(p string) WebRequest { w.protocol = p; return w }

// WithSession returns the view with a copy of the session values given.
func (w WebRequest) WithSession(s map[string]string) WebRequest {
	w.session = copyStringMap(s)
	return w
}

// Headers returns the request's headers, with the Host as the Host header.
func (w WebRequest) Headers() http.Header {
	if w.request == nil {
		return http.Header{}
	}
	h := w.request.Header.Clone()
	if h == nil {
		h = http.Header{}
	}
	if w.request.Host != "" {
		h.Set("Host", w.request.Host)
	}
	return h
}

// Cookies returns the request's cookies.
func (w WebRequest) Cookies() []*http.Cookie {
	if w.request == nil {
		return nil
	}
	return w.request.Cookies()
}

// Query returns the query string parameters, and the form parameters when the
// request was already parsed. The body is never read.
func (w WebRequest) Query() url.Values {
	if w.request == nil {
		return url.Values{}
	}
	src := w.request.Form
	if src == nil && w.request.URL != nil {
		src = w.request.URL.Query()
	}
	out := make(url.Values, len(src))
	for k, v := range src {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// Session returns the session values.
func (w WebRequest) Session() map[string]string { return copyStringMap(w.session) }

// ClientIP returns the client IP address.
func (w WebRequest) ClientIP() string { return w.clientIP }

// Protocol returns the request protocol.
func (w WebRequest) Protocol() string { return w.protocol }

// Method returns the request method.
func (w WebRequest) Method() string {
	if w.request == nil {
		return ""
	}
	return w.request.Method
}

// Path returns the path of the request line exactly as it was sent, with no
// decoding and without the query string. It is empty for a request that a
// server did not read, as such a request has no request line.
func (w WebRequest) Path() string {
	path, _ := w.requestTarget()
	return path
}

// RawQuery returns the query string of the request line exactly as it was
// sent, with no decoding and without the "?", or nothing when it has none.
func (w WebRequest) RawQuery() string {
	_, query := w.requestTarget()
	return query
}

// requestTarget splits the request line's target into its path and query
// string. An absolute-form target, which a client sends to a proxy, gives the
// path after its authority. The asterisk form is its own path and the
// authority form has none (RFC 9112 section 3.2).
func (w WebRequest) requestTarget() (path, query string) {
	if w.request == nil {
		return "", ""
	}
	target := w.request.RequestURI
	if i := strings.IndexByte(target, '?'); i >= 0 {
		target, query = target[:i], target[i+1:]
	}
	if strings.HasPrefix(target, "/") || target == "*" {
		return target, query
	}
	if i := strings.Index(target, "://"); i >= 0 {
		authorityAndPath := target[i+len("://"):]
		if j := strings.IndexByte(authorityAndPath, '/'); j >= 0 {
			return authorityAndPath[j:], query
		}
	}
	return "", query
}

func copyStringMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// String returns a fixed placeholder.
func (w WebRequest) String() string { return webRequestPlaceholder }

// Format writes the fixed placeholder whatever the verb, so %v, %+v, %#v and
// %s all print the same.
func (w WebRequest) Format(f fmt.State, _ rune) {
	_, _ = f.Write([]byte(webRequestPlaceholder))
}

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
