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

// PropertyNames holds the name of each required property by required property
// index. The names are those the engine was built with, or with none, those of
// the first data file it loaded. The C library keeps them across a reload, so
// one table, made when the engine is built, names the indexes of every data
// set the manager loads.
type PropertyNames struct {
	names []string
}

// NewPropertyNames reads the names from the manager's current data set. It is
// called once, after the manager's first load.
func NewPropertyNames(manager *ResourceManager) *PropertyNames {
	dataSet := (*C.DataSetIpi)(unsafe.Pointer(C.DataSetGet(manager.CPtr)))
	defer C.DataSetRelease((*C.DataSetBase)(unsafe.Pointer(dataSet)))
	return &PropertyNames{names: requiredPropertyNames(dataSet)}
}

// Release does nothing, because the names hold no data set.
//
// Deprecated: there is nothing to release, so remove the call.
func (p *PropertyNames) Release() {}

// For returns the names whichever data set the results were made from,
// because an index names the same property in every data set the manager
// loads. It does not look at r, so results from another manager's data set
// are given this manager's names.
//
// Deprecated: use Name, which gives the name at a required property index.
func (p *PropertyNames) For(r *ResultsIpi) []string {
	if p == nil {
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
// is the place of a property in the names the engine was given, or when it was
// given none, in the properties of the first data file it loaded.
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
