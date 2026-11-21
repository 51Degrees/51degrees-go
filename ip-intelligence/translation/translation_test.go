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
	"strings"
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	pltranslation "github.com/51Degrees/51degrees-go/pipeline/v4/elements/translation"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// --- test doubles and helpers -------------------------------------------------

// fakeIPElement stands in for an IP Intelligence engine. It writes the given
// weighted codes under IPIElementDataKey as Aspect Property Values, as
// ipi_onpremise and ipi_cloud do, or no value at all when noValue is set.
type fakeIPElement struct {
	*core.BaseElement
	geo, pop []core.WeightedValue[string]
	noValue  bool
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
	for name, codes := range map[string][]core.WeightedValue[string]{
		PropertyCountryCodesGeographical: e.geo,
		PropertyCountryCodesPopulation:   e.pop,
	} {
		if e.noValue {
			d.Set(name, engines.NoValue[[]core.WeightedValue[string]](
				"No IP address was supplied."))
		} else {
			d.Set(name, engines.NewAspectPropertyValue(codes))
		}
	}
	return nil
}

// weightify gives descending weights by input order, so the first code is
// the most probable.
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

// weighted builds a weighted list from code and raw weighting pairs.
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
		t.Fatalf("build country code engine: %v", err)
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

func buildPipeline(t *testing.T, elements ...core.FlowElement) core.Pipeline {
	t.Helper()
	b := core.NewPipelineBuilder()
	for _, e := range elements {
		b.AddFlowElement(e)
	}
	p, err := b.Build()
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	return p
}

// codePipeline holds the IP element and the country code engine.
func codePipeline(t *testing.T, geo, pop []core.WeightedValue[string]) core.Pipeline {
	t.Helper()
	return buildPipeline(t, newFakeIPElement(geo, pop), codeEngine(t))
}

// fullPipeline holds the IP element and both country engines.
func fullPipeline(t *testing.T, geo, pop []core.WeightedValue[string]) core.Pipeline {
	t.Helper()
	return buildPipeline(t,
		newFakeIPElement(geo, pop), codeEngine(t), countriesEngine(t))
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

// allKnownMap gives each known code's English name.
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

// localeTable gives the embedded country name table of a locale.
func localeTable(t *testing.T, locale string) map[string]string {
	t.Helper()
	src, err := countriesSources()
	if err != nil {
		t.Fatalf("countries sources: %v", err)
	}
	return buildLanguages(src)[locale]
}

// comparerFromCulture gives the order a list sorted in culture has.
func comparerFromCulture(culture string) func(a, b string) int {
	c := collate.New(language.Und, collate.IgnoreCase)
	if culture != "" {
		c = collate.New(language.MustParse(culture))
	}
	return c.CompareString
}

// assertSortedTail checks names[skip:] are in order.
func assertSortedTail(t *testing.T, names []string, skip int, comparer func(a, b string) int) {
	t.Helper()
	for i := skip + 1; i < len(names); i++ {
		if comparer(names[i-1], names[i]) > 0 {
			t.Fatalf("tail not sorted at %d: %q > %q", i, names[i-1], names[i])
		}
	}
}

// assertAligned checks the name at each position is the translated name of
// the code at that position.
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
		if tr, ok := table[english]; ok {
			want = tr
		}
		if names[i] != want {
			t.Fatalf("misaligned at %d: code %q -> %q, want %q",
				i, codes[i], names[i], want)
		}
	}
}

func values(w []core.WeightedValue[string]) []string {
	out := make([]string, len(w))
	for i, v := range w {
		out[i] = v.Value()
	}
	return out
}

// --- the country code engine --------------------------------------------------

