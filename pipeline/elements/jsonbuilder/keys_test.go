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
	"reflect"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// mixedCaseEngine writes its Element Data under a Data Key and Property
// names that are not in lower case.
type mixedCaseEngine struct{ *core.BaseElement }

func newMixedCaseEngine() *mixedCaseEngine {
	e := &mixedCaseEngine{BaseElement: core.NewBaseElement("Hardware",
		core.NewWhitelistFilter(), nil)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: "IsMobile", Element: e, Available: true},
		{Name: "ZipCode", Element: e, Available: true},
		{Name: "ProfileScript", Element: e, Available: true,
			DelayExecution: true, EvidenceProperties: []string{"DelayedScript"}},
		{Name: "DelayedScript", Element: e, Available: true, DelayExecution: true},
	})
	return e
}

func (e *mixedCaseEngine) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("Hardware", func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	d.Set("IsMobile", true)
	d.Set("ZipCode", engines.NoValue[string]("requires client-side evidence"))
	d.Set("ProfileScript", core.JavaScript("profile()"))
	d.Set("DelayedScript", core.JavaScript("delayed()"))
	return nil
}

// The key of each Element Data's entry and of each Property, meta-Property
// suffix included, is written in lower case, apart from the top-level
// javascriptProperties. The names javascriptProperties and
// evidenceproperties list are the same lower case names, so each finds its
// entry in the JSON.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/json-builder.md#process
func TestJSONBuilderKeysInLowerCase(t *testing.T) {
	result := buildJSON(t, newMixedCaseEngine())
	if _, present := result["Hardware"]; present {
		t.Error("the Data Key Hardware is written as it was given")
	}
	hw, ok := result["hardware"].(map[string]any)
	if !ok {
		t.Fatalf("no hardware entry in %v", result)
	}
	for key := range hw {
		if key != strings.ToLower(key) {
			t.Errorf("the key %q is not in lower case", key)
		}
	}
	for _, key := range []string{"ismobile", "zipcode", "zipcodenullreason",
		"profilescript", "profilescriptdelayexecution",
		"profilescriptevidenceproperties", "delayedscript",
		"delayedscriptdelayexecution"} {
		if _, present := hw[key]; !present {
			t.Errorf("the key %q is missing from %v", key, hw)
		}
	}
	if got := hw["profilescriptevidenceproperties"]; !reflect.DeepEqual(got,
		[]any{"hardware.delayedscript"}) {
		t.Errorf("profilescriptevidenceproperties is %v, want "+
			"[hardware.delayedscript]", got)
	}

	js, ok := result["javascriptProperties"].([]any)
	if !ok {
		t.Fatalf("javascriptProperties is %v", result["javascriptProperties"])
	}
	names := map[any]bool{}
	for _, name := range js {
		names[name] = true
	}
	for _, want := range []string{"hardware.profilescript", "hardware.delayedscript"} {
		if !names[want] {
			t.Errorf("javascriptProperties %v does not hold %s", js, want)
		}
	}
}
