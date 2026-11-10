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

package dd

// #include <string.h>
// #include "device-detection-cxx.h"
// #include "device-detection-cxx.h"
import "C"
import (
	"runtime"
	"strings"
	"unsafe"
)

// PropertiesRequired wraps around a pointer to a value of C PropertiesRequired
// structure.
type PropertiesRequired struct {
	CPtr *C.PropertiesRequired
}

// propertiesFinalizer check if C resource has been explicitly
// freed by Free method. Panic if it is not.
func propertiesFinalizer(props *PropertiesRequired) {
	if props.CPtr != nil {
		panic("ERROR: PropertiesRequired should be freed explicitly by " +
			"its Free method.")
	}
}

// NewPropertiesRequired creates a new PropertiesRequired object.
func NewPropertiesRequired(
	properties string) *PropertiesRequired {
	// Create C properties required
	cProps := C.PropertiesDefault
	cProps.string = C.CString(properties)
	props := &PropertiesRequired{&cProps}
	runtime.SetFinalizer(props, propertiesFinalizer)
	return props
}

// propertyIndexes maps each property name a manager was created with, as
// given and in lower case, to its required property index, which is its place
// among the names. A reload keeps the names, so it keeps every index.
type propertyIndexes map[string]int

// newPropertyIndexes reads the names the manager's data set was created with.
func newPropertyIndexes(manager *ResourceManager) propertyIndexes {
	names := requiredPropertyNames(manager)
	indexes := make(propertyIndexes, 2*len(names))
	for i, name := range names {
		indexes[name] = i
		indexes[strings.ToLower(name)] = i
	}
	return indexes
}

// index returns the required property index of the name, compared as C
// compares names, without regard to case, or -1 for a name not given.
func (indexes propertyIndexes) index(name string) int {
	if i, ok := indexes[name]; ok {
		return i
	}
	if i, ok := indexes[strings.ToLower(name)]; ok {
		return i
	}
	return -1
}

// requiredPropertyNames returns the names the manager's data set was created
// with, in required property index order, including any the data set has no
// property for.
func requiredPropertyNames(manager *ResourceManager) []string {
	dataSet := C.DataSetGet(manager.CPtr)
	defer C.DataSetRelease(dataSet)
	available := dataSet.available
	count := int(C.PropertiesGetRequiredPropertyCount(available))
	names := make([]string, count)
	for i := 0; i < count; i++ {
		if name := C.PropertiesGetNameFromRequiredIndex(
			available, C.int(i)); name != nil {
			names[i] = C.GoString(&name.value)
		}
	}
	return names
}

// Free free the C resource and set CPtr to nil
func (properties *PropertiesRequired) Free() {
	C.free(unsafe.Pointer(properties.CPtr.string))
	properties.CPtr = nil
}

// Properties returns the required properties string
func (properties *PropertiesRequired) Properties() string {
	return C.GoString(properties.CPtr.string)
}
