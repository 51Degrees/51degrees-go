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
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"go.yaml.in/yaml/v3"
)

// The case files of the derived-properties repository, which every language
// runs, so the answers are checked to match case for case.
// See https://github.com/51Degrees/derived-properties/blob/cc30638632050795d9acc80d5118a290ffaba085/docs/testing.md

// sharedFolder returns the derived-properties submodule. CI checks it out, so
// a missing submodule fails there and skips elsewhere.
func sharedFolder(t *testing.T) string {
	t.Helper()
	folder := filepath.Join("..", "..", "..", "derived-properties")
	if _, err := os.Stat(filepath.Join(folder, "tests")); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("the derived-properties submodule is missing: %v", err)
		}
		t.Skipf("run 'git submodule update --init derived-properties' to run this test: %v", err)
	}
	return folder
}

type caseFile struct {
	Script string `yaml:"Script"`
	Cases  []struct {
		Name       string               `yaml:"Name"`
		Properties map[string]yaml.Node `yaml:"Properties"`
		Expect     struct {
			// Value is a zero node where the case expects no value.
			Value   yaml.Node `yaml:"Value"`
			Missing []string  `yaml:"Missing"`
		} `yaml:"Expect"`
	} `yaml:"Cases"`
}

// stubSource supplies values under one key and lists every property a script
// reads under it, as an engine lists the properties it can supply.
type stubSource struct {
	*core.BaseElement
	values  map[string]any
	addData bool
}

func newStubSource(key string, names []string, values map[string]any) *stubSource {
	s := &stubSource{BaseElement: core.NewBaseElement(key, nil, nil), values: values, addData: true}
	props := make([]core.PropertyMetadata, len(names))
	for i, name := range names {
		props[i] = core.PropertyMetadata{Name: name, Element: s, Available: true}
	}
	s.SetProperties(props)
	return s
}

func (s *stubSource) Process(fd core.FlowData) error {
	if !s.addData {
		return nil
	}
	data := fd.GetOrAdd(s.DataKey(), func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	for name, value := range s.values {
		data.Set(name, value)
	}
	return nil
}

// sourcesFor returns a stub for each key the element reads, holding the
// values given for it.
func sourcesFor(e *Element, values map[string]map[string]any) []core.FlowElement {
	names := map[string][]string{}
	for _, r := range e.reads {
		if r.key >= 0 {
			key := r.property.elementKey()
			names[key] = append(names[key], r.property.propertyName())
		}
	}
	var stubs []core.FlowElement
	for _, key := range e.keys {
		stubs = append(stubs, newStubSource(key, names[key], values[key]))
	}
	return stubs
}

// processWith runs one request through stubs holding the values and the
// element, and returns what the element wrote for the output property.
func processWith(t *testing.T, e *Element, values map[string]map[string]any,
	output string) engines.PropertyValueAccessor {
	t.Helper()
	b := core.NewPipelineBuilder()
	for _, stub := range sourcesFor(e, values) {
		b.AddFlowElement(stub)
	}
	p, err := b.AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("the pipeline was not built: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("processing failed: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("no derived element data: %v", err)
	}
	raw, err := data.Get(output)
	if err != nil {
		t.Fatalf("no value for %s: %v", output, err)
	}
	value, ok := raw.(engines.PropertyValueAccessor)
	if !ok {
		t.Fatalf("%s is %T, not an AspectPropertyValue", output, raw)
	}
	return value
}

// caseValue gives a value from a case file as a source element holds it. The
// String form is the string, NoValue is a value with no value, and a list of
// Value and Weight pairs is a list of weighted values.
func caseValue(t *testing.T, n yaml.Node, wrap bool) any {
	t.Helper()
	switch n.Kind {
	case yaml.ScalarNode:
		var value any
		switch n.ShortTag() {
		case "!!null":
			return nil
		case "!!bool":
			var b bool
			_ = n.Decode(&b)
			value = b
		case "!!int":
			var i int
			_ = n.Decode(&i)
			value = i
		case "!!float":
			var f float64
			_ = n.Decode(&f)
			value = f
		default:
			value = n.Value
		}
		if wrap {
			return engines.NewAspectPropertyValue[any](value)
		}
		return value
	case yaml.MappingNode:
		var m map[string]yaml.Node
		if err := n.Decode(&m); err != nil {
			t.Fatal(err)
		}
		if v, ok := m["NoValue"]; ok {
			return engines.NoValue[string](v.Value)
		}
		if v, ok := m["String"]; ok {
			if v.ShortTag() == "!!null" {
				return nil
			}
			if wrap {
				return engines.NewAspectPropertyValue[any](v.Value)
			}
			return v.Value
		}
		return m
	case yaml.SequenceNode:
		// A list whose members are not all weighted values cannot be read
		// where one value is needed, whatever it holds.
		// A case gives weights of any size, and a raw weighting is 16 bits,
		// so each weight is scaled against the largest, which keeps the order.
		var values []any
		var weights []float64
		largest := 0.0
		for _, item := range n.Content {
			var m map[string]yaml.Node
			if item.Kind != yaml.MappingNode || item.Decode(&m) != nil {
				return make([]any, len(n.Content))
			}
			v, ok := m["Value"]
			if !ok {
				return make([]any, len(n.Content))
			}
			var w float64
			if wn, ok := m["Weight"]; ok {
				_ = wn.Decode(&w)
			}
			values = append(values, caseValue(t, v, false))
			weights = append(weights, w)
			largest = math.Max(largest, w)
		}
		var weighted []core.WeightedValue[any]
		for i, value := range values {
			raw := uint16(0)
			if largest > 0 && weights[i] > 0 {
				raw = uint16(math.Round(weights[i] / largest * core.MaxRawWeighting))
			}
			weighted = append(weighted, core.NewWeightedValue[any](value, raw))
		}
		if len(weighted) == 0 {
			return []any{}
		}
		return weighted
	}
	return nil
}

// valueText is the text of a value, as the case files compare values.
func valueText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	case float64:
		return jsNumber(x)
	}
	return fmt.Sprint(v)
}

