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

// Package translation adds the on-premise country translation Flow Elements to
// an IP Intelligence Pipeline. The IP Intelligence Engines give weighted lists
// of ISO country codes. These elements turn those codes into country names in
// the language the request asks for, and give complete ordered lists, so that a
// page can show a dropdown with the most probable country first.
//
// The Pipeline is built in three stages.
//
//	IP Intelligence Engine, on-premise or cloud  (Element Data "ip")
//	  -> CountryCodeTranslationEngine  (code -> English name)
//	  -> CountriesTranslationEngine    (English name -> requested language, and the All lists)
//
// This follows the .NET FiftyOne.IpIntelligence.Translation package. The first
// stage is the Pipeline module's Translation Engine with the country code data.
package translation

import pltranslation "github.com/51Degrees/51degrees-go/pipeline/v4/elements/translation"

// Element Data keys the country translation Flow Elements read and write.
const (
	// IPIElementDataKey is the key of the Element Data the IP Intelligence
	// Engines add, both ipi_onpremise.DataKey and ipi_cloud.DataKey. It holds
	// the weighted country code lists.
	IPIElementDataKey = "ip"
	// CountryNamesKey is the key CountryCodeTranslationEngine writes the
	// weighted English country names under. It is the key every Translation
	// Engine writes under, so the engine must not be in a parallel group with
	// another Translation Engine.
	CountryNamesKey = pltranslation.DataKey
	// CountryNamesTranslatedKey is the key CountriesTranslationEngine writes
	// under. It holds the translated weighted names and the ordered All lists
	// that a dropdown shows.
	CountryNamesTranslatedKey = "countrynamestranslated"
)

// Source Property names read from the IP Intelligence Element Data.
const (
	PropertyCountryCodesGeographical = "CountryCodesGeographical"
	PropertyCountryCodesPopulation   = "CountryCodesPopulation"
)

// Property names CountryCodeTranslationEngine gives, each a list of English
// country names in the order of the codes it was made from.
const (
	PropertyCountryNamesGeographical = "CountryNamesGeographical"
	PropertyCountryNamesPopulation   = "CountryNamesPopulation"
)

// Property names CountriesTranslationEngine gives.
const (
	// Weighted country names in the requested language.
	PropertyCountryNamesGeographicalTranslated = "CountryNamesGeographicalTranslated"
	PropertyCountryNamesPopulationTranslated   = "CountryNamesPopulationTranslated"
	// Complete ordered lists of names in the requested language, the labels of
	// a dropdown.
	PropertyCountryNamesGeographicalAllTranslated = "CountryNamesGeographicalAllTranslated"
	PropertyCountryNamesPopulationAllTranslated   = "CountryNamesPopulationAllTranslated"
	// Complete ordered lists of codes, in the same order as the names above,
	// the values of a dropdown.
	PropertyCountryCodesGeographicalAll = "CountryCodesGeographicalAll"
	PropertyCountryCodesPopulationAll   = "CountryCodesPopulationAll"
	// SortingCultureUsed is the culture the All lists were sorted with, for
	// tests. It is empty when the names stay English and an invariant sort is
	// used.
	PropertySortingCultureUsed = "SortingCultureUsed"
)
