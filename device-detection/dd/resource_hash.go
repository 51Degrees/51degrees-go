/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, Davidson House,
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

// #include <string.h>
// #include "device-detection-cxx.h"
// #include "device-detection-cxx.h"
import "C"
import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unsafe"
)

// InitFromFile initializes a resource manager based on a given data file path.
// The input properties is a comma separated string list. This matches the C API
// fiftyoneDegreesHashInitManagerFromFile.
//
// The manager keeps the required property index of each name, so a read by
// name costs a map lookup rather than a search of the data set. The index is
// the name's place among the names given, or with no names, among the names
// of the first data file loaded, and a reload does not change it.
func InitManagerFromFile(
	manager *ResourceManager,
	config ConfigHash,
	properties string,
	filePath string) error {

	e := NewException()
	cPath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cPath))
	propsRequired := NewPropertiesRequired(properties)
	defer propsRequired.Free()
	s := C.HashInitManagerFromFile(
		manager.CPtr,
		config.CPtr,
		propsRequired.CPtr,
		cPath,
		e.CPtr,
	)

	// Check exception
	if !e.IsOkay() {
		return messageError(C.ExceptionGetMessage(e.CPtr))
	}

	// Check status code
	if s != C.SUCCESS {
		return messageError(C.StatusGetMessage(s, cPath))
	}

	// Keep the index of each name, so a read by name does not search the data
	// set. The C library keeps the names across a reload, so the table holds.
	manager.indexes = newPropertyIndexes(manager)

	// Construct the Set Headers list
	manager.setHeaders = constructResponseHeaders(manager)

	// Init Http header keys
	initHttpHeaderKeys(manager)

	return nil
}

// InitManagerFromMemory initializes a resource manager from a data file held
// in memory, the size bytes at memory. The input properties is a comma
// separated string list. This matches the C API
// fiftyoneDegreesHashInitManagerFromMemory.
//
// The data is copied, so the caller's memory can be reused once this returns,
// and each data set frees its copy when it is no longer in use. Reload the
// manager from new data in memory with ReloadFromBytes.
func InitManagerFromMemory(
	manager *ResourceManager,
	config ConfigHash,
	properties string,
	memory unsafe.Pointer,
	size uint64) error {
	if memory == nil || size == 0 {
		return errors.New("there is no data in memory to load")
	}
	data := C.malloc(C.size_t(size))
	C.memcpy(data, memory, C.size_t(size))

	// The data set frees the copy, so the configuration says to.
	cConfig := (*C.ConfigHash)(C.malloc(C.sizeof_ConfigHash))
	defer C.free(unsafe.Pointer(cConfig))
	*cConfig = *config.CPtr
	cConfig.b.b.freeData = C.IntToBool(1)

	e := NewException()
	propsRequired := NewPropertiesRequired(properties)
	defer propsRequired.Free()
	s := C.HashInitManagerFromMemory(
		manager.CPtr,
		cConfig,
		propsRequired.CPtr,
		data,
		C.long(size),
		e.CPtr,
	)
	// A data set that was not created leaves the copy with the caller.
	if !e.IsOkay() {
		C.free(data)
		return messageError(C.ExceptionGetMessage(e.CPtr))
	}
	if s != C.SUCCESS {
		C.free(data)
		return messageError(C.StatusGetMessage(s, nil))
	}

	manager.fromMemory = true
	manager.indexes = newPropertyIndexes(manager)
	manager.setHeaders = constructResponseHeaders(manager)
	initHttpHeaderKeys(manager)
	return nil
}

// ReloadFromFile reloads the data set being used by the resource manager using
// the specified data file location. This is corresponding to the C API
// fiftyoneDegreesHashReloadManagerFromFile.
func (manager *ResourceManager) ReloadFromFile(fileName string) error {
	exp := NewException()
	cFilenamePath := C.CString(fileName)
	defer C.free(unsafe.Pointer(cFilenamePath))
	s := C.HashReloadManagerFromFile(
		manager.CPtr,
		cFilenamePath,
		exp.CPtr,
	)
	return reloadError(s, exp, cFilenamePath)
}

