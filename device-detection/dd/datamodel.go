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

// freeMetadataText frees a text the C library allocated because it did not
// fit the buffer given, with the library's own Free, which Go cannot call
// directly.
static void freeMetadataText(char *text) {
	fiftyoneDegreesFree(text);
}

// profileValueIndexes copies up to length of the value indexes of the
// profile with the id into indexes, and gives how many the profile holds,
// or -1 when the data file holds no profile with the id or it cannot be
// read.
static long profileValueIndexes(
	fiftyoneDegreesDataSetHash *dataSet,
	uint32_t profileId,
	uint32_t *indexes,
	uint32_t length,
	fiftyoneDegreesException *exception) {
	fiftyoneDegreesCollectionItem item;
	fiftyoneDegreesDataReset(&item.data);
	const fiftyoneDegreesProfile *profile =
		fiftyoneDegreesProfileGetByProfileId(
			dataSet->profileOffsets,
			dataSet->profiles,
			profileId,
			&item,
			exception);
	if (profile == NULL) {
		return -1;
	}
	const uint32_t count = profile->valueCount;
	if (indexes != NULL) {
		memcpy(
			indexes,
			(const uint32_t*)(profile + 1),
			(count < length ? count : length) * sizeof(uint32_t));
	}
	FIFTYONE_DEGREES_COLLECTION_RELEASE(dataSet->profiles, &item);
	return (long)count;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// ComponentMetadata describes a component of the data file. A component
// groups related Properties, such as those of the hardware, and each of its
// profiles holds values of each of them.
type ComponentMetadata struct {
	// Id stays the same when the data file is updated.
	Id uint8
	// Name is the component's name, such as HardwarePlatform.
	Name string
	// DefaultProfileId is the profile whose values the component's
	// Properties have when no profile matches the evidence.
	DefaultProfileId uint32
	// Properties are the names of the component's Properties.
	Properties []string
}

// ProfileMetadata describes a profile of the data file, which holds values
// of the Properties of one component.
type ProfileMetadata struct {
	// Id stays the same when the data file is updated.
	Id uint32
	// ComponentId is the id of the component the profile holds values for.
	ComponentId uint8
}

// ValueMetadata describes a value a Property of the data file can have.
type ValueMetadata struct {
	// Property is the name of the Property the value belongs to.
	Property string
	// Name is the value as text, unique among the Property's values.
	Name string
	// Description says what the value means.
	Description string
	// Url links to more about the value.
	Url string
	// IsDefault says the value is its Property's default value.
	IsDefault bool
}

// Components gives every component the data file defines, with the names of
// its Properties and its default profile, through the C library's Hash
// metadata functions.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/data-model-specification/README.md?plain=1#L16-L31
func (manager *ResourceManager) Components() ([]ComponentMetadata, error) {
	cDataSet := (*C.DataSetHash)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))

	names, componentIds, err := propertyNamesAndComponents(cDataSet)
	if err != nil {
		return nil, err
	}
	count := uint32(C.fiftyoneDegreesHashGetComponentCount(cDataSet))
	all := make([]ComponentMetadata, 0, count)
	for i := uint32(0); i < count; i++ {
		e := NewException()
		var id C.uint8_t
		var defaultProfileId C.uint32_t
		if !C.fiftyoneDegreesHashGetComponent(cDataSet, C.uint32_t(i), &id,
			&defaultProfileId, e.CPtr) {
			if !e.IsOkay() {
				return nil, messageError(C.ExceptionGetMessage(e.CPtr))
			}
			// The data file gives no component, or no default profile,
			// at the index.
			continue
		}
		name, err := componentName(cDataSet, i)
		if err != nil {
			return nil, err
		}
		c := ComponentMetadata{
			Id:               uint8(id),
			Name:             name,
			DefaultProfileId: uint32(defaultProfileId),
		}
		for j, componentId := range componentIds {
			if names[j] != "" && componentId == c.Id {
				c.Properties = append(c.Properties, names[j])
			}
		}
		all = append(all, c)
	}
	return all, nil
}

