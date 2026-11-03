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

// Package translation implements the Translation Engine, which translates values
// from a source Element's Data using YAML translation sources and writes the
// translated values into a shared Translation Element Data.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/translation-engine.md
package translation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core/typedmap"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// DataKey is the Element Data key shared by all Translation Engines in a
// Pipeline. They write their translated Properties into this single instance.
const DataKey = "translation"

// Evidence keys used for language selection, in order of precedence.
const (
	EvidenceTranslation    = "query.translation"
	EvidenceAcceptLanguage = "header.accept-language"
)

// MissingBehavior controls what happens when a translation is unavailable.
type MissingBehavior int

const (
	// Original keeps the original value (default).
	Original MissingBehavior = iota
	// EmptyString substitutes an empty string.
	EmptyString
	// FlowError records an error on the Flow Data.
	FlowError
)

// ParseMissingBehavior converts a configuration string to a MissingBehavior.
func ParseMissingBehavior(s string) MissingBehavior {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "emptystring":
		return EmptyString
	case "flowerror":
		return FlowError
	default:
		return Original
	}
}

type mapping struct {
	source      string
	destination string
}

// Engine is a Translation Engine.
type Engine struct {
	*core.BaseElement
	sourceKey       string
	translations    []mapping
	fixedLanguage   string
	missingBehavior MissingBehavior
	// lookups maps a normalized language code to a source-value -> translation map.
	lookups map[string]map[string]string
}

// Data is the shared Translation Element Data. It uses a concurrent map because
// multiple Translation Engines may write to it in parallel.
type Data struct {
	values typedmap.Map
}

func newData() *Data { return &Data{values: typedmap.NewConcurrentMap()} }

// Get returns a translated Property value by name.
func (d *Data) Get(name string) (any, error) {
	v, ok := d.values.GetString(name)
	if !ok {
		return nil, core.NewDataError("translation '" + name + "' is not present")
	}
	return v, nil
}

// Set stores a translated Property value.
func (d *Data) Set(name string, value any) { d.values.AddString(name, value) }

// AsMap returns all translated Property values.
func (d *Data) AsMap() map[string]any { return d.values.AsMap() }

// GetAs retrieves a translated value converted to type T.
func GetAs[T any](d *Data, name string) (T, error) {
	var zero T
	raw, err := d.Get(name)
	if err != nil {
		return zero, err
	}
	v, ok := raw.(T)
	if !ok {
		return zero, core.NewDataError("translation '" + name + "' is not of the requested type")
	}
	return v, nil
}

// Process translates the configured Properties for the request's target language.
func (e *Engine) Process(fd core.FlowData) error {
	source, sourceErr := fd.Get(e.sourceKey)
	table := e.tableFor(e.resolveLanguageCandidates(fd))

	out := fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData { return newData() }).(*Data)

	for _, m := range e.translations {
		// Always write the destination Property (matching the .NET reference) so a
		// missing source value is surfaced as a present-but-empty value rather
		// than an absent Property.
		if sourceErr != nil {
			out.Set(m.destination, e.missingSource(fd, m.source))
			continue
		}
		raw, err := source.Get(m.source)
		if err != nil {
			out.Set(m.destination, e.missingSource(fd, m.source))
			continue
		}
		out.Set(m.destination, e.translateValue(fd, table, raw))
	}
	return nil
}

// missingSource produces the destination value when a source value is absent,
// according to the configured MissingBehavior.
func (e *Engine) missingSource(fd core.FlowData, propName string) string {
	if e.missingBehavior == FlowError {
		fd.AddError(fmt.Errorf("no source value for translation property '%s'", propName), e)
	}
	return ""
}

