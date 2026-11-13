/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, Davidson House,
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

package main

/*
 This example illustrates how to perform simple device detections using
 Device IDs. It runs with the free Lite data file, and shows the properties
 that only the Enterprise data file has as needing it.
*/

import (
	"fmt"
	"log"

	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
	"github.com/51Degrees/51degrees-go/examples/device-detection/onpremise/common"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"github.com/51Degrees/51degrees-go/device-detection/v4/onpremise"
)

// contactUs is where to ask for a copy of the Enterprise data file, which has
// the properties the free data file leaves out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-onpremise-match_device_id-match_device_id.go&utm_term=enterprise-data-file"

// function match performs a match on an input User-Agent, obtain the device id
// and perform a second match using the obtained device-id. Then from the
// returned result of the second match, determin if the device is a mobile
// device. Returns that output, a line for each property only the Enterprise
// data file has, and whether the data file lacks any of them.
// This function use two ResultsHash inputs so that two matches can be performed
// independently to guarantee the result of the second match is not impacted by
// the result of the first match.
func matchDeviceId(
	engine *onpremise.Engine,
	evidence []onpremise.Evidence) (string, string, bool) {
	// Perform detection. There are no results to free when it fails.
	results, err := engine.Process(evidence)
	if err != nil {
		log.Fatalln(err)
	}

	// Make sure results object is freed after function execution.
	defer results.Free()

	// Obtain DeviceId from results
	deviceId, err := results.DeviceId()
	if err != nil {
		log.Fatalln(err)
	}

	// Obtain raw results object
	devIdResults := engine.NewResultsHash(1, 0)

	// Make sure results object is freed after function execution.
	defer devIdResults.Free()

	// Obtain results again, using device Id.
	err = devIdResults.MatchDeviceId(deviceId)
	if err != nil {
		log.Fatalln(err)
	}

	// If results has values for required property
	propertyName := "IsMobile"
	hasValues, err := devIdResults.HasValues(propertyName)
	if err != nil {
		log.Fatalln(err)
	}

	returnStr := ""
	if !hasValues {
		returnStr = fmt.Sprintf("Property %s does not have a matched value.\n", propertyName)
	} else {
		// Get the values in string
		value, err := devIdResults.ValuesString(
			propertyName,
			",")
		if err != nil {
			log.Fatalln(err)
		}

		returnStr = fmt.Sprintf("\tIsMobile: %s\n", value)
	}
	enterprise, missing := dd_example.EnterpriseValues(devIdResults, "\t")
	return returnStr, enterprise, missing
}

func runMatchDeviceId(engine *onpremise.Engine) {
	// Perform detection on each Evidence. The IsMobile values are checked
	// below. The properties only the Enterprise data file has are shown too,
	// with values that depend on the data file in use.
	actual := ""
	shown := ""
	missing := false
	for i, device := range []struct {
		name     string
		evidence []onpremise.Evidence
	}{
		{"Mobile", common.ExampleEvidenceMobile},
		{"Desktop", common.ExampleEvidenceDesktop},
		{"MediaHub", common.ExampleEvidenceMediaHub},
	} {
		heading := fmt.Sprintf("%s User-Agent: %s\n",
			device.name, common.GetEvidenceUserAgent(device.evidence))
		if i > 0 {
			heading = "\n" + heading
		}
		isMobile, enterprise, needs := matchDeviceId(engine, device.evidence)
		actual += heading + isMobile
		shown += heading + isMobile + enterprise
		missing = missing || needs
	}

	// Expected output
	expected := "Mobile User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1.2 Mobile/15E148 Safari/604.1\n"
	expected += "\tIsMobile: True\n"
	expected += "\n"
	expected += "Desktop User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36\n"
	expected += "\tIsMobile: False\n"
	expected += "\n"
	expected += "MediaHub User-Agent: Mozilla/5.0 (Linux; U; Android 4.4.2; en-us; A464BG Build/KOT49H) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Mobile Safari/537.36\n"
	expected += "\tIsMobile: True\n"
	if actual != expected {
		log.Println("Expected:")
		log.Println(expected)
		log.Println("")
		log.Println("Actual:")
		log.Println(actual)
		log.Fatalln("Output does not match expected.")
	}
	if missing {
		shown += dd_example.EnterpriseNote(contactUs)
	}
	log.Println(shown)
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
			runMatchDeviceId(engine)

			return nil
		},
	)
}

// Output, with the free Lite data file:
// Mobile User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1.2 Mobile/15E148 Safari/604.1
// 	IsMobile: True
// 	HardwareVendor: needs the Enterprise data file
// 	HardwareName: needs the Enterprise data file
// 	DeviceType: needs the Enterprise data file
//
// Desktop User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
// 	IsMobile: False
// 	HardwareVendor: needs the Enterprise data file
// 	HardwareName: needs the Enterprise data file
// 	DeviceType: needs the Enterprise data file
//
// MediaHub User-Agent: Mozilla/5.0 (Linux; U; Android 4.4.2; en-us; A464BG Build/KOT49H) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Mobile Safari/537.36
// 	IsMobile: True
// 	HardwareVendor: needs the Enterprise data file
// 	HardwareName: needs the Enterprise data file
// 	DeviceType: needs the Enterprise data file
// Some of these properties need the Enterprise data file. Contact us for a copy at <contactUs>
