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
)

// translationFS embeds the translation data, a copy of
// cxx/data/ip-intelligence/Translations that ci/update-translations.ps1
// makes. A Go module holds only its own files, so the data is part of the
// package. countrycodes.en_GB.yml maps an ISO code to an English name, and
// each OSM/countries.<locale>.yml maps an English name to the name in that
// locale.
//
//go:embed Translations
var translationFS embed.FS

// translationRoot is the embedded path of the Translations folder.
const translationRoot = "Translations"

// countryCodesFile is the name of the country code source.
const countryCodesFile = "countrycodes.en_GB.yml"

// countryCodeSources gives the country code source, keyed by its file name,
// which names its locale.
func countryCodeSources() (map[string]string, error) {
	b, err := translationFS.ReadFile(translationRoot + "/" + countryCodesFile)
	if err != nil {
		return nil, err
	}
	return map[string]string{countryCodesFile: string(b)}, nil
}

// countriesSources gives the country name sources, keyed by file name, each
// of which names its locale.
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

// loadAllCountries gives every code and English name in the country code
// source. The order does not matter, because the complete lists sort the
// countries they add.
func loadAllCountries() ([]countryCodeName, error) {
	sources, err := countryCodeSources()
	if err != nil {
		return nil, err
	}
	table := parseFlatYAML(sources[countryCodesFile])
	out := make([]countryCodeName, 0, len(table))
	for code, name := range table {
		out = append(out, countryCodeName{code: code, englishName: name})
	}
	return out, nil
}
