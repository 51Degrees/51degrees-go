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
// translated values into a Translation Element Data.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/translation-engine.md
package translation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core/typedmap"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// DataKey is the default Element Data key for a Translation Engine. Engines that
// share it write their translated Properties into a single instance; an Engine
// can be given a distinct key via Builder.SetElementDataKey.
const DataKey = "translation"

// Evidence keys used for language selection, in order of precedence.
const (
	EvidenceTranslation         = "query.translation"
	EvidenceQueryAcceptLanguage = "query.accept-language"
	EvidenceAcceptLanguage      = "header.accept-language"
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
// An unrecognized value gives Original.
func ParseMissingBehavior(s string) MissingBehavior {
	mb, _ := lookupMissingBehavior(s)
	return mb
}

// lookupMissingBehavior converts a configuration string to a MissingBehavior,
// ignoring case and surrounding space, and reports whether the value is one of
// Original, EmptyString or FlowError.
func lookupMissingBehavior(s string) (MissingBehavior, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "original":
		return Original, true
	case "emptystring":
		return EmptyString, true
	case "flowerror":
		return FlowError, true
	default:
		return Original, false
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
	languages       *Languages
}

// Data is a Translation Element Data. It uses a concurrent map because multiple
// Translation Engines may write to a shared instance in parallel.
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
	if sourceErr != nil {
		// The whole source element is absent: record a single error and write no
		// destination Properties.
		fd.AddError(fmt.Errorf(
			"no source element '%s' was available to translate", e.sourceKey), e)
		return nil
	}

	candidates := CandidatesFromEvidence(fd, e.fixedLanguage)
	_, table, ok := e.languages.Resolve(candidates)

	// Under FlowError, a request that names no language, or names only languages
	// with no configured translator (and no base-language fallback), is reported
	// as a single engine-level error and no Properties are written. Under the
	// other behaviors a nil table simply yields identity/empty translations.
	if !ok && e.missingBehavior == FlowError && !containsBaseLanguage(candidates) {
		if len(candidates) == 0 {
			fd.AddError(errors.New(
				"the evidence did not contain a language to translate to"), e)
		} else {
			fd.AddError(fmt.Errorf(
				"there was no translator configured for the language '%s'",
				candidates[0]), e)
		}
		return nil
	}

	translator := NewTranslator(table, e.missingBehavior)

	out := fd.GetOrAdd(e.DataKey(),
		func(core.FlowData) core.ElementData { return newData() }).(*Data)

	for _, m := range e.translations {
		raw, err := source.Get(m.source)
		if err != nil {
			// The source element is present but this Property is not: write a
			// no-value result and record no error.
			out.Set(m.destination, engines.NoValue[string](fmt.Sprintf(
				"The source property '%s' was not present in the source data.",
				m.source)))
			continue
		}
		value, errs := translator.TranslateValue(raw)
		out.Set(m.destination, value)
		for _, translationErr := range errs {
			fd.AddError(translationErr, e)
		}
	}
	return nil
}

// containsBaseLanguage reports whether any candidate requests the base language
// (e.g. "en" / "en_gb"). In that case the source values are already in that
// language, so translation is intentionally skipped rather than treated as a
// missing translator under FlowError.
func containsBaseLanguage(candidates []string) bool {
	for _, c := range candidates {
		if c != "" && strings.HasPrefix(c, BaseLanguage) {
			return true
		}
	}
	return false
}
