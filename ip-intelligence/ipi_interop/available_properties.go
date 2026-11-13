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
import "unsafe"

// GetAvailablePropertyNames returns the names of the required properties that
// the manager's current data set has, in required property index order. They
// are the names the engine was given that the data file has, or when the
// engine was given none, the properties of the first data file it loaded that
// the current one has.
func GetAvailablePropertyNames(manager *ResourceManager) []string {
	cDataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))

	available := cDataSet.b.b.available
	count := int(C.fiftyoneDegreesPropertiesGetRequiredPropertyCount(available))
	names := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if C.fiftyoneDegreesPropertiesGetAvailableIndex(available, C.int(i)) < 0 {
			continue
		}
		res := C.fiftyoneDegreesPropertiesGetNameFromRequiredIndex(available, C.int(i))
		if res != nil {
			names = append(names, C.GoString(&res.value))
		}
	}
	return names
}
