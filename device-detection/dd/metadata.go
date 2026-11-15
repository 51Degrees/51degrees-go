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
#include <stdlib.h>
#include <string.h>
#include "device-detection-cxx.h"

// freeText frees a text the C library allocated because it did not fit the
// buffer given, with the library's own Free, which Go cannot call directly.
static void freeText(char *text) {
	fiftyoneDegreesFree(text);
}
*/
import "C"
import (
	"reflect"
	"unsafe"
)

// metadataTextBytes is the buffer a text of the metadata is read into. A
// longer text is read into memory the C library allocates.
const metadataTextBytes = 1024

// PropertyMetadata describes a Property the data file defines.
type PropertyMetadata struct {
	// Name is the name a value of the Property is read by.
	Name string
	// Category groups the Property with others.
	Category string
	// Description says what the Property gives.
	Description string
	// Url links to more about the Property.
	Url string
	// Type is the Go type a value of the Property is read as.
	Type reflect.Type
	// DataTiers are the data files that hold the Property, such as Lite.
	DataTiers []string
	// Available says the manager was made with the Property, so results
	// hold its values.
	Available bool
	// IsList says the Property can have more than one value.
	IsList bool
	// IsObsolete says the Property will leave future data files.
	IsObsolete bool
	// ComponentId is the id of the component the Property belongs to.
	ComponentId uint8
	// DisplayOrder is the suggested place of the Property where Properties
	// are listed.
	DisplayOrder uint8
	// IsMandatory says a profile is valid only when it holds a value of the
	// Property other than the default.
	IsMandatory bool
	// Show says the Property is shown where Properties are listed.
	Show bool
	// ShowValues says the Property's values are shown where values are
	// listed.
	ShowValues bool
}

// Properties gives the metadata of every Property the data file defines,
// those the manager was not made with included, through the C library's
// Hash metadata functions.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/properties.md?plain=1#L65-L110
func (manager *ResourceManager) Properties() ([]PropertyMetadata, error) {
	cDataSet := (*C.DataSetHash)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))

	count := uint32(C.fiftyoneDegreesHashGetPropertyCount(cDataSet))
	all := make([]PropertyMetadata, 0, count)
	for i := uint32(0); i < count; i++ {
		e := NewException()
		var valueType, componentId, displayOrder C.uint8_t
		var isMandatory, isList, isObsolete, show, showValues, isAvailable C.bool
		var mapCount C.uint32_t
		if !C.fiftyoneDegreesHashGetProperty(cDataSet, C.uint32_t(i),
			&valueType, &componentId, &displayOrder, &isMandatory, &isList,
			&isObsolete, &show, &showValues, &mapCount, &isAvailable, nil,
			e.CPtr) {
			if !e.IsOkay() {
				return nil, messageError(C.ExceptionGetMessage(e.CPtr))
			}
			// The properties collection gave nothing at the index.
			continue
		}
		m := PropertyMetadata{
			Type:         propertyValueType(valueType).goType(bool(isList)),
			Available:    bool(isAvailable),
			IsList:       bool(isList),
			IsObsolete:   bool(isObsolete),
			ComponentId:  uint8(componentId),
			DisplayOrder: uint8(displayOrder),
			IsMandatory:  bool(isMandatory),
			Show:         bool(show),
			ShowValues:   bool(showValues),
		}
		texts := []struct {
			text C.fiftyoneDegreesPropertyText
			into *string
		}{
			{C.fiftyoneDegreesPropertyText(C.FIFTYONE_DEGREES_PROPERTY_TEXT_NAME),
				&m.Name},
			{C.fiftyoneDegreesPropertyText(C.FIFTYONE_DEGREES_PROPERTY_TEXT_CATEGORY),
				&m.Category},
			{C.fiftyoneDegreesPropertyText(C.FIFTYONE_DEGREES_PROPERTY_TEXT_DESCRIPTION),
				&m.Description},
			{C.fiftyoneDegreesPropertyText(C.FIFTYONE_DEGREES_PROPERTY_TEXT_URL),
				&m.Url},
		}
		for _, t := range texts {
			text, err := propertyText(cDataSet, i, t.text)
			if err != nil {
				return nil, err
			}
			*t.into = text
		}
		for j := uint32(0); j < uint32(mapCount); j++ {
			tier, ok, err := propertyMapText(cDataSet, i, j)
			if err != nil {
				return nil, err
			}
			if ok {
				m.DataTiers = append(m.DataTiers, tier)
			}
		}
		all = append(all, m)
	}
	return all, nil
}

// propertyText gives a text of the property at the index. A text the
// property does not have is empty.
func propertyText(
	dataSet *C.DataSetHash,
	index uint32,
	text C.fiftyoneDegreesPropertyText) (string, error) {
	var buffer [metadataTextBytes]C.char
	e := NewException()
	got := C.fiftyoneDegreesHashGetPropertyText(dataSet, C.uint32_t(index),
		text, &buffer[0], C.size_t(len(buffer)), e.CPtr)
	if !e.IsOkay() {
		return "", messageError(C.ExceptionGetMessage(e.CPtr))
	}
	if got == nil {
		return "", nil
	}
	value := C.GoString(got)
	if got != &buffer[0] {
		C.freeText(got)
	}
	return value, nil
}

// propertyMapText gives the name of one of the maps, being the data tiers,
// the property at the index is in, and false when the data file gives no
// name for it.
func propertyMapText(
	dataSet *C.DataSetHash,
	index, mapIndex uint32) (string, bool, error) {
	var buffer [metadataTextBytes]C.char
	e := NewException()
	got := C.fiftyoneDegreesHashGetPropertyMapText(dataSet, C.uint32_t(index),
		C.uint32_t(mapIndex), &buffer[0], C.size_t(len(buffer)), e.CPtr)
	if !e.IsOkay() {
		return "", false, messageError(C.ExceptionGetMessage(e.CPtr))
	}
	if got == nil {
		return "", false, nil
	}
	value := C.GoString(got)
	if got != &buffer[0] {
		C.freeText(got)
	}
	return value, true, nil
}
