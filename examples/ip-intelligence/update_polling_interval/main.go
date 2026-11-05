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

@example update_polling_interval/main.go
# Update polling interval example of using 51Degrees IP intelligence.

This example shows how to use 51Degrees on-premise IP intelligence to
process IP address before and after IPI file will be updated.

This example is available in full on [GitHub](https://github.com/51Degrees/51degrees-go/tree/main/examples/ip-intelligence/update_polling_interval).

@include{doc} example-require-datafile-ipi.txt

@include{doc} example-how-to-run-ipi.txt

## In detail, the example shows how to

### 1. Specify config for engine:

This setting specifies the performance profile that will be used when initializing the C library.

```
config := ipi_interop.NewConfigIpi(ipi_interop.InMemory)
```
<br/>
### 2. Initialization of the engine with the following parameters:

```
	//Create on-premise engine
	engine, err := ipi_onpremise.New(
		// Optimized config provided
		ipi_onpremise.WithConfigIpi(config),
		// Path to your data file
		ipi_onpremise.WithDataFile(params.DataFile),

		// To test this example, you need to:
		// 1. Host an IP Intelligence data file (.ipi) at a custom URL accessible to this application
		// 2. Provide that custom URL using WithDataUpdateUrl parameter
		// 3. No license key is required when using a custom URL
		// For production use, you will eventually need to use a Distributor service and
		// license key to keep your data file updated.
		// To obtain access to enterprise data files for hosting, please contact us at https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-update_polling_interval-main.go&utm_term=top
		ipi_onpremise.WithLicenseKey(params.LicenseKey),

		// WithAutoUpdate enables or disables auto update
		ipi_onpremise.WithAutoUpdate(true),

		// Set the frequency in minutes that the pipeline should
		// check for updates to data files. A recommended
		// polling interval in a production environment is
		// around 30 minutes.
		ipi_onpremise.WithPollingInterval(3),

		// Set the max amount of time in seconds that should be
		// added to the polling interval. This is useful in datacenter
		// applications where multiple instances may be polling for
		// updates at the same time. A recommended amount in production
		// environments is 600 seconds.
		ipi_onpremise.WithRandomization(1),

		// Enable update on startup, the auto update system
		// will be used to check for an update before the
		// IP intelligence engine is created.
		ipi_onpremise.WithUpdateOnStart(false),

		// Optionally provide your own file URL
		ipi_onpremise.WithDataUpdateUrl("<custom URL>"),

		// By default a temp copy should be created, unless you are using InMemory performance profile
		// ipi_onpremise.WithTempDataCopy(false),

		// File System Watcher is by default enabled
		// ipi_onpremise.WithFileWatch(false),

		// By default logging is on
		// ipi_onpremise.WithLogging(false),

		// Custom logger implementing LogWriter interface can be passed
		// ipi_onpremise.WithCustomLogger()

		// Set properties for checking, default is [] = all properties
		// ipi_onpremise.WithProperties([]string{}),
	)
```
*/

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/51Degrees/51degrees-go/examples/ip-intelligence/common"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_onpremise"
)

// contactUs is where to ask for a licence key and a copy of the Enterprise
// data file, which automatic updates download.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-ip-intelligence-update_polling_interval-main.go&utm_term=licence-key"

// processEvidence logs each property's value for the address, and a property
// the data file does not have as needing the Enterprise data file.
func processEvidence(
	engine *ipi_onpremise.Engine,
	available map[string]bool,
	ipAddress string) {
	result, err := engine.Process(ipAddress)
	if err != nil {
		log.Printf("Failed to process %s: %v", ipAddress, err)
		return
	}

	log.Printf("IP address %s:", ipAddress)
	for _, property := range common.Properties {
		if !available[property] {
			log.Printf("%s: %s", property, common.NeedsEnterprise)
			continue
		}
		if value, weight, found := result.GetValueWeightByProperty(property); found {
			log.Printf("%s: %+v (weight %.2f)", property, value, weight)
		} else {
			log.Printf("%s: no value for this address", property)
		}
	}
}

