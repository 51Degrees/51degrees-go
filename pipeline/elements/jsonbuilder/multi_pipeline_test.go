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

package jsonbuilder

import (
	"encoding/json"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// timingEngine gives the Property withvalue a value, and its metadata says
// whether the Property's execution is delayed.
type timingEngine struct{ *core.BaseElement }

func newTimingEngine(delayed bool) *timingEngine {
	e := &timingEngine{BaseElement: core.NewBaseElement("timing",
		core.NewWhitelistFilter(), nil)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: "withvalue", Element: e, Available: true,
			DelayExecution: delayed},
	})
	return e
}

func (e *timingEngine) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("timing", func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	d.Set("withvalue", engines.NewAspectPropertyValue("x"))
	return nil
}

// pipelineWith builds a Pipeline of a Sequence Element, the engine and the
// JSON Builder given.
func pipelineWith(t *testing.T, engine core.FlowElement,
	builder *Element) core.Pipeline {
	t.Helper()
	p, err := core.NewPipelineBuilder().
		AddFlowElement(sequence.New(nil)).
		AddFlowElement(engine).
		AddFlowElement(builder).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// timingJSON processes new Flow Data in the Pipeline and returns the timing
// entry of the JSON the builder wrote.
func timingJSON(t *testing.T, p core.Pipeline) map[string]any {
	t.Helper()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get json: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(data.(*Data).JSON()), &result); err != nil {
		t.Fatalf("invalid JSON produced: %v", err)
	}
	timing, ok := result["timing"].(map[string]any)
	if !ok {
		t.Fatalf("no timing entry in %v", result)
	}
	return timing
}

// One JSON Builder added to two Pipelines writes each Pipeline's JSON from the
// Property metadata of that Pipeline, whichever Pipeline it ran in before.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/advanced-features/multi-pipeline-elements.md?plain=1#L16-L19
func TestOneJSONBuilderInTwoPipelines(t *testing.T) {
	builder := New(nil, nil)
	delayed := pipelineWith(t, newTimingEngine(true), builder)
	notDelayed := pipelineWith(t, newTimingEngine(false), builder)
	for i, run := range []struct {
		name     string
		pipeline core.Pipeline
		want     bool
	}{
		{"the delayed Pipeline", delayed, true},
		{"the Pipeline that does not delay", notDelayed, false},
		{"the delayed Pipeline", delayed, true},
	} {
		_, got := timingJSON(t, run.pipeline)["withvaluedelayexecution"]
		if got != run.want {
			t.Errorf("run %d in %s: withvaluedelayexecution present is "+
				"%v, want %v", i+1, run.name, got, run.want)
		}
	}
}
