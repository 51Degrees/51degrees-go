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
This example illustrates how to read each property's value as its Go type.
PropertyType gives the type the data file records for a property, so IsMobile
is read as a bool, ScreenPixelsWidth as an int, and HardwareName, which holds a
list of values, as a []string. It runs with the free Lite data file, and shows
the properties that only the Enterprise data file has as needing it.
*/

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"

	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
)

// contactUs is where to ask for a copy of the Enterprise data file, which has
// the properties the free data file leaves out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-dd-strongly_typed-strongly_typed.go&utm_term=enterprise-data-file"

// separator joins the values of a property that holds a list, and is a
// character no value holds.
const separator = "\x1f"

// properties are those the example reads. The free data file has the first
// four, and the Enterprise data file has them all.
var properties = append([]string{
	"IsMobile",
	"ScreenPixelsWidth",
	"PlatformName",
	"BrowserVersion",
}, dd_example.EnterpriseProperties...)

// typed reads the property's value as the Go type the data file records for
// it, and returns it with that type, or nil and why when there is none.
func typed(results *dd.ResultsHash, property string) (any, reflect.Type, string) {
	if !dd_example.HasProperty(results, property) {
		return nil, nil, dd_example.NeedsEnterprise
	}
	valueType := results.PropertyType(
		results.RequiredPropertyIndexFromName(property))
	hasValues, err := results.HasValues(property)
	if err != nil {
		log.Fatalln(err)
	}
	if !hasValues {
		return nil, valueType, "no value for this device"
	}
	text, err := results.ValuesString(property, separator)
	if err != nil {
		log.Fatalln(err)
	}
	var value any
	switch valueType.Kind() {
	case reflect.Bool:
		value, err = strconv.ParseBool(text)
	case reflect.Int:
		value, err = strconv.Atoi(text)
	case reflect.Float64:
		value, err = strconv.ParseFloat(text, 64)
	case reflect.Slice:
		value = strings.Split(text, separator)
	default:
		value = text
	}
	if err != nil {
		return nil, valueType, fmt.Sprintf("%q is not a %s", text, valueType)
	}
	return value, valueType, ""
}

// match detects the User-Agent and returns a line for each property, with its
// value and Go type, and whether the data file lacks any of them.
func match(results *dd.ResultsHash, ua string) (string, bool) {
	if err := results.MatchUserAgent(ua); err != nil {
		log.Fatalln(err)
	}
	output := ""
	missing := false
	for _, property := range properties {
		value, valueType, why := typed(results, property)
		switch {
		case why == dd_example.NeedsEnterprise:
			output += fmt.Sprintf("\t%s: %s\n", property, why)
			missing = true
		case why != "":
			output += fmt.Sprintf("\t%s: %s\n", property, why)
		default:
			output += fmt.Sprintf("\t%s: %#v (%s)\n", property, value, valueType)
		}
	}
	return output, missing
}

func runStronglyTyped(perf dd.PerformanceProfile) string {
	manager := dd.NewResourceManager()
	config := dd.NewConfigHash(perf)
	filePath := dd_example.GetFilePathByName([]string{dd_example.LiteDataFile})
	err := dd.InitManagerFromFile(
		manager,
		*config,
		strings.Join(properties, ","),
		filePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer manager.Free()

	results := dd.NewResultsHash(manager, 1, 0)
	defer results.Free()

	userAgents := []struct{ name, ua string }{
		{"Mobile", "Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) " +
			"AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 " +
			"Safari/9537.53"},
		{"Desktop", "Mozilla/5.0 (Windows NT 6.3; WOW64; rv:41.0) " +
			"Gecko/20100101 Firefox/41.0"},
	}
	output := ""
	missing := false
	for _, userAgent := range userAgents {
		values, needs := match(results, userAgent.ua)
		output += fmt.Sprintf("%s User-Agent: %s\n%s\n",
			userAgent.name, userAgent.ua, values)
		missing = missing || needs
	}
	if missing {
		output += dd_example.EnterpriseNote(contactUs)
	}
	return output
}

func main() {
	dd_example.PerformExample(dd.Default, runStronglyTyped)
	// Output, with the free Lite data file, starts:
	// Mobile User-Agent: Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 Safari/9537.53
	// 	IsMobile: true (bool)
	// 	ScreenPixelsWidth: 640 (int)
	// 	PlatformName: "iOS" (string)
	// 	BrowserVersion: "7.0" (string)
	// 	HardwareVendor: needs the Enterprise data file
	// 	HardwareName: needs the Enterprise data file
	// 	DeviceType: needs the Enterprise data file
}