// expectedText is the text of an expected value in a case file.
func expectedText(n *yaml.Node) string {
	switch n.ShortTag() {
	case "!!int", "!!float":
		f, err := strconv.ParseFloat(n.Value, 64)
		if err == nil {
			return jsNumber(f)
		}
	}
	return n.Value
}

func TestSharedCases(t *testing.T) {
	folder := sharedFolder(t)
	files, _ := filepath.Glob(filepath.Join(folder, "tests", "*.cases.yaml"))
	if len(files) == 0 {
		t.Fatal("no case files in the derived-properties submodule")
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var cases caseFile
		if err := yaml.Unmarshal(data, &cases); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		e, err := NewBuilder().AddBuiltInScript(BuiltInScript(cases.Script)).Build()
		if err != nil {
			t.Fatalf("%s: the script %s did not build: %v", file, cases.Script, err)
		}
		output := e.scripts[0].model.output.name
		for _, wrap := range []bool{false, true} {
			for _, c := range cases.Cases {
				name := fmt.Sprintf("%s/%s/wrapped=%v", cases.Script, c.Name, wrap)
				t.Run(name, func(t *testing.T) {
					values := map[string]map[string]any{}
					for property, n := range c.Properties {
						key, prop, _ := strings.Cut(property, ".")
						key = strings.ToLower(key)
						if values[key] == nil {
							values[key] = map[string]any{}
						}
						values[key][prop] = caseValue(t, n, wrap)
					}
					got := processWith(t, e, values, output)
					var value *yaml.Node
					if c.Expect.Value.Kind != 0 {
						value = &c.Expect.Value
					}
					checkCase(t, got, value, c.Expect.Missing, output)
				})
			}
		}
	}
}

