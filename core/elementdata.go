package core

import "github.com/51Degrees/pipeline-go/core/typedmap"

// ElementData is a container, within Flow Data, for the Property values relating
// to a particular Flow Element. Values are set by the Element during processing
// and retrieved using the string key of the Property.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/conceptual-overview.md#element-data
type ElementData interface {
	// Get returns the value of a Property by its (case-insensitive) string name.
	// It returns an error if the Property is not present.
	Get(propertyName string) (any, error)
	// Set stores the value of a Property.
	Set(propertyName string, value any)
	// AsMap returns all Property values keyed by normalized (lower-case) name.
	AsMap() map[string]any
}

// BaseElementData is the default ElementData implementation that concrete
// Element Data types embed. It stores Property values in a typed map.
type BaseElementData struct {
	values typedmap.Map
}

// NewBaseElementData creates a BaseElementData. Element Data is accessed by a
// single thread per the thread-safety specification, so a non-concurrent map is
// used.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/thread-safety.md
func NewBaseElementData() *BaseElementData {
	return &BaseElementData{values: typedmap.NewMap()}
}

// Get returns the value of a Property by name, or an error if absent.
func (d *BaseElementData) Get(propertyName string) (any, error) {
	v, ok := d.values.GetString(propertyName)
	if !ok {
		return nil, NewDataError("property '" + propertyName + "' is not present in this element data")
	}
	return v, nil
}

// Set stores a Property value.
func (d *BaseElementData) Set(propertyName string, value any) {
	d.values.AddString(propertyName, value)
}

// AsMap returns a copy of all Property values.
func (d *BaseElementData) AsMap() map[string]any { return d.values.AsMap() }

// GetPropertyAs retrieves a Property value from Element Data and converts it to
// T. It is the strongly-typed accessor analogue described in access-to-results.
func GetPropertyAs[T any](d ElementData, propertyName string) (T, error) {
	var zero T
	raw, err := d.Get(propertyName)
	if err != nil {
		return zero, err
	}
	v, ok := raw.(T)
	if !ok {
		return zero, NewDataError("property '" + propertyName + "' is not of the requested type")
	}
	return v, nil
}
