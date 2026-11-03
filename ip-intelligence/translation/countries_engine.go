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
	"sort"
	"strings"

	"github.com/51Degrees/pipeline-go/core"
	pltranslation "github.com/51Degrees/pipeline-go/elements/translation"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// CountriesTranslationEngine (Engine 2) translates the weighted English country
// names from Engine 1 into the browser language and builds complete ordered
// "All" lists: the weighted countries first (most probable first), then every
// remaining known country alphabetically by translated name. It reads the
// weighted codes directly from the IP Intelligence element data to pair codes
// with names.
type CountriesTranslationEngine struct {
	*core.BaseElement
	languages    *pltranslation.Languages
	allCountries []countryCodeName
	validCodes   map[string]struct{}
}

// CountriesTranslationEngineBuilder builds Engine 2. It loads the localized
// country-name resources (countries.<locale>.yml) and the full list of known
// countries (countrycodes.en_GB.yml), both embedded.
type CountriesTranslationEngineBuilder struct {
	logger *slog.Logger
}

// NewCountriesTranslationEngineBuilder creates the Engine 2 builder.
func NewCountriesTranslationEngineBuilder(
	logger *slog.Logger) *CountriesTranslationEngineBuilder {
	return &CountriesTranslationEngineBuilder{logger: logger}
}

// Build constructs the countries translation engine.
func (b *CountriesTranslationEngineBuilder) Build() (*CountriesTranslationEngine, error) {
	sources, err := countriesSources()
	if err != nil {
		return nil, err
	}
	allCountries, err := loadAllCountries()
	if err != nil {
		return nil, err
	}
	languages := pltranslation.NewLanguages(pltranslation.BuildLookups(sources))
	return newCountriesTranslationEngine(b.logger, languages, allCountries), nil
}

