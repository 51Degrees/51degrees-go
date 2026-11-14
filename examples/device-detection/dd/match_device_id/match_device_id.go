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

/**
@example dd/match_device_id/match_device_id.go
This example shows how to perform device detections using Device IDs,
with the dd package.

This example is available in full on [GitHub](https://github.com/51Degrees/51degrees-go/tree/main/examples/device-detection/dd/match_device_id).

@include{doc} example-require-datafile.txt
*/

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

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
)

// contactUs is where to ask for a copy of the Enterprise data file, which has
// the properties the free data file leaves out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-dd-match_device_id-match_device_id.go&utm_term=enterprise-data-file"

// function match performs a match on an input User-Agent, obtain the device id
// and perform a second match using the obtained device-id. Then from the
// returned result of the second match, determin if the device is a mobile
// device. Returns that output, a line for each property only the Enterprise
// data file has, and whether the data file lacks any of them.
// This function use two ResultsHash inputs so that two matches can be performed
// independently to guarantee the result of the second match is not impacted by
// the result of the first match.
func matchDeviceId(
	results *dd.ResultsHash,
	devIdResults *dd.ResultsHash,
	ua string) (string, string, bool) {
	// Perform detection
	err := results.MatchUserAgent(ua)
	if err != nil {
		log.Fatalln(err)
	}

	// Obtain DeviceId from results
	deviceId, err := results.DeviceId()
	if err != nil {
		log.Fatalln(err)
	}

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

func runMatchDeviceId(perf dd.PerformanceProfile) string {
	// Initialise manager
	manager := dd.NewResourceManager()
	config := dd.NewConfigHash(perf)
	filePath := dd_example.GetFilePathByName([]string{dd_example.LiteDataFile})

	err := dd.InitManagerFromFile(
		manager,
		*config,
		"",
		filePath)
	if err != nil {
		log.Fatalln(err)
	}

	// Make sure manager object will be freed after the function execution
	defer manager.Free()

	// Create results
	results := dd.NewResultsHash(manager, 1, 0)
	devIdResults := dd.NewResultsHash(manager, 1, 0)

	// Make sure results object is freed after function execution.
	defer results.Free()
	defer devIdResults.Free()

	// User-Agent string of an iPhone mobile device.
	const uaMobile = "Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) " +
		"AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 " +
		"Safari/9537.53"

	// User-Agent string of Firefox Web browser version 41 on desktop.
	const uaDesktop = "Mozilla/5.0 (Windows NT 6.3; WOW64; rv:41.0) " +
		"Gecko/20100101 Firefox/41.0"

	// User-Agent string of a MediaHub device.
	const uaMediaHub = "Mozilla/5.0 (SMART-TV; Linux; Tizen 5.0) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/2.1 TV Safari/537.36"

	// Perform detection on each User-Agent. The IsMobile values are checked
	// below. The properties only the Enterprise data file has are shown too,
	// with values that depend on the data file in use.
	actual := ""
	shown := ""
	missing := false
	for i, device := range []struct{ name, ua string }{
		{"Mobile", uaMobile},
		{"Desktop", uaDesktop},
		{"MediaHub", uaMediaHub},
	} {
		heading := fmt.Sprintf("%s User-Agent: %s\n", device.name, device.ua)
		if i > 0 {
			heading = "\n" + heading
		}
		isMobile, enterprise, needs := matchDeviceId(
			results, devIdResults, device.ua)
		actual += heading + isMobile
		shown += heading + isMobile + enterprise
		missing = missing || needs
	}

	// Expected output
	expected := "Mobile User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 Safari/9537.53\n"
	expected += "\tIsMobile: True\n"
	expected += "\n"
	expected += "Desktop User-Agent: Mozilla/5.0 (Windows NT 6.3; WOW64; rv:41.0) Gecko/20100101 Firefox/41.0\n"
	expected += "\tIsMobile: False\n"
	expected += "\n"
	expected += "MediaHub User-Agent: Mozilla/5.0 (SMART-TV; Linux; Tizen 5.0) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/2.1 TV Safari/537.36\n"
	expected += "\tIsMobile: False\n"
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
	return shown
}

func main() {
	dd_example.PerformExample(dd.Default, runMatchDeviceId)
	// Output, with the free Lite data file:
	// Mobile User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 Safari/9537.53
	// 	IsMobile: True
	// 	HardwareVendor: needs the Enterprise data file
	// 	HardwareName: needs the Enterprise data file
	// 	DeviceType: needs the Enterprise data file
	//
	// Desktop User-Agent: Mozilla/5.0 (Windows NT 6.3; WOW64; rv:41.0) Gecko/20100101 Firefox/41.0
	// 	IsMobile: False
	// 	HardwareVendor: needs the Enterprise data file
	// 	HardwareName: needs the Enterprise data file
	// 	DeviceType: needs the Enterprise data file
	//
	// MediaHub User-Agent: Mozilla/5.0 (SMART-TV; Linux; Tizen 5.0) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/2.1 TV Safari/537.36
	// 	IsMobile: False
	// 	HardwareVendor: needs the Enterprise data file
	// 	HardwareName: needs the Enterprise data file
	// 	DeviceType: needs the Enterprise data file
	// Some of these properties need the Enterprise data file. Contact us for a copy at <contactUs>
}
