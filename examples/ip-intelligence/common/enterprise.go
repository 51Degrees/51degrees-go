package common

import (
	"fmt"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_onpremise"
)

// NeedsEnterprise is shown in place of the value of a property that the data
// file in use does not have, because only the Enterprise data file has it.
const NeedsEnterprise = "needs the Enterprise data file"

// maxProperties is more than the number of properties any data file has.
const maxProperties = 4096

// AvailableProperties returns the properties the engine's data file has, out
// of those the engine was given, or every property of the data file when it
// was given none. The free data files have only a few.
func AvailableProperties(engine *ipi_onpremise.Engine) map[string]bool {
	available := make(map[string]bool)
	for i := 0; i < maxProperties; i++ {
		if name := engine.GetPropertyNameByIndex(i); name != "" {
			available[name] = true
		}
	}
	return available
}

// PrintEnterpriseNote writes, when any of the properties is not in the data
// file, where to ask for a copy of the Enterprise data file that has them.
func PrintEnterpriseNote(
	available map[string]bool,
	properties []string,
	contactUs string) {
	for _, property := range properties {
		if !available[property] {
			fmt.Printf("\nSome of these properties need the Enterprise data "+
				"file. Contact us for a copy at %s\n", contactUs)
			return
		}
	}
}
