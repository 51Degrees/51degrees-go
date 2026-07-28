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

// Package javascriptbuilder implements the JavaScript Builder Element, which
// merges the JSON Builder output into a Mustache template to produce the
// client-side JavaScript used to access results and gather additional Evidence.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/javascript-builder.md
package javascriptbuilder

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/jsonbuilder"
	"github.com/51Degrees/pipeline-go/elements/sequence"
)

// The generated artifact is JavaScript, and the Mustache renderer deliberately
// never escapes, so client-influenced values that reach a code position must be
// validated here before rendering rather than relying on downstream escaping.
var (
	// jsObjectNameRe matches a single JavaScript identifier, matching the .NET
	// reference. A dotted path is rejected because the name renders at a
	// declaration position ("var <name> = ..."), where a dot is a SyntaxError.
	jsObjectNameRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)
	// hostRe matches a bare host or host:port (including bracketed IPv6); it
	// excludes any character that could break out of a JS string literal.
	hostRe = regexp.MustCompile(`^[A-Za-z0-9._:\-\[\]]+$`)
)

func validObjectName(s string) bool { return jsObjectNameRe.MatchString(s) }
func validHost(s string) bool       { return s != "" && hostRe.MatchString(s) }

//go:embed template.mustache
var templateSource string

// DataKey is the Element Data key for the JavaScript Builder Element.
const DataKey = "javascript"

// JavaScriptProperty is the Property holding the produced JavaScript string.
const JavaScriptProperty = "javascript"

// Default object name for the global client-side object.
const DefaultObjectName = "fod"

// Evidence keys consumed by the element.
const (
	EvidenceHost          = "header.host"
	EvidenceProtocol      = "header.protocol"
	EvidenceObjectName    = "query.fod-js-object-name"
	EvidenceEnableCookies = "query.fod-js-enable-cookies"
)

// Element is the JavaScript Builder Element.
type Element struct {
	*core.BaseElement
	tmpl *mustacheTemplate

	protocol      string
	host          string
	endpoint      string
	objectName    string
	minify        bool
	enableCookies bool
}

// Builder constructs a JavaScript Builder Element.
type Builder struct {
	logger        *slog.Logger
	protocol      string
	host          string
	endpoint      string
	objectName    string
	minify        bool
	enableCookies bool
}

// DefaultEndpoint is the callback endpoint used when one is not configured,
// matching the .NET reference so the client-side update round-trip is enabled by
// default.
const DefaultEndpoint = "/51dpipeline/json"

// NewBuilder creates a Builder with default options.
func NewBuilder() *Builder {
	return &Builder{
		objectName:    DefaultObjectName,
		endpoint:      DefaultEndpoint,
		minify:        true,
		enableCookies: true,
	}
}

// SetLogger sets the logger.
func (b *Builder) SetLogger(l *slog.Logger) *Builder { b.logger = l; return b }

// SetProtocol sets the callback protocol (otherwise taken from Evidence).
func (b *Builder) SetProtocol(p string) *Builder { b.protocol = p; return b }

// SetHost sets the callback host (otherwise taken from Evidence).
func (b *Builder) SetHost(h string) *Builder { b.host = h; return b }

// SetEndpoint sets the callback endpoint path.
func (b *Builder) SetEndpoint(e string) *Builder { b.endpoint = e; return b }

// SetObjectName sets the global JavaScript object name (default "fod").
func (b *Builder) SetObjectName(n string) *Builder { b.objectName = n; return b }

// SetMinify enables or disables minification (default true).
func (b *Builder) SetMinify(v bool) *Builder { b.minify = v; return b }

// SetEnableCookies controls whether results are retained in cookies (default
// true) or session storage.
func (b *Builder) SetEnableCookies(v bool) *Builder { b.enableCookies = v; return b }

