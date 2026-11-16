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
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// nodeKind is the kind of value a node of a parsed script holds.
type nodeKind int

const (
	nullNode nodeKind = iota
	boolNode
	numberNode
	stringNode
	listNode
	mapNode
)

// node is one value of a parsed script. A mapping keeps its keys in the order
// they are written, because checks are numbered and faults reported in that
// order.
type node struct {
	kind   nodeKind
	b      bool
	num    float64
	str    string
	keys   []string
	values []*node
	items  []*node
	// line is the one based line the value starts on, or 0 where unknown.
	line int
}

// get returns the value of a mapping's key, matched without regard to case.
func (n *node) get(name string) *node {
	if n == nil || n.kind != mapNode {
		return nil
	}
	for i, key := range n.keys {
		if strings.EqualFold(key, name) {
			return n.values[i]
		}
	}
	return nil
}

// valueOf returns the value of a mapping's key as it is written.
func (n *node) valueOf(key string) *node {
	for i, k := range n.keys {
		if k == key {
			return n.values[i]
		}
	}
	return nil
}

// parseError is text that is not YAML or JSON, or is not a mapping.
type parseError struct {
	message string
	line    int
}

func (e *parseError) Error() string { return e.message }

var (
	jsonStart = regexp.MustCompile(`^\s*\{`)
	errorLine = regexp.MustCompile(`line (\d+)`)
)

// parseScript reads the text of a script, as YAML or JSON. JSON is read as the
// JSON subset of YAML 1.2, and only true and false are booleans, as in the
// YAML core schema.
// See https://github.com/51Degrees/derived-properties/blob/cc30638632050795d9acc80d5118a290ffaba085/docs/format-1.md#two-places-where-the-languages-are-allowed-to-differ
func parseScript(text string) (*node, error) {
	what := "YAML"
	if jsonStart.MatchString(text) {
		what = "JSON"
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil {
		line := 0
		if m := errorLine.FindStringSubmatch(err.Error()); m != nil {
			line, _ = strconv.Atoi(m[1])
		}
		message := strings.TrimPrefix(err.Error(), "yaml: ")
		return nil, &parseError{fmt.Sprintf("the text is not valid %s: %s", what, message), line}
	}
	if len(root.Content) == 0 {
		return nil, &parseError{"the script is empty", 0}
	}
	document, err := convertNode(root.Content[0], 0)
	if err != nil {
		return nil, err
	}
	if document.kind == nullNode {
		return nil, &parseError{"the script is empty", 0}
	}
	if document.kind != mapNode {
		return nil, &parseError{"the script must be a mapping of keys to values at the top level", 0}
	}
	if err := requireDistinctKeys(document); err != nil {
		return nil, err
	}
	return document, nil
}

// maxAliasDepth bounds how far aliases are followed, so an alias that refers
// to itself cannot loop.
const maxAliasDepth = 100

// convertNode turns a YAML node into a node of the script.
func convertNode(y *yaml.Node, depth int) (*node, error) {
	if depth > maxAliasDepth {
		return nil, &parseError{"the text is not valid YAML: aliases nest too deeply", y.Line}
	}
	n := &node{line: y.Line}
	switch y.Kind {
	case yaml.AliasNode:
		return convertNode(y.Alias, depth+1)
	case yaml.DocumentNode:
		if len(y.Content) == 0 {
			n.kind = nullNode
			return n, nil
		}
		return convertNode(y.Content[0], depth)
	case yaml.SequenceNode:
		n.kind = listNode
		for _, item := range y.Content {
			c, err := convertNode(item, depth)
			if err != nil {
				return nil, err
			}
			n.items = append(n.items, c)
		}
	case yaml.MappingNode:
		n.kind = mapNode
		for i := 0; i+1 < len(y.Content); i += 2 {
			v, err := convertNode(y.Content[i+1], depth)
			if err != nil {
				return nil, err
			}
			n.keys = append(n.keys, y.Content[i].Value)
			n.values = append(n.values, v)
		}
	default:
		convertScalar(y, n)
	}
	return n, nil
}

// convertScalar reads a scalar by the tag the YAML core schema gives it. A tag
// the format does not use, such as a date, leaves the text as written.
func convertScalar(y *yaml.Node, n *node) {
	switch y.ShortTag() {
	case "!!null":
		n.kind = nullNode
	case "!!bool":
		var b bool
		if y.Decode(&b) == nil {
			n.kind, n.b = boolNode, b
			return
		}
		n.kind, n.str = stringNode, y.Value
	case "!!int", "!!float":
		var v any
		if y.Decode(&v) == nil {
			if f, ok := toFloat(v); ok {
				n.kind, n.num = numberNode, f
				return
			}
		}
		if f, err := strconv.ParseFloat(y.Value, 64); err == nil {
			n.kind, n.num = numberNode, f
			return
		}
		n.kind, n.str = stringNode, y.Value
	default:
		n.kind, n.str = stringNode, y.Value
	}
}

// toFloat converts a number YAML decoded to a float64.
func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

// requireDistinctKeys refuses two keys of one mapping that differ only in
// case, because keys are matched without regard to case and one of the two
// would be dropped. It refuses a key written twice for the same reason.
func requireDistinctKeys(n *node) error {
	switch n.kind {
	case listNode:
		for _, item := range n.items {
			if err := requireDistinctKeys(item); err != nil {
				return err
			}
		}
	case mapNode:
		seen := map[string]string{}
		for i, key := range n.keys {
			lower := strings.ToLower(key)
			if first, ok := seen[lower]; ok {
				if first == key {
					return &parseError{fmt.Sprintf(
						"the key '%s' is written twice in one mapping, so one of the two would be dropped",
						key), n.line}
				}
				return &parseError{fmt.Sprintf(
					"the keys '%s' and '%s' differ only in case, and keys are matched without "+
						"regard to case, so one of the two would be dropped", first, key), n.line}
			}
			seen[lower] = key
			if err := requireDistinctKeys(n.values[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// isWhole reports whether a number has no fractional part.
func isWhole(f float64) bool {
	return !math.IsInf(f, 0) && !math.IsNaN(f) && f == math.Trunc(f)
}
