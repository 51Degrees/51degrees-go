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

// Package setheaders implements the Set Headers Element, which gathers the HTTP
// response header values that other Elements want set (typically to request more
// Evidence such as User-Agent Client Hints).
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/set-headers-element.md
package setheaders

import (
	"log/slog"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// DataKey is the Element Data key for the Set Headers Element.
const DataKey = "set-headers"

// ResponseHeaderDictionaryProperty is the Property holding the response headers.
const ResponseHeaderDictionaryProperty = "response-header-dictionary"

// setHeaderPrefix marks Properties intended for response headers.
const setHeaderPrefix = "SetHeader"

// Element is the Set Headers Element.
type Element struct {
	*core.BaseElement
}

// New creates a Set Headers Element.
func New(logger *slog.Logger) *Element {
	e := &Element{BaseElement: core.NewBaseElement(DataKey, core.NewWhitelistFilter(), logger)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: ResponseHeaderDictionaryProperty, Element: e, Category: "SetHeaders", Available: true},
	})
	return e
}

// Data is the Element Data produced by the Set Headers Element.
type Data struct {
	*core.BaseElementData
}

// ResponseHeaders returns the response header name/value pairs.
func (d *Data) ResponseHeaders() map[string]string {
	if v, err := d.Get(ResponseHeaderDictionaryProperty); err == nil {
		if m, ok := v.(map[string]string); ok {
			return m
		}
	}
	return nil
}

// Process scans the Property metadata of all Pipeline elements for Properties
// named with the SetHeader convention, reads their values, and assembles the
// response-header dictionary. Property metadata names (which retain their case)
// are used because the convention depends on case to delimit the identifier and
// header name, whereas stored Property values are keyed case-insensitively.
func (e *Element) Process(fd core.FlowData) error {
	out := map[string][]string{}
	seen := map[string]map[string]struct{}{}

	for _, pm := range fd.Pipeline().Properties() {
		if !strings.HasPrefix(pm.Name, setHeaderPrefix) || pm.Element == nil {
			continue
		}
		headerName := extractHeaderName(pm.Name)
		if headerName == "" {
			continue
		}
		data, err := fd.Get(pm.Element.DataKey())
		if err != nil {
			continue
		}
		raw, err := data.Get(pm.Name)
		if err != nil {
			continue
		}
		value, ok := stringValue(raw)
		if !ok || value == "" || value == "Unknown" {
			continue
		}
		if seen[headerName] == nil {
			seen[headerName] = map[string]struct{}{}
		}
		for _, segment := range strings.Split(value, ",") {
			segment = strings.TrimSpace(segment)
			if segment == "" {
				continue
			}
			if _, dup := seen[headerName][segment]; dup {
				continue
			}
			seen[headerName][segment] = struct{}{}
			out[headerName] = append(out[headerName], segment)
		}
	}

	headers := make(map[string]string, len(out))
	for name, segments := range out {
		headers[name] = strings.Join(segments, ",")
	}

	data := fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData {
		return &Data{BaseElementData: core.NewBaseElementData()}
	})
	data.Set(ResponseHeaderDictionaryProperty, headers)
	return nil
}

// Factory builds a Set Headers Element from configuration.
func Factory(_ map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	return New(logger), nil
}

// extractHeaderName parses the header name from a SetHeader Property name. The
// format is SetHeader[Identifier][HeaderName], where Identifier has no
// upper-case characters after its first character, so the header name begins at
// the next upper-case character after the identifier.
func extractHeaderName(propName string) string {
	rest := propName[len(setHeaderPrefix):]
	if rest == "" {
		return ""
	}
	// Skip the identifier: its first character plus anything up to the next
	// upper-case letter, which begins the header name.
	i := 1
	for i < len(rest) && !isUpper(rest[i]) {
		i++
	}
	return rest[i:]
}

func isUpper(b byte) bool { return b >= 'A' && b <= 'Z' }

// stringValue extracts a string from a Property value that may be a plain string
// or an AspectPropertyValue[string].
func stringValue(raw any) (string, bool) {
	switch t := raw.(type) {
	case string:
		return t, true
	case engines.AspectPropertyValue[string]:
		if !t.HasValue() {
			return "", false
		}
		v, err := t.Value()
		return v, err == nil
	default:
		return "", false
	}
}
