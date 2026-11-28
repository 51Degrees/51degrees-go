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

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core/typedmap"
)

// DataKey is the Element Data key of every Translation Engine. The Engines
// in a Pipeline write their translated Properties into one instance under
// it, so two of them must not be members of the same parallel group.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/translation-engine.md?plain=1#L131-L136
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
	languages       languages
}

// Data is the shared Translation Element Data. Several Translation Engines may
// write this one instance, one after another, so two of them MUST NOT be
// members of the same parallel group.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/advanced-features/parallel-processing.md#how-a-group-uses-the-flow-data
type Data struct {
	core.WriteGuard
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

// Set stores a translated Property value. It panics with
// core.ErrElementDataFinished once the engines that write the instance have
// finished processing.
func (d *Data) Set(name string, value any) {
	d.CheckWrite(name)
	d.values.AddString(name, value)
}

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

// Process translates the configured Properties for the request's target
// language. Every destination is written, even when the source element or
// the source Property is absent.
func (e *Engine) Process(fd core.FlowData) error {
	source, sourceErr := fd.Get(e.sourceKey)
	t := translator{
		table:    e.languages.resolve(candidatesFromEvidence(fd, e.fixedLanguage)),
		behavior: e.missingBehavior,
	}

	out := fd.GetOrAdd(DataKey,
		func(core.FlowData) core.ElementData { return newData() }).(*Data)

	for _, m := range e.translations {
		if sourceErr != nil {
			out.Set(m.destination, e.missingSource(fd, m.source))
			continue
		}
		raw, err := source.Get(m.source)
		if err != nil {
			out.Set(m.destination, e.missingSource(fd, m.source))
			continue
		}
		value, errs := t.translateValue(raw)
		out.Set(m.destination, value)
		for _, translationErr := range errs {
			fd.AddError(translationErr, e)
		}
	}
	return nil
}

// missingSource gives the destination value when a source value is absent,
// an empty string, recording an error under FlowError.
func (e *Engine) missingSource(fd core.FlowData, propName string) string {
	if e.missingBehavior == FlowError {
		fd.AddError(fmt.Errorf(
			"no source value for translation property '%s'", propName), e)
	}
	return ""
}
