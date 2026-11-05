/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2025 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
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

/**
@example getting_started/main.go
# Getting started example of using 51Degrees IP intelligence.

The example shows how to use 51Degrees on-premise IP intelligence to determine the IP parameters
(IpRangeStart, IpRangeEnd, RegisteredCountry, RegisteredName, Longitude, Latitude, Areas, Mcc) of a given IP address
in golang wrapper integration. It runs with the free Lite data file, and shows the properties that only the
Enterprise data file has as needing it.

This example is available in full on [GitHub](https://github.com/51Degrees/51degrees-go/tree/main/examples/ip-intelligence/getting_started).

@include{doc} example-require-datafile-ipi.txt
@include{doc} example-how-to-run-ipi.txt

## In detail, the example shows how to

### 1. Specify config for engine:
<br/>
This setting specifies the performance profile that will be used when initializing the C library.

```
config := ipi_interop.NewConfigIpi(ipi_interop.InMemory)
```
<br/>
### 2. Initialization of the engine with the following parameters:

```
engine, err := ipi_onpremise.New(

	// Optimized config provided
	ipi_onpremise.WithConfigIpi(config),
	// Path to your data file
	ipi_onpremise.WithDataFile(params.DataFile),
	// Enable automatic updates.
	ipi_onpremise.WithAutoUpdate(false),
	// Optional properties configuration
	ipi_onpremise.WithProperties(common.Properties),
)
```
<br/>
<b>WithConfigIpi</b> allows to configure the Ipi matching algorithm.

```
ipi_onpremise.WithConfigIpi(config)
```
<br/>
<b>WithDataFile</b> sets the path to the local data file, this parameter is required to start the engine.

```
ipi_onpremise.WithDataFile(params.DataFile),
```
<br/>
<b>WithAutoUpdate</b> enables or disables auto update.

```
ipi_onpremise.WithAutoUpdate(false),
```
<br/>
<b>WithProperties</b> (Optional) Configures the list of properties to be returned. If not configured - all properties will be returned.

```
ipi_onpremise.WithProperties(common.Properties),
```
<br/>
### 3. Engine output with the parameter of the required address to receive data

```
result, err := engine.Process(ipiItem.IpAddress)
```
<br/>
### 4. Getting the results of the value after processing

For most properties (non-weighted):
```
val, _ := result.GetValueByProperty(property)
```

For MCC property (weighted):
```
val, weight, _ := result.GetValueWeightByProperty(property)
```
<br/>
### Output

Each property the data file does not have is shown as needing the Enterprise
data file, and the example then says where to ask for a copy. The free Lite
data file has RegisteredCountry, RegisteredName and RegisteredOwner, and the
Enterprise data file has them all.

```
IP address 185.28.167.77:
	IpRangeStart: needs the Enterprise data file
	...
	RegisteredCountry: <the country code> (weight 1.00)
```
*/

package main

import (
	"fmt"
	"log"

	"github.com/51Degrees/51degrees-go/examples/ip-intelligence/common"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_onpremise"
)

// contactUs is where to ask for a copy of the Enterprise data file, which has
// the properties the free data files leave out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-ip-intelligence-getting_started-main.go&utm_term=enterprise-data-file"

// addresses are an IPv4 address, an IPv6 address and the loopback address.
var addresses = []string{
	"185.28.167.77",
	"fdaa:bbcc:ddee:0:995f:d63a:f2a1:f189",
	"127.0.0.1",
}

// printProperties writes each property's value for the address, with its
// weight, and shows a property the data file does not have as needing the
// Enterprise data file.
func printProperties(
	engine *ipi_onpremise.Engine,
	available map[string]bool,
	address string) {
	values, err := engine.Process(address)
	if err != nil {
		log.Fatalf("Failed to process %s: %v", address, err)
	}

	fmt.Printf("IP address %s:\n", address)
	for _, property := range common.Properties {
		if !available[property] {
			fmt.Printf("\t%s: %s\n", property, common.NeedsEnterprise)
			continue
		}
		value, weight, found := values.GetValueWeightByProperty(property)
		if !found {
			fmt.Printf("\t%s: no value for this address\n", property)
			continue
		}
		fmt.Printf("\t%s: %v (weight %.2f)\n", property, value, weight)
	}
	fmt.Println()
}

// main creates the engine with the data file, looks up each address and
// prints the results.
func main() {
	common.RunExample(
		func(params *common.ExampleParams) error {
			//Create config
			config := ipi_interop.NewConfigIpi(ipi_interop.InMemory)

			//Create on-premise engine
			engine, err := ipi_onpremise.New(
				// Optimized config provided
				ipi_onpremise.WithConfigIpi(config),
				// Path to your data file
				ipi_onpremise.WithDataFile(params.DataFile),
				// Enable automatic updates.
				ipi_onpremise.WithAutoUpdate(false),
				ipi_onpremise.WithTempDataCopy(false),
				// The properties to return. Those the data file does not
				// have are left out of the results.
				ipi_onpremise.WithProperties(common.Properties),
			)
			if err != nil {
				log.Fatalf("Failed to create engine: %v", err)
			}
			// Stopping the engine frees its data set. Deferring the call
			// stops it however this function returns.
			defer engine.Stop()

			available := common.AvailableProperties(engine)
			for _, address := range addresses {
				printProperties(engine, available, address)
			}
			common.PrintEnterpriseNote(available, common.Properties, contactUs)

			return nil
		})
}
