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

// A value converted from Azimuth or Declination is given rounded to 3
// decimal places, as a number and as a point in a WKT shape. A stored
// Declination of 6751 converts to 18.542741172521133 and is given as 18.543.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/value-types.md#rounding-of-azimuth-and-declination-values
func TestAzimuthAndDeclinationAreRoundedTo3Places(t *testing.T) {
	for _, test := range []struct {
		name   string
		got    float64
		wanted float64
	}{
		{"Declination 6751", DeclinationToDegrees(6751), 18.543},
		{"Azimuth 6751", AzimuthToDegrees(6751), 37.085},
		{"Azimuth -12345", AzimuthToDegrees(-12345), -67.815},
		{"Declination 32767", DeclinationToDegrees(32767), 90},
	} {
		if test.got != test.wanted {
			t.Errorf("%s gave %v, want %v", test.name, test.got, test.wanted)
		}
	}

	data := []byte{
		0x00,       // big-endian
		0x03,       // polygon
		0x00, 0x01, // 1 ring
		0x00, 0x02, // 2 points
		0x1A, 0x5F, 0x1A, 0x5F, // (6751, 6751)
		0xCF, 0xC7, 0x00, 0x00, // (-12345, 0)
	}
	wkt, err := DecodeWKBRPolygon(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if want := "POLYGON ((37.085 18.543, -67.815 0))"; wkt.String() != want {
		t.Errorf("the polygon is %q, want %q", wkt.String(), want)
	}
}

// A WKT value written to JSON is an object with one member, value, holding
// the WKT text.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/value-types.md#wkt
func TestAWktValueIsWrittenToJSONAsAnObject(t *testing.T) {
	encoded, err := json.Marshal(map[string]any{
		"areas": WktString("POINT(2 4)")})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"areas":{"value":"POINT(2 4)"}}`; string(encoded) != want {
		t.Errorf("the JSON is %s, want %s", encoded, want)
	}
}
