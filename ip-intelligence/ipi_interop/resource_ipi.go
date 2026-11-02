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

//#include <string.h>
//#include "ip-intelligence-cxx.h"
import "C"
import (
	"errors"
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

	return nil
}

// ReloadFromFile replaces the data set being used by the resource manager with
// one loaded from the specified data file, using the configuration and
// properties given. It calls the C API fiftyoneDegreesIpiInitManagerFromFile,
// which checks the status and the exception before it makes the new data set
// active, so a file that fails to load leaves the data set in use unchanged.
func (manager *ResourceManager) ReloadFromFile(config ConfigIpi, properties string, filePath string) error {
	exp := NewException()
	defer exp.Free()

	cPath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cPath))

	propsRequired := NewPropertiesRequired(properties)
	defer propsRequired.Free()

	// The init function makes the new data set active without releasing the
	// one it replaces. Holding that one across the call and releasing it
	// afterwards frees it once the last lookup using it has finished.
	previous := C.DataSetGet(manager.CPtr)
	defer C.DataSetRelease(previous)

	s := C.IpiInitManagerFromFile(
		manager.CPtr,
		config.CPtr,
		propsRequired.CPtr,
		cPath,
		exp.CPtr,
	)
	return reloadError(s, exp, cPath)
}

// ReloadFromOriginalFile reloads the data set being used by the resource
// manager using the data file location which was used when the manager was
// created. This is corresponding to the C API
// fiftyoneDegreesIpiReloadManagerFromOriginalFile
func (manager *ResourceManager) ReloadFromOriginalFile() error {
	exp := NewException()
	defer exp.Free()

	s := C.IpiReloadManagerFromOriginalFile(
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
		cMessage := C.ExceptionGetMessage(exp.CPtr)
		defer C.MemoryStandardFree(unsafe.Pointer(cMessage))
		return errors.New(C.GoString(cMessage))
	}
	if s != C.SUCCESS {
		cMessage := C.StatusGetMessage(s, fileName)
		defer C.MemoryStandardFree(unsafe.Pointer(cMessage))
		return errors.New(C.GoString(cMessage))
	}
	return nil
}

func GetPublishedDate(manager *ResourceManager) time.Time {
	cDataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	// Release the dataset
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))
	published := cDataSet.header.published
	return time.Date(int(published.year), time.Month(published.month), int(published.day), 0, 0, 0, 0, time.UTC)
}
