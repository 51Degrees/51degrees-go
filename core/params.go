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

package core

import (
	"fmt"
	"strconv"
	"strings"
)

// This file provides helpers for reading build parameters from the loosely
// typed map[string]any produced by configuration deserialization. Per the
// pipeline-configuration specification, configuration files contain no type
// information, so values may arrive as strings even where a numeric or boolean
// is expected; these helpers perform the necessary conversions.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/pipeline-configuration.md#deserialization

// Param looks up a build parameter by name (case-insensitive) and reports
// whether it was present.
func Param(params map[string]any, name string) (any, bool) {
	for k, v := range params {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return nil, false
}

// StringParam returns the named parameter as a string.
func StringParam(params map[string]any, name string) (string, bool) {
	v, ok := Param(params, name)
	if !ok {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	default:
		return fmt.Sprintf("%v", t), true
	}
}

// BoolParam returns the named parameter as a bool, accepting native booleans and
// the strings "true"/"false".
func BoolParam(params map[string]any, name string) (bool, bool) {
	return boolParam(params, name)
}

// IntParam returns the named parameter as an int, accepting native numeric types
// and numeric strings.
func IntParam(params map[string]any, name string) (int, bool) {
	v, ok := Param(params, name)
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64: // encoding/json decodes numbers to float64
		return int(t), true
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n, true
		}
	}
	return 0, false
}

// FloatParam returns the named parameter as a float64.
func FloatParam(params map[string]any, name string) (float64, bool) {
	v, ok := Param(params, name)
	if !ok {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// StringListParam returns the named parameter as a list of strings. A list may
// be supplied as a native slice or as a comma-separated string.
func StringListParam(params map[string]any, name string) ([]string, bool) {
	v, ok := Param(params, name)
	if !ok {
		return nil, false
	}
	switch t := v.(type) {
	case []string:
		return t, true
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, fmt.Sprintf("%v", item))
		}
		return out, true
	case string:
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out, true
	}
	return nil, false
}
