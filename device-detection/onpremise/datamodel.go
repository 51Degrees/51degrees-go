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

package onpremise

import (
	"errors"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
)

// errNoData is returned by the metadata of an engine with no data loaded.
var errNoData = errors.New("the engine has no data loaded")

// Components gives every component the data file in use defines, with the
// names of its Properties and its default profile.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/usage-examples.md?plain=1#L166-L184
func (e *Engine) Components() ([]dd.ComponentMetadata, error) {
	if e.manager == nil {
		return nil, errNoData
	}
	return e.manager.Components()
}

// Profiles gives every profile the data file in use holds, in order of id.
func (e *Engine) Profiles() ([]dd.ProfileMetadata, error) {
	if e.manager == nil {
		return nil, errNoData
	}
	return e.manager.Profiles()
}

// Values gives every value of every Property the data file in use defines.
func (e *Engine) Values() ([]dd.ValueMetadata, error) {
	if e.manager == nil {
		return nil, errNoData
	}
	return e.manager.Values()
}

// ProfileValues gives the values the profile with the id holds, and an
// error when the data file in use holds no such profile.
func (e *Engine) ProfileValues(profileId uint32) ([]dd.ValueMetadata, error) {
	if e.manager == nil {
		return nil, errNoData
	}
	return e.manager.ProfileValues(profileId)
}
