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
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// The evaluator follows the reference implementation, tools/evaluate.mjs in
// the derived-properties repository, so a script gives the same answer and the
// same message in every language.
// See https://github.com/51Degrees/derived-properties/blob/cc30638632050795d9acc80d5118a290ffaba085/docs/format-1.md#absent-and-invalid-source-properties

// usualCauses closes every message for a derived property with no value.
const usualCauses = "Usual causes are the element that supplies the property not being in " +
	"the pipeline, or being added after this element rather than before it, " +
	"the property being excluded in the engine configuration, the property " +
	"not being included in the resource key, or JavaScript that populates " +
	"the property not having run yet."

const notPresent = "property not present on this request"

// slot is one source property as read for one request, converted to the type
// the script infers for it, or the reason it could not be read.
type slot struct {
	available bool
	b         bool
	i         int64
	f         float64
	s         string
	reason    string
}

// readSource reads a property from the Element Data of its source element.
// found is false where the request carries no Element Data under the key.
func readSource(data core.ElementData, found bool, p sourceProperty) slot {
	if !found {
		return notAvailable(p, notPresent)
	}
	raw, err := data.Get(p.propertyName())
	if err != nil || raw == nil {
		return notAvailable(p, notPresent)
	}
	return readValue(raw, p)
}

func notAvailable(p sourceProperty, detail string) slot {
	return slot{reason: fmt.Sprintf("element '%s' has no value for '%s': %s",
		p.name[:strings.IndexByte(p.name, '.')], p.propertyName(), detail)}
}

// readValue converts a value an element holds. A value with no value carries
// its own message, a list of weighted values gives the value with the highest
// weight, and any other list cannot be read where one value is needed.
func readValue(raw any, p sourceProperty) slot {
	if accessor, ok := raw.(engines.PropertyValueAccessor); ok {
		if !accessor.HasValue() {
			message := accessor.NoValueMessage()
			if message == "" {
				message = notPresent
			}
			return notAvailable(p, message)
		}
		raw = accessor.AsInterface()
		if raw == nil {
			return notAvailable(p, notPresent)
		}
	}
	if value, weighted, isList := heaviest(raw); isList {
		if !weighted {
			return slot{reason: "held a list where a single value is needed"}
		}
		if value == nil {
			return notAvailable(p, notPresent)
		}
		raw = value
	} else if value, ok := singleWeighted(raw); ok {
		raw = value
	}
	if s, ok := convertValue(raw, p.typ); ok {
		return s
	}
	return slot{reason: fmt.Sprintf("held '%s' which cannot be read as %s", displayRaw(raw), p.typ)}
}

// heaviest returns the value with the highest weight from a list of weighted
// values, the first where two weigh the same. isList reports whether raw is a
// list, and weighted whether it is a non empty list of weighted values.
func heaviest(raw any) (value any, weighted bool, isList bool) {
	switch v := raw.(type) {
	case []core.WeightedValue[string]:
		return best(v)
	case []core.WeightedValue[bool]:
		return best(v)
	case []core.WeightedValue[int]:
		return best(v)
	case []core.WeightedValue[int32]:
		return best(v)
	case []core.WeightedValue[int64]:
		return best(v)
	case []core.WeightedValue[float32]:
		return best(v)
	case []core.WeightedValue[float64]:
		return best(v)
	case []core.WeightedValue[any]:
		return best(v)
	case []string, []bool, []int, []int32, []int64, []float32, []float64, []any:
		return nil, false, true
	}
	return nil, false, false
}

func best[T any](list []core.WeightedValue[T]) (any, bool, bool) {
	if len(list) == 0 {
		return nil, false, true
	}
	b := list[0]
	for _, w := range list[1:] {
		if w.RawWeighting() > b.RawWeighting() {
			b = w
		}
	}
	return b.Value(), true, true
}

// singleWeighted returns the value of a single weighted value.
func singleWeighted(raw any) (any, bool) {
	switch v := raw.(type) {
	case core.WeightedValue[string]:
		return v.Value(), true
	case core.WeightedValue[bool]:
		return v.Value(), true
	case core.WeightedValue[int]:
		return v.Value(), true
	case core.WeightedValue[float64]:
		return v.Value(), true
	}
	return nil, false
}

