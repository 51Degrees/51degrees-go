package core

import "reflect"

// PropertyMetadata describes a Property that a Flow Element can populate.
// All Flow Elements MUST expose this metadata, and a Pipeline MUST expose the
// aggregated metadata for all its elements.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/properties.md#property-metadata
type PropertyMetadata struct {
	// Name is the string name used to access the Property value in Element Data.
	Name string
	// Element is the Flow Element that populates this Property.
	Element FlowElement
	// Category is the name of the category the Property belongs to. Used to help
	// organize Properties for elements that populate large numbers of them.
	Category string
	// Type is the type of value returned by the Property. May be nil if unknown.
	Type reflect.Type
	// Available reports whether the Property is currently available to the
	// caller. If false, the value will not be present in Element Data.
	Available bool
	// DelayExecution controls execution of JavaScript Properties. See web integration.
	DelayExecution bool
	// EvidenceProperties lists the names of any JavaScript Properties that can be
	// executed to help determine the value of this Property.
	EvidenceProperties []string
	// ItemProperties holds metadata for the Properties of complex object values
	// (for example, the Hardware profile lookup Engine).
	ItemProperties []PropertyMetadata
}

// ItemPropertyDictionary returns ItemProperties keyed on (lower-case) Property
// name, matching the "Item Property Dictionary" metadata field.
func (p PropertyMetadata) ItemPropertyDictionary() map[string]PropertyMetadata {
	if len(p.ItemProperties) == 0 {
		return nil
	}
	out := make(map[string]PropertyMetadata, len(p.ItemProperties))
	for _, ip := range p.ItemProperties {
		out[lower(ip.Name)] = ip
	}
	return out
}
