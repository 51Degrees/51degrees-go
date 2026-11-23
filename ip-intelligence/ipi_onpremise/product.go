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

package ipi_onpremise

import (
	"strings"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

// productPrefix starts the name the Distributor gives each product of this
// kind of data file.
const productPrefix = "IPIV4"

// distributorProduct returns the name the Distributor gives the product of a
// data file of the data tier, which is the prefix followed by the tier. A
// tier that already starts with the prefix, whatever its case, is the name as
// it is, and no tier gives no name.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/data-updates.md
func distributorProduct(tier string) string {
	tier = strings.TrimSpace(tier)
	if tier == "" {
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
	return distributorProduct(ipi_interop.GetDataSetName(e.manager))
}
