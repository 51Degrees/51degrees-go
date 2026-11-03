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
	"embed"

	pltranslation "github.com/51Degrees/pipeline-go/elements/translation"
)

// translationFS embeds the translation YAML from the ip-intelligence-data
// submodule so the package stays a single source of truth with the data repo.
// countrycodes.en_GB.yml maps ISO code -> English name; the OSM/
// countries.<locale>.yml files map English name -> localized name.
//
// The submodule must be checked out for the build to succeed (git submodule
// update --init translation/ip-intelligence-data).
//
//go:embed ip-intelligence-data/Translations
var translationFS embed.FS

// translationRoot is the embed path prefix for the Translations folder inside
// the ip-intelligence-data submodule.
const translationRoot = "ip-intelligence-data/Translations"

// countryCodeSources returns the country-code translation source, keyed by file
// name so the locale can be derived from it.
func countryCodeSources() (map[string]string, error) {
	b, err := translationFS.ReadFile(translationRoot + "/countrycodes.en_GB.yml")
	if err != nil {
		return nil, err
	}
	return map[string]string{"countrycodes.en_GB.yml": string(b)}, nil
}

// countriesSources returns the localized country-name translation sources, keyed
// by file name so the locale can be derived from each.
func countriesSources() (map[string]string, error) {
	entries, err := translationFS.ReadDir(translationRoot + "/OSM")
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		b, err := translationFS.ReadFile(translationRoot + "/OSM/" + entry.Name())
		if err != nil {
			return nil, err
		}
		out[entry.Name()] = string(b)
	}
	return out, nil
}

// countryCodeName pairs an ISO country code with its English name.
type countryCodeName struct {
	code        string
	englishName string
}

// loadAllCountries returns every known (code, English name) pair from
// countrycodes.en_GB.yml. The order is unspecified because the "All" list
// ordering re-sorts the tail, so it does not affect output.
func loadAllCountries() ([]countryCodeName, error) {
	sources, err := countryCodeSources()
	if err != nil {
		return nil, err
	}
	table := pltranslation.BuildLookups(sources)["en_gb"]
	out := make([]countryCodeName, 0, len(table))
	for code, name := range table {
		out = append(out, countryCodeName{code: code, englishName: name})
	}
	return out, nil
}
