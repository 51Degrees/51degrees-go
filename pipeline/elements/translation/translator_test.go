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

import "testing"

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
