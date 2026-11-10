package ipi_interop

import (
	"math"
	"testing"
)

// A value converted from Azimuth or Declination is given to 3 decimal places,
// as the float64 nearest the rounded decimal. The first case is the
// specification's example, a stored Declination of 6751.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/value-types.md#rounding-of-azimuth-and-declination-values
func TestRoundCoordinate(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{90.0 * (6751 / 32767.0), 18.543},
		{-122.41941, -122.419},
		{179.99961, 180},
		{51.5, 51.5},
		{0, 0},
	}
	for _, c := range cases {
		if got := roundCoordinate(c.in); got != c.want {
			t.Errorf("roundCoordinate(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// A value that rounds to zero is given as zero, not as negative zero, which
// matches the text the C library writes for it.
func TestRoundCoordinateGivesNoNegativeZero(t *testing.T) {
	for _, in := range []float64{-0.0004, math.Copysign(0, -1)} {
		if got := roundCoordinate(in); got != 0 || math.Signbit(got) {
			t.Errorf("roundCoordinate(%v) = %v, want 0", in, got)
		}
	}
}
