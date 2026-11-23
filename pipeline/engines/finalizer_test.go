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
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

const (
	finalizeFlows          = 50
	finalizeTimeoutSeconds = 10
	finalizePollMs         = 10
)

// finalizedData is Aspect Data with a finalizer, as the Aspect Data of an
// on-premise Engine has.
type finalizedData struct{ *BaseAspectData }

// finalizingEngine adds finalizedData whose finalizer counts the instances
// finalized.
type finalizingEngine struct {
	*BaseAspectEngine
	finalized atomic.Int64
}

func (e *finalizingEngine) Process(fd core.FlowData) error {
	return e.Execute(fd, func() (AspectData, error) {
		d := &finalizedData{BaseAspectData: NewBaseAspectData()}
		runtime.SetFinalizer(d, func(*finalizedData) { e.finalized.Add(1) })
		return d, nil
	})
}

// processAndDrop processes Flow Data and keeps none of them, nor the Aspect
// Data they hold.
//
//go:noinline
func processAndDrop(t *testing.T, p core.Pipeline, flows int) {
	t.Helper()
	for i := 0; i < flows; i++ {
		fd := p.CreateFlowData()
		if err := fd.Process(); err != nil {
			t.Fatal(err)
		}
		if _, err := fd.Get("finalized"); err != nil {
			t.Fatal(err)
		}
	}
}

// An on-premise Engine releases native results from a finalizer once nothing
// holds its Aspect Data, for a caller that closes neither the Flow Data nor
// the Aspect Data. Go runs no finalizer on an object that can reach itself,
// so Aspect Data added to a Flow Data must keep no reference back to itself,
// or the finalizer never runs and the native results are never released.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/features/resource-cleanup.md?plain=1#L60-L61
func TestAspectDataNoLongerHeldIsFinalized(t *testing.T) {
	e := &finalizingEngine{BaseAspectEngine: NewBaseAspectEngine(
		"finalized", core.NewWhitelistFilter(), nil)}
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	processAndDrop(t, p, finalizeFlows)
	deadline := time.Now().Add(finalizeTimeoutSeconds * time.Second)
	for e.finalized.Load() < finalizeFlows && time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(finalizePollMs * time.Millisecond)
	}
	if n := e.finalized.Load(); n != finalizeFlows {
		t.Errorf("%d of %d Aspect Data no longer held were finalized within "+
			"%d seconds", n, finalizeFlows, finalizeTimeoutSeconds)
	}
}
