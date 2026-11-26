/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2025 51 Degrees Mobile Experts Limited, Davidson House,
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

@example update_polling_interval/main.go
# Update polling interval example of using 51Degrees IP intelligence.

This example shows how to use 51Degrees on-premise IP intelligence to
process IP address before and after IPI file will be updated.

This example is available in full on [GitHub](https://github.com/51Degrees/51degrees-go/tree/main/examples/ip-intelligence/update_polling_interval).

@include{doc} example-require-datafile-ipi.txt

When the License Key holds several products, the engine asks for the
product of the data file it has loaded. To ask for another product, such as
IPIV4Enterprise, give its name with -product. The example downloads from the
51Degrees Distributor. To download from another address that answers as the
Distributor does, such as a copy you host, give it with -url.

```
go run ./update_polling_interval -product IPIV4Enterprise -url https://example.com/api/v2/download
```

## In detail, the example shows how to

### 1. Specify config for engine:

This setting specifies the performance profile that will be used when initializing the C library.
LowMemory reads the data file as it is needed, because updates download the
Enterprise data file, which is several gigabytes.

```
config := ipi_interop.NewConfigIpi(ipi_interop.LowMemory)
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

		// Updates download the Enterprise data file from the Distributor
		// with the license key, which _51DEGREES_LICENSE_KEY gives.
		// To obtain access to enterprise data files for hosting, please contact us at https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-update_polling_interval-main.go&utm_term=top
		ipi_onpremise.WithLicenseKey(params.LicenseKey),

		// Ask for the product -product names, as the Distributor spells
		// it, such as IPIV4Enterprise. When none is given, no product is
		// named.
		ipi_onpremise.WithProduct(*product),

		// WithAutoUpdate enables or disables auto update
		ipi_onpremise.WithAutoUpdate(true),

		// Set the frequency in seconds that the pipeline should
		// check for updates to data files. The example checks every
		// few seconds so that an update can arrive while it waits
		// between its two sets of lookups. A recommended polling
		// interval in a production environment is around 30
		// minutes, which is 1800 seconds.
		ipi_onpremise.WithPollingInterval(3),

		// Set the maximum random time in seconds added to each polling
		// interval. A new time from 0 up to this maximum is drawn for
		// each interval, so that multiple instances in a datacenter do
		// not poll for updates at the same time. A recommended amount
		// in production environments is 600 seconds.
		ipi_onpremise.WithRandomization(1),

		// Enable update on startup, the auto update system
		// will be used to check for an update before the
		// IP intelligence engine is created.
		ipi_onpremise.WithUpdateOnStart(false),

		// LowMemory keeps the data file open, so the engine reads a
		// temp copy and an update can replace the original.
		ipi_onpremise.WithTempDataCopy(true),

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
	"flag"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/51Degrees/51degrees-go/examples/ip-intelligence/common"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_onpremise"
)

// contactUs is where to ask for a licence key and a copy of the Enterprise
// data file, which automatic updates download.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-ip-intelligence-update_polling_interval-main.go&utm_term=licence-key"

var (
	product = flag.String("product", "",
		"the product to download, such as IPIV4Enterprise, in place of the "+
			"product of the data file")
	dataUpdateUrl = flag.String("url", "",
		"the address to download updates from, in place of the "+
			"Distributor's")
)

// updateUrl gives the address with what the Distributor reads from the
// query added to it, being the License Key, the type of data file and
// Download=True. A value the address has already is kept.
func updateUrl(address, licenseKey string) (string, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for _, parameter := range [][2]string{
		{"LicenseKeys", licenseKey},
		{"Type", "IPIV41"},
		{"Download", "True"},
	} {
		if query.Get(parameter[0]) == "" {
			query.Set(parameter[0], parameter[1])
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// processEvidence logs each property's value for the address. With a free
// data file, a property that only the Enterprise data file has values for is
// logged as needing it.
func processEvidence(
	engine *ipi_onpremise.Engine,
	free bool,
	ipAddress string) {
	result, err := engine.Process(ipAddress)
	if err != nil {
		log.Printf("Failed to process %s: %v", ipAddress, err)
		return
	}

	log.Printf("IP address %s:", ipAddress)
	for _, property := range common.NetworkProperties {
		value, weight, found := result.GetValueWeightByProperty(property)
		switch {
		case found:
			log.Printf("%s: %+v (weight %.2f)", property, value, weight)
		case free && !common.FreeProperties[property]:
			log.Printf("%s: %s", property, common.NeedsEnterprise)
		default:
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
// no license key to download updates with.
func runWithoutUpdates(params *common.ExampleParams) {
	engine, err := ipi_onpremise.New(
		ipi_onpremise.WithConfigIpi(ipi_interop.NewConfigIpi(ipi_interop.InMemory)),
		ipi_onpremise.WithDataFile(params.DataFile),
		ipi_onpremise.WithAutoUpdate(false),
		ipi_onpremise.WithTempDataCopy(false),
		ipi_onpremise.WithProperties(common.NetworkProperties),
	)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Stop()

	free := common.IsFreeDataFile(engine, common.NetworkProperties)
	processEvidence(engine, free, evidences[0])
	processEvidence(engine, free, evidences[1])

	fmt.Printf("\nAutomatic updates download the Enterprise data file from "+
		"the Distributor with a license key, which _51DEGREES_LICENSE_KEY "+
		"gives. Contact us for a key at %s\n", contactUs)
}

func main() {
	flag.Parse()
	common.RunExampleWithDataFile(common.AsnDataFile,
		func(params *common.ExampleParams) error {
			if params.LicenseKey == "" {
				runWithoutUpdates(params)
				return nil
			}

			// Updates download the Enterprise data file from the
			// Distributor with the license key, which
			// _51DEGREES_LICENSE_KEY gives.
			// To obtain access to enterprise data files for hosting, please contact us at https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-update_polling_interval-main.go&utm_term=main
			updates := ipi_onpremise.WithLicenseKey(params.LicenseKey)
			if *dataUpdateUrl != "" {
				// Download from the address -url gives in place of the
				// Distributor's. The license key is only added to the
				// Distributor's address, so it goes in this one's query.
				address, err := updateUrl(*dataUpdateUrl, params.LicenseKey)
				if err != nil {
					return err
				}
				updates = ipi_onpremise.WithDataUpdateUrl(address)
			}

			// LowMemory reads the data file as it is needed rather than
			// loading it all, because updates download the Enterprise data
			// file, which is several gigabytes.
			config := ipi_interop.NewConfigIpi(ipi_interop.LowMemory)

			//Create on-premise engine
			engine, err := ipi_onpremise.New(
				// Optimized config provided
				ipi_onpremise.WithConfigIpi(config),
				// Path to your data file
				ipi_onpremise.WithDataFile(params.DataFile),

				// The license key, which the engine adds to the
				// Distributor's address, or the address -url gives.
				updates,

				// Ask for the product -product names, as the Distributor
				// spells it, such as IPIV4Enterprise. When none is given,
				// no product is named. The first request then names no
				// product, and when the License Key holds several products
				// the engine asks for the product of the data file it has
				// loaded.
				ipi_onpremise.WithProduct(*product),

				// Enable automatic updates.
				ipi_onpremise.WithAutoUpdate(true),

				// Set the frequency in seconds that the pipeline should
				// check for updates to data files. The example checks every
				// few seconds so that an update can arrive while it waits
				// between its two sets of lookups. A recommended polling
				// interval in a production environment is around 30
				// minutes, which is 1800 seconds.
				ipi_onpremise.WithPollingInterval(3),

				// Set the maximum random time in seconds added to each polling
				// interval. A new time from 0 up to this maximum is drawn for
				// each interval, so that multiple instances in a datacenter do
				// not poll for updates at the same time. A recommended amount
				// in production environments is 600 seconds.
				ipi_onpremise.WithRandomization(1),

				// Enable update on startup, the auto update system
				// will be used to check for an update before the
				// IP intelligence engine is created.
				ipi_onpremise.WithUpdateOnStart(false),

				// LowMemory keeps the data file open, so the engine reads a
				// temp copy and an update can replace the original.
				ipi_onpremise.WithTempDataCopy(true),

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

			free := common.IsFreeDataFile(engine, common.NetworkProperties)

			//process before file has been updated
			processEvidence(engine, free, evidences[0])
			processEvidence(engine, free, evidences[1])

			<-time.After(30 * time.Second)

			// The data file can have been replaced, so it is checked again
			// before the second pair of addresses.
			free = common.IsFreeDataFile(engine, common.NetworkProperties)

			//process after file has been updated
			processEvidence(engine, free, evidences[2])
			processEvidence(engine, free, evidences[3])

			return nil
		})
}
