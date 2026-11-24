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

package onpremise

import (
	"net/url"
	"strings"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
)

// productPrefix starts the name the Distributor gives each product of this
// kind of data file.
const productPrefix = "V4"

// distributorProduct returns the name the Distributor gives the product of a
// data file of the data tier, which is the prefix followed by the tier. A
// tier that already starts with the prefix, whatever its case, is the name as
// it is. No tier, or a tier that is the prefix alone, gives no name.
// See https://github.com/51Degrees/specifications/blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/features/data-updates.md#product-names
func distributorProduct(tier string) string {
	tier = strings.TrimSpace(tier)
	if tier == "" || strings.EqualFold(tier, productPrefix) {
		return ""
	}
	if len(tier) >= len(productPrefix) &&
		strings.EqualFold(tier[:len(productPrefix)], productPrefix) {
		return tier
	}
	return productPrefix + tier
}

// dataFileProduct is the name the Distributor gives the product of the loaded
// data file, which an update asks for when the license key holds several
// products. It is an empty string before a data file is loaded and once the
// engine has stopped.
func (e *Engine) dataFileProduct() string {
	if !e.acquire() {
		return ""
	}
	defer e.release()
	if e.manager == nil {
		return ""
	}
	return distributorProduct(dd.GetDataSetName(e.manager))
}

// productParameter is the query parameter of a data file URL that names the
// product.
const productParameter = "Product"

// namesProduct reports whether rawQuery, as it is written, has a Product
// parameter with a value, whatever the case or escaping of its name.
func namesProduct(rawQuery string) bool {
	for _, pair := range queryPairs(rawQuery) {
		if _, value, _ := strings.Cut(pair, "="); value != "" && isProductPair(pair) {
			return true
		}
	}
	return false
}

// queryWithProduct returns rawQuery with product as its one Product
// parameter, added at its end. Any Product parameter it has, whatever the case
// or escaping of its name, is removed, and the other pairs, empty ones
// included, are kept as they are written, in their order.
func queryWithProduct(rawQuery string, product string) string {
	var kept []string
	for _, pair := range queryPairs(rawQuery) {
		if !isProductPair(pair) {
			kept = append(kept, pair)
		}
	}
	return strings.Join(append(kept, productParameter+"="+url.QueryEscape(product)), "&")
}

// queryPairs splits a query as it is written into its pairs, empty ones
// included, and gives none for an empty query.
func queryPairs(rawQuery string) []string {
	if rawQuery == "" {
		return nil
	}
	return strings.Split(rawQuery, "&")
}

// isProductPair reports whether a pair of a query as it is written is a
// Product parameter, whatever the case or escaping of its name.
func isProductPair(pair string) bool {
	name, _, _ := strings.Cut(pair, "=")
	if unescaped, err := url.QueryUnescape(name); err == nil {
		name = unescaped
	}
	return strings.EqualFold(name, productParameter)
}
