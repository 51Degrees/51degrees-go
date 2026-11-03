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

	pltranslation "github.com/51Degrees/pipeline-go/elements/translation"
)

// CountryCodeTranslationEngineBuilder builds Engine 1, which translates the
// weighted ISO country-code lists from IP Intelligence into weighted English
// country names. It needs no configuration: the country-code resource
// (countrycodes.en_GB.yml) is embedded and the fixed language is English.
type CountryCodeTranslationEngineBuilder struct {
	logger *slog.Logger
}

// NewCountryCodeTranslationEngineBuilder creates the Engine 1 builder.
func NewCountryCodeTranslationEngineBuilder(
	logger *slog.Logger) *CountryCodeTranslationEngineBuilder {
	return &CountryCodeTranslationEngineBuilder{logger: logger}
}

// Build constructs the country-code translation engine. It is a standard
// pipeline translation engine that reads the weighted codes from the IP
// Intelligence element data and writes weighted English names under
// CountryNamesKey.
func (b *CountryCodeTranslationEngineBuilder) Build() (*pltranslation.Engine, error) {
	sources, err := countryCodeSources()
	if err != nil {
		return nil, err
	}
	builder := pltranslation.NewBuilder().
		SetLogger(b.logger).
		SetSourceElementDataKey(IPIElementDataKey).
		SetElementDataKey(CountryNamesKey).
		AddTranslation(PropertyCountryCodesGeographical, PropertyCountryNamesGeographical).
		AddTranslation(PropertyCountryCodesPopulation, PropertyCountryNamesPopulation).
		SetFixedLanguage("en_GB").
		SetMissingTranslationBehavior(pltranslation.Original)
	for name, content := range sources {
		builder.AddSourceContent(name, content)
	}
	return builder.Build()
}
