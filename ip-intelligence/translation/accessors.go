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

import "github.com/51Degrees/pipeline-go/core"

// CountryNamesData provides typed access to the CountryCodeTranslationEngine
// (Engine 1) output: weighted English country names. It mirrors the .NET
// ICountryCodeTranslationData interface.
type CountryNamesData struct{ core.ElementData }

// CountryNames returns the Engine 1 output from the Flow Data.
func CountryNames(fd core.FlowData) (CountryNamesData, error) {
	d, err := fd.Get(CountryNamesKey)
	if err != nil {
		return CountryNamesData{}, err
	}
	return CountryNamesData{ElementData: d}, nil
}

// CountryNamesGeographical returns the weighted English names for the
// geographical dimension.
func (d CountryNamesData) CountryNamesGeographical() []core.WeightedValue[string] {
	return weightedValue(d, PropertyCountryNamesGeographical)
}

// CountryNamesPopulation returns the weighted English names for the population
// dimension.
func (d CountryNamesData) CountryNamesPopulation() []core.WeightedValue[string] {
	return weightedValue(d, PropertyCountryNamesPopulation)
}

// CountriesData provides typed access to the CountriesTranslationEngine
// (Engine 2) output. It mirrors the .NET ICountriesTranslationData interface.
type CountriesData struct{ core.ElementData }

// Countries returns the Engine 2 output from the Flow Data.
func Countries(fd core.FlowData) (CountriesData, error) {
	d, err := fd.Get(CountryNamesTranslatedKey)
	if err != nil {
		return CountriesData{}, err
	}
	return CountriesData{ElementData: d}, nil
}

// CountryNamesGeographicalTranslated returns the weighted localized names for
// the geographical dimension.
func (d CountriesData) CountryNamesGeographicalTranslated() []core.WeightedValue[string] {
	return weightedValue(d, PropertyCountryNamesGeographicalTranslated)
}

// CountryNamesPopulationTranslated returns the weighted localized names for the
// population dimension.
func (d CountriesData) CountryNamesPopulationTranslated() []core.WeightedValue[string] {
	return weightedValue(d, PropertyCountryNamesPopulationTranslated)
}

// CountryNamesGeographicalAllTranslated returns the complete ordered list of
// localized names for the geographical dimension (the dropdown labels).
func (d CountriesData) CountryNamesGeographicalAllTranslated() []string {
	return stringList(d, PropertyCountryNamesGeographicalAllTranslated)
}

// CountryNamesPopulationAllTranslated returns the complete ordered list of
// localized names for the population dimension.
func (d CountriesData) CountryNamesPopulationAllTranslated() []string {
	return stringList(d, PropertyCountryNamesPopulationAllTranslated)
}

// CountryCodesGeographicalAll returns the complete ordered list of codes for the
// geographical dimension (the dropdown values), index-aligned with the names.
func (d CountriesData) CountryCodesGeographicalAll() []string {
	return stringList(d, PropertyCountryCodesGeographicalAll)
}

// CountryCodesPopulationAll returns the complete ordered list of codes for the
// population dimension, index-aligned with the names.
func (d CountriesData) CountryCodesPopulationAll() []string {
	return stringList(d, PropertyCountryCodesPopulationAll)
}

// SortingCultureUsed returns the culture used to sort the "All" lists, or an
// empty string when names stayed English or an invariant sort was used.
func (d CountriesData) SortingCultureUsed() string {
	v, err := d.Get(PropertySortingCultureUsed)
	if err != nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func weightedValue(d core.ElementData, name string) []core.WeightedValue[string] {
	v, err := d.Get(name)
	if err != nil {
		return nil
	}
	w, _ := v.([]core.WeightedValue[string])
	return w
}

func stringList(d core.ElementData, name string) []string {
	v, err := d.Get(name)
	if err != nil {
		return nil
	}
	s, _ := v.([]string)
	return s
}
