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

package derivedproperty

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

const bandYAML = `Format: 1
Name: Band
Version: 1.0.0
Output:
  Name: Band
  Description: A band.
  ValueType: string
  IsList: false
  Values:
    - { Name: Old }
    - { Name: New, Description: Recent. }
Checks:
  Recent: { Property: device.BrowserReleaseAge, Lt: 2.0 }
Rules:
  - When: { All: [ { Check: Recent }, { Property: device.IsCrawler, Eq: false } ] }
    Then: New
  - Else: Old
`

const bandJSON = `{"Format": 1, "Name": "Band", "Version": "1.0.0",
 "Output": {"Name": "Band", "Description": "A band.", "ValueType": "string", "IsList": false,
  "Values": [{"Name": "Old"}, {"Name": "New", "Description": "Recent."}]},
 "Checks": {"Recent": {"Property": "device.BrowserReleaseAge", "Lt": 2}},
 "Rules": [{"When": {"All": [{"Check": "Recent"}, {"Property": "device.IsCrawler", "Eq": false}]},
  "Then": "New"}, {"Else": "Old"}]}`

// recordingHandler keeps the message of every log record at or above level.
type recordingHandler struct {
	mu      sync.Mutex
	level   slog.Level
	records []slog.Record
}

func (h *recordingHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler           { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler                { return h }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *recordingHandler) messages(level slog.Level) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for _, r := range h.records {
		if r.Level == level {
			out = append(out, r.Message)
		}
	}
	return out
}

func mustValidate(t *testing.T, text string) *scriptModel {
	t.Helper()
	model, faults := validateText(text, "", "code")
	if len(faults) > 0 {
		t.Fatalf("the script did not validate:\n%s", (&ValidationError{Faults: faults}).Error())
	}
	return model
}

// A YAML script and the JSON that mirrors it give the same model, and the
// canonical form adds the inferred types and the computed dependencies.
func TestYAMLAndJSONGiveTheSameModel(t *testing.T) {
	fromYAML := mustValidate(t, bandYAML).canonical()
	fromJSON := mustValidate(t, bandJSON).canonical()
	if fromYAML != fromJSON {
		t.Errorf("YAML gave\n%s\nand JSON gave\n%s", fromYAML, fromJSON)
	}
	for _, want := range []string{
		"\"device.BrowserReleaseAge\": {\n      \"Type\": \"int\"\n    }",
		"\"Dependencies\": [\n      \"device.BrowserReleaseAge\",\n      \"device.IsCrawler\"\n    ]",
		"\"Lt\": 2\n",
	} {
		if !strings.Contains(fromYAML, want) {
			t.Errorf("the canonical form lacks %q:\n%s", want, fromYAML)
		}
	}
}

// 2 and 2.0 are both whole numbers, so they infer the same type and do not
// conflict.
func TestWholeNumbersInferInt(t *testing.T) {
	model := mustValidate(t, strings.Replace(bandYAML, "Checks:\n",
		"Checks:\n  Young: { Property: device.BrowserReleaseAge, Lt: 2 }\n", 1))
	if got := model.properties[0].typ; got != typeInt {
		t.Errorf("device.BrowserReleaseAge infers %s, want int", got)
	}
}

// Every fault in every script is reported at once, one line each.
func TestBuildReportsEveryFault(t *testing.T) {
	_, err := NewBuilder().
		AddScript("First", strings.Replace(bandYAML, "Name: Band\nVersion", "Name: First\nVersion: x\nV", 1)).
		AddScript("Second", strings.Replace(bandYAML, "Format: 1", "Format: 2", 1)).
		AddBuiltInScript("NoSuchScript").
		Build()
	var invalid *ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("Build gave %v, want a ValidationError", err)
	}
	message := invalid.Error()
	for _, want := range []string{"First (code)", "Second (code)", "Format must be 1, found 2",
		"'NoSuchScript' is not a built-in script"} {
		if !strings.Contains(message, want) {
			t.Errorf("the faults lack %q:\n%s", want, message)
		}
	}
	if lines := strings.Count(message, "\n") + 1; lines != len(invalid.Faults) {
		t.Errorf("%d lines for %d faults", lines, len(invalid.Faults))
	}
}

func TestBuildNeedsAScript(t *testing.T) {
	if _, err := NewBuilder().Build(); err == nil {
		t.Error("an element with no script was built")
	}
}

func TestTwoScriptsWithOneOutputAreRefused(t *testing.T) {
	_, err := NewBuilder().AddScript("Band", bandYAML).AddScript("Band", bandJSON).Build()
	if err == nil || !strings.Contains(err.Error(), "is added more than once") ||
		!strings.Contains(err.Error(), "is also produced by the script 'Band'") {
		t.Errorf("Build gave %v", err)
	}
}

