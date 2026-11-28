/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence (EUPL)
 * v.1.2 and is subject to its terms as set out below.
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
	pltranslation "github.com/51Degrees/51degrees-go/pipeline/v4/elements/translation"
)

// The functions in this file read translation sources and choose the
// request's language by the same rules as the Pipeline's Translation Engine,
// which keeps its own unexported, so that CountriesTranslationEngine names
// and sorts the countries in the language that engine would choose.
// TestLanguageIsTheTranslationEngines checks that the two agree.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/translation-engine.md?plain=1#L84-L95

// languages are lookup tables of source value to translated value, keyed by
// normalized locale, such as "fr_fr", or by language alone, such as "fr".
type languages map[string]map[string]string

// buildLanguages reads sources, each keyed by a file name that names its
// locale, such as "countries.fr_FR.yml", into lookup tables. A source whose
// name names no locale is left out, and sources for one locale are merged.
func buildLanguages(sources map[string]string) languages {
	out := languages{}
	for name, content := range sources {
		locale := localeFromName(name)
		if locale == "" {
			continue
		}
		if out[locale] == nil {
			out[locale] = map[string]string{}
		}
		for k, v := range parseFlatYAML(content) {
			out[locale][k] = v
		}
	}
	return out
}

// resolve gives the locale and table of the first candidate that has a
// table, or an empty locale and nil when none has.
func (l languages) resolve(candidates []string) (string, map[string]string) {
	for _, candidate := range candidates {
		if locale, t := l.forOne(candidate); t != nil {
			return locale, t
		}
	}
	return "", nil
}

// forOne gives the locale and table for one language. It tries the exact
// locale, then the language alone, then the lowest sorted locale of that
// language, so that "en" and "en_us" both reach "en_gb".
func (l languages) forOne(lang string) (string, map[string]string) {
	if lang == "" {
		return "", nil
	}
	if t, ok := l[lang]; ok {
		return lang, t
	}
	base := lang
	if i := strings.IndexByte(lang, '_'); i >= 0 {
		base = lang[:i]
	}
	if t, ok := l[base]; ok {
		return base, t
	}
	var codes []string
	for code := range l {
		if strings.HasPrefix(code, base+"_") {
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return "", nil
	}
	sort.Strings(codes)
	return codes[0], l[codes[0]]
}

// candidatesFromEvidence gives the languages to try, most preferred first.
// The Evidence keys are read in the specification's order, and each value is
// read as an Accept-Language list, highest q-value first.
func candidatesFromEvidence(fd core.FlowData) []string {
	var out []string
	for _, key := range []string{
		pltranslation.EvidenceTranslation,
		pltranslation.EvidenceQueryAcceptLanguage,
		pltranslation.EvidenceAcceptLanguage,
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

// normalizeLang gives a locale in lower case with an underscore, so "en-GB"
// gives "en_gb".
func normalizeLang(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "-", "_"))
}

// parseAcceptLanguage gives the languages of an Accept-Language value,
// highest q-value first.
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

// localeFromName gives the locale a source file name names, in the
// "[x].[locale].yml" form, such as "countries.en_GB.yml", or the bare
// "[locale].yml" form, or an empty string when it names none.
func localeFromName(name string) string {
	base := name
	for _, ext := range []string{".yml", ".yaml"} {
		if strings.HasSuffix(strings.ToLower(base), ext) {
			base = base[:len(base)-len(ext)]
			break
		}
	}
	if i := strings.LastIndexByte(base, '.'); i >= 0 {
		if loc := normalizeLang(base[i+1:]); looksLikeLocale(loc) {
			return loc
		}
	}
	if loc := normalizeLang(base); looksLikeLocale(loc) {
		return loc
	}
	return ""
}

// looksLikeLocale reports whether s, already normalized, is a language such
// as "en" or a language and region such as "en_gb".
func looksLikeLocale(s string) bool {
	parts := strings.Split(s, "_")
	if len(parts) == 0 || len(parts) > 2 {
		return false
	}
	for _, p := range parts {
		if len(p) != 2 {
			return false
		}
		for i := 0; i < len(p); i++ {
			if p[i] < 'a' || p[i] > 'z' {
				return false
			}
		}
	}
	return true
}

// parseFlatYAML reads the flat "key: value" form of the translation sources.
// Blank lines and lines starting with '#' are left out, and single or double
// quotes around a key or a value are removed.
func parseFlatYAML(content string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		i := strings.IndexByte(trimmed, ':')
		if i < 0 {
			continue
		}
		key := unquote(strings.TrimSpace(trimmed[:i]))
		value := unquote(strings.TrimSpace(trimmed[i+1:]))
		if key != "" {
			out[key] = value
		}
	}
	return out
}

// unquote removes one pair of matching single or double quotes around s.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
