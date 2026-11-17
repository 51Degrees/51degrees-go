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
	"runtime"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// enginePipeline gives a Pipeline holding the engine, reading the ASN data
// file.
func enginePipeline(t *testing.T) core.Pipeline {
	t.Helper()
	engine, err := NewPipelineEngine(
		WithDataFile(compatibleDataFile(t)), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		engine.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { pipeline.Close() })
	return pipeline
}

// processAddress processes a Flow Data holding the address, and gives the
// Data the engine added.
func processAddress(t *testing.T, pipeline core.Pipeline, address string) *Data {
	t.Helper()
	fd := pipeline.CreateFlowData()
	if address != "" {
		fd.AddEvidence("query.client-ip", address)
	}
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*Data](fd)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// propertiesRead gives the number of Properties the Data has read from the
// native results.
func propertiesRead(d *Data) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.read)
}

// In a Pipeline, as on its own, the engine reads a Property value from the
// native results only when that Property is requested, and reads nothing
// while it processes the Flow Data.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L128-L130
func TestPipelineEngineReadsAPropertyOnlyWhenItIsRequested(t *testing.T) {
	data := processAddress(t, enginePipeline(t), compatibleAddress)
	if n := propertiesRead(data); n != 0 {
		t.Fatalf("processing read %d Properties, want none", n)
	}
	name := textOf(t, data, "asnname")
	if !name.HasValue() {
		t.Errorf("asnname for %s has no value", compatibleAddress)
	}
	if n := propertiesRead(data); n != 1 {
		t.Errorf("getting asnname read %d Properties, want 1", n)
	}
}

// With no IP address in the evidence the engine reads, every Property has no
// value, and the message says why.
func TestPipelineEngineWithNoAddressSaysSo(t *testing.T) {
	data := processAddress(t, enginePipeline(t), "")
	name := textOf(t, data, "asnname")
	if name.HasValue() {
		t.Errorf("asnname is %v with no address, want no value", name.AsInterface())
	}
	if name.NoValueMessage() != noAddressReason {
		t.Errorf("asnname gave the message %q, want %q",
			name.NoValueMessage(), noAddressReason)
	}
}

// A Flow Data has no Close, so the native results of the Data the engine adds
// to one are freed once nothing holds the Data. A ResultsIpi that is
// collected before it is freed panics, which ends the test binary, so this
// test fails that way when the results are not freed.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L54-L61
func TestPipelineEngineFreesTheResultsOfDataNoLongerHeld(t *testing.T) {
	pipeline := enginePipeline(t)
	for i := 0; i < 100; i++ {
		processAddress(t, pipeline, compatibleAddress)
	}
	// Each collection lets the finalizers queued by the one before run, so
	// three are enough for the Data, then its results, to be collected.
	for i := 0; i < 3; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
}
