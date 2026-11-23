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

	pltranslation "github.com/51Degrees/51degrees-go/pipeline/v4/elements/translation"
)

// CountryCodeTranslationEngineBuilder builds the Translation Engine that
// turns the weighted country codes of the IP Intelligence Element Data into
// English country names. It needs no configuration, because the country code
// source is embedded and the language is fixed as English.
type CountryCodeTranslationEngineBuilder struct {
	logger *slog.Logger
}

// NewCountryCodeTranslationEngineBuilder creates the builder.
func NewCountryCodeTranslationEngineBuilder(
	logger *slog.Logger) *CountryCodeTranslationEngineBuilder {
	return &CountryCodeTranslationEngineBuilder{logger: logger}
}

// Build gives the Pipeline's Translation Engine reading the codes under
// IPIElementDataKey. It writes PropertyCountryNamesGeographical and
// PropertyCountryNamesPopulation into the Translation Element Data, under
// pltranslation.DataKey, each a list of names in the order of the codes. A
// code with no name, such as the engine's "Unknown", stays as it is.
func (b *CountryCodeTranslationEngineBuilder) Build() (*pltranslation.Engine, error) {
	sources, err := countryCodeSources()
	if err != nil {
		return nil, err
	}
	builder := pltranslation.NewBuilder().
		SetLogger(b.logger).
		SetSourceElementDataKey(IPIElementDataKey).
		AddTranslation(PropertyCountryCodesGeographical, PropertyCountryNamesGeographical).
		AddTranslation(PropertyCountryCodesPopulation, PropertyCountryNamesPopulation).
		SetFixedLanguage("en_GB").
		SetMissingTranslationBehavior(pltranslation.Original)
	for name, content := range sources {
		builder.AddSourceContent(name, content)
	}
	return builder.Build()
}
