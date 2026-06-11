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

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/core/typedmap"
	"github.com/51Degrees/pipeline-go/engines"
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
	source, err := fd.Get(e.sourceKey)
	if err != nil {
		// No source data: nothing to translate.
		return nil
	}
	lang := e.resolveLanguage(fd)
	table := e.tableFor(lang)

	out := fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData { return newData() }).(*Data)

	for _, m := range e.translations {
		raw, err := source.Get(m.source)
		if err != nil {
			continue
		}
		out.Set(m.destination, e.translateValue(fd, table, raw))
	}
	return nil
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

// resolveLanguage determines the target language: a fixed language if set,
// otherwise the highest-preference language from the evidence keys in order.
func (e *Engine) resolveLanguage(fd core.FlowData) string {
	if e.fixedLanguage != "" {
		return normalizeLang(e.fixedLanguage)
	}
	if v, ok := fd.Evidence().Get(EvidenceTranslation); ok {
		if s := fmt.Sprintf("%v", v); s != "" {
			return normalizeLang(s)
		}
	}
	if v, ok := fd.Evidence().Get(EvidenceAcceptLanguage); ok {
		langs := parseAcceptLanguage(fmt.Sprintf("%v", v))
		if len(langs) > 0 {
			return normalizeLang(langs[0])
		}
	}
	return ""
}

// tableFor returns the translation table for a language, falling back to a
// two-letter language match if there is no exact locale match.
func (e *Engine) tableFor(lang string) map[string]string {
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
