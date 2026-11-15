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

package ipi_interop

/*
#include "ip-intelligence-cxx.h"

// freeMetadataText frees a text the C library allocated because it did not
// fit the buffer given, with the library's own Free, which Go cannot call
// directly.
static void freeMetadataText(char *text) {
	fiftyoneDegreesFree(text);
}
*/
import "C"
import "unsafe"

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
	// Type is the type the data file stores the Property's values as.
	Type PropertyValueType
	// DataTiers are the data files that hold values of the Property, such as
	// Enterprise.
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

// ComponentMetadata describes a component of the data file, which groups
// related Properties, such as those of the network or the location.
type ComponentMetadata struct {
	// Id stays the same when the data file is updated.
	Id uint8
	// Name is the component's name, such as Location.
	Name string
	// Properties are the names of the component's Properties.
	Properties []string
}

// Properties gives the metadata of every Property the data file defines,
// those the manager was not made with included, through the C library's IP
// Intelligence metadata functions.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/required-examples.md?plain=1#L167-L173
func (manager *ResourceManager) Properties() ([]PropertyMetadata, error) {
	cDataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))
	return propertyMetadata(cDataSet)
}

// propertyMetadata gives the metadata of every Property of the data set.
func propertyMetadata(cDataSet *C.DataSetIpi) ([]PropertyMetadata, error) {
	// One exception serves every call, cleared before each.
	e := NewException()
	defer e.Free()

	count := uint32(C.fiftyoneDegreesIpiGetPropertyCount(cDataSet))
	all := make([]PropertyMetadata, 0, count)
	for i := uint32(0); i < count; i++ {
		e.Clear()
		var valueType, componentId C.uint8_t
		var isList, isObsolete, isAvailable C.bool
		var mapCount C.uint32_t
		if !C.fiftyoneDegreesIpiGetProperty(cDataSet, C.uint32_t(i),
			&valueType, &componentId, nil, nil, &isList, &isObsolete, nil,
			nil, &mapCount, &isAvailable, nil, e.CPtr) {
			if !e.IsOkay() {
				return nil, messageError(C.ExceptionGetMessage(e.CPtr))
			}
			// The properties collection gave nothing at the index.
			continue
		}
		m := PropertyMetadata{
			Type:        PropertyValueType(valueType),
			Available:   bool(isAvailable),
			IsList:      bool(isList),
			IsObsolete:  bool(isObsolete),
			ComponentId: uint8(componentId),
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
			var buffer [metadataTextBytes]C.char
			e.Clear()
			got := C.fiftyoneDegreesIpiGetPropertyText(cDataSet,
				C.uint32_t(i), t.text, &buffer[0], C.size_t(len(buffer)),
				e.CPtr)
			if !e.IsOkay() {
				return nil, messageError(C.ExceptionGetMessage(e.CPtr))
			}
			*t.into = takeText(got, &buffer[0])
		}
		for j := uint32(0); j < uint32(mapCount); j++ {
			var buffer [metadataTextBytes]C.char
			e.Clear()
			got := C.fiftyoneDegreesIpiGetPropertyMapText(cDataSet,
				C.uint32_t(i), C.uint32_t(j), &buffer[0],
				C.size_t(len(buffer)), e.CPtr)
			if !e.IsOkay() {
				return nil, messageError(C.ExceptionGetMessage(e.CPtr))
			}
			// The data file gives no name for a map that is left out.
			if got != nil {
				m.DataTiers = append(m.DataTiers, takeText(got, &buffer[0]))
			}
		}
		all = append(all, m)
	}
	return all, nil
}

// Components gives every component the data file defines, with the names of
// its Properties, through the C library's IP Intelligence metadata
// functions.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/required-examples.md?plain=1#L167-L173
func (manager *ResourceManager) Components() ([]ComponentMetadata, error) {
	cDataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))

	properties, err := propertyMetadata(cDataSet)
	if err != nil {
		return nil, err
	}

	e := NewException()
	defer e.Free()

	count := uint32(C.fiftyoneDegreesIpiGetComponentCount(cDataSet))
	all := make([]ComponentMetadata, 0, count)
	for i := uint32(0); i < count; i++ {
		var id C.uint8_t
		if !C.fiftyoneDegreesIpiGetComponent(cDataSet, C.uint32_t(i), &id,
			nil) {
			continue
		}
		var buffer [metadataTextBytes]C.char
		e.Clear()
		got := C.fiftyoneDegreesIpiGetComponentName(cDataSet, C.uint32_t(i),
			&buffer[0], C.size_t(len(buffer)), e.CPtr)
		if !e.IsOkay() {
			return nil, messageError(C.ExceptionGetMessage(e.CPtr))
		}
		c := ComponentMetadata{Id: uint8(id), Name: takeText(got, &buffer[0])}
		for _, p := range properties {
			if p.ComponentId == c.Id {
				c.Properties = append(c.Properties, p.Name)
			}
		}
		all = append(all, c)
	}
	return all, nil
}

// takeText gives the text the C library wrote, empty when it wrote none,
// and frees it when it was too long for the buffer and the library
// allocated it.
func takeText(got, buffer *C.char) string {
	if got == nil {
		return ""
	}
	value := C.GoString(got)
	if got != buffer {
		C.freeMetadataText(got)
	}
	return value
}
