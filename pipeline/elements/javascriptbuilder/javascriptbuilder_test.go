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

package javascriptbuilder

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/jsonbuilder"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
)

func TestMustacheVariables(t *testing.T) {
	// All variable forms render raw (no HTML escaping), because the template
	// produces JavaScript, not HTML.
	tmpl := parseMustache(`Hello {{name}}, raw {{&raw}}, triple {{{trip}}}`)
	out := tmpl.render(map[string]any{"name": "<b>", "raw": "<b>", "trip": "<i>"})
	if out != "Hello <b>, raw <b>, triple <i>" {
		t.Fatalf("variables should render raw: %q", out)
	}
}

func TestMustacheSections(t *testing.T) {
	tmpl := parseMustache(`{{#on}}ON{{/on}}{{^on}}OFF{{/on}}`)
	if got := tmpl.render(map[string]any{"on": true}); got != "ON" {
		t.Fatalf("section true: %q", got)
	}
	if got := tmpl.render(map[string]any{"on": false}); got != "OFF" {
		t.Fatalf("inverted section: %q", got)
	}
}

func TestMustacheNestedSections(t *testing.T) {
	tmpl := parseMustache(`{{#a}}A{{#b}}B{{/b}}{{^b}}notB{{/b}}{{/a}}`)
	if got := tmpl.render(map[string]any{"a": true, "b": false}); got != "AnotB" {
		t.Fatalf("nested sections: %q", got)
	}
	if got := tmpl.render(map[string]any{"a": false, "b": true}); got != "" {
		t.Fatalf("outer false should suppress all: %q", got)
	}
}

func TestJavaScriptBuilderRender(t *testing.T) {
	p, err := core.NewPipelineBuilder().
		AddFlowElement(sequence.New(nil)).
		AddFlowElement(jsonbuilder.New(nil, nil)).
		AddFlowElement(NewBuilder().SetObjectName("myObj").SetEndpoint("/51dpipeline/json").Build()).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("header.host", "example.com")
	fd.AddEvidence("header.protocol", "https")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	js := data.(*Data).JavaScript()
	if !strings.Contains(js, "var myObj = new fiftyoneDegreesManager()") {
		t.Fatalf("object name not applied:\n%s", js[:min(len(js), 200)])
	}
	// URL should have been assembled from protocol/host/endpoint and used in the
	// update path.
	if !strings.Contains(js, "https://example.com/51dpipeline/json") {
		t.Fatalf("callback URL not present in script")
	}
	// JSON object embedded.
	if !strings.Contains(js, "var json = {") {
		t.Fatalf("JSON object not embedded")
	}
	// The numeric sequence must render as its literal value, not blank (which
	// would produce the syntax error "var sequence = ;").
	if !strings.Contains(js, "var sequence = 1;") {
		t.Fatalf("sequence not rendered as a number: %q", grepLine(js, "var sequence"))
	}
}

func TestMustacheStringifyNumeric(t *testing.T) {
	tmpl := parseMustache(`n={{&n}} f={{&f}}`)
	out := tmpl.render(map[string]any{"n": 7, "f": 3.5})
	if out != "n=7 f=3.5" {
		t.Fatalf("numeric stringify wrong: %q", out)
	}
}

func grepLine(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func TestCallbackURLNoDoubleSlash(t *testing.T) {
	p, err := core.NewPipelineBuilder().
		AddFlowElement(sequence.New(nil)).
		AddFlowElement(jsonbuilder.New(nil, nil)).
		AddFlowElement(NewBuilder().SetEndpoint("/51dpipeline/json").Build()).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.host", "example.com/") // trailing slash
	fd.AddEvidence("header.protocol", "https")
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	js := data.(*Data).JavaScript()
	if !strings.Contains(js, "https://example.com/51dpipeline/json") {
		t.Fatalf("expected single-slash callback URL")
	}
	if strings.Contains(js, "example.com//51dpipeline") {
		t.Fatalf("double slash in callback URL")
	}
}

func TestLightMinify(t *testing.T) {
	src := "line1\n   \n  // comment\nline2  \n"
	out := lightMinify(src)
	if strings.Contains(out, "// comment") {
		t.Fatalf("comment not removed: %q", out)
	}
	if strings.Contains(out, "line2  ") {
		t.Fatalf("trailing whitespace not trimmed: %q", out)
	}
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), out)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// renderWith builds the standard sequence+json+javascript pipeline, applies the
// given evidence, and returns the produced script.
func renderWith(t *testing.T, evidence map[string]string) string {
	t.Helper()
	p, err := core.NewPipelineBuilder().
		AddFlowElement(sequence.New(nil)).
		AddFlowElement(jsonbuilder.New(nil, nil)).
		AddFlowElement(NewBuilder().SetEndpoint("/51dpipeline/json").Build()).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.protocol", "https")
	for k, v := range evidence {
		fd.AddEvidence(k, v)
	}
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, _ := fd.Get(DataKey)
	return data.(*Data).JavaScript()
}

