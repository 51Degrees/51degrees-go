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
@example examples/onpremise/getting_started.go
# Getting started example of using 51Degrees IP intelligence.

The example shows how to use 51Degrees on-premise IP intelligence to determine the IP parameters
(IpRangeStart, IpRangeEnd, RegisteredCountry, RegisteredName, Longitude, Latitude, Areas, Mcc) of a given IP address
in golang wrapper integration.

This example is available in full on [GitHub](https://github.com/51Degrees/ip-intelligence-go-examples/tree/main/ipi_onpremise/getting_started).

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
<br/>
### Expected output:

```
2025/06/10 09:45:06 Expected result for 185.28.167.77:
Expected & Actual:
IpRangeStart: 185.28.167.0:1.00
IpRangeEnd: 185.28.167.127:1.00
RegisteredCountry: GB:1.00
RegisteredName: CUSTOMERS-Subnet9:1.00
Longitude: 0.5273598437452315:1.00
Latitude: 51.34617145298623:1.00
Areas: POLYGON((-6.372 49.745,-6.416 49.753,-6.46 49.764,-6.499 49.781,-6.532 49.803,-6.554 49.83,-6.57 49.857,-8.311 54.381,-8.317 54.409,-8.317 54.48,-8.311 54.497,-7.625 57.584,-7.614 57.609,-7.592 57.631,-6.867 58.298,-6.839 58.32,-6.806 58.334,-6.762 58.347,-6.312 58.457,-3.301 59.166,-3.263 59.171,-3.225 59.174,-3.186 59.174,-3.148 59.171,-3.109 59.163,-2.84 59.092,-2.807 59.081,-2.774 59.07,-2.752 59.053,-2.736 59.034,-1.56 57.556,1.928 52.733,1.934 52.72,1.956 52.681,1.967 52.651,1.983 52.5,1.983 52.489,1.983 52.481,1.983 52.456,1.621 51.157,1.61 51.126,1.588 51.096,1.566 51.069,1.533 51.047,1.121 50.805,1.088 50.789,1.049 50.778,0.33 50.607,0.297 50.602,-5.153 49.83,-5.175 49.827,-6.328 49.745,-6.372 49.745)):1.00
Mcc: N/A:1.00

2025/06/10 09:45:06 Expected result for fdaa:bbcc:ddee:0:995f:d63a:f2a1:f189:
Expected & Actual:
IpRangeStart: fc00:0000:0000:0000:0000:0000:0000:0000:1.00
IpRangeEnd: fdff:ffff:ffff:ffff:ffff:ffff:ffff:ffff:1.00
RegisteredCountry: EU:1.00
RegisteredName: IANA-BLK:1.00
Longitude: -0.005493331705679495:1.00
Latitude: -0.0027466658528397473:1.00
Areas: POLYGON EMPTY:1.00
Mcc: N/A:1.00

2025/06/10 09:45:06 Not expected result for 127.0.0.1:
Expected:

Actual:
IpRangeStart: 127.0.0.0:1.00
IpRangeEnd: 127.255.255.255:1.00
RegisteredCountry: US:1.00
RegisteredName: SPECIAL-IPV4-LOOPBACK-IANA-RESERVED:1.00
Longitude: :0.00
Latitude: :0.00
Areas: POLYGON EMPTY:1.00
Mcc: N/A:1.00
```
*/

package main

import (
	"bytes"
	"fmt"
	"log"

	"github.com/51Degrees/ip-intelligence-go/v4/examples/common"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_interop"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_onpremise"
)

// gettingStartedTests defines test cases with IPs, their expected properties, and validation for working examples.
var gettingStartedTests = []*common.TestIpi{
	{
		IpAddress: "185.28.167.77",
		Expected: `IpRangeStart: 255.255.255.255
IpRangeEnd: 255.255.255.255
AccuracyRadiusMin: 0
AccuracyRadiusMax: 0
RegisteredCountry: Unknown
RegisteredName: Unknown
Longitude: 0
Latitude: 0
Areas: POLYGON EMPTY
Mcc: Unknown:1.00`,
		IsWorkingExample: true,
	},
	{
		IpAddress: "fdaa:bbcc:ddee:0:995f:d63a:f2a1:f189",
		Expected: `IpRangeStart: 255.255.255.255
IpRangeEnd: 255.255.255.255
AccuracyRadiusMin: 0
AccuracyRadiusMax: 0
RegisteredCountry: Unknown
RegisteredName: Unknown
Longitude: 0
Latitude: 0
Areas: POLYGON EMPTY
Mcc: Unknown:1.00`},
	{
		IpAddress:        "127.0.0.1",
		Expected:         ``,
		IsWorkingExample: false,
	},
}

// testIpi processes and validates IP address information using the provided engine and test data.
// It logs errors, actual results, and discrepancies compared to expected results for working or non-working examples.
func testIpi(engine *ipi_onpremise.Engine, ipiItem *common.TestIpi) {
	// Process IP address with engine
	result, err := engine.Process(ipiItem.IpAddress)
	if err != nil {
		log.Printf("Error processing Getting Started Example: %v", err)
		return
	}

	var actual bytes.Buffer
	// Getting property from a result
	for _, property := range common.Properties {
		if property == "Mcc" {
			// Only Mcc property has weight
			val, weight, _ := result.GetValueWeightByProperty(property)
			fmt.Fprintf(&actual, "%s: %+v:%.2f\n", property, val, weight)
		} else {
			// All other properties are non-weighted
			val, _ := result.GetValueByProperty(property)
			fmt.Fprintf(&actual, "%s: %+v\n", property, val)
		}
	}

	log.Printf("IP Address %s:\nActual result:\n%s\n", ipiItem.IpAddress, actual.String())

	if (ipiItem.IsWorkingExample && actual.String() != ipiItem.Expected) || (!ipiItem.IsWorkingExample && actual.String() == ipiItem.Expected) {
		log.Printf("\nExpected result:\n%s\n", ipiItem.Expected)
		return
	}
}

// runGettingStarted executes the "Getting Started" example tests, processing IP data for validation using the provided engine.
func runGettingStarted(engine *ipi_onpremise.Engine) {
	for _, test := range gettingStartedTests {
		testIpi(engine, test)
	}
}

// main initializes and runs the main example logic for demonstrating IP processing using an on-premise engine configuration.
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
				// Defined list of properties
				ipi_onpremise.WithProperties(common.Properties),
			)
			if err != nil {
				log.Fatalf("Failed to create engine: %v", err)
			}

			// Run example
			runGettingStarted(engine)

			engine.Stop()

			return nil
		})
}