// The country code engine alone turns GB and FR into United Kingdom and France
// for both weightings, in the Translation Element Data.
func TestCountryNamesFromCodes(t *testing.T) {
	p := codePipeline(t, weightify("GB", "FR"), weightify("GB", "FR"))
	defer p.Close()
	fd := processWith(t, p, nil)

	data, err := CountryNames(fd)
	if err != nil {
		t.Fatalf("country names data: %v", err)
	}
	for _, names := range [][]string{
		data.CountryNamesGeographical(), data.CountryNamesPopulation()} {
		if strings.Join(names, ",") != "United Kingdom,France" {
			t.Fatalf("names = %q, want United Kingdom, France", names)
		}
	}
	if _, err := fd.Get(pltranslation.DataKey); err != nil {
		t.Fatalf("no Translation Element Data under %q: %v", pltranslation.DataKey, err)
	}
}

// The country code engine is the Pipeline's Translation Engine, reading the IP
// Intelligence Element Data, so it shares its Element Data with any other.
func TestCountryCodeEngineIsTheTranslationEngine(t *testing.T) {
	e := codeEngine(t)
	if e.DataKey() != pltranslation.DataKey {
		t.Fatalf("ElementDataKey = %q, want %q", e.DataKey(), pltranslation.DataKey)
	}
}

// --- the countries engine -----------------------------------------------------

// With fr_FR, GB and FR become Royaume-Uni and France, weighted.
func TestTranslatedCountry(t *testing.T) {
	p := fullPipeline(t, weightify("GB", "FR"), weightify("GB", "FR"))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr_FR"})

	data, _ := Countries(fd)
	geo := data.CountryNamesGeographicalTranslated()
	if strings.Join(values(geo), ",") != "Royaume-Uni,France" {
		t.Fatalf("translated = %q, want Royaume-Uni, France", values(geo))
	}
}

// With explicit weights (FR 30000, GB 35535) and fr_FR, GB then FR lead, every
// country is listed once, and the rest are sorted in French.
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

// As above, with weights given by input order.
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

// With no language asked for, the names stay in English, GB leads and the rest
// are sorted.
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

// In English an accented name such as "Åland Islands" sorts with the other A
// names, not after "Zimbabwe" as a byte order would put it.
func TestAccentedNamesSortLinguisticallyOnEnglishPath(t *testing.T) {
	p := fullPipeline(t, weightify("GB"), weightify("GB"))
	defer p.Close()
	fd := processWith(t, p, nil)

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
	aland := indexOf("Åland Islands")
	zimbabwe := indexOf("Zimbabwe")
	if aland < 0 || zimbabwe < 0 {
		t.Fatalf("expected both names present (aland=%d zimbabwe=%d)", aland, zimbabwe)
	}
	if aland > zimbabwe {
		t.Fatalf("accented name sorted after Zimbabwe (aland=%d zimbabwe=%d)", aland, zimbabwe)
	}
}

// With no weighted codes and de_DE, every country is listed, all sorted.
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

// When the IP Intelligence Element Data has no value, as when the Evidence
// holds no IP address, the weighted lists are empty and the complete lists
// still hold every country.
func TestAllListsWhenTheIPDataHasNoValue(t *testing.T) {
	ip := newFakeIPElement(nil, nil)
	ip.noValue = true
	p := buildPipeline(t, ip, codeEngine(t), countriesEngine(t))
	defer p.Close()
	fd := processWith(t, p, map[string]any{
		pltranslation.EvidenceAcceptLanguage: "fr_FR"})

	data, _ := Countries(fd)
	if n := len(data.CountryNamesGeographicalTranslated()); n != 0 {
		t.Fatalf("%d weighted names, want none", n)
	}
	names := data.CountryNamesGeographicalAllTranslated()
	codes := data.CountryCodesGeographicalAll()
	if len(names) <= 200 {
		t.Fatalf("expected all countries, got %d", len(names))
	}
	assertSortedTail(t, names, 0, comparerFromCulture(data.SortingCultureUsed()))
	assertAligned(t, codes, names, localeTable(t, "fr_fr"))
	if errs := fd.Errors(); len(errs) != 0 {
		t.Fatalf("errors = %v, want none", errs)
	}
}

