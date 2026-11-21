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
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	pltranslation "github.com/51Degrees/51degrees-go/pipeline/v4/elements/translation"
)

// CountryNamesData gives typed access to the English country names
// CountryCodeTranslationEngine writes into the Translation Element Data.
type CountryNamesData struct{ core.ElementData }

// CountryNames gives the Translation Element Data from the Flow Data.
func CountryNames(fd core.FlowData) (CountryNamesData, error) {
	d, err := fd.Get(pltranslation.DataKey)
	if err != nil {
		return CountryNamesData{}, err
	}
	return CountryNamesData{ElementData: d}, nil
}

// CountryNamesGeographical gives the English names of the geographical
// weighting, in the order of the codes.
func (d CountryNamesData) CountryNamesGeographical() []string {
	return stringList(d, PropertyCountryNamesGeographical)
}

// CountryNamesPopulation gives the English names of the population
// weighting, in the order of the codes.
func (d CountryNamesData) CountryNamesPopulation() []string {
	return stringList(d, PropertyCountryNamesPopulation)
}

// CountriesData gives typed access to CountriesTranslationEngine's Element
// Data.
type CountriesData struct{ core.ElementData }

// Countries gives CountriesTranslationEngine's Element Data from the Flow
// Data.
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
