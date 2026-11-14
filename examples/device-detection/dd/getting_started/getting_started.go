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
@example dd/getting_started/getting_started.go
This example shows how to perform simple device detections on User-Agent
strings with the dd package. It runs with the free Lite data file, and
shows the properties that only the Enterprise data file has as needing it.

This example is available in full on [GitHub](https://github.com/51Degrees/51degrees-go/tree/main/examples/device-detection/dd/getting_started).

@include{doc} example-require-datafile.txt
*/

package main

/*
This example illustrates how to perform simple device detections on given
User-Agent strings. It runs with the free Lite data file, and shows the
properties that only the Enterprise data file has as needing it.
*/

import (
	"fmt"
	"log"

	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
)

// contactUs is where to ask for a copy of the Enterprise data file, which has
// the properties the free data file leaves out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-dd-getting_started-getting_started.go&utm_term=enterprise-data-file"

// properties are those the example shows. The free data file has the first
// five, and the Enterprise data file has them all.
var properties = append([]string{
	"IsMobile",
	"PlatformName",
	"PlatformVersion",
	"BrowserName",
	"BrowserVersion",
}, dd_example.EnterpriseProperties...)

// match performs a detection on the User-Agent and returns each property's
// value, or that it needs the Enterprise data file when the data file does
// not have it, and whether any property needs it.
func match(
	results *dd.ResultsHash,
	ua string) (string, bool) {
	// Perform detection
	err := results.MatchUserAgent(ua)
	if err != nil {
		log.Fatalln(err)
	}

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
			output += fmt.Sprintf("\t%s: no value for this User-Agent\n", property)
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

func runGettingStarted(perf dd.PerformanceProfile) string {
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

	// Make sure results object is freed after function execution.
	defer results.Free()

	// User-Agent strings of an iPhone, Firefox 41 on a Windows desktop, and
	// a MediaHub device.
	userAgents := []struct{ name, ua string }{
		{"Mobile", "Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) " +
			"AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 " +
			"Safari/9537.53"},
		{"Desktop", "Mozilla/5.0 (Windows NT 6.3; WOW64; rv:41.0) " +
			"Gecko/20100101 Firefox/41.0"},
		{"MediaHub", "Mozilla/5.0 (SMART-TV; Linux; Tizen 5.0) AppleWebKit/537.36 " +
			"(KHTML, like Gecko) SamsungBrowser/2.1 TV Safari/537.36"},
	}

	output := ""
	missing := false
	for _, userAgent := range userAgents {
		values, needs := match(results, userAgent.ua)
		output += fmt.Sprintf("%s User-Agent: %s\n%s\n", userAgent.name, userAgent.ua, values)
		if needs {
			missing = true
		}
	}
	if missing {
		output += dd_example.EnterpriseNote(contactUs)
	}
	return output
}

func main() {
	dd_example.PerformExample(dd.Default, runGettingStarted)
}
