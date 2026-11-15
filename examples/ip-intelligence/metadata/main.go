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
@example metadata/main.go
# Metadata example of using 51Degrees IP intelligence.

The example shows what the IP Intelligence data file describes about itself:
the date it was published and when a newer one is expected, the Evidence keys
the engine reads an IP address from, in the order it tries them, and each
component with its properties, each with its category, the data files that
hold its values and its description. It runs with the free Lite data file, and
shows the properties that only the Enterprise data file has values for as
needing it.

This example is available in full on [GitHub](https://github.com/51Degrees/51degrees-go/tree/main/examples/ip-intelligence/metadata).

@include{doc} example-require-datafile-ipi.txt

## In detail, the example shows how to

### 1. Create the engine with the data file

```
engine, err := ipi_onpremise.New(
	ipi_onpremise.WithConfigIpi(ipi_interop.NewConfigIpi(ipi_interop.LowMemory)),
	ipi_onpremise.WithDataFile(params.DataFile),
	ipi_onpremise.WithAutoUpdate(false),
	ipi_onpremise.WithTempDataCopy(false),
	ipi_onpremise.WithProperties(common.Properties),
)
```
<br/>
### 2. Get the dates of the data file

```
published := engine.PublishedDate()
next := engine.NextUpdateDate()
```
<br/>
### 3. Get the Evidence keys the engine reads, in the order it tries them

```
keys := engine.GetHttpHeaderKeys()
```
<br/>
### 4. Get the components and the metadata of their properties

```
components, err := engine.Components()
properties, err := engine.Properties()
```
<br/>
### Output

```
Data file
	Published: <the date>
	Next update: <the date>

Evidence keys, in the order they are tried
	query.<a header>
	...
	server.<a header>

Component <a component>
	<a property> (<its category>, in <the data files>)
		<its description>
	...
```
*/

package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/51Degrees/51degrees-go/examples/ip-intelligence/common"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_onpremise"
)

// contactUs is where to ask for a copy of the Enterprise data file, which has
// the properties the free data files leave out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-ip-intelligence-metadata-main.go&utm_term=enterprise-data-file"

// dateLayout is how the example writes a date.
const dateLayout = "2006-01-02"

// prefixes are the names written before each Evidence key, by its prefix.
var prefixes = map[ipi_interop.EvidencePrefix]string{
	ipi_interop.HttpEvidenceQuery:  "query",
	ipi_interop.HttpEvidenceServer: "server",
}

// printDates writes when the data file was published and when a newer one is
// expected, where the data file says.
func printDates(engine *ipi_onpremise.Engine) {
	fmt.Println("Data file")
	fmt.Printf("\tPublished: %s\n", engine.PublishedDate().Format(dateLayout))
	if next := engine.NextUpdateDate(); !next.IsZero() {
		fmt.Printf("\tNext update: %s\n", next.Format(dateLayout))
	}
	fmt.Println()
}

// printEvidenceKeys writes the Evidence keys the engine reads an IP address
// from, in the order it tries them.
func printEvidenceKeys(engine *ipi_onpremise.Engine) {
	fmt.Println("Evidence keys, in the order they are tried")
	for _, key := range engine.GetHttpHeaderKeys() {
		fmt.Printf("\t%s.%s\n", prefixes[key.Prefix], key.Key)
	}
	fmt.Println()
}

// printComponents writes each component with its properties, each with its
// category, the data files that hold its values and its description.
func printComponents(engine *ipi_onpremise.Engine) error {
	components, err := engine.Components()
	if err != nil {
		return err
	}
	properties, err := engine.Properties()
	if err != nil {
		return err
	}
	byName := make(map[string]ipi_interop.PropertyMetadata, len(properties))
	for _, property := range properties {
		byName[property.Name] = property
	}
	for _, component := range components {
		fmt.Printf("Component %s\n", component.Name)
		for _, name := range component.Properties {
			property := byName[name]
			fmt.Printf("\t%s (%s, in %s)\n", property.Name, property.Category,
				strings.Join(property.DataTiers, ", "))
			fmt.Printf("\t\t%s\n", property.Description)
		}
		fmt.Println()
	}
	return nil
}

// main creates the engine with the data file and prints its metadata.
func main() {
	common.RunExample(
		func(params *common.ExampleParams) error {
			// The low memory configuration reads only the parts of the data
			// file that are used, which suits reading the metadata.
			engine, err := ipi_onpremise.New(
				ipi_onpremise.WithConfigIpi(
					ipi_interop.NewConfigIpi(ipi_interop.LowMemory)),
				ipi_onpremise.WithDataFile(params.DataFile),
				ipi_onpremise.WithAutoUpdate(false),
				ipi_onpremise.WithTempDataCopy(false),
				// The properties the engine is made with. Every property of
				// the data file is listed, and these are the ones results
				// hold values for.
				ipi_onpremise.WithProperties(common.Properties),
			)
			if err != nil {
				log.Fatalf("Failed to create engine: %v", err)
			}
			// Stopping the engine frees its data set. Deferring the call
			// stops it however this function returns.
			defer engine.Stop()

			printDates(engine)
			printEvidenceKeys(engine)
			if err := printComponents(engine); err != nil {
				return err
			}
			if common.IsFreeDataFile(engine, common.Properties) {
				common.PrintNeedsEnterprise(common.Properties, contactUs)
			}
			return nil
		})
}
