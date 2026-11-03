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

package engines

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
)

// countingData is sample Aspect Data.
type countingData struct{ *BaseAspectData }

func newCountingData() *countingData { return &countingData{BaseAspectData: NewBaseAspectData()} }

// countingEngine records how many times it actually computes a result, so tests
// can verify that caching and lazy loading behave as expected.
type countingEngine struct {
	*BaseAspectEngine
	computeCount atomic.Int64
	delay        time.Duration
}

func newCountingEngine() *countingEngine {
	e := &countingEngine{BaseAspectEngine: NewBaseAspectEngine("counter", core.NewWhitelistFilter("query.user-agent"), nil)}
	e.SetDataSourceTier("test")
	return e
}

func (e *countingEngine) Process(fd core.FlowData) error {
	return e.Execute(fd, func() (AspectData, error) {
		e.computeCount.Add(1)
		if e.delay > 0 {
			time.Sleep(e.delay)
		}
		d := newCountingData()
		ua, _ := fd.Evidence().Get("query.user-agent")
		d.Set("ua", ua)
		return d, nil
	})
}

func process(t *testing.T, p core.Pipeline, ua string) core.FlowData {
	t.Helper()
	fd := p.CreateFlowData()
	fd.AddEvidence("query.user-agent", ua)
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	return fd
}

func TestEngineCaching(t *testing.T) {
	e := newCountingEngine()
	e.SetCache(cache.NewLRUCache(100, 4))
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()

	// Same evidence twice: compute should run only once (the cache provides a
	// demonstrable benefit, as required by the caching specification).
	process(t, p, "agentA")
	fd2 := process(t, p, "agentA")
	if got := e.computeCount.Load(); got != 1 {
		t.Fatalf("expected 1 compute for identical evidence, got %d", got)
	}
	d, _ := fd2.Get("counter")
	if !d.(AspectData).CacheHit() {
		t.Fatal("second identical request should be a cache hit")
	}

	// Different evidence: compute runs again.
	process(t, p, "agentB")
	if got := e.computeCount.Load(); got != 2 {
		t.Fatalf("expected 2 computes for distinct evidence, got %d", got)
	}
}

func TestEngineNoCache(t *testing.T) {
	e := newCountingEngine()
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()
	process(t, p, "agentA")
	process(t, p, "agentA")
	if got := e.computeCount.Load(); got != 2 {
		t.Fatalf("without a cache, each request computes: got %d", got)
	}
}

func TestEngineLazyLoading(t *testing.T) {
	e := newCountingEngine()
	e.delay = 30 * time.Millisecond
	e.SetLazyLoading(true, time.Second)
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("query.user-agent", "agentLazy")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	d, err := fd.Get("counter")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	// Accessing the value blocks until the background compute finishes.
	v, err := d.Get("ua")
	if err != nil {
		t.Fatalf("lazy get: %v", err)
	}
	if v != "agentLazy" {
		t.Fatalf("got %v", v)
	}
}

func TestEngineLazyTimeout(t *testing.T) {
	e := newCountingEngine()
	e.delay = 200 * time.Millisecond
	e.SetLazyLoading(true, 10*time.Millisecond)
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()
	fd := process(t, p, "slow")
	d, _ := fd.Get("counter")
	if _, err := d.Get("ua"); err == nil {
		t.Fatal("expected timeout error from lazy access")
	}
}

func TestAspectPropertyValue(t *testing.T) {
	v := NewAspectPropertyValue("Aries")
	if !v.HasValue() {
		t.Fatal("should have value")
	}
	if got, _ := v.Value(); got != "Aries" {
		t.Fatalf("got %v", got)
	}

	nv := NoValue[string]("not available in lite data")
	if nv.HasValue() {
		t.Fatal("should not have value")
	}
	if _, err := nv.Value(); err == nil {
		t.Fatal("expected NoValueError")
	} else {
		var nve *core.NoValueError
		if !asNoValue(err, &nve) {
			t.Fatalf("expected NoValueError, got %T", err)
		}
	}
	if nv.ValueOrDefault("fallback") != "fallback" {
		t.Fatal("ValueOrDefault should return default")
	}
}

func asNoValue(err error, target **core.NoValueError) bool {
	if e, ok := err.(*core.NoValueError); ok {
		*target = e
		return true
	}
	return false
}
