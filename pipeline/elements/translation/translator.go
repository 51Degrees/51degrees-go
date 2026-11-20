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

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// translator translates values with one lookup table of source value to
// translated value, applying its MissingBehavior to a value the table has
// no entry for. A nil table has no entries.
type translator struct {
	table    map[string]string
	behavior MissingBehavior
}

// translateValue translates a value of a type the specification lists, being
// a string, a weighted string, a list of either, or any of those in an
// AspectPropertyValue. A weighted string gives a string and a list of them a
// list of strings. A value of any other type is given back unchanged. The
// errors are those the FlowError behavior records, one for each value with
// no translation.
func (t translator) translateValue(raw any) (any, []error) {
	switch v := raw.(type) {
	case string:
		s, err := t.translateString(v)
		return s, appendErr(nil, err)
	case []string:
		return t.translateList(v)
	case core.WeightedValue[string]:
		s, err := t.translateString(v.Value())
		return s, appendErr(nil, err)
	case []core.WeightedValue[string]:
		values := make([]string, len(v))
		for i, wv := range v {
			values[i] = wv.Value()
		}
		return t.translateList(values)
	case engines.PropertyValueAccessor:
		if !v.HasValue() {
			return raw, nil
		}
		return t.translateValue(v.AsInterface())
	default:
		return raw, nil
	}
}

// translateList translates each value of a list.
func (t translator) translateList(values []string) ([]string, []error) {
	out := make([]string, len(values))
	var errs []error
	for i, s := range values {
		r, err := t.translateString(s)
		out[i] = r
		errs = appendErr(errs, err)
	}
	return out, errs
}

// translateString gives the table's entry for the value, matched with its
// case as given, or what the MissingBehavior gives when there is none. Under
// FlowError the value is kept and the error names it.
func (t translator) translateString(value string) (string, error) {
	if r, ok := t.table[value]; ok {
		return r, nil
	}
	switch t.behavior {
	case EmptyString:
		return "", nil
	case FlowError:
		return value, fmt.Errorf("no translation available for '%s'", value)
	default: // Original
		return value, nil
	}
}

func appendErr(errs []error, err error) []error {
	if err != nil {
		errs = append(errs, err)
	}
	return errs
}

// languages are the lookup tables of an Engine, keyed by normalized locale,
// such as "fr_fr", or by language alone, such as "fr".
type languages map[string]map[string]string

// resolve gives the table of the first candidate that has one, or nil when
// none has.
func (l languages) resolve(candidates []string) map[string]string {
	for _, candidate := range candidates {
		if t := l.forOne(candidate); t != nil {
			return t
		}
	}
	return nil
}

// forOne gives the table for one language. It tries the exact locale, then
// the language alone, then the lowest sorted locale of that language, so
// that "en" and "en_us" both reach "en_gb".
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/translation-engine.md?plain=1#L90-L93
func (l languages) forOne(lang string) map[string]string {
	if lang == "" {
		return nil
	}
	if t, ok := l[lang]; ok {
		return t
	}
	base := lang
	if i := strings.IndexByte(lang, '_'); i >= 0 {
		base = lang[:i]
	}
	if t, ok := l[base]; ok {
		return t
	}
	var codes []string
	for code := range l {
		if strings.HasPrefix(code, base+"_") {
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return nil
	}
	sort.Strings(codes)
	return l[codes[0]]
}

// candidatesFromEvidence gives the languages to try, most preferred first.
// A fixed language, normalized, is the only candidate. Otherwise the
// Evidence keys are read in the specification's order, and each value is
// read as an Accept-Language list, highest q-value first.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/translation-engine.md?plain=1#L84-L95
func candidatesFromEvidence(fd core.FlowData, fixedLanguage string) []string {
	if fixedLanguage != "" {
		return []string{normalizeLang(fixedLanguage)}
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