// Build creates the JavaScript Builder Element.
func (b *Builder) Build() *Element {
	e := &Element{
		BaseElement: core.NewBaseElement(DataKey, core.NewWhitelistFilter(
			EvidenceHost, EvidenceProtocol, EvidenceObjectName, EvidenceEnableCookies), b.logger),
		tmpl:          parseMustache(templateSource),
		protocol:      b.protocol,
		host:          b.host,
		endpoint:      b.endpoint,
		objectName:    b.objectName,
		minify:        b.minify,
		enableCookies: b.enableCookies,
	}
	e.SetProperties([]core.PropertyMetadata{
		{Name: JavaScriptProperty, Element: e, Category: "JavaScript", Type: nil, Available: true},
	})
	return e
}

// New creates a JavaScript Builder Element with default options.
func New(logger *slog.Logger) *Element { return NewBuilder().SetLogger(logger).Build() }

// Data is the Element Data produced by the JavaScript Builder.
type Data struct {
	*core.BaseElementData
}

// JavaScript returns the produced JavaScript string.
func (d *Data) JavaScript() string {
	if v, err := d.Get(JavaScriptProperty); err == nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Process renders the client-side JavaScript from the JSON Builder output.
func (e *Element) Process(fd core.FlowData) error {
	jsonStr := e.jsonObject(fd)
	sessionID, seq := e.sessionInfo(fd)

	protocol := firstNonEmpty(e.protocol, evidenceString(fd, EvidenceProtocol), "https")
	host := strings.TrimRight(firstNonEmpty(e.host, evidenceString(fd, EvidenceHost)), "/")

	// The object name reaches a bare code position in the template. Honor the
	// client-supplied evidence value only if it is a valid JS identifier;
	// otherwise fall back to the configured/default name.
	objectName := firstNonEmpty(e.objectName, DefaultObjectName)
	if ev := evidenceString(fd, EvidenceObjectName); ev != "" && validObjectName(ev) {
		objectName = ev
	}

	// The callback URL reaches a JS string literal. Assemble it, then validate
	// the whole URL (matching the .NET reference, which parses the assembled
	// string with new Uri and drops it on failure) so that no client-influenced
	// component - protocol or host - can break out of the literal.
	callbackURL := ""
	if protocol != "" && host != "" && e.endpoint != "" {
		callbackURL = validCallbackURL(
			protocol+"://"+host+ensureLeadingSlash(e.endpoint), host)
	}

	ctx := map[string]any{
		"_objName":              objectName,
		"_jsonObject":           jsonStr,
		"_sessionId":            sessionID,
		"_sequence":             seq,
		"_parameters":           e.queryParameters(fd),
		"_url":                  callbackURL,
		"_updateEnabled":        callbackURL != "",
		"_enableCookies":        e.cookiesEnabled(fd),
		"_hasDelayedProperties": strings.Contains(jsonStr, "delayexecution"),
		"_supportsPromises":     e.supportsPromises(fd),
		"_supportsFetch":        e.supportsFetch(fd),
	}

	rendered := e.tmpl.render(ctx)
	if e.minify {
		rendered = lightMinify(rendered)
	}

	out := fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData {
		return &Data{BaseElementData: core.NewBaseElementData()}
	})
	out.Set(JavaScriptProperty, rendered)
	return nil
}

// Factory builds a JavaScript Builder Element from configuration.
func Factory(params map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	b := NewBuilder().SetLogger(logger)
	if v, ok := core.StringParam(params, "Protocol"); ok {
		b.SetProtocol(v)
	}
	if v, ok := core.StringParam(params, "Host"); ok {
		b.SetHost(v)
	}
	if v, ok := core.StringParam(params, "Endpoint"); ok {
		b.SetEndpoint(v)
	}
	if v, ok := core.StringParam(params, "ObjectName"); ok {
		b.SetObjectName(v)
	}
	if v, ok := core.BoolParam(params, "Minify"); ok {
		b.SetMinify(v)
	}
	if v, ok := core.BoolParam(params, "EnableCookies"); ok {
		b.SetEnableCookies(v)
	}
	return b.Build(), nil
}

// --- helpers -------------------------------------------------------------

