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
	"reflect"
	"strings"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// DataKey is the key of the engine's Element Data in a Flow Data, the same as
// the Device Detection Cloud Engine's.
const DataKey = "device"

// DeviceData is the Aspect Data the engine gives, from ProcessData or as a
// Pipeline engine. Each Property is held under its name in lower case. A
// Property the data file holds as a list gives an
// engines.AspectPropertyValue[[]string], and any other Property an
// engines.AspectPropertyValue[string]. A Property with no value says why.
// The values are copied out of the native results, so the data holds no
// native memory and needs no freeing.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/data-model.md?plain=1#L10-L12
type DeviceData struct {
	*engines.BaseAspectData
}

// ProcessData detects the device from the evidence, as Process does, and
// gives the results as Aspect Data.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/data-model.md?plain=1#L10-L12
func (e *Engine) ProcessData(evidenceList []Evidence) (*DeviceData, error) {
	results, err := e.Process(evidenceList)
	if err != nil {
		return nil, err
	}
	defer results.Free()
	return e.deviceData(results), nil
}

// deviceData copies the value of each Property the results hold into
// DeviceData.
func (e *Engine) deviceData(results *dd.ResultsHash) *DeviceData {
	data := &DeviceData{BaseAspectData: engines.NewBaseAspectData()}
	kinds := e.propertyKinds()
	for index, name := range results.AvailableProperties() {
		key := strings.ToLower(name)
		values, reason := propertyValues(results, index)
		list := kinds[key] == listProperty
		switch {
		case list && reason == "":
			data.Set(key, engines.NewAspectPropertyValue(values))
		case list:
			data.Set(key, engines.NoValue[[]string](reason))
		case reason == "" && kinds[key] == booleanProperty:
			data.Set(key, engines.NewAspectPropertyValue(
				strings.ToLower(strings.Join(values, "|"))))
		case reason == "":
			data.Set(key, engines.NewAspectPropertyValue(strings.Join(values, "|")))
		default:
			data.Set(key, engines.NoValue[string](reason))
		}
	}
	return data
}

// propertyKind is how DeviceData gives the value of a Property.
type propertyKind int

const (
	// textProperty is given as its text, as the data file holds it.
	textProperty propertyKind = iota
	// listProperty is given as a list of Strings.
	listProperty
	// booleanProperty is given as "true" or "false", as the Device Detection
	// Cloud Engine gives a Boolean, where the data file holds "True" or
	// "False".
	booleanProperty
)

// propertyValues gives the values of the Property at the required property
// index, or the reason it has none.
func propertyValues(results *dd.ResultsHash, index int) ([]string, string) {
	has, err := results.HasValuesByIndex(index)
	if err != nil {
		return nil, err.Error()
	}
	if !has {
		reason, err := results.NoValueReasonMessageByIndex(index)
		if err != nil {
			return nil, err.Error()
		}
		return nil, noValueReason(reason)
	}
	values, err := results.Values(index)
	if err != nil {
		return nil, err.Error()
	}
	if len(values) == 0 {
		return nil, noValueReason("")
	}
	return values, ""
}

// noValueReason gives a reason for a Property with no value, where the data
// file gave none.
func noValueReason(reason string) string {
	if reason == "" {
		return "The data file holds no value of this Property for the evidence."
	}
	return reason
}

// propertyKinds gives, by lower case name, how DeviceData gives each Property
// of the data file in use that is not text. The answer is kept once the data
// file has given it.
func (e *Engine) propertyKinds() map[string]propertyKind {
	if kinds := e.kinds.Load(); kinds != nil {
		return *kinds
	}
	properties, err := e.Properties()
	if err != nil {
		return map[string]propertyKind{}
	}
	kinds := make(map[string]propertyKind, len(properties))
	for _, p := range properties {
		switch {
		case p.IsList:
			kinds[strings.ToLower(p.Name)] = listProperty
		case p.Type == reflect.TypeOf(true):
			kinds[strings.ToLower(p.Name)] = booleanProperty
		}
	}
	e.kinds.Store(&kinds)
	return kinds
}