// TestObjectNameInjectionRejected verifies a hostile fod-js-object-name cannot
// reach the bare code position; the builder falls back to the default name.
func TestObjectNameInjectionRejected(t *testing.T) {
	js := renderWith(t, map[string]string{
		"header.host":              "example.com",
		"query.fod-js-object-name": "x = 1; alert('xss'); var y",
	})
	// The object name reaches a bare code position; that line must use the safe
	// default. (The payload is also echoed, JSON-escaped, in the _parameters
	// object, which is a safe string literal, so we assert on the code line.)
	line := grepLine(js, "new fiftyoneDegreesManager()")
	if !strings.Contains(line, "var "+DefaultObjectName+" = new fiftyoneDegreesManager()") {
		t.Fatalf("object declaration not safe: %q", line)
	}
}

// TestObjectNameValidAccepted verifies a valid single identifier is honored.
func TestObjectNameValidAccepted(t *testing.T) {
	js := renderWith(t, map[string]string{
		"header.host":              "example.com",
		"query.fod-js-object-name": "myFod",
	})
	if !strings.Contains(js, "var myFod = new fiftyoneDegreesManager()") {
		t.Fatalf("valid object name not applied")
	}
}

// TestObjectNameDottedRejected verifies a dotted path is rejected: the name
// renders at a declaration position where a dot is a JavaScript SyntaxError,
// so only a single identifier is accepted.
func TestObjectNameDottedRejected(t *testing.T) {
	js := renderWith(t, map[string]string{
		"header.host":              "example.com",
		"query.fod-js-object-name": "window.fod",
	})
	line := grepLine(js, "new fiftyoneDegreesManager()")
	if !strings.Contains(line, "var "+DefaultObjectName+" = new fiftyoneDegreesManager()") {
		t.Fatalf("dotted object name should fall back to the default: %q", line)
	}
}

// TestProtocolInjectionRejected verifies a hostile protocol evidence value
// cannot reach the callback URL literal: only http and https are accepted, so
// any other scheme disables the callback instead of being emitted.
func TestProtocolInjectionRejected(t *testing.T) {
	js := renderWith(t, map[string]string{
		"header.host":     "example.com",
		"header.protocol": "javascript'-alert(1)-'",
	})
	if strings.Contains(js, "alert(1)") {
		t.Fatalf("crafted protocol reached the generated script")
	}
}

// TestSessionIDInjectionRejected verifies a hostile session-id cannot break out
// of its JS string literal. The Sequence Element keeps any non-empty text as
// the session id, and the JavaScript Builder writes one that does not match
// sessionIDRe as an empty string.
func TestSessionIDInjectionRejected(t *testing.T) {
	js := renderWith(t, map[string]string{
		"header.host":      "example.com",
		"query.session-id": `"; alert('xss'); var z="`,
	})
	line := strings.TrimSpace(grepLine(js, "var sessionId ="))
	if want := `var sessionId = "";`; line != want {
		t.Fatalf("sessionId line = %q, want %q", line, want)
	}
}

// TestHostInjectionRejected verifies a hostile Host header cannot break out of
// the callback URL string literal; an invalid host suppresses the URL.
func TestHostInjectionRejected(t *testing.T) {
	js := renderWith(t, map[string]string{
		"header.host": "example.com'; alert('xss'); '",
	})
	if strings.Contains(js, "alert('xss')") {
		t.Fatalf("host payload reached the script:\n%s", grepLine(js, "alert"))
	}
}

// TestFetchDefaultsToXHR verifies the script uses the
// XMLHttpRequest fallback when no Fetch property is present.
func TestFetchDefaultsToXHR(t *testing.T) {
	js := renderWith(t, map[string]string{"header.host": "example.com"})
	if !strings.Contains(js, "createCORSRequest") {
		t.Fatalf("expected the XHR/CORS fallback path when fetch support is unknown")
	}
}

