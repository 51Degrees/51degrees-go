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
	"errors"
	"io"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

// evidenceCounting is countingResults populated from Evidence. It records
// being freed where another goroutine can see it.
type evidenceCounting struct {
	*countingResults
	freed atomic.Int32
}

func (c *evidenceCounting) ResultsIpiFromEvidence(*ipi_interop.Evidence) error {
	return nil
}

func (c *evidenceCounting) Free() { c.freed.Add(1) }

// noEvidence is Evidence that holds nothing.
type noEvidence struct{}

func (noEvidence) Get(string) (any, bool) { return nil, false }
func (noEvidence) AsMap() map[string]any  { return map[string]any{} }
func (noEvidence) Keys() []string         { return nil }

// newCountingAspect gives an AspectEngine whose Properties are A, B and C,
// with results that count each read, and the AspectData of one lookup.
func newCountingAspect(t *testing.T) (*AspectData, *evidenceCounting) {
	t.Helper()
	engine, counting := newCountingEngine()
	a := &AspectEngine{
		engine:    engine,
		names:     []string{"a", "b", "c"},
		positions: map[string]int{"a": 0, "b": 1, "c": 2},
	}
	results := &evidenceCounting{countingResults: counting}
	data, err := a.processInto(results, noEvidence{})
	if err != nil {
		t.Fatal(err)
	}
	return newAspectData(a, data), results
}

// The Element Data MUST read a Property value from ResultsIpi only when that
// Property is requested, and MUST NOT copy Property values out of ResultsIpi
// while the Engine processes the Flow Data. The Element Data MAY keep a
// value once it has been read.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L128-L136
func TestAspectDataReadsAPropertyOnlyWhenItIsRequested(t *testing.T) {
	data, results := newCountingAspect(t)
	defer data.Close()
	if reads := results.readsMade(); len(reads) != 0 {
		t.Fatalf("processing read %v, want no read", reads)
	}

	for _, name := range []string{"B", "b"} {
		values, err := readWeighted(t, data, name).Value()
		if err != nil {
			t.Fatal(err)
		}
		if len(values) != 1 || values[0].Value() != "b" ||
			values[0].RawWeighting() != 65535 {
			t.Errorf("%s is %v, want b with the raw weighting 65535", name,
				values)
		}
	}
	reads := results.readsMade()
	if len(reads) != 1 || len(reads[0]) != 1 || reads[0][0] != 1 {
		t.Errorf("two requests for B read %v, want one read of position 1",
			reads)
	}

	values, err := readWeighted(t, data, "c").Value()
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].Value() != "c" ||
		values[0].RawWeighting() != 32768 {
		t.Errorf("c is %v, want c with the raw weighting 32768", values)
	}
}

// AsMap is a request for every Property, so it reads each one that has not
// been read, and reads none a second time.
func TestAsMapReadsEachPropertyOnce(t *testing.T) {
	data, results := newCountingAspect(t)
	defer data.Close()
	readWeighted(t, data, "a")
	for i := 0; i < 2; i++ {
		all := data.AsMap()
		for _, name := range []string{"a", "b", "c"} {
			if _, ok := all[name].(weightedText); !ok {
				t.Errorf("AsMap holds a %T for %s", all[name], name)
			}
		}
	}
	if reads := results.readsMade(); len(reads) != 3 {
		t.Errorf("the reads were %v, want one of each Property", reads)
	}
}

// There MUST be some mechanism to ensure that such resources are cleaned up
// when the Element Data is no longer needed. Close is that mechanism, and a
// read after it gives ErrDataFreed.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L54-L61
func TestCloseFreesTheNativeResults(t *testing.T) {
	data, results := newCountingAspect(t)
	var closer io.Closer = data
	readWeighted(t, data, "a")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	if freed := results.freed.Load(); freed != 1 {
		t.Fatalf("Close freed the native results %d times, want once", freed)
	}
	for _, name := range []string{"a", "b"} {
		if _, err := data.Get(name); !errors.Is(err, ErrDataFreed) {
			t.Errorf("Get(%s) after Close gave %v, want ErrDataFreed", name,
				err)
		}
	}
	if all := data.AsMap(); len(all) != 0 {
		t.Errorf("AsMap after Close holds %v, want nothing", all)
	}
	if err := closer.Close(); err != nil {
		t.Errorf("a second Close gave %v", err)
	}
	if freed := results.freed.Load(); freed != 1 {
		t.Errorf("the native results were freed %d times, want once", freed)
	}
}

// An AspectData that is never closed has its native results freed once
// nothing uses it any more, when the garbage collector finds it.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L47-L52
func TestAnAspectDataNeverClosedIsFreedByTheGarbageCollector(t *testing.T) {
	var results *evidenceCounting
	func() {
		var data *AspectData
		data, results = newCountingAspect(t)
		readWeighted(t, data, "a")
	}()
	deadline := time.Now().Add(leakCheckTimeoutSeconds * time.Second)
	for results.freed.Load() == 0 && time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(leakCheckPollMs * time.Millisecond)
	}
	if freed := results.freed.Load(); freed != 1 {
		t.Errorf("the native results were freed %d times, want once", freed)
	}
}

// A value is given as the text the cloud service writes for it, and a weight
// as a raw weighting from 0 to 65535.
func TestValuesAreTextWithARawWeighting(t *testing.T) {
	for _, c := range []struct {
		value any
		text  string
	}{
		{"GB", "GB"},
		{15169, "15169"},
		{51.5, "51.5"},
		{-0.127, "-0.127"},
		{float64(250), "250"},
		{true, "true"},
	} {
		if got := valueText(c.value); got != c.text {
			t.Errorf("%v is given as %q, want %q", c.value, got, c.text)
		}
	}
	for _, c := range []struct {
		weight float64
		raw    uint16
	}{
		{1, 65535},
		{0, 0},
		{0.5, 32768},
		{0.25, 16384},
		{-1, 0},
		{2, 65535},
	} {
		if got := toRawWeighting(c.weight); got != c.raw {
			t.Errorf("the weight %v is given as %d, want %d", c.weight, got,
				c.raw)
		}
	}
}
