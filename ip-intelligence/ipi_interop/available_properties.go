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
import "unsafe"

// GetAvailablePropertyNames returns the names of all properties available in
// the dataset after the manager has been initialized. It enumerates the
// available-properties set stored in the dataset, which is the same set used
// by the C results-collection function when it receives a NULL property-index
// array (i.e. the "return all properties" path).
//
// This function is called by the engine when no explicit properties list was
// supplied, so that the Go-side name cache covers every property the C engine
// may return.
func GetAvailablePropertyNames(manager *ResourceManager) []string {
	cDataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))

	count := int(cDataSet.b.b.available.count)
	names := make([]string, 0, count)
	for i := 0; i < count; i++ {
		res := C.fiftyoneDegreesPropertiesGetNameFromRequiredIndex(
			cDataSet.b.b.available, C.int(i),
		)
		if res != nil {
			names = append(names, C.GoString(&res.value))
		}
	}
	return names
}