func humanConfidence(t *testing.T, opts ...func(*Builder)) *Element {
	t.Helper()
	b := NewBuilder().AddBuiltInScript(HumanConfidence)
	for _, opt := range opts {
		opt(b)
	}
	e, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// humanValues is a request HumanConfidence reads as High.
var humanValues = map[string]map[string]any{
	"device": {"IsCrawler": false, "HasWebDriver": "False", "IsHeadless": false, "IsVisible": "True",
		"BrowserReleaseYear": 2026, "BrowserReleaseAge": 1, "BrowserPreviewYear": 0, "BrowserPreviewAge": 0},
	"ip": {"HumanProbability": 9, "ConnectionType": "Broadband"},
}

// humanDevice lists the device properties HumanConfidence reads.
var humanDevice = []string{"IsCrawler", "HasWebDriver", "IsHeadless", "IsVisible", "BrowserReleaseYear",
	"BrowserReleaseAge", "BrowserPreviewYear", "BrowserPreviewAge"}

// The Pipeline check names a source property no element supplies, a key no
// element has, and a derived property two elements produce.
func TestPipelineCheck(t *testing.T) {
	device := humanDevice
	for _, test := range []struct {
		name     string
		elements func(e *Element) []core.FlowElement
		want     string
	}{
		{"no element has the key", func(e *Element) []core.FlowElement {
			return []core.FlowElement{newStubSource("device", device, nil), e}
		}, "no element in the pipeline has the element data key 'ip'"},
		{"the property is not listed", func(e *Element) []core.FlowElement {
			return []core.FlowElement{newStubSource("device", device, nil),
				newStubSource("ip", []string{"CountryCode"}, nil), e}
		}, "no element under 'ip' lists the property 'HumanProbability'"},
		{"two elements produce one property", func(e *Element) []core.FlowElement {
			return []core.FlowElement{newStubSource("device", device, nil),
				newStubSource("ip", []string{"HumanProbability", "ConnectionType"}, nil), e,
				humanConfidence(t)}
		}, "two Derived Property Elements in the pipeline produce the derived property 'HumanConfidence'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := core.NewPipelineBuilder()
			for _, element := range test.elements(humanConfidence(t)) {
				b.AddFlowElement(element)
			}
			_, err := b.Build()
			if err == nil || !strings.Contains(errors.Unwrap(err).Error(), test.want) {
				t.Errorf("Build gave %v, want an error saying %q", err, test.want)
			}
		})
	}
}

