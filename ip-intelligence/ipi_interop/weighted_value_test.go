/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, 5 Charlotte Close,
 * Caversham, Reading, Berkshire, United Kingdom RG4 7BY.
 *
 * This Original Work is licensed under the European Union Public Licence (EUPL)
 * v.1.2 and is subject to its terms as set out below.
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
package ipi_interop

import (
	"reflect"
	"testing"
)

func TestValues_GetValueWeightByProperty(t *testing.T) {
	tests := []struct {
		name        string
		values      Values
		property    string
		wantValue   interface{}
		wantWeight  float64
		wantSuccess bool
	}{
		{
			name: "existing property with single value",
			values: Values{
				"browser": []*WeightedValue{
					{Value: "Chrome", Weight: 0.9},
				},
			},
			property:    "browser",
			wantValue:   "Chrome",
			wantWeight:  0.9,
			wantSuccess: true,
		},
		{
			name: "existing property with multiple values",
			values: Values{
				"platform": []*WeightedValue{
					{Value: "Windows", Weight: 0.8},
					{Value: "Linux", Weight: 0.2},
				},
			},
			property:    "platform",
			wantValue:   "Windows",
			wantWeight:  0.8,
			wantSuccess: true,
		},
		{
			name: "non-existing property",
			values: Values{
				"os": []*WeightedValue{
					{Value: "Android", Weight: 1.0},
				},
			},
			property:    "browser",
			wantValue:   "",
			wantWeight:  0,
			wantSuccess: false,
		},
		{
			name:        "empty values map",
			values:      Values{},
			property:    "anything",
			wantValue:   "",
			wantWeight:  0,
			wantSuccess: false,
		},
		{
			name: "property with empty value slice",
			values: Values{
				"empty": []*WeightedValue{},
			},
			property:    "empty",
			wantValue:   "",
			wantWeight:  0,
			wantSuccess: false,
		},
		{
			name: "different value types",
			values: Values{
				"string":  []*WeightedValue{{Value: "text", Weight: 0.5}},
				"int":     []*WeightedValue{{Value: 42, Weight: 0.7}},
				"bool":    []*WeightedValue{{Value: true, Weight: 0.9}},
				"float64": []*WeightedValue{{Value: 3.14, Weight: 0.6}},
			},
			property:    "int",
			wantValue:   42,
			wantWeight:  0.7,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotWeight, gotSuccess := tt.values.GetValueWeightByProperty(tt.property)

			if gotSuccess != tt.wantSuccess {
				t.Errorf("GetValueWeightByProperty() success = %v, want %v", gotSuccess, tt.wantSuccess)
			}

			if !reflect.DeepEqual(gotValue, tt.wantValue) {
				t.Errorf("GetValueWeightByProperty() value = %v, want %v", gotValue, tt.wantValue)
			}

			if gotWeight != tt.wantWeight {
				t.Errorf("GetValueWeightByProperty() weight = %v, want %v", gotWeight, tt.wantWeight)
			}
		})
	}
}

func TestValues_MultipleProperties(t *testing.T) {
	values := Values{
		"browser": []*WeightedValue{
			{Value: "Chrome", Weight: 0.9},
			{Value: "Firefox", Weight: 0.1},
		},
		"os": []*WeightedValue{
			{Value: "Windows", Weight: 0.7},
			{Value: "MacOS", Weight: 0.2},
			{Value: "Linux", Weight: 0.1},
		},
	}

	tests := []struct {
		name        string
		property    string
		wantValue   interface{}
		wantWeight  float64
		wantSuccess bool
	}{
		{
			name:        "get first browser",
			property:    "browser",
			wantValue:   "Chrome",
			wantWeight:  0.9,
			wantSuccess: true,
		},
		{
			name:        "get first os",
			property:    "os",
			wantValue:   "Windows",
			wantWeight:  0.7,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotWeight, gotSuccess := values.GetValueWeightByProperty(tt.property)

			if gotSuccess != tt.wantSuccess {
				t.Errorf("GetValueWeightByProperty() success = %v, want %v", gotSuccess, tt.wantSuccess)
			}

			if !reflect.DeepEqual(gotValue, tt.wantValue) {
				t.Errorf("GetValueWeightByProperty() value = %v, want %v", gotValue, tt.wantValue)
			}

			if gotWeight != tt.wantWeight {
				t.Errorf("GetValueWeightByProperty() weight = %v, want %v", gotWeight, tt.wantWeight)
			}
		})
	}
}

