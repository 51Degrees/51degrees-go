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

package ipi_onpremise

// These tests follow the Engine rules for the data an IP Intelligence Engine
// gives, which bind the on-premise Engine used on its own, outside a
// Pipeline, as they bind it in one.

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// weightedValues is what WeightedValues gives for a Property of the Engine.
type weightedValues = engines.AspectPropertyValue[[]core.WeightedValue[any]]

// textValue is what Get gives for a Property the data file does not weight,
// as the IP Intelligence Cloud Engine gives a Property it does not weight.
type textValue = engines.AspectPropertyValue[string]

// cloudWeighted is what Get gives for a Property the data file weights, as
// the IP Intelligence Cloud Engine gives a weighted Property.
type cloudWeighted = engines.AspectPropertyValue[[]core.WeightedValue[string]]

// weightedData is a Data that also gives each Property as weighted values.
type weightedData interface {
	WeightedValues(property string) (weightedValues, error)
}

// aspectData gives the Data as Aspect Data, failing the test when it is not.
func aspectData(t *testing.T, data *Data) engines.AspectData {
	t.Helper()
	var v any = data
	a, ok := v.(engines.AspectData)
	if !ok {
		t.Fatalf("the Data ProcessData gives is a %T, which does not "+
			"implement Aspect Data", data)
	}
	return a
}

// weighted gives the Data as one that gives each Property as weighted
// values, failing the test when it does not.
func weighted(t *testing.T, data *Data) weightedData {
	t.Helper()
	var v any = data
	w, ok := v.(weightedData)
	if !ok {
		t.Fatalf("the Data ProcessData gives is a %T, which has no "+
			"WeightedValues, so a Property is not also given as weighted values",
			data)
	}
	return w
}

// propertyValues gets the named Property as weighted values, failing the
// test when it is missing.
func propertyValues(t *testing.T, w weightedData, name string) weightedValues {
	t.Helper()
	values, err := w.WeightedValues(name)
	if err != nil {
		t.Fatalf("WeightedValues(%q) gave %v", name, err)
	}
	return values
}

// textOf gets the named Property from the Aspect Data, failing the test when
// it is missing or not text.
func textOf(t *testing.T, a engines.AspectData, name string) textValue {
	t.Helper()
	v, err := a.Get(name)
	if err != nil {
		t.Fatalf("Get(%q) gave %v", name, err)
	}
	text, ok := v.(textValue)
	if !ok {
		t.Fatalf("Get(%q) gave a %T, want a %T, as the cloud engine gives a "+
			"Property that is not weighted", name, v, textValue{})
	}
	return text
}

// cloudText is the text the cloud engine gives a value as.
func cloudText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return fmt.Sprint(value)
}

// rawWeighting is the raw weighting of a weight from 0 to 1.
func rawWeighting(weight float64) uint16 {
	return uint16(math.Round(weight * core.MaxRawWeighting))
}

// All Engines MUST populate data objects that implement Aspect Data, and
// Element data returned by an IP Intelligence Engine MUST extend IAspectData.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/data-model.md?plain=1#L10-L12
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/data-model.md?plain=1#L34-L35
func TestDataIsAspectData(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	a := aspectData(t, data)
	if a.CacheHit() {
		t.Error("the Data says it came from a cache, which no Data made " +
			"by processing does")
	}
}

// Get gives each Property of the Engine, by its name in any case, as the IP
// Intelligence Cloud Engine gives it, so code reading one reads the other. A
// Property the data file does not weight is text. Get reads only that
// Property, and only when it is requested.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L96-L98
func TestGetGivesEachPropertyAsTheCloudEngineDoes(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	a := aspectData(t, data)

	for _, c := range []struct{ name, want string }{{"b", "b"}, {"C", "c"}} {
		got, err := textOf(t, a, c.name).Value()
		if err != nil {
			t.Fatalf("%s has no value: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s gave %q, want %q", c.name, got, c.want)
		}
	}
	if reads := results.readsMade(); !reflect.DeepEqual(reads, [][]int{{1}, {2}}) {
		t.Errorf("getting b then C read the positions %v, want [[1] [2]]", reads)
	}
}

