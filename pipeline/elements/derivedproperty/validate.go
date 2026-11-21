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

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// The validator follows the reference implementation, tools/validate.mjs in
// the derived-properties repository, with the same paths and messages, so a
// fault reads the same in every language.
// See https://github.com/51Degrees/derived-properties/blob/cc30638632050795d9acc80d5118a290ffaba085/docs/format-1.md

// Fault is one thing wrong with a script.
type Fault struct {
	// Script is the name the script was given, or empty where it had none.
	Script string
	// Source is the built-in script's name, the file path, or "code".
	Source string
	// Path is the place in the document, such as Rules[3].When.All[1], or
	// empty for the document as a whole.
	Path string
	// Line is the one based line, or 0 where the parser gave none.
	Line int
	// Message says what is wrong.
	Message string
}

// String gives the fault as one line.
func (f Fault) String() string {
	script := f.Script
	if script == "" {
		script = "script"
	}
	where := f.Path
	if where == "" {
		where = "(document)"
	}
	line := ""
	if f.Line > 0 {
		line = fmt.Sprintf(" line %d", f.Line)
	}
	return fmt.Sprintf("%s (%s)%s at %s: %s", script, f.Source, line, where, f.Message)
}

// ValidationError is every fault found in the scripts given to a Builder.
type ValidationError struct {
	Faults []Fault
}

func (e *ValidationError) Error() string {
	lines := make([]string, len(e.Faults))
	for i, f := range e.Faults {
		lines[i] = f.String()
	}
	return strings.Join(lines, "\n")
}

