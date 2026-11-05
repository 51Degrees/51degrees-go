package main

import (
	"fmt"
	"log"
	"time"

	"github.com/51Degrees/51degrees-go/device-detection/v4/onpremise"
	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
	"github.com/51Degrees/51degrees-go/examples/device-detection/onpremise/common"
)

// contactUs is where to ask for a licence key and a copy of the Enterprise
// data file, which automatic updates download.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-onpremise-update_polling_interval-update_polling_interval.go&utm_term=licence-key"

// properties are those the example shows. The free data file has the
// platform and browser ones, and the Enterprise data file has them all.
var properties = []string{
	"HardwareVendor",
	"HardwareName",
	"HardwareModel",
	"PlatformName",
	"PlatformVersion",
	"BrowserName",
	"DeviceType",
}

func processExampleEvidence(engine *onpremise.Engine, evidence []onpremise.Evidence) {
	//Process evidence. There are no results to free when it fails.
	resultsHash, err := engine.Process(evidence)
	if err != nil {
		log.Printf("Failed to process evidence: %v", err)
		return
	}
	defer resultsHash.Free()
	//Get values from results
	for _, property := range properties {
		if !dd_example.HasProperty(resultsHash, property) {
			log.Printf("%s: %s", property, dd_example.NeedsEnterprise)
			continue
		}
		value, _ := resultsHash.ValuesString(property, ",")
		log.Printf("%s: %s", property, value)
	}
	log.Printf("\n")
}

// runWithoutUpdates shows the data file the example has, once, when there is
// no licence key to download updates with.
func runWithoutUpdates(params common.ExampleParams) {
	engine, err := onpremise.New(
		onpremise.WithDataFile(params.DataFile),
		onpremise.WithAutoUpdate(false),
	)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Stop()

	processExampleEvidence(engine, common.ExampleEvidenceDesktop)
	processExampleEvidence(engine, common.ExampleEvidenceMobile)

	fmt.Printf("Automatic updates download the Enterprise data file with a "+
		"licence key, which LICENSE_KEY gives. Contact us for a key and a "+
		"copy at %s\n", contactUs)
}

func main() {
	common.RunExample(
		func(params common.ExampleParams) error {
			if params.LicenseKey == "" {
				runWithoutUpdates(params)
				return nil
			}

			//Create on-premise engine
			engine, err := onpremise.New(
				// Path to your data file
				onpremise.WithDataFile(params.DataFile),

				// For automatic updates to work you will need to provide a license key.
				// A licence key comes with the Enterprise data file, from https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-onpremise-update_polling_interval-update_polling_interval.go&utm_term=main
				onpremise.WithLicenseKey(params.LicenseKey),

				// Enable automatic updates.
				onpremise.WithAutoUpdate(true),

				// Set the frequency in minutes that the pipeline should
				// check for updates to data files. A recommended
				// polling interval in a production environment is
				// around 30 minutes.
				onpremise.WithPollingInterval(3),

				// Set the max amount of time in seconds that should be
				// added to the polling interval. This is useful in datacenter
				// applications where multiple instances may be polling for
				// updates at the same time. A recommended amount in production
				// environments is 600 seconds.
				onpremise.WithRandomization(1),

				// Enable update on startup, the auto update system
				// will be used to check for an update before the
				// device detection engine is created.
				onpremise.WithUpdateOnStart(false),

				// Optionally provide your own file URL
				// onpremise.WithDataUpdateUrl("<custom URL>"),

				// By default a temp copy should be created, unless you are using InMemory performance profile
				// onpremise.WithTempDataCopy(false),

				// File System Watcher is by default enabled
				// onpremise.WithFileWatch(false),

				// By default logging is on
				// onpremise.WithLogging(false),

				// Custom logger implementing LogWriter interface can be passed
				// onpremise.WithCustomLogger()

				// Set properties for checking, default is [] = all properties
				// onpremise.WithProperties([]string{}),
			)

			if err != nil {
				log.Fatalf("Failed to create engine: %v", err)
			}

			defer engine.Stop()

			//process before file has been updated
			processExampleEvidence(engine, common.ExampleEvidenceDesktop)
			processExampleEvidence(engine, common.ExampleEvidenceMobile)

			<-time.After(20 * time.Second)

			//process again after the file has presumably been updated
			processExampleEvidence(engine, common.ExampleEvidenceDesktop)
			processExampleEvidence(engine, common.ExampleEvidenceMobile)

			return nil
		},
	)
}
