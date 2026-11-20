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
	"reflect"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// queryAcceptLanguage is the second Evidence key the specification lists.
const queryAcceptLanguage = "query.accept-language"

// behaviors are the three MissingTranslationBehavior values by name.
var behaviors = map[string]MissingBehavior{
	"Original": Original, "EmptyString": EmptyString, "FlowError": FlowError,
}

// frenchAndGerman are two sources for the same values.
var frenchAndGerman = map[string]string{
	"countries.fr_FR.yml": "England: Angleterre\nScotland: Ecosse",
	"countries.de_DE.yml": "England: England\nScotland: Schottland",
}

// valuesEngine is a source element that adds the given Property values under
// the key "ip".
type valuesEngine struct {
	*core.BaseElement
	values map[string]any
}

func newValuesEngine(values map[string]any) *valuesEngine {
	return &valuesEngine{
		BaseElement: core.NewBaseElement(
			"ip", core.NewWhitelistFilter(), nil),
		values: values,
	}
}

func (e *valuesEngine) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("ip", func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	for k, v := range e.values {
		d.Set(k, v)
	}
	return nil
}

// newEngine builds an Engine that reads from "ip" with the given sources and
// translations, each translation a source and destination pair.
func newEngine(t *testing.T, behavior MissingBehavior, fixed string,
	sources map[string]string, translations ...string) *Engine {
	t.Helper()
	b := NewBuilder().
		SetSourceElementDataKey("ip").
		SetMissingTranslationBehavior(behavior)
	for name, content := range sources {
		b.AddSourceContent(name, content)
	}
	for i := 0; i+1 < len(translations); i += 2 {
		b.AddTranslation(translations[i], translations[i+1])
	}
	if fixed != "" {
		b.SetFixedLanguage(fixed)
	}
	e, err := b.Build()
	if err != nil {
		t.Fatalf("build engine: %v", err)
	}
	return e
}

// translate processes a Flow Data with the given Evidence through a Pipeline
// of source then engine. It gives the Translation Data and the message of
// every error the Flow Data holds.
func translate(t *testing.T, source core.FlowElement, engine *Engine,
	evidence map[string]string) (*Data, []string) {
	t.Helper()
	p, err := core.NewPipelineBuilder().
		AddFlowElement(source).
		AddFlowElement(engine).
		Build()
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	fd := p.CreateFlowData()
	for k, v := range evidence {
		fd.AddEvidence(k, v)
	}
	_ = fd.Process()
	var errs []string
	for _, fe := range fd.Errors() {
		errs = append(errs, fe.Err.Error())
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("the Flow Data holds no %q Element Data: %v (errors %q)",
			DataKey, err, errs)
	}
	return data.(*Data), errs
}

// wantValue fails t unless the Property name holds want, of want's type.
func wantValue(t *testing.T, d *Data, name string, want any) {
	t.Helper()
	got, err := d.Get(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v (%T), want %#v (%T)", name, got, got, want, want)
	}
}

// wantErrors fails t unless there is one error for each of the given texts,
// each naming its text.
func wantErrors(t *testing.T, errs []string, texts ...string) {
	t.Helper()
	if len(errs) != len(texts) {
		t.Fatalf("errors %q, want %d naming %q", errs, len(texts), texts)
	}
	for i, text := range texts {
		if !strings.Contains(errs[i], text) {
			t.Fatalf("error %d is %q, want it to name %q", i, errs[i], text)
		}
	}
}

// The specification lists three Evidence keys in order, query.translation,
// query.accept-language and header.accept-language, and a value in any of
// them is read as an Accept-Language list, the highest preference first.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/translation-engine.md?plain=1#L84-L95
func TestTheEvidenceKeysAreReadInTheSpecificationsOrder(t *testing.T) {
	cases := []struct {
		name     string
		evidence map[string]string
		want     string
	}{
		{"query.translation first", map[string]string{
			EvidenceTranslation:    "de-DE",
			queryAcceptLanguage:    "fr-FR",
			EvidenceAcceptLanguage: "fr-FR",
		}, "Schottland"},
		{"query.accept-language before the header", map[string]string{
			queryAcceptLanguage:    "de-DE",
			EvidenceAcceptLanguage: "fr-FR",
		}, "Schottland"},
		{"query.accept-language alone", map[string]string{
			queryAcceptLanguage: "de",
		}, "Schottland"},
		{"query.translation as a list", map[string]string{
			EvidenceTranslation: "de-DE;q=0.8, fr-FR;q=0.9",
		}, "Ecosse"},
		{"header.accept-language alone", map[string]string{
			EvidenceAcceptLanguage: "fr-FR",
		}, "Ecosse"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := newValuesEngine(map[string]any{"Country": "Scotland"})
			engine := newEngine(t, Original, "", frenchAndGerman,
				"Country", "CountryTranslated")
			d, errs := translate(t, source, engine, c.evidence)
			wantErrors(t, errs)
			wantValue(t, d, "CountryTranslated", c.want)
		})
	}
}