// Profiles gives every profile the data file holds, in order of id, with the
// component each holds values for.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/data-model-specification/README.md?plain=1#L60-L70
func (manager *ResourceManager) Profiles() ([]ProfileMetadata, error) {
	cDataSet := (*C.DataSetHash)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))

	count := uint32(C.fiftyoneDegreesHashGetProfileCount(cDataSet))
	all := make([]ProfileMetadata, 0, count)
	for i := uint32(0); i < count; i++ {
		e := NewException()
		var id C.uint32_t
		var componentId C.uint8_t
		if !C.fiftyoneDegreesHashGetProfile(cDataSet, C.uint32_t(i), &id,
			&componentId, e.CPtr) {
			if !e.IsOkay() {
				return nil, messageError(C.ExceptionGetMessage(e.CPtr))
			}
			// The profiles collection gave nothing at the index.
			continue
		}
		all = append(all, ProfileMetadata{
			Id:          uint32(id),
			ComponentId: uint8(componentId),
		})
	}
	return all, nil
}

// Values gives every value of every Property the data file defines, a
// Property's values together in the order the data file holds them, with
// the Property's default value marked.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/data-model-specification/README.md?plain=1#L72-L82
func (manager *ResourceManager) Values() ([]ValueMetadata, error) {
	cDataSet := (*C.DataSetHash)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))

	names, _, err := propertyNamesAndComponents(cDataSet)
	if err != nil {
		return nil, err
	}
	var all []ValueMetadata
	for i, name := range names {
		if name == "" {
			continue
		}
		e := NewException()
		var first, count C.uint32_t
		var defaultIndex C.long
		if !C.fiftyoneDegreesHashGetPropertyValueIndexes(cDataSet,
			C.uint32_t(i), &first, &count, &defaultIndex, e.CPtr) {
			if !e.IsOkay() {
				return nil, messageError(C.ExceptionGetMessage(e.CPtr))
			}
			// The properties collection gave nothing at the index.
			continue
		}
		for v := uint32(first); v < uint32(first)+uint32(count); v++ {
			m, err := valueMetadata(cDataSet, v)
			if err != nil {
				return nil, err
			}
			m.Property = name
			m.IsDefault = int64(defaultIndex) == int64(v)
			all = append(all, m)
		}
	}
	return all, nil
}

// ProfileValues gives the values the profile with the id holds, in the order
// the data file holds them, and an error when the data file holds no such
// profile.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/data-model-specification/README.md?plain=1#L60-L70
func (manager *ResourceManager) ProfileValues(
	profileId uint32) ([]ValueMetadata, error) {
	cDataSet := (*C.DataSetHash)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))

	// The count and then the indexes are read from the data set held above,
	// so a refresh between the two reads cannot change the profile.
	e := NewException()
	count := C.profileValueIndexes(cDataSet, C.uint32_t(profileId), nil, 0,
		e.CPtr)
	if !e.IsOkay() {
		return nil, messageError(C.ExceptionGetMessage(e.CPtr))
	}
	if count < 0 {
		return nil, fmt.Errorf("the data file holds no profile %d", profileId)
	}
	if count == 0 {
		return nil, nil
	}
	indexes := make([]C.uint32_t, count)
	e = NewException()
	C.profileValueIndexes(cDataSet, C.uint32_t(profileId), &indexes[0],
		C.uint32_t(len(indexes)), e.CPtr)
	if !e.IsOkay() {
		return nil, messageError(C.ExceptionGetMessage(e.CPtr))
	}

	names, _, err := propertyNamesAndComponents(cDataSet)
	if err != nil {
		return nil, err
	}
	defaults := make(map[uint32]int64)
	all := make([]ValueMetadata, 0, len(indexes))
	for _, index := range indexes {
		e := NewException()
		var propertyIndex C.uint32_t
		if !C.fiftyoneDegreesHashGetValue(cDataSet, index, &propertyIndex, nil,
			nil, e.CPtr) {
			if !e.IsOkay() {
				return nil, messageError(C.ExceptionGetMessage(e.CPtr))
			}
			// The values collection gave nothing at the index.
			continue
		}
		m, err := valueMetadata(cDataSet, uint32(index))
		if err != nil {
			return nil, err
		}
		property := uint32(propertyIndex)
		if int(property) < len(names) {
			m.Property = names[property]
		}
		defaultIndex, ok := defaults[property]
		if !ok {
			var d C.long
			e := NewException()
			if !C.fiftyoneDegreesHashGetPropertyValueIndexes(cDataSet,
				propertyIndex, nil, nil, &d, e.CPtr) {
				if !e.IsOkay() {
					return nil, messageError(C.ExceptionGetMessage(e.CPtr))
				}
				d = -1
			}
			defaultIndex = int64(d)
			defaults[property] = defaultIndex
		}
		m.IsDefault = defaultIndex == int64(index)
		all = append(all, m)
	}
	return all, nil
}

