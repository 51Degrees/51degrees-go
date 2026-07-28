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
	"strings"
	"testing"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/jsonbuilder"
	"github.com/51Degrees/pipeline-go/elements/sequence"
)

func TestMustacheVariables(t *testing.T) {
	// All variable forms render raw (no HTML escaping), matching the .NET
	// SkipHtmlEncoding behavior, because the template produces JavaScript.
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
// and the .NET reference only accepts a single identifier.
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
// cannot reach the callback URL literal: the assembled URL is parsed and only
// http/https survive, matching the .NET reference which drops a URL that new
// Uri rejects.
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
// of its JS string literal; the sequence element replaces it with a GUID.
func TestSessionIDInjectionRejected(t *testing.T) {
	js := renderWith(t, map[string]string{
		"header.host":      "example.com",
		"query.session-id": `"; alert('xss'); var z="`,
	})
	// The session id reaches a JS string literal; that line must not carry the
	// payload (the sequence element replaces it with a GUID).
	line := grepLine(js, "var sessionId =")
	if strings.Contains(line, "alert") {
		t.Fatalf("session-id payload reached the sessionId literal: %q", line)
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

// TestFetchDefaultsToXHR verifies that, matching .NET, the script uses the
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
	line := grepLine(js, "var parameters =")
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
