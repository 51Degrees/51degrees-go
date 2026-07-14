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
	"encoding/binary"
	"fmt"
	"strings"
)

// This file implements the space-optimized value types described in
// https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/value-types.md
//
// These are primarily used by the IP Intelligence Engine, but the decoding
// logic is part of the core Pipeline so that it can be shared.

const int16Max = 32767.0

// AzimuthToDegrees converts a stored int16 Azimuth (compressed longitude) back
// to a floating point longitude value in the range [-180, 180]. The result is
// clamped because the int16 minimum (-32768) divided by int16Max (32767) would
// otherwise fall just outside the range.
func AzimuthToDegrees(stored int16) float64 {
	return clampDegrees(180.0*(float64(stored)/int16Max), 180.0)
}

// DeclinationToDegrees converts a stored int16 Declination (compressed latitude)
// back to a floating point latitude value in the range [-90, 90]. The result is
// clamped for the same boundary reason as AzimuthToDegrees.
func DeclinationToDegrees(stored int16) float64 {
	return clampDegrees(90.0*(float64(stored)/int16Max), 90.0)
}

// clampDegrees constrains v to [-limit, limit].
func clampDegrees(v, limit float64) float64 {
	if v > limit {
		return limit
	}
	if v < -limit {
		return -limit
	}
	return v
}

// WktString holds a Well-Known Text representation of a geometry. It is the
// analogue of the WktString interface in the reference implementations, which
// extends string.
type WktString string

// String returns the WKT string.
func (w WktString) String() string { return string(w) }

// WKBR geometry type identifiers (reduced Well-Known Binary).
const (
	wkbrTypePolygon = 3
)

// DecodeWKBRPolygon decodes a reduced Well-Known Binary (WKBR) polygon into a
// WKT string. The reduced format halves the size of the count fields and stores
// coordinates as int16 values using the Azimuth (x = longitude) and Declination
// (y = latitude) compression.
//
// The structure decoded is:
//
//	struct WKBRPolygon { byte byteOrder; byte wkbType; uint16 numRings; WKBRLinearRing rings[numRings] }
//	struct WKBRLinearRing { uint16 numPoints; WKBRPoint points[numPoints] }
//	struct WKBRPoint { int16 x; int16 y }
func DecodeWKBRPolygon(data []byte) (WktString, error) {
	if len(data) < 4 {
		return "", fmt.Errorf("wkbr polygon: need at least 4 bytes, got %d", len(data))
	}
	order, err := byteOrder(data[0])
	if err != nil {
		return "", err
	}
	if t := data[1]; t != wkbrTypePolygon {
		return "", fmt.Errorf("wkbr polygon: unexpected geometry type %d", t)
	}
	pos := 2
	numRings := int(order.Uint16(data[pos:]))
	pos += 2

	// Grow lazily rather than pre-allocating from the untrusted count fields, so
	// a malformed buffer cannot trigger a huge allocation before the bounds
	// checks below reject it.
	var rings []string
	for r := 0; r < numRings; r++ {
		if pos+2 > len(data) {
			return "", fmt.Errorf("wkbr polygon: truncated reading ring %d count", r)
		}
		numPoints := int(order.Uint16(data[pos:]))
		pos += 2
		var coords []string
		for p := 0; p < numPoints; p++ {
			if pos+4 > len(data) {
				return "", fmt.Errorf("wkbr polygon: truncated reading ring %d point %d", r, p)
			}
			x := int16(order.Uint16(data[pos:]))
			y := int16(order.Uint16(data[pos+2:]))
			pos += 4
			// x = longitude (Azimuth), y = latitude (Declination), per the spec.
			coords = append(coords, fmt.Sprintf("%g %g", AzimuthToDegrees(x), DeclinationToDegrees(y)))
		}
		rings = append(rings, "("+strings.Join(coords, ", ")+")")
	}
	return WktString("POLYGON (" + strings.Join(rings, ", ") + ")"), nil
}

func byteOrder(b byte) (binary.ByteOrder, error) {
	switch b {
	case 0:
		return binary.BigEndian, nil
	case 1:
		return binary.LittleEndian, nil
	default:
		return nil, fmt.Errorf("wkbr: invalid byte order marker %d", b)
	}
}