// Get gives a Property the data file weights as the cloud engine gives a
// weighted Property, each value as text with its raw weighting, and no value
// with a message where the address has none.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/cloud-aspect-engine.md?plain=1#L24-L26
func TestGetGivesAWeightedPropertyAsTheCloudEngineDoes(t *testing.T) {
	e := &Engine{}
	e.setPositions([]string{"A", "C", "D"})
	e.weighted.Store(&map[string]bool{"c": true, "d": true})
	results := &countingResults{values: map[int][]*ipi_interop.WeightedValue{
		1: {{Value: "c", Weight: 0.5}, {Value: 1.25, Weight: 0.5}},
	}}
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	a := aspectData(t, data)
	got, err := a.Get("C")
	if err != nil {
		t.Fatal(err)
	}
	c, ok := got.(cloudWeighted)
	if !ok {
		t.Fatalf("C gave a %T, want a %T", got, cloudWeighted{})
	}
	list, err := c.Value()
	want := []core.WeightedValue[string]{
		core.NewWeightedValue("c", rawWeighting(0.5)),
		core.NewWeightedValue("1.25", rawWeighting(0.5)),
	}
	if err != nil || !reflect.DeepEqual(list, want) {
		t.Errorf("C gave %v (%v), want %v", list, err, want)
	}
	got, err = a.Get("D")
	if err != nil {
		t.Fatal(err)
	}
	d, ok := got.(cloudWeighted)
	if !ok {
		t.Fatalf("D gave a %T, want a %T", got, cloudWeighted{})
	}
	if d.HasValue() || d.NoValueMessage() == "" {
		t.Errorf("D gave %v with the message %q, want no value and a message",
			d.AsInterface(), d.NoValueMessage())
	}
}

// Each property MUST also be exposed as a weighted value. WeightedValues
// gives each Property of the Engine, by its name in any case, as its weighted
// values, reading only that Property, and only when it is requested.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/data-model.md?plain=1#L37
func TestEachPropertyIsAlsoGivenAsWeightedValues(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	w := weighted(t, data)

	for _, c := range []struct {
		name  string
		value any
		raw   uint16
	}{
		{"b", "b", core.MaxRawWeighting},
		{"C", "c", rawWeighting(0.5)},
	} {
		got, err := propertyValues(t, w, c.name).Value()
		if err != nil {
			t.Fatalf("%s has no value: %v", c.name, err)
		}
		want := []core.WeightedValue[any]{core.NewWeightedValue(c.value, c.raw)}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s gave %v, want %v", c.name, got, want)
		}
	}
	if reads := results.readsMade(); !reflect.DeepEqual(reads, [][]int{{1}, {2}}) {
		t.Errorf("getting b then C read the positions %v, want [[1] [2]]", reads)
	}
}

// A Property of the Engine with no value for the address says that there is
// no value, with a message, as a Property with no value does in any Aspect
// Data, from Get and from WeightedValues.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/properties.md#null-values
func TestAPropertyWithNoValueSaysSo(t *testing.T) {
	e := &Engine{}
	e.setPositions([]string{"A", "D"})
	results := &countingResults{values: map[int][]*ipi_interop.WeightedValue{
		0: {{Value: "a", Weight: 1}},
	}}
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	d := textOf(t, aspectData(t, data), "D")
	if d.HasValue() {
		t.Errorf("Get gave D the value %v, want no value", d.AsInterface())
	}
	if d.NoValueMessage() == "" {
		t.Error("Get gave D no value and no message saying why")
	}
	w := propertyValues(t, weighted(t, data), "D")
	if w.HasValue() {
		t.Errorf("WeightedValues gave D the value %v, want no value", w.AsInterface())
	}
	if w.NoValueMessage() == "" {
		t.Error("WeightedValues gave D no value and no message saying why")
	}
}

// A name that is not a Property of the Engine gives an error and reads
// nothing.
func TestGetOfAnUnknownPropertyGivesAnError(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	if v, err := aspectData(t, data).Get("D"); err == nil {
		t.Errorf("Get gave D as %v and no error, want an error", v)
	}
	if v, err := weighted(t, data).WeightedValues("D"); err == nil {
		t.Errorf("WeightedValues gave D as %v and no error, want an error", v)
	}
	if reads := results.readsMade(); len(reads) != 0 {
		t.Errorf("getting D read %v, want nothing read", reads)
	}
}

