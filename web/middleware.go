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
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/javascriptbuilder"
	"github.com/51Degrees/pipeline-go/elements/jsonbuilder"
	"github.com/51Degrees/pipeline-go/elements/setheaders"
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
	pipeline      core.Pipeline
	javaScriptURL string
	jsonURL       string
	maxAge        time.Duration
}

// Option configures a Middleware.
type Option func(*Middleware)

// WithJavaScriptURL overrides the intercepted JavaScript endpoint path.
func WithJavaScriptURL(path string) Option { return func(m *Middleware) { m.javaScriptURL = path } }

// WithJSONURL overrides the intercepted JSON endpoint path.
func WithJSONURL(path string) Option { return func(m *Middleware) { m.jsonURL = path } }

// WithMaxAge sets the client cache lifetime for the intercepted endpoints.
func WithMaxAge(d time.Duration) Option { return func(m *Middleware) { m.maxAge = d } }

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
	populateEvidence(fd, r)
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
// cookies, query and form parameters, client IP, host and protocol.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/web-integration.md#populating-evidence
func populateEvidence(fd core.FlowData, r *http.Request) {
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

	_ = r.ParseForm()
	for name, values := range r.Form {
		if len(values) > 0 {
			fd.AddEvidence("query."+name, values[0])
		}
	}

	fd.AddEvidence("server.client-ip", clientIP(r))
	fd.AddEvidence("header.protocol", requestProtocol(r))
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
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d private", int(m.maxAge.Seconds())))
	if vary := m.varyHeader(); vary != "" {
		w.Header().Set("Vary", vary)
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Content-Type", contentType)

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

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func requestProtocol(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if proto := r.Header.Get("X-Origin-Proto"); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