// Sources that run in a parallel group are listed after the elements at the
// top level, and the check still finds them.
func TestPipelineCheckFindsAParallelGroup(t *testing.T) {
	e := humanConfidence(t)
	stubs := sourcesFor(e, humanValues)
	p, err := core.NewPipelineBuilder().AddParallel(stubs...).AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("the pipeline was not built: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*Data](fd)
	if err != nil {
		t.Fatal(err)
	}
	value, err := GetAs[engines.AspectPropertyValue[string]](data, "HumanConfidence")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := value.Value(); err != nil || got != "High" {
		t.Errorf("HumanConfidence is %q, %v, want High", got, err)
	}
}

// A request with no Element Data under a key a script reads gives no value,
// and the error is logged once for the script.
func TestNoElementDataIsLoggedOnce(t *testing.T) {
	logs := &recordingHandler{level: slog.LevelInfo}
	e := humanConfidence(t, func(b *Builder) { b.SetLogger(slog.New(logs)) })
	stubs := sourcesFor(e, humanValues)
	for _, stub := range stubs {
		if stub.DataKey() == "ip" {
			stub.(*stubSource).addData = false
		}
	}
	b := core.NewPipelineBuilder()
	for _, stub := range stubs {
		b.AddFlowElement(stub)
	}
	p, err := b.AddFlowElement(e).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for i := 0; i < 3; i++ {
		fd := p.CreateFlowData()
		if err := fd.Process(); err != nil {
			t.Fatal(err)
		}
		data, _ := core.Get[*Data](fd)
		value, _ := GetAs[engines.AspectPropertyValue[string]](data, "HumanConfidence")
		if value.HasValue() || !strings.Contains(value.NoValueMessage(),
			"'ip.HumanProbability' (element 'ip' has no value for 'HumanProbability': "+notPresent+")") {
			t.Errorf("request %d gave %v, %q", i, value.HasValue(), value.NoValueMessage())
		}
	}
	errs := logs.messages(slog.LevelError)
	if len(errs) != 1 || !strings.Contains(errs[0], "reads 'ip.HumanProbability'") ||
		!strings.Contains(errs[0], "after the element that supplies 'ip'") {
		t.Errorf("errors logged: %q", errs)
	}
}

const mobileYAML = `Format: 1
Name: Mobile
Version: 1.0.0
Output: { Name: Mobile, Description: Whether the device is mobile., ValueType: bool, IsList: false }
Rules:
  - When: { Property: device.IsMobile, Eq: true }
    Then: true
  - Else: false
`

const kindYAML = `Format: 1
Name: Kind
Version: 1.0.0
Output: { Name: Kind, Description: The kind of device., ValueType: string, IsList: false }
Rules:
  - When: { Property: derived.Mobile, Eq: true }
    Then: Phone
  - Else: Other
`

// A script can read the property another script in the same element
// produces, under the key derived.
func TestDerivedFromDerived(t *testing.T) {
	e, err := NewBuilder().AddScript("Mobile", mobileYAML).AddScript("Kind", kindYAML).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := processWith(t, e, map[string]map[string]any{"device": {"IsMobile": "True"}}, "Kind")
	if !got.HasValue() || got.AsInterface() != "Phone" {
		t.Errorf("Kind is %v, %q", got.AsInterface(), got.NoValueMessage())
	}
}

// The element accepts no evidence, writes AspectPropertyValues and exposes
// each script's metadata.
func TestMetadata(t *testing.T) {
	e := humanConfidence(t)
	if keys := e.EvidenceKeyFilter(); keys.Include("header.user-agent") {
		t.Error("the element accepts evidence")
	}
	props := e.Properties()
	if len(props) != 1 || props[0].Name != "HumanConfidence" || props[0].Category != "Human Detection" ||
		props[0].Type != reflect.TypeOf(engines.AspectPropertyValue[string]{}) || !props[0].Available {
		t.Errorf("properties %+v", props)
	}
	scripts := e.Scripts()
	if len(scripts) != 1 || scripts[0].Source != "HumanConfidence" || scripts[0].Format != 1 {
		t.Fatalf("scripts %+v", scripts)
	}
	var values []string
	for _, v := range scripts[0].Output.Values {
		values = append(values, v.Name)
	}
	if strings.Join(values, ",") != "High,Medium,Low,Unknown" {
		t.Errorf("values %v", values)
	}
	if len(scripts[0].Output.Dependencies) != 10 {
		t.Errorf("dependencies %v", scripts[0].Output.Dependencies)
	}
}

// The build logs one line per script, the compiled model at debug level and
// a warning carrying a deprecated script's note.
func TestBuildLogs(t *testing.T) {
	logs := &recordingHandler{level: slog.LevelDebug}
	deprecated := strings.Replace(bandYAML, "Version: 1.0.0\n",
		"Version: 1.0.0\nDeprecated: true\nDeprecationNote: Use HumanConfidence.\n", 1)
	_, err := NewBuilder().SetLogger(slog.New(logs)).
		AddBuiltInScript(HumanConfidence).AddScript("Band", deprecated).Build()
	if err != nil {
		t.Fatal(err)
	}
	info := logs.messages(slog.LevelInfo)
	if len(info) != 2 || !strings.Contains(info[0], "'HumanConfidence' version 0.8.0, format 1, from "+
		"HumanConfidence, produces 'derived.HumanConfidence'") {
		t.Errorf("information lines %q", info)
	}
	debug := logs.messages(slog.LevelDebug)
	if len(debug) != 2 || !strings.Contains(debug[0], "\"Name\": \"HumanConfidence\"") {
		t.Errorf("debug lines %q", debug)
	}
	warn := logs.messages(slog.LevelWarn)
	if len(warn) != 1 || !strings.Contains(warn[0], "Use HumanConfidence.") {
		t.Errorf("warnings %q", warn)
	}
}

// A Pipeline configuration names built-in scripts and script files, and an
// unknown build parameter is refused.
func TestConfiguration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Band.yaml"), []byte(bandYAML), 0644); err != nil {
		t.Fatal(err)
	}
	element, err := NewFromConfiguration(map[string]any{
		"Scripts":     "HumanConfidence, IsGdpr",
		"scriptfiles": filepath.Join(dir, "*.yaml"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(element.(*Element).Scripts()); got != 3 {
		t.Errorf("%d scripts, want 3", got)
	}
	if _, err := NewFromConfiguration(map[string]any{"Script": "HumanConfidence"}, nil); err == nil {
		t.Error("an unknown build parameter was accepted")
	}
	if _, err := NewFromConfiguration(map[string]any{"ScriptFiles": filepath.Join(dir, "*.json")}, nil); err == nil ||
		!strings.Contains(err.Error(), "no file matches") {
		t.Errorf("a pattern matching no file gave %v", err)
	}
	registry := core.NewBuilderRegistry()
	registry.Register(BuilderName, NewFromConfiguration)
}
