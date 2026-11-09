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
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// recordsEngine writes a Property whose value is a list of records, each
// with Property names in mixed case, beside values that are set but nil.
type recordsEngine struct{ *core.BaseElement }

func newRecordsEngine() *recordsEngine {
	return &recordsEngine{BaseElement: core.NewBaseElement("devices",
		core.NewWhitelistFilter(), nil)}
}

func (e *recordsEngine) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("devices", func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	record := core.NewBaseElementData()
	record.Set("HardwareModel", engines.NewAspectPropertyValue("X1"))
	record.Set("HardwareVendor", engines.NoValue[string]("no vendor"))
	record.Set("SetButNil", engines.NewAspectPropertyValue[any](nil))
	record.Set("NilList", engines.NewAspectPropertyValue[[]string](nil))
	d.Set("Profiles", []core.ElementData{record})
	d.Set("SetButNil", engines.NewAspectPropertyValue[any](nil))
	d.Set("NilList", []string(nil))
	return nil
}

// A record's Property names are written in lower case, as the top-level keys
// are, and a value that is set but nil has no entry and no nullreason, at
// the top level and in a record, while a Property assigned no value is
// written as null with its nullreason. The .NET JSON Builder does the same.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/json-builder.md#process
func TestJSONBuilderRecordsAndNilValues(t *testing.T) {
	result := buildJSON(t, newRecordsEngine())
	devices, ok := result["devices"].(map[string]any)
	if !ok {
		t.Fatalf("no devices entry in %v", result)
	}
	for _, key := range []string{"setbutnil", "setbutnilnullreason",
		"nillist", "nillistnullreason"} {
		if v, present := devices[key]; present {
			t.Errorf("devices has %q as %v, want no entry", key, v)
		}
	}
	profiles, ok := devices["profiles"].([]any)
	if !ok || len(profiles) != 1 {
		t.Fatalf("profiles is %v, want one record", devices["profiles"])
	}
	want := map[string]any{
		"hardwaremodel":            "X1",
		"hardwarevendor":           nil,
		"hardwarevendornullreason": "no vendor",
	}
	if !reflect.DeepEqual(profiles[0], want) {
		t.Errorf("the record is %v, want %v", profiles[0], want)
	}
}
