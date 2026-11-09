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
	"encoding/json"
	"testing"
)

// A Property holding weighted values serializes, as the JSON Builder writes
// it, in the form the specification's example shows.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/weighted-values.md#serializing
func TestWeightedValuesSerializeAsTheExample(t *testing.T) {
	property := map[string]any{"weightedpropertyname": []WeightedValue[string]{
		NewWeightedValue("value 1", 32768),
		NewWeightedValue("value 2", 32767),
	}}
	b, err := json.Marshal(property)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"weightedpropertyname":[` +
		`{"rawweighting":32768,"value":"value 1"},` +
		`{"rawweighting":32767,"value":"value 2"}]}`
	if string(b) != want {
		t.Fatalf("serialized as %s, want %s", b, want)
	}
}

// Weighted values read back from the lower case form a cloud response
// carries, and from the form with capitals that earlier versions wrote.
func TestWeightedValuesRead(t *testing.T) {
	for _, raw := range []string{
		`[{"rawweighting":49151,"value":"GB"},{"rawweighting":16384,"value":"IE"}]`,
		`[{"RawWeighting":49151,"Value":"GB"},{"RawWeighting":16384,"Value":"IE"}]`,
	} {
		var values []WeightedValue[string]
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			t.Fatalf("read %s: %v", raw, err)
		}
		if len(values) != 2 ||
			values[0].Value() != "GB" || values[0].RawWeighting() != 49151 ||
			values[1].Value() != "IE" || values[1].RawWeighting() != 16384 {
			t.Errorf("read %s as %+v", raw, values)
		}
	}
}
