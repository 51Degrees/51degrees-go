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
	"fmt"
	"math"
	"strings"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// noValueMessage says why a Property of the Engine has no value.
const noValueMessage = "The IP address was not found in the data file, or " +
	"the data file has no value of this Property for it."

// ErrDataFreed is returned by a read from a Data after its Free.
var ErrDataFreed = errors.New("the IP Intelligence data has been freed")

// nativeResults is the part of ipi_interop.ResultsIpi that Data uses.
type nativeResults interface {
	ResultsIpiFromIpAddress(ipAddress string) error
	HasValues() bool
	GetWeightedValuesByIndexes(indexes []int, propertyNameResolver func(int) string) (ipi_interop.Values, error)
	Free()
}

// Data is the result of processing one IP address. It holds the native
// results, and reads a Property's values from them only when that Property
// is requested, keeping what it has read so a second request does not call
// the native code again. Processing reads no value.
//
// Data is Aspect Data (engines.AspectData), as the Data of the IP Intelligence
// Cloud Engine is, whether the Engine is used in a Pipeline or on its own.
// Get gives each Property of the Engine as its weighted values.
//
// Data is not safe to use from several goroutines at once, because reading a
// value writes into the native results and into the values Data keeps, and
// Data takes no lock. The caller is responsible for thread-safe access to one
// it shares, for example with a sync.Mutex of its own.
//
// The native results hold memory the Go garbage collector does not manage, so
// call Free when the Data is no longer needed. A read after Free returns
// ErrDataFreed.
//
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/data-model.md?plain=1#L10-L12
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/data-model.md?plain=1#L34-L37
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L520-L525
type Data struct {
	*engines.BaseAspectData
	results   nativeResults
	hasValues bool
	nameAt    func(int) string
	positions map[string]int
	read      map[int][]*ipi_interop.WeightedValue
}

// HasValues reports whether the address was found in the data file, so that
// the Data has values to read.
func (d *Data) HasValues() bool {
	return d.hasValues
}

// Values returns the values of the named Property, reading them from the
// native results the first time the Property is requested. The name is one
// of the Properties the Engine was created with. The Property's position is
// found from the table of names the Engine made when it was created.
func (d *Data) Values(property string) ([]*ipi_interop.WeightedValue, error) {
	index, ok := d.positions[property]
	if !ok {
		return nil, fmt.Errorf(
			"the Property '%s' is not one the Engine was created with", property)
	}
	return d.ValuesByIndex(index)
}

// ValuesByIndex returns the values of the Property at the given position in
// the Properties the Engine was created with, reading them from the native
// results the first time the Property is requested.
func (d *Data) ValuesByIndex(index int) ([]*ipi_interop.WeightedValue, error) {
	if d.results == nil {
		return nil, ErrDataFreed
	}
	if values, ok := d.read[index]; ok {
		return values, nil
	}
	var values []*ipi_interop.WeightedValue
	if d.hasValues {
		read, err := d.results.GetWeightedValuesByIndexes([]int{index}, d.nameAt)
		if err != nil {
			return nil, err
		}
		// Only the one Property was asked for, so the map holds at most one
		// entry, whatever name it was given.
		for _, v := range read {
			values = v
		}
	}
	if values == nil {
		values = []*ipi_interop.WeightedValue{}
	}
	d.read[index] = values
	return values, nil
}

// Get returns the value of a Property by its name, in any case. A Property of
// the Engine is read from the native results when it is requested, as Values
// reads it, and is given as an
// engines.AspectPropertyValue[[]core.WeightedValue[any]], which has no value,
// with a message, where the address has none. Each weighted value holds the
// raw weighting of its weight, out of core.MaxRawWeighting. Any other name is
// looked up in the values Set has stored, and gives an error when it is not
// there. A Property of the Engine read after Free gives ErrDataFreed.
func (d *Data) Get(propertyName string) (any, error) {
	index, ok := d.position(propertyName)
	if !ok {
		return d.BaseAspectData.Get(propertyName)
	}
	values, err := d.ValuesByIndex(index)
	if err != nil {
		return nil, err
	}
	return aspectValues(values), nil
}

