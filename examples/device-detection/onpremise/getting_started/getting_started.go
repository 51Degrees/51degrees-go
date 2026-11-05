/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, 5 Charlotte Close,
 * Caversham, Reading, Berkshire, United Kingdom RG4 7BY.
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

package main

/*
 This example illustrates how to perform simple device detections on given
 Evidence. It runs with the free Lite data file, and shows the properties
 that only the Enterprise data file has as needing it.
*/

import (
	"fmt"
	"log"

	"github.com/51Degrees/51degrees-go/examples/device-detection/onpremise/common"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"github.com/51Degrees/51degrees-go/device-detection/v4/onpremise"
	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
)

// contactUs is where to ask for a copy of the Enterprise data file, which has
// the properties the free data file leaves out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-onpremise-getting_started-getting_started.go&utm_term=enterprise-data-file"

// properties are those the example shows. The free data file has the first
// five, and the Enterprise data file has them all.
var properties = append([]string{
	"IsMobile",
	"PlatformName",
	"PlatformVersion",
	"BrowserName",
	"BrowserVersion",
}, dd_example.EnterpriseProperties...)

// match performs a detection on the Evidence and returns each property's
// value, or that it needs the Enterprise data file when the data file does
// not have it, and whether any property needs it.
func match(
	engine *onpremise.Engine,
	evidence []onpremise.Evidence) (string, bool) {
	// Perform detection. There are no results to free when it fails.
	results, err := engine.Process(evidence)
	if err != nil {
		log.Fatalln(err)
	}

	// Make sure results object is freed after function execution.
	defer results.Free()

	output := ""
	missing := false
	for _, property := range properties {
		if !dd_example.HasProperty(results, property) {
			output += fmt.Sprintf("\t%s: %s\n", property, dd_example.NeedsEnterprise)
			missing = true
			continue
		}

		// If results has values for the property
		hasValues, err := results.HasValues(property)
		if err != nil {
			log.Fatalln(err)
		}
		if !hasValues {
			output += fmt.Sprintf("\t%s: no value for this device\n", property)
			continue
		}

		// Get the values in string
		value, err := results.ValuesString(property, ",")
		if err != nil {
			log.Fatalln(err)
		}
		output += fmt.Sprintf("\t%s: %s\n", property, value)
	}
	return output, missing
}

func runGettingStarted(engine *onpremise.Engine) {
	devices := []struct {
		name     string
		evidence []onpremise.Evidence
	}{
		{"Mobile", common.ExampleEvidenceMobile},
		{"Desktop", common.ExampleEvidenceDesktop},
		{"MediaHub", common.ExampleEvidenceMediaHub},
	}

	output := ""
	missing := false
	for _, device := range devices {
		values, needs := match(engine, device.evidence)
		output += fmt.Sprintf("%s User-Agent: %s\n%s\n",
			device.name, common.GetEvidenceUserAgent(device.evidence), values)
		if needs {
			missing = true
		}
	}
	if missing {
		output += dd_example.EnterpriseNote(contactUs)
	}
	fmt.Print(output)
}

func main() {
	common.RunExample(
		func(params common.ExampleParams) error {
			//... Example code
			//Create config
			config := dd.NewConfigHash(dd.Default)

			//Create on-premise engine
			engine, err := onpremise.New(
				// Optimized config provided
				onpremise.WithConfigHash(config),
				// Path to your data file
				onpremise.WithDataFile(params.DataFile),
				// Enable automatic updates.
				onpremise.WithAutoUpdate(false),
			)

			if err != nil {
				log.Fatalf("Failed to create engine: %v", err)
			}
			// Stopping the engine frees its data set. Deferring the call
			// stops it however this function returns.
			defer engine.Stop()

			// Run example
			runGettingStarted(engine)

			return nil
		},
	)
}
