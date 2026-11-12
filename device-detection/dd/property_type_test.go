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

package dd

import (
	"reflect"
	"testing"
)

// The type of a Boolean property, read by the index of its name.
func TestPropertyTypeOfABooleanProperty(t *testing.T) {
	manager := initResourceManager("IsMobile", t)
	defer freeResourceManager(manager, t)
	results := NewResultsHash(manager, 1, 0)
	defer results.Free()

	index := results.RequiredPropertyIndexFromName("IsMobile")
	if index < 0 {
		t.Fatalf("Expected \"IsMobile\" to have an index.\n")
	}
	if got := results.PropertyType(index); got != reflect.TypeOf(false) {
		t.Errorf("Expected \"IsMobile\" to be read as bool but got %v.\n", got)
	}
}

// The type of a string property, read by the index of its name.
func TestPropertyTypeOfAStringProperty(t *testing.T) {
	manager := initResourceManager("IsMobile,BrowserName", t)
	defer freeResourceManager(manager, t)
	results := NewResultsHash(manager, 1, 0)
	defer results.Free()

	index := results.RequiredPropertyIndexFromName("BrowserName")
	if index < 0 {
		t.Fatalf("Expected \"BrowserName\" to have an index.\n")
	}
	if got := results.PropertyType(index); got != reflect.TypeOf("") {
		t.Errorf("Expected \"BrowserName\" to be read as string but got %v.\n",
			got)
	}
}

// Every property of the data file has a type, whichever file the tests found,
// under every performance profile, and reading the types leaves no native
// memory behind.
func TestPropertyTypeOfEveryAvailableProperty(t *testing.T) {
	testDifferentPerformanceProfiles("", func(
		manager *ResourceManager, t *testing.T) {
		results := NewResultsHash(manager, 1, 0)
		defer results.Free()

		available := results.AvailableProperties()
		if len(available) == 0 {
			t.Fatalf("Expected the data file to have properties.\n")
		}
		for i, name := range available {
			if results.PropertyType(i) == nil {
				t.Errorf("Expected a type for %q at index %d.\n", name, i)
			}
		}
	}, t)
}

// An index outside the names the manager was created with, and a name the
// data set has no property for, both give nil rather than a type.
func TestPropertyTypeOutsideTheNamesGiven(t *testing.T) {
	manager := initResourceManager("IsMobile,NoSuchProperty", t)
	defer freeResourceManager(manager, t)
	results := NewResultsHash(manager, 1, 0)
	defer results.Free()

	for _, index := range []int{-1, 2, 1000} {
		if got := results.PropertyType(index); got != nil {
			t.Errorf("Expected nil at index %d but got %v.\n", index, got)
		}
	}
	index := results.RequiredPropertyIndexFromName("NoSuchProperty")
	if index < 0 {
		t.Fatalf("Expected \"NoSuchProperty\" to keep its place among the " +
			"names given.\n")
	}
	if got := results.PropertyType(index); got != nil {
		t.Errorf("Expected nil for a name the data set has no property "+
			"for but got %v.\n", got)
	}
}

// The Go type each value type a Hash data file holds is read as, which is the
// mapping the C library's own names for those types describe.
func TestPropertyValueTypeGoType(t *testing.T) {
	data := []struct {
		valueType propertyValueType
		isList    bool
		expected  reflect.Type
	}{
		{stringValueType, false, reflect.TypeOf("")},
		{stringValueType, true, reflect.TypeOf([]string(nil))},
		{integerValueType, false, reflect.TypeOf(int(0))},
		{doubleValueType, false, reflect.TypeOf(float64(0))},
		{booleanValueType, false, reflect.TypeOf(false)},
		{javaScriptValueType, false, reflect.TypeOf("")},
		// A type no Hash data file holds, which the C library names a
		// string.
		{propertyValueType(200), false, reflect.TypeOf("")},
	}
	for _, d := range data {
		if got := d.valueType.goType(d.isList); got != d.expected {
			t.Errorf("Expected value type %d with isList %v to be read as "+
				"%v but got %v.\n", d.valueType, d.isList, d.expected, got)
		}
	}
}
