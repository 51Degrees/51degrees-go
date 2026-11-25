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

package ipi_interop

//#include <string.h>
//#include "ip-intelligence-cxx.h"
import "C"
import (
	"strings"
	"time"
	"unsafe"
)

// InitFromFile initializes a resource manager based on a given data file path.
// The input properties is a comma separated string list. This matches the C API
// fiftyoneDegreesIpiInitManagerFromFile.
func InitManagerFromFile(manager *ResourceManager, config ConfigIpi, properties string, filePath string) error {
	exp := NewException()
	defer exp.Free()

	cPath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cPath))

	propsRequired := NewPropertiesRequired(properties)
	defer propsRequired.Free()

	s := C.IpiInitManagerFromFile(
		manager.CPtr,
		config.CPtr,
		propsRequired.CPtr,
		cPath,
		exp.CPtr,
	)

	// Check exception
	if !exp.IsOkay() {
		return messageError(C.ExceptionGetMessage(exp.CPtr))
	}

	// Check status code
	if s != C.SUCCESS {
		return messageError(C.StatusGetMessage(s, cPath))
	}

	initEvidenceKeys(manager)
	return nil
}

// evidenceKeysSeparator is written between the keys the data set lists.
const evidenceKeysSeparator = ","

// initEvidenceKeys works out the Evidence keys the data in use accepts, from
// the data set, and keeps them in the order the native code tries them, which
// is every query key before any server key, each in the data set's order.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L75-L84
func initEvidenceKeys(manager *ResourceManager) {
	cDataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))
	separator := C.CString(evidenceKeysSeparator)
	defer C.free(unsafe.Pointer(separator))
	length := C.IpiGetEvidenceKeys(cDataSet, nil, 0, separator)
	buffer := (*C.char)(C.malloc(length + 1))
	defer C.free(unsafe.Pointer(buffer))
	C.IpiGetEvidenceKeys(cDataSet, buffer, length+1, separator)

	var query, server []EvidenceKey
	for _, key := range strings.Split(C.GoString(buffer),
		evidenceKeysSeparator) {
		cKey := C.CString(key)
		prefix := C.EvidenceMapPrefix(cKey)
		C.free(unsafe.Pointer(cKey))
		if prefix == nil {
			continue
		}
		name := strings.TrimPrefix(key[int(prefix.prefixLength):], ".")
		k := EvidenceKey{EvidencePrefix(prefix.prefixEnum), name}
		if k.Prefix == HttpEvidenceQuery {
			query = append(query, k)
		} else {
			server = append(server, k)
		}
	}
	manager.keysMu.Lock()
	manager.HttpHeaderKeys = append(query, server...)
	manager.keysMu.Unlock()
}

// ReloadFromFile reloads the data set of the manager from the data file at
// filePath, through fiftyoneDegreesIpiReloadManagerFromFile. The new data set
// keeps the configuration and the property names the manager was created
// with, or with no names, those of the first data file it loaded, as a reload
// of the original file does, so each index names the same property
// afterwards. The C reload makes the new data set active through
// fiftyoneDegreesResourceReplace, which frees the data set it replaced once the
// last lookup using it has finished, so nothing is held here. A failed reload
// keeps the data set in use and returns an error. A reload that succeeds
// works the accepted Evidence keys out again from the new data.
//
// Deprecated: config and properties are ignored, because a reload keeps the
// manager's configuration and names. Pass the values the manager was created
// with.
func (manager *ResourceManager) ReloadFromFile(config ConfigIpi, properties string, filePath string) error {
	exp := NewException()
	defer exp.Free()

	cPath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cPath))

	s := C.IpiReloadManagerFromFile(manager.CPtr, cPath, exp.CPtr)
	if err := reloadError(s, exp, cPath); err != nil {
		return err
	}
	initEvidenceKeys(manager)
	return nil
}

// ReloadFromOriginalFile reloads the data set being used by the resource
// manager using the data file location which was used when the manager was
// created, and works the accepted Evidence keys out again from the new data.
// This is corresponding to the C API
// fiftyoneDegreesIpiReloadManagerFromOriginalFile
func (manager *ResourceManager) ReloadFromOriginalFile() error {
	exp := NewException()
	defer exp.Free()

	s := C.IpiReloadManagerFromOriginalFile(
		manager.CPtr,
		exp.CPtr,
	)
	if s == C.SUCCESS && exp.IsOkay() {
		initEvidenceKeys(manager)
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

// GetPublishedDateFromFile returns the date the IP Intelligence data file at
// path was published, read from its header alone without loading the file,
// so that an update can tell whether a new data file is newer before loading
// it. It returns the zero time and an error when the file cannot be opened,
// is not an IP Intelligence data file of the version this library reads, or
// holds no valid date. This matches the C API
// fiftyoneDegreesIpiGetPublishedFromFile.
func GetPublishedDateFromFile(path string) (time.Time, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	var date C.fiftyoneDegreesDate
	if s := C.fiftyoneDegreesIpiGetPublishedFromFile(cPath, &date); s != C.SUCCESS {
		return time.Time{}, messageError(C.StatusGetMessage(s, cPath))
	}
	return time.Date(int(date.year), time.Month(date.month), int(date.day), 0, 0, 0, 0, time.UTC), nil
}

func GetPublishedDate(manager *ResourceManager) time.Time {
	cDataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	// Release the dataset
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))
	published := cDataSet.header.published
	return time.Date(int(published.year), time.Month(published.month), int(published.day), 0, 0, 0, 0, time.UTC)
}

// GetNextUpdateDate returns the date after which the loaded data file expects
// a new data file to be available, or the zero time when the data file gives
// no date.
func GetNextUpdateDate(manager *ResourceManager) time.Time {
	cDataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
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
