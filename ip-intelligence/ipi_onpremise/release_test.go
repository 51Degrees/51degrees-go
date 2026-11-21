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

// These tests follow the resource cleanup rules for Element Data that holds
// native resources, which a Go garbage collector cannot see.

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
)

// closer gives the Data as an io.Closer, failing the test when it is not.
func closer(t *testing.T, data *Data) io.Closer {
	t.Helper()
	var v any = data
	c, ok := v.(io.Closer)
	if !ok {
		t.Fatalf("the Data is a %T, which has no Close, so a caller cannot "+
			"release its native results when it has finished with it", data)
	}
	return c
}

// resultsHeld reports whether the Data still holds its native results.
func resultsHeld(d *Data) bool {
	return d.results != nil
}

// "There MUST be some mechanism to ensure that such resources are cleaned up
// when the Element Data is no longer needed." Close releases the native
// results at once, rather than when the garbage collector next runs, which
// "fails to clean resources up fast enough" because it cannot see the native
// memory. A second Close does nothing.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L47-L52
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L60-L61
func TestCloseFreesTheNativeResultsAtOnce(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	c := closer(t, data)
	if _, err := data.Get("a"); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close gave %v", err)
	}
	if !results.freed {
		t.Error("Close did not free the native results")
	}
	if _, err := data.Get("b"); !errors.Is(err, ErrDataFreed) {
		t.Errorf("Get after Close gave %v, want ErrDataFreed", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("a second Close gave %v", err)
	}
}

// "There MUST be some mechanism to ensure that such resources are cleaned up
// when the Element Data is no longer needed." The finalizer a Pipeline sets
// and Close are two such ways, so Close after Free, which the finalizer
// calls, does nothing and gives no error.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L60-L61
func TestCloseAfterFreeDoesNothing(t *testing.T) {
	e, results := newCountingEngine()
	data, err := e.processInto(results, "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	c := closer(t, data)
	data.Free()
	for i := 0; i < 2; i++ {
		if err := c.Close(); err != nil {
			t.Errorf("Close %d after Free gave %v", i+1, err)
		}
	}
	if !results.freed {
		t.Error("Free did not free the native results")
	}
}

// "There MUST be some mechanism to ensure that such resources are cleaned up
// when the Element Data is no longer needed." In a Pipeline, Close on the
// Data the engine added releases its native results at once, without waiting
// for the garbage collector.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L60-L61
func TestCloseInAPipelineFreesTheNativeResultsAtOnce(t *testing.T) {
	data := processAddress(t, enginePipeline(t), compatibleAddress)
	if !resultsHeld(data) {
		t.Fatal("the Data holds no native results before Close")
	}
	if err := closer(t, data).Close(); err != nil {
		t.Fatalf("Close gave %v", err)
	}
	if resultsHeld(data) {
		t.Error("the Data still holds its native results after Close")
	}
	if _, err := data.Get("asnname"); !errors.Is(err, ErrDataFreed) {
		t.Errorf("Get after Close gave %v, want ErrDataFreed", err)
	}
}

// cacheSetter is an engine whose SetCache says whether the cache was added.
type cacheSetter interface {
	SetCache(c cache.Cache) error
}

// "the system MUST NOT allow a cache to be added to Engines where the Element
// Data they produce requires cleanup beyond what normal garbage collection
// will achieve." The Data of this engine holds native results that must be
// released, so SetCache refuses a cache with an error, and the engine then
// caches nothing.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L74-L76
func TestPipelineEngineRefusesACache(t *testing.T) {
	engine, err := NewPipelineEngine(
		WithDataFile(compatibleDataFile(t)), WithAutoUpdate(false))
	if err != nil {
		t.Fatal(err)
	}
	var v any = engine
	setter, ok := v.(cacheSetter)
	if !ok {
		engine.Close()
		t.Fatal("SetCache gives no error, so a cache can be added to an " +
			"engine whose Data must be released")
	}
	err = setter.SetCache(cache.NewLRUCache(cache.DefaultSize, 0))
	if err == nil {
		t.Error("SetCache added a cache, want an error")
	} else if !strings.Contains(err.Error(), "cache") {
		t.Errorf("the error %q does not say the cache was refused", err)
	}
	pipeline, err := core.NewPipelineBuilder().AddFlowElement(engine).Build()
	if err != nil {
		engine.Close()
		t.Fatal(err)
	}
	defer pipeline.Close()
	first := processAddress(t, pipeline, compatibleAddress)
	second := processAddress(t, pipeline, compatibleAddress)
	if first == second || second.CacheHit() {
		t.Error("the second Data for the same address came from a cache")
	}
}
