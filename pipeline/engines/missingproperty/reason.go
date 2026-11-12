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
	"reflect"
	"strings"

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

// Decide gives the error for a Property the Element Data does not hold, with
// the first reason that applies in the specification's order. engines are
// the Engines that populate the Element Data.
//
// A cloud request that failed for the Flow Data decides first, naming the
// first Engine. Otherwise each Engine is asked in turn and the first to give
// a reason other than Unknown decides. Where every Engine gives Unknown the
// message names the last, and with no Engine at all it names Unknown.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/properties.md#deciding-the-reason
func Decide(property string, engines []Engine, ctx Context) *core.PropertyMissingError {
	if ctx.CloudRequestFailed {
		for _, e := range engines {
			if e != nil {
				return ForCloudRequestFailed(property, e.DataKey())
			}
		}
		return ForCloudRequestFailed(property, "")
	}
	var result *core.PropertyMissingError
	for _, e := range engines {
		if e == nil {
			continue
		}
		result = decideForEngine(property, e, ctx.Accessors)
		if result.Reason != ReasonUnknown {
			return result
		}
	}
	if result == nil {
		return Generic(property, noEngine)
	}
	return result
}

// decideForEngine gives the error one Engine gives for the Property. Names
// are compared ignoring case, as the Element Data compares them when a
// value is read.
func decideForEngine(property string, e Engine, accessors []string) *core.PropertyMissingError {
	key := e.DataKey()
	cloud, isCloud := e.(CloudEngine)
	loaded := true
	if l, ok := e.(core.PropertiesLoader); ok {
		loaded = l.PropertiesLoaded()
	}

	// Metadata that lists the Property. An On-premise Engine whose data file
	// tier is not among the Property's tiers needs another data file, and
	// otherwise a Property that is not available was excluded.
	var properties []core.PropertyMetadata
	listed := false
	if loaded {
		properties = e.Properties()
		for _, p := range properties {
			if !strings.EqualFold(p.Name, property) {
				continue
			}
			listed = true
			if !isCloud && !contains(p.DataTiersWherePresent, e.DataSourceTier()) {
				return ForDataFileTier(property, key, typeName(e),
					p.DataTiersWherePresent)
			}
			if !p.Available {
				return ForExcludedInConfig(property, key)
			}
			break
		}
	}

	// A Cloud Engine. Loaded metadata that lists no Properties means the
	// Resource Key has no product for the Engine.
	if isCloud {
		if loaded {
			if len(properties) == 0 {
				return ForProductNotInResourceKey(property, key)
			}
			return ForPropertyNotInResourceKey(property, key,
				propertyNames(properties))
		}
		if cloud.PropertyListRequestFailed() {
			return ForCloudRequestFailed(property, key)
		}
		return Generic(property, key)
	}

	// An On-premise Engine whose metadata does not list the Property, where
	// the Element Data has a typed accessor for it. The data tiers are not
	// known, so the message leaves them out.
	if !listed && containsFold(accessors, property) {
		return ForDataFileTier(property, key, typeName(e), nil)
	}
	return Generic(property, key)
}

// typeName is this implementation's own name for the Engine, being its
// package and type, for example ipi_onpremise.Engine.
func typeName(e Engine) string {
	t := reflect.TypeOf(e)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.String()
}

// propertyNames gives the name of each Property in properties.
func propertyNames(properties []core.PropertyMetadata) []string {
	names := make([]string, 0, len(properties))
	for _, p := range properties {
		names = append(names, p.Name)
	}
	return names
}

// contains reports whether values holds value exactly.
func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

// containsFold reports whether values holds value, ignoring case.
func containsFold(values []string, value string) bool {
	for _, v := range values {
		if strings.EqualFold(v, value) {
			return true
		}
	}
	return false
}
