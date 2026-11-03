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

// Package translation adds the on-premise country translation flow elements to
// the IP Intelligence pipeline. The IP Intelligence engine emits weighted lists
// of ISO country codes; these elements turn those codes into localized country
// names and produce complete ordered lists so a demo can render a dropdown with
// the most probable country first.
//
// The pipeline is built in three stages:
//
//	IP Intelligence on-premise engine (via IpiEngineElement)
//	  -> CountryCodeTranslationEngine  (code -> English name)
//	  -> CountriesTranslationEngine    (English name -> browser language + All lists)
//
// This mirrors the .NET FiftyOne.IpIntelligence.Translation package and reuses
// the generic translation behaviour in pipeline-go's elements/translation.
package translation

// Element-data keys used by the country translation flow elements.
const (
	// IPIElementDataKey is the key under which IpiEngineElement stores the
	// weighted country-code lists read from the IP Intelligence engine. It is
	// also the source key for the country-code translation engine.
	IPIElementDataKey = "ip-intelligence"

	// CountryNamesKey is the key used by CountryCodeTranslationEngine
	// (Engine 1); it holds the weighted English country names.
	CountryNamesKey = "countrynames"

	// CountryNamesTranslatedKey is the key used by CountriesTranslationEngine
	// (Engine 2); it holds the translated weighted names and the ordered "All"
	// lists that the dropdown binds to.
	CountryNamesTranslatedKey = "countrynamestranslated"
)

// Evidence keys carrying the client IP address for the IP Intelligence engine,
// in order of precedence.
const (
	EvidenceQueryClientIP  = "query.client-ip"
	EvidenceServerClientIP = "server.client-ip"
)

// Source Property names read from the IP Intelligence element data.
const (
	PropertyCountryCodesGeographical = "CountryCodesGeographical"
	PropertyCountryCodesPopulation   = "CountryCodesPopulation"
)

// Property names produced by CountryCodeTranslationEngine (Engine 1): weighted
// English country names.
const (
	PropertyCountryNamesGeographical = "CountryNamesGeographical"
	PropertyCountryNamesPopulation   = "CountryNamesPopulation"
)

// Property names produced by CountriesTranslationEngine (Engine 2).
const (
	// Weighted, localized country names.
	PropertyCountryNamesGeographicalTranslated = "CountryNamesGeographicalTranslated"
	PropertyCountryNamesPopulationTranslated   = "CountryNamesPopulationTranslated"

	// Complete ordered lists of localized names (the dropdown labels).
	PropertyCountryNamesGeographicalAllTranslated = "CountryNamesGeographicalAllTranslated"
	PropertyCountryNamesPopulationAllTranslated   = "CountryNamesPopulationAllTranslated"

	// Complete ordered lists of codes, index-aligned with the names above (the
	// dropdown values).
	PropertyCountryCodesGeographicalAll = "CountryCodesGeographicalAll"
	PropertyCountryCodesPopulationAll   = "CountryCodesPopulationAll"

	// SortingCultureUsed reports the culture used to sort the "All" lists (for
	// tests); empty when names stay English or an invariant sort is used.
	PropertySortingCultureUsed = "SortingCultureUsed"
)
