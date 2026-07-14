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

package translation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/engines"
)

// BaseLanguage is the language that source values are assumed to already be in.
// When it is the highest-priority requested language, no translation is
// performed and the source values are kept as-is.
const BaseLanguage = "en"

// Translator translates string based values using a single lookup table and a
// MissingBehavior. It handles a plain string, a list of strings, and a weighted
// list of strings, always returning the same shape as the input and preserving
// each weighted value's raw weighting. A nil table means no translation is
// available, in which case the MissingBehavior is applied to every value.
type Translator struct {
	table    map[string]string
	behavior MissingBehavior
}

// NewTranslator creates a Translator for the given lookup table (source value ->
// translated value). The table may be nil, in which case every value is treated
// as having no translation. Lookups are case-insensitive (matching the .NET
// reference, which uses an invariant-ignore-case comparer), so the table is
// stored with case-folded keys.
func NewTranslator(table map[string]string, behavior MissingBehavior) *Translator {
	var folded map[string]string
	if table != nil {
		folded = make(map[string]string, len(table))
		for k, v := range table {
			folded[strings.ToLower(k)] = v
		}
	}
	return &Translator{table: folded, behavior: behavior}
}

// TranslateValue translates a value of any supported shape (string, []string,
// core.WeightedValue[string], []core.WeightedValue[string], or an
// engines.PropertyValueAccessor wrapping one of those). It returns the
// translated value alongside any errors produced under the FlowError behavior.
func (t *Translator) TranslateValue(raw any) (any, []error) {
	switch v := raw.(type) {
	case string:
		return collect(t.translateString(v))
	case []string:
		out := make([]string, len(v))
		var errs []error
		for i, s := range v {
			r, err := t.translateString(s)
			out[i] = r
			errs = appendErr(errs, err)
		}
		return out, errs
	case core.WeightedValue[string]:
		r, err := t.translateString(v.Value())
		return core.NewWeightedValue(r, v.RawWeighting()), appendErr(nil, err)
	case []core.WeightedValue[string]:
		out := make([]core.WeightedValue[string], len(v))
		var errs []error
		for i, wv := range v {
			r, err := t.translateString(wv.Value())
			out[i] = core.NewWeightedValue(r, wv.RawWeighting())
			errs = appendErr(errs, err)
		}
		return out, errs
	case engines.PropertyValueAccessor:
		if !v.HasValue() {
			return raw, nil
		}
		translated, errs := t.TranslateValue(v.AsInterface())
		return wrapAspectPropertyValue(translated), errs
	default:
		// An unrecognized value shape is a programming/configuration error; report
		// it rather than silently copying it through, matching the .NET reference
		// which throws NotSupportedException.
		return raw, []error{fmt.Errorf("unsupported value type %T for translation", raw)}
	}
}

// wrapAspectPropertyValue re-wraps a translated value in an AspectPropertyValue
// so a wrapped input yields a wrapped output, matching the .NET reference. An
// unrecognized shape is returned unchanged.
func wrapAspectPropertyValue(v any) any {
	switch t := v.(type) {
	case string:
		return engines.NewAspectPropertyValue(t)
	case []string:
		return engines.NewAspectPropertyValue(t)
	case core.WeightedValue[string]:
		return engines.NewAspectPropertyValue(t)
	case []core.WeightedValue[string]:
		return engines.NewAspectPropertyValue(t)
	default:
		return v
	}
}

// TranslateString translates a single value, keeping it unchanged when the
// behavior is Original and no translation exists.
func (t *Translator) TranslateString(value string) string {
	r, _ := t.translateString(value)
	return r
}

// TranslateWeighted translates a weighted list, preserving each entry's raw
// weighting. A nil receiver leaves the values unchanged.
func (t *Translator) TranslateWeighted(
	values []core.WeightedValue[string]) []core.WeightedValue[string] {
	out := make([]core.WeightedValue[string], len(values))
	for i, wv := range values {
		out[i] = core.NewWeightedValue(t.TranslateString(wv.Value()), wv.RawWeighting())
	}
	return out
}

