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

// "An Engine whose Element Data another Flow Element also writes, as two
// Flow Elements that share one instance do, MUST NOT allow a cache to be
// added either, because the other Flow Element would write into an instance
// that other requests are reading."
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L234-L237
// and https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/caching.md?plain=1#L132-L135

import (
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/cache"
)

// sharingWriter writes a value into the Element Data that a counting engine
// adds, so the two share one instance.
type sharingWriter struct{ *core.BaseElement }

func newSharingWriter() *sharingWriter {
	return &sharingWriter{core.NewBaseElement("counter", nil, nil)}
}

func (w *sharingWriter) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("counter", func(core.FlowData) core.ElementData {
		return newCountingData()
	})
	if c, ok := d.(*countingData); ok {
		c.Set("written", true)
	}
	return nil
}

// ownKeyElement is a Flow Element with a key of its own.
type ownKeyElement struct{ *core.BaseElement }

func (o *ownKeyElement) Process(core.FlowData) error { return nil }

// isRefusedForTheCache reports whether err says that the Engine has a cache
// and another Flow Element writes its Element Data.
func isRefusedForTheCache(err error) bool {
	return err != nil && strings.Contains(err.Error(), "cache") &&
		strings.Contains(err.Error(), "another Flow Element")
}

// A Pipeline that puts an Engine with a cache beside another Flow Element
// that writes its Element Data is refused, wherever the other one is.
func TestAnEngineWithACacheIsRefusedWhereItsDataIsShared(t *testing.T) {
	for _, c := range []struct {
		name     string
		elements func(e *countingEngine) *core.PipelineBuilder
	}{
		{"after it", func(e *countingEngine) *core.PipelineBuilder {
			return core.NewPipelineBuilder().AddFlowElement(e).
				AddFlowElement(newSharingWriter())
		}},
		{"before it", func(e *countingEngine) *core.PipelineBuilder {
			return core.NewPipelineBuilder().AddFlowElement(newSharingWriter()).
				AddFlowElement(e)
		}},
		{"in a parallel group after it", func(e *countingEngine) *core.PipelineBuilder {
			return core.NewPipelineBuilder().AddFlowElement(e).AddParallel(
				newSharingWriter(),
				&ownKeyElement{core.NewBaseElement("other", nil, nil)})
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newCountingEngine()
			e.SetCache(cache.NewLRUCache(100, 4))
			p, err := c.elements(e).Build()
			if err == nil {
				p.Close()
				t.Fatal("the Pipeline was built")
			}
			if !isRefusedForTheCache(err) {
				t.Errorf("the error %q does not say that the Engine has a "+
					"cache and another Flow Element writes its Element Data", err)
			}
		})
	}
}

// A cache added to an Engine already in a Pipeline where another Flow Element
// writes its Element Data is refused, so the Engine stays without one.
func TestACacheIsRefusedForAnEngineWhoseDataIsAlreadyShared(t *testing.T) {
	e := newCountingEngine()
	p, err := core.NewPipelineBuilder().
		AddFlowElement(e).AddFlowElement(newSharingWriter()).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	e.SetCache(cache.NewLRUCache(100, 4))
	process(t, p, "agentA")
	process(t, p, "agentA")
	if got := e.computeCount.Load(); got != 2 {
		t.Errorf("the engine computed %d times for the same Evidence twice, "+
			"want twice, with no cache", got)
	}
}

// An Engine with a cache and no other Flow Element writing its Element Data
// still builds.
func TestAnEngineWithACacheBuildsWhereItsDataIsItsOwn(t *testing.T) {
	e := newCountingEngine()
	e.SetCache(cache.NewLRUCache(100, 4))
	p, err := core.NewPipelineBuilder().AddFlowElement(e).
		AddFlowElement(&ownKeyElement{core.NewBaseElement("other", nil, nil)}).Build()
	if err != nil {
		t.Fatalf("a Pipeline with the Engine's data its own was refused: %v", err)
	}
	p.Close()
}