func TestValues_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		values      Values
		property    string
		wantValue   interface{}
		wantWeight  float64
		wantSuccess bool
	}{
		{
			name:        "nil map",
			values:      nil,
			property:    "anything",
			wantValue:   "",
			wantWeight:  0,
			wantSuccess: false,
		},
		{
			name: "empty string property",
			values: Values{
				"": []*WeightedValue{{Value: "value", Weight: 1.0}},
			},
			property:    "",
			wantValue:   "value",
			wantWeight:  1.0,
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotWeight, gotSuccess := tt.values.GetValueWeightByProperty(tt.property)

			if gotSuccess != tt.wantSuccess {
				t.Errorf("GetValueWeightByProperty() success = %v, want %v", gotSuccess, tt.wantSuccess)
			}

			if !reflect.DeepEqual(gotValue, tt.wantValue) {
				t.Errorf("GetValueWeightByProperty() value = %v, want %v", gotValue, tt.wantValue)
			}

			if gotWeight != tt.wantWeight {
				t.Errorf("GetValueWeightByProperty() weight = %v, want %v", gotWeight, tt.wantWeight)
			}
		})
	}
}

func TestValues_Append(t *testing.T) {
	tests := []struct {
		name           string
		initialValues  Values
		property       string
		value          interface{}
		expectedLength int
		expectedValue  interface{}
	}{
		{
			name:           "append to empty values",
			initialValues:  make(Values),
			property:       "browser",
			value:          "Chrome",
			expectedLength: 1,
			expectedValue:  "Chrome",
		},
		{
			name: "append to existing property",
			initialValues: Values{
				"browser": []*WeightedValue{
					{Value: "Firefox", Weight: 0.0},
				},
			},
			property:       "browser",
			value:          "Chrome",
			expectedLength: 2,
			expectedValue:  "Chrome",
		},
		{
			name: "append to new property in existing values",
			initialValues: Values{
				"os": []*WeightedValue{
					{Value: "Windows", Weight: 0.0},
				},
			},
			property:       "browser",
			value:          "Safari",
			expectedLength: 1,
			expectedValue:  "Safari",
		},
		{
			name:           "append integer value",
			initialValues:  make(Values),
			property:       "version",
			value:          42,
			expectedLength: 1,
			expectedValue:  42,
		},
		{
			name:           "append boolean value",
			initialValues:  make(Values),
			property:       "mobile",
			value:          true,
			expectedLength: 1,
			expectedValue:  true,
		},
		{
			name:           "append float64 value",
			initialValues:  make(Values),
			property:       "score",
			value:          3.14159,
			expectedLength: 1,
			expectedValue:  3.14159,
		},
		{
			name:           "append nil value",
			initialValues:  make(Values),
			property:       "nullable",
			value:          nil,
			expectedLength: 1,
			expectedValue:  nil,
		},
		{
			name:           "append with empty string property",
			initialValues:  make(Values),
			property:       "",
			value:          "empty_key",
			expectedLength: 1,
			expectedValue:  "empty_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := tt.initialValues
			values.Append(tt.property, tt.value)

			// Check if property exists
			propertyValues, exists := values[tt.property]
			if !exists {
				t.Errorf("Append() property %s was not created", tt.property)
				return
			}

			// Check length
			if len(propertyValues) != tt.expectedLength {
				t.Errorf("Append() length = %d, want %d", len(propertyValues), tt.expectedLength)
			}

			// Check the last added value (should be at the end)
			lastIndex := len(propertyValues) - 1
			if lastIndex < 0 {
				t.Errorf("Append() no values found in property %s", tt.property)
				return
			}

			lastValue := propertyValues[lastIndex]
			if !reflect.DeepEqual(lastValue.Value, tt.expectedValue) {
				t.Errorf("Append() value = %v, want %v", lastValue.Value, tt.expectedValue)
			}

			// Weight should be 0.0 for non-weighted append
			if lastValue.Weight != 0.0 {
				t.Errorf("Append() weight = %v, want 0.0", lastValue.Weight)
			}
		})
	}
}

