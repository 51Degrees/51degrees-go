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
	"math"
	"strconv"
	"strings"
)

// valueType is one of the four value types format 1 has.
type valueType string

const (
	typeString valueType = "string"
	typeBool   valueType = "bool"
	typeInt    valueType = "int"
	typeDouble valueType = "double"
)

// The range of int, which format 1 fixes at a signed 32 bit whole number so
// that one script gives one answer in every language.
const (
	intMin = math.MinInt32
	intMax = math.MaxInt32
)

// scriptModel is a validated script, the same in every language.
type scriptModel struct {
	format          int
	name            string
	version         string
	deprecated      bool
	deprecationNote string
	source          string
	output          *outputModel
	// properties are the source properties in the order the script first
	// names them, checks before rules. A comparison reads its property from
	// the slot with the same index.
	properties []sourceProperty
	checks     []namedCheck
	rules      []rule
}

// sourceProperty is a property another element produces, which a script
// reads as the type it infers from the literals compared with it.
type sourceProperty struct {
	// key is the name in lower case, which identifies the property.
	key string
	// name is the name as the script first writes it, elementKey.Property.
	name string
	typ  valueType
}

// elementKey returns the Element Data key half of the name.
func (p sourceProperty) elementKey() string {
	return strings.ToLower(p.name[:strings.IndexByte(p.name, '.')])
}

// propertyName returns the property half of the name, as written.
func (p sourceProperty) propertyName() string {
	return p.name[strings.IndexByte(p.name, '.')+1:]
}

type namedCheck struct {
	name      string
	condition *condition
}

// rule is a When with a Then, or, where when is nil, an Else.
type rule struct {
	when  *condition
	value literal
}

type conditionKind int

const (
	compareCondition conditionKind = iota
	checkCondition
	aggregateCondition
	allCondition
	anyCondition
	notCondition
)

// condition is one condition of a script, which is always true or false.
type condition struct {
	kind conditionKind

	// A comparison of a source property with a literal.
	slot     int
	property string
	op       string
	operand  *node
	typ      valueType
	// The literal, or the members of an In or NotIn list, as the type the
	// property is read as.
	literal  literal
	literals []literal

	// A reference to a check by name.
	check string
	index int

	// A count of the checks in a group that passed or failed. A nil group is
	// every check.
	agg     string
	group   []int
	count   int64
	groupOf []string

	// The members of All and Any, or the one condition of Not.
	items []*condition
}

// faultedCondition is what a condition that could not be read becomes. A
// fault has been recorded by then, so the model is discarded unevaluated.
func faultedCondition() *condition { return &condition{kind: allCondition} }

// literal is a scalar from a script, being a boolean, a number or a string.
type literal struct {
	kind nodeKind
	b    bool
	num  float64
	str  string
}

func literalOf(n *node) literal {
	return literal{kind: n.kind, b: n.b, num: n.num, str: n.str}
}

// String gives the string form of the literal, as JavaScript's String does,
// which is how a rule value is compared with Output.Values.
func (l literal) String() string {
	switch l.kind {
	case boolNode:
		return strconv.FormatBool(l.b)
	case numberNode:
		return jsNumber(l.num)
	default:
		return l.str
	}
}

// outputModel is the Output block of a script. A pointer field is nil where
// the script does not give the key.
type outputModel struct {
	name            string
	description     string
	valueType       valueType
	storedValueType *string
	defaultValue    *string
	isList          bool
	isMandatory     *bool
	isObsolete      *bool
	category        *string
	isPopular       *bool
	exportValues    *bool
	url             *string
	displayOrder    *int64
	propertyID      *int64
	vendorIDs       *node
	// dependencies is the list the script gives, or nil, in which case
	// computed holds every source property the script names.
	dependencies *node
	computed     []string
	values       []valueEntry
	hasValues    bool
}

type valueEntry struct {
	name        literal
	description *string
}

// jsNumber prints a number as JavaScript prints it, so a message or a string
// read from a number reads the same in every language.
func jsNumber(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	abs := math.Abs(f)
	if abs < 1e-6 || abs >= 1e21 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		mantissa, exponent, _ := strings.Cut(s, "e")
		sign := exponent[:1]
		digits := strings.TrimLeft(exponent[1:], "0")
		return mantissa + "e" + sign + digits
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
