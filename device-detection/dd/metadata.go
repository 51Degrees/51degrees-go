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

import "reflect"

// PropertyMetadata describes a Property the data file defines.
type PropertyMetadata struct {
	// Name is the name a value of the Property is read by.
	Name string
	// Category groups the Property with others.
	Category string
	// Description says what the Property gives.
	Description string
	// Url links to more about the Property.
	Url string
	// Type is the Go type a value of the Property is read as.
	Type reflect.Type
	// DataTiers are the data files that hold the Property, such as Lite.
	DataTiers []string
	// Available says the manager was made with the Property, so results
	// hold its values.
	Available bool
	// IsList says the Property can have more than one value.
	IsList bool
	// IsObsolete says the Property will leave future data files.
	IsObsolete bool
	// ComponentId is the id of the component the Property belongs to.
	ComponentId uint8
}

// Properties gives the metadata of every Property the data file defines.
func (manager *ResourceManager) Properties() ([]PropertyMetadata, error) {
	return nil, nil
}