// checkCase compares what a script produced with a case's Expect. A missing
// value names every property that could not be read, so the message gives the
// count and each name.
func checkCase(t *testing.T, got engines.PropertyValueAccessor, value *yaml.Node, missing []string,
	output string) {
	t.Helper()
	if value != nil {
		if !got.HasValue() {
			t.Fatalf("expected '%s' but there was no value. %s", value.Value, got.NoValueMessage())
		}
		if text := valueText(got.AsInterface()); text != expectedText(value) {
			t.Errorf("expected '%s' but got '%s'", expectedText(value), text)
		}
		return
	}
	if got.HasValue() {
		t.Fatalf("expected no value naming %v but got '%v'", missing, got.AsInterface())
	}
	message := got.NoValueMessage()
	noun := "1 source property was not available"
	if len(missing) != 1 {
		noun = fmt.Sprintf("%d source properties were not available", len(missing))
	}
	prefix := fmt.Sprintf("Derived property '%s' has no value because %s. ", output, noun)
	if !strings.HasPrefix(message, prefix) {
		t.Errorf("message %q does not start %q", message, prefix)
	}
	lower := strings.ToLower(message)
	for _, name := range missing {
		if !strings.Contains(lower, "'"+strings.ToLower(name)+"' (") {
			t.Errorf("message %q does not name %s", message, name)
		}
	}
	if !strings.HasSuffix(message, usualCauses) {
		t.Errorf("message %q does not end with the usual causes", message)
	}
}

type rejectionCase struct {
	Name   string `yaml:"Name"`
	Script string `yaml:"Script"`
	Expect struct {
		Paths    []string `yaml:"Paths"`
		Mentions []string `yaml:"Mentions"`
	} `yaml:"Expect"`
}

func TestRejectedScripts(t *testing.T) {
	folder := sharedFolder(t)
	files, _ := filepath.Glob(filepath.Join(folder, "tests", "invalid", "*.yaml"))
	if len(files) == 0 {
		t.Fatal("no rejection cases in the derived-properties submodule")
	}
	sort.Strings(files)
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var c rejectionCase
			if err := yaml.Unmarshal(data, &c); err != nil {
				t.Fatal(err)
			}
			model, faults := validateText(c.Script, c.Name, filepath.Base(file))
			if model != nil || len(faults) == 0 {
				t.Fatal("the script validated, and the case says it must not")
			}
			all := (&ValidationError{Faults: faults}).Error()
			for _, path := range c.Expect.Paths {
				found := false
				for _, f := range faults {
					found = found || f.Path == path
				}
				if !found {
					t.Errorf("expected a fault at '%s'. Faults were:\n%s", path, all)
				}
			}
			for _, mention := range c.Expect.Mentions {
				found := false
				for _, f := range faults {
					found = found || strings.Contains(f.Message, mention)
				}
				if !found {
					t.Errorf("expected a fault mentioning '%s'. Faults were:\n%s", mention, all)
				}
			}
		})
	}
}

// The scripts this package ships are the scripts in the submodule, and every
// one of them has a BuiltInScript.
func TestBuiltInScriptsAreTheSubmodule(t *testing.T) {
	folder := sharedFolder(t)
	shipped, err := builtInFiles.ReadDir("scripts")
	if err != nil {
		t.Fatal(err)
	}
	inSubmodule, _ := filepath.Glob(filepath.Join(folder, "scripts", "*"))
	if len(shipped) != len(inSubmodule) {
		t.Errorf("%d scripts are shipped and the submodule has %d", len(shipped), len(inSubmodule))
	}
	for _, entry := range shipped {
		copied, _ := builtInFiles.ReadFile("scripts/" + entry.Name())
		original, err := os.ReadFile(filepath.Join(folder, "scripts", entry.Name()))
		if err != nil {
			t.Errorf("%s is not in the submodule: %v", entry.Name(), err)
			continue
		}
		if string(copied) != string(original) {
			t.Errorf("the shipped %s differs from the submodule's", entry.Name())
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		listed := false
		for _, s := range BuiltInScripts() {
			listed = listed || string(s) == name
		}
		if !listed {
			t.Errorf("%s has no BuiltInScript", name)
		}
	}
	for _, s := range BuiltInScripts() {
		if _, err := NewBuilder().AddBuiltInScript(s).Build(); err != nil {
			t.Errorf("%s does not build: %v", s, err)
		}
	}
}