func TestValues_InitProperty(t *testing.T) {
	tests := []struct {
		name          string
		initialValues Values
		property      string
		expectExists  bool
		expectLength  int
	}{
		{
			name:          "init property in empty values",
			initialValues: make(Values),
			property:      "browser",
			expectExists:  true,
			expectLength:  0,
		},
		{
			name: "init property that already exists",
			initialValues: Values{
				"browser": []*WeightedValue{
					{Value: "Chrome", Weight: 0.9},
				},
			},
			property:     "browser",
			expectExists: true,
			expectLength: 1, // Should not change existing values
		},
		{
			name: "init new property in existing values",
			initialValues: Values{
				"os": []*WeightedValue{
					{Value: "Windows", Weight: 0.7},
				},
			},
			property:     "browser",
			expectExists: true,
			expectLength: 0,
		},
		{
			name:          "init property with empty string key",
			initialValues: make(Values),
			property:      "",
			expectExists:  true,
			expectLength:  0,
		},
		{
			name: "init property that exists but is empty",
			initialValues: Values{
				"empty": []*WeightedValue{},
			},
			property:     "empty",
			expectExists: true,
			expectLength: 0, // Should remain empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := tt.initialValues
			values.InitProperty(tt.property)

			// Check if property exists after init
			propertyValues, exists := values[tt.property]
			if exists != tt.expectExists {
				t.Errorf("InitProperty() exists = %v, want %v", exists, tt.expectExists)
			}

			if !exists {
				return
			}

			// Check length
			if len(propertyValues) != tt.expectLength {
				t.Errorf("InitProperty() length = %d, want %d", len(propertyValues), tt.expectLength)
			}

			// Verify it's a valid slice (not nil)
			if propertyValues == nil {
				t.Errorf("InitProperty() created nil slice for property %s", tt.property)
			}
		})
	}
}

func TestValues_AppendAfterInit(t *testing.T) {
	// Test the combination of InitProperty followed by Append
	values := make(Values)
	property := "test_property"

	// Initialize the property
	values.InitProperty(property)

	// Verify it's initialized correctly
	if _, exists := values[property]; !exists {
		t.Errorf("InitProperty() failed to create property %s", property)
	}

	if len(values[property]) != 0 {
		t.Errorf("InitProperty() created non-empty slice, length = %d", len(values[property]))
	}

	// Append some values
	values.Append(property, "value1")
	values.Append(property, "value2")

	// Verify append worked correctly
	if len(values[property]) != 2 {
		t.Errorf("Append() after InitProperty() length = %d, want 2", len(values[property]))
	}

	// Verify values are in correct order
	if values[property][0].Value != "value1" || values[property][0].Weight != 0.0 {
		t.Errorf("First appended value incorrect: got %v with weight %v",
			values[property][0].Value, values[property][0].Weight)
	}

	if values[property][1].Value != "value2" || values[property][1].Weight != 0.0 {
		t.Errorf("Second appended value incorrect: got %v with weight %v",
			values[property][1].Value, values[property][1].Weight)
	}
}

