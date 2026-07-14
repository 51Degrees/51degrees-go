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
	"fmt"
	"strings"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/core/typedmap"
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
	_, table, _ := e.languages.Resolve(CandidatesFromEvidence(fd, e.fixedLanguage))
	translator := NewTranslator(table, e.missingBehavior)

	out := fd.GetOrAdd(e.DataKey(),
		func(core.FlowData) core.ElementData { return newData() }).(*Data)

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
		value, errs := translator.TranslateValue(raw)
		out.Set(m.destination, value)
		for _, translationErr := range errs {
			fd.AddError(translationErr, e)
		}
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
