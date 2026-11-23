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
#include <string.h>
#include "ip-intelligence-cxx.h"

// dataSetName copies the name the data set gives itself into name, which
// holds length bytes, and gives false when the name cannot be read.
static bool dataSetName(
	fiftyoneDegreesDataSetIpi *dataSet,
	char *name,
	size_t length,
	fiftyoneDegreesException *exception) {
	fiftyoneDegreesCollectionItem item;
	fiftyoneDegreesDataReset(&item.data);
	const fiftyoneDegreesString *text = fiftyoneDegreesStringGet(
		dataSet->strings,
		(uint32_t)dataSet->header.nameOffset,
		&item,
		exception);
	if (text == NULL) {
		return false;
	}
	strncpy(name, &text->value, length - 1);
	name[length - 1] = '\0';
	FIFTYONE_DEGREES_COLLECTION_RELEASE(dataSet->strings, &item);
	return true;
}
*/
import "C"
import "unsafe"

// dataSetNameBytes is the buffer the name of a data set is read into.
const dataSetNameBytes = 256

// GetDataSetName returns the name the loaded data file gives itself, which
// is its data tier, such as Lite or Enterprise. It returns an empty string
// when the name cannot be read.
func GetDataSetName(manager *ResourceManager) string {
	cDataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	if cDataSet == nil {
		return ""
	}
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(cDataSet)))
	var buffer [dataSetNameBytes]C.char
	e := NewException()
	read := C.dataSetName(cDataSet, &buffer[0], C.size_t(len(buffer)), e.CPtr)
	if !bool(read) || !e.IsOkay() {
		return ""
	}
	return C.GoString(&buffer[0])
}
