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

// MarshalJSON serializes the weighted value with the lower case keys the
// JSON Builder and the cloud service use, as
// {"rawweighting": <uint16>, "value": <value>}.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/weighted-values.md#serializing
func (w WeightedValue[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		RawWeighting uint16 `json:"rawweighting"`
		Value        T      `json:"value"`
	}{RawWeighting: w.rawWeighting, Value: w.value})
}

// UnmarshalJSON reconstructs a weighted value from the form produced by
// MarshalJSON, so a value serialized on one side of the API can be rebuilt on
// the other. Without it the unexported fields could not be repopulated. The
// keys match without regard to case, so the form with capitals that earlier
// versions wrote is read too.
func (w *WeightedValue[T]) UnmarshalJSON(data []byte) error {
	var aux struct {
		RawWeighting uint16 `json:"rawweighting"`
		Value        T      `json:"value"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	w.rawWeighting = aux.RawWeighting
	w.value = aux.Value
	return nil
}