// propertyNamesAndComponents gives the name and component id of each
// Property by its index. A Property the properties collection gives nothing
// for has no name.
func propertyNamesAndComponents(
	dataSet *C.DataSetHash) ([]string, []uint8, error) {
	count := uint32(C.fiftyoneDegreesHashGetPropertyCount(dataSet))
	names := make([]string, count)
	componentIds := make([]uint8, count)
	for i := uint32(0); i < count; i++ {
		e := NewException()
		var componentId C.uint8_t
		if !C.fiftyoneDegreesHashGetProperty(dataSet, C.uint32_t(i), nil,
			&componentId, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			e.CPtr) {
			if !e.IsOkay() {
				return nil, nil, messageError(C.ExceptionGetMessage(e.CPtr))
			}
			continue
		}
		name, err := propertyText(dataSet, i, C.fiftyoneDegreesPropertyText(
			C.FIFTYONE_DEGREES_PROPERTY_TEXT_NAME))
		if err != nil {
			return nil, nil, err
		}
		names[i] = name
		componentIds[i] = uint8(componentId)
	}
	return names, componentIds, nil
}

// componentName gives the name of the component at the index.
func componentName(dataSet *C.DataSetHash, index uint32) (string, error) {
	var buffer [metadataTextBytes]C.char
	e := NewException()
	got := C.fiftyoneDegreesHashGetComponentName(dataSet, C.uint32_t(index),
		&buffer[0], C.size_t(len(buffer)), e.CPtr)
	if !e.IsOkay() {
		return "", messageError(C.ExceptionGetMessage(e.CPtr))
	}
	return takeText(got, &buffer[0]), nil
}

// valueMetadata gives the name, description and URL of the value at the
// index in the values collection.
func valueMetadata(dataSet *C.DataSetHash, index uint32) (ValueMetadata,
	error) {
	var m ValueMetadata
	texts := []struct {
		text C.fiftyoneDegreesValueText
		into *string
	}{
		{C.fiftyoneDegreesValueText(C.FIFTYONE_DEGREES_VALUE_TEXT_NAME),
			&m.Name},
		{C.fiftyoneDegreesValueText(C.FIFTYONE_DEGREES_VALUE_TEXT_DESCRIPTION),
			&m.Description},
		{C.fiftyoneDegreesValueText(C.FIFTYONE_DEGREES_VALUE_TEXT_URL),
			&m.Url},
	}
	for _, t := range texts {
		var buffer [metadataTextBytes]C.char
		e := NewException()
		got := C.fiftyoneDegreesHashGetValueText(dataSet, C.uint32_t(index),
			t.text, &buffer[0], C.size_t(len(buffer)), e.CPtr)
		if !e.IsOkay() {
			return m, messageError(C.ExceptionGetMessage(e.CPtr))
		}
		*t.into = takeText(got, &buffer[0])
	}
	return m, nil
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