var evidences = common.IpEvidences{
	"185.28.167.77",
	"fdaa:bbcc:ddee:0:995f:d63a:f2a1:f189",
	"145.23.184.233",
	"e58e:9ea7:1049:5a42:503d:c45c:f55a:0bd3",
}

// runWithoutUpdates shows the data file the example has, once, when there is
// neither a licence key nor an address to download updates from.
func runWithoutUpdates(params *common.ExampleParams) {
	engine, err := ipi_onpremise.New(
		ipi_onpremise.WithConfigIpi(ipi_interop.NewConfigIpi(ipi_interop.InMemory)),
		ipi_onpremise.WithDataFile(params.DataFile),
		ipi_onpremise.WithAutoUpdate(false),
		ipi_onpremise.WithTempDataCopy(false),
		ipi_onpremise.WithProperties(common.Properties),
	)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Stop()

	available := common.AvailableProperties(engine)
	processEvidence(engine, available, evidences[0])
	processEvidence(engine, available, evidences[1])

	fmt.Printf("\nAutomatic updates download the Enterprise data file with a "+
		"licence key, which IPI_KEY gives, or from a copy you host, whose "+
		"address IPI_DATA_FILE_URL gives. Contact us for a key and a copy "+
		"at %s\n", contactUs)
}

func main() {
	common.RunExample(
		func(params *common.ExampleParams) error {
			if params.LicenseKey == "" && params.DataFileUrl == "" {
				runWithoutUpdates(params)
				return nil
			}

			config := ipi_interop.NewConfigIpi(ipi_interop.InMemory)

			//Create on-premise engine
			engine, err := ipi_onpremise.New(
				// Optimized config provided
				ipi_onpremise.WithConfigIpi(config),
				// Path to your data file
				ipi_onpremise.WithDataFile(params.DataFile),

				// To test this example, you need to:
				// 1. Host an IP Intelligence data file (.ipi) at a custom URL accessible to this application
				// 2. Provide that custom URL using WithDataUpdateUrl parameter
				// 3. No license key is required when using a custom URL
				// For production use, you will eventually need to use a Distributor service and
				// license key to keep your data file updated.
				// To obtain access to enterprise data files for hosting, please contact us at https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-update_polling_interval-main.go&utm_term=main
				ipi_onpremise.WithLicenseKey(params.LicenseKey),

				// Enable automatic updates.
				ipi_onpremise.WithAutoUpdate(true),

				// Set the frequency in minutes that the pipeline should
				// check for updates to data files. A recommended
				// polling interval in a production environment is
				// around 30 minutes.
				ipi_onpremise.WithPollingInterval(3),

				// Set the max amount of time in seconds that should be
				// added to the polling interval. This is useful in datacenter
				// applications where multiple instances may be polling for
				// updates at the same time. A recommended amount in production
				// environments is 600 seconds.
				ipi_onpremise.WithRandomization(1),

				// Enable update on startup, the auto update system
				// will be used to check for an update before the
				// IP intelligence engine is created.
				ipi_onpremise.WithUpdateOnStart(false),

				// Optionally provide your own file URL
				ipi_onpremise.WithDataUpdateUrl(params.DataFileUrl),

				// By default a temp copy should be created, unless you are using InMemory performance profile
				ipi_onpremise.WithTempDataCopy(false),

				// File System Watcher is by default enabled
				// ipi_onpremise.WithFileWatch(false),

				// By default logging is on
				// ipi_onpremise.WithLogging(false),

				// Custom logger implementing LogWriter interface can be passed
				// ipi_onpremise.WithCustomLogger()

				// Set properties for checking, default is [] = all properties
				ipi_onpremise.WithProperties([]string{}),
			)
			if err != nil {
				log.Fatalf("Failed to create engine: %v", err)
			}

			defer engine.Stop()

			available := common.AvailableProperties(engine)

			//process before file has been updated
			processEvidence(engine, available, evidences[0])
			processEvidence(engine, available, evidences[1])

			<-time.After(30 * time.Second)

			// The data file can have changed, so its properties are read
			// again before the second pair of addresses.
			available = common.AvailableProperties(engine)

			//process after file has been updated
			processEvidence(engine, available, evidences[2])
			processEvidence(engine, available, evidences[3])

			return nil
		})
}
