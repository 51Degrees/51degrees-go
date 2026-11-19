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
	"math"
	"reflect"
	"testing"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// weightedValues is what Get gives for a Property of the Engine.
type weightedValues = engines.AspectPropertyValue[[]core.WeightedValue[any]]

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

// propertyValues gets the named Property from the Aspect Data, failing the
// test when it is missing or not weighted values.
func propertyValues(t *testing.T, a engines.AspectData, name string) weightedValues {
	t.Helper()
	v, err := a.Get(name)
	if err != nil {
		t.Fatalf("Get(%q) gave %v", name, err)
	}
	values, ok := v.(weightedValues)
	if !ok {
		t.Fatalf("Get(%q) gave a %T, want a %T", name, v, weightedValues{})
	}
	return values
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

// Each property MUST also be exposed as a weighted value. Get gives each
// Property of the Engine, by its name in any case, as its weighted values,
// reading only that Property, and only when it is requested.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/data-model.md?plain=1#L37
func TestGetGivesEachPropertyAsWeightedValues(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	a := aspectData(t, data)

	for _, c := range []struct {
		name  string
		index int
		value any
		raw   uint16
	}{
		{"b", 1, "b", core.MaxRawWeighting},
		{"C", 2, "c", rawWeighting(0.5)},
	} {
		got, err := propertyValues(t, a, c.name).Value()
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

// A Property of the Engine with no value for the address gives weighted
// values that say there is no value, with a message, as a Property with no
// value does in any Aspect Data.
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
	d := propertyValues(t, aspectData(t, data), "D")
	if d.HasValue() {
		t.Errorf("D gave the value %v, want no value", d.AsInterface())
	}
	if d.NoValueMessage() == "" {
		t.Error("D has no value and no message saying why")
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
		t.Errorf("D gave %v and no error, want an error", v)
	}
	if reads := results.readsMade(); len(reads) != 0 {
		t.Errorf("getting D read %v, want nothing read", reads)
	}
}

// AsMap gives every Property of the Engine under its name in lower case, as
// its weighted values.
func TestAsMapGivesEveryProperty(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	m := aspectData(t, data).AsMap()
	for name, value := range map[string]any{"a": "a", "b": "b", "c": "c"} {
		v, ok := m[name].(weightedValues)
		if !ok {
			t.Errorf("AsMap holds %T under %q, want weighted values", m[name], name)
			continue
		}
		got, err := v.Value()
		if err != nil || len(got) != 1 || got[0].Value() != value {
			t.Errorf("AsMap holds %v (%v) under %q, want the one value %v",
				got, err, name, value)
		}
	}
}

// With a data file, Get gives each Property the values and weights Process
// gives for it.
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
	data, err := e.ProcessData(reloadTestAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	a := aspectData(t, data)
	for name, values := range want {
		v := propertyValues(t, a, name)
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
