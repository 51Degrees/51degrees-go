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

package usagesharing

import (
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// "A builder MUST refuse, with an error saying that the usage sharing element
// must be the last Flow Element and outside any parallel group, a Pipeline
// that puts it anywhere else".
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L106-L112
func TestTheUsageSharingElementIsRefusedAnywhereButLast(t *testing.T) {
	sharing := func() *Element {
		return NewBuilder().SetSharePercentage(0).
			SetShareUsageURL("http://127.0.0.1:1/").Build()
	}
	engine := func() core.FlowElement {
		return &userAgentEngine{BaseElement: core.NewBaseElement("device",
			core.NewWhitelistFilter("header.user-agent"), nil)}
	}
	for _, c := range []struct {
		name     string
		elements func(e *Element) *core.PipelineBuilder
	}{
		{"first", func(e *Element) *core.PipelineBuilder {
			return core.NewPipelineBuilder().AddFlowElement(e).
				AddFlowElement(engine())
		}},
		{"in a parallel group", func(e *Element) *core.PipelineBuilder {
			return core.NewPipelineBuilder().AddParallel(engine(), e)
		}},
		{"in a group inside a parallel group", func(e *Element) *core.PipelineBuilder {
			return core.NewPipelineBuilder().AddFlowElement(engine()).
				AddFlowElement(core.NewParallelElements(nil,
					core.NewParallelElements(nil, e)))
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := sharing()
			defer e.Close()
			p, err := c.elements(e).Build()
			if err == nil {
				p.Close()
				t.Fatal("the Pipeline was built")
			}
			for _, want := range []string{"usage sharing element",
				"must be the last Flow Element", "outside any parallel group"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the error %q does not say %q", err, want)
				}
			}
		})
	}
}
