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
// merges the JSON Builder output into javascript-templates' Mustache template
// to produce the client-side JavaScript used to access results and gather
// additional Evidence.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/javascript-builder.md
package javascriptbuilder

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/jsonbuilder"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
)

// The generated artifact is JavaScript, and the Mustache renderer deliberately
// never escapes, so client-influenced values that reach a code position must be
// validated here before rendering rather than relying on downstream escaping.
var (
	// jsObjectNameRe matches a single JavaScript identifier. A dotted path is
	// rejected because the name renders at a declaration position
	// ("var <name> = ..."), where a dot is a SyntaxError.
	jsObjectNameRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`)

	// reservedObjectNames are the names the object cannot have, as the
	// object name section of the specification lists them. They are the
	// reserved words of ECMA-262 section 12.7.2 and those reserved in strict
	// mode code, eval and arguments, which strict mode code cannot declare
	// and of which eval would replace the page's own eval function, the
	// globals Infinity, NaN and undefined and the browser's Window properties
	// from window to originAgentCluster below, none of which a top level var
	// can replace, the browser's location, where a top level var navigates
	// the page away, and the constructor the script defines.
	// See https://github.com/51Degrees/specifications/blob/a7ea4f07ade93162406263ad495df17631704670/pipeline-specification/pipeline-elements/javascript-builder.md#object-name
	reservedObjectNames = map[string]bool{
		"await": true, "break": true, "case": true, "catch": true,
		"class": true, "const": true, "continue": true, "debugger": true,
		"default": true, "delete": true, "do": true, "else": true,
		"enum": true, "export": true, "extends": true, "false": true,
		"finally": true, "for": true, "function": true, "if": true,
		"import": true, "in": true, "instanceof": true, "new": true,
		"null": true, "return": true, "super": true, "switch": true,
		"this": true, "throw": true, "true": true, "try": true,
		"typeof": true, "var": true, "void": true, "while": true,
		"with": true, "yield": true,
		"implements": true, "interface": true, "let": true, "package": true,
		"private": true, "protected": true, "public": true, "static": true,
		"eval": true, "arguments": true,
		"Infinity": true, "NaN": true, "undefined": true,
		"window": true, "document": true, "top": true, "name": true,
		"status": true, "history": true, "navigator": true, "closed": true,
		"frameElement": true, "isSecureContext": true,
		"crossOriginIsolated": true, "trustedTypes": true, "crypto": true,
		"indexedDB": true, "localStorage": true, "sessionStorage": true,
		"caches": true, "cookieStore": true, "speechSynthesis": true,
		"styleMedia": true, "crashReport": true, "credentialless": true,
		"customElements": true, "documentPictureInPicture": true,
		"fence": true, "launchQueue": true, "originAgentCluster": true,
		"location": true, "fiftyoneDegreesManager": true,
	}

	// sessionIDRe is the form a session id must have to be written into the
	// script, which holds it inside double quotes. Any other session id is
	// written as an empty string, as step 5 of the process says.
	sessionIDRe = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
)

// validObjectName reports whether s is a valid object name, as the object
// name section of the specification defines one.
func validObjectName(s string) bool {
	return jsObjectNameRe.MatchString(s) && !reservedObjectNames[s]
}

// templateSource is javascript-templates' JavaScriptResource.mustache, the
// template every 51Degrees language renders. The package holds a copy of the
// javascript-templates submodule's file because a Go module carries no
// submodule, and ci/check-javascript-templates.ps1 fails when they differ.
//
//go:embed javascript-templates/JavaScriptResource.mustache
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

	// configErr, when set, is returned by Factory and OnPipelineAdded.
	configErr error
	// refusedURLLogged is set once a refused callback URL has been logged.
	refusedURLLogged atomic.Bool
	// refusedNameLogged is set once an ignored requested object name has
	// been logged.
	refusedNameLogged atomic.Bool
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

// DefaultEndpoint is the path the web integration serves the JSON endpoint
// on unless told otherwise. The builder's endpoint is empty until
// SetEndpoint is called, which gives a script with no callback, so pass
// DefaultEndpoint to SetEndpoint for a callback to that path.
const DefaultEndpoint = "/51dpipeline/json"

// NewBuilder creates a Builder with default options. The endpoint is empty,
// so the script has no callback until SetEndpoint is called.
func NewBuilder() *Builder {
	return &Builder{
		objectName:    DefaultObjectName,
		minify:        true,
		enableCookies: true,
	}
}

// SetLogger sets the logger.
func (b *Builder) SetLogger(l *slog.Logger) *Builder { b.logger = l; return b }

// SetProtocol sets the callback protocol (otherwise taken from Evidence). It
// must be empty, or http or https ignoring ASCII case. Any other value makes
// Factory return a configuration error and the built element refuse every
// pipeline.
func (b *Builder) SetProtocol(p string) *Builder { b.protocol = p; return b }

// SetHost sets the callback host (otherwise taken from Evidence).
func (b *Builder) SetHost(h string) *Builder { b.host = h; return b }

// SetEndpoint sets the callback endpoint path (default empty, which gives no
// callback). DefaultEndpoint is the path the web integration serves.
func (b *Builder) SetEndpoint(e string) *Builder { b.endpoint = e; return b }

// SetObjectName sets the global JavaScript object name (default "fod"). It
// must match ^[A-Za-z_$][A-Za-z0-9_$]*$ and not be a JavaScript reserved
// word, eval, arguments, Infinity, NaN, undefined, window, document, top,
// name, status, history, navigator, closed, frameElement, isSecureContext,
// crossOriginIsolated, trustedTypes, crypto, indexedDB, localStorage,
// sessionStorage, caches, cookieStore, speechSynthesis, styleMedia,
// crashReport, credentialless, customElements, documentPictureInPicture,
// fence, launchQueue, originAgentCluster, location or
// fiftyoneDegreesManager. Any other value, an empty one included, makes
// Factory return a configuration error and the built element refuse every
// pipeline.
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
	var problems []string
	if b.protocol != "" && !isScheme(b.protocol) {
		problems = append(problems, "protocol "+strconv.Quote(b.protocol)+
			" is not http or https")
	}
	if !validObjectName(b.objectName) {
		problems = append(problems, "object name "+
			strconv.Quote(b.objectName)+" does not match "+
			"^[A-Za-z_$][A-Za-z0-9_$]*$, or is a JavaScript reserved word "+
			"or another name the object cannot have, as SetObjectName "+
			"lists them")
	}
	if len(problems) > 0 {
		e.configErr = core.NewConfigurationError("the JavaScript builder's "+
			strings.Join(problems, ", and its "), nil)
	}
	e.SetProperties([]core.PropertyMetadata{
		{Name: JavaScriptProperty, Element: e, Category: "JavaScript", Type: nil, Available: true},
	})
	return e
}

// New creates a JavaScript Builder Element with default options, which give
// a script with no callback because the endpoint is empty.
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
	host := firstNonEmpty(e.host, evidenceString(fd, EvidenceHost))

	// The object name reaches a bare code position in the template. Honor the
	// client-supplied evidence value only if it is a valid object name, and
	// otherwise use the configured name, which Build has checked. A name that
	// is not valid is logged once, without its text, so that a visitor can
	// neither fill the log nor write into it.
	objectName := e.objectName
	if ev := evidenceString(fd, EvidenceObjectName); !isBlank(ev) {
		if validObjectName(ev) {
			objectName = ev
		} else if !e.refusedNameLogged.Swap(true) {
			e.Logger().Warn("javascript builder: the requested object name "+
				"in the query.fod-js-object-name evidence is not a valid "+
				"object name, so the configured name is used. This is "+
				"reported once.", "configured", e.objectName)
		}
	}

	// The callback URL reaches a JS string literal, and its protocol and host
	// can come from the request, so a URL that is not valid is left out. It
	// is logged once, with line breaks and other control characters escaped
	// by strconv.Quote, whose outer quotes are left to the log handler.
	callbackURL, refused := buildCallbackURL(protocol, host, e.endpoint)
	if refused != "" && !e.refusedURLLogged.Swap(true) {
		quoted := strconv.Quote(refused)
		e.Logger().Error("javascript builder: the callback URL is not valid, "+
			"so the script has no callback. Check the configured protocol, "+
			"host and endpoint, and the header.protocol and header.host "+
			"evidence. This is reported once.",
			"url", quoted[1:len(quoted)-1])
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
		// The block that gathers the visitor's answer, which a 51Did is
		// made from, from the 51Degrees Preference Management Platform
		// (PMP) or the Transparency and Consent Framework. Go has no
		// 51Did element, so the block is left out.
		"_userPrompt": false,
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
	e := b.Build()
	if e.configErr != nil {
		return nil, e.configErr
	}
	return e, nil
}

// OnPipelineAdded refuses a pipeline when the element's configuration is not
// valid, because Build returns no error.
func (e *Element) OnPipelineAdded(p core.Pipeline) error {
	if e.configErr != nil {
		return e.configErr
	}
	return e.BaseElement.OnPipelineAdded(p)
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

// noSequence is the sequence written when no Sequence Element ran, being
// the number the Sequence Element gives a session's first request.
const noSequence = 1

// sessionInfo returns the session id and sequence from the Sequence
// Element's data, or an empty session id and noSequence without it. A
// session id that does not match sessionIDRe is returned as empty, because
// whichever element wrote it under that data key may not have checked it.
func (e *Element) sessionInfo(fd core.FlowData) (string, int) {
	data, err := fd.Get(sequence.DataKey)
	if err != nil {
		return "", noSequence
	}
	sessionID := ""
	if v, err := data.Get(sequence.SessionIDProperty); err == nil {
		if s, ok := v.(string); ok && sessionIDRe.MatchString(s) {
			sessionID = s
		}
	}
	seq := noSequence
	if v, err := data.Get(sequence.SequenceProperty); err == nil {
		if n, ok := v.(int); ok {
			seq = n
		}
	}
	return sessionID, seq
}

// queryParameters returns a JSON object of the query.* Evidence values (with the
// "query." prefix removed) for relaying in client-side callbacks. The session id
// and sequence are excluded (the template adds them explicitly) and both keys
// and values are URL-encoded so the client can append them to the callback URL
// directly.
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
	// Default to false when no Fetch property is present, so the generated
	// script uses the XMLHttpRequest fallback rather than assuming fetch() is
	// available.
	return false
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
