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

// Package engines builds on the core Pipeline package to add the features that
// are common to 51Degrees Aspect Engines: result caching, data-file updates,
// missing-property reporting, lazy loading, and the null-supporting
// AspectPropertyValue type.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#engines
package engines

import (
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines/missingproperty"
)

// AspectData is a specialization of Element Data produced by an Aspect Engine.
// Beyond standard Element Data it tracks whether the value came from a cache.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#aspect-data
type AspectData interface {
	core.ElementData
	// CacheHit reports whether this data was served from an Engine cache.
	CacheHit() bool
	setCacheHit(bool)
}

// BaseAspectData is the default AspectData implementation that concrete Aspect
// Data types embed. A cached instance may be read by multiple goroutines, so
// the cache-hit flag is stored atomically.
type BaseAspectData struct {
	*core.BaseElementData
	cacheHit atomic.Bool
	// source is what a missing Property is explained from, nil until the
	// data is added to a Flow Data under the key of an Engine.
	source atomic.Pointer[dataSource]
}

// dataSource is the Engine that populates Aspect Data, and what else the
// reason for a missing Property is decided from.
type dataSource struct {
	engine missingproperty.Engine
	// data is the Aspect Data as the Flow Data holds it, whose type has the
	// typed accessors.
	data core.ElementData
	// cloudRequestFailed is true where a Cloud Aspect Engine populated the
	// data and the cloud request for its Flow Data failed.
	cloudRequestFailed bool

	accessorsOnce sync.Once
	accessors     []string
}

// cloudRequestState is implemented by a Cloud Aspect Engine, which knows
// whether the cloud request for a Flow Data failed.
type cloudRequestState interface {
	CloudRequestFailed(fd core.FlowData) bool
}

// baseAspectDataType is the type whose methods are not typed accessors.
var baseAspectDataType = reflect.TypeOf((*BaseAspectData)(nil))

// accessorNames gives the names of the typed accessors of the Aspect Data,
// being the methods its type has beyond those of BaseAspectData, counting
// the methods of the types it embeds.
func (s *dataSource) accessorNames() []string {
	s.accessorsOnce.Do(func() {
		t := reflect.TypeOf(s.data)
		for i := 0; i < t.NumMethod(); i++ {
			name := t.Method(i).Name
			if _, base := baseAspectDataType.MethodByName(name); !base {
				s.accessors = append(s.accessors, name)
			}
		}
	})
	return s.accessors
}

// NewBaseAspectData creates a BaseAspectData.
func NewBaseAspectData() *BaseAspectData {
	return &BaseAspectData{BaseElementData: core.NewBaseElementData()}
}

// AddedToFlowData keeps the Engine in the Pipeline whose Data Key is key, so
// that Get can say why a Property is missing. The first call decides, so
// Aspect Data served from a cache keeps the Engine that computed it.
func (d *BaseAspectData) AddedToFlowData(fd core.FlowData, key string, data core.ElementData) {
	if d.source.Load() != nil {
		return
	}
	engine, ok := fd.Pipeline().GetElement(key).(missingproperty.Engine)
	if !ok {
		return
	}
	s := &dataSource{engine: engine, data: data}
	if c, ok := engine.(cloudRequestState); ok {
		s.cloudRequestFailed = c.CloudRequestFailed(fd)
	}
	d.source.CompareAndSwap(nil, s)
}

// Get returns the value of a Property by name. Where the Property is absent
// and the Engine that populates the data is known, the error is a
// core.PropertyMissingError that gives the reason.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/properties.md#missing-properties
func (d *BaseAspectData) Get(propertyName string) (any, error) {
	v, err := d.BaseElementData.Get(propertyName)
	if err == nil {
		return v, nil
	}
	s := d.source.Load()
	if s == nil {
		return nil, err
	}
	return nil, missingproperty.Decide(propertyName,
		[]missingproperty.Engine{s.engine}, missingproperty.Context{
			CloudRequestFailed: s.cloudRequestFailed,
			Accessors:          s.accessorNames(),
		})
}

// CacheHit reports whether the data was served from a cache.
func (d *BaseAspectData) CacheHit() bool     { return d.cacheHit.Load() }
func (d *BaseAspectData) setCacheHit(v bool) { d.cacheHit.Store(v) }

// AspectPropertyValue wraps a Property value to support the "no value" state
// required of Engine Properties. Accessing the value when none is set returns a
// NoValueError carrying a message that explains why it is absent.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/properties.md#null-values
type AspectPropertyValue[T any] struct {
	value          T
	hasValue       bool
	noValueMessage string
}

// NewAspectPropertyValue creates a value-present AspectPropertyValue.
func NewAspectPropertyValue[T any](value T) AspectPropertyValue[T] {
	return AspectPropertyValue[T]{value: value, hasValue: true}
}

// NoValue creates a value-absent AspectPropertyValue with an explanatory message.
func NoValue[T any](message string) AspectPropertyValue[T] {
	if message == "" {
		message = "This property has no value set."
	}
	return AspectPropertyValue[T]{noValueMessage: message}
}

// HasValue reports whether a value is set.
func (a AspectPropertyValue[T]) HasValue() bool { return a.hasValue }

// Value returns the wrapped value, or a NoValueError if none is set.
func (a AspectPropertyValue[T]) Value() (T, error) {
	if !a.hasValue {
		var zero T
		return zero, core.NewNoValueError(a.noValueMessage, nil)
	}
	return a.value, nil
}

// ValueOrDefault returns the wrapped value if set, otherwise the supplied default.
func (a AspectPropertyValue[T]) ValueOrDefault(def T) T {
	if a.hasValue {
		return a.value
	}
	return def
}

// NoValueMessage returns the message explaining why no value is set.
func (a AspectPropertyValue[T]) NoValueMessage() string { return a.noValueMessage }

// AsInterface returns the wrapped value as an untyped any (nil if no value is
// set). It allows non-generic code, such as the JSON Builder, to inspect the
// value without knowing T.
func (a AspectPropertyValue[T]) AsInterface() any {
	if a.hasValue {
		return a.value
	}
	return nil
}

// PropertyValueAccessor is the non-generic view of an AspectPropertyValue,
// allowing code that does not know the value type to test for "no value", read
// the explanatory message, and obtain the boxed value.
type PropertyValueAccessor interface {
	// HasValue reports whether a value is present.
	HasValue() bool
	// NoValueMessage returns the explanation for why no value is present. It is
	// empty when HasValue is true.
	NoValueMessage() string
	// AsInterface returns the wrapped value as an untyped any, or nil if no value
	// is present.
	AsInterface() any
}
