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

import "testing"

// closingData is Element Data that holds something to clean up, and counts
// how many times it is closed.
type closingData struct {
	*BaseElementData
	closed int
}

func (d *closingData) Close() error {
	d.closed++
	return nil
}

// closingWriter adds a closingData under the key "native", and keeps it in
// *data. A shared one is marked shared, as a cache marks what it holds.
func closingWriter(data **closingData, shared bool) *funcElement {
	return newFuncElement("native", nil, func(fd FlowData) error {
		fd.GetOrAdd("native", func(FlowData) ElementData {
			*data = &closingData{BaseElementData: NewBaseElementData()}
			if shared {
				MarkShared(*data)
			}
			return *data
		})
		return nil
	})
}

// closedAfterProcessing processes a Flow Data with element, closes it twice,
// and gives how many times the Element Data in *data was closed.
func closedAfterProcessing(t *testing.T, element FlowElement,
	data **closingData) int {
	t.Helper()
	p, err := NewPipelineBuilder().SetLogger(quietLogger()).
		AddFlowElement(element).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := closeOf(t, fd).Close(); err != nil {
			t.Fatalf("Close gave %v", err)
		}
	}
	if *data == nil {
		t.Fatal("the Element Data was never added")
	}
	return (*data).closed
}

// Closing a Flow Data triggers the clean up of each Element Data it holds
// that needs it, being one with a Close method, once however many times the
// Flow Data is closed.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L5-L13
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L103-L107
func TestClosingAFlowDataClosesElementDataThatNeedsCleanup(t *testing.T) {
	var data *closingData
	if n := closedAfterProcessing(t, closingWriter(&data, false),
		&data); n != 1 {
		t.Errorf("the Element Data was closed %d times, want once", n)
	}
}

// Element Data a cache shares with other requests is left alone, because
// those requests still read it.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L89-L96
func TestClosingAFlowDataLeavesSharedElementDataOpen(t *testing.T) {
	var data *closingData
	if n := closedAfterProcessing(t, closingWriter(&data, true),
		&data); n != 0 {
		t.Errorf("shared Element Data was closed %d times, want never", n)
	}
}
