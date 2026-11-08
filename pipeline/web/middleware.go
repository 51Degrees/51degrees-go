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

package web

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/javascriptbuilder"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/jsonbuilder"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/setheaders"
)

// Default intercepted URL paths.
const (
	DefaultJavaScriptURL = "/51Degrees.core.js"
	DefaultJSONURL       = "/51dpipeline/json"
	DefaultMaxAge        = 30 * time.Minute
)

type contextKey struct{}

var flowDataKey = contextKey{}

// Middleware integrates a Pipeline into a net/http handler chain. For each
// request it creates a Flow Data, populates Evidence from the request, processes
// it, sets any requested response headers, and makes the Flow Data available via
// FromContext. It also serves the intercepted client-side JavaScript and JSON
// endpoints.
type Middleware struct {
	pipeline       core.Pipeline
	javaScriptURL  string
	jsonURL        string
	maxAge         time.Duration
	trustedProxies []*net.IPNet
}

// Option configures a Middleware.
type Option func(*Middleware)

// WithJavaScriptURL overrides the intercepted JavaScript endpoint path.
func WithJavaScriptURL(path string) Option { return func(m *Middleware) { m.javaScriptURL = path } }

// WithJSONURL overrides the intercepted JSON endpoint path.
func WithJSONURL(path string) Option { return func(m *Middleware) { m.jsonURL = path } }

// WithMaxAge sets the client cache lifetime for the intercepted endpoints.
func WithMaxAge(d time.Duration) Option { return func(m *Middleware) { m.maxAge = d } }

// WithTrustedProxies enables use of X-Forwarded-For / X-Real-IP for the client
// IP, but only when the immediate peer is one of the given trusted-proxy CIDR
// ranges. Without this option the peer address is always used, so a client
// cannot spoof its IP by setting a forwarding header. Invalid CIDRs are ignored.
func WithTrustedProxies(cidrs ...string) Option {
	return func(m *Middleware) {
		for _, c := range cidrs {
			if _, n, err := net.ParseCIDR(c); err == nil {
				m.trustedProxies = append(m.trustedProxies, n)
			}
		}
	}
}

