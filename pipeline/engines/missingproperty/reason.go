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

package missingproperty

import (
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// Engine is what the reason for a missing Property is decided from, being an
// Engine that populates the Element Data the Property was asked of.
type Engine interface {
	// DataKey is the Data Key of the Engine, which the message names.
	DataKey() string
	// Properties is the Engine's Property metadata.
	Properties() []core.PropertyMetadata
	// DataSourceTier is the tier of the data file an On-premise Engine uses.
	DataSourceTier() string
}

// CloudEngine is an Engine whose Properties come from the cloud service. An
// Engine that does not implement it is an On-premise Engine.
type CloudEngine interface {
	Engine
	// PropertyListRequestFailed reports whether the Engine's request for its
	// property list failed, leaving its metadata unloaded.
	PropertyListRequestFailed() bool
}

// Context is what Decide knows beyond the Engines' own metadata.
type Context struct {
	// CloudRequestFailed is true when a Cloud Aspect Engine populated the
	// Element Data and the Cloud Request Engine has no response for this
	// Flow Data because its request to the remote server failed.
	CloudRequestFailed bool
	// Accessors names the typed accessors of the Element Data, counting
	// those its type inherits.
	Accessors []string
}

// Decide gives the error for a Property that engines define and the Element
// Data does not hold.
func Decide(property string, engines []Engine, ctx Context) *core.PropertyMissingError {
	return Generic(property, noEngine)
}
