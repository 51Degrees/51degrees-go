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

package common

import (
	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_onpremise"
)

// NeedsEnterprise is shown in place of the value of a property that the data
// file in use does not have, because only the Enterprise data file has it.
const NeedsEnterprise = "needs the Enterprise data file"

// EnterpriseNote says where to ask for a copy of the Enterprise data file,
// with the address of the contact page in place of %s.
const EnterpriseNote = "Some of these properties need the Enterprise data " +
	"file. Contact us for a copy at %s"

// HasDeviceProperty reports whether the Device Detection data file has the
// property, which the engine must have been asked for.
func HasDeviceProperty(results *dd.ResultsHash, property string) bool {
	return results.RequiredPropertyIndexFromName(property) >= 0
}

// FreeIpProperties are the IP Intelligence properties the free Lite data file
// has values for. It lists the other properties too, but with no values, so
// only the Enterprise data file can give them.
var FreeIpProperties = map[string]bool{
	"RegisteredCountry": true,
	"RegisteredName":    true,
	"RegisteredOwner":   true,
}

// probeAddress has a range and a location in the Enterprise data file.
const probeAddress = "185.28.167.77"

// IsFreeIpData reports whether the IP Intelligence engine's data file is a
// free one. It is when none of the properties but the free ones has a value
// for an address that the Enterprise data file has values for.
func IsFreeIpData(engine *ipi_onpremise.Engine, properties []string) bool {
	values, err := engine.Process(probeAddress)
	if err != nil {
		return false
	}
	for _, property := range properties {
		if FreeIpProperties[property] {
			continue
		}
		if _, found := values.GetValueByProperty(property); found {
			return false
		}
	}
	return true
}
