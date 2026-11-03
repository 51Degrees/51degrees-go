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

// weightedIPEngine is a fake source element producing a weighted list of values.
type weightedIPEngine struct{ *core.BaseElement }

func newWeightedIPEngine() *weightedIPEngine {
	return &weightedIPEngine{
		BaseElement: core.NewBaseElement("ip", core.NewWhitelistFilter(), nil),
	}
}

func (e *weightedIPEngine) Process(fd core.FlowData) error {
	d := fd.GetOrAdd("ip", func(core.FlowData) core.ElementData {
		return core.NewBaseElementData()
	})
	d.Set("Countries", []core.WeightedValue[string]{
		core.NewWeightedValue("England", 30000),
		core.NewWeightedValue("Scotland", 20000),
	})
	return nil
}

// TestEnglishShortCircuit verifies that when English (the base language) is the
// highest-priority requested language, resolution stops there rather than
// falling through to a lower-priority language that does have a table.
func TestEnglishShortCircuit(t *testing.T) {
	p := buildPipeline(t, frEngine(t, Original, ""))
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence(EvidenceAcceptLanguage, "en-US,en;q=0.9,fr-FR;q=0.5")
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	if v, _ := GetAs[string](data.(*Data), "CountryTranslated"); v != "England" {
		t.Fatalf("English short-circuit failed, got %q (expected English kept)", v)
	}
}

// TestQueryAcceptLanguageEvidence verifies the query.accept-language evidence
// key is honoured for language selection.
func TestQueryAcceptLanguageEvidence(t *testing.T) {
	p := buildPipeline(t, frEngine(t, Original, ""))
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence(EvidenceQueryAcceptLanguage, "fr-FR")
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	if v, _ := GetAs[string](data.(*Data), "CountryTranslated"); v != "Angleterre" {
		t.Fatalf("query.accept-language not honoured, got %q", v)
	}
}

// TestQueryTranslationOverridesHeader verifies query.translation takes
// precedence over a different header.accept-language.
func TestQueryTranslationOverridesHeader(t *testing.T) {
	p := buildPipeline(t, frEngine(t, Original, ""))
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence(EvidenceTranslation, "fr_FR")
	fd.AddEvidence(EvidenceAcceptLanguage, "de-DE")
	_ = fd.Process()
	data, _ := fd.Get(DataKey)
	if v, _ := GetAs[string](data.(*Data), "CountryTranslated"); v != "Angleterre" {
		t.Fatalf("query.translation did not override header, got %q", v)
	}
}

// TestWeightedListPreservesWeights verifies that translating a weighted list
// keeps each entry's raw weighting and returns a weighted list (not a plain
// list of strings).
func TestWeightedListPreservesWeights(t *testing.T) {
	e, err := NewBuilder().
		SetSourceElementDataKey("ip").
		AddSourceContent("countries.fr_FR.yml", "England: Angleterre\nScotland: Ecosse").
		AddTranslation("Countries", "CountriesTranslated").
		SetFixedLanguage("fr_FR").
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, err := core.NewPipelineBuilder().
		AddFlowElement(newWeightedIPEngine()).
		AddFlowElement(e).
		Build()
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data, _ := fd.Get(DataKey)
	got, err := GetAs[[]core.WeightedValue[string]](data.(*Data), "CountriesTranslated")
	if err != nil {
		t.Fatalf("expected a weighted list, got error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].Value() != "Angleterre" || got[0].RawWeighting() != 30000 {
		t.Fatalf("entry 0 = (%q, %d)", got[0].Value(), got[0].RawWeighting())
	}
	if got[1].Value() != "Ecosse" || got[1].RawWeighting() != 20000 {
		t.Fatalf("entry 1 = (%q, %d)", got[1].Value(), got[1].RawWeighting())
	}
}

// TestConfigurableElementDataKey verifies an Engine can store its output under a
// distinct Element Data key instead of the shared default.
func TestConfigurableElementDataKey(t *testing.T) {
	e, err := NewBuilder().
		SetSourceElementDataKey("ip").
		SetElementDataKey("countrynames").
		AddSourceContent("countries.fr_FR.yml", "England: Angleterre").
		AddTranslation("Country", "CountryTranslated").
		SetFixedLanguage("fr_FR").
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
	if _, err := fd.Get("countrynames"); err != nil {
		t.Fatalf("expected output under 'countrynames': %v", err)
	}
	if _, err := fd.Get(DataKey); err == nil {
		t.Fatalf("did not expect output under the default '%s' key", DataKey)
	}
}
