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

package ipi_onpremise

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// coordinatePlaces is how many decimal places a value converted from
// Azimuth or Declination may have.
const coordinatePlaces = 3

// decimalPlaces returns how many decimal places a coordinate value has, read
// from the shortest text that gives the same value.
func decimalPlaces(t *testing.T, value any) int {
	t.Helper()
	var text string
	switch v := value.(type) {
	case float64:
		text = strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		text = strconv.FormatFloat(float64(v), 'f', -1, 32)
	case string:
		text = v
	default:
		t.Fatalf("a coordinate is a %T, %v", value, value)
	}
	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		return len(text) - dot - 1
	}
	return 0
}

// Latitude and Longitude are converted from Declination and Azimuth, so each
// value is given to 3 decimal places.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/value-types.md#rounding-of-azimuth-and-declination-values
func TestCoordinatesToThreeDecimalPlaces(t *testing.T) {
	e := propertiesTestEngine(t, testDataPath(t, enterpriseTestDataFile), nil, false)
	checked := 0
	for _, address := range propertiesTestAddresses {
		values, err := e.Process(address)
		if err != nil {
			t.Fatalf("process %s: %v", address, err)
		}
		for name, list := range values {
			if !strings.EqualFold(name, "Latitude") &&
				!strings.EqualFold(name, "Longitude") {
				continue
			}
			for _, v := range list {
				checked++
				if places := decimalPlaces(t, v.Value); places > coordinatePlaces {
					t.Errorf("%s %s is %s, with %d decimal places", address, name,
						fmt.Sprint(v.Value), places)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatalf("%s gave no Latitude or Longitude", enterpriseTestDataFile)
	}
}