// ReloadFromOriginalFile reloads the data set being used by the resource
// manager using the data file location which was used when the manager was
// created. This is corresponding to the C API
// fiftyoneDegreesHashReloadManagerFromOriginalFile
func (manager *ResourceManager) ReloadFromOriginalFile() error {
	exp := NewException()
	s := C.HashReloadManagerFromOriginalFile(
		manager.CPtr,
		exp.CPtr,
	)
	if s == C.SUCCESS && exp.IsOkay() {
		return nil
	}
	cDataSet := C.DataSetGet(manager.CPtr)
	defer C.DataSetRelease(cDataSet)
	return reloadError(s, exp, &cDataSet.masterFileName[0])
}

// reloadError returns the failure of a reload, or nil if it worked. A reload
// can report a failure in the exception alone or in the status alone, so both
// are checked.
func reloadError(s C.StatusCode, exp *Exception, fileName *C.char) error {
	if !exp.IsOkay() {
		return messageError(C.ExceptionGetMessage(exp.CPtr))
	}
	if s != C.SUCCESS {
		return messageError(C.StatusGetMessage(s, fileName))
	}
	return nil
}

// ReloadFromMemory takes no data to reload from, so it reloads nothing and
// returns an error.
//
// Deprecated: Use ReloadFromBytes, which takes the new data.
func (manager *ResourceManager) ReloadFromMemory() error {
	return errors.New("ReloadFromMemory has no data to reload from, so " +
		"use ReloadFromBytes")
}

// ReloadFromBytes reloads the data set from a data file held in data, and
// works the accepted Evidence keys out again from the new data. The data is
// copied, so it can be reused once this returns. Only a manager made from
// memory with InitManagerFromMemory reloads from memory, because only its
// data sets free their copy when they are no longer in use. This matches the
// C API fiftyoneDegreesHashReloadManagerFromMemory.
func (manager *ResourceManager) ReloadFromBytes(data []byte) error {
	if !manager.fromMemory {
		return errors.New("a manager made from a data file reloads from a " +
			"data file, with ReloadFromFile")
	}
	if len(data) == 0 {
		return errors.New("there is no data in memory to reload from")
	}
	copied := C.CBytes(data)
	e := NewException()
	s := C.HashReloadManagerFromMemory(
		manager.CPtr,
		copied,
		C.long(len(data)),
		e.CPtr,
	)
	// The copy now belongs to the data set the C library made for it, which
	// frees it when it is freed, a reload that fails included. The only
	// failure before that data set exists is the C library failing to
	// allocate it, which leaves the copy unfreed rather than freed twice.
	if err := reloadError(s, e, nil); err != nil {
		return err
	}
	initHttpHeaderKeys(manager)
	return nil
}

// extractSetHeaderName extracts the header name to be set from a 'SetHeader'
// property. Panic if the property is not in correct format
func extractSetHeaderName(property string) (headerName string, err error) {
	// Get the substring that match the first part of a SetHeader property
	// which is in the format: SetHeader[A-Z][a-z]*[A-Z].*
	re := regexp.MustCompile(`^SetHeader[A-Z][a-z]*`)
	matchStr := re.FindString(property)
	if matchStr == "" {
		// No match found
		return "", fmt.Errorf(ErrNoMatch)
	}

	// Get the remaining of the property name which is the header to set
	matchStr = property[len(matchStr):]
	re = regexp.MustCompile(`^[A-Z].*`)
	if !re.Match([]byte(matchStr)) {
		return "", fmt.Errorf(ErrSHPropertyIncorrectFormat)
	}
	return matchStr, nil
}

// constructResponseHeaders goes through all available properties, extracted the
// response header name and map it to the list of associated properties, then
// return the map. Returns 'nil' if the map is empty.
func constructResponseHeaders(manager *ResourceManager) map[string]([]string) {
	// Get dataset from resource
	cDataSet := (*C.DataSetHash)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	// Release the dataset
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))

	// Loop through available properties, which with properties named are the
	// names given, as PropertiesGetNameFromRequiredIndex takes them
	cAvailable := cDataSet.b.b.available
	availableCount := int(C.PropertiesGetRequiredPropertyCount(cAvailable))
	responseHeaders := make(map[string]([]string))
	// Loop through the C available properties list and add it to the
	// new Go available array.
	for i := 0; i < availableCount; i++ {
		cName := C.PropertiesGetNameFromRequiredIndex(cAvailable, C.int(i))
		name := C.GoString(&cName.value)
		// Obtain the Header name and add the property to the corresponding list
		setHeaderName, err := extractSetHeaderName(name)
		if err != nil && err.Error() == ErrSHPropertyIncorrectFormat {
			panic(ErrSHPropertyIncorrectFormat)
		} else if err == nil {
			if responseHeaders[setHeaderName] == nil {
				responseHeaders[setHeaderName] = make([]string, 0)
			}
			responseHeaders[setHeaderName] =
				append(responseHeaders[setHeaderName], name)
		}
	}
	if len(responseHeaders) == 0 {
		return nil
	}
	return responseHeaders
}

