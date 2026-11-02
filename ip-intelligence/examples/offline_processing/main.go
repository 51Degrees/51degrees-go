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
@example examples/onpremise/offline_processing.go
# Offline processing example of using 51Degrees IP intelligence.

This example demonstrates one possible use of the 51Degrees on-premise IP intelligence
API and data for offline data processing. It also demonstrates that you can reuse the
retrieved results for multiple uses and only then release it.

This example is available in full on [GitHub](https://github.com/51Degrees/ip-intelligence-go-examples/tree/main/ipi_onpremise/offline_processing).

@include{doc} example-require-datafile-ipi.txt

@include{doc} example-how-to-run-ipi.txt

## In detail, the example shows how to

### 1. Specify config for engine:

This setting specifies the performance profile that will be used when initializing the C library.
<br/>
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
<br/>
```
ipi_onpremise.WithConfigIpi(config)
```
<br/>
<b>WithDataFile</b> sets the path to the local data file, this parameter is required to start the engine
<br/>
```
ipi_onpremise.WithDataFile(params.DataFile),
```
<br/>
<b>WithAutoUpdate</b> enables or disables auto update
<br/>
```
ipi_onpremise.WithAutoUpdate(false),
```
<br/>
<b>WithProperties</b> (Optional) Configures the list of properties to be returned. If not configured - all properties will be returned.

```
ipi_onpremise.WithProperties(common.Properties),
```
<br/>
### 3. Run evidence processing with parameters

```
runOfflineProcessing(engine, params)
```
<br/>
### 4. Load evidence one by one from the EvidenceYaml file

```
file, err := os.OpenFile(evidenceFilePath, os.O_RDONLY, 0444)
if err != nil {
	log.Fatalf("Failed to open file \"%s\".\n", evidenceFilePath)
}
defer func() {
	if err := file.Close(); err != nil {
		log.Fatalf("Failed to close file \"%s\".\n", evidenceFilePath)
	}
}()
```
<br/>
### 5. Create a new file for writing processed evidence

```
outFile, err := os.Create(outputFilePath)
if err != nil {
	log.Fatalf("Failed to create file %s.\n", outputFilePath)
}
defer func() {
	if err := outFile.Close(); err != nil {
		log.Fatalf("Failed to close file \"%s\".\n", outputFilePath)
	}
}()
```
<br/>
### 6. Get values by property

```
// For most properties (non-weighted):
value, found := result.GetValueByProperty(property)

// For MCC property (weighted):
value, weight, found := result.GetValueWeightByProperty(property)
```
*/

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/51Degrees/ip-intelligence-go/v4/examples/common"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_interop"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_onpremise"
	"github.com/goccy/go-yaml"
)

// PropertiesData represents a structured data type containing various property fields commonly associated with network or location data.
type PropertiesData struct {
	IpRangeStart      any `yaml:"ip-range-start"`
	IpRangeEnd        any `yaml:"ip-range-end"`
	AccuracyRadiusMin any `yaml:"accuracy-radius-min"`
	AccuracyRadiusMax any `yaml:"accuracy-radius-max"`
	RegisteredCountry any `yaml:"registered-country"`
	RegisteredName    any `yaml:"registered-name"`
	Longitude         any `yaml:"longitude"`
	Latitude          any `yaml:"latitude"`
	Areas             any `yaml:"areas"`
	Mcc               any `yaml:"mcc"`
}