// TestParametersExcludeSessionAndSequence verifies the _parameters object omits
// session-id and sequence (added explicitly by the template) and keeps others.
func TestParametersExcludeSessionAndSequence(t *testing.T) {
	js := renderWith(t, map[string]string{
		"header.host":      "example.com",
		"query.session-id": "12345678-1234-4234-8234-1234567890ab",
		"query.myparam":    "myvalue",
	})
	line := grepLine(js, "var renderedParameters =")
	if !strings.Contains(line, "myparam") {
		t.Fatalf("parameters should include myparam: %q", line)
	}
	if strings.Contains(line, "session-id") || strings.Contains(line, "1234567890ab") {
		t.Fatalf("parameters should exclude the session id: %q", line)
	}
}

// TestMustacheMissingKeyInSection verifies a variable with no context value
// renders as empty inside a section rather than erroring.
func TestMustacheMissingKeyInSection(t *testing.T) {
	tmpl := parseMustache(`{{#on}}[{{missing}}]{{/on}}`)
	if got := tmpl.render(map[string]any{"on": true}); got != "[]" {
		t.Fatalf("missing key in section: %q", got)
	}
}

// TestMustacheUnbalancedSection verifies an unterminated section is handled
// best-effort (its unclosed content is dropped) rather than panicking.
func TestMustacheUnbalancedSection(t *testing.T) {
	tmpl := parseMustache(`a{{#on}}b`)
	if got := tmpl.render(map[string]any{"on": true}); got != "a" {
		t.Fatalf("unbalanced section: %q", got)
	}
}

// buildError returns the error from Factory and from building a pipeline that
// holds an element built with the given object name.
func buildError(name string) (factoryErr, pipelineErr error) {
	_, factoryErr = Factory(map[string]any{"ObjectName": name}, nil)
	p, pipelineErr := core.NewPipelineBuilder().
		AddFlowElement(NewBuilder().SetObjectName(name).Build()).
		Build()
	if p != nil {
		p.Close()
	}
	return factoryErr, pipelineErr
}

// A configured object name that is not valid, including an empty one, is
// refused when the element is built. A valid name matches
// ^[A-Za-z_$][A-Za-z0-9_$]*$ and is none of the names the object name
// section refuses, being the reserved words, including those reserved in
// strict mode, eval, arguments, Infinity, NaN, undefined, the browser's
// Window properties that a top level var cannot replace, location and
// fiftyoneDegreesManager.
func TestObjectNameConfiguredNotValid(t *testing.T) {
	for _, name := range []string{
		"x y", "", " ", " fod", "fod ", "fod\n", "1fod", "a.b", "a-b",
		"a'b", "é", "fod;alert(1)",
		"await", "break", "case", "catch", "class", "const", "continue",
		"debugger", "default", "delete", "do", "else", "enum", "export",
		"extends", "false", "finally", "for", "function", "if", "import",
		"in", "instanceof", "new", "null", "return", "super", "switch",
		"this", "throw", "true", "try", "typeof", "var", "void", "while",
		"with", "yield",
		"implements", "interface", "let", "package", "private",
		"protected", "public", "static",
		"Infinity", "NaN", "undefined", "fiftyoneDegreesManager",
		"eval", "arguments", "window", "document", "top", "name", "status",
		"history", "navigator", "closed", "location",
		"frameElement", "isSecureContext", "crossOriginIsolated",
		"trustedTypes", "crypto", "indexedDB", "localStorage",
		"sessionStorage", "caches", "cookieStore", "speechSynthesis",
		"styleMedia", "crashReport", "credentialless", "customElements",
		"documentPictureInPicture", "fence", "launchQueue",
		"originAgentCluster",
	} {
		t.Run(name, func(t *testing.T) {
			var ce *core.ConfigurationError
			factoryErr, pipelineErr := buildError(name)
			if !errors.As(factoryErr, &ce) {
				t.Errorf("Factory gave %v, want a configuration error", factoryErr)
			}
			if !errors.As(pipelineErr, &ce) {
				t.Errorf("pipeline build gave %v, want a configuration error",
					pipelineErr)
			}
		})
	}
}

