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
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// Builder constructs a Translation Engine.
type Builder struct {
	logger          *slog.Logger
	sourceKey       string
	elementDataKey  string
	patterns        []string
	contents        map[string]string
	translations    []mapping
	fixedLanguage   string
	missingBehavior MissingBehavior
}

// NewBuilder creates a Translation Engine builder.
func NewBuilder() *Builder {
	return &Builder{contents: map[string]string{}, missingBehavior: Original}
}

// SetLogger sets the logger.
func (b *Builder) SetLogger(l *slog.Logger) *Builder { b.logger = l; return b }

// SetSourceElementDataKey sets the Element Data key whose values are translated.
func (b *Builder) SetSourceElementDataKey(key string) *Builder { b.sourceKey = key; return b }

// SetElementDataKey sets the Element Data key under which this Engine stores its
// translated Properties. It defaults to DataKey, which is shared by all
// Translation Engines; set a distinct key to keep an Engine's output separate.
func (b *Builder) SetElementDataKey(key string) *Builder { b.elementDataKey = key; return b }

// AddSource adds a translation source file path or glob pattern.
func (b *Builder) AddSource(pattern string) *Builder {
	b.patterns = append(b.patterns, pattern)
	return b
}

// AddSourceContent adds translation source content directly, keyed by a filename
// that encodes the locale (e.g. "countries.en_GB.yml"). Useful for embedded data.
func (b *Builder) AddSourceContent(name, content string) *Builder {
	b.contents[name] = content
	return b
}

// AddTranslation maps a source Property to a destination Property.
func (b *Builder) AddTranslation(source, destination string) *Builder {
	b.translations = append(b.translations, mapping{source: source, destination: destination})
	return b
}

// SetFixedLanguage sets a fixed target language, bypassing Evidence selection.
func (b *Builder) SetFixedLanguage(lang string) *Builder { b.fixedLanguage = lang; return b }

// SetMissingTranslationBehavior sets the behavior when a translation is missing.
func (b *Builder) SetMissingTranslationBehavior(mb MissingBehavior) *Builder {
	b.missingBehavior = mb
	return b
}

// Build loads the configured sources, constructs the translation lookups, and
// returns the Engine.
func (b *Builder) Build() (*Engine, error) {
	if b.sourceKey == "" {
		return nil, core.NewConfigurationError("a SourceElementDataKey is required for a TranslationEngine", nil)
	}

	sources := map[string]string{}
	for _, pattern := range b.patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, core.NewConfigurationError("invalid translation source pattern '"+pattern+"'", err)
		}
		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, core.NewConfigurationError("could not read translation source '"+path+"'", err)
			}
			sources[filepath.Base(path)] = string(data)
		}
	}
	for name, content := range b.contents {
		sources[name] = content
	}

	// Validate the configuration up front rather than silently building a
	// no-op engine.
	if len(b.translations) == 0 {
		return nil, core.NewConfigurationError("a TranslationEngine requires at least one translation (AddTranslation)", nil)
	}
	if len(sources) == 0 {
		return nil, core.NewConfigurationError("a TranslationEngine requires at least one translation source", nil)
	}
	for name := range sources {
		if localeFromName(name) == "" {
			return nil, core.NewConfigurationError("translation source '"+name+"' does not encode a valid locale", nil)
		}
	}

	elementDataKey := b.elementDataKey
	if elementDataKey == "" {
		elementDataKey = DataKey
	}

	e := &Engine{
		BaseElement: core.NewBaseElement(
			elementDataKey,
			core.NewWhitelistFilter(
				EvidenceTranslation,
				EvidenceQueryAcceptLanguage,
				EvidenceAcceptLanguage),
			b.logger),
		sourceKey:       b.sourceKey,
		translations:    b.translations,
		fixedLanguage:   b.fixedLanguage,
		missingBehavior: b.missingBehavior,
		languages:       NewLanguages(BuildLookups(sources)),
	}
	props := make([]core.PropertyMetadata, 0, len(b.translations))
	for _, m := range b.translations {
		props = append(props, core.PropertyMetadata{Name: m.destination, Element: e, Category: "Translation", Available: true})
	}
	e.SetProperties(props)
	return e, nil
}

// Factory builds a Translation Engine from configuration.
func Factory(params map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	b := NewBuilder().SetLogger(logger)
	if v, ok := core.StringParam(params, "SourceElementDataKey"); ok {
		b.SetSourceElementDataKey(v)
	}
	if v, ok := core.StringListParam(params, "Sources"); ok {
		for _, s := range v {
			b.AddSource(s)
		}
	}
	if v, ok := core.StringParam(params, "FixedLanguage"); ok {
		b.SetFixedLanguage(v)
	}
	if v, ok := core.StringParam(params, "MissingTranslationBehaviour"); ok {
		b.SetMissingTranslationBehavior(ParseMissingBehavior(v))
	}
	if raw, ok := core.Param(params, "Translations"); ok {
		if list, ok := raw.([]any); ok {
			for _, item := range list {
				if m, ok := item.(map[string]any); ok {
					src, _ := core.StringParam(m, "SourceProperty")
					dst, _ := core.StringParam(m, "DestinationProperty")
					if src != "" && dst != "" {
						b.AddTranslation(src, dst)
					}
				}
			}
		}
	}
	return b.Build()
}

// BuildLookups parses translation source contents (each keyed by a filename that
// encodes the locale, e.g. "countries.fr_FR.yml") into a map of normalized
// locale -> (source value -> translated value). Sources whose name does not
// encode a locale are ignored, and sources for the same locale are merged.
// It is exported so callers embedding their own YAML (e.g. an IP Intelligence
// translation engine) build lookups exactly as the Builder does.
func BuildLookups(sources map[string]string) map[string]map[string]string {
	lookups := map[string]map[string]string{}
	for name, content := range sources {
		locale := localeFromName(name)
		if locale == "" {
			continue
		}
		if lookups[locale] == nil {
			lookups[locale] = map[string]string{}
		}
		for k, v := range parseFlatYAML(content) {
			lookups[locale][k] = v
		}
	}
	return lookups
}

// localeFromName extracts the locale from a source file name. It supports the
// "[x].[locale].yml" convention (e.g. "countries.en_GB.yml") and the bare
// "[locale].yml" form used in the specification's own example (e.g.
// "en_GB.yaml"). The candidate is validated to look like a locale so that a
// prefix segment such as "countries" is not mistaken for one.
func localeFromName(name string) string {
	base := name
	for _, ext := range []string{".yml", ".yaml", ".YML", ".YAML"} {
		if strings.HasSuffix(strings.ToLower(base), strings.ToLower(ext)) {
			base = base[:len(base)-len(ext)]
			break
		}
	}
	// Try the segment after the last '.', then fall back to the whole base.
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

// looksLikeLocale reports whether s (already normalized to lower-case with '_'
// separators) matches a language ("en") or language_region ("en_gb") code.
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

// parseFlatYAML parses the flat "key: value" subset of YAML used by translation
// source files. Blank lines and lines beginning with '#' are ignored, and
// surrounding single or double quotes are stripped from keys and values.
func parseFlatYAML(content string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
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
