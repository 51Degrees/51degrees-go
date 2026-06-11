// Package setheaders implements the Set Headers Element, which gathers the HTTP
// response header values that other Elements want set (typically to request more
// Evidence such as User-Agent Client Hints).
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/set-headers-element.md
package setheaders

import (
	"log/slog"
	"strings"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/engines"
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
	// Skip the identifier: its first character then any lower-case characters.
	i := 1
	for i < len(rest) && isLower(rest[i]) {
		i++
	}
	return rest[i:]
}

func isLower(b byte) bool { return b >= 'a' && b <= 'z' }

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
