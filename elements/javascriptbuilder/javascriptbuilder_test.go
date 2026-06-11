package javascriptbuilder

import (
	"strings"
	"testing"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/jsonbuilder"
	"github.com/51Degrees/pipeline-go/elements/sequence"
)

func TestMustacheVariables(t *testing.T) {
	tmpl := parseMustache(`Hello {{name}}, raw {{&raw}}, triple {{{trip}}}`)
	out := tmpl.render(map[string]any{"name": "<b>", "raw": "<b>", "trip": "<i>"})
	if !strings.Contains(out, "Hello &lt;b&gt;") {
		t.Fatalf("escaped variable not applied: %q", out)
	}
	if !strings.Contains(out, "raw <b>") || !strings.Contains(out, "triple <i>") {
		t.Fatalf("unescaped variables wrong: %q", out)
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
