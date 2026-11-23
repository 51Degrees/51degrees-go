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

package onpremise

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

const (
	releaseFlows          = 20
	releaseTimeoutSeconds = 10
	releasePollMs         = 10
)

// Closing the Flow Data that holds a DeviceData releases its native results
// at once, so a caller that only closes the Flow Data, as the web middleware
// does, leaves nothing to the garbage collector.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L103-L107
func TestClosingTheFlowDataReleasesTheResults(t *testing.T) {
	engine, err := NewPipelineEngine(WithDataFile(aspectDataFile),
		WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		engine.Close()
		t.Fatal(err)
	}
	defer pipeline.Close()
	fd := pipeline.CreateFlowData()
	fd.AddEvidence("header.user-agent", aspectIPhone)
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	data, err := core.Get[*DeviceData](fd)
	if err != nil {
		t.Fatal(err)
	}
	if data.results == nil {
		t.Fatal("the DeviceData holds no native results before its Flow " +
			"Data is closed")
	}
	if err := fd.Close(); err != nil {
		t.Fatalf("Close gave %v", err)
	}
	if data.results != nil {
		t.Error("the DeviceData still holds its native results after its " +
			"Flow Data was closed")
	}
}

// releaseCounter counts the native results released through it.
type releaseCounter struct {
	deviceResults
	released *atomic.Int64
}

func (r releaseCounter) Free() {
	r.deviceResults.Free()
	r.released.Add(1)
}

// processAndDrop processes Flow Data, counting the release of the native
// results of each DeviceData, and keeps none of them.
//
//go:noinline
func processAndDrop(t *testing.T, pipeline core.Pipeline,
	released *atomic.Int64) {
	t.Helper()
	for i := 0; i < releaseFlows; i++ {
		fd := pipeline.CreateFlowData()
		fd.AddEvidence("header.user-agent", aspectIPhone)
		if err := fd.Process(); err != nil {
			t.Fatal(err)
		}
		data, err := core.Get[*DeviceData](fd)
		if err != nil {
			t.Fatal(err)
		}
		data.results = releaseCounter{data.results, released}
	}
}

// A caller that closes neither the Flow Data nor the DeviceData leaves the
// native results to the finalizer the engine sets, which releases them once
// nothing holds the DeviceData.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L60-L61
func TestResultsOfDeviceDataNoLongerHeldAreReleased(t *testing.T) {
	engine, err := NewPipelineEngine(WithDataFile(aspectDataFile),
		WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		engine.Close()
		t.Fatal(err)
	}
	defer pipeline.Close()

	var released atomic.Int64
	processAndDrop(t, pipeline, &released)
	deadline := time.Now().Add(releaseTimeoutSeconds * time.Second)
	for released.Load() < releaseFlows && time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(releasePollMs * time.Millisecond)
	}
	if n := released.Load(); n != releaseFlows {
		t.Errorf("the native results of %d of %d DeviceData no longer held "+
			"were released within %d seconds", n, releaseFlows,
			releaseTimeoutSeconds)
	}
}
