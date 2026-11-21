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
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	pltranslation "github.com/51Degrees/51degrees-go/pipeline/v4/elements/translation"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// englishLocale is the locale of the English names, which stay as they are.
const englishLocale = "en_gb"

// dimension names the Properties of one of the two weightings,
// geographical or population.
type dimension struct {
	codes, names, translated, allNames, allCodes string
}

var dimensions = []dimension{
	{
		codes:      PropertyCountryCodesGeographical,
		names:      PropertyCountryNamesGeographical,
		translated: PropertyCountryNamesGeographicalTranslated,
		allNames:   PropertyCountryNamesGeographicalAllTranslated,
		allCodes:   PropertyCountryCodesGeographicalAll,
	},
	{
		codes:      PropertyCountryCodesPopulation,
		names:      PropertyCountryNamesPopulation,
		translated: PropertyCountryNamesPopulationTranslated,
		allNames:   PropertyCountryNamesPopulationAllTranslated,
		allCodes:   PropertyCountryCodesPopulationAll,
	},
}

// CountriesTranslationEngine translates the English country names that
// CountryCodeTranslationEngine writes into the request's language, keeping
// the weighting of each code, and builds the complete lists. Each complete
// list holds the weighted countries first, most probable first, then every
// other known country in the order of its translated name. It reads the
// weighted codes from the IP Intelligence Element Data to pair each name
// with its code.
type CountriesTranslationEngine struct {
	*core.BaseElement
	languages    languages
	cultures     map[string]string
	collators    map[string]*sync.Pool
	allCountries []countryCodeName
	validCodes   map[string]struct{}
}

// CountriesTranslationEngineBuilder builds a CountriesTranslationEngine
// with the embedded country name sources and list of known countries.
type CountriesTranslationEngineBuilder struct {
	logger *slog.Logger
}

// NewCountriesTranslationEngineBuilder creates the builder.
func NewCountriesTranslationEngineBuilder(
	logger *slog.Logger) *CountriesTranslationEngineBuilder {
	return &CountriesTranslationEngineBuilder{logger: logger}
}

// Build gives the engine. Add it to a Pipeline after the IP Intelligence
// engine and CountryCodeTranslationEngine, and never in a parallel group
// with either, because it reads what they write.
func (b *CountriesTranslationEngineBuilder) Build() (*CountriesTranslationEngine, error) {
	sources, err := countriesSources()
	if err != nil {
		return nil, err
	}
	allCountries, err := loadAllCountries()
	if err != nil {
		return nil, err
	}
	return newCountriesTranslationEngine(b.logger, sources, allCountries), nil
}

// newCountriesTranslationEngine builds the engine from country name sources,
// each keyed by a file name that names its locale, and the known countries.
// The English names are added as a source of their own, so that a request
// for English chooses it as the Translation Engine would choose any other
// language.
func newCountriesTranslationEngine(
	logger *slog.Logger,
	sources map[string]string,
	allCountries []countryCodeName,
) *CountriesTranslationEngine {
	e := &CountriesTranslationEngine{
		languages:    buildLanguages(sources),
		cultures:     map[string]string{},
		collators:    map[string]*sync.Pool{"": collatorPool(language.Und)},
		allCountries: allCountries,
		validCodes:   make(map[string]struct{}, len(allCountries)),
	}
	if _, ok := e.languages[englishLocale]; !ok {
		english := make(map[string]string, len(allCountries))
		for _, c := range allCountries {
			english[c.englishName] = c.englishName
		}
		e.languages[englishLocale] = english
	}
	for locale := range e.languages {
		if locale == englishLocale {
			continue
		}
		tag, err := language.Parse(strings.ReplaceAll(locale, "_", "-"))
		if err != nil {
			continue
		}
		culture := tag.String()
		e.cultures[locale] = culture
		if e.collators[culture] == nil {
			e.collators[culture] = collatorPool(tag)
		}
	}
	for _, c := range allCountries {
		// The codes are matched ignoring case.
		e.validCodes[strings.ToUpper(c.code)] = struct{}{}
	}
	e.BaseElement = core.NewBaseElement(
		CountryNamesTranslatedKey,
		core.NewWhitelistFilter(
			pltranslation.EvidenceTranslation,
			pltranslation.EvidenceQueryAcceptLanguage,
			pltranslation.EvidenceAcceptLanguage,
		),
		logger,
	)
	weighted := reflect.TypeOf([]core.WeightedValue[string](nil))
	list := reflect.TypeOf([]string(nil))
	props := make([]core.PropertyMetadata, 0, 3*len(dimensions))
	for _, d := range dimensions {
		props = append(props,
			property(e, d.translated, weighted),
			property(e, d.allNames, list),
			property(e, d.allCodes, list))
	}
	// SortingCultureUsed is written to the Element Data but is not offered as
	// a Property, because it exists to show how a list was sorted.
	e.SetProperties(props)
	return e
}

func property(e core.FlowElement, name string, t reflect.Type) core.PropertyMetadata {
	return core.PropertyMetadata{
		Name: name, Element: e, Category: "Translation", Type: t, Available: true,
	}
}

