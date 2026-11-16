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
	"sync"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

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
// Data is safe to read from several goroutines at once. Only one goroutine at
// a time reads from the native results.
//
// The native results hold memory the Go garbage collector does not manage, so
// call Free when the Data is no longer needed. A read after Free returns
// ErrDataFreed.
type Data struct {
	mu        sync.Mutex
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
	d.mu.Lock()
	defer d.mu.Unlock()
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

// Free releases the native results. It is safe to call more than once.
func (d *Data) Free() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.results != nil {
		d.results.Free()
		d.results = nil
	}
}

// ProcessData processes the given IP address and returns its Data, which
// reads each Property's values from the native results only when that
// Property is requested. The caller calls Free on the Data when it is done
// with it.
func (e *Engine) ProcessData(ipAddress string) (*Data, error) {
	return e.processInto(ipi_interop.NewResultsIpi(e.manager), ipAddress)
}

// processInto processes the address into results and wraps them in a Data,
// reading no value. It frees results when processing fails.
func (e *Engine) processInto(results nativeResults, ipAddress string) (*Data, error) {
	if err := results.ResultsIpiFromIpAddress(ipAddress); err != nil {
		results.Free()
		return nil, err
	}
	var nameAt func(int) string
	if names := e.names.Load(); names != nil {
		nameAt = names.Name
	}
	var positions map[string]int
	if p := e.positions.Load(); p != nil {
		positions = *p
	}
	return &Data{
		results:   results,
		hasValues: results.HasValues(),
		nameAt:    nameAt,
		positions: positions,
		read:      make(map[int][]*ipi_interop.WeightedValue),
	}, nil
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
