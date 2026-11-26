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
#include "device-detection-cxx.h"
*/
import "C"
import (
	"errors"
	"unsafe"
)

// errProductFromFileChanged is the error GetProductFromFile gives when the
// product name read from the header of the data file needs more bytes than
// the read just before it said, which happens only when the file is replaced
// between the two reads.
var errProductFromFileChanged = errors.New(
	"the data file changed while its product name was read")

// GetProductFromFile returns the name the Distributor gives the product of
// the Device Detection Hash data file at fileName, such as V4TAC, read from the header of
// the file without loading it. It returns an error when the file cannot be
// opened, is not a Device Detection Hash data file this library reads, or names no
// tier.
// See https://github.com/51Degrees/specifications/blob/a5699da253d13661478f5923964c22d1010c1df3/pipeline-specification/features/data-updates.md#product-names
func GetProductFromFile(fileName string) (string, error) {
	cName := C.CString(fileName)
	defer C.free(unsafe.Pointer(cName))

	// The first read gives the bytes the name and its terminating null need.
	var needed C.size_t
	s := C.fiftyoneDegreesHashGetProductFromFile(cName, nil, 0, &needed)
	if s != C.FIFTYONE_DEGREES_STATUS_INSUFFICIENT_CAPACITY {
		return "", messageError(C.StatusGetMessage(s, cName))
	}

	buffer := (*C.char)(C.malloc(needed))
	defer C.free(unsafe.Pointer(buffer))
	length := needed
	s = C.fiftyoneDegreesHashGetProductFromFile(cName, buffer, length, &needed)
	switch {
	case s == C.FIFTYONE_DEGREES_STATUS_INSUFFICIENT_CAPACITY:
		return "", errProductFromFileChanged
	case s != C.FIFTYONE_DEGREES_STATUS_SUCCESS:
		return "", messageError(C.StatusGetMessage(s, cName))
	}
	return C.GoString(buffer), nil
}
