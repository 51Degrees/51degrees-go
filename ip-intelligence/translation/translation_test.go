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
	"testing"

	"github.com/51Degrees/pipeline-go/core"
	pltranslation "github.com/51Degrees/pipeline-go/elements/translation"
)

// --- test doubles and helpers -------------------------------------------------

// fakeIPElement stands in for the IP Intelligence adapter: it writes the given
// weighted country-code lists under IPIElementDataKey, mirroring the .NET mock
// IP element (which exposes CountryCodesGeographical/Population).
type fakeIPElement struct {
	*core.BaseElement
	geo, pop []core.WeightedValue[string]
}

func newFakeIPElement(geo, pop []core.WeightedValue[string]) *fakeIPElement {
	e := &fakeIPElement{geo: geo, pop: pop}
	e.BaseElement = core.NewBaseElement(
		IPIElementDataKey, core.NewWhitelistFilter(), nil)
	return e
}

func (e *fakeIPElement) Process(fd core.FlowData) error {
	d := fd.GetOrAdd(IPIElementDataKey, func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	d.Set(PropertyCountryCodesGeographical, e.geo)
	d.Set(PropertyCountryCodesPopulation, e.pop)
	return nil
}

// weightify assigns descending weights by input order (first code highest),
// mirroring the .NET Weightify helper so the most probable country leads.
func weightify(codes ...string) []core.WeightedValue[string] {
	n := len(codes)
	if n == 0 {
		return []core.WeightedValue[string]{}
	}
	weightLeft := 65535
	buckets := make([]int, n)
	for i := 0; i < n; i++ {
		buckets[i] = n - i
		weightLeft -= buckets[i]
	}
	additional := weightLeft / n
	weightLeft -= additional * n
	for i := 0; i < n; i++ {
		buckets[i] += additional
	}
	buckets[0] += weightLeft
	out := make([]core.WeightedValue[string], n)
	for i, c := range codes {
		out[i] = core.NewWeightedValue(c, uint16(buckets[i]))
	}
	return out
}

// weighted builds an explicit weighted list from (code, rawWeight) pairs.
func weighted(pairs ...any) []core.WeightedValue[string] {
	out := make([]core.WeightedValue[string], 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, core.NewWeightedValue(
			pairs[i].(string), uint16(pairs[i+1].(int))))
	}
	return out
}

func codeEngine(t *testing.T) *pltranslation.Engine {
	t.Helper()
	e, err := NewCountryCodeTranslationEngineBuilder(nil).Build()
	if err != nil {
		t.Fatalf("build country-code engine: %v", err)
	}
	return e
}

func countriesEngine(t *testing.T) *CountriesTranslationEngine {
	t.Helper()
	e, err := NewCountriesTranslationEngineBuilder(nil).Build()
	if err != nil {
		t.Fatalf("build countries engine: %v", err)
	}
	return e
}

// codePipeline builds a pipeline with just the IP element and Engine 1.
func codePipeline(t *testing.T, geo, pop []core.WeightedValue[string]) core.Pipeline {
	t.Helper()
	p, err := core.NewPipelineBuilder().
		AddFlowElement(newFakeIPElement(geo, pop)).
		AddFlowElement(codeEngine(t)).
		Build()
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	return p
}

// fullPipeline builds the IP element, Engine 1 and Engine 2.
func fullPipeline(t *testing.T, geo, pop []core.WeightedValue[string]) core.Pipeline {
	t.Helper()
	p, err := core.NewPipelineBuilder().
		AddFlowElement(newFakeIPElement(geo, pop)).
		AddFlowElement(codeEngine(t)).
		AddFlowElement(countriesEngine(t)).
		Build()
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	return p
}

func processWith(t *testing.T, p core.Pipeline, evidence map[string]any) core.FlowData {
	t.Helper()
	fd := p.CreateFlowData()
	for k, v := range evidence {
		fd.AddEvidence(k, v)
	}
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	return fd
}

// allKnownMap returns the code -> English name map from the embedded data.
func allKnownMap(t *testing.T) map[string]string {
	t.Helper()
	all, err := loadAllCountries()
	if err != nil {
		t.Fatalf("load all countries: %v", err)
	}
	m := make(map[string]string, len(all))
	for _, c := range all {
		m[c.code] = c.englishName
	}
	return m
}