// Evidence "en" or "en-US" reaches a source keyed "en_GB", because the
// lookups are keyed on the language in the file names and match each form
// of the Evidence. The keys of this source are country codes, not English.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/translation-engine.md?plain=1#L77-L78
// and https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/translation-engine.md?plain=1#L90-L93
// and https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/translation-engine.md?plain=1#L113
func TestEnglishEvidenceReachesAnEnglishSource(t *testing.T) {
	codes := map[string]string{"countrycodes.en_GB.yml": "GB: United Kingdom"}
	for bname, behavior := range behaviors {
		for _, key := range []string{EvidenceTranslation, EvidenceAcceptLanguage} {
			for _, lang := range []string{"en", "en-US", "en-GB", "en_GB"} {
				name := bname + "/" + key + "=" + lang
				t.Run(name, func(t *testing.T) {
					source := newValuesEngine(map[string]any{
						"CountryCode": "GB"})
					engine := newEngine(t, behavior, "", codes,
						"CountryCode", "CountryName")
					d, errs := translate(t, source, engine,
						map[string]string{key: lang})
					wantErrors(t, errs)
					wantValue(t, d, "CountryName", "United Kingdom")
				})
			}
		}
	}
}

// A language of one letter is not a language, so it reaches no source.
func TestAOneLetterLanguageReachesNoSource(t *testing.T) {
	source := newValuesEngine(map[string]any{"Country": "Scotland"})
	engine := newEngine(t, Original, "", frenchAndGerman,
		"Country", "CountryTranslated")
	d, errs := translate(t, source, engine,
		map[string]string{EvidenceTranslation: "f"})
	wantErrors(t, errs)
	wantValue(t, d, "CountryTranslated", "Scotland")
}

// A Translation Engine builds with no translation, with no source, and with
// a source whose name holds no locale, which it leaves out.
// It still needs a SourceElementDataKey.
func TestBuildAcceptsWhatTheReleaseAccepted(t *testing.T) {
	builds := map[string]*Builder{
		"no translation": NewBuilder().
			SetSourceElementDataKey("ip").
			AddSourceContent("countries.fr_FR.yml", "England: Angleterre"),
		"no source": NewBuilder().
			SetSourceElementDataKey("ip").
			AddTranslation("Country", "CountryTranslated"),
		"a source name with no locale": NewBuilder().
			SetSourceElementDataKey("ip").
			AddSourceContent("countries.yml", "England: Angleterre").
			AddTranslation("Country", "CountryTranslated"),
	}
	for name, b := range builds {
		t.Run(name, func(t *testing.T) {
			if _, err := b.Build(); err != nil {
				t.Fatalf("Build: %v", err)
			}
		})
	}
	t.Run("no SourceElementDataKey", func(t *testing.T) {
		_, err := NewBuilder().
			AddSourceContent("countries.fr_FR.yml", "England: Angleterre").
			AddTranslation("Country", "CountryTranslated").
			Build()
		if err == nil {
			t.Fatal("Build gave no error with no SourceElementDataKey")
		}
	})
	t.Run("the source with no locale is left out", func(t *testing.T) {
		source := newValuesEngine(map[string]any{"Country": "England"})
		engine := newEngine(t, Original, "fr_FR", map[string]string{
			"countries.fr_FR.yml": "England: Angleterre",
			"README.yml":          "England: Wrong",
		}, "Country", "CountryTranslated")
		d, errs := translate(t, source, engine, nil)
		wantErrors(t, errs)
		wantValue(t, d, "CountryTranslated", "Angleterre")
	})
}

// A weighted string gives a string, and a list of weighted strings gives a
// list of strings.
func TestWeightedValuesGiveStrings(t *testing.T) {
	source := newValuesEngine(map[string]any{
		"Country": core.NewWeightedValue("England", 30000),
		"Countries": []core.WeightedValue[string]{
			core.NewWeightedValue("England", 30000),
			core.NewWeightedValue("Scotland", 20000),
		},
	})
	engine := newEngine(t, Original, "fr_FR", frenchAndGerman,
		"Country", "CountryTranslated", "Countries", "CountriesTranslated")
	d, errs := translate(t, source, engine, nil)
	wantErrors(t, errs)
	wantValue(t, d, "CountryTranslated", "Angleterre")
	wantValue(t, d, "CountriesTranslated", []string{"Angleterre", "Ecosse"})
}

// A value in an AspectPropertyValue gives its translation unwrapped.
func TestAnAspectPropertyValueGivesThePlainValue(t *testing.T) {
	source := newValuesEngine(map[string]any{
		"Country": engines.NewAspectPropertyValue("England"),
		"Countries": engines.NewAspectPropertyValue(
			[]string{"England", "Scotland"}),
	})
	engine := newEngine(t, Original, "fr_FR", frenchAndGerman,
		"Country", "CountryTranslated", "Countries", "CountriesTranslated")
	d, errs := translate(t, source, engine, nil)
	wantErrors(t, errs)
	wantValue(t, d, "CountryTranslated", "Angleterre")
	wantValue(t, d, "CountriesTranslated", []string{"Angleterre", "Ecosse"})
}