// NewMiddleware creates a Middleware for the given Pipeline.
func NewMiddleware(pipeline core.Pipeline, opts ...Option) *Middleware {
	m := &Middleware{
		pipeline:      pipeline,
		javaScriptURL: DefaultJavaScriptURL,
		jsonURL:       DefaultJSONURL,
		maxAge:        DefaultMaxAge,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// FromContext returns the processed Flow Data stored on the request context.
func FromContext(ctx context.Context) (core.FlowData, bool) {
	fd, ok := ctx.Value(flowDataKey).(core.FlowData)
	return fd, ok
}

// Handler wraps next so that every request is processed by the Pipeline and the
// resulting Flow Data is attached to the request context. The intercepted
// endpoints are served directly and do not call next.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case m.javaScriptURL:
			m.serveJavaScript(w, r)
			return
		case m.jsonURL:
			m.serveJSON(w, r)
			return
		}

		fd := m.process(r)
		m.applyResponseHeaders(w, fd)
		ctx := context.WithValue(r.Context(), flowDataKey, fd)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// process creates a Flow Data, populates Evidence from the request, and runs the
// Pipeline. Errors are guarded so that a processing failure cannot take down the
// host application.
func (m *Middleware) process(r *http.Request) core.FlowData {
	fd := m.pipeline.CreateFlowData()
	m.populateEvidence(fd, r)
	// The web integration MUST NOT take down the host site, so any processing
	// error or panic is contained here. Errors remain available via fd.Errors();
	// a recovered panic is logged so production failures are still visible.
	defer func() {
		if rec := recover(); rec != nil {
			m.logger().Warn("panic recovered during pipeline processing", "error", rec)
		}
	}()
	_ = fd.Process()
	return fd
}

// logger returns a logger for the middleware: slog.Default, since the Pipeline
// does not expose its own.
func (m *Middleware) logger() *slog.Logger { return slog.Default() }

// populateEvidence adds Evidence from the HTTP request: headers (except cookie),
// cookies, query and form parameters, client IP, host and protocol, and the
// request itself as core.EvidenceHTTPRequest.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/web-integration.md#populating-evidence
func (m *Middleware) populateEvidence(fd core.FlowData, r *http.Request) {
	for name, values := range r.Header {
		if strings.EqualFold(name, "Cookie") {
			continue
		}
		if len(values) > 0 {
			fd.AddEvidence("header."+strings.ToLower(name), values[0])
		}
	}
	fd.AddEvidence("header.host", r.Host)

	for _, c := range r.Cookies() {
		fd.AddEvidence("cookie."+c.Name, c.Value)
	}

	// Form parameters are exposed as query evidence. ParseForm consumes the
	// request body only for a urlencoded form (POST/PUT/PATCH); only in that
	// case is the body buffered - capped at the same ~10 MB that net/http
	// itself applies to urlencoded forms - and reset afterwards so a
	// downstream handler can still read it in full. Any other body is never
	// read.
	if hasFormBody(r) {
		body, overflow := readCapped(r.Body, maxFormBodyBytes)
		if overflow {
			// Too large to buffer: skip the form evidence and splice the read
			// bytes back so the downstream handler still sees the full body.
			m.logger().Warn("request form body exceeds the evidence buffer cap, skipping form evidence",
				"cap", maxFormBodyBytes)
			r.Body = spliceBody(body, r.Body)
		} else {
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err := r.ParseForm(); err != nil {
				m.logger().Warn("could not parse request form", "error", err)
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
	} else if err := r.ParseForm(); err != nil {
		// No form body: ParseForm only parses the URL query and reads nothing.
		m.logger().Warn("could not parse request query", "error", err)
	}
	for name, values := range r.Form {
		if len(values) > 0 {
			fd.AddEvidence("query."+name, values[0])
		}
	}

	fd.AddEvidence("server.client-ip", m.clientIP(r))
	fd.AddEvidence("header.protocol", m.requestProtocol(r))

	// The request itself, whether or not an element accepts it, so that the
	// Usage Sharing Element can share headers no element accepts.
	fd.AddEvidence(core.EvidenceHTTPRequest, core.NewWebRequest(r))
}

// applyResponseHeaders sets the response headers requested by the Set Headers
// Element, appending to any header that is already present.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/web-integration.md#setting-response-headers
func (m *Middleware) applyResponseHeaders(w http.ResponseWriter, fd core.FlowData) {
	data, err := fd.Get(setheaders.DataKey)
	if err != nil {
		return
	}
	shd, ok := data.(*setheaders.Data)
	if !ok {
		return
	}
	for name, value := range shd.ResponseHeaders() {
		if value == "" {
			continue
		}
		if existing := w.Header().Get(name); existing != "" {
			w.Header().Set(name, existing+","+value)
		} else {
			w.Header().Set(name, value)
		}
	}
}

func (m *Middleware) serveJavaScript(w http.ResponseWriter, r *http.Request) {
	fd := m.process(r)
	js := ""
	if data, err := fd.Get(javascriptbuilder.DataKey); err == nil {
		if d, ok := data.(*javascriptbuilder.Data); ok {
			js = d.JavaScript()
		}
	}
	m.serveCacheable(w, r, fd, "application/x-javascript", []byte(js))
}

func (m *Middleware) serveJSON(w http.ResponseWriter, r *http.Request) {
	fd := m.process(r)
	jsonStr := "{}"
	if data, err := fd.Get(jsonbuilder.DataKey); err == nil {
		if d, ok := data.(*jsonbuilder.Data); ok {
			jsonStr = d.JSON()
		}
	}
	m.serveCacheable(w, r, fd, "application/json", []byte(jsonStr))
}

// serveCacheable writes the body with the cache-control, Vary, ETag,
// Content-Type and Content-Length headers required by the specification, and
// returns 304 Not Modified when the request's If-None-Match matches.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/web-integration.md#cache-header-detail
func (m *Middleware) serveCacheable(w http.ResponseWriter, r *http.Request, fd core.FlowData, contentType string, body []byte) {
	etag := m.etag(fd)
	w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", int(m.maxAge.Seconds())))
	if vary := m.varyHeader(); vary != "" {
		w.Header().Set("Vary", vary)
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", contentType)
	// Reflect the request Origin so a cross-origin client-side callback to these
	// endpoints is permitted. The literal "null" origin is not reflected.
	if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}

	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// etag computes a hash over the Evidence values that the Pipeline's filter
// accepts, so that the cached content is revalidated when relevant Evidence
// changes.
func (m *Middleware) etag(fd core.FlowData) string {
	filter := m.pipeline.EvidenceKeyFilter()
	all := fd.Evidence().AsMap()
	keys := make([]string, 0, len(all))
	for k := range all {
		if filter == nil || filter.Include(k) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	h := fnv.New64a()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0x1f})
		h.Write([]byte(fmt.Sprintf("%v", all[k])))
		h.Write([]byte{0x1e})
	}
	return fmt.Sprintf(`"%x"`, h.Sum64())
}

// varyHeader lists the HTTP headers in the Pipeline's Evidence key filter that
// can be enumerated, so caches re-fetch when those request headers change.
func (m *Middleware) varyHeader() string {
	filter := m.pipeline.EvidenceKeyFilter()
	lister, ok := filter.(core.KeyLister)
	if !ok {
		return "User-Agent"
	}
	var headers []string
	for _, key := range lister.Keys() {
		prefix, field := core.SplitEvidenceKey(key)
		if prefix == core.PrefixHeader {
			headers = append(headers, http.CanonicalHeaderKey(field))
		}
	}
	if len(headers) == 0 {
		return "User-Agent"
	}
	return strings.Join(headers, ", ")
}

// clientIP returns the client IP address. By default this is the connection's
// peer address, which a client cannot forge. Forwarding headers
// (X-Forwarded-For, X-Real-IP) are consulted only when trusted proxies are
// configured and the immediate peer is one of them; X-Forwarded-For is then
// walked from the right, skipping trusted hops, so the returned address is the
// client as seen by the trusted edge rather than a value the client supplied.
func (m *Middleware) clientIP(r *http.Request) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	if len(m.trustedProxies) == 0 || !m.isTrustedProxy(peer) {
		return peer
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(parts[i])
			if ip != "" && !m.isTrustedProxy(ip) {
				return ip
			}
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		return xr
	}
	return peer
}

// isTrustedProxy reports whether ip falls within a configured trusted-proxy range.
func (m *Middleware) isTrustedProxy(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, n := range m.trustedProxies {
		if n.Contains(parsed) {
			return true
		}
	}
	return false
}

// maxFormBodyBytes caps how much of a urlencoded form body is buffered for
// evidence, mirroring net/http's own ~10 MB limit for urlencoded forms.
const maxFormBodyBytes = 10 << 20

// hasFormBody reports whether ParseForm would read this request's body: a
// POST/PUT/PATCH carrying a urlencoded form.
func hasFormBody(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return false
	}
	if r.Body == nil {
		return false
	}
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mt == "application/x-www-form-urlencoded"
}