// localeTable returns the translation table for a locale from the embedded data.
func localeTable(t *testing.T, locale string) map[string]string {
	t.Helper()
	src, err := countriesSources()
	if err != nil {
		t.Fatalf("countries sources: %v", err)
	}
	return pltranslation.BuildLookups(src)[locale]
}

// comparerFromCulture rebuilds the sort comparer for an asserted culture.
func comparerFromCulture(culture string) func(a, b string) int {
	_, c := comparerFor(culture, culture != "")
	return c
}

// assertSortedTail checks names[skip:] are non-descending by comparer.
func assertSortedTail(t *testing.T, names []string, skip int, comparer func(a, b string) int) {
	t.Helper()
	for i := skip + 1; i < len(names); i++ {
		if comparer(names[i-1], names[i]) > 0 {
			t.Fatalf("tail not sorted at %d: %q > %q", i, names[i-1], names[i])
		}
	}
}

// assertAligned checks every (code, name) pair is consistent with the data: the
// name at position i is the (possibly translated) name of the code at i.
func assertAligned(t *testing.T, codes, names []string, table map[string]string) {
	t.Helper()
	if len(codes) != len(names) {
		t.Fatalf("length mismatch: %d codes, %d names", len(codes), len(names))
	}
	known := allKnownMap(t)
	for i := range codes {
		english, ok := known[codes[i]]
		if !ok {
			t.Fatalf("unknown code %q at %d", codes[i], i)
		}
		want := english
		if tr, ok := table[english]; ok && tr != "" {
			want = tr
		}
		if names[i] != want {
			t.Fatalf("misaligned at %d: code %q -> %q, want %q",
				i, codes[i], names[i], want)
		}
	}
}

// --- 10a: ported .NET coverage -----------------------------------------------

// CountryNamesFromCodes: Engine 1 alone turns GB,FR into United Kingdom, France
// for both dimensions.
func TestCountryNamesFromCodes(t *testing.T) {
	p := codePipeline(t, weightify("GB", "FR"), weightify("GB", "FR"))
	defer p.Close()
	fd := processWith(t, p, nil)

	data, err := CountryNames(fd)
	if err != nil {
		t.Fatalf("country names data: %v", err)
	}
	geo := data.CountryNamesGeographical()
	pop := data.CountryNamesPopulation()
	if len(geo) != 2 || len(pop) != 2 {
		t.Fatalf("expected 2 names each, got geo=%d pop=%d", len(geo), len(pop))
	}
	if geo[0].Value() != "United Kingdom" || geo[1].Value() != "France" {
		t.Fatalf("geo names = %q, %q", geo[0].Value(), geo[1].Value())
	}
	if pop[0].Value() != "United Kingdom" || pop[1].Value() != "France" {
		t.Fatalf("pop names = %q, %q", pop[0].Value(), pop[1].Value())
	}
}