// Geographical DE and population US, CN are listed apart.
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

// With de_DE, Deutschland is weighted and leads the complete list.
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

// fr-FR, with a hyphen, is read as fr_FR.
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

// With en-US,en;q=0.9 the names stay in English.
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

// English asked for first wins over the languages after it.
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

// With es,de-DE;q=0.8,fr;q=0.5 the names are in Spanish, as es reaches es_ES.
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

// With only the engine's "Unknown" code, the lists hold every known country,
// sorted, and no "Unknown".
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

// With DE then "Unknown", DE leads and "Unknown" is left out.
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

// query.translation comes before header.accept-language.
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

// Each translated name keeps its code's raw weighting.
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

// Two codes with the same weighting are ordered by translated name. In French
// Allemagne (DE) sorts before France (FR), so DE leads although FR is first.
func TestEqualWeightTieBreak(t *testing.T) {
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

// A language with no source leaves the names in English, with every list
// filled.
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

// A name missing from a locale's source stays in English.
func TestMissingSingleNameStaysEnglish(t *testing.T) {
	// The embedded data is complete, so a source that leaves out France is
	// given here.
	allCountries := []countryCodeName{
		{code: "GB", englishName: "United Kingdom"},
		{code: "FR", englishName: "France"},
	}
	countries := newCountriesTranslationEngine(nil,
		map[string]string{"countries.fr_FR.yml": "United Kingdom: Royaume-Uni"},
		allCountries)
	p := buildPipeline(t,
		newFakeIPElement(weightify("GB", "FR"), weightify("GB", "FR")),
		codeEngine(t), countries)
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

// Every position of the complete lists pairs a code with its name, the sorted
// part included.
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

// SortingCultureUsed is the culture chosen, and empty for English.
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
	p := fullPipeline(t, weightify("GB"), weightify("GB"))
	defer p.Close()
	for _, tc := range cases {
		fd := processWith(t, p, map[string]any{
			pltranslation.EvidenceAcceptLanguage: tc.lang})
		data, _ := Countries(fd)
		if got := data.SortingCultureUsed(); got != tc.want {
			t.Errorf("lang %q: culture = %q, want %q", tc.lang, got, tc.want)
		}
	}
}

// The population list holds translated names, which differ from the
// geographical list's.
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

// The countries engine's Element Data cannot be written once it has
// processed.
func TestCountriesDataIsReadOnlyOnceProcessed(t *testing.T) {
	p := fullPipeline(t, weightify("GB"), weightify("GB"))
	defer p.Close()
	fd := processWith(t, p, nil)
	data, err := fd.Get(CountryNamesTranslatedKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() {
		r := recover()
		err, ok := r.(error)
		if !ok || !strings.Contains(err.Error(), "finished processing") {
			t.Errorf("a write after processing panicked with %v, want an "+
				"error saying processing has finished", r)
		}
	}()
	data.Set("written-after-processing", true)
}

// Requests in different languages processed at once each get their own
// language's names and order.
func TestRequestsInSeveralLanguagesAtOnce(t *testing.T) {
	p := fullPipeline(t, weightify("GB", "DE"), weightify("US"))
	defer p.Close()
	langs := []string{"fr_FR", "de_DE", "ja", "pl", "", "sv", "tr", "cs"}
	want := map[string][]string{}
	for _, lang := range langs {
		fd := processWith(t, p, map[string]any{
			pltranslation.EvidenceAcceptLanguage: lang})
		data, _ := Countries(fd)
		want[lang] = data.CountryNamesGeographicalAllTranslated()
	}
	var wg sync.WaitGroup
	errs := make(chan string, 8*len(langs))
	for i := 0; i < 8; i++ {
		for _, lang := range langs {
			wg.Add(1)
			go func(lang string) {
				defer wg.Done()
				fd := p.CreateFlowData()
				fd.AddEvidence(pltranslation.EvidenceAcceptLanguage, lang)
				if err := fd.Process(); err != nil {
					errs <- err.Error()
					return
				}
				data, _ := Countries(fd)
				got := data.CountryNamesGeographicalAllTranslated()
				if strings.Join(got, "|") != strings.Join(want[lang], "|") {
					errs <- fmt.Sprintf("%q: the list differs from one processed alone", lang)
				}
			}(lang)
		}
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// englishSource gives a source that maps each English name to itself, as the
// countries engine's English names do.
func englishSource(all []countryCodeName) string {
	var b strings.Builder
	for _, c := range all {
		fmt.Fprintf(&b, "%q: %q\n", c.englishName, c.englishName)
	}
	return b.String()
}

// The countries engine chooses the language the Pipeline's Translation Engine
// chooses from the same sources, for each way the Evidence can ask for one.
// A Translation Engine given the countries engine's sources, the English
// names included, translates the country code engine's names, and its names
// must be the countries engine's.
func TestLanguageIsTheTranslationEngines(t *testing.T) {
	sources, err := countriesSources()
	if err != nil {
		t.Fatalf("countries sources: %v", err)
	}
	all, err := loadAllCountries()
	if err != nil {
		t.Fatalf("load all countries: %v", err)
	}
	const referenceNames = "ReferenceNames"
	b := pltranslation.NewBuilder().
		SetSourceElementDataKey(pltranslation.DataKey).
		AddTranslation(PropertyCountryNamesGeographical, referenceNames).
		AddSourceContent("countries.en_GB.yml", englishSource(all))
	for name, content := range sources {
		b.AddSourceContent(name, content)
	}
	reference, err := b.Build()
	if err != nil {
		t.Fatalf("build reference engine: %v", err)
	}
	codes := weightify("GB", "DE", "FR", "ES", "JP", "US", "CN", "IT", "PL", "HU")
	p := buildPipeline(t,
		newFakeIPElement(codes, codes),
		codeEngine(t),
		newCountriesTranslationEngine(nil, sources, all),
		reference)
	defer p.Close()

	accept := pltranslation.EvidenceAcceptLanguage
	query := pltranslation.EvidenceQueryAcceptLanguage
	fixed := pltranslation.EvidenceTranslation
	cases := []map[string]any{
		nil,
		{accept: "fr_FR"},
		{accept: "fr-FR"},
		{accept: "fr"},
		{accept: "FR-fr"},
		{accept: "es"},
		{accept: "de-AT"},
		{accept: "pt-BR"},
		{accept: "ja"},
		{accept: "en"},
		{accept: "EN-gb"},
		{accept: "en-US,en;q=0.9,de-DE;q=0.5"},
		{accept: "zz-ZZ"},
		{accept: "zz, de;q=0.5"},
		{accept: "fr;q=0.1, de;q=0.9"},
		{accept: "nl-BE;q=0.8, sv"},
		{accept: "uk, ro;q=0.9"},
		{accept: "*"},
		{accept: ""},
		{accept: "f"},
		{fixed: "de_DE", accept: "fr"},
		{fixed: "zz", accept: "fr"},
		{query: "it", accept: "fr"},
		{query: "en", accept: "hu"},
	}
	for _, evidence := range cases {
		fd := processWith(t, p, evidence)
		names, err := fd.Get(pltranslation.DataKey)
		if err != nil {
			t.Fatalf("%v: no Translation Element Data: %v", evidence, err)
		}
		raw, err := names.Get(referenceNames)
		if err != nil {
			t.Fatalf("%v: no reference names: %v", evidence, err)
		}
		want, _ := raw.([]string)
		data, _ := Countries(fd)
		got := values(data.CountryNamesGeographicalTranslated())
		if strings.Join(got, "|") != strings.Join(want, "|") || len(got) != len(codes) {
			t.Errorf("%v: names = %q, the Translation Engine gives %q",
				evidence, got, want)
		}
	}
}