// newCountriesTranslationEngine constructs the engine from a localized-name
// resolver and the full list of known countries. It is the single construction
// path used by the builder and by tests that inject custom lookups.
func newCountriesTranslationEngine(
	logger *slog.Logger,
	languages *pltranslation.Languages,
	allCountries []countryCodeName,
) *CountriesTranslationEngine {
	e := &CountriesTranslationEngine{
		languages:    languages,
		allCountries: allCountries,
		validCodes:   make(map[string]struct{}, len(allCountries)),
	}
	for _, c := range allCountries {
		// Key on the upper-case code so membership is case-insensitive, matching
		// the .NET reference (OrdinalIgnoreCase).
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
	e.SetProperties([]core.PropertyMetadata{
		translatedProp(e, PropertyCountryNamesGeographicalTranslated),
		translatedProp(e, PropertyCountryNamesPopulationTranslated),
		translatedProp(e, PropertyCountryNamesGeographicalAllTranslated),
		translatedProp(e, PropertyCountryNamesPopulationAllTranslated),
		translatedProp(e, PropertyCountryCodesGeographicalAll),
		translatedProp(e, PropertyCountryCodesPopulationAll),
		// SortingCultureUsed is deliberately NOT advertised as a Property, matching
		// the .NET reference, which excludes it from the engine's Properties (it is
		// diagnostic and not intended for customer use). The value is still written
		// to the Element Data and readable via the accessor.
	})
	return e
}

func translatedProp(e core.FlowElement, name string) core.PropertyMetadata {
	return core.PropertyMetadata{
		Name: name, Element: e, Category: "Translation", Available: true,
	}
}

// Process performs the weighted translation and builds the "All" lists.
func (e *CountriesTranslationEngine) Process(fd core.FlowData) error {
	out := fd.GetOrAdd(CountryNamesTranslatedKey,
		func(core.FlowData) core.ElementData { return core.NewBaseElementData() })

	// Resolve the locale, translator and comparer. Resolve applies the English
	// short-circuit: for English or an unknown language it returns ok=false, so
	// names stay English (identity translation) and an invariant comparer is
	// used with an empty SortingCultureUsed.
	locale, table, ok := e.languages.Resolve(
		pltranslation.CandidatesFromEvidence(fd, ""))
	translator := pltranslation.NewTranslator(table, pltranslation.Original)
	culture, comparer := comparerFor(locale, ok)
	out.Set(PropertySortingCultureUsed, culture)

	// Translate the weighted English names from Engine 1 (weights preserved).
	geoTranslated := translator.TranslateWeighted(
		readWeighted(fd, CountryNamesKey, PropertyCountryNamesGeographical))
	popTranslated := translator.TranslateWeighted(
		readWeighted(fd, CountryNamesKey, PropertyCountryNamesPopulation))
	out.Set(PropertyCountryNamesGeographicalTranslated, geoTranslated)
	out.Set(PropertyCountryNamesPopulationTranslated, popTranslated)

	// Read the weighted codes directly from the IP Intelligence element data.
	geoCodes := readWeighted(fd, IPIElementDataKey, PropertyCountryCodesGeographical)
	popCodes := readWeighted(fd, IPIElementDataKey, PropertyCountryCodesPopulation)

	geoAllCodes, geoAllNames := e.buildAllLists(geoTranslated, geoCodes, translator, comparer)
	popAllCodes, popAllNames := e.buildAllLists(popTranslated, popCodes, translator, comparer)
	out.Set(PropertyCountryCodesGeographicalAll, geoAllCodes)
	out.Set(PropertyCountryNamesGeographicalAllTranslated, geoAllNames)
	out.Set(PropertyCountryCodesPopulationAll, popAllCodes)
	out.Set(PropertyCountryNamesPopulationAllTranslated, popAllNames)
	return nil
}

// allListEntry is a code paired with its (translated) name and, for the weighted
// set, its raw weighting.
type allListEntry struct {
	code   string
	name   string
	weight uint16
}

// buildAllLists produces the ordered, index-aligned code and name lists for one
// dimension (geographical or population).
func (e *CountriesTranslationEngine) buildAllLists(
	translatedNames []core.WeightedValue[string],
	weightedCodes []core.WeightedValue[string],
	translator *pltranslation.Translator,
	comparer func(a, b string) int,
) (codes []string, names []string) {
	// 1. The weighted countries, most probable first. Drop any code that is not
	// a real country (e.g. the "Unknown" sentinel) so it never leads the list
	// or is re-added by the remaining step below.
	count := len(translatedNames)
	if len(weightedCodes) < count {
		count = len(weightedCodes)
	}
	weighted := make([]allListEntry, 0, count)
	seen := make(map[string]struct{}, count)
	for i := 0; i < count; i++ {
		code := weightedCodes[i].Value()
		if _, real := e.validCodes[strings.ToUpper(code)]; !real {
			continue
		}
		weighted = append(weighted, allListEntry{
			code:   code,
			name:   translatedNames[i].Value(),
			weight: translatedNames[i].RawWeighting(),
		})
		seen[code] = struct{}{}
	}
	sort.SliceStable(weighted, func(i, j int) bool {
		if weighted[i].weight != weighted[j].weight {
			return weighted[i].weight > weighted[j].weight
		}
		if c := comparer(weighted[i].name, weighted[j].name); c != 0 {
			return c < 0
		}
		return weighted[i].code < weighted[j].code
	})

	// 2. Every remaining known country, alphabetical by translated name.
	remaining := make([]allListEntry, 0, len(e.allCountries))
	for _, cc := range e.allCountries {
		if _, dup := seen[cc.code]; dup {
			continue
		}
		remaining = append(remaining, allListEntry{
			code: cc.code,
			name: translator.TranslateString(cc.englishName),
		})
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		if c := comparer(remaining[i].name, remaining[j].name); c != 0 {
			return c < 0
		}
		return remaining[i].code < remaining[j].code
	})

	// 3. Concatenate and split into the two index-aligned lists.
	total := len(weighted) + len(remaining)
	codes = make([]string, 0, total)
	names = make([]string, 0, total)
	for _, entry := range weighted {
		codes = append(codes, entry.code)
		names = append(names, entry.name)
	}
	for _, entry := range remaining {
		codes = append(codes, entry.code)
		names = append(names, entry.name)
	}
	return codes, names
}

// readWeighted reads a weighted string list Property from an element's data,
// returning nil when it is absent or of an unexpected type.
func readWeighted(fd core.FlowData, dataKey, property string) []core.WeightedValue[string] {
	d, err := fd.Get(dataKey)
	if err != nil {
		return nil
	}
	v, err := d.Get(property)
	if err != nil {
		return nil
	}
	w, _ := v.([]core.WeightedValue[string])
	return w
}

// comparerFor returns the culture tag and comparer used to sort translated
// names. When a locale resolved it uses a locale-aware collator and reports the
// canonical culture (e.g. "fr-FR"); otherwise it uses a culture-invariant,
// case-insensitive linguistic collation and reports an empty culture. The
// invariant path uses a Unicode root collator (matching .NET's
// InvariantCultureIgnoreCase) so accented names such as "Åland Islands" sort
// next to "A" rather than after "Z".
func comparerFor(locale string, ok bool) (string, func(a, b string) int) {
	if ok {
		if tag, err := language.Parse(strings.ReplaceAll(locale, "_", "-")); err == nil {
			collator := collate.New(tag)
			return tag.String(), func(a, b string) int {
				return collator.CompareString(a, b)
			}
		}
	}
	collator := collate.New(language.Und, collate.IgnoreCase)
	return "", func(a, b string) int {
		return collator.CompareString(a, b)
	}
}
