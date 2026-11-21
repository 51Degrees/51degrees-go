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

// Package translation adds the country translation Flow Elements to an IP
// Intelligence Pipeline. The IP Intelligence engines give weighted lists of
// ISO country codes, and these elements turn the codes into country names in
// the request's language, with complete ordered lists so that a page can show
// every country with the most probable first.
//
// A Pipeline holds three elements, in this order:
//
//	IP Intelligence engine           (ipi_onpremise or ipi_cloud, key "ip")
//	  -> CountryCodeTranslationEngine (code -> English name)
//	  -> CountriesTranslationEngine   (English name -> request language, and
//	                                   the complete lists)
//
// CountryCodeTranslationEngine is the Pipeline's Translation Engine with the
// country code data, so it writes its names into the shared Translation
// Element Data. CountriesTranslationEngine has Element Data of its own.
//
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/translation-engine.md
package translation

// Element Data keys the country translation elements read and write.
const (
	// IPIElementDataKey is the Element Data key of the IP Intelligence
	// engines, ipi_onpremise.DataKey and ipi_cloud.DataKey. The country
	// translation elements read the weighted country codes from it.
	IPIElementDataKey = "ip"

	// CountryNamesTranslatedKey is the key of CountriesTranslationEngine's
	// Element Data, which holds the translated weighted names and the
	// complete lists.
	CountryNamesTranslatedKey = "countrynamestranslated"
)

// Properties read from the IP Intelligence Element Data.
const (
	PropertyCountryCodesGeographical = "CountryCodesGeographical"
	PropertyCountryCodesPopulation   = "CountryCodesPopulation"
)

// Properties CountryCodeTranslationEngine writes into the Translation Element
// Data, each a list of English country names in the order of the codes.
const (
	PropertyCountryNamesGeographical = "CountryNamesGeographical"
	PropertyCountryNamesPopulation   = "CountryNamesPopulation"
)

// Properties CountriesTranslationEngine writes into its own Element Data.
const (
	// Weighted country names in the request's language, each with the
	// weighting of its code.
	PropertyCountryNamesGeographicalTranslated = "CountryNamesGeographicalTranslated"
	PropertyCountryNamesPopulationTranslated   = "CountryNamesPopulationTranslated"

	// Every known country's name in the request's language, the weighted
	// countries first.
	PropertyCountryNamesGeographicalAllTranslated = "CountryNamesGeographicalAllTranslated"
	PropertyCountryNamesPopulationAllTranslated   = "CountryNamesPopulationAllTranslated"

	// The codes of the lists above, in the same order.
	PropertyCountryCodesGeographicalAll = "CountryCodesGeographicalAll"
	PropertyCountryCodesPopulationAll   = "CountryCodesPopulationAll"

	// SortingCultureUsed is the culture the complete lists are sorted in,
	// such as "fr-FR". It is empty when the names stay in English, which
	// are sorted ignoring case in no particular culture.
	PropertySortingCultureUsed = "SortingCultureUsed"
)
