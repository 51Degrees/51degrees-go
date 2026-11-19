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
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/missingproperty"
)

// AspectData is the Element Data the AspectEngine adds to a Flow Data. It is
// Aspect Data, and is read as the Cloud Aspect Engine's Element Data is.
// Get takes a Property name in any case and gives an
// engines.AspectPropertyValue[[]core.WeightedValue[string]], holding each
// value of the Property as text with its raw weighting, or no value and the
// reason.
//
// A Property's values are read from the native results the first time the
// Property is requested, and kept. Processing reads none.
//
// AspectData is not safe to use from several goroutines at once, because
// reading a value writes into the native results, and AspectData takes no
// lock. The caller is responsible for thread-safe access to one it shares,
// for example with a sync.Mutex of its own.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L520-L525
//
// The native results hold memory the Go garbage collector does not manage.
// Close frees it, and a read after Close returns ErrDataFreed. An AspectData
// that is never closed frees it when the garbage collector finds the
// AspectData is no longer used.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/data-model.md?plain=1#L10-L37
type AspectData struct {
	*engines.BaseAspectData
	engine *AspectEngine
	data   *Data
}

// newAspectData wraps the Data of one lookup as Aspect Data, and has the
// garbage collector free its native results if Close is never called.
func newAspectData(engine *AspectEngine, data *Data) *AspectData {
	d := &AspectData{
		BaseAspectData: engines.NewBaseAspectData(),
		engine:         engine,
		data:           data,
	}
	runtime.SetFinalizer(d, func(d *AspectData) { d.data.Free() })
	return d
}

// Get returns the value of the named Property, ignoring the case of the
// name. For a Property the Engine was created with it is an
// engines.AspectPropertyValue[[]core.WeightedValue[string]], read from the
// native results the first time the Property is requested. For any other
// name the error is a core.PropertyMissingError that gives the reason.
func (d *AspectData) Get(propertyName string) (any, error) {
	index, ok := d.engine.positions[strings.ToLower(propertyName)]
	if !ok {
		return d.missing(propertyName)
	}
	return d.value(index, propertyName)
}

// AsMap returns the value of every Property the Engine was created with, by
// its name in lower case, reading each one that has not been read yet.
// After Close it holds none of them.
func (d *AspectData) AsMap() map[string]any {
	all := d.BaseAspectData.AsMap()
	for index, name := range d.engine.names {
		if value, err := d.value(index, name); err == nil {
			all[name] = value
		}
	}
	return all
}

// Close frees the native results. It is safe to call more than once, but not
// while another goroutine reads the AspectData.
func (d *AspectData) Close() error {
	d.data.Free()
	return nil
}

// value reads the Property at the position and gives its values as text,
// each with its raw weighting, or no value and the reason.
func (d *AspectData) value(index int, name string) (any, error) {
	read, err := d.data.ValuesByIndex(index)
	if err != nil {
		return nil, err
	}
	if len(read) == 0 {
		return engines.NoValue[[]core.WeightedValue[string]](
			d.noValueMessage(name)), nil
	}
	values := make([]core.WeightedValue[string], 0, len(read))
	for _, v := range read {
		values = append(values, core.NewWeightedValue(
			valueText(v.Value), toRawWeighting(v.Weight)))
	}
	return engines.NewAspectPropertyValue(values), nil
}

// noValueMessage says why the Property has no value.
func (d *AspectData) noValueMessage(name string) string {
	if !d.data.HasValues() {
		return "The Evidence held no IP address that the Engine could " +
			"read, so the Property '" + name + "' has no value. Add the " +
			"address as Evidence under one of the keys the Engine " +
			"accepts, such as query.client-ip."
	}
	return "The data file in use holds no value of the Property '" + name +
		"' for this IP address."
}

// missing gives the answer for a name that is not a Property the Engine was
// created with. A value another Flow Element set under the name is returned.
// A Property the data file in use defines was left out of the Properties the
// Engine was created with, and any other name is explained by the Engine's
// metadata.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L400-L410
func (d *AspectData) missing(name string) (any, error) {
	if value, err := d.BaseElementData.Get(name); err == nil {
		return value, nil
	}
	for _, p := range d.engine.Properties() {
		if strings.EqualFold(p.Name, name) {
			return nil, missingproperty.ForExcludedInConfig(name, DataKey)
		}
	}
	return d.BaseAspectData.Get(name)
}

// valueText gives a value the native code read as text, as the cloud
// service's response writes it.
func valueText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}
