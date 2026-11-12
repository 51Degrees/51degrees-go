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

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
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

// messagePrefix begins every message in the table, naming the Property and the
// Data Key of the Engine. Each builder adds the sentence for its reason.
const messagePrefix = "Property '%s' not found in data for element '%s'. "

// noEngine is the name the message carries where there is no Engine to name.
const noEngine = "Unknown"

// ForDataFileTier builds the error for a Property the current license or data
// file does not include. dataTiers names the data tiers where the Property is
// present, from its metadata, and engineType is this implementation's own name
// for the Engine. Where dataTiers is empty the message leaves out the sentence
// naming the tiers, as the table's second row does.
func ForDataFileTier(property, element, engineType string, dataTiers []string) *core.PropertyMissingError {
	msg := fmt.Sprintf(messagePrefix, property, element) +
		"This is because your license or data file does not include this " +
		"property. "
	if len(dataTiers) == 0 {
		msg += "To see which licenses include it, see " +
			"https://51degrees.com/pricing."
	} else {
		msg += fmt.Sprintf("It is available with the %s license/data for "+
			"the %s. To upgrade, see https://51degrees.com/pricing.",
			strings.Join(dataTiers, ","), engineType)
	}
	return &core.PropertyMissingError{PropertyName: property, ElementName: element, Message: msg, Reason: ReasonDataFileTier}
}

// ForExcludedInConfig builds the error for a Property excluded by configuration.
func ForExcludedInConfig(property, element string) *core.PropertyMissingError {
	msg := fmt.Sprintf(messagePrefix, property, element) +
		"This is because the property has been excluded when configuring " +
		"the engine."
	return &core.PropertyMissingError{PropertyName: property, ElementName: element, Message: msg, Reason: ReasonExcludedInConfig}
}

// ForProductNotInResourceKey builds the error for a Resource Key that includes
// no Properties at all under the Engine's Data Key, which is the reason a
// license key without the Engine's product gives. The message names that Data
// Key as the product.
func ForProductNotInResourceKey(property, element string) *core.PropertyMissingError {
	msg := fmt.Sprintf(messagePrefix, property, element) +
		fmt.Sprintf("This is because your resource key does not include "+
			"access to any properties under '%s'. Check or create a "+
			"resource key with the properties you need in the configurator "+
			"at https://configure.51degrees.com.", element)
	return &core.PropertyMissingError{PropertyName: property, ElementName: element, Message: msg, Reason: ReasonProductNotInKey}
}

// ForPropertyNotInResourceKey builds the error for a Resource Key that
// includes the Engine's product but not this Property. includedProperties
// names the Properties in the Engine's metadata.
func ForPropertyNotInResourceKey(property, element string, includedProperties []string) *core.PropertyMissingError {
	msg := fmt.Sprintf(messagePrefix, property, element) +
		fmt.Sprintf("This is because your resource key does not include "+
			"access to this property. The properties included for your key "+
			"under '%s' are %s. Check or change your key in the "+
			"configurator at https://configure.51degrees.com.",
			element, strings.Join(includedProperties, ","))
	return &core.PropertyMissingError{PropertyName: property, ElementName: element, Message: msg, Reason: ReasonPropertyNotInKey}
}

// Generic builds the error for a Property whose reason is unknown, which is
// the reason where none of the others applies.
func Generic(property, element string) *core.PropertyMissingError {
	msg := fmt.Sprintf(messagePrefix, property, element) +
		"The reason for this is unknown. Please check that the aspect and " +
		"property name are correct."
	return &core.PropertyMissingError{PropertyName: property, ElementName: element, Message: msg, Reason: ReasonUnknown}
}

// ForCloudRequestFailed builds the error for a Property that is missing
// because the request to the remote server failed, as opposed to the Property
// not being accessible with the Resource Key. element is the Data Key of the
// first Engine that populates the Element Data, and an empty element is named
// Unknown, which is the case where there is no such Engine.
func ForCloudRequestFailed(property, element string) *core.PropertyMissingError {
	if element == "" {
		element = noEngine
	}
	msg := fmt.Sprintf(messagePrefix, property, element) +
		"This is because an upstream cloud request failed, so no data was " +
		"available to populate the property."
	return &core.PropertyMissingError{
		PropertyName: property, ElementName: element, Message: msg,
		Reason: ReasonCloudRequestFailed,
	}
}
