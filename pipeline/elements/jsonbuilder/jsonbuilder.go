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

// Package jsonbuilder implements the JSON Builder Element, which creates a JSON
// representation of the values in a Flow Data for use by the JavaScript Builder
// and client-side features.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/json-builder.md
package jsonbuilder

import (
	"encoding/json"
	"log/slog"
	"math"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// DataKey is the Element Data key for the JSON Builder Element.
const DataKey = "json"

// JSONProperty is the Property holding the produced JSON string.
const JSONProperty = "json"

// MaxJavaScriptIterations is the sequence number at or above which the
// javascriptProperties list is omitted, to prevent infinite client-side loops.
const MaxJavaScriptIterations = 10

// excludedDataKeys are the Element data keys whose Properties are never included
// in the JSON output.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/json-builder.md#process
var excludedDataKeys = map[string]struct{}{
	"set-headers":    {},
	"cloud-response": {}, // Cloud Request Engine
	"share-usage":    {}, // Usage Sharing Element
}

// Element is the JSON Builder Element.
type Element struct {
	*core.BaseElement
	includeProperties map[string]struct{} // fully-qualified, lower-case; empty = all
}

// New creates a JSON Builder Element. setProperties optionally restricts which
// Properties are included (fully-qualified names, e.g. "device.ismobile");
// JavaScript and meta Properties are always included.
func New(logger *slog.Logger, setProperties []string) *Element {
	e := &Element{BaseElement: core.NewBaseElement(DataKey, core.NewWhitelistFilter(), logger)}
	if len(setProperties) > 0 {
		e.includeProperties = make(map[string]struct{}, len(setProperties))
		for _, p := range setProperties {
			e.includeProperties[strings.ToLower(p)] = struct{}{}
		}
	}
	e.SetProperties([]core.PropertyMetadata{
		{Name: JSONProperty, Element: e, Category: "JSON", Available: true},
	})
	return e
}

// Data is the Element Data produced by the JSON Builder.
type Data struct {
	*core.BaseElementData
}

// JSON returns the produced JSON string.
func (d *Data) JSON() string {
	if v, err := d.Get(JSONProperty); err == nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Process builds the JSON representation of the Flow Data. It fails and sets
// no JSON when the query.sequence Evidence holds no integer, which means no
// Sequence Element has run before it.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/json-builder.md#process
func (e *Element) Process(fd core.FlowData) error {
	seq, ok := sequenceNumber(fd)
	if !ok {
		return core.NewConfigurationError("the JSON Builder Element found no "+
			"sequence number, so a Sequence Element needs to be placed before "+
			"it in the Pipeline", nil)
	}
	meta := buildMetaIndex(fd.Pipeline().Properties())
	result := map[string]any{}
	var jsProperties []string

	for elementKey, data := range fd.ElementDataAsMap() {
		if _, excluded := excludedDataKeys[elementKey]; excluded {
			continue
		}
		obj := map[string]any{}
		for propName, raw := range data.AsMap() {
			fullName := elementKey + "." + propName
			pm, hasMeta := meta[strings.ToLower(fullName)]
			isJS := isJavaScript(raw)

			// Apply the SetProperties include-list, always keeping JavaScript
			// Properties (meta Properties are emitted alongside their base).
			if e.includeProperties != nil && !isJS {
				if _, ok := e.includeProperties[strings.ToLower(fullName)]; !ok {
					continue
				}
			}

			// A raw (non-AspectPropertyValue) nil is omitted entirely. A
			// no-value AspectPropertyValue still emits the name plus its
			// nullreason below.
			if raw == nil {
				continue
			}

			value, isNull, nullReason := normalizeValue(raw)
			if isNull {
				obj[propName] = nil
				obj[propName+"nullreason"] = nullReason
			} else {
				obj[propName] = value
			}

			if hasMeta {
				if pm.DelayExecution {
					obj[propName+"delayexecution"] = true
				}
				if eps := delayedEvidenceProperties(pm, elementKey, meta); len(eps) > 0 {
					obj[propName+"evidenceproperties"] = eps
				}
			}
			if isJS {
				jsProperties = append(jsProperties, fullName)
			}
		}
		result[elementKey] = obj
	}

	if seq < MaxJavaScriptIterations {
		if jsProperties == nil {
			jsProperties = []string{}
		}
		result["javascriptProperties"] = jsProperties
	}

	if errs := fd.Errors(); len(errs) > 0 {
		msgs := make([]string, 0, len(errs))
		for _, fe := range errs {
			msgs = append(msgs, fe.Err.Error())
		}
		result["errors"] = msgs
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	out := fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData {
		return &Data{BaseElementData: core.NewBaseElementData()}
	})
	out.Set(JSONProperty, string(encoded))
	return nil
}

// Factory builds a JSON Builder Element from configuration.
func Factory(params map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	props, _ := core.StringListParam(params, "SetProperties")
	return New(logger, props), nil
}

// --- helpers -------------------------------------------------------------

func buildMetaIndex(props []core.PropertyMetadata) map[string]core.PropertyMetadata {
	index := make(map[string]core.PropertyMetadata, len(props))
	for _, pm := range props {
		if pm.Element == nil {
			continue
		}
		full := strings.ToLower(pm.Element.DataKey() + "." + pm.Name)
		index[full] = pm
	}
	return index
}

// normalizeValue unwraps AspectPropertyValue values, returning the underlying
// value, whether it is a "no value", and the no-value message.
func normalizeValue(raw any) (value any, isNull bool, nullReason string) {
	if raw == nil {
		return nil, true, "This property has no value."
	}
	if apv, ok := raw.(engines.PropertyValueAccessor); ok {
		if !apv.HasValue() {
			return nil, true, apv.NoValueMessage()
		}
		raw = apv.AsInterface()
	}
	// A list of sub-element data (e.g. multiple device profiles) is recursed so
	// each nested object carries its own properties and no-value reasons.
	if list, ok := raw.([]core.ElementData); ok {
		out := make([]map[string]any, len(list))
		for i, d := range list {
			out[i] = normalizeElementData(d)
		}
		return out, false, ""
	}
	return raw, false, ""
}

// normalizeElementData builds the JSON object for a nested Element Data,
// emitting a nullreason for any no-value property and recursing further.
func normalizeElementData(d core.ElementData) map[string]any {
	obj := map[string]any{}
	for name, raw := range d.AsMap() {
		if raw == nil {
			continue
		}
		value, isNull, nullReason := normalizeValue(raw)
		if isNull {
			obj[name] = nil
			obj[name+"nullreason"] = nullReason
		} else {
			obj[name] = value
		}
	}
	return obj
}

func isJavaScript(raw any) bool {
	switch t := raw.(type) {
	case core.JavaScript:
		return true
	case engines.PropertyValueAccessor:
		_, ok := t.AsInterface().(core.JavaScript)
		return ok
	default:
		return false
	}
}

// delayedEvidenceProperties returns the fully-qualified names of the Property's
// Evidence Properties that have the delay-execution flag set.
func delayedEvidenceProperties(pm core.PropertyMetadata, elementKey string, meta map[string]core.PropertyMetadata) []string {
	var out []string
	for _, ep := range pm.EvidenceProperties {
		full := ep
		if !strings.Contains(ep, ".") {
			full = elementKey + "." + ep
		}
		if m, ok := meta[strings.ToLower(full)]; ok && m.DelayExecution {
			out = append(out, full)
		}
	}
	return out
}

// sequenceNumber returns the integer in the query.sequence Evidence, which
// the Sequence Element writes, and false when that holds no integer. Text,
// such as a query string gives, is the Sequence Element's input rather than
// a sequence number, so it does not count.
func sequenceNumber(fd core.FlowData) (int, bool) {
	raw, ok := fd.Evidence().Get(sequence.EvidenceSequence)
	if !ok {
		return 0, false
	}
	switch n := raw.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), int64(int(n)) == n
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), int64(int(n)) == int64(n)
	case uint:
		return int(n), n <= math.MaxInt
	case uint64:
		return int(n), n <= math.MaxInt
	default:
		return 0, false
	}
}
