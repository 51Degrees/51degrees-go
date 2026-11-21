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
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
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
