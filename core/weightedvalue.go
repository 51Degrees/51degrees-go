package core

import (
	"encoding/json"
	"math"
)

// MaxRawWeighting is the value that the raw weightings of all values for a
// single property sum to, representing a 1.0 likelihood.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/weighted-values.md
const MaxRawWeighting = math.MaxUint16

// WeightedValue pairs a value with a weighting that expresses how likely it is
// relative to other values for the same property. It is the analogue of
// IWeightedValue<T>.
//
// The raw weighting is a 16-bit unsigned integer; the weightings of all values
// for a property sum to MaxRawWeighting (65535).
type WeightedValue[T any] struct {
	value        T
	rawWeighting uint16
}

// NewWeightedValue creates a WeightedValue from a value and its raw weighting.
func NewWeightedValue[T any](value T, rawWeighting uint16) WeightedValue[T] {
	return WeightedValue[T]{value: value, rawWeighting: rawWeighting}
}

// Value returns the underlying value.
func (w WeightedValue[T]) Value() T { return w.value }

// RawWeighting returns the raw 16-bit weighting as stored.
func (w WeightedValue[T]) RawWeighting() uint16 { return w.rawWeighting }

// Weighting returns the weighting as a probability in the range [0, 1], computed
// by dividing the raw weighting by MaxRawWeighting.
func (w WeightedValue[T]) Weighting() float64 {
	return float64(w.rawWeighting) / float64(MaxRawWeighting)
}

// MarshalJSON serializes the weighted value in the form expected throughout the
// API: {"RawWeighting": <uint16>, "Value": <value>}.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/weighted-values.md#serializing
func (w WeightedValue[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		RawWeighting uint16 `json:"RawWeighting"`
		Value        T      `json:"Value"`
	}{RawWeighting: w.rawWeighting, Value: w.value})
}
