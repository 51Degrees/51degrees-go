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
@example onpremise/metadata/metadata.go
This example shows what the Device Detection data file describes about
itself: the Evidence keys the engine reads, each component with its number of
profiles and its properties, each property with its category, type, the data
files that hold it, its description and its values, and the values of the
profiles an iPhone is matched to. It runs with the free Lite data file, and
shows the properties that only the Enterprise data file has as needing it.

This example is available in full on [GitHub](https://github.com/51Degrees/51degrees-go/tree/main/examples/device-detection/onpremise/metadata).

@include{doc} example-require-datafile.txt
*/

package main

import (
	"fmt"
	"log"
	"strings"

	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
	"github.com/51Degrees/51degrees-go/examples/device-detection/onpremise/common"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"github.com/51Degrees/51degrees-go/device-detection/v4/onpremise"
)

// contactUs is where to ask for a copy of the Enterprise data file, which has
// the properties the free data file leaves out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-onpremise-metadata-metadata.go&utm_term=enterprise-data-file"

// mobileUserAgent is the User-Agent of an iPhone, whose profiles the example
// lists the values of.
const mobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) " +
	"AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 " +
	"Safari/9537.53"

// mostValuesShown is the most values of a property the example lists. A
// property with more, such as a model name, is shown with its count.
const mostValuesShown = 10

// properties are the properties the engine is made with, so that results
// hold their values. The metadata describes every property of the data file.
var properties = append([]string{"IsMobile", "PlatformName", "BrowserName"},
	dd_example.EnterpriseProperties...)

// prefixName is the name written before an Evidence key with the prefix.
func prefixName(prefix dd.EvidencePrefix) string {
	switch prefix {
	case dd.HttpHeaderString:
		return "header"
	case dd.HttpEvidenceQuery:
		return "query"
	case dd.HttpEvidenceCookie:
		return "cookie"
	case dd.HttpEvidenceServer:
		return "server"
	}
	return "other"
}

// printEvidenceKeys writes the Evidence keys the engine reads.
func printEvidenceKeys(engine *onpremise.Engine) {
	fmt.Println("Evidence keys")
	for _, key := range engine.GetHttpHeaderKeys() {
		fmt.Printf("\t%s.%s\n", prefixName(key.Prefix), key.Key)
	}
	fmt.Println()
}

// valuesText describes the values of a property, being each value with the
// default marked, or the number of values when there are many.
func valuesText(values []dd.ValueMetadata) string {
	if len(values) > mostValuesShown {
		return fmt.Sprintf("%d values", len(values))
	}
	names := make([]string, 0, len(values))
	for _, value := range values {
		if value.IsDefault {
			names = append(names, value.Name+" (default)")
		} else {
			names = append(names, value.Name)
		}
	}
	return strings.Join(names, ", ")
}

// printComponents writes each component with its number of profiles, its
// default profile and its properties, each with its category, type, the data
// files that hold it, its description and its values.
func printComponents(engine *onpremise.Engine) error {
	components, err := engine.Components()
	if err != nil {
		return err
	}
	all, err := engine.Properties()
	if err != nil {
		return err
	}
	profiles, err := engine.Profiles()
	if err != nil {
		return err
	}
	values, err := engine.Values()
	if err != nil {
		return err
	}
	byName := make(map[string]dd.PropertyMetadata, len(all))
	for _, property := range all {
		byName[property.Name] = property
	}
	profileCount := make(map[uint8]int)
	for _, profile := range profiles {
		profileCount[profile.ComponentId]++
	}
	valuesOf := make(map[string][]dd.ValueMetadata)
	for _, value := range values {
		valuesOf[value.Property] = append(valuesOf[value.Property], value)
	}

	for _, component := range components {
		fmt.Printf("Component %s, %d profiles, default profile %d\n",
			component.Name, profileCount[component.Id],
			component.DefaultProfileId)
		for _, name := range component.Properties {
			property := byName[name]
			fmt.Printf("\t%s (%s, %v, in %s)\n", property.Name,
				property.Category, property.Type,
				strings.Join(property.DataTiers, ", "))
			fmt.Printf("\t\t%s\n", property.Description)
			fmt.Printf("\t\tValues: %s\n", valuesText(valuesOf[name]))
		}
		fmt.Println()
	}
	return nil
}

// printMatchedProfiles detects the iPhone's User-Agent and writes, for each
// profile it is matched to, the values the profile holds of the properties
// the engine was made with.
func printMatchedProfiles(engine *onpremise.Engine) error {
	results, err := engine.Process([]onpremise.Evidence{{
		Prefix: dd.HttpHeaderString,
		Key:    "User-Agent",
		Value:  mobileUserAgent,
	}})
	if err != nil {
		return err
	}
	defer results.Free()
	deviceId, err := results.DeviceId()
	if err != nil {
		return err
	}

	wanted := make(map[string]bool, len(properties))
	for _, name := range properties {
		wanted[name] = true
	}
	fmt.Printf("The iPhone is matched to the profiles %s\n", deviceId)
	for _, part := range strings.Split(deviceId, "-") {
		var id uint32
		if _, err := fmt.Sscan(part, &id); err != nil || id == 0 {
			continue
		}
		held, err := engine.ProfileValues(id)
		if err != nil {
			return err
		}
		fmt.Printf("\tProfile %d holds %d values, among them\n", id,
			len(held))
		for _, value := range held {
			if wanted[value.Property] {
				fmt.Printf("\t\t%s %s\n", value.Property, value.Name)
			}
		}
	}
	fmt.Println()
	return nil
}

// printEnterprise writes each property only the Enterprise data file has,
// with its description, or as needing that file when the data file in use
// does not define it, and returns whether any needed it.
func printEnterprise(engine *onpremise.Engine) (bool, error) {
	all, err := engine.Properties()
	if err != nil {
		return false, err
	}
	byName := make(map[string]dd.PropertyMetadata, len(all))
	for _, property := range all {
		byName[property.Name] = property
	}
	fmt.Println("Properties only the Enterprise data file has")
	missing := false
	for _, name := range dd_example.EnterpriseProperties {
		if property, ok := byName[name]; ok {
			fmt.Printf("\t%s: %s\n", name, property.Description)
		} else {
			fmt.Printf("\t%s: %s\n", name, dd_example.NeedsEnterprise)
			missing = true
		}
	}
	fmt.Println()
	return missing, nil
}

// main creates the engine with the data file and prints its metadata.
func main() {
	common.RunExample(
		func(params common.ExampleParams) error {
			engine, err := onpremise.New(
				onpremise.WithConfigHash(dd.NewConfigHash(dd.Default)),
				onpremise.WithProperties(properties),
				onpremise.WithDataFile(params.DataFile),
				onpremise.WithAutoUpdate(false),
			)
			if err != nil {
				log.Fatalf("Failed to create engine: %v", err)
			}
			// Stopping the engine frees its data set. Deferring the call
			// stops it however this function returns.
			defer engine.Stop()

			printEvidenceKeys(engine)
			if err := printComponents(engine); err != nil {
				return err
			}
			if err := printMatchedProfiles(engine); err != nil {
				return err
			}
			missing, err := printEnterprise(engine)
			if err != nil {
				return err
			}
			if missing {
				fmt.Print(dd_example.EnterpriseNote(contactUs))
			}
			return nil
		},
	)
}