// convertValue converts a source value, which arrives as its native type or
// as a string, to a type. Values are never coerced loosely.
func convertValue(raw any, typ valueType) (slot, bool) {
	switch v := raw.(type) {
	case string:
		return convertString(v, typ)
	case core.WktString:
		return convertString(string(v), typ)
	case bool:
		switch typ {
		case typeBool:
			return slot{available: true, b: v}, true
		case typeString:
			return slot{available: true, s: boolString(v)}, true
		}
		return slot{}, false
	}
	if i, ok := wholeNumber(raw); ok {
		switch typ {
		case typeInt:
			if i < intMin || i > intMax {
				return slot{}, false
			}
			return slot{available: true, i: i}, true
		case typeDouble:
			return slot{available: true, f: float64(i)}, true
		case typeString:
			return slot{available: true, s: strconv.FormatInt(i, 10)}, true
		}
		return slot{}, false
	}
	if f, ok := floatNumber(raw); ok {
		switch typ {
		case typeInt:
			if !isWhole(f) || f < intMin || f > intMax {
				return slot{}, false
			}
			return slot{available: true, i: int64(f)}, true
		case typeDouble:
			if math.IsInf(f, 0) || math.IsNaN(f) {
				return slot{}, false
			}
			return slot{available: true, f: f}, true
		case typeString:
			return slot{available: true, s: jsNumber(f)}, true
		}
	}
	return slot{}, false
}

// wholeNumber reads a value of one of Go's integer types.
func wholeNumber(raw any) (int64, bool) {
	switch v := raw.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return uintNumber(uint64(v))
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		return uintNumber(v)
	}
	return 0, false
}

func uintNumber(v uint64) (int64, bool) {
	if v > math.MaxInt64 {
		return 0, false
	}
	return int64(v), true
}

func floatNumber(raw any) (float64, bool) {
	switch v := raw.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

// convertString converts the string form of a value. N/A, Unknown and the
// empty string never become false or zero.
func convertString(raw string, typ valueType) (slot, bool) {
	switch typ {
	case typeBool:
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "true":
			return slot{available: true, b: true}, true
		case "false":
			return slot{available: true, b: false}, true
		}
	case typeInt:
		trimmed := strings.TrimSpace(raw)
		if !isIntegerText(trimmed) {
			return slot{}, false
		}
		i, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil || i < intMin || i > intMax {
			return slot{}, false
		}
		return slot{available: true, i: i}, true
	case typeDouble:
		trimmed := strings.TrimSpace(raw)
		if !isDecimalText(trimmed) {
			return slot{}, false
		}
		f, err := strconv.ParseFloat(trimmed, 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return slot{}, false
		}
		return slot{available: true, f: f}, true
	case typeString:
		return slot{available: true, s: raw}, true
	}
	return slot{}, false
}

// isIntegerText reports whether s is an optional sign and then digits.
func isIntegerText(s string) bool {
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	return len(s) > 0 && digits(s) == len(s)
}

// isDecimalText reports whether s is an optional sign, digits with a full stop
// as the decimal separator, and an optional exponent.
func isDecimalText(s string) bool {
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	whole := digits(s)
	s = s[whole:]
	fraction := 0
	if len(s) > 0 && s[0] == '.' {
		s = s[1:]
		fraction = digits(s)
		s = s[fraction:]
	}
	if whole == 0 && fraction == 0 {
		return false
	}
	if len(s) > 0 && (s[0] == 'e' || s[0] == 'E') {
		s = s[1:]
		if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
			s = s[1:]
		}
		exponent := digits(s)
		if exponent == 0 {
			return false
		}
		s = s[exponent:]
	}
	return len(s) == 0
}

// digits counts the ASCII digits at the start of s.
func digits(s string) int {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}

func boolString(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// displayRaw prints a source value in a message.
func displayRaw(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case bool:
		return boolString(v)
	}
	if i, ok := wholeNumber(raw); ok {
		return strconv.FormatInt(i, 10)
	}
	if f, ok := floatNumber(raw); ok {
		return jsNumber(f)
	}
	return fmt.Sprint(raw)
}

