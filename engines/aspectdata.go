// Package engines builds on the core Pipeline package to add the features that
// are common to 51Degrees Aspect Engines: result caching, data-file updates,
// missing-property reporting, lazy loading, and the null-supporting
// AspectPropertyValue type.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#engines
package engines

import (
	"sync/atomic"

	"github.com/51Degrees/pipeline-go/core"
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
}

// NewBaseAspectData creates a BaseAspectData.
func NewBaseAspectData() *BaseAspectData {
	return &BaseAspectData{BaseElementData: core.NewBaseElementData()}
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
	HasValue() bool
	NoValueMessage() string
	AsInterface() any
}
