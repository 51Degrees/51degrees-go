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

package ipi_onpremise

import (
	"errors"
	"time"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

// errNoData is returned by the metadata of an engine with no data loaded.
var errNoData = errors.New("the engine has no data loaded")

// Properties gives the metadata of every Property the data file in use
// defines, with each one's category, description, type, the data tiers that
// hold its values, and whether the engine was made with it.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/required-examples.md?plain=1#L167-L173
func (e *Engine) Properties() ([]ipi_interop.PropertyMetadata, error) {
	if !e.acquire() {
		return nil, errEngineStopped
	}
	defer e.release()
	if e.manager == nil {
		return nil, errNoData
	}
	return e.manager.Properties()
}

// Components gives every component the data file in use defines, with the
// names of its Properties.
func (e *Engine) Components() ([]ipi_interop.ComponentMetadata, error) {
	if !e.acquire() {
		return nil, errEngineStopped
	}
	defer e.release()
	if e.manager == nil {
		return nil, errNoData
	}
	return e.manager.Components()
}

// PublishedDate is the date the data file in use was published, or the zero
// time before a data file is loaded.
func (e *Engine) PublishedDate() time.Time {
	return e.publishedDate()
}

// NextUpdateDate is the date after which the data file in use expects a newer
// one, or the zero time when it gives none or no data file is loaded.
func (e *Engine) NextUpdateDate() time.Time {
	return e.nextUpdateDate()
}
