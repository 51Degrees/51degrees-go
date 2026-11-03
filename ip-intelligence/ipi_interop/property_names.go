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

//#include "ip-intelligence-cxx.h"
import "C"
import "unsafe"

// PropertyNames holds the name of each required property of one data set, by
// required property index. It keeps that data set in memory until Release,
// so while it exists no other data set can be at the same address, and For
// can tell results made from it by their data set pointer alone.
type PropertyNames struct {
	dataSet *C.DataSetIpi
	names   []string
}

// NewPropertyNames reads the names from the manager's current data set. The
// caller must call Release before the manager is freed.
func NewPropertyNames(manager *ResourceManager) *PropertyNames {
	dataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	return &PropertyNames{dataSet: dataSet, names: requiredPropertyNames(dataSet)}
}

// Release lets the data set be freed once nothing else uses it.
func (p *PropertyNames) Release() {
	if p.dataSet != nil {
		C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(p.dataSet)))
		p.dataSet = nil
	}
}

// For returns the names when the results were made from the data set they
// were read from, and nil otherwise.
func (p *PropertyNames) For(r *ResultsIpi) []string {
	if p == nil || p.dataSet == nil || r == nil || r.CPtr == nil ||
		unsafe.Pointer(r.CPtr.b.dataSet) != unsafe.Pointer(p.dataSet) {
		return nil
	}
	return p.names
}

// Name returns the name at the required property index, or an empty string
// for an index outside the required properties.
func (p *PropertyNames) Name(index int) string {
	if p == nil || index < 0 || index >= len(p.names) {
		return ""
	}
	return p.names[index]
}

// requiredPropertyNames reads the name at each required property index, which
// is the place of a property in the names the engine was given, or in the data
// set's own properties when it was given none.
func requiredPropertyNames(dataSet *C.DataSetIpi) []string {
	available := dataSet.b.b.available
	names := make([]string, int(C.fiftyoneDegreesPropertiesGetRequiredPropertyCount(available)))
	for i := range names {
		name := C.fiftyoneDegreesPropertiesGetNameFromRequiredIndex(available, C.int(i))
		if name != nil {
			names[i] = C.GoString(&name.value)
		}
	}
	return names
}
