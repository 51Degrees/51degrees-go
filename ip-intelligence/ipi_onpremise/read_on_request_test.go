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

import (
	"reflect"
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

// countingResults stands in for the native results. It records each read of
// values, so a test can tell when, and for which Properties, they are read.
type countingResults struct {
	mu      sync.Mutex
	values  map[int][]*ipi_interop.WeightedValue
	reads   [][]int
	address string
	freed   bool
}

func (c *countingResults) ResultsIpiFromIpAddress(ipAddress string) error {
	c.address = ipAddress
	return nil
}

func (c *countingResults) HasValues() bool { return true }

func (c *countingResults) GetWeightedValuesByIndexes(
	indexes []int, _ func(int) string) (ipi_interop.Values, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reads = append(c.reads, append([]int(nil), indexes...))
	values := ipi_interop.Values{}
	if indexes == nil {
		for i, v := range c.values {
			values[string(rune('A'+i))] = v
		}
		return values, nil
	}
	for _, i := range indexes {
		if v, ok := c.values[i]; ok {
			values[string(rune('A'+i))] = v
		}
	}
	return values, nil
}

func (c *countingResults) Free() { c.freed = true }

func (c *countingResults) readsMade() [][]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]int(nil), c.reads...)
}

// newCountingEngine gives an Engine whose required Properties are A, B and C,
// at positions 0, 1 and 2, and results holding a value for each.
func newCountingEngine() (*Engine, *countingResults) {
	e := &Engine{}
	e.setPositions([]string{"A", "B", "C"})
	results := &countingResults{values: map[int][]*ipi_interop.WeightedValue{
		0: {{Value: "a", Weight: 1}},
		1: {{Value: "b", Weight: 1}},
		2: {{Value: "c", Weight: 0.5}},
	}}
	return e, results
}

// The Element Data MUST read a Property value from ResultsIpi only when that
// Property is requested, and MUST NOT copy Property values out of ResultsIpi
// while the Engine processes the Flow Data.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L128-L130
func TestProcessingReadsNoPropertyValue(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	if results.address != "203.0.113.1" {
		t.Errorf("the native results were made for %q, want 203.0.113.1",
			results.address)
	}
	if reads := results.readsMade(); len(reads) != 0 {
		t.Errorf("processing read values for %v, want no value read until "+
			"a Property is requested", reads)
	}
}

// A Property is read when it is requested, and no other Property is read with
// it.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L128-L130
func TestARequestedPropertyIsReadAlone(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()

	values, err := data.Values("B")
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Value != "b" {
		t.Errorf("B gave %v, want the one value b", values)
	}
	if reads := results.readsMade(); !reflect.DeepEqual(reads, [][]int{{1}}) {
		t.Errorf("reading B read the positions %v, want only [[1]]", reads)
	}

	values, err = data.ValuesByIndex(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Value != "c" || values[0].Weight != 0.5 {
		t.Errorf("position 2 gave %v, want c with the weight 0.5", values)
	}
	if reads := results.readsMade(); !reflect.DeepEqual(reads, [][]int{{1}, {2}}) {
		t.Errorf("reading B then position 2 read %v, want [[1] [2]]", reads)
	}
}

// The Element Data MAY keep a value once it has been read, so that a second
// request for the same Property does not call the native code again.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L134-L136
func TestASecondRequestDoesNotReadAgain(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	for i := 0; i < 3; i++ {
		if _, err := data.Values("A"); err != nil {
			t.Fatal(err)
		}
	}
	if reads := results.readsMade(); !reflect.DeepEqual(reads, [][]int{{0}}) {
		t.Errorf("three requests for A read %v, want one read, [[0]]", reads)
	}
}

// A name the Engine was not created with gives an error and reads nothing.
func TestAnUnknownPropertyReadsNothing(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	if _, err := data.Values("D"); err == nil {
		t.Error("D gave no error, want one, as the Engine was not created with it")
	}
	if reads := results.readsMade(); len(reads) != 0 {
		t.Errorf("a request for D read %v, want nothing read", reads)
	}
}

// Free releases the native results, and a read after it gives an error rather
// than reading freed memory.
func TestFreeReleasesTheResults(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	data.Free()
	if !results.freed {
		t.Error("Free did not free the native results")
	}
	if _, err := data.Values("A"); err == nil {
		t.Error("a read after Free gave no error")
	}
	data.Free()
}

// Finished Element Data MUST be safe to read from several threads at once, so
// Aspect Data that reads its values from a ResultsIpi instance MUST let only
// one thread at a time read from its results.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L520-L525
func TestDataIsReadFromSeveralGoroutines(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			name := []string{"A", "B", "C"}[g%3]
			if _, err := data.Values(name); err != nil {
				t.Error(err)
			}
		}(g)
	}
	wg.Wait()
	if reads := results.readsMade(); len(reads) != 3 {
		t.Errorf("eight goroutines reading three Properties read %v, "+
			"want each Property read once", reads)
	}
}

// With a data file, each Property read on request gives the values Process
// gives for it.
func TestProcessDataGivesTheValuesOfProcess(t *testing.T) {
	e := newReloadTestEngine(t, WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithAutoUpdate(false), WithFileWatch(false))
	want, err := e.Process(reloadTestAddress)
	if err != nil {
		t.Fatal(err)
	}
	data, err := e.ProcessData(reloadTestAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer data.Free()
	if len(want) == 0 {
		t.Fatalf("%s gave no values", reloadTestAddress)
	}
	for name, values := range want {
		got, err := data.Values(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(got) != len(values) {
			t.Errorf("%s gave %d values, want %d", name, len(got), len(values))
			continue
		}
		for i := range values {
			if !reflect.DeepEqual(*got[i], *values[i]) {
				t.Errorf("%s value %d is %v, want %v", name, i, *got[i], *values[i])
			}
		}
	}
}
