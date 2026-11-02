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

import "testing"

func TestNewPropertiesRequired(t *testing.T) {
	tests := []struct {
		name       string
		properties string
	}{
		{
			name:       "basic properties",
			properties: "property1,property2,property3",
		},
		{
			name:       "empty properties",
			properties: "",
		},
		{
			name:       "single property",
			properties: "property1",
		},
		{
			name:       "properties with spaces",
			properties: "property 1, property 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := NewPropertiesRequired(tt.properties)
			defer props.Free()

			if props == nil {
				t.Fatal("NewPropertiesRequired returned nil")
			}

			if props.CPtr == nil {
				t.Fatal("NewPropertiesRequired created properties with nil CPtr")
			}

			if props.CPtr.string == nil {
				t.Fatal("NewPropertiesRequired created properties with nil string")
			}

			// Check if the string was properly set
			if got := props.Properties(); got != tt.properties {
				t.Errorf("Properties() = %v, want %v", got, tt.properties)
			}
		})
	}
}

func TestPropertiesRequired_Free(t *testing.T) {
	tests := []struct {
		name       string
		properties string
	}{
		{
			name:       "free basic properties",
			properties: "test1,test2",
		},
		{
			name:       "free empty properties",
			properties: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := NewPropertiesRequired(tt.properties)

			// Store CPtr for later verification
			originalCPtr := props.CPtr

			if originalCPtr == nil {
				t.Fatal("Properties created with nil CPtr")
			}

			props.Free()

			if props.CPtr != nil {
				t.Error("Free() did not set CPtr to nil")
			}
		})
	}
}

func TestPropertiesRequired_Properties(t *testing.T) {
	tests := []struct {
		name       string
		properties string
	}{
		{
			name:       "get basic properties",
			properties: "prop1,prop2,prop3",
		},
		{
			name:       "get empty properties",
			properties: "",
		},
		{
			name:       "get properties with special characters",
			properties: "prop-1,prop_2,prop.3",
		},
		{
			name:       "get properties with spaces",
			properties: "prop 1, prop 2, prop 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := NewPropertiesRequired(tt.properties)
			defer props.Free()

			if got := props.Properties(); got != tt.properties {
				t.Errorf("Properties() = %v, want %v", got, tt.properties)
			}
		})
	}
}

func TestPropertiesRequired_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		properties string
		testFunc   func(*testing.T, *PropertiesRequired)
	}{
		{
			name:       "very long property string",
			properties: createLongString(1000),
			testFunc: func(t *testing.T, p *PropertiesRequired) {
				got := p.Properties()
				if len(got) != 1000 {
					t.Errorf("Properties() returned string of length %d, want %d", len(got), 1000)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			props := NewPropertiesRequired(tt.properties)
			defer props.Free()

			if tt.testFunc != nil {
				tt.testFunc(t, props)
			}
		})
	}
}

// Helper function to create a string of specified length
func createLongString(length int) string {
	result := make([]rune, length)
	for i := 0; i < length; i++ {
		result[i] = 'a'
	}
	return string(result)
}