// isPseudoHeader check if a header is pseudo header by checking for
// unit separator character in the header.
func isPseudoHeader(header string) bool {
	pseudoHeaderSep := byte(int(C.FIFTYONE_DEGREES_PSEUDO_HEADER_SEP))
	return strings.IndexByte(header, pseudoHeaderSep) > 0
}

// getUniqueHeaders returns the unique headers whose values will be needed
// to determine properties of each component.
func getUniqueHeaders(manager *ResourceManager) []string {
	// Get data set
	cDataSet := (*C.DataSetHash)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	// Release the dataset
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))
	// Map the items list to Go slice. This is done once so every access to
	// each result won't have to cast and create the slice again.
	//
	// The size of C.ResultHash array is governed by the address space and
	// and max value of a Integer. Thus take max Integer value and divided by
	// the size of a single C.ResultHash to make sure the total size of the array
	// will not be bigger than the size of the address space and the max Integer
	// value.
	var cUniqueHeaders = (*[math.MaxInt32 / int(C.sizeof_Header)]C.Header)(
		unsafe.Pointer(cDataSet.b.b.uniqueHeaders.items))[:cDataSet.b.b.uniqueHeaders.capacity:cDataSet.b.b.uniqueHeaders.capacity]

	uniqueHeadersCount := int(cDataSet.b.b.uniqueHeaders.count)
	a := make([]string, 0, uniqueHeadersCount)
	for i := 0; i < uniqueHeadersCount; i++ {
		header := C.GoString(cUniqueHeaders[i].name)
		a = append(a, header)
	}
	return a
}

// initHttpHeaderKeys initialized the http keys required for this engine
func initHttpHeaderKeys(manager *ResourceManager) {
	// Get data set
	cDataSet := (*C.DataSetHash)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	// Release the dataset
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))
	config := (*C.fiftyoneDegreesConfigBase)(unsafe.Pointer(cDataSet.b.b.config))
	usesUpperPrefixedHeaders := cIntToBool(int(C.BoolToInt(config.usesUpperPrefixedHeaders)))

	prefixes := []EvidencePrefix{HttpHeaderString, HttpEvidenceQuery}
	uniqueHeaders := getUniqueHeaders(manager)

	// Create http header keys
	keys := make([]EvidenceKey, 0, len(prefixes)*len(uniqueHeaders))
	for _, header := range uniqueHeaders {
		key := ""
		if usesUpperPrefixedHeaders {
			key += "HTTP_"
		}
		key += header
		for _, p := range prefixes {
			keys = append(keys, EvidenceKey{p, key})
		}
	}
	manager.HttpHeaderKeys = keys
}

func GetPublishedDate(manager *ResourceManager) time.Time {
	cDataSet := (*C.DataSetHash)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	// Release the dataset
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))
	published := cDataSet.header.published
	return time.Date(int(published.year), time.Month(published.month), int(published.day), 0, 0, 0, 0, time.UTC)
}

// GetNextUpdateDate returns the date after which the loaded data file expects
// a new data file to be available, or the zero time when the data file gives
// no date.
func GetNextUpdateDate(manager *ResourceManager) time.Time {
	cDataSet := (*C.DataSetHash)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	if cDataSet == nil {
		return time.Time{}
	}
	// Release the dataset
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))
	next := cDataSet.header.nextUpdate
	if next.year == 0 {
		return time.Time{}
	}
	return time.Date(int(next.year), time.Month(next.month), int(next.day), 0, 0, 0, 0, time.UTC)
}
