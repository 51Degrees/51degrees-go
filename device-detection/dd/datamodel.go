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

// ComponentMetadata describes a component of the data file. A component
// groups related Properties, such as those of the hardware, and each of its
// profiles holds values of each of them.
type ComponentMetadata struct {
	// Id stays the same when the data file is updated.
	Id uint8
	// Name is the component's name, such as HardwarePlatform.
	Name string
	// DefaultProfileId is the profile whose values the component's
	// Properties have when no profile matches the evidence.
	DefaultProfileId uint32
	// Properties are the names of the component's Properties.
	Properties []string
}

// ProfileMetadata describes a profile of the data file, which holds values
// of the Properties of one component.
type ProfileMetadata struct {
	// Id stays the same when the data file is updated.
	Id uint32
	// ComponentId is the id of the component the profile holds values for.
	ComponentId uint8
}

// ValueMetadata describes a value a Property of the data file can have.
type ValueMetadata struct {
	// Property is the name of the Property the value belongs to.
	Property string
	// Name is the value as text, unique among the Property's values.
	Name string
	// Description says what the value means.
	Description string
	// Url links to more about the value.
	Url string
	// IsDefault says the value is its Property's default value.
	IsDefault bool
}

// Components gives every component the data file defines.
func (manager *ResourceManager) Components() ([]ComponentMetadata, error) {
	return nil, nil
}

// Profiles gives every profile the data file holds.
func (manager *ResourceManager) Profiles() ([]ProfileMetadata, error) {
	return nil, nil
}

// Values gives every value of every Property the data file defines.
func (manager *ResourceManager) Values() ([]ValueMetadata, error) {
	return nil, nil
}

// ProfileValues gives the values the profile with the id holds.
func (manager *ResourceManager) ProfileValues(
	profileId uint32) ([]ValueMetadata, error) {
	return nil, nil
}
