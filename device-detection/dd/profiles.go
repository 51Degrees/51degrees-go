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

// #include <stdlib.h>
// #include <string.h>
// #include "device-detection-cxx.h"
import "C"
import "unsafe"

// InitManagerFromFileWithIndexedProperties initializes a resource manager as
// InitManagerFromFile does, and indexes the profiles holding each value of the
// Properties named, such as TAC and NativeModel, so ProfileIdsForPropertyValue
// finds them without reading every profile. Names are matched exactly. A
// reload indexes the same names again. This matches the C API
// fiftyoneDegreesHashInitManagerFromFileWithIndexedProperties.
func InitManagerFromFileWithIndexedProperties(
	manager *ResourceManager,
	config ConfigHash,
	properties string,
	filePath string,
	indexed []string) error {

	e := NewException()
	cPath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cPath))
	propsRequired := NewPropertiesRequired(properties)
	defer propsRequired.Free()

	// The C library copies the names.
	names := make([]*C.char, len(indexed))
	for i, name := range indexed {
		names[i] = C.CString(name)
	}
	defer func() {
		for _, name := range names {
			C.free(unsafe.Pointer(name))
		}
	}()
	var cNames **C.char
	if len(names) > 0 {
		cNames = &names[0]
	}

	s := C.HashInitManagerFromFileWithIndexedProperties(
		manager.CPtr,
		config.CPtr,
		propsRequired.CPtr,
		cPath,
		cNames,
		C.int(len(names)),
		e.CPtr,
	)
	if !e.IsOkay() {
		return messageError(C.ExceptionGetMessage(e.CPtr))
	}
	if s != C.SUCCESS {
		return messageError(C.StatusGetMessage(s, cPath))
	}

	// As InitManagerFromFile does.
	manager.indexes = newPropertyIndexes(manager)
	manager.setHeaders = constructResponseHeaders(manager)
	initHttpHeaderKeys(manager)
	return nil
}

// ProfileIdsForPropertyValue gives the ids of the profiles that hold value for
// property, both matched exactly as the data file holds them, in ascending id
// with each once. A Property the manager was made to index is found through
// the native code's index, and any other by reading every profile, with the
// same ids. This matches the C API
// fiftyoneDegreesHashGetProfileIdsForPropertyAndValue.
func (manager *ResourceManager) ProfileIdsForPropertyValue(
	property, value string) ([]uint32, error) {
	cProperty := C.CString(property)
	defer C.free(unsafe.Pointer(cProperty))
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cValue))

	// The first call counts the ids. A refresh between the calls can change
	// the count, so the ids are read again until they fit.
	var ids []uint32
	for {
		e := NewException()
		var buffer *C.uint32_t
		if len(ids) > 0 {
			buffer = (*C.uint32_t)(unsafe.Pointer(&ids[0]))
		}
		count := uint32(C.HashGetProfileIdsForPropertyAndValue(
			manager.CPtr,
			cProperty,
			cValue,
			buffer,
			C.uint32_t(len(ids)),
			e.CPtr,
		))
		if !e.IsOkay() {
			return nil, messageError(C.ExceptionGetMessage(e.CPtr))
		}
		if buffer != nil && count <= uint32(len(ids)) {
			return ids[:count], nil
		}
		if count == 0 {
			return []uint32{}, nil
		}
		ids = make([]uint32, count)
	}
}