var (
	valueTypes     = []string{"string", "bool", "int", "double"}
	operatorNames  = []string{"Eq", "Ne", "Gt", "Ge", "Lt", "Le", "In", "NotIn", "StartsWith", "EndsWith", "Contains"}
	operatorTypes  = map[string][]valueType{}
	aggregateNames = []string{"Passed", "Failed"}
	topLevelKeys   = []string{"Format", "Name", "Version", "Deprecated", "DeprecationNote", "Output", "Checks", "Rules"}
	// outputKeys are in the order the canonical form prints them.
	outputKeys = []string{"Name", "Description", "ValueType", "StoredValueType", "DefaultValue", "IsList",
		"IsMandatory", "IsObsolete", "Category", "IsPopular", "ExportValues", "Url", "DisplayOrder",
		"PropertyId", "VendorIds", "Dependencies", "Values"}

	identifier      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)
	sourcePattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*\.[A-Za-z][A-Za-z0-9]*$`)
	semanticVersion = regexp.MustCompile(
		`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+(?:\.[0-9A-Za-z.-]+)*)?(?:\+[0-9A-Za-z.-]+)?$`)
)

func init() {
	all := []valueType{typeBool, typeInt, typeDouble, typeString}
	numbers := []valueType{typeInt, typeDouble}
	for _, op := range []string{"Eq", "Ne", "In", "NotIn"} {
		operatorTypes[op] = all
	}
	for _, op := range []string{"Gt", "Ge", "Lt", "Le"} {
		operatorTypes[op] = numbers
	}
	for _, op := range []string{"StartsWith", "EndsWith", "Contains"} {
		operatorTypes[op] = []valueType{typeString}
	}
}

// validator collects the faults of one script while building its model.
type validator struct {
	document *node
	script   string
	source   string
	faults   []Fault
	// properties holds where each source property was first seen and the type
	// inferred there, so a conflict can name both places.
	properties map[string]*propertyUse
}

type propertyUse struct {
	slot     int
	typ      valueType
	typePath string
}

func (v *validator) fault(path string, at *node, message string) {
	line := 0
	if at != nil && (at.kind == mapNode || at.kind == listNode) {
		line = at.line
	}
	v.faults = append(v.faults, Fault{Script: v.script, Source: v.source, Path: path, Line: line, Message: message})
}

// validateText parses and validates the text of a script. name is the file
// name without its extension, which the script's Name must equal, or empty
// where there is none. source says where the script came from.
func validateText(text, name, source string) (*scriptModel, []Fault) {
	document, err := parseScript(text)
	if err != nil {
		line := 0
		if pe, ok := err.(*parseError); ok {
			line = pe.line
		}
		return nil, []Fault{{Script: name, Source: source, Line: line, Message: err.Error()}}
	}
	v := &validator{document: document, script: name, source: source, properties: map[string]*propertyUse{}}
	model := v.buildModel()
	if len(v.faults) > 0 {
		return nil, v.faults
	}
	return model, nil
}

func (v *validator) buildModel() *scriptModel {
	for _, key := range unknownKeys(v.document, topLevelKeys) {
		v.fault(key, v.document, fmt.Sprintf("unknown key '%s' at the top level. Expected one of %s",
			key, strings.Join(topLevelKeys, ", ")))
	}
	m := &scriptModel{source: v.source}
	m.format = v.readFormat()
	m.name = v.readName()
	m.version = v.readVersion()
	v.readDeprecation(m)
	m.output = v.readOutput()
	// Checks are read before rules, so that a rule can refer to one.
	checkNames := v.readCheckNames()
	v.readChecks(m, checkNames)
	v.readRules(m, checkNames)
	if m.output.dependencies == nil {
		for _, p := range m.properties {
			m.output.computed = append(m.output.computed, p.name)
		}
	}
	return m
}

func (v *validator) readFormat() int {
	value := v.document.get("Format")
	if value == nil {
		v.fault("Format", v.document, "required key 'Format' is missing")
		return 0
	}
	if value.kind != numberNode || value.num != 1 {
		v.fault("Format", v.document, "Format must be 1, found "+jsonOf(value))
		return 0
	}
	return 1
}

func (v *validator) readName() string {
	value := v.document.get("Name")
	if value == nil {
		v.fault("Name", v.document, "required key 'Name' is missing")
		return ""
	}
	if value.kind != stringNode {
		v.fault("Name", v.document, "Name expected a string, found "+typeOf(value))
		return ""
	}
	if !identifier.MatchString(value.str) {
		v.fault("Name", v.document, fmt.Sprintf("script name '%s' does not match the pattern %s",
			value.str, identifier.String()))
		return value.str
	}
	if v.script != "" && v.script != value.str {
		v.fault("Name", v.document, fmt.Sprintf("script name '%s' must equal the file name '%s'",
			value.str, v.script))
	}
	return value.str
}

func (v *validator) readVersion() string {
	value := v.document.get("Version")
	if value == nil {
		v.fault("Version", v.document, "required key 'Version' is missing")
		return ""
	}
	if value.kind != stringNode || !semanticVersion.MatchString(value.str) {
		v.fault("Version", v.document,
			"Version expected a semantic version such as 1.0.0, found "+jsonOf(value))
		return ""
	}
	return value.str
}

func (v *validator) readDeprecation(m *scriptModel) {
	if deprecated := v.document.get("Deprecated"); deprecated != nil {
		if deprecated.kind != boolNode {
			v.fault("Deprecated", v.document, "Deprecated expected a boolean, found "+typeOf(deprecated))
		} else {
			m.deprecated = deprecated.b
		}
	}
	note := v.document.get("DeprecationNote")
	if note != nil && note.kind != stringNode {
		v.fault("DeprecationNote", v.document, "DeprecationNote expected a string, found "+typeOf(note))
	} else if note != nil {
		m.deprecationNote = note.str
	}
	if m.deprecated && m.deprecationNote == "" {
		v.fault("DeprecationNote", v.document,
			"a deprecated script must say what to use instead in DeprecationNote")
	}
	if !m.deprecated && m.deprecationNote != "" {
		v.fault("DeprecationNote", v.document, "DeprecationNote is only allowed when Deprecated is true")
	}
}

func (v *validator) readOutput() *outputModel {
	output := &outputModel{}
	fallbackName := v.script
	if fallbackName == "" {
		fallbackName = "unknown"
	}
	raw := v.document.get("Output")
	if raw == nil {
		v.fault("Output", v.document, "required key 'Output' is missing")
		output.name = fallbackName
		return output
	}
	if raw.kind != mapNode {
		v.fault("Output", v.document, "Output expected a mapping, found "+typeOf(raw))
		output.name = fallbackName
		return output
	}
	for _, key := range unknownKeys(raw, outputKeys) {
		v.fault("Output."+key, raw, fmt.Sprintf("unknown key '%s' under Output. A typo in a metadata "+
			"field is a fault rather than a value that is quietly dropped", key))
	}

	if name := raw.get("Name"); name == nil {
		v.fault("Output.Name", raw, "required key 'Name' is missing")
	} else if name.kind != stringNode || !identifier.MatchString(name.str) {
		v.fault("Output.Name", raw, fmt.Sprintf("Output.Name %s does not match the pattern %s",
			jsonOf(name), identifier.String()))
	} else {
		output.name = name.str
	}
	if output.name == "" {
		output.name = fallbackName
	}

	if description := raw.get("Description"); description == nil {
		v.fault("Output.Description", raw, "required key 'Description' is missing. Say what the "+
			"property asserts, not how far to trust it")
	} else if description.kind != stringNode || strings.TrimSpace(description.str) == "" {
		v.fault("Output.Description", raw, "Output.Description expected a non empty string, found "+
			typeOf(description))
	} else {
		output.description = description.str
	}

	if vt := raw.get("ValueType"); vt == nil {
		v.fault("Output.ValueType", raw, "required key 'ValueType' is missing")
	} else if vt.kind != stringNode || !contains(valueTypes, strings.ToLower(vt.str)) {
		v.fault("Output.ValueType", raw, fmt.Sprintf("Output.ValueType '%s' is not allowed in format 1. "+
			"Expected one of %s", displayOf(vt), strings.Join(valueTypes, ", ")))
	} else {
		output.valueType = valueType(strings.ToLower(vt.str))
	}

	if isList := raw.get("IsList"); isList == nil {
		v.fault("Output.IsList", raw, "required key 'IsList' is missing")
	} else if isList.kind != boolNode {
		v.fault("Output.IsList", raw, "Output.IsList expected a boolean, found "+typeOf(isList))
	} else if isList.b {
		v.fault("Output.IsList", raw, "Output.IsList must be false in format 1. List outputs are deferred")
	}

	v.readValues(raw, output)

	for _, key := range []string{"IsMandatory", "IsObsolete", "IsPopular", "ExportValues"} {
		value := raw.get(key)
		if value == nil {
			continue
		}
		if value.kind != boolNode {
			v.fault("Output."+key, raw, fmt.Sprintf("Output.%s expected a boolean, found %s", key, typeOf(value)))
			continue
		}
		b := value.b
		switch key {
		case "IsMandatory":
			output.isMandatory = &b
		case "IsObsolete":
			output.isObsolete = &b
		case "IsPopular":
			output.isPopular = &b
		case "ExportValues":
			output.exportValues = &b
		}
	}

	for _, key := range []string{"Category", "Url", "StoredValueType"} {
		value := raw.get(key)
		if value == nil {
			continue
		}
		if value.kind != stringNode {
			v.fault("Output."+key, raw, fmt.Sprintf("Output.%s expected a string, found %s", key, typeOf(value)))
			continue
		}
		s := value.str
		switch key {
		case "Category":
			output.category = &s
		case "Url":
			output.url = &s
		case "StoredValueType":
			output.storedValueType = &s
		}
	}
	for _, key := range []string{"DisplayOrder", "PropertyId"} {
		value := raw.get(key)
		if value == nil {
			continue
		}
		if value.kind != numberNode || !isWhole(value.num) {
			v.fault("Output."+key, raw, fmt.Sprintf("Output.%s expected an integer, found %s", key, typeOf(value)))
			continue
		}
		n := int64(value.num)
		if key == "DisplayOrder" {
			output.displayOrder = &n
		} else {
			output.propertyID = &n
		}
	}
	for _, key := range []string{"VendorIds", "Dependencies"} {
		value := raw.get(key)
		if value == nil {
			continue
		}
		if value.kind != listNode {
			v.fault("Output."+key, raw, fmt.Sprintf("Output.%s expected a list, found %s", key, typeOf(value)))
			continue
		}
		if key == "VendorIds" {
			output.vendorIDs = value
		} else {
			output.dependencies = value
		}
	}

	// DefaultValue is the string form of the value, so it is checked against
	// the value type and the value list.
	if dv := raw.get("DefaultValue"); dv != nil {
		if dv.kind != stringNode {
			v.fault("Output.DefaultValue", raw, "Output.DefaultValue expected a string holding the string "+
				"form of the value, found "+typeOf(dv))
		} else {
			s := dv.str
			output.defaultValue = &s
			if output.valueType != "" {
				if _, ok := convertString(s, output.valueType); !ok {
					v.fault("Output.DefaultValue", raw, fmt.Sprintf("Output.DefaultValue '%s' cannot be read as %s",
						s, output.valueType))
				}
			}
			if output.hasValues && !output.listsValue(s) {
				v.fault("Output.DefaultValue", raw, fmt.Sprintf("Output.DefaultValue '%s' is not one of the "+
					"values listed under Output.Values", s))
			}
		}
	}
	return output
}

func (v *validator) readValues(raw *node, output *outputModel) {
	values := raw.get("Values")
	if values == nil {
		return
	}
	if values.kind != listNode {
		v.fault("Output.Values", raw, "Output.Values expected a list, found "+typeOf(values))
		return
	}
	if output.valueType != "" && output.valueType != typeString && output.valueType != typeInt {
		v.fault("Output.Values", raw, fmt.Sprintf("Output.Values is only allowed where ValueType is string "+
			"or int, not %s", output.valueType))
		return
	}
	for index, entry := range values.items {
		path := fmt.Sprintf("Output.Values[%d]", index)
		if entry.kind != mapNode {
			v.fault(path, values, "a value expected a mapping of Name and Description, found "+typeOf(entry))
			continue
		}
		for _, key := range unknownKeys(entry, []string{"Name", "Description"}) {
			v.fault(path+"."+key, entry, fmt.Sprintf("unknown key '%s' in a value. Expected Name and Description", key))
		}
		name := entry.get("Name")
		if name == nil || (name.kind != stringNode && name.kind != numberNode) {
			v.fault(path+".Name", entry, "a value must have a Name")
			continue
		}
		e := valueEntry{name: literalOf(name)}
		if description := entry.get("Description"); description != nil {
			if description.kind != stringNode {
				v.fault(path+".Description", entry, "a value Description expected a string, found "+typeOf(description))
			}
			s := description.str
			e.description = &s
		}
		output.values = append(output.values, e)
	}
	seen := map[string]bool{}
	for _, e := range output.values {
		key := e.name.String()
		if seen[key] {
			v.fault("Output.Values", values, fmt.Sprintf("the value '%s' is listed more than once", key))
		}
		seen[key] = true
	}
	output.hasValues = true
}

// listsValue reports whether the string form of a value is one of the names
// under Output.Values.
func (o *outputModel) listsValue(s string) bool {
	for _, e := range o.values {
		if e.name.String() == s {
			return true
		}
	}
	return false
}

func (v *validator) readCheckNames() []string {
	raw := v.document.get("Checks")
	if raw == nil {
		return nil
	}
	if raw.kind != mapNode {
		v.fault("Checks", v.document, "Checks expected a mapping of names to conditions, found "+typeOf(raw))
		return nil
	}
	var names []string
	for _, name := range raw.keys {
		if !identifier.MatchString(name) {
			v.fault("Checks."+name, raw, fmt.Sprintf("check name '%s' does not match the pattern %s",
				name, identifier.String()))
			continue
		}
		names = append(names, name)
	}
	return names
}

func (v *validator) readChecks(m *scriptModel, checkNames []string) {
	raw := v.document.get("Checks")
	if raw == nil || raw.kind != mapNode {
		return
	}
	for _, name := range checkNames {
		c := v.readCondition(m, raw.valueOf(name), "Checks."+name, raw, checkNames)
		m.checks = append(m.checks, namedCheck{name: name, condition: c})
	}
}

func (v *validator) readRules(m *scriptModel, checkNames []string) {
	raw := v.document.get("Rules")
	if raw == nil {
		v.fault("Rules", v.document, "required key 'Rules' is missing")
		return
	}
	if raw.kind != listNode {
		v.fault("Rules", v.document, "Rules expected a list, found "+typeOf(raw))
		return
	}
	if len(raw.items) == 0 {
		v.fault("Rules", raw, "Rules must hold at least one rule")
		return
	}
	// Every script ends in an Else, so a script always chooses a value once
	// its source properties have been read.
	if raw.items[len(raw.items)-1].get("Else") == nil {
		v.fault("Rules", raw, "the last rule must be an Else, which is what a script falls back to "+
			"when no earlier rule matched")
	}
	for index, entry := range raw.items {
		path := fmt.Sprintf("Rules[%d]", index)
		if entry.kind != mapNode {
			v.fault(path, raw, "a rule expected a mapping, found "+typeOf(entry))
			continue
		}
		for _, key := range unknownKeys(entry, []string{"When", "Then", "Else"}) {
			v.fault(path+"."+key, entry, fmt.Sprintf("unknown key '%s' in a rule. Expected When and Then, or Else", key))
		}
		when, then, otherwise := entry.get("When"), entry.get("Then"), entry.get("Else")
		isLast := index == len(raw.items)-1
		switch {
		case otherwise != nil && when != nil:
			v.fault(path, entry, "a rule has both When and Else. A rule is either When with Then, or Else on its own")
			continue
		case otherwise != nil && !isLast:
			v.fault(path, entry, "Else is only allowed on the last rule")
			continue
		case otherwise == nil && when == nil:
			v.fault(path, entry, "a rule needs a When, or an Else on the last rule")
			continue
		case when != nil && then == nil:
			v.fault(path, entry, "a rule with When needs a Then")
			continue
		}
		valuePath, valueNode := path+".Then", then
		if otherwise != nil {
			valuePath, valueNode = path+".Else", otherwise
		}
		value := v.readRuleValue(m, valueNode, valuePath, entry)
		r := rule{value: value}
		if when != nil {
			r.when = v.readCondition(m, when, path+".When", entry, checkNames)
		}
		m.rules = append(m.rules, r)
	}
}

// readRuleValue reads a Then or an Else, which is a literal of
// Output.ValueType and one of Output.Values where that list is given.
func (v *validator) readRuleValue(m *scriptModel, raw *node, path string, at *node) literal {
	if raw.kind == nullNode {
		v.fault(path, at, "a rule value is a null literal, which format 1 does not allow")
		return literal{}
	}
	if raw.kind == mapNode || raw.kind == listNode {
		v.fault(path, at, "a rule value is a literal of the output value type, found "+typeOf(raw))
		return literal{}
	}
	if m.output.valueType != "" {
		literalType, _ := inferType(raw)
		if !literalMatches(literalType, m.output.valueType) {
			v.fault(path, at, fmt.Sprintf("expected a %s to match Output.ValueType, found %s",
				m.output.valueType, typeOf(raw)))
			return literal{}
		}
	}
	value := literalOf(raw)
	if m.output.hasValues && !m.output.listsValue(value.String()) {
		names := make([]string, len(m.output.values))
		for i, e := range m.output.values {
			names[i] = e.name.String()
		}
		v.fault(path, at, fmt.Sprintf("'%s' is not one of the values listed under Output.Values (%s)",
			value.String(), strings.Join(names, ", ")))
		return literal{}
	}
	return value
}

func (v *validator) readCondition(m *scriptModel, raw *node, path string, parent *node,
	checkNames []string) *condition {
	if raw == nil || raw.kind != mapNode {
		v.fault(path, parent, "a condition expected a mapping, found "+typeOf(raw))
		return faultedCondition()
	}
	if len(raw.keys) == 0 {
		v.fault(path, raw, "a condition is empty")
		return faultedCondition()
	}
	has := func(name string) bool { return raw.get(name) != nil }
	switch {
	case has("Property"):
		return v.readComparison(m, raw, path)
	case has("Check"):
		return v.readCheckReference(raw, path, checkNames)
	case has("Passed") || has("Failed"):
		return v.readAggregate(raw, path, checkNames)
	case has("All") || has("Any"):
		which := "Any"
		if has("All") {
			which = "All"
		}
		if len(raw.keys) != 1 {
			v.fault(path, raw, fmt.Sprintf("%s must be the only key of its condition, found %s",
				which, strings.Join(raw.keys, ", ")))
		}
		items := raw.get(which)
		if items.kind != listNode {
			v.fault(path+"."+which, raw, fmt.Sprintf("%s expected a list of conditions, found %s", which, typeOf(items)))
			return faultedCondition()
		}
		if len(items.items) == 0 {
			v.fault(path+"."+which, raw, which+" must list at least one condition")
			return faultedCondition()
		}
		c := &condition{kind: allCondition}
		if which == "Any" {
			c.kind = anyCondition
		}
		for index, item := range items.items {
			c.items = append(c.items, v.readCondition(m, item, fmt.Sprintf("%s.%s[%d]", path, which, index),
				items, checkNames))
		}
		return c
	case has("Not"):
		if len(raw.keys) != 1 {
			v.fault(path, raw, "Not must be the only key of its condition, found "+strings.Join(raw.keys, ", "))
		}
		item := v.readCondition(m, raw.get("Not"), path+".Not", raw, checkNames)
		return &condition{kind: notCondition, items: []*condition{item}}
	}
	v.fault(path, raw, "a condition must be a comparison, a Check reference, an aggregate, All, Any or "+
		"Not. Found the keys "+strings.Join(raw.keys, ", "))
	return faultedCondition()
}

func (v *validator) readComparison(m *scriptModel, raw *node, path string) *condition {
	var propertyKey string
	var operatorKeys []string
	for _, key := range raw.keys {
		if propertyKey == "" && strings.EqualFold(key, "Property") {
			propertyKey = key
			continue
		}
		operatorKeys = append(operatorKeys, key)
	}
	property := raw.valueOf(propertyKey)
	if property.kind != stringNode || !sourcePattern.MatchString(property.str) {
		v.fault(path+".Property", raw, fmt.Sprintf("'%s' is not a source property. Write it as "+
			"elementKey.PropertyName, for example device.IsCrawler", displayOf(property)))
		return faultedCondition()
	}
	if len(operatorKeys) == 0 {
		v.fault(path, raw, fmt.Sprintf("a comparison on '%s' has no operator. Expected exactly one of %s",
			property.str, strings.Join(operatorNames, ", ")))
		return faultedCondition()
	}
	var known []string
	for _, key := range operatorKeys {
		if canonicalName(operatorNames, key) != "" {
			known = append(known, key)
		} else {
			v.fault(path, raw, fmt.Sprintf("unknown operator '%s', expected one of %s",
				key, strings.Join(operatorNames, ", ")))
		}
	}
	if len(known) == 0 {
		return faultedCondition()
	}
	if len(operatorKeys) > 1 {
		v.fault(path, raw, "a condition takes exactly one operator, found "+strings.Join(operatorKeys, ", "))
		return faultedCondition()
	}
	op := canonicalName(operatorNames, known[0])
	operand := raw.valueOf(known[0])
	if operand.kind == nullNode {
		v.fault(path+"."+op, raw, "a null literal is not allowed. Give the value to compare against")
		return faultedCondition()
	}

	var typ valueType
	if op == "In" || op == "NotIn" {
		if operand.kind != listNode {
			v.fault(path+"."+op, raw, fmt.Sprintf("%s expects a list of values, found %s", op, typeOf(operand)))
			return faultedCondition()
		}
		if len(operand.items) == 0 {
			v.fault(path+"."+op, raw, op+" expects a non empty list")
			return faultedCondition()
		}
		// A member with no type, such as a mapping, joins as an empty string,
		// as JavaScript joins a null.
		var memberTypes []string
		numeric, allSame := true, true
		for _, member := range operand.items {
			if member.kind == nullNode {
				v.fault(path+"."+op, raw, "a null literal is not allowed in a list")
				return faultedCondition()
			}
			t, _ := inferType(member)
			memberTypes = append(memberTypes, string(t))
			if t != typeInt && t != typeDouble {
				numeric = false
			}
			if string(t) != memberTypes[0] {
				allSame = false
			}
		}
		if !numeric && !allSame {
			v.fault(path+"."+op, raw, "every member of a list must be of the same type, found "+
				strings.Join(memberTypes, ", "))
			return faultedCondition()
		}
		switch {
		case numeric && contains(memberTypes, string(typeDouble)):
			typ = typeDouble
		case numeric:
			typ = typeInt
		default:
			typ = valueType(memberTypes[0])
		}
	} else {
		t, ok := inferType(operand)
		if !ok {
			v.fault(path+"."+op, raw, fmt.Sprintf("the literal %s has no type format 1 knows", jsonOf(operand)))
			return faultedCondition()
		}
		typ = t
	}

	if !containsType(operatorTypes[op], typ) {
		allowed := make([]string, len(operatorTypes[op]))
		for i, t := range operatorTypes[op] {
			allowed[i] = string(t)
		}
		v.fault(path, raw, fmt.Sprintf("operator '%s' is not allowed on type %s. It is allowed on %s",
			op, displayType(typ), strings.Join(allowed, ", ")))
		return faultedCondition()
	}

	c := &condition{kind: compareCondition, property: property.str, op: op, operand: operand, typ: typ}
	c.slot = v.useProperty(m, property.str, typ, path, raw)
	if operand.kind == listNode {
		for _, member := range operand.items {
			c.literals = append(c.literals, literalOf(member))
		}
	} else {
		c.literal = literalOf(operand)
	}
	return c
}

func (v *validator) readCheckReference(raw *node, path string, checkNames []string) *condition {
	if len(raw.keys) != 1 {
		v.fault(path, raw, "Check must be the only key of its condition, found "+strings.Join(raw.keys, ", "))
	}
	name := raw.get("Check")
	if name.kind != stringNode {
		v.fault(path+".Check", raw, "Check expected the name of a check, found "+typeOf(name))
		return faultedCondition()
	}
	index := indexOf(checkNames, name.str)
	if index < 0 {
		v.fault(path+".Check", raw, fmt.Sprintf("check '%s' is not defined. The checks are %s",
			name.str, listOrNone(checkNames)))
		return faultedCondition()
	}
	return &condition{kind: checkCondition, check: name.str, index: index}
}

func (v *validator) readAggregate(raw *node, path string, checkNames []string) *condition {
	var aggregateKeys []string
	for _, key := range raw.keys {
		if canonicalName(aggregateNames, key) != "" {
			aggregateKeys = append(aggregateKeys, key)
		}
	}
	if len(aggregateKeys) > 1 {
		v.fault(path, raw, fmt.Sprintf("an aggregate condition takes one of %s, found %s",
			strings.Join(aggregateNames, ", "), strings.Join(aggregateKeys, ", ")))
		return faultedCondition()
	}
	agg := canonicalName(aggregateNames, aggregateKeys[0])
	group, groupNames := v.readGroup(raw.valueOf(aggregateKeys[0]), path+"."+agg, raw, checkNames)

	var operatorKeys []string
	for _, key := range raw.keys {
		if key != aggregateKeys[0] {
			operatorKeys = append(operatorKeys, key)
		}
	}
	if len(operatorKeys) == 0 {
		v.fault(path, raw, "an aggregate condition has no operator. Expected exactly one of Eq, Ne, Gt, Ge, Lt, Le")
		return faultedCondition()
	}
	if len(operatorKeys) > 1 {
		v.fault(path, raw, "a condition takes exactly one operator, found "+strings.Join(operatorKeys, ", "))
		return faultedCondition()
	}
	op := canonicalName(operatorNames, operatorKeys[0])
	if op == "" {
		v.fault(path, raw, fmt.Sprintf("unknown operator '%s', expected one of Eq, Ne, Gt, Ge, Lt, Le",
			operatorKeys[0]))
		return faultedCondition()
	}
	if !containsType(operatorTypes[op], typeInt) {
		v.fault(path, raw, fmt.Sprintf("operator '%s' is not allowed on a count, which is an int", op))
		return faultedCondition()
	}
	// A count is compared with a whole number and with nothing else.
	operand := raw.valueOf(operatorKeys[0])
	if operand.kind != numberNode || !isWhole(operand.num) {
		v.fault(path+"."+op, raw, "an aggregate is compared with a whole number, found "+typeOf(operand))
		return faultedCondition()
	}
	return &condition{kind: aggregateCondition, agg: agg, group: group, groupOf: groupNames, op: op,
		operand: operand, count: int64(operand.num)}
}

// readGroup reads the word Checks, meaning every check, which gives a nil
// group, or a list of check names, which gives their indexes.
func (v *validator) readGroup(raw *node, path string, at *node, checkNames []string) ([]int, []string) {
	switch raw.kind {
	case stringNode:
		if strings.EqualFold(raw.str, "Checks") {
			return nil, nil
		}
		v.fault(path, at, fmt.Sprintf("a group is the word Checks, meaning every check, or a list of check "+
			"names. Found '%s'", raw.str))
		return []int{}, []string{}
	case listNode:
		indexes, names := []int{}, []string{}
		for _, name := range raw.items {
			if name.kind != stringNode {
				v.fault(path, at, "a group lists check names, found "+typeOf(name))
				continue
			}
			index := indexOf(checkNames, name.str)
			if index < 0 {
				v.fault(path, at, fmt.Sprintf("check '%s' is not defined. The checks are %s",
					name.str, listOrNone(checkNames)))
				continue
			}
			indexes = append(indexes, index)
			names = append(names, name.str)
		}
		return indexes, names
	}
	v.fault(path, at, "a group is the word Checks or a list of check names, found "+typeOf(raw))
	return []int{}, []string{}
}

// useProperty records that a condition names a source property with the type
// its literal infers, and returns the slot the property is read from.
func (v *validator) useProperty(m *scriptModel, name string, typ valueType, path string, at *node) int {
	key := strings.ToLower(name)
	use, ok := v.properties[key]
	if !ok {
		use = &propertyUse{slot: len(m.properties), typ: typ, typePath: path}
		v.properties[key] = use
		m.properties = append(m.properties, sourceProperty{key: key, name: name, typ: typ})
		return use.slot
	}
	if use.typ != typ {
		v.fault(path, at, fmt.Sprintf("'%s' is inferred as %s here but was already inferred as %s at %s. "+
			"Every use of a property must infer the same type", name, typ, use.typ, use.typePath))
	}
	return use.slot
}

// inferType gives the type a literal infers. A number infers its type from
// its value rather than the way it is written, so 8, 8.0 and 8e0 all infer
// int, and a whole number outside the range of int infers double.
func inferType(n *node) (valueType, bool) {
	switch n.kind {
	case boolNode:
		return typeBool, true
	case numberNode:
		if !isWhole(n.num) {
			return typeDouble, true
		}
		if n.num >= intMin && n.num <= intMax {
			return typeInt, true
		}
		return typeDouble, true
	case stringNode:
		return typeString, true
	}
	return "", false
}

// displayType names a type in a message, as the reference implementation
// does for a literal with no type.
func displayType(t valueType) string {
	if t == "" {
		return "null"
	}
	return string(t)
}

// literalMatches reports whether a literal of one type may stand for a value
// of another. A whole number reads as a double.
func literalMatches(literalType, outputType valueType) bool {
	return literalType == outputType || (literalType == typeInt && outputType == typeDouble)
}

// typeOf describes a value in a message.
func typeOf(n *node) string {
	if n == nil {
		return "nothing"
	}
	switch n.kind {
	case nullNode:
		return "a null literal"
	case listNode:
		return "a list"
	case boolNode:
		return "a boolean"
	case numberNode:
		if isWhole(n.num) {
			return "an integer"
		}
		return "a number"
	case stringNode:
		return "a string (" + jsonOf(n) + ")"
	}
	return "a mapping"
}

// displayOf gives a value as JavaScript's template strings print it.
func displayOf(n *node) string {
	switch n.kind {
	case nullNode:
		return "null"
	case boolNode, numberNode:
		return literalOf(n).String()
	case stringNode:
		return n.str
	case listNode:
		parts := make([]string, len(n.items))
		for i, item := range n.items {
			parts[i] = displayOf(item)
		}
		return strings.Join(parts, ",")
	}
	return "[object Object]"
}

// jsonOf gives a value as compact JSON, as JSON.stringify does.
func jsonOf(n *node) string {
	return string(compactJSON(n))
}

func compactJSON(n *node) []byte {
	switch n.kind {
	case nullNode:
		return []byte("null")
	case boolNode:
		if n.b {
			return []byte("true")
		}
		return []byte("false")
	case numberNode:
		if math.IsInf(n.num, 0) || math.IsNaN(n.num) {
			return []byte("null")
		}
		return []byte(jsNumber(n.num))
	case stringNode:
		return jsonString(n.str)
	case listNode:
		out := []byte("[")
		for i, item := range n.items {
			if i > 0 {
				out = append(out, ',')
			}
			out = append(out, compactJSON(item)...)
		}
		return append(out, ']')
	}
	out := []byte("{")
	for i, key := range n.keys {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, jsonString(key)...)
		out = append(out, ':')
		out = append(out, compactJSON(n.values[i])...)
	}
	return append(out, '}')
}

// jsonString quotes a string as JSON, leaving <, > and & as they are, as
// JavaScript does.
func jsonString(s string) []byte {
	var b strings.Builder
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(s)
	return []byte(strings.TrimSuffix(b.String(), "\n"))
}

func unknownKeys(n *node, allowed []string) []string {
	var unknown []string
	for _, key := range n.keys {
		if canonicalName(allowed, key) == "" {
			unknown = append(unknown, key)
		}
	}
	return unknown
}

// canonicalName returns the name in names that matches key without regard to
// case, or empty.
func canonicalName(names []string, key string) string {
	for _, name := range names {
		if strings.EqualFold(name, key) {
			return name
		}
	}
	return ""
}

func contains(list []string, s string) bool { return indexOf(list, s) >= 0 }

func indexOf(list []string, s string) int {
	for i, item := range list {
		if item == s {
			return i
		}
	}
	return -1
}

func containsType(list []valueType, t valueType) bool {
	for _, item := range list {
		if item == t {
			return true
		}
	}
	return false
}

func listOrNone(names []string) string {
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}
