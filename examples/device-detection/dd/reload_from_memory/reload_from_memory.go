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

/**
@example dd/reload_from_memory/reload_from_memory.go
This example shows how a resource manager is made from a data file held in
memory and reloaded from a newer one held in memory, as an application does
when it downloads a data file and never writes it to disk. It runs with the
free Lite data file, and shows the properties that only the Enterprise data
file has as needing it.

This example is available in full on [GitHub](https://github.com/51Degrees/51degrees-go/tree/main/examples/device-detection/dd/reload_from_memory).

@include{doc} example-require-datafile.txt
*/

package main

import (
	"fmt"
	"log"
	"os"
	"unsafe"

	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
)

// contactUs is where to ask for a copy of the Enterprise data file, which has
// the properties the free data file leaves out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-dd-reload_from_memory-reload_from_memory.go&utm_term=enterprise-data-file"

// mobileUserAgent is the User-Agent of an iPhone, which the example detects
// before and after the reload.
const mobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) " +
	"AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 " +
	"Safari/9537.53"

// properties are those the example shows. The free data file has the first
// two, and the Enterprise data file has them all.
var properties = append([]string{"IsMobile", "PlatformName"},
	dd_example.EnterpriseProperties...)

// readDataFile reads the data file the example uses into memory, as an
// application holds one it has downloaded.
func readDataFile() []byte {
	data, err := os.ReadFile(dd_example.DataFilePath())
	if err != nil {
		log.Fatalln(err)
	}
	return data
}

// detect detects the iPhone's User-Agent and returns each property's value,
// or that it needs the Enterprise data file when the data file does not have
// it, and whether any property needs it.
func detect(manager *dd.ResourceManager) (string, bool) {
	results := dd.NewResultsHash(manager, 1, 0)
	defer results.Free()
	if err := results.MatchUserAgent(mobileUserAgent); err != nil {
		log.Fatalln(err)
	}
	output := ""
	missing := false
	for _, property := range properties {
		if !dd_example.HasProperty(results, property) {
			output += fmt.Sprintf("\t%s: %s\n", property,
				dd_example.NeedsEnterprise)
			missing = true
			continue
		}
		value, err := results.ValuesString(property, ",")
		if err != nil {
			log.Fatalln(err)
		}
		output += fmt.Sprintf("\t%s: %s\n", property, value)
	}
	return output, missing
}

func runReloadFromMemory(perf dd.PerformanceProfile) string {
	// Make the manager from the data file held in memory. The manager copies
	// the data, so the memory can be reused once this returns.
	data := readDataFile()
	manager := dd.NewResourceManager()
	if err := dd.InitManagerFromMemory(manager, *dd.NewConfigHash(perf), "",
		unsafe.Pointer(&data[0]), uint64(len(data))); err != nil {
		log.Fatalln(err)
	}
	// Make sure manager object will be freed after the function execution
	defer manager.Free()

	before, missing := detect(manager)
	output := fmt.Sprintf("Made from memory, the iPhone gives\n%s\n", before)

	// An application reloads from the newer data file it has downloaded into
	// memory. The example reads the same data file again. A reload that
	// fails keeps the data in use and returns the error.
	if err := manager.ReloadFromBytes(readDataFile()); err != nil {
		log.Fatalln(err)
	}
	after, needs := detect(manager)
	output += fmt.Sprintf("Reloaded from memory, the iPhone gives\n%s\n", after)

	if missing || needs {
		output += dd_example.EnterpriseNote(contactUs)
	}
	return output
}

func main() {
	dd_example.PerformExample(dd.Default, runReloadFromMemory)
}
