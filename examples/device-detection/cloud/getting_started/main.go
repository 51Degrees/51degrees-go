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
@example cloud/getting_started/main.go
# Getting started with Device Detection in the cloud

This example shows how to use the 51Degrees cloud service to detect the
device, operating system and browser a User-Agent comes from, with a Pipeline
built from an options file rather than in code.

The options file, `getting_started.json`, names two elements. A Cloud Request
Engine sends the evidence to the cloud service with your Resource Key, and the
Device Detection Cloud Engine, which must come after it, reads the results
from the cloud's response. The on-premise examples in this folder build their
engine in code instead.

This example is available in full on [GitHub](https://github.com/51Degrees/51degrees-go/tree/main/examples/device-detection/cloud/getting_started).

## Resource Key

The cloud service needs a Resource Key that includes Device Detection
properties. Create one for free with the
[Configurator](https://configure.51degrees.com/?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-cloud-getting_started-main.go&utm_term=resource-key),
then put it in the `_51DEGREES_RESOURCE_KEY` environment variable, or give it
as the example's first argument. Run with no Resource Key, the example says
where to create one and which variable to put it in, and ends without an
error.

## Running the example

```
cd examples/device-detection/cloud/getting_started
go run .
```
*/

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/dd_cloud"
	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

// optionsFile is the options file the Pipeline is built from, in the folder
// the example runs in.
const optionsFile = "getting_started.json"

// resourceKeyPlaceholder stands in the options file for the Resource Key,
// which the example puts in its place.
const resourceKeyPlaceholder = "!!YOUR_RESOURCE_KEY!!"

// resourceKeyVariable is the environment variable the example reads the
// Resource Key from.
const resourceKeyVariable = "_51DEGREES_RESOURCE_KEY"

// configuratorURL is where a Resource Key can be created for free.
const configuratorURL = "https://configure.51degrees.com/?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-cloud-getting_started-main.go&utm_term=resource-key-required"

// findOutMore is printed when the example finishes.
const findOutMore = `
Find out more
  Device Detection: https://51degrees.com/device-detection?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-cloud-getting_started-main.go&utm_term=find-out-more
  Resource Keys: https://51degrees.com/documentation/_services__cloud__resource_keys.html?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-cloud-getting_started-main.go&utm_term=find-out-more
  Property Dictionary: https://51degrees.com/developers/property-dictionary?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-cloud-getting_started-main.go&utm_term=find-out-more
  https://github.com/51Degrees/51degrees-go
`

// userAgents are the User-Agents the example detects.
var userAgents = []string{
	"Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 Safari/9537.53",
	"Mozilla/5.0 (Windows NT 6.3; WOW64; rv:41.0) Gecko/20100101 Firefox/41.0",
}

// properties are the Properties the example shows for each User-Agent. A
// Property the Resource Key does not include is reported as missing.
var properties = []string{
	"IsMobile",
	"PlatformName",
	"PlatformVersion",
	"BrowserName",
	"BrowserVersion",
	"HardwareVendor",
	"HardwareName",
	"DeviceType",
}

func main() {
	resourceKey := os.Getenv(resourceKeyVariable)
	if len(os.Args) > 1 {
		resourceKey = os.Args[1]
	}
	if err := run(optionsFile, resourceKey, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

// run builds the Pipeline from the options file at path, with resourceKey in
// place of the placeholder, and writes the Properties of each User-Agent to
// out.
func run(path, resourceKey string, out io.Writer) error {
	if resourceKey == "" {
		// With no Resource Key there is nothing to detect, so the example
		// says how to get one and ends cleanly, which lets it run with no
		// settings.
		fmt.Fprintf(out, "No Resource Key was given, so the example does not "+
			"use the cloud service. Create one that includes Device Detection "+
			"properties for free at %s, then put it in the %s environment "+
			"variable or give it as the first argument.\n",
			configuratorURL, resourceKeyVariable)
		fmt.Fprint(out, findOutMore)
		return nil
	}

	// Read the options file. Each element it names is built by the factory
	// registered under its BuilderName.
	options, err := core.LoadPipelineOptions(path)
	if err != nil {
		return fmt.Errorf("could not read the options file %s: %w", path, err)
	}
	for _, element := range options.Elements {
		if element.BuildParameters["ResourceKey"] == resourceKeyPlaceholder {
			element.BuildParameters["ResourceKey"] = resourceKey
		}
	}
	registry := core.NewBuilderRegistry()
	registry.Register("CloudRequestEngine", requestengine.Factory)
	registry.Register("DeviceDetectionCloudEngine", dd_cloud.Factory)

	// The Pipeline is made once and used for every detection. Closing it
	// frees what its elements hold.
	pipeline, err := registry.BuildFromConfiguration(options, nil)
	if err != nil {
		return fmt.Errorf("could not build the Pipeline: %w", err)
	}
	defer pipeline.Close()

	for _, userAgent := range userAgents {
		// A Flow Data holds the evidence for one detection and its results.
		fd := pipeline.CreateFlowData()
		fd.AddEvidence("header.user-agent", userAgent)
		if err := fd.Process(); err != nil {
			return fmt.Errorf("could not process %s: %w", userAgent, err)
		}
		data, err := core.Get[*dd_cloud.Data](fd)
		if err != nil {
			return fmt.Errorf("no Device Detection results for %s: %w", userAgent, err)
		}
		fmt.Fprintf(out, "User-Agent: %s\n", userAgent)
		for _, name := range properties {
			fmt.Fprintf(out, "\t%s: %s\n", name, valueText(data, strings.ToLower(name)))
		}
		fmt.Fprintln(out)
	}
	fmt.Fprint(out, findOutMore)
	return nil
}

// valueText gives a Property's value as text, or the reason it has none. A
// list gives each of its values.
func valueText(data *dd_cloud.Data, name string) string {
	value, err := data.Get(name)
	if err != nil {
		return "This property is not in the results, so the Resource Key may not include it."
	}
	switch v := value.(type) {
	case engines.AspectPropertyValue[[]string]:
		values, err := v.Value()
		if err != nil {
			return v.NoValueMessage()
		}
		return strings.Join(values, ", ")
	case engines.AspectPropertyValue[string]:
		text, err := v.Value()
		if err != nil {
			return v.NoValueMessage()
		}
		return text
	}
	return fmt.Sprint(value)
}