// collatorPool gives a pool of collators for tag, because a collator cannot
// be used by two goroutines at once. language.Und gives collators that ignore
// case, for the English names.
func collatorPool(tag language.Tag) *sync.Pool {
	if tag == language.Und {
		return &sync.Pool{New: func() any {
			return collate.New(language.Und, collate.IgnoreCase)
		}}
	}
	return &sync.Pool{New: func() any { return collate.New(tag) }}
}

// Process translates the weighted names and builds the complete lists in
// the request's language. When no language is asked for, or none that has a
// source, the names stay in English.
func (e *CountriesTranslationEngine) Process(fd core.FlowData) error {
	out := fd.GetOrAdd(CountryNamesTranslatedKey,
		func(core.FlowData) core.ElementData { return core.NewBaseElementData() })

	locale, table := e.languages.resolve(candidatesFromEvidence(fd))
	if locale == englishLocale {
		table = nil
	}
	culture := e.cultures[locale]
	pool := e.collators[culture]
	collator := pool.Get().(*collate.Collator)
	defer pool.Put(collator)
	out.Set(PropertySortingCultureUsed, culture)

	for _, d := range dimensions {
		codes := weightedCodes(fd, d.codes)
		translated := translateWeighted(englishNames(fd, d.names), codes, table)
		allCodes, allNames := e.buildAllLists(translated, codes, table, collator)
		out.Set(d.translated, translated)
		out.Set(d.allCodes, allCodes)
		out.Set(d.allNames, allNames)
	}
	return nil
}

// allListEntry is a code with its translated name and, for a weighted
// country, its raw weighting.
type allListEntry struct {
	code   string
	name   string
	weight uint16
}

// buildAllLists gives the complete list of codes and the names in the same
// order for one weighting.
func (e *CountriesTranslationEngine) buildAllLists(
	translated []core.WeightedValue[string],
	codes []core.WeightedValue[string],
	table map[string]string,
	collator *collate.Collator,
) ([]string, []string) {
	// The weighted countries, most probable first. A code that is no known
	// country, such as the engine's "Unknown", is left out, so that it never
	// leads the list.
	count := min(len(translated), len(codes))
	weighted := make([]allListEntry, 0, count)
	seen := make(map[string]struct{}, count)
	for i := 0; i < count; i++ {
		code := codes[i].Value()
		if _, known := e.validCodes[strings.ToUpper(code)]; !known {
			continue
		}
		weighted = append(weighted, allListEntry{
			code:   code,
			name:   translated[i].Value(),
			weight: translated[i].RawWeighting(),
		})
		seen[code] = struct{}{}
	}
	sort.SliceStable(weighted, func(i, j int) bool {
		if weighted[i].weight != weighted[j].weight {
			return weighted[i].weight > weighted[j].weight
		}
		if c := collator.CompareString(weighted[i].name, weighted[j].name); c != 0 {
			return c < 0
		}
		return weighted[i].code < weighted[j].code
	})

	// Every other known country, in the order of its translated name.
	remaining := make([]allListEntry, 0, len(e.allCountries))
	for _, c := range e.allCountries {
		if _, dup := seen[c.code]; dup {
			continue
		}
		remaining = append(remaining, allListEntry{
			code: c.code,
			name: translate(c.englishName, table),
		})
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		if c := collator.CompareString(remaining[i].name, remaining[j].name); c != 0 {
			return c < 0
		}
		return remaining[i].code < remaining[j].code
	})

	total := len(weighted) + len(remaining)
	allCodes := make([]string, 0, total)
	allNames := make([]string, 0, total)
	for _, entry := range append(weighted, remaining...) {
		allCodes = append(allCodes, entry.code)
		allNames = append(allNames, entry.name)
	}
	return allCodes, allNames
}

// translate gives the table's name for an English name, or the English name
// when the table has none.
func translate(name string, table map[string]string) string {
	if t, ok := table[name]; ok {
		return t
	}
	return name
}

// translateWeighted gives each English name in the table's language with the
// raw weighting of the code at the same position.
func translateWeighted(
	names []string,
	codes []core.WeightedValue[string],
	table map[string]string,
) []core.WeightedValue[string] {
	count := min(len(names), len(codes))
	out := make([]core.WeightedValue[string], count)
	for i := 0; i < count; i++ {
		out[i] = core.NewWeightedValue(
			translate(names[i], table), codes[i].RawWeighting())
	}
	return out
}

// englishNames gives a list of names CountryCodeTranslationEngine wrote into
// the Translation Element Data, or nil when there is none.
func englishNames(fd core.FlowData, property string) []string {
	v := value(fd, pltranslation.DataKey, property)
	names, _ := v.([]string)
	return names
}

// weightedCodes gives a weighted list of codes from the IP Intelligence
// Element Data, or nil when there is none.
func weightedCodes(fd core.FlowData, property string) []core.WeightedValue[string] {
	v := value(fd, IPIElementDataKey, property)
	codes, _ := v.([]core.WeightedValue[string])
	return codes
}

// value gives a Property of the Element Data under key, taken out of an
// Aspect Property Value, or nil when there is no value.
func value(fd core.FlowData, key, property string) any {
	d, err := fd.Get(key)
	if err != nil {
		return nil
	}
	v, err := d.Get(property)
	if err != nil {
		return nil
	}
	if a, ok := v.(engines.PropertyValueAccessor); ok {
		if !a.HasValue() {
			return nil
		}
		return a.AsInterface()
	}
	return v
}