// translateValue translates a source value of any supported shape.
func (e *Engine) translateValue(fd core.FlowData, table map[string]string, raw any) any {
	switch v := raw.(type) {
	case string:
		return e.translateString(fd, table, v)
	case []string:
		out := make([]string, len(v))
		for i, s := range v {
			out[i] = e.translateString(fd, table, s)
		}
		return out
	case core.WeightedValue[string]:
		return e.translateString(fd, table, v.Value())
	case []core.WeightedValue[string]:
		out := make([]string, len(v))
		for i, wv := range v {
			out[i] = e.translateString(fd, table, wv.Value())
		}
		return out
	case engines.PropertyValueAccessor:
		if !v.HasValue() {
			return raw
		}
		return e.translateValue(fd, table, v.AsInterface())
	default:
		return raw
	}
}

func (e *Engine) translateString(fd core.FlowData, table map[string]string, value string) string {
	if table != nil {
		if translated, ok := table[value]; ok {
			return translated
		}
	}
	switch e.missingBehavior {
	case EmptyString:
		return ""
	case FlowError:
		fd.AddError(fmt.Errorf("no translation available for '%s'", value), e)
		return value
	default: // Original
		return value
	}
}

// resolveLanguageCandidates returns the target-language candidates in order of
// preference: a fixed language if set, otherwise query.translation followed by
// every Accept-Language entry in descending q-order. The first candidate with a
// matching translation table is used.
func (e *Engine) resolveLanguageCandidates(fd core.FlowData) []string {
	if e.fixedLanguage != "" {
		return []string{normalizeLang(e.fixedLanguage)}
	}
	var out []string
	if v, ok := fd.Evidence().Get(EvidenceTranslation); ok {
		if s := fmt.Sprintf("%v", v); s != "" {
			out = append(out, normalizeLang(s))
		}
	}
	if v, ok := fd.Evidence().Get(EvidenceAcceptLanguage); ok {
		for _, l := range parseAcceptLanguage(fmt.Sprintf("%v", v)) {
			out = append(out, normalizeLang(l))
		}
	}
	return out
}

// tableFor returns the translation table for the first of the given language
// candidates that resolves to one.
func (e *Engine) tableFor(candidates []string) map[string]string {
	for _, lang := range candidates {
		if t := e.tableForOne(lang); t != nil {
			return t
		}
	}
	return nil
}

// tableForOne returns the translation table for a single language, falling back
// to a two-letter language match if there is no exact locale match.
func (e *Engine) tableForOne(lang string) map[string]string {
	if lang == "" {
		return nil
	}
	if t, ok := e.lookups[lang]; ok {
		return t
	}
	base := lang
	if i := strings.IndexByte(lang, '_'); i >= 0 {
		base = lang[:i]
	}
	if t, ok := e.lookups[base]; ok {
		return t
	}
	// Deterministic prefix fallback: pick the lowest-sorted matching code so the
	// result does not depend on map iteration order.
	codes := make([]string, 0, len(e.lookups))
	for code := range e.lookups {
		if strings.HasPrefix(code, base) {
			codes = append(codes, code)
		}
	}
	if len(codes) > 0 {
		sort.Strings(codes)
		return e.lookups[codes[0]]
	}
	return nil
}

// normalizeLang converts a locale to a canonical lower-case underscore form
// (e.g. "en-GB" -> "en_gb").
func normalizeLang(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "-", "_"))
}

// parseAcceptLanguage parses an Accept-Language header value into languages
// ordered by descending q-value.
// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Accept-Language
func parseAcceptLanguage(header string) []string {
	type entry struct {
		lang string
		q    float64
	}
	var entries []entry
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lang := part
		q := 1.0
		if i := strings.Index(part, ";"); i >= 0 {
			lang = strings.TrimSpace(part[:i])
			params := part[i+1:]
			if j := strings.Index(params, "q="); j >= 0 {
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(params[j+2:]), 64); err == nil {
					q = parsed
				}
			}
		}
		if lang != "" && lang != "*" {
			entries = append(entries, entry{lang: lang, q: q})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].q > entries[j].q })
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.lang
	}
	return out
}
