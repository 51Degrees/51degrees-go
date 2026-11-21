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

package core

import (
	"sync/atomic"
	"testing"
)

// lateElement has no Properties until they are made to arrive, and counts
// how often it is asked for them.
type lateElement struct {
	*BaseElement
	arrived atomic.Bool
	asked   atomic.Int64
}

func newLateElement(key string) *lateElement {
	return &lateElement{BaseElement: NewBaseElement(key, nil, nil)}
}

func (e *lateElement) Process(FlowData) error { return nil }

func (e *lateElement) Properties() []PropertyMetadata {
	e.asked.Add(1)
	if !e.arrived.Load() {
		return nil
	}
	return []PropertyMetadata{{Name: "late", Element: e, Available: true}}
}

func (e *lateElement) PropertiesLoaded() bool { return e.arrived.Load() }

// names returns the names of props.
func names(props []PropertyMetadata) []string {
	out := make([]string, len(props))
	for i, pm := range props {
		out[i] = pm.Name
	}
	return out
}

// The Pipeline asks an element for its Properties again until it reports
// they have loaded, and keeps them from then on, whether the element is in
// the Pipeline or in a parallel group.
func TestPipelineGathersPropertiesUntilLoaded(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		late := newLateElement("late")
		var el FlowElement = late
		if grouped {
			el = NewParallelElements(nil, late, newStarSignElement())
		}
		p, err := NewPipelineBuilder().AddFlowElement(el).Build()
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		for _, pm := range p.Properties() {
			if pm.Name == "late" {
				t.Fatalf("grouped %v: the Properties are there before they arrive", grouped)
			}
		}
		late.arrived.Store(true)
		found := false
		for _, pm := range p.Properties() {
			found = found || pm.Name == "late"
		}
		if !found {
			t.Errorf("grouped %v: the Properties %v lack the ones that arrived",
				grouped, names(p.Properties()))
		}
		asked := late.asked.Load()
		p.Properties()
		p.Properties()
		if got := late.asked.Load(); got != asked {
			t.Errorf("grouped %v: asked %d more times once loaded, want none",
				grouped, got-asked)
		}
		p.Close()
	}
}
