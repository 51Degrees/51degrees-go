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

/*
#include <string.h>
#include "device-detection-cxx.h"

// Reads the value type of the property at the index in the data set, and
// whether the property holds a list of values. Both are given as bytes, so
// the caller needs no C boolean type. Returns false when the property cannot
// be read, in which case the exception holds the reason.
static bool getPropertyType(
	fiftyoneDegreesDataSetHash *dataSet,
	uint32_t propertyIndex,
	uint8_t *valueType,
	uint8_t *isList,
	fiftyoneDegreesException *exception) {
	bool list = false;
	if (fiftyoneDegreesHashGetProperty(
			dataSet,
			propertyIndex,
			valueType,
			NULL,
			NULL,
			NULL,
			&list,
			NULL,
			NULL,
			NULL,
			NULL,
			NULL,
			NULL,
			exception) == false) {
		return false;
	}
	*isList = list ? 1 : 0;
	return true;
}
*/
import "C"
import "reflect"

// propertyValueType is the type of value a property holds, as the data file
// records it. The numbers are the C library's
// fiftyoneDegreesPropertyValueType, and a Hash data file holds only these
// five.
type propertyValueType uint8

const (
	stringValueType     propertyValueType = 0
	integerValueType    propertyValueType = 1
	doubleValueType     propertyValueType = 2
	booleanValueType    propertyValueType = 3
	javaScriptValueType propertyValueType = 4
)

// goType gives the Go type a value of this property type is read as, where
// isList says whether the property holds a list of values rather than one.
func (t propertyValueType) goType(isList bool) reflect.Type {
	switch t {
	case stringValueType:
		if isList {
			return reflect.TypeOf([]string(nil))
		}
		return reflect.TypeOf("")
	case integerValueType:
		return reflect.TypeOf(int(0))
	case doubleValueType:
		return reflect.TypeOf(float64(0))
	case booleanValueType:
		return reflect.TypeOf(false)
	case javaScriptValueType:
		// A script, which the data file holds as text.
		return reflect.TypeOf("")
	}
	// A type no Hash data file holds, which the C library also names a
	// string.
	return reflect.TypeOf("")
}

// PropertyType returns the Go type a value of the property at the required
// property index is read as, being its place among the names the manager was
// created with, or with no names, among the names of the first data file
// loaded. An index outside those names, one the data set has no property for,
// or a property the data set cannot be read for, gives nil.
func (results *ResultsHash) PropertyType(
	requiredPropertyIndex int) reflect.Type {
	dataSet := (*C.DataSetHash)(results.CPtr.b.b.dataSet)
	propertyIndex := C.PropertiesGetPropertyIndexFromRequiredIndex(
		dataSet.b.b.available, C.int(requiredPropertyIndex))
	if propertyIndex < 0 {
		return nil
	}
	var valueType, isList C.uint8_t
	exp := NewException()
	read := C.getPropertyType(
		dataSet,
		C.uint32_t(propertyIndex),
		&valueType,
		&isList,
		exp.CPtr)
	if !bool(read) || !exp.IsOkay() {
		return nil
	}
	return propertyValueType(valueType).goType(isList != 0)
}