// AsMap returns the values Set has stored with every Property of the Engine,
// each under its name in lower case as Get gives it, reading from the native
// results any Property not yet read. After Free, the Properties of the Engine
// not yet read are left out.
func (d *Data) AsMap() map[string]any {
	m := d.BaseAspectData.AsMap()
	if m == nil {
		m = make(map[string]any, len(d.positions))
	}
	for name, index := range d.positions {
		values, err := d.ValuesByIndex(index)
		if err != nil {
			continue
		}
		m[strings.ToLower(name)] = aspectValues(values)
	}
	return m
}

// position finds the position of the named Property of the Engine, matching
// the name in any case.
func (d *Data) position(name string) (int, bool) {
	if index, ok := d.positions[name]; ok {
		return index, true
	}
	for n, index := range d.positions {
		if strings.EqualFold(n, name) {
			return index, true
		}
	}
	return 0, false
}

// aspectValues gives values as the weighted values of an Aspect Data
// Property, with no value where there are none.
func aspectValues(
	values []*ipi_interop.WeightedValue) engines.AspectPropertyValue[[]core.WeightedValue[any]] {
	if len(values) == 0 {
		return engines.NoValue[[]core.WeightedValue[any]](noValueMessage)
	}
	weighted := make([]core.WeightedValue[any], len(values))
	for i, v := range values {
		weighted[i] = core.NewWeightedValue[any](v.Value, toRawWeighting(v.Weight))
	}
	return engines.NewAspectPropertyValue(weighted)
}

// toRawWeighting gives a weight from 0 to 1 as a raw weighting out of
// core.MaxRawWeighting.
func toRawWeighting(weight float64) uint16 {
	switch {
	case weight <= 0:
		return 0
	case weight >= 1:
		return core.MaxRawWeighting
	}
	return uint16(math.Round(weight * core.MaxRawWeighting))
}

// Free releases the native results. It is safe to call more than once.
func (d *Data) Free() {
	if d.results != nil {
		d.results.Free()
		d.results = nil
	}
}

// ProcessData processes the given IP address and returns its Data, which
// reads each Property's values from the native results only when that
// Property is requested. The caller calls Free on the Data when it is done
// with it. The Data is not safe to use from several goroutines at once, so
// it belongs to one goroutine at a time. The caller is responsible for
// thread-safe access to one it shares. It gives an error once the engine has
// stopped.
func (e *Engine) ProcessData(ipAddress string) (*Data, error) {
	if !e.acquire() {
		return nil, errEngineStopped
	}
	defer e.release()
	return e.processInto(ipi_interop.NewResultsIpi(e.manager), ipAddress)
}

// processInto processes the address into results and wraps them in a Data,
// reading no value. It frees results when processing fails.
func (e *Engine) processInto(results nativeResults, ipAddress string) (*Data, error) {
	if err := results.ResultsIpiFromIpAddress(ipAddress); err != nil {
		results.Free()
		return nil, err
	}
	return e.newData(results), nil
}

// newData wraps populated results in a Data, reading no value.
func (e *Engine) newData(results nativeResults) *Data {
	var nameAt func(int) string
	if names := e.names.Load(); names != nil {
		nameAt = names.Name
	}
	var positions map[string]int
	if p := e.positions.Load(); p != nil {
		positions = *p
	}
	return &Data{
		BaseAspectData: engines.NewBaseAspectData(),
		results:        results,
		hasValues:      results.HasValues(),
		nameAt:         nameAt,
		positions:      positions,
		read:           make(map[int][]*ipi_interop.WeightedValue),
	}
}

// setPositions makes the table from each required Property name to its
// position. It is made once, with the table of names, and never from a later
// data set.
func (e *Engine) setPositions(names []string) {
	positions := make(map[string]int, len(names))
	for i, name := range names {
		positions[name] = i
	}
	e.positions.Store(&positions)
}

// namesOf lists the names in the table at each position, in order.
func namesOf(names *ipi_interop.PropertyNames) []string {
	var list []string
	for i := 0; ; i++ {
		name := names.Name(i)
		if name == "" {
			return list
		}
		list = append(list, name)
	}
}
