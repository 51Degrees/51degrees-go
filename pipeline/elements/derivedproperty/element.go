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

// Package derivedproperty implements the Derived Property Element, which
// produces new Property values from Property values other Flow Elements have
// already produced, following rules written in a script, and writes them into
// the Element Data under the key derived.
//
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/pipeline-elements/derived-property-element.md
package derivedproperty

import (
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core/typedmap"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// DataKey is the Element Data key every Derived Property Element in a
// Pipeline writes under.
const DataKey = "derived"

// Data is the Element Data of the Derived Property Elements, holding one
// engines.AspectPropertyValue per script, keyed by the script's output
// Property name. Several elements write it, so it is safe for concurrent use.
type Data struct {
	values typedmap.Map
}

func newData() *Data { return &Data{values: typedmap.NewConcurrentMap()} }

// Get returns a derived Property value by name.
func (d *Data) Get(name string) (any, error) {
	v, ok := d.values.GetString(name)
	if !ok {
		return nil, core.NewDataError("derived property '" + name + "' is not present")
	}
	return v, nil
}

// Set stores a derived Property value.
func (d *Data) Set(name string, value any) { d.values.AddString(name, value) }

// AsMap returns every derived Property value.
func (d *Data) AsMap() map[string]any { return d.values.AsMap() }

// GetAs returns a derived Property value as T, which is
// engines.AspectPropertyValue of the script's value type: string, bool, int
// or float64 for double.
func GetAs[T any](d *Data, name string) (T, error) {
	var zero T
	raw, err := d.Get(name)
	if err != nil {
		return zero, err
	}
	v, ok := raw.(T)
	if !ok {
		return zero, core.NewDataError("derived property '" + name + "' is not of the requested type")
	}
	return v, nil
}

// Element is a Derived Property Element. Everything it holds is fixed when it
// is built, so concurrent requests need nothing further.
type Element struct {
	*core.BaseElement
	scripts  []*compiledScript
	reads    []sourceRead
	keys     []string
	metadata []ScriptMetaData
}

// compiledScript is a script with each of its source properties resolved to
// one of the element's reads.
type compiledScript struct {
	model *scriptModel
	reads []int
	// logged records, without a lock, that the error for a request with no
	// Element Data under a key the script reads has been logged.
	logged atomic.Bool
}

// sourceRead is one source property, read once per request as one type.
type sourceRead struct {
	property sourceProperty
	// key is the index of the Element Data key in Element.keys, or -1 for a
	// derived property, which is read when the script that names it runs.
	key int
}

// Scripts returns the metadata of every script the element holds.
func (e *Element) Scripts() []ScriptMetaData {
	out := make([]ScriptMetaData, len(e.metadata))
	copy(out, e.metadata)
	return out
}

// Process evaluates every script and writes one value, or one value with no
// value carrying a message, per script.
func (e *Element) Process(fd core.FlowData) error {
	stored := fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData { return newData() })
	out, ok := stored.(*Data)
	if !ok {
		return fmt.Errorf("the element data under '%s' is %T, not the Derived Property Element's", DataKey, stored)
	}
	sources := make([]core.ElementData, len(e.keys))
	found := make([]bool, len(e.keys))
	for i, key := range e.keys {
		data, err := fd.Get(key)
		sources[i], found[i] = data, err == nil
	}
	slots := make([]slot, len(e.reads))
	for i, r := range e.reads {
		if r.key >= 0 {
			slots[i] = readSource(sources[r.key], found[r.key], r.property)
		}
	}
	for _, s := range e.scripts {
		e.processScript(s, slots, found, out)
	}
	return nil
}

func (e *Element) processScript(s *compiledScript, slots []slot, found []bool, out *Data) {
	scriptSlots := make([]slot, len(s.reads))
	var missing []sourceProperty
	var reasons []string
	var noData *sourceRead
	for i, index := range s.reads {
		r := &e.reads[index]
		if r.key < 0 {
			scriptSlots[i] = readSource(out, true, r.property)
		} else {
			scriptSlots[i] = slots[index]
			if !found[r.key] && noData == nil {
				noData = r
			}
		}
		if !scriptSlots[i].available {
			missing = append(missing, s.model.properties[i])
			reasons = append(reasons, scriptSlots[i].reason)
		}
	}
	output := s.model.output
	if len(missing) > 0 {
		if noData != nil && s.logged.CompareAndSwap(false, true) {
			key := noData.property.elementKey()
			e.Logger().Error(fmt.Sprintf("Derived property script '%s' reads '%s', and a request carried no "+
				"element data under '%s'. Add this element to the pipeline after the element that supplies "+
				"'%s'. This error is reported once for this script.", s.model.name, noData.property.name, key, key))
		}
		out.Set(output.name, noValue(output.valueType, missingMessage(output.name, missing, reasons)))
		return
	}
	out.Set(output.name, valueOf(output.valueType, s.model.evaluate(scriptSlots)))
}

