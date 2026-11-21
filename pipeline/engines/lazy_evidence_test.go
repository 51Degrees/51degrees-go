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
)

// evidenceReadingEngine reads the Evidence while it works, as most Engines
// do, and records when that work has finished.
type evidenceReadingEngine struct {
	*BaseAspectEngine
	finished atomic.Bool
}

func (e *evidenceReadingEngine) Process(fd core.FlowData) error {
	return e.Execute(fd, func() (AspectData, error) {
		time.Sleep(20 * time.Millisecond)
		d := newCountingData()
		d.Set("keys", len(fd.Evidence().Keys()))
		e.finished.Store(true)
		return d, nil
	})
}

// evidenceAddingElement adds Evidence for the Flow Elements after it, and
// records whether the Engine before it had finished its work by then.
type evidenceAddingElement struct {
	*core.BaseElement
	before      *evidenceReadingEngine
	beforeEnded atomic.Bool
}

func (e *evidenceAddingElement) Process(fd core.FlowData) error {
	e.beforeEnded.Store(e.before.finished.Load())
	fd.AddEvidence("query.added", "value")
	return nil
}

// Setting the deprecated lazy loading option has no effect, so an Engine has
// finished its work, and its reads of the Evidence, before a later Flow
// Element adds Evidence. The Evidence takes no lock, so a read on another
// goroutine at that point would race the add.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L32-L40
func TestAnEngineWithLazyLoadingSetEndsBeforeLaterElementsAddEvidence(
	t *testing.T) {
	reader := &evidenceReadingEngine{BaseAspectEngine: NewBaseAspectEngine(
		"reader", core.NewWhitelistFilter("query.user-agent"), nil)}
	reader.SetLazyLoading(true, time.Second)
	adder := &evidenceAddingElement{
		BaseElement: core.NewBaseElement("adder", nil, nil),
		before:      reader,
	}
	p, err := core.NewPipelineBuilder().
		AddFlowElement(reader).
		AddFlowElement(adder).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("query.user-agent", "agent")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	if !adder.beforeEnded.Load() {
		t.Error("the Engine with lazy loading set was still working when " +
			"a later Flow Element added Evidence, so it read the Evidence " +
			"on another goroutine during the add")
	}
	d, err := fd.Get("reader")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if v, err := d.Get("keys"); err != nil || v != 1 {
		t.Errorf("the Engine read %v Evidence keys with error %v, want 1 "+
			"and no error", v, err)
	}
}
