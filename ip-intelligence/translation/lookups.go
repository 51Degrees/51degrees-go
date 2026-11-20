/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2025 51 Degrees Mobile Experts Limited, Davidson House,
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

// The Pipeline module's Translation Engine reads its sources and chooses its
// language inside the engine, and CountriesTranslationEngine needs the chosen
// language's table itself, to sort the All lists in that language. So this
// file reads the same sources and chooses the language by the same rules,
// except that an English request stops the search, because the names are
// already English.

// baseLanguage is the language the country names are written in.
const baseLanguage = "en"

// lookups are translation tables keyed by normalized locale, such as "fr_fr".
type lookups map[string]map[string]string

// buildLookups parses translation sources, each keyed by a file name that
// gives its locale, such as "countries.fr_FR.yml", into a table for each
// locale. A source whose name gives no locale is left out, and sources for
// the same locale are merged.
func buildLookups(sources map[string]string) lookups {
	out := lookups{}
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

// resolve gives the locale and table of the first candidate there is a table
// for, trying the exact locale, then the language alone, then the lowest
// sorted locale of that language. A candidate in the base language that has
// no exact table stops the search, because the names are already in that
// language. ok is false when no table is chosen.
func (l lookups) resolve(candidates []string) (locale string, table map[string]string, ok bool) {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if t, found := l[candidate]; found {
			return candidate, t, true
		}
		base := candidate
		if i := strings.IndexByte(candidate, '_'); i >= 0 {
			base = candidate[:i]
		}
		if base == baseLanguage {
			return "", nil, false
		}
		if t, found := l[base]; found {
			return base, t, true
		}
		var codes []string
		for code := range l {
			if strings.HasPrefix(code, base+"_") {
				codes = append(codes, code)
			}
		}
		if len(codes) > 0 {
			sort.Strings(codes)
			return codes[0], l[codes[0]], true
		}
	}
	return "", nil, false
}

// foldKeys gives each table with its keys in lower case, for translateName.
func (l lookups) foldKeys() lookups {
	out := make(lookups, len(l))
	for locale, table := range l {
		folded := make(map[string]string, len(table))
		for k, v := range table {
			folded[strings.ToLower(k)] = v
		}
		out[locale] = folded
	}
	return out
}

// translateName gives the entry of a table with folded keys for the name,
// ignoring case, or the name itself when there is none.
func translateName(table map[string]string, name string) string {
	if r, ok := table[strings.ToLower(name)]; ok && strings.TrimSpace(r) != "" {
		return r
	}
	return name
}

// candidatesFromEvidence gives the languages to try, most preferred first,
// read from the Evidence keys the Translation Engine reads, in its order, each
// as an Accept-Language list with the highest q-value first.
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

// normalizeLang gives a locale in lower case with underscores, such as
// "en_gb" for "en-GB".
func normalizeLang(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), "-", "_"))
}

// parseAcceptLanguage gives the languages of an Accept-Language value, the
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

// localeFromName gives the locale a source file name ends with, such as
// "fr_fr" for "countries.fr_FR.yml" or "en_gb" for "en_GB.yml", or an empty
// string when it ends with none.
func localeFromName(name string) string {
	base := name
	for _, ext := range []string{".yml", ".yaml"} {
		if strings.HasSuffix(strings.ToLower(base), ext) {
			base = base[:len(base)-len(ext)]
			break
		}
	}
	if i := strings.LastIndexByte(base, '.'); i >= 0 {
		base = base[i+1:]
	}
	if loc := normalizeLang(base); looksLikeLocale(loc) {
		return loc
	}
	return ""
}

// looksLikeLocale reports whether a normalized value is a language, such as
// "en", or a language and region, such as "en_gb".
func looksLikeLocale(s string) bool {
	parts := strings.Split(s, "_")
	if len(parts) > 2 {
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

// parseFlatYAML parses the flat "key: value" YAML of the translation files.
// Blank lines and lines starting with '#' are skipped, and single or double
// quotes around a key or value are removed.
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

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