func TestValues_MultipleAppendsOrder(t *testing.T) {
	// Test that multiple appends maintain correct order
	values := make(Values)
	property := "order_test"

	// Append multiple values
	testValues := []interface{}{
		"first",
		"second",
		"third",
		42,
		true,
	}

	for _, tv := range testValues {
		values.Append(property, tv)
	}

	// Verify all values are present in correct order
	if len(values[property]) != len(testValues) {
		t.Errorf("MultipleAppendsOrder() length = %d, want %d", len(values[property]), len(testValues))
	}

	for i, tv := range testValues {
		if !reflect.DeepEqual(values[property][i].Value, tv) {
			t.Errorf("MultipleAppendsOrder() value at index %d = %v, want %v",
				i, values[property][i].Value, tv)
		}
		if values[property][i].Weight != 0.0 {
			t.Errorf("MultipleAppendsOrder() weight at index %d = %v, want 0.0",
				i, values[property][i].Weight)
		}
	}
}

func TestValues_AppendWithWeight(t *testing.T) {
	// Test AppendWithWeight specifically for MCC property
	tests := []struct {
		name           string
		initialValues  Values
		property       string
		value          interface{}
		weight         float64
		expectedLength int
		expectedValue  interface{}
		expectedWeight float64
	}{
		{
			name:           "append MCC with weight",
			initialValues:  make(Values),
			property:       "Mcc",
			value:          "310",
			weight:         0.95,
			expectedLength: 1,
			expectedValue:  "310",
			expectedWeight: 0.95,
		},
		{
			name: "append MCC with weight to existing",
			initialValues: Values{
				"Mcc": []*WeightedValue{
					{Value: "250", Weight: 0.8},
				},
			},
			property:       "Mcc",
			value:          "310",
			weight:         0.9,
			expectedLength: 2,
			expectedValue:  "310",
			expectedWeight: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := tt.initialValues
			values.AppendWithWeight(tt.property, tt.value, tt.weight)

			// Check if property exists
			propertyValues, exists := values[tt.property]
			if !exists {
				t.Errorf("AppendWithWeight() property %s was not created", tt.property)
				return
			}

			// Check length
			if len(propertyValues) != tt.expectedLength {
				t.Errorf("AppendWithWeight() length = %d, want %d", len(propertyValues), tt.expectedLength)
			}

			// Check the last added value (should be at the end)
			lastIndex := len(propertyValues) - 1
			if lastIndex < 0 {
				t.Errorf("AppendWithWeight() no values found in property %s", tt.property)
				return
			}

			lastValue := propertyValues[lastIndex]
			if !reflect.DeepEqual(lastValue.Value, tt.expectedValue) {
				t.Errorf("AppendWithWeight() value = %v, want %v", lastValue.Value, tt.expectedValue)
			}

			if lastValue.Weight != tt.expectedWeight {
				t.Errorf("AppendWithWeight() weight = %v, want %v", lastValue.Weight, tt.expectedWeight)
			}
		})
	}
}

func TestValues_GetValueByProperty(t *testing.T) {
	tests := []struct {
		name        string
		values      Values
		property    string
		wantValue   interface{}
		wantSuccess bool
	}{
		{
			name: "existing property with single value",
			values: Values{
				"browser": []*WeightedValue{
					{Value: "Chrome", Weight: 0.0},
				},
			},
			property:    "browser",
			wantValue:   "Chrome",
			wantSuccess: true,
		},
		{
			name: "existing property with multiple values",
			values: Values{
				"platform": []*WeightedValue{
					{Value: "Windows", Weight: 0.0},
					{Value: "Linux", Weight: 0.0},
				},
			},
			property:    "platform",
			wantValue:   "Windows",
			wantSuccess: true,
		},
		{
			name: "non-existing property",
			values: Values{
				"os": []*WeightedValue{
					{Value: "Android", Weight: 0.0},
				},
			},
			property:    "browser",
			wantValue:   "",
			wantSuccess: false,
		},
		{
			name:        "empty values map",
			values:      Values{},
			property:    "anything",
			wantValue:   "",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotSuccess := tt.values.GetValueByProperty(tt.property)

			if gotSuccess != tt.wantSuccess {
				t.Errorf("GetValueByProperty() success = %v, want %v", gotSuccess, tt.wantSuccess)
			}

			if !reflect.DeepEqual(gotValue, tt.wantValue) {
				t.Errorf("GetValueByProperty() value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}
