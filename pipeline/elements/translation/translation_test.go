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
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// ipEngine is a fake source element producing values to be translated.
type ipEngine struct{ *core.BaseElement }

func newIPEngine() *ipEngine {
	e := &ipEngine{BaseElement: core.NewBaseElement("ip", core.NewWhitelistFilter(), nil)}
	return e
}

func (e *ipEngine) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("ip", func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	d.Set("Country", "England")
	d.Set("Countries", []string{"England", "Scotland"})
	return nil
}

func buildPipeline(t *testing.T, engine *Engine) core.Pipeline {
	t.Helper()
	p, err := core.NewPipelineBuilder().AddFlowElement(newIPEngine()).AddFlowElement(engine).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return p
}

func frEngine(t *testing.T, behavior MissingBehavior, fixed string) *Engine {
	t.Helper()
	b := NewBuilder().
		SetSourceElementDataKey("ip").
		AddSourceContent("countries.fr_FR.yml", "England: Angleterre\nScotland: Ecosse").
		AddTranslation("Country", "CountryTranslated").
		AddTranslation("Countries", "CountriesTranslated").
		SetMissingTranslationBehavior(behavior)
	if fixed != "" {
		b.SetFixedLanguage(fixed)
	}
	e, err := b.Build()
	if err != nil {
		t.Fatalf("build engine: %v", err)
	}
	return e
}

func TestTranslateFixedLanguage(t *testing.T) {
	p := buildPipeline(t, frEngine(t, Original, "fr_FR"))
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	td := data.(*Data)
	if v, _ := GetAs[string](td, "CountryTranslated"); v != "Angleterre" {
		t.Fatalf("CountryTranslated = %q", v)
	}
	list, err := GetAs[[]string](td, "CountriesTranslated")
	if err != nil || len(list) != 2 || list[0] != "Angleterre" || list[1] != "Ecosse" {
		t.Fatalf("CountriesTranslated = %v (%v)", list, err)
	}
}

func TestTranslateFromEvidenceHyphenated(t *testing.T) {
	p := buildPipeline(t, frEngine(t, Original, ""))
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence(EvidenceTranslation, "fr-FR") // hyphenated form
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	if v, _ := GetAs[string](data.(*Data), "CountryTranslated"); v != "Angleterre" {
		t.Fatalf("expected Angleterre, got %q", v)
	}
}

func TestTranslateTwoLetterFallback(t *testing.T) {
	p := buildPipeline(t, frEngine(t, Original, ""))
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence(EvidenceAcceptLanguage, "fr;q=0.9, en;q=0.8")
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	if v, _ := GetAs[string](data.(*Data), "CountryTranslated"); v != "Angleterre" {
		t.Fatalf("two-letter fallback failed, got %q", v)
	}
}

func TestAcceptLanguageSecondCandidateWins(t *testing.T) {
	// The first Accept-Language candidate has no table; the engine must fall
	// through to the next candidate in q-order rather than giving up.
	p := buildPipeline(t, frEngine(t, Original, ""))
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence(EvidenceAcceptLanguage, "de-DE;q=0.9, fr-FR;q=0.8")
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	if v, _ := GetAs[string](data.(*Data), "CountryTranslated"); v != "Angleterre" {
		t.Fatalf("expected fallback to fr_FR translation, got %q", v)
	}
}

func TestMissingSourcePropertyStillWritesDestination(t *testing.T) {
	// A translation whose source property is absent must still produce the
	// destination Property (present, empty), not omit it.
	e, err := NewBuilder().
		SetSourceElementDataKey("ip").
		AddSourceContent("countries.en_GB.yml", "England: England").
		AddTranslation("DoesNotExist", "Translated").
		SetFixedLanguage("en_GB").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p := buildPipeline(t, e)
	defer p.Close()
	fd := p.CreateFlowData()
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	if _, err := data.Get("Translated"); err != nil {
		t.Fatalf("destination property should be present even when source is missing: %v", err)
	}
}

func TestMissingBehaviorOriginal(t *testing.T) {
	// No matching language => no table => original preserved.
	p := buildPipeline(t, frEngine(t, Original, "de_DE"))
	defer p.Close()
	fd := p.CreateFlowData()
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	if v, _ := GetAs[string](data.(*Data), "CountryTranslated"); v != "England" {
		t.Fatalf("Original behavior should keep value, got %q", v)
	}
}

func TestMissingBehaviorEmptyString(t *testing.T) {
	p := buildPipeline(t, frEngine(t, EmptyString, "de_DE"))
	defer p.Close()
	fd := p.CreateFlowData()
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	if v, _ := GetAs[string](data.(*Data), "CountryTranslated"); v != "" {
		t.Fatalf("EmptyString behavior should blank value, got %q", v)
	}
}

func TestMissingBehaviorFlowError(t *testing.T) {
	p := buildPipeline(t, frEngine(t, FlowError, "de_DE"))
	defer p.Close()
	fd := p.CreateFlowData()
	// Suppress so Process returns nil but error is collected; here exceptions are
	// not suppressed so Process returns the error.
	err := fd.Process()
	if err == nil {
		t.Fatal("expected a flow error for missing translation")
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	got := parseAcceptLanguage("en-US,en;q=0.9,fr;q=0.95")
	if len(got) != 3 || got[0] != "en-US" || got[1] != "fr" {
		t.Fatalf("unexpected order: %v", got)
	}
}

func TestLocaleFromName(t *testing.T) {
	cases := map[string]string{
		"countries.en_GB.yml":     "en_gb",
		"countryCodes.es_ES.yaml": "es_es",
		"foo.fr-FR.yml":           "fr_fr",
		"en_GB.yaml":              "en_gb", // bare-locale form from the spec example
		"fr.yml":                  "fr",    // two-letter language only
		"countries.yml":           "",      // no locale segment -> dropped
	}
	for name, want := range cases {
		if got := localeFromName(name); got != want {
			t.Fatalf("localeFromName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestBareLocaleSourceFile(t *testing.T) {
	// A source file named "<locale>.yaml" (no "[x]." prefix), as used in the
	// specification's own example, must load correctly.
	e, err := NewBuilder().
		SetSourceElementDataKey("ip").
		AddSourceContent("en_GB.yaml", "England: England (UK)").
		AddTranslation("Country", "CountryTranslated").
		SetFixedLanguage("en_GB").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p := buildPipeline(t, e)
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, _ := fd.Get(DataKey)
	if v, _ := GetAs[string](data.(*Data), "CountryTranslated"); v != "England (UK)" {
		t.Fatalf("bare-locale source not loaded, got %q", v)
	}
}