// TranslatedCountry: Engines 1 and 2 with fr_FR turn GB,FR into Royaume-Uni,
// France (weighted, translated).
func TestTranslatedCountry(t *testing.T) {
	p := fullPipeline(t, weightify("GB", "FR"), weightify("GB", "FR"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr_FR"})

	data, _ := Countries(fd)
	geo := data.CountryNamesGeographicalTranslated()
	if len(geo) != 2 {
		t.Fatalf("expected 2 translated names, got %d", len(geo))
	}
	if geo[0].Value() != "Royaume-Uni" || geo[1].Value() != "France" {
		t.Fatalf("translated = %q, %q", geo[0].Value(), geo[1].Value())
	}
}

// AllListsProducedCorrectlySorted: explicit weights (FR=30000, GB=35535), fr_FR:
// GB then FR first (weight descending), >200 countries, GB/FR not repeated, tail
// sorted by the locale comparer.
func TestAllListsProducedCorrectlySorted(t *testing.T) {
	geo := weighted("FR", 30000, "GB", 35535)
	p := fullPipeline(t, geo, geo)
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr_FR"})

	data, _ := Countries(fd)
	names := data.CountryNamesGeographicalAllTranslated()
	codes := data.CountryCodesGeographicalAll()

	if len(names) != len(codes) {
		t.Fatalf("names/codes length mismatch: %d vs %d", len(names), len(codes))
	}
	if codes[0] != "GB" || names[0] != "Royaume-Uni" {
		t.Fatalf("first = (%q, %q), want (GB, Royaume-Uni)", codes[0], names[0])
	}
	if codes[1] != "FR" || names[1] != "France" {
		t.Fatalf("second = (%q, %q), want (FR, France)", codes[1], names[1])
	}
	if len(names) <= 200 {
		t.Fatalf("expected all countries, got %d", len(names))
	}
	for _, c := range codes[2:] {
		if c == "GB" || c == "FR" {
			t.Fatalf("weighted code %q repeated in tail", c)
		}
	}
	assertSortedTail(t, names, 2, comparerFromCulture(data.SortingCultureUsed()))
}

// AllListsProducedCorrectly: as above, with weights assigned by input order.
func TestAllListsProducedCorrectly(t *testing.T) {
	p := fullPipeline(t, weightify("GB", "FR"), weightify("GB", "FR"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr_FR"})

	data, _ := Countries(fd)
	names := data.CountryNamesGeographicalAllTranslated()
	codes := data.CountryCodesGeographicalAll()

	if codes[0] != "GB" || names[0] != "Royaume-Uni" {
		t.Fatalf("first = (%q, %q)", codes[0], names[0])
	}
	if codes[1] != "FR" || names[1] != "France" {
		t.Fatalf("second = (%q, %q)", codes[1], names[1])
	}
	if len(names) <= 200 {
		t.Fatalf("expected all countries, got %d", len(names))
	}
	assertSortedTail(t, names, 2, comparerFromCulture(data.SortingCultureUsed()))
}

// AllListsWithoutLanguage: no evidence -> English names, GB first, tail
// alphabetical.
func TestAllListsWithoutLanguage(t *testing.T) {
	p := fullPipeline(t, weightify("GB"), weightify("GB"))
	defer p.Close()
	fd := processWith(t, p, nil)

	data, _ := Countries(fd)
	names := data.CountryNamesGeographicalAllTranslated()
	codes := data.CountryCodesGeographicalAll()

	if names[0] != "United Kingdom" || codes[0] != "GB" {
		t.Fatalf("first = (%q, %q), want (GB, United Kingdom)", codes[0], names[0])
	}
	if data.SortingCultureUsed() != "" {
		t.Fatalf("SortingCultureUsed = %q, want empty", data.SortingCultureUsed())
	}
	if len(names) <= 200 {
		t.Fatalf("expected all countries, got %d", len(names))
	}
	assertSortedTail(t, names, 1, comparerFromCulture(""))
}

// AccentedNamesSortLinguisticallyOnEnglishPath: on the English/fallback path the
// tail must use culture-invariant linguistic collation (matching .NET's
// InvariantCultureIgnoreCase), so an accented name like "Åland Islands" sorts
// next to the other "A" names rather than after "Z" as a byte-ordinal sort would
// place it.
func TestAccentedNamesSortLinguisticallyOnEnglishPath(t *testing.T) {
	p := fullPipeline(t, weightify("GB"), weightify("GB"))
	defer p.Close()
	fd := processWith(t, p, nil) // no Accept-Language -> English

	data, _ := Countries(fd)
	names := data.CountryNamesGeographicalAllTranslated()

	indexOf := func(name string) int {
		for i, n := range names {
			if n == name {
				return i
			}
		}
		return -1
	}
	aland := indexOf("Åland Islands") // "Åland Islands"
	zimbabwe := indexOf("Zimbabwe")
	if aland < 0 || zimbabwe < 0 {
		t.Fatalf("expected both names present (aland=%d zimbabwe=%d)", aland, zimbabwe)
	}
	if aland > zimbabwe {
		t.Fatalf("accented name sorted after Zimbabwe (aland=%d zimbabwe=%d); "+
			"expected linguistic collation to place it near the A's", aland, zimbabwe)
	}
}

// AllListsWithNoIpData: no weighted codes, de_DE -> every country present, fully
// alphabetical.
func TestAllListsWithNoIpData(t *testing.T) {
	p := fullPipeline(t, weightify(), weightify())
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "de_DE"})

	data, _ := Countries(fd)
	names := data.CountryNamesGeographicalAllTranslated()
	codes := data.CountryCodesGeographicalAll()

	if len(names) <= 200 {
		t.Fatalf("expected all countries, got %d", len(names))
	}
	assertSortedTail(t, names, 0, comparerFromCulture(data.SortingCultureUsed()))
	assertAligned(t, codes, names, localeTable(t, "de_de"))
}

// PopulationAndGeographicalAreIndependent: geo DE, pop US,CN.
func TestPopulationAndGeographicalAreIndependent(t *testing.T) {
	p := fullPipeline(t, weightify("DE"), weightify("US", "CN"))
	defer p.Close()
	fd := processWith(t, p, nil)

	data, _ := Countries(fd)
	geoCodes := data.CountryCodesGeographicalAll()
	popCodes := data.CountryCodesPopulationAll()

	if geoCodes[0] != "DE" {
		t.Fatalf("geo[0] = %q, want DE", geoCodes[0])
	}
	if popCodes[0] != "US" || popCodes[1] != "CN" {
		t.Fatalf("pop[0:2] = %q, %q, want US, CN", popCodes[0], popCodes[1])
	}
}

// GermanTranslation: de_DE -> Deutschland weighted and first in the All list.
func TestGermanTranslation(t *testing.T) {
	p := fullPipeline(t, weightify("DE"), weightify("DE"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "de_DE"})

	data, _ := Countries(fd)
	if v := data.CountryNamesGeographicalTranslated()[0].Value(); v != "Deutschland" {
		t.Fatalf("translated = %q, want Deutschland", v)
	}
	if v := data.CountryNamesGeographicalAllTranslated()[0]; v != "Deutschland" {
		t.Fatalf("all[0] = %q, want Deutschland", v)
	}
	if v := data.CountryCodesGeographicalAll()[0]; v != "DE" {
		t.Fatalf("code[0] = %q, want DE", v)
	}
}

// AcceptLanguageWithDash: fr-FR (dash) resolves the same as fr_FR.
func TestAcceptLanguageWithDash(t *testing.T) {
	p := fullPipeline(t, weightify("GB"), weightify("GB"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr-FR"})

	data, _ := Countries(fd)
	if v := data.CountryNamesGeographicalTranslated()[0].Value(); v != "Royaume-Uni" {
		t.Fatalf("translated = %q, want Royaume-Uni", v)
	}
}

// EnglishLanguageNoTranslation: en-US,en;q=0.9 -> names stay English.
func TestEnglishLanguageNoTranslation(t *testing.T) {
	p := fullPipeline(t, weightify("FR", "DE"), weightify("FR", "DE"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "en-US,en;q=0.9"})

	data, _ := Countries(fd)
	geo := data.CountryNamesGeographicalTranslated()
	if geo[0].Value() != "France" || geo[1].Value() != "Germany" {
		t.Fatalf("translated = %q, %q, want France, Germany", geo[0].Value(), geo[1].Value())
	}
	all := data.CountryNamesGeographicalAllTranslated()
	if all[0] != "France" || all[1] != "Germany" {
		t.Fatalf("all[0:2] = %q, %q, want France, Germany", all[0], all[1])
	}
}

// EnglishPreferredOverOtherLanguages: en-US,en;q=0.9,de-DE;q=0.5 -> English wins.
func TestEnglishPreferredOverOtherLanguages(t *testing.T) {
	p := fullPipeline(t, weightify("DE"), weightify("DE"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "en-US,en;q=0.9,de-DE;q=0.5,fr;q=0.3"})

	data, _ := Countries(fd)
	if v := data.CountryNamesGeographicalTranslated()[0].Value(); v != "Germany" {
		t.Fatalf("translated = %q, want Germany (English, not Deutschland)", v)
	}
}

// PreferredLanguageMatchedBeforeLowerPriority: es,de-DE;q=0.8,fr;q=0.5 -> Spanish
// Alemania (2-letter es resolves to es_ES).
func TestPreferredLanguageMatchedBeforeLowerPriority(t *testing.T) {
	p := fullPipeline(t, weightify("DE"), weightify("DE"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "es,de-DE;q=0.8,fr;q=0.5"})

	data, _ := Countries(fd)
	if v := data.CountryNamesGeographicalTranslated()[0].Value(); v != "Alemania" {
		t.Fatalf("translated = %q, want Alemania (Spanish)", v)
	}
}

// --- .NET sentinel coverage ---------------------------------------------------

// AllListsExcludeUnknownWhenNoRealCodes: with only the "Unknown" sentinel, the
// lists are exactly all known countries, alphabetical, no "Unknown".
func TestAllListsExcludeUnknownWhenNoRealCodes(t *testing.T) {
	p := fullPipeline(t, weightify("Unknown"), weightify("Unknown"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "de_DE"})

	data, _ := Countries(fd)
	names := data.CountryNamesGeographicalAllTranslated()
	codes := data.CountryCodesGeographicalAll()
	if len(names) <= 200 {
		t.Fatalf("expected all countries, got %d", len(names))
	}
	for _, c := range codes {
		if c == "Unknown" {
			t.Fatal("Unknown must not appear in codes")
		}
	}
	for _, n := range names {
		if n == "Unknown" {
			t.Fatal("Unknown must not appear in names")
		}
	}
	assertSortedTail(t, names, 0, comparerFromCulture(data.SortingCultureUsed()))
	assertAligned(t, codes, names, localeTable(t, "de_de"))
}

// AllListsExcludeUnknownWhenMixedWithRealCode: DE then Unknown -> DE leads,
// Unknown dropped, tail alphabetical.
func TestAllListsExcludeUnknownWhenMixedWithRealCode(t *testing.T) {
	p := fullPipeline(t, weightify("DE", "Unknown"), weightify("DE", "Unknown"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "de_DE"})

	data, _ := Countries(fd)
	names := data.CountryNamesGeographicalAllTranslated()
	codes := data.CountryCodesGeographicalAll()
	if codes[0] != "DE" || names[0] != "Deutschland" {
		t.Fatalf("first = (%q, %q), want (DE, Deutschland)", codes[0], names[0])
	}
	for _, c := range codes {
		if c == "Unknown" {
			t.Fatal("Unknown must not appear")
		}
	}
	assertSortedTail(t, names, 1, comparerFromCulture(data.SortingCultureUsed()))
}

// --- 10b: gap coverage --------------------------------------------------------

// Gap 1: bare query.translation translates to French, and query.translation
// overrides a different header.accept-language.
func TestQueryTranslationEvidenceAndPrecedence(t *testing.T) {
	p := fullPipeline(t, weightify("GB"), weightify("GB"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceTranslation:    "fr_FR",
		pltranslation.EvidenceAcceptLanguage: "de-DE",
	})
	data, _ := Countries(fd)
	if v := data.CountryNamesGeographicalTranslated()[0].Value(); v != "Royaume-Uni" {
		t.Fatalf("translated = %q, want Royaume-Uni (query.translation wins)", v)
	}
	if c := data.SortingCultureUsed(); c != "fr-FR" {
		t.Fatalf("culture = %q, want fr-FR", c)
	}
}

// Gap 2: weights are preserved through translation; only the name changes.
func TestWeightsPreservedThroughTranslation(t *testing.T) {
	geo := weighted("GB", 40000, "FR", 25000)
	p := fullPipeline(t, geo, geo)
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr_FR"})

	data, _ := Countries(fd)
	got := data.CountryNamesGeographicalTranslated()
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].RawWeighting() != 40000 || got[1].RawWeighting() != 25000 {
		t.Fatalf("weights = %d, %d, want 40000, 25000",
			got[0].RawWeighting(), got[1].RawWeighting())
	}
	if got[0].Value() != "Royaume-Uni" || got[1].Value() != "France" {
		t.Fatalf("names = %q, %q", got[0].Value(), got[1].Value())
	}
}

// Gap 3: equal-weight tie-break falls back to translated-name order.
func TestEqualWeightTieBreak(t *testing.T) {
	// FR and DE with identical weights; in French, Allemagne (DE) sorts before
	// France (FR), so DE must lead despite FR being listed first.
	geo := weighted("FR", 30000, "DE", 30000)
	p := fullPipeline(t, geo, geo)
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr_FR"})

	data, _ := Countries(fd)
	codes := data.CountryCodesGeographicalAll()
	names := data.CountryNamesGeographicalAllTranslated()
	if codes[0] != "DE" || names[0] != "Allemagne" {
		t.Fatalf("first = (%q, %q), want (DE, Allemagne)", codes[0], names[0])
	}
	if codes[1] != "FR" || names[1] != "France" {
		t.Fatalf("second = (%q, %q), want (FR, France)", codes[1], names[1])
	}
}

// Gap 4: an unknown locale falls back to English with every property populated.
func TestUnknownLocaleFallsBackToEnglish(t *testing.T) {
	p := fullPipeline(t, weightify("FR"), weightify("FR"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "zz-ZZ"})

	data, _ := Countries(fd)
	if v := data.CountryNamesGeographicalTranslated()[0].Value(); v != "France" {
		t.Fatalf("translated = %q, want France (English)", v)
	}
	if data.SortingCultureUsed() != "" {
		t.Fatalf("culture = %q, want empty", data.SortingCultureUsed())
	}
	names := data.CountryNamesGeographicalAllTranslated()
	codes := data.CountryCodesGeographicalAll()
	if len(names) <= 200 || len(codes) != len(names) {
		t.Fatalf("expected populated aligned lists, got %d/%d", len(codes), len(names))
	}
}

// Gap 5: a known English name absent from a locale map is returned unchanged.
func TestMissingSingleNameStaysEnglish(t *testing.T) {
	// Custom fr_FR table that omits "France"; the shipped data is complete, so a
	// fixture is used. Engine 1 is real (GB/FR -> English names).
	langs := pltranslation.NewLanguages(pltranslation.BuildLookups(
		map[string]string{"countries.fr_FR.yml": "United Kingdom: Royaume-Uni"}))
	allCountries := []countryCodeName{
		{code: "GB", englishName: "United Kingdom"},
		{code: "FR", englishName: "France"},
	}
	engine2 := newCountriesTranslationEngine(nil, langs, allCountries)

	p, err := core.NewPipelineBuilder().
		AddFlowElement(newFakeIPElement(weightify("GB", "FR"), weightify("GB", "FR"))).
		AddFlowElement(codeEngine(t)).
		AddFlowElement(engine2).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr_FR"})

	data, _ := Countries(fd)
	got := data.CountryNamesGeographicalTranslated()
	if got[0].Value() != "Royaume-Uni" {
		t.Fatalf("GB translated = %q, want Royaume-Uni", got[0].Value())
	}
	if got[1].Value() != "France" {
		t.Fatalf("FR (missing in map) = %q, want France kept", got[1].Value())
	}
}

// Gap 6: every position in the "All" lists is code/name aligned, including the
// alphabetical tail.
func TestFullIndexAlignment(t *testing.T) {
	p := fullPipeline(t, weightify("GB", "FR", "DE"), weightify("US", "CN"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr_FR"})

	data, _ := Countries(fd)
	table := localeTable(t, "fr_fr")
	assertAligned(t, data.CountryCodesGeographicalAll(),
		data.CountryNamesGeographicalAllTranslated(), table)
	assertAligned(t, data.CountryCodesPopulationAll(),
		data.CountryNamesPopulationAllTranslated(), table)
}

// Gap 7: SortingCultureUsed reflects the resolved culture, empty for English.
func TestSortingCultureUsed(t *testing.T) {
	cases := []struct {
		lang, want string
	}{
		{"fr_FR", "fr-FR"},
		{"de_DE", "de-DE"},
		{"es", "es-ES"},
		{"en-US,en;q=0.9", ""},
		{"zz-ZZ", ""},
	}
	for _, tc := range cases {
		p := fullPipeline(t, weightify("GB"), weightify("GB"))
		fd := processWith(t, p, map[string]any{
			pltranslation.EvidenceAcceptLanguage: tc.lang})
		data, _ := Countries(fd)
		if got := data.SortingCultureUsed(); got != tc.want {
			p.Close()
			t.Fatalf("lang %q: culture = %q, want %q", tc.lang, got, tc.want)
		}
		p.Close()
	}
}

// Gap 8: the population "All" list carries translated names (not codes) and can
// differ from the geographical list.
func TestPopulationTranslatedNames(t *testing.T) {
	p := fullPipeline(t, weightify("DE"), weightify("US", "CN"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr_FR"})

	data, _ := Countries(fd)
	table := localeTable(t, "fr_fr")
	popNames := data.CountryNamesPopulationAllTranslated()
	geoNames := data.CountryNamesGeographicalAllTranslated()

	wantUS := table["United States"]
	if popNames[0] != wantUS || popNames[0] == "US" {
		t.Fatalf("pop[0] = %q, want translated %q", popNames[0], wantUS)
	}
	if geoNames[0] != table["Germany"] {
		t.Fatalf("geo[0] = %q, want %q", geoNames[0], table["Germany"])
	}
	if popNames[0] == geoNames[0] {
		t.Fatalf("population and geographical lists should differ at position 0")
	}
}
