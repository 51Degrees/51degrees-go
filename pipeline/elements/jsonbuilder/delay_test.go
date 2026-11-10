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
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// delayedNoValueEngine gives two Properties whose delay execution flag is
// set, one with a value and one assigned no value.
type delayedNoValueEngine struct{ *core.BaseElement }

func newDelayedNoValueEngine() *delayedNoValueEngine {
	e := &delayedNoValueEngine{BaseElement: core.NewBaseElement("timing",
		core.NewWhitelistFilter(), nil)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: "withvalue", Element: e, Available: true, DelayExecution: true},
		{Name: "novalue", Element: e, Available: true, DelayExecution: true},
	})
	return e
}

func (e *delayedNoValueEngine) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("timing", func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	d.Set("withvalue", engines.NewAspectPropertyValue("x"))
	d.Set("novalue", engines.NoValue[string]("not gathered yet"))
	return nil
}

// delayexecution is written only for a Property that has a value, as the
// .NET and Java JSON Builders write it. A Property assigned no value is
// written as null with its nullreason and no delayexecution.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/json-builder.md#process
func TestJSONBuilderDelayExecutionOnlyWithAValue(t *testing.T) {
	result := buildJSON(t, newDelayedNoValueEngine())
	timing, ok := result["timing"].(map[string]any)
	if !ok {
		t.Fatalf("no timing entry in %v", result)
	}
	if timing["withvaluedelayexecution"] != true {
		t.Errorf("withvaluedelayexecution is %v, want true",
			timing["withvaluedelayexecution"])
	}
	if v, present := timing["novaluedelayexecution"]; present {
		t.Errorf("timing has novaluedelayexecution as %v, want no entry", v)
	}
	if v, present := timing["novalue"]; !present || v != nil {
		t.Errorf("novalue is %v (present %v), want null", v, present)
	}
	if timing["novaluenullreason"] != "not gathered yet" {
		t.Errorf("novaluenullreason is %v, want the no value message",
			timing["novaluenullreason"])
	}
}