// A value of a type the specification does not list is copied through with
// no error.
func TestAnUnlistedValueTypeIsCopiedThrough(t *testing.T) {
	source := newValuesEngine(map[string]any{"Population": 56000000})
	engine := newEngine(t, Original, "fr_FR", frenchAndGerman,
		"Population", "PopulationTranslated")
	d, errs := translate(t, source, engine, nil)
	wantErrors(t, errs)
	wantValue(t, d, "PopulationTranslated", 56000000)
}

// A value is looked up with its case as given, and a translation that is
// empty in the source is used.
func TestTheLookupUsesTheValueAsGiven(t *testing.T) {
	source := newValuesEngine(map[string]any{
		"Country": "England", "Other": "Scotland"})
	engine := newEngine(t, Original, "fr_FR", map[string]string{
		"countries.fr_FR.yml": "england: Angleterre\nScotland: ''",
	}, "Country", "CountryTranslated", "Other", "OtherTranslated")
	d, errs := translate(t, source, engine, nil)
	wantErrors(t, errs)
	wantValue(t, d, "CountryTranslated", "England")
	wantValue(t, d, "OtherTranslated", "")
}

// A fixed language is used as it is given, even one that names no source,
// and the Evidence is not read.
func TestAFixedLanguageIsUsedAsGiven(t *testing.T) {
	source := newValuesEngine(map[string]any{"Country": "England"})
	engine := newEngine(t, Original, "french", frenchAndGerman,
		"Country", "CountryTranslated")
	d, errs := translate(t, source, engine,
		map[string]string{EvidenceTranslation: "fr-FR"})
	wantErrors(t, errs)
	wantValue(t, d, "CountryTranslated", "England")
}

// Under FlowError a value with no translation is kept, and an error names
// it.
func TestFlowErrorKeepsAValueWithNoTranslation(t *testing.T) {
	source := newValuesEngine(map[string]any{
		"Countries": []string{"England", "Wales"}})
	engine := newEngine(t, FlowError, "fr_FR", frenchAndGerman,
		"Countries", "CountriesTranslated")
	d, errs := translate(t, source, engine, nil)
	wantErrors(t, errs, "'Wales'")
	wantValue(t, d, "CountriesTranslated", []string{"Angleterre", "Wales"})
}

// Under FlowError a request that names no language still writes every
// destination, keeping each value, with an error naming each value.
func TestFlowErrorWithNoLanguageWritesEveryDestination(t *testing.T) {
	source := newValuesEngine(map[string]any{
		"Country": "England", "Countries": []string{"Scotland"}})
	engine := newEngine(t, FlowError, "", frenchAndGerman,
		"Country", "CountryTranslated", "Countries", "CountriesTranslated")
	d, errs := translate(t, source, engine, nil)
	wantErrors(t, errs, "'England'", "'Scotland'")
	wantValue(t, d, "CountryTranslated", "England")
	wantValue(t, d, "CountriesTranslated", []string{"Scotland"})
}

// A source Property that is absent gives an empty string, with an error
// under FlowError only.
func TestAnAbsentSourcePropertyGivesAnEmptyString(t *testing.T) {
	for name, behavior := range behaviors {
		t.Run(name, func(t *testing.T) {
			source := newValuesEngine(map[string]any{"Country": "England"})
			engine := newEngine(t, behavior, "fr_FR", frenchAndGerman,
				"DoesNotExist", "Translated")
			d, errs := translate(t, source, engine, nil)
			if behavior == FlowError {
				wantErrors(t, errs, "'DoesNotExist'")
			} else {
				wantErrors(t, errs)
			}
			wantValue(t, d, "Translated", "")
		})
	}
}

// When the source element is absent every destination is still written as
// an empty string, with an error for each under FlowError only.
func TestAnAbsentSourceElementWritesEveryDestination(t *testing.T) {
	for name, behavior := range behaviors {
		t.Run(name, func(t *testing.T) {
			source := newValuesEngine(map[string]any{"Country": "England"})
			engine, err := NewBuilder().
				SetSourceElementDataKey("absent").
				AddSourceContent("countries.fr_FR.yml",
					"England: Angleterre").
				AddTranslation("Country", "CountryTranslated").
				AddTranslation("Countries", "CountriesTranslated").
				SetFixedLanguage("fr_FR").
				SetMissingTranslationBehavior(behavior).
				Build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			d, errs := translate(t, source, engine, nil)
			if behavior == FlowError {
				wantErrors(t, errs, "'Country'", "'Countries'")
			} else {
				wantErrors(t, errs)
			}
			wantValue(t, d, "CountryTranslated", "")
			wantValue(t, d, "CountriesTranslated", "")
		})
	}
}