// readCapped reads at most cap bytes from rc, reporting whether more data
// remained (overflow). The reader is not closed.
func readCapped(rc io.Reader, capBytes int64) ([]byte, bool) {
	data, _ := io.ReadAll(io.LimitReader(rc, capBytes+1))
	if int64(len(data)) > capBytes {
		return data, true
	}
	return data, false
}

// spliceBody rejoins already-read bytes with the unread remainder of the
// original body so the downstream handler sees the full stream.
func spliceBody(read []byte, rest io.ReadCloser) io.ReadCloser {
	return struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(read), rest), rest}
}

// requestProtocol returns the request protocol. The actual connection is
// trusted first (it cannot be spoofed). The forwarding headers are consulted
// only when the immediate peer is a configured trusted proxy - the same gate as
// clientIP - and only the literal values "http"/"https" are accepted, because
// the value becomes header.protocol evidence and reaches the client-side
// callback URL.
func (m *Middleware) requestProtocol(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	if len(m.trustedProxies) > 0 && m.isTrustedProxy(peer) {
		for _, name := range []string{"X-Origin-Proto", "X-Forwarded-Proto"} {
			proto := strings.ToLower(strings.TrimSpace(r.Header.Get(name)))
			if proto == "http" || proto == "https" {
				return proto
			}
		}
	}
	return "http"
}