func valueOf(t valueType, l literal) any {
	switch t {
	case typeBool:
		return engines.NewAspectPropertyValue(l.b)
	case typeInt:
		return engines.NewAspectPropertyValue(int(l.num))
	case typeDouble:
		return engines.NewAspectPropertyValue(l.num)
	}
	return engines.NewAspectPropertyValue(l.str)
}

func noValue(t valueType, message string) any {
	switch t {
	case typeBool:
		return engines.NoValue[bool](message)
	case typeInt:
		return engines.NoValue[int](message)
	case typeDouble:
		return engines.NoValue[float64](message)
	}
	return engines.NoValue[string](message)
}

// goType is the Go type of a script's value, inside the
// engines.AspectPropertyValue the element writes.
func goType(t valueType) reflect.Type {
	switch t {
	case typeBool:
		return reflect.TypeOf(engines.AspectPropertyValue[bool]{})
	case typeInt:
		return reflect.TypeOf(engines.AspectPropertyValue[int]{})
	case typeDouble:
		return reflect.TypeOf(engines.AspectPropertyValue[float64]{})
	}
	return reflect.TypeOf(engines.AspectPropertyValue[string]{})
}

// childLister is a group of elements that run in parallel.
type childLister interface {
	Children() []core.FlowElement
}

// OnPipelineAdded checks that some element in the Pipeline supplies every
// source property a script names, and that no two Derived Property Elements
// produce the same Property. Where an element sits is not judged, because a
// Pipeline lists the members of a parallel group after the elements at the
// top level, so a position does not say what ran first.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/pipeline-elements/derived-property-element.md#the-pipeline-check
func (e *Element) OnPipelineAdded(p core.Pipeline) error {
	elements := flatten(p.FlowElements())
	var problems []string
	for _, s := range e.scripts {
		for _, sp := range s.model.properties {
			if problem := e.checkSupplied(elements, s.model.name, sp); problem != "" {
				problems = append(problems, problem)
			}
		}
	}
	// Each name counts the distinct Derived Property Elements producing it.
	producers := map[string]int{}
	counted := map[*Element]bool{}
	for _, element := range elements {
		other, ok := element.(*Element)
		if !ok || counted[other] {
			continue
		}
		counted[other] = true
		for _, s := range other.scripts {
			producers[strings.ToLower(s.model.output.name)]++
		}
	}
	for _, s := range e.scripts {
		if producers[strings.ToLower(s.model.output.name)] > 1 {
			problems = append(problems, fmt.Sprintf("two Derived Property Elements in the pipeline produce "+
				"the derived property '%s'", s.model.output.name))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, ". "))
	}
	return e.BaseElement.OnPipelineAdded(p)
}

// checkSupplied returns why no element supplies a source property, or empty
// where one does. An element whose Properties are not known yet, such as a
// cloud engine that could not reach the service, is taken to supply the
// properties under its key, with a warning, so that a passing failure does not
// stop the Pipeline being built.
func (e *Element) checkSupplied(elements []core.FlowElement, script string, sp sourceProperty) string {
	key, name := sp.elementKey(), sp.propertyName()
	keyFound := false
	for _, element := range elements {
		if !strings.EqualFold(element.DataKey(), key) {
			continue
		}
		keyFound = true
		for _, metadata := range element.Properties() {
			if strings.EqualFold(metadata.Name, name) {
				return ""
			}
		}
		if loader, ok := element.(core.PropertiesLoader); ok && !loader.PropertiesLoaded() {
			e.Logger().Warn(fmt.Sprintf("Derived property script '%s' reads '%s', and the element under "+
				"'%s' has not loaded its properties yet, so whether it supplies '%s' cannot be checked.",
				script, sp.name, key, name))
			return ""
		}
	}
	if !keyFound {
		return fmt.Sprintf("derived property script '%s' reads '%s', and no element in the pipeline has "+
			"the element data key '%s'", script, sp.name, key)
	}
	return fmt.Sprintf("derived property script '%s' reads '%s', and no element under '%s' lists the "+
		"property '%s'", script, sp.name, key, name)
}

// flatten lists the elements of a Pipeline with the members of every parallel
// group.
func flatten(elements []core.FlowElement) []core.FlowElement {
	var out []core.FlowElement
	for _, element := range elements {
		out = append(out, element)
		if group, ok := element.(childLister); ok {
			out = append(out, flatten(group.Children())...)
		}
	}
	return out
}
