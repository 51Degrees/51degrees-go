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
	"sync"
	"testing"
)

// manyValuesElement writes an Element Data holding more than one value, so a
// reader reads several entries of the same instance.
type manyValuesElement struct{ *BaseElement }

func (e *manyValuesElement) Process(fd FlowData) error {
	data := fd.GetOrAdd(e.DataKey(), func(FlowData) ElementData { return NewBaseElementData() })
	data.Set("platform", "linux")
	data.Set("platformversion", "6.8.0")
	return nil
}

// Once the Flow Element that writes an Element Data has finished, the members
// of a parallel group may read that instance at the same time, so reading the
// Element Data from several goroutines at once is safe. The Flow Data itself
// is used by one goroutine at a time, so only the test's goroutine reads it.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L12-L16
// and https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/advanced-features/parallel-processing.md?plain=1#L41-L43
func TestElementDataIsReadFromSeveralGoroutinesAtOnce(t *testing.T) {
	const (
		readers        = 16
		readsPerReader = 200
	)
	el := &manyValuesElement{BaseElement: NewBaseElement("device", NewWhitelistFilter(), nil)}
	p, err := NewPipelineBuilder().AddFlowElement(el).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get("device")
	if err != nil {
		t.Fatalf("Flow Data gave no device data: %v", err)
	}
	if byKey := fd.ElementDataAsMap(); len(byKey) != 1 {
		t.Fatalf("the Flow Data holds %d Element Data, want 1", len(byKey))
	}

	// Process has returned, so the only write is over before any read below.
	var wg sync.WaitGroup
	start := make(chan struct{})
	failures := make(chan string, readers)
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for n := 0; n < readsPerReader; n++ {
				value, err := data.Get("platform")
				if err != nil {
					failures <- "platform is missing: " + err.Error()
					return
				}
				if value != "linux" {
					failures <- "platform read as something other than linux"
					return
				}
				typed, err := GetPropertyAs[string](data, "platformversion")
				if err != nil || typed != "6.8.0" {
					failures <- "platformversion read as something other than 6.8.0"
					return
				}
				if all := data.AsMap(); len(all) != 2 {
					failures <- "the Element Data holds other than its two values"
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}
