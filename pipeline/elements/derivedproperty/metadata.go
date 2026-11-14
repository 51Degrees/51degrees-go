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

package derivedproperty

// ScriptMetaData describes one script an Element holds. Its Output is the
// whole definition of the derived property, so a JSON builder, the cloud or a
// documentation generator can read every field.
type ScriptMetaData struct {
	Name            string
	Version         string
	Format          int
	Deprecated      bool
	DeprecationNote string
	// Source is the built-in script's name, the file path, or "code".
	Source string
	Output OutputMetaData
}

// OutputMetaData is the Output block of a script, with the field names
// 51Degrees uses for the metadata of every property. A pointer is nil where
// the script does not give the field.
type OutputMetaData struct {
	Name        string
	Description string
	// ValueType is string, bool, int or double.
	ValueType       string
	StoredValueType *string
	// DefaultValue is metadata only. Nothing reads it while a request is
	// processed.
	DefaultValue *string
	IsList       bool
	IsMandatory  *bool
	IsObsolete   *bool
	Category     *string
	IsPopular    *bool
	ExportValues *bool
	Url          *string
	DisplayOrder *int64
	PropertyId   *int64
	VendorIds    []any
	// Dependencies is the list the script gives, or every source property its
	// checks and rules name where it gives none.
	Dependencies []any
	// Values lists the values the property can have, or is nil where the
	// script gives no list.
	Values []ValueMetaData
}

// ValueMetaData is one entry of Output.Values.
type ValueMetaData struct {
	// Name is the string form of the value, as a rule's value is compared.
	Name        string
	Description *string
}

func (m *scriptModel) metadata() ScriptMetaData {
	o := m.output
	out := OutputMetaData{
		Name:            o.name,
		Description:     o.description,
		ValueType:       string(o.valueType),
		StoredValueType: o.storedValueType,
		DefaultValue:    o.defaultValue,
		IsList:          o.isList,
		IsMandatory:     o.isMandatory,
		IsObsolete:      o.isObsolete,
		Category:        o.category,
		IsPopular:       o.isPopular,
		ExportValues:    o.exportValues,
		Url:             o.url,
		DisplayOrder:    o.displayOrder,
		PropertyId:      o.propertyID,
	}
	if o.vendorIDs != nil {
		out.VendorIds = plainValue(o.vendorIDs).([]any)
	}
	if o.dependencies != nil {
		out.Dependencies = plainValue(o.dependencies).([]any)
	} else {
		out.Dependencies = make([]any, len(o.computed))
		for i, d := range o.computed {
			out.Dependencies[i] = d
		}
	}
	if o.hasValues {
		out.Values = make([]ValueMetaData, len(o.values))
		for i, v := range o.values {
			out.Values[i] = ValueMetaData{Name: v.name.String(), Description: v.description}
		}
	}
	return ScriptMetaData{
		Name:            m.name,
		Version:         m.version,
		Format:          m.format,
		Deprecated:      m.deprecated,
		DeprecationNote: m.deprecationNote,
		Source:          m.source,
		Output:          out,
	}
}

// plainValue gives a value from a script as Go values: nil, a bool, a
// float64, a string, a list or a map.
func plainValue(n *node) any {
	switch n.kind {
	case boolNode:
		return n.b
	case numberNode:
		return n.num
	case stringNode:
		return n.str
	case listNode:
		items := make([]any, len(n.items))
		for i, item := range n.items {
			items[i] = plainValue(item)
		}
		return items
	case mapNode:
		values := make(map[string]any, len(n.keys))
		for i, key := range n.keys {
			values[key] = plainValue(n.values[i])
		}
		return values
	}
	return nil
}