// A valid configured object name is accepted and written into the script,
// and fod is used when no name is configured.
func TestObjectNameConfiguredValid(t *testing.T) {
	for _, c := range []struct {
		params map[string]any
		want   string
	}{
		{map[string]any{}, "fod"},
		{map[string]any{"ObjectName": "myObj"}, "myObj"},
		{map[string]any{"ObjectName": "_"}, "_"},
		{map[string]any{"ObjectName": "$"}, "$"},
		{map[string]any{"ObjectName": "$fod_1"}, "$fod_1"},
		{map[string]any{"ObjectName": "Let"}, "Let"},
		{map[string]any{"ObjectName": "async"}, "async"},
		{map[string]any{"ObjectName": "nan"}, "nan"},
	} {
		t.Run(c.want, func(t *testing.T) {
			el, err := Factory(c.params, nil)
			if err != nil {
				t.Fatalf("Factory: %v", err)
			}
			js := renderElement(t, el.(*Element), nil)
			line := grepLine(js, "new fiftyoneDegreesManager()")
			if !strings.Contains(line, "var "+c.want+" = new") {
				t.Errorf("object declaration %q, want the name %q", line, c.want)
			}
		})
	}
}

// Step 3, a requested object name that is not a valid object name is
// ignored in favour of the configured name.
func TestObjectNameRequestedNotValid(t *testing.T) {
	for _, name := range []string{
		"x y", "class", "let", "null", "NaN", "Infinity", "undefined",
		"fiftyoneDegreesManager", "eval", "arguments", "window", "document",
		"top", "name", "status", "history", "navigator", "closed",
		"location", "frameElement", "isSecureContext", "crossOriginIsolated",
		"trustedTypes", "crypto", "indexedDB", "localStorage",
		"sessionStorage", "caches", "cookieStore", "speechSynthesis",
		"styleMedia", "crashReport", "credentialless", "customElements",
		"documentPictureInPicture", "fence", "launchQueue",
		"originAgentCluster",
	} {
		t.Run(name, func(t *testing.T) {
			js := renderElement(t, NewBuilder().SetObjectName("configured").Build(),
				map[string]string{"query.fod-js-object-name": name})
			line := grepLine(js, "new fiftyoneDegreesManager()")
			if !strings.Contains(line, "var configured = new") {
				t.Errorf("object declaration %q, want the configured name", line)
			}
		})
	}
}

// The configuration error names the pattern a valid name must match, as
// café is a JavaScript identifier that the pattern refuses.
func TestObjectNameConfiguredErrorText(t *testing.T) {
	const want = "does not match ^[A-Za-z_$][A-Za-z0-9_$]*$"
	factoryErr, pipelineErr := buildError("café")
	for _, err := range []error{factoryErr, pipelineErr} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("gave %v, want it to say it %s", err, want)
		}
	}
}

// Step 3, a requested object name that is not valid is reported with a
// warning once per element, and the warning does not repeat the requested
// text. A blank or valid request is not reported.
func TestObjectNameRequestedNotValidLogged(t *testing.T) {
	const requested = "s3cr3t v4lue"
	t.Run("once per element", func(t *testing.T) {
		h := &recordingHandler{}
		logger := slog.New(h)
		first := NewBuilder().SetLogger(logger).SetObjectName("configured").Build()
		second := NewBuilder().SetLogger(logger).SetObjectName("configured").Build()
		for _, name := range []string{requested, "class", "eval"} {
			for _, e := range []*Element{first, second} {
				renderElement(t, e,
					map[string]string{EvidenceObjectName: name})
			}
		}
		records := h.records()
		if len(records) != 2 {
			t.Fatalf("logged %d records, want 2: %q", len(records), records)
		}
		for _, r := range records {
			if !strings.HasPrefix(r, "WARN ") {
				t.Errorf("logged %q, want a warning", r)
			}
			if strings.Contains(r, requested) {
				t.Errorf("logged %q, which repeats the requested name", r)
			}
		}
	})
	t.Run("not when blank or valid", func(t *testing.T) {
		for _, name := range []string{"", " ", "\t", "myFod"} {
			h := &recordingHandler{}
			renderElement(t, NewBuilder().SetLogger(slog.New(h)).Build(),
				map[string]string{EvidenceObjectName: name})
			if n := len(h.records()); n != 0 {
				t.Errorf("%q logged %d records, want none: %q",
					name, n, h.records())
			}
		}
	})
}

// Step 5, with no Sequence Element the script carries the sequence 1 and an
// empty session id.
func TestSequenceWithoutSequenceElement(t *testing.T) {
	p, err := core.NewPipelineBuilder().
		AddFlowElement(NewBuilder().SetEndpoint(DefaultEndpoint).Build()).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.host", "example.com")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	js := data.(*Data).JavaScript()
	if line := grepLine(js, "var sequence ="); !strings.Contains(line, "var sequence = 1;") {
		t.Errorf("sequence line %q, want var sequence = 1;", line)
	}
	if line := grepLine(js, "var sessionId ="); !strings.Contains(line, `var sessionId = "";`) {
		t.Errorf("session id line %q, want an empty session id", line)
	}
}

