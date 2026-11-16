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
	"fmt"
	"log"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_onpremise"
)

// NeedsEnterprise is shown in place of the value of a property that only the
// Enterprise data file has values for.
const NeedsEnterprise = "needs the Enterprise data file"

// FreeProperties are the properties the free data files have values for, the
// first three from the Lite file and the last two from the Asn file. The Lite
// file lists the other properties too, but with no values, so only the
// Enterprise data file can give them.
var FreeProperties = map[string]bool{
	"RegisteredCountry": true,
	"RegisteredName":    true,
	"RegisteredOwner":   true,
	"Asn":               true,
	"AsnName":           true,
}

// probeAddress has a range and a location in the Enterprise data file.
const probeAddress = "185.28.167.77"

// IsFreeDataFile reports whether the engine's data file is a free one. It is
// when none of the properties but the free ones has a value for an address
// that the Enterprise data file has values for.
func IsFreeDataFile(engine *ipi_onpremise.Engine, properties []string) bool {
	values, err := engine.Process(probeAddress)
	if err != nil {
		return false
	}
	for _, property := range properties {
		if FreeProperties[property] {
			continue
		}
		if _, found := values.GetValueByProperty(property); found {
			return false
		}
	}
	return true
}

// PrintNeedsEnterprise writes, for an example that shows no values of its
// own, each of properties that only the Enterprise data file has as needing
// that file, then where to ask for a copy.
func PrintNeedsEnterprise(properties []string, contactUs string) {
	fmt.Println("\nThis example used a free data file, which has no values " +
		"for these properties:")
	for _, property := range properties {
		if !FreeProperties[property] {
			fmt.Printf("\t%s: %s\n", property, NeedsEnterprise)
		}
	}
	PrintEnterpriseNote(contactUs)
}

// PrintEnterpriseNote writes where to ask for a copy of the Enterprise data
// file, which has values for the properties the free data file does not.
func PrintEnterpriseNote(contactUs string) {
	fmt.Printf("\nSome of these properties need the Enterprise data file. "+
		"Contact us for a copy at %s\n", contactUs)
}

// ShowEnterprise opens the data file with properties, looks up one address
// and prints each property, as needing the Enterprise data file when the
// data file is a free one, then where to ask for a copy. It is for the
// examples that measure with fewer properties and show these afterwards, so
// the measurement is unchanged. The caller stops its own engine first, so
// the data file is never loaded twice at once, and the low memory
// configuration reads only what the one lookup needs.
func ShowEnterprise(dataFile, contactUs string, properties []string) {
	engine, err := ipi_onpremise.New(
		ipi_onpremise.WithConfigIpi(
			ipi_interop.NewConfigIpi(ipi_interop.LowMemory)),
		ipi_onpremise.WithDataFile(dataFile),
		ipi_onpremise.WithAutoUpdate(false),
		ipi_onpremise.WithTempDataCopy(false),
		ipi_onpremise.WithProperties(properties),
	)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Stop()

	free := IsFreeDataFile(engine, properties)
	values, err := engine.Process(probeAddress)
	if err != nil {
		log.Fatalf("Failed to process %s: %v", probeAddress, err)
	}
	fmt.Printf("\nIP address %s:\n", probeAddress)
	for _, property := range properties {
		value, found := values.GetValueByProperty(property)
		switch {
		case found:
			fmt.Printf("\t%s: %v\n", property, value)
		case free && !FreeProperties[property]:
			fmt.Printf("\t%s: %s\n", property, NeedsEnterprise)
		default:
			fmt.Printf("\t%s: no value for this address\n", property)
		}
	}
	if free {
		PrintEnterpriseNote(contactUs)
	}
}