// AsMap gives every Property of the Engine under its name in lower case, as
// Get gives it.
func TestAsMapGivesEveryProperty(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	m := aspectData(t, data).AsMap()
	for name, value := range map[string]string{"a": "a", "b": "b", "c": "c"} {
		v, ok := m[name].(textValue)
		if !ok {
			t.Errorf("AsMap holds %T under %q, want %T", m[name], name, textValue{})
			continue
		}
		if got, err := v.Value(); err != nil || got != value {
			t.Errorf("AsMap holds %q (%v) under %q, want %q", got, err, name, value)
		}
	}
}

// With a data file, Get gives each Property the values Process gives for it,
// in the form the cloud engine gives a Property that is weighted or not, and
// WeightedValues gives the same values with their weights.
func TestGetGivesTheValuesOfProcess(t *testing.T) {
	e := newReloadTestEngine(t, WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithAutoUpdate(false), WithFileWatch(false))
	want, err := e.Process(reloadTestAddress)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) == 0 {
		t.Fatalf("%s gave no values", reloadTestAddress)
	}
	properties, err := e.Properties()
	if err != nil {
		t.Fatal(err)
	}
	isWeighted := map[string]bool{}
	for _, p := range properties {
		isWeighted[strings.ToLower(p.Name)] = p.Type.IsWeighted()
	}
	data, err := e.ProcessData(reloadTestAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	a := aspectData(t, data)
	for name, values := range want {
		if isWeighted[strings.ToLower(name)] {
			checkCloudWeighted(t, a, name, values)
		} else {
			checkText(t, a, name, values)
		}
	}
	w := weighted(t, data)
	for name, values := range want {
		v := propertyValues(t, w, name)
		if len(values) == 0 {
			if v.HasValue() {
				t.Errorf("%s gave %v, want no value", name, v.AsInterface())
			}
			continue
		}
		got, err := v.Value()
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(got) != len(values) {
			t.Errorf("%s gave %d values, want %d", name, len(got), len(values))
			continue
		}
		for i, w := range values {
			if !reflect.DeepEqual(got[i].Value(), w.Value) ||
				got[i].RawWeighting() != rawWeighting(w.Weight) {
				t.Errorf("%s value %d is %v, want %v with the weight %v",
					name, i, got[i], w.Value, w.Weight)
			}
		}
	}
}

// checkText checks that Get gives the Property as text, being the first of
// values, or no value when there are none.
func checkText(t *testing.T, a engines.AspectData, name string,
	values []*ipi_interop.WeightedValue) {
	t.Helper()
	v := textOf(t, a, name)
	if len(values) == 0 {
		if v.HasValue() {
			t.Errorf("%s gave %v, want no value", name, v.AsInterface())
		}
		return
	}
	if got, err := v.Value(); err != nil || got != cloudText(values[0].Value) {
		t.Errorf("%s gave %q (%v), want %q", name, got, err,
			cloudText(values[0].Value))
	}
}

// checkCloudWeighted checks that Get gives the Property as the cloud engine
// gives a weighted one, each value as text with its weight.
func checkCloudWeighted(t *testing.T, a engines.AspectData, name string,
	values []*ipi_interop.WeightedValue) {
	t.Helper()
	got, err := a.Get(name)
	if err != nil {
		t.Fatalf("Get(%q) gave %v", name, err)
	}
	v, ok := got.(cloudWeighted)
	if !ok {
		t.Errorf("%s gave a %T, want a %T", name, got, cloudWeighted{})
		return
	}
	if len(values) == 0 {
		if v.HasValue() {
			t.Errorf("%s gave %v, want no value", name, v.AsInterface())
		}
		return
	}
	list, err := v.Value()
	if err != nil || len(list) != len(values) {
		t.Errorf("%s gave %v (%v), want %d values", name, list, err, len(values))
		return
	}
	for i, w := range values {
		if list[i].Value() != cloudText(w.Value) ||
			list[i].RawWeighting() != rawWeighting(w.Weight) {
			t.Errorf("%s value %d is %v, want %q with the weight %v",
				name, i, list[i], cloudText(w.Value), w.Weight)
		}
	}
}