// evaluate runs a script on the slots read for it. Every slot is available,
// so every condition is true or false and the Else always matches.
func (m *scriptModel) evaluate(slots []slot) literal {
	checks := make([]bool, len(m.checks))
	for i, c := range m.checks {
		checks[i] = evaluateCondition(c.condition, slots, checks)
	}
	for _, r := range m.rules {
		if r.when == nil || evaluateCondition(r.when, slots, checks) {
			return r.value
		}
	}
	// The validator makes every script end in an Else.
	panic("the rules of '" + m.output.name + "' do not end in an Else")
}

func evaluateCondition(c *condition, slots []slot, checks []bool) bool {
	switch c.kind {
	case compareCondition:
		return compare(slots[c.slot], c)
	case checkCondition:
		return checks[c.index]
	case aggregateCondition:
		return compareCount(countOf(c, checks), c.op, c.count)
	case allCondition:
		for _, item := range c.items {
			if !evaluateCondition(item, slots, checks) {
				return false
			}
		}
		return true
	case anyCondition:
		for _, item := range c.items {
			if evaluateCondition(item, slots, checks) {
				return true
			}
		}
		return false
	case notCondition:
		return !evaluateCondition(c.items[0], slots, checks)
	}
	return false
}

// countOf counts the checks of a group that passed or failed.
func countOf(c *condition, checks []bool) int64 {
	var count int64
	tally := func(passed bool) {
		if (c.agg == "Passed") == passed {
			count++
		}
	}
	if c.group == nil {
		for _, passed := range checks {
			tally(passed)
		}
	} else {
		for _, index := range c.group {
			tally(checks[index])
		}
	}
	return count
}

func compareCount(left int64, op string, right int64) bool {
	switch op {
	case "Eq":
		return left == right
	case "Ne":
		return left != right
	case "Gt":
		return left > right
	case "Ge":
		return left >= right
	case "Lt":
		return left < right
	case "Le":
		return left <= right
	}
	return false
}

// compare applies a comparison's operator to the value read for its property.
func compare(s slot, c *condition) bool {
	switch c.op {
	case "In":
		for _, member := range c.literals {
			if equal(s, member, c.typ) {
				return true
			}
		}
		return false
	case "NotIn":
		for _, member := range c.literals {
			if equal(s, member, c.typ) {
				return false
			}
		}
		return true
	case "Eq":
		return equal(s, c.literal, c.typ)
	case "Ne":
		return !equal(s, c.literal, c.typ)
	case "StartsWith":
		return strings.HasPrefix(s.s, c.literal.str)
	case "EndsWith":
		return strings.HasSuffix(s.s, c.literal.str)
	case "Contains":
		return strings.Contains(s.s, c.literal.str)
	}
	left, right := number(s, c.typ), c.literal.num
	switch c.op {
	case "Gt":
		return left > right
	case "Ge":
		return left >= right
	case "Lt":
		return left < right
	case "Le":
		return left <= right
	}
	return false
}

// equal reports whether the value read equals a literal, ordinally for
// strings.
func equal(s slot, l literal, typ valueType) bool {
	switch typ {
	case typeBool:
		return l.kind == boolNode && s.b == l.b
	case typeString:
		return l.kind == stringNode && s.s == l.str
	}
	return l.kind == numberNode && number(s, typ) == l.num
}

func number(s slot, typ valueType) float64 {
	if typ == typeInt {
		return float64(s.i)
	}
	return s.f
}

// missingMessage says why a script has no value, naming every source property
// that could not be read.
func missingMessage(outputName string, missing []sourceProperty, reasons []string) string {
	noun := "1 source property was not available"
	if len(missing) != 1 {
		noun = fmt.Sprintf("%d source properties were not available", len(missing))
	}
	details := make([]string, len(missing))
	for i, p := range missing {
		details[i] = fmt.Sprintf("'%s' (%s).", p.name, reasons[i])
	}
	return fmt.Sprintf("Derived property '%s' has no value because %s. %s %s",
		outputName, noun, strings.Join(details, " "), usualCauses)
}
