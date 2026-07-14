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

// Package missingproperty constructs the errors returned when a Property that is
// defined for an Engine is not present in the result set.
//
// The messages match those defined in the specification exactly so they are
// consistent across language implementations.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/properties.md#missing-properties
package missingproperty

import (
	"fmt"
	"strings"

	"github.com/51Degrees/pipeline-go/core"
)

// Reason codes set on core.PropertyMissingError.Reason so a caller can branch
// on why a Property is missing without parsing the message.
const (
	ReasonUnknown            = "unknown"
	ReasonDataFileTier       = "datafiletier"
	ReasonExcludedInConfig   = "excludedinconfig"
	ReasonProductNotInKey    = "productnotinresourcekey"
	ReasonPropertyNotInKey   = "propertynotinresourcekey"
	ReasonCloudRequestFailed = "cloudrequestfailed"
)

// ForDataFileTier builds the error for a Property absent from the current data
// file. dataTiers lists the license/data file types in which it is available.
func ForDataFileTier(property, element string, dataTiers []string) *core.PropertyMissingError {
	tiers := strings.Join(dataTiers, ", ")
	if tiers == "" {
		tiers = "Unknown" // matches the .NET reference when the tier list is absent
	}
	msg := fmt.Sprintf("Property '%s' not found in data for element '%s'. "+
		"This is because your license and/or data file does not include this Property. "+
		"The Property is available with the %s license/data.",
		property, element, tiers)
	return &core.PropertyMissingError{PropertyName: property, ElementName: element, Message: msg, Reason: ReasonDataFileTier}
}

// ForExcludedInConfig builds the error for a Property excluded by configuration.
func ForExcludedInConfig(property, element string) *core.PropertyMissingError {
	msg := fmt.Sprintf("Property '%s' not found in data for element '%s'. "+
		"This is because the Property has been excluded when configuring the Engine.",
		property, element)
	return &core.PropertyMissingError{PropertyName: property, ElementName: element, Message: msg, Reason: ReasonExcludedInConfig}
}

// ForProductNotInResourceKey builds the error for a product not included with
// the Resource Key.
func ForProductNotInResourceKey(property, element, product string) *core.PropertyMissingError {
	msg := fmt.Sprintf("Property '%s' not found in data for element '%s'. "+
		"This is because your Resource Key does not include access to any Properties under '%s'. "+
		"For more details on Resource Keys, see our explainer: "+
		"https://51degrees.com/documentation/_info__resource_keys.html",
		property, element, product)
	return &core.PropertyMissingError{PropertyName: property, ElementName: element, Message: msg, Reason: ReasonProductNotInKey}
}

// ForPropertyNotInResourceKey builds the error for a Property not included with
// the Resource Key. includedProperties lists the Properties that are available
// under the product for this key.
func ForPropertyNotInResourceKey(property, element, product string, includedProperties []string) *core.PropertyMissingError {
	msg := fmt.Sprintf("Property '%s' not found in data for element '%s'. "+
		"This is because your Resource Key does not include access to this Property. "+
		"Properties that are included for this key under '%s' are %s. "+
		"For more details on Resource Keys, see our explainer: "+
		"https://51degrees.com/documentation/_info__resource_keys.html",
		property, element, product, strings.Join(includedProperties, ", "))
	return &core.PropertyMissingError{PropertyName: property, ElementName: element, Message: msg, Reason: ReasonPropertyNotInKey}
}

// Generic builds a non-specific missing-property error.
func Generic(property, element string) *core.PropertyMissingError {
	msg := fmt.Sprintf("Property '%s' not found in data for element '%s'.", property, element)
	return &core.PropertyMissingError{PropertyName: property, ElementName: element, Message: msg, Reason: ReasonUnknown}
}

// ForCloudRequestFailed builds the error for a Property that is missing because
// the cloud request failed, as opposed to the Property not being accessible
// with the Resource Key.
func ForCloudRequestFailed(property, element string) *core.PropertyMissingError {
	msg := fmt.Sprintf("Property '%s' not found in data for element '%s'. "+
		"This is because the request to the cloud service failed.",
		property, element)
	return &core.PropertyMissingError{
		PropertyName: property, ElementName: element, Message: msg,
		Reason: ReasonCloudRequestFailed,
	}
}