func (e *Element) jsonObject(fd core.FlowData) string {
	data, err := fd.Get(jsonbuilder.DataKey)
	if err != nil {
		return "{}"
	}
	if v, err := data.Get(jsonbuilder.JSONProperty); err == nil {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "{}"
}

func (e *Element) sessionInfo(fd core.FlowData) (string, int) {
	data, err := fd.Get(sequence.DataKey)
	if err != nil {
		return "", 0
	}
	sessionID := ""
	if v, err := data.Get(sequence.SessionIDProperty); err == nil {
		if s, ok := v.(string); ok {
			sessionID = s
		}
	}
	seq := 0
	if v, err := data.Get(sequence.SequenceProperty); err == nil {
		if n, ok := v.(int); ok {
			seq = n
		}
	}
	return sessionID, seq
}

// queryParameters returns a JSON object of the query.* Evidence values (with the
// "query." prefix removed) for relaying in client-side callbacks. Matching the
// .NET reference, the session id and sequence are excluded (the template adds
// them explicitly) and both keys and values are URL-encoded so the client can
// append them to the callback URL directly.
func (e *Element) queryParameters(fd core.FlowData) string {
	params := map[string]string{}
	for key, val := range fd.Evidence().AsMap() {
		prefix, field := core.SplitEvidenceKey(key)
		if prefix != core.PrefixQuery {
			continue
		}
		if field == sequence.SessionIDProperty || field == sequence.SequenceProperty {
			continue
		}
		params[url.QueryEscape(field)] = url.QueryEscape(anyToString(val))
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func (e *Element) cookiesEnabled(fd core.FlowData) bool {
	if v := evidenceString(fd, EvidenceEnableCookies); v != "" {
		return strings.EqualFold(v, "true")
	}
	return e.enableCookies
}

// supportsPromises reports whether Device Detection is present and its "promise"
// Property reports "Full".
func (e *Element) supportsPromises(fd core.FlowData) bool {
	for _, data := range fd.ElementDataAsMap() {
		if v, err := data.Get("promise"); err == nil {
			if s, ok := v.(string); ok && strings.EqualFold(s, "Full") {
				return true
			}
		}
	}
	return false
}

// supportsFetch reports whether the client supports the Fetch API. If a "fetch"
// Property is present (e.g. from Device Detection) it is honored; otherwise it
// defaults to true. When false, the template emits the XMLHttpRequest fallback.
func (e *Element) supportsFetch(fd core.FlowData) bool {
	for _, data := range fd.ElementDataAsMap() {
		if v, err := data.Get("fetch"); err == nil {
			switch t := v.(type) {
			case bool:
				return t
			case string:
				return !strings.EqualFold(t, "false") && !strings.EqualFold(t, "none")
			}
		}
	}
	// Match the .NET reference: default to false when no Fetch property is
	// present, so the generated script uses the XMLHttpRequest fallback rather
	// than assuming fetch() is available.
	return false
}

// validCallbackURL parses the assembled callback URL and returns it only when
// it survives a round trip: http/https scheme, the expected (already
// hostRe-validated) host, and no characters that could break out of a JS
// string literal. Anything else returns "" and the callback is disabled,
// matching the .NET reference which drops the URL when new Uri throws.
func validCallbackURL(raw, host string) string {
	if !validHost(host) {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	if u.Host != host {
		return ""
	}
	if strings.ContainsAny(raw, `'"\<>`) {
		return ""
	}
	return raw
}

func evidenceString(fd core.FlowData, key string) string {
	if v, ok := fd.Evidence().Get(key); ok {
		return anyToString(v)
	}
	return ""
}

func anyToString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func ensureLeadingSlash(s string) string {
	if s == "" || strings.HasPrefix(s, "/") {
		return s
	}
	return "/" + s
}

// lightMinify performs a conservative reduction of the generated script: it
// drops blank lines and whole-line comments and trims trailing whitespace. It
// deliberately does NOT attempt to strip inline comments or collapse tokens, as
// that cannot be done safely without a full JavaScript parser. A heavier
// minifier can be applied externally if required.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/javascript-builder.md#process
func lightMinify(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		stripped := strings.TrimSpace(trimmed)
		if stripped == "" {
			continue
		}
		if strings.HasPrefix(stripped, "//") {
			continue
		}
		out = append(out, trimmed)
	}
	return strings.Join(out, "\n")
}