// translateString looks the value up in the table (case-insensitively) and
// applies the configured MissingBehavior when it is absent. Under FlowError it
// returns an empty string together with a non-nil error so the caller can
// record it, matching the .NET reference which returns null and adds the error.
func (t *Translator) translateString(value string) (string, error) {
	if t != nil && t.table != nil {
		if r, ok := t.table[strings.ToLower(value)]; ok && strings.TrimSpace(r) != "" {
			return r, nil
		}
	}
	behavior := Original
	if t != nil {
		behavior = t.behavior
	}
	switch behavior {
	case EmptyString:
		return "", nil
	case FlowError:
		return "", fmt.Errorf("no translation found for '%s'", value)
	default: // Original
		return value, nil
	}
}

func collect(r string, err error) (any, []error) { return r, appendErr(nil, err) }

func appendErr(errs []error, err error) []error {
	if err != nil {
		errs = append(errs, err)
	}
	return errs
}

// Languages resolves the best translation table for a request from a set of
// locale-keyed lookup tables. Keys are normalized locale codes (e.g. "fr_fr").
type Languages struct {
	lookups      map[string]map[string]string
	baseLanguage string
}

// NewLanguages creates a Languages resolver over the given lookups (normalized
// locale -> (source value -> translated value)).
func NewLanguages(lookups map[string]map[string]string) *Languages {
	return &Languages{lookups: lookups, baseLanguage: BaseLanguage}
}

// Resolve finds the table for the first candidate that matches an available
// locale, in order. An exact locale match wins. Otherwise, if a candidate's
// language is the base language (e.g. "en"), resolution stops and returns
// false, because the source values are already in that language and must not
// fall through to a lower-priority language. Failing both, a two-letter /
// prefix fallback is used (deterministically, lowest-sorted locale first).
// The returned locale is the matched key, empty when ok is false.
func (l *Languages) Resolve(candidates []string) (locale string, table map[string]string, ok bool) {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if t, found := l.lookups[candidate]; found {
			return candidate, t, true
		}
		// The source values are already in the base language; stop rather than
		// falling through to a lower-priority language.
		if l.baseLanguage != "" && strings.HasPrefix(candidate, l.baseLanguage) {
			return "", nil, false
		}
		base := candidate
		if i := strings.IndexByte(candidate, '_'); i >= 0 {
			base = candidate[:i]
		}
		if t, found := l.lookups[base]; found {
			return base, t, true
		}
		codes := make([]string, 0, len(l.lookups))
		for code := range l.lookups {
			if strings.HasPrefix(code, base) {
				codes = append(codes, code)
			}
		}
		if len(codes) > 0 {
			sort.Strings(codes)
			return codes[0], l.lookups[codes[0]], true
		}
	}
	return "", nil, false
}

// CandidatesFromEvidence returns the ordered target-language candidates for a
// request. A fixed language, if set, takes precedence over all evidence.
// Otherwise the evidence keys are read in precedence order (query.translation,
// query.accept-language, header.accept-language) and each value is parsed as an
// Accept-Language list, with entries normalized (lower-case, '-' -> '_').
func CandidatesFromEvidence(fd core.FlowData, fixedLanguage string) []string {
	if fixedLanguage != "" {
		// Only honour a fixed language that looks like a valid locale; an invalid
		// value is ignored and selection falls back to evidence, matching the .NET
		// reference (which validates the fixed language and falls back otherwise).
		if n := normalizeLang(fixedLanguage); looksLikeLocale(n) {
			return []string{n}
		}
	}
	var out []string
	for _, key := range []string{
		EvidenceTranslation,
		EvidenceQueryAcceptLanguage,
		EvidenceAcceptLanguage,
	} {
		v, ok := fd.Evidence().Get(key)
		if !ok {
			continue
		}
		for _, l := range parseAcceptLanguage(fmt.Sprintf("%v", v)) {
			if n := normalizeLang(l); n != "" {
				out = append(out, n)
			}
		}
	}
	return out
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
				if parsed, err := strconv.ParseFloat(
					strings.TrimSpace(params[j+2:]), 64); err == nil {
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
