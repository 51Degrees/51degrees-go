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

import "strings"

// The canonical form follows tools/canonical.mjs in the derived-properties
// repository, so the debug log shows the model a script became in the same
// text in every language, and a YAML script and the JSON that mirrors it
// print the same.
// See https://github.com/51Degrees/derived-properties/blob/cc30638632050795d9acc80d5118a290ffaba085/docs/testing.md#the-tester-page

// object is a JSON object that keeps its keys in the order they are set.
type object struct {
	keys   []string
	values []any
}

func (o *object) set(key string, value any) {
	o.keys = append(o.keys, key)
	o.values = append(o.values, value)
}

// rawJSON is JSON text that is written as it is.
type rawJSON string

// canonical prints a model as canonical JSON, without a trailing newline.
func (m *scriptModel) canonical() string {
	var b strings.Builder
	writeJSON(&b, m.canonicalObject(), 0)
	return b.String()
}

func (m *scriptModel) canonicalObject() *object {
	result := &object{}
	result.set("Format", rawJSON("1"))
	result.set("Name", m.name)
	result.set("Version", m.version)
	if m.deprecated {
		result.set("Deprecated", true)
		result.set("DeprecationNote", m.deprecationNote)
	}
	result.set("Output", m.output.canonicalObject())

	// The inferred type of every source property, which the file does not
	// carry because the literals compared with it give it.
	properties := &object{}
	for _, p := range m.properties {
		t := &object{}
		t.set("Type", string(p.typ))
		properties.set(p.name, t)
	}
	result.set("Properties", properties)

	if len(m.checks) > 0 {
		checks := &object{}
		for _, c := range m.checks {
			checks.set(c.name, canonicalCondition(c.condition))
		}
		result.set("Checks", checks)
	}
	rules := make([]any, len(m.rules))
	for i, r := range m.rules {
		o := &object{}
		if r.when == nil {
			o.set("Else", literalJSON(r.value))
		} else {
			o.set("When", canonicalCondition(r.when))
			o.set("Then", literalJSON(r.value))
		}
		rules[i] = o
	}
	result.set("Rules", rules)
	return result
}

func (o *outputModel) canonicalObject() *object {
	result := &object{}
	result.set("Name", o.name)
	if o.description != "" {
		result.set("Description", o.description)
	}
	if o.valueType != "" {
		result.set("ValueType", string(o.valueType))
	}
	setString(result, "StoredValueType", o.storedValueType)
	setString(result, "DefaultValue", o.defaultValue)
	result.set("IsList", o.isList)
	setBool(result, "IsMandatory", o.isMandatory)
	setBool(result, "IsObsolete", o.isObsolete)
	setString(result, "Category", o.category)
	setBool(result, "IsPopular", o.isPopular)
	setBool(result, "ExportValues", o.exportValues)
	setString(result, "Url", o.url)
	setInt(result, "DisplayOrder", o.displayOrder)
	setInt(result, "PropertyId", o.propertyID)
	if o.vendorIDs != nil {
		result.set("VendorIds", nodeJSON(o.vendorIDs))
	}
	if o.dependencies != nil {
		result.set("Dependencies", nodeJSON(o.dependencies))
	} else {
		dependencies := make([]any, len(o.computed))
		for i, d := range o.computed {
			dependencies[i] = d
		}
		result.set("Dependencies", dependencies)
	}
	if o.hasValues {
		values := make([]any, len(o.values))
		for i, v := range o.values {
			entry := &object{}
			entry.set("Name", literalJSON(v.name))
			if v.description != nil {
				entry.set("Description", *v.description)
			}
			values[i] = entry
		}
		result.set("Values", values)
	}
	return result
}

func setString(o *object, key string, value *string) {
	if value != nil {
		o.set(key, *value)
	}
}

func setBool(o *object, key string, value *bool) {
	if value != nil {
		o.set(key, *value)
	}
}

func setInt(o *object, key string, value *int64) {
	if value != nil {
		o.set(key, rawJSON(jsNumber(float64(*value))))
	}
}

func canonicalCondition(c *condition) *object {
	result := &object{}
	switch c.kind {
	case compareCondition:
		result.set("Property", c.property)
		result.set(c.op, nodeJSON(c.operand))
	case checkCondition:
		result.set("Check", c.check)
	case aggregateCondition:
		if c.group == nil {
			result.set(c.agg, "Checks")
		} else {
			names := make([]any, len(c.groupOf))
			for i, name := range c.groupOf {
				names[i] = name
			}
			result.set(c.agg, names)
		}
		result.set(c.op, rawJSON(jsNumber(float64(c.count))))
	case allCondition, anyCondition:
		items := make([]any, len(c.items))
		for i, item := range c.items {
			items[i] = canonicalCondition(item)
		}
		if c.kind == allCondition {
			result.set("All", items)
		} else {
			result.set("Any", items)
		}
	case notCondition:
		result.set("Not", canonicalCondition(c.items[0]))
	}
	return result
}

func literalJSON(l literal) any {
	switch l.kind {
	case boolNode:
		return l.b
	case numberNode:
		return rawJSON(jsNumber(l.num))
	}
	return l.str
}

// nodeJSON gives a value from the script with its types as written.
func nodeJSON(n *node) any {
	switch n.kind {
	case nullNode, boolNode, numberNode:
		return rawJSON(compactJSON(n))
	case stringNode:
		return n.str
	case listNode:
		items := make([]any, len(n.items))
		for i, item := range n.items {
			items[i] = nodeJSON(item)
		}
		return items
	}
	result := &object{}
	for i, key := range n.keys {
		result.set(key, nodeJSON(n.values[i]))
	}
	return result
}

// writeJSON writes a value as JSON.stringify does with an indent of two.
func writeJSON(b *strings.Builder, value any, depth int) {
	newline := func(d int) {
		b.WriteByte('\n')
		b.WriteString(strings.Repeat("  ", d))
	}
	switch v := value.(type) {
	case rawJSON:
		b.WriteString(string(v))
	case string:
		b.Write(jsonString(v))
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case []any:
		if len(v) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			newline(depth + 1)
			writeJSON(b, item, depth+1)
		}
		newline(depth)
		b.WriteByte(']')
	case *object:
		if len(v.keys) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, key := range v.keys {
			if i > 0 {
				b.WriteByte(',')
			}
			newline(depth + 1)
			b.Write(jsonString(key))
			b.WriteString(": ")
			writeJSON(b, v.values[i], depth+1)
		}
		newline(depth)
		b.WriteByte('}')
	}
}
