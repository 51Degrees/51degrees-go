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
@example getting_started_cloud/main.go
# Getting started with IP Intelligence in the cloud

This example shows how to use the 51Degrees cloud service to find out about an
IP address, such as the network it is registered to, the country it is
registered in and the range it belongs to.

A Cloud Request Engine sends the IP address to the cloud service with your
Resource Key, and the IP Intelligence Cloud Aspect Engine reads the results
from the cloud's response.

This example is available in full on [GitHub](https://github.com/51Degrees/51degrees-go/tree/main/examples/ip-intelligence/getting_started_cloud).

## Resource Key

The cloud service needs a Resource Key that includes IP Intelligence
properties. Create one for free with the
[Configurator](https://51degrees.com/configure/?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-ip-intelligence-getting_started_cloud-main.go&utm_term=resource-key),
then put it in the `_51DEGREES_RESOURCE_KEY` environment variable, or give it
as the example's first argument.

## Running the example

```
cd examples/ip-intelligence
go run ./getting_started_cloud
```
*/

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_cloud"
	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// resourceKeyVariable is the environment variable the example reads the
// Resource Key from.
const resourceKeyVariable = "_51DEGREES_RESOURCE_KEY"

// configuratorURL is where a Resource Key can be created for free.
const configuratorURL = "https://51degrees.com/configure/?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-ip-intelligence-getting_started_cloud-main.go&utm_term=resource-key-required"

// findOutMore is printed when the example finishes.
const findOutMore = `
Find out more
  IP Intelligence: https://51degrees.com/ip-intelligence?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-ip-intelligence-getting_started_cloud-main.go&utm_term=find-out-more
  Resource Keys: https://51degrees.com/documentation/_services__cloud__resource_keys.html?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-ip-intelligence-getting_started_cloud-main.go&utm_term=find-out-more
  Property Dictionary: https://51degrees.com/developers/property-dictionary?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-ip-intelligence-getting_started_cloud-main.go&utm_term=find-out-more
  https://github.com/51Degrees/51degrees-go
`

// ipAddresses are the addresses the example looks up.
var ipAddresses = []string{
	"116.154.188.222",
	"1.3.32.31",
	"45.236.48.61",
	"2001:0db8:085a:0000:0000:8a2e:0370:7334",
}

// properties are the Properties the example shows for each address. A
// Property the Resource Key does not include is reported as missing.
var properties = []string{
	"RegisteredName",
	"RegisteredOwner",
	"RegisteredCountry",
	"IpRangeStart",
	"IpRangeEnd",
	"Country",
	"CountryCode",
	"CountryCode3",
	"Region",
	"State",
	"Town",
	"Latitude",
	"Longitude",
	"Areas",
	"AccuracyRadiusMin",
	"TimeZoneOffset",
}

func main() {
	resourceKey := os.Getenv(resourceKeyVariable)
	if len(os.Args) > 1 {
		resourceKey = os.Args[1]
	}
	if resourceKey == "" {
		log.Fatalf("No Resource Key was given, so the example cannot use the "+
			"cloud service. Create one that includes IP Intelligence "+
			"properties for free at %s, then put it in the %s environment "+
			"variable or give it as the first argument.",
			configuratorURL, resourceKeyVariable)
	}

	// The Cloud Request Engine sends the evidence to the cloud service, and
	// the IP Intelligence Cloud Aspect Engine reads its results.
	requestEngine, err := requestengine.NewBuilder().
		SetResourceKey(resourceKey).
		Build()
	if err != nil {
		log.Fatalf("Failed to create the cloud request engine: %v", err)
	}
	pipeline, err := core.NewPipelineBuilder().
		AddFlowElement(requestEngine).
		AddFlowElement(ipi_cloud.New(nil)).
		Build()
	if err != nil {
		log.Fatalf("Failed to create the pipeline: %v", err)
	}
	defer pipeline.Close()

	for _, ip := range ipAddresses {
		// The cloud service looks up the address in the query.client-ip
		// evidence.
		fd := pipeline.CreateFlowData()
		fd.AddEvidence("query.client-ip", ip)
		if err := fd.Process(); err != nil {
			log.Fatalf("Failed to process %s: %v", ip, err)
		}
		data, err := core.Get[*ipi_cloud.Data](fd)
		if err != nil {
			log.Fatalf("No IP Intelligence results for %s: %v", ip, err)
		}
		fmt.Printf("Results for %s:\n", ip)
		for _, name := range properties {
			fmt.Printf("\t%s: %s\n", name, valueText(data, strings.ToLower(name)))
		}
	}
	fmt.Print(findOutMore)
}

// valueText gives a Property's value as text, or the reason it has none. A
// weighted Property gives each value with its weighting.
func valueText(data *ipi_cloud.Data, name string) string {
	value, err := data.Get(name)
	if err != nil {
		return "This property is not in the results, so the Resource Key may not include it."
	}
	switch v := value.(type) {
	case engines.AspectPropertyValue[[]core.WeightedValue[string]]:
		values, err := v.Value()
		if err != nil {
			return v.NoValueMessage()
		}
		parts := make([]string, 0, len(values))
		for _, w := range values {
			parts = append(parts, fmt.Sprintf("%s (%.2f)", w.Value(), w.Weighting()))
		}
		return strings.Join(parts, ", ")
	case engines.AspectPropertyValue[string]:
		text, err := v.Value()
		if err != nil {
			return v.NoValueMessage()
		}
		return text
	}
	return fmt.Sprint(value)
}