// sessionElement stands in for the Sequence Element under its data key,
// giving every request the same session id.
type sessionElement struct {
	*core.BaseElement
	sessionID string
}

func newSessionElement(sessionID string) *sessionElement {
	return &sessionElement{
		BaseElement: core.NewBaseElement(sequence.DataKey, nil, nil),
		sessionID:   sessionID,
	}
}

func (s *sessionElement) Process(fd core.FlowData) error {
	data := fd.GetOrAdd(sequence.DataKey,
		func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	data.Set(sequence.SessionIDProperty, s.sessionID)
	data.Set(sequence.SequenceProperty, 1)
	return nil
}

// Step 5, a session id that does not match ^[A-Za-z0-9-]{1,64}$ is written
// as an empty string, whichever element gave it, because the script holds
// it inside quotes.
func TestSessionID(t *testing.T) {
	longest := strings.Repeat("a", 64)
	for _, c := range []struct{ id, want string }{
		{"12345678-1234-4234-8234-1234567890ab",
			"12345678-1234-4234-8234-1234567890ab"},
		{"a", "a"},
		{"Session-1", "Session-1"},
		{longest, longest},
		{"", ""},
		{longest + "a", ""},
		{`"; alert(1); var z="`, ""},
		{"abc\n", ""},
		{"abc def", ""},
		{"abc_def", ""},
		{"a.b", ""},
		{`a\`, ""},
		{"é", ""},
		{"abc\u2028", ""},
	} {
		t.Run(c.id, func(t *testing.T) {
			p, err := core.NewPipelineBuilder().
				AddFlowElement(newSessionElement(c.id)).
				AddFlowElement(NewBuilder().Build()).
				Build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			defer p.Close()
			fd := p.CreateFlowData()
			if err := fd.Process(); err != nil {
				t.Fatalf("process: %v", err)
			}
			data, err := fd.Get(DataKey)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			want := `var sessionId = "` + c.want + `";`
			line := grepLine(data.(*Data).JavaScript(), "var sessionId =")
			if strings.TrimSpace(line) != want {
				t.Errorf("session id line %q, want %q", line, want)
			}
		})
	}
}

// The endpoint is empty unless one is set, as the configuration options say,
// so the script has no callback and nothing is logged. DefaultEndpoint can
// be passed to SetEndpoint.
func TestEndpointDefault(t *testing.T) {
	evidence := map[string]string{
		"header.protocol": "https",
		"header.host":     "example.com",
	}
	if DefaultEndpoint != "/51dpipeline/json" {
		t.Errorf("DefaultEndpoint is %q", DefaultEndpoint)
	}
	t.Run("NewBuilder", func(t *testing.T) {
		h := &recordingHandler{}
		e := NewBuilder().SetLogger(slog.New(h)).Build()
		if got := scriptURL(t, e, evidence); got != "" {
			t.Errorf("gave %q, want no callback", got)
		}
		if n := len(h.records()); n != 0 {
			t.Errorf("logged %d records, want none: %q", n, h.records())
		}
	})
	t.Run("New", func(t *testing.T) {
		if got := scriptURL(t, New(nil), evidence); got != "" {
			t.Errorf("gave %q, want no callback", got)
		}
	})
	t.Run("Factory", func(t *testing.T) {
		el, err := Factory(map[string]any{}, nil)
		if err != nil {
			t.Fatalf("Factory: %v", err)
		}
		if got := scriptURL(t, el.(*Element), evidence); got != "" {
			t.Errorf("gave %q, want no callback", got)
		}
	})
	t.Run("SetEndpoint(DefaultEndpoint)", func(t *testing.T) {
		e := NewBuilder().SetEndpoint(DefaultEndpoint).Build()
		want := "https://example.com/51dpipeline/json"
		if got := scriptURL(t, e, evidence); got != want {
			t.Errorf("gave %q, want %q", got, want)
		}
	})
}

// renderElement processes the evidence in a pipeline holding the sequence
// element, the JSON builder and the element, and returns the script.
func renderElement(t *testing.T, e *Element, evidence map[string]string) string {
	t.Helper()
	p, err := core.NewPipelineBuilder().
		AddFlowElement(sequence.New(nil)).
		AddFlowElement(jsonbuilder.New(nil, nil)).
		AddFlowElement(e).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	for k, v := range evidence {
		fd.AddEvidence(k, v)
	}
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	return data.(*Data).JavaScript()
}