// getIpi retrieves property data and associated comments for the specified IP address using the provided engine.
// Returns structured property data, a map of YAML comments, and an error if one occurs.
func getIpi(engine *ipi_onpremise.Engine, IpAddress string) (*PropertiesData, yaml.CommentMap, error) {
	result, err := engine.Process(IpAddress)
	if err != nil {
		log.Printf("Error processing Getting Started Example: %v", err)
		return nil, nil, err
	}

	data := &PropertiesData{}

	comments := yaml.CommentMap{}

	for _, property := range common.Properties {
		var value any
		var weight float64
		var found bool

		// Only Mcc property has weight
		if property == "Mcc" {
			value, weight, found = result.GetValueWeightByProperty(property)
		} else {
			value, found = result.GetValueByProperty(property)
		}

		if !found {
			continue
		}

		switch property {
		case "IpRangeStart":
			data.IpRangeStart = value
			// No weight comment for non-weighted properties
		case "IpRangeEnd":
			data.IpRangeEnd = value
			// No weight comment for non-weighted properties
		case "AccuracyRadiusMin":
			data.AccuracyRadiusMin = value
			// No weight comment for non-weighted properties
		case "AccuracyRadiusMax":
			data.AccuracyRadiusMax = value
			// No weight comment for non-weighted properties
		case "RegisteredCountry":
			data.RegisteredCountry = value
			// No weight comment for non-weighted properties
		case "RegisteredName":
			data.RegisteredName = value
			// No weight comment for non-weighted properties
		case "Longitude":
			data.Longitude = value
			// No weight comment for non-weighted properties
		case "Latitude":
			data.Latitude = value
			// No weight comment for non-weighted properties
		case "Areas":
			data.Areas = value
			// No weight comment for non-weighted properties
		case "Mcc":
			data.Mcc = value
			// Only Mcc has weight comment
			comments["$.mcc"] = []*yaml.Comment{
				yaml.LineComment(fmt.Sprintf("Weight: %.1f", weight)),
			}
		}
	}

	return data, comments, nil
}

// runOfflineProcessing processes an evidence YAML file, retrieves IP-related data, and writes the output to a new YAML file.
func runOfflineProcessing(engine *ipi_onpremise.Engine, params *common.ExampleParams) {
	evidenceFilePath := common.GetFilePathByPath(params.EvidenceYaml)
	evDir := filepath.Dir(evidenceFilePath)
	evBase := strings.TrimSuffix(filepath.Base(evidenceFilePath), filepath.Ext(evidenceFilePath))
	outputFilePath := fmt.Sprintf("%s/%s.processed.yml", evDir, evBase)

	outFile, err := os.Create(outputFilePath)
	if err != nil {
		log.Fatalf("Failed to create file %s.\n", outputFilePath)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			log.Fatalf("Failed to close file \"%s\".\n", outputFilePath)
		}
	}()

	// Open the Evidence Records file for processing
	file, err := os.OpenFile(evidenceFilePath, os.O_RDONLY, 0444)
	if err != nil {
		log.Fatalf("Failed to open file \"%s\".\n", evidenceFilePath)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Fatalf("Failed to close file \"%s\".\n", evidenceFilePath)
		}
	}()

	// Create a new scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Loop through the file and read each line
	for scanner.Scan() {
		line := scanner.Text() // Get the line as a string

		var doc map[string]string
		if err := yaml.Unmarshal([]byte(line), &doc); err != nil {
			log.Printf("Failed to unmarshal line \"%s\".\n", line)
			continue
		}

		// if the line is empty - skip this line
		if doc == nil {
			continue
		}

		if _, err = outFile.WriteString("---\n"); err != nil {
			log.Printf("Failed to write end for file \"%s\". %v\n", outputFilePath, err)
			continue
		}

		// process client-ip from the file
		data, comments, err := getIpi(engine, doc["server.client-ip"])
		if err != nil {
			log.Printf("Failed to get Ipi for line \"%s\".\n", line)
			continue
		}

		// marshal data to yaml
		dataYml, err := yaml.MarshalWithOptions(data, yaml.WithComment(comments))
		if err != nil {
			log.Printf("Failed to marshal Ipi for line \"%s\".\n", line)
			continue
		}

		if _, err = outFile.Write(dataYml); err != nil {
			log.Printf("Failed to write to file \"%s\".\n", outputFilePath)
			continue
		}
	}

	if _, err = outFile.WriteString("...\n"); err != nil {
		log.Printf("Failed to write end for file \"%s\". %v\n", outputFilePath, err)
	}

	// Check for errors during the scan
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading file: %s", err)
	}
}

// main is the entry point of the application. It initializes configuration, engine, and executes offline processing.
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
			runOfflineProcessing(engine, params)

			engine.Stop()

			return nil
		})
}
