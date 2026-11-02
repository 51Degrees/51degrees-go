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
@example examples/onpremise/performance.go
# The example illustrates a "clock-time" benchmark for assessing detection speed.

It's important to understand the trade-offs between performance, memory usage and accuracy, that
the 51Degrees pipeline configuration makes available, and this example shows a range of
different configurations to illustrate the difference in performance.

By default, this example runs in multi-threaded mode using all available CPU cores for optimal
performance. Use the `--single` flag to run in single-threaded mode for comparison.

Requesting properties from a single component
reduces detection time compared with requesting properties from multiple components. If you
don't specify any properties to detect, then all properties are detected.

This example is available in full on [GitHub](https://github.com/51Degrees/ip-intelligence-go-examples/tree/main/ipi_onpremise/performance).

@include{doc} example-require-datafile-ipi.txt

@include{doc} example-how-to-run-ipi.txt

## Usage
```
# Multi-threaded mode (default)
go run .

# Single-threaded mode
go run . --single
```

## In detail, the example shows how to

### 1. Specify config for engine:

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
	// Set only 1 parameter for getting data
	ipi_onpremise.WithProperties([]string{"RegisteredName"}),
)
```
<br/>
<b>WithConfigIpi</b> allows to configure the Ipi matching algorithm.

```
ipi_onpremise.WithConfigIpi(config)
```
<br/>
<b>WithDataFile</b> sets the path to the local data file, this parameter is required to start the engine

```
ipi_onpremise.WithDataFile(params.DataFile),
```
<br/>
<b>WithAutoUpdate</b> enables or disables auto update

```
ipi_onpremise.WithAutoUpdate(false),
```
<br/>
<b>WithProperties</b> configures an Engine with a comma-separated list of manager properties derived from the provided slice.te

```
ipi_onpremise.WithProperties([]string{"RegisteredName"}),
```
<br/>
### 3. Run evidence processing with parameters and get the report as returned value

```
report, err := runPerformance(engine, params)
```
<br/>
### Expected output (performance_report.log):

Multi-threaded mode (default):
```
Average 0.00075 ms per Evidence Record
Average 1333333.33 detections per second
Total Evidence Records: 20000
Iteration Count: 1
Processed Evidence Records: 120000
Number of CPUs: 12
```

Single-threaded mode (--single flag):
```
Average 0.00202 ms per Evidence Record
Average 495867.77 detections per second
Total Evidence Records: 20000
Iteration Count: 1
Processed Evidence Records: 120000
Number of CPUs: 12
```
*/

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/51Degrees/ip-intelligence-go/v4/examples/common"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_interop"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_onpremise"
	"github.com/goccy/go-yaml"
)

// reportFile defines the default path and filename for storing performance test results.
const reportFile = "performance_report.log"

// iterationCount defines the fixed number of iterations to be used for processing or performance testing loops.
const iterationCount = 5 // TODO: implement/remove

// readYaml reads YAML data from a file, processes it line by line, and parses it into an IpEvidences collection.
// It takes ExampleParams as input, retrieves the file path, and reads evidence records from the specified YAML file.
// Returns IpEvidences containing the parsed data and an error if file operations or unmarshalling fail.
func readYaml(params *common.ExampleParams) (common.IpEvidences, error) {
	evidenceFilePath := common.GetFilePathByPath(params.EvidenceYaml)

	// Open the Evidence Records file for processing
	file, err := os.OpenFile(evidenceFilePath, os.O_RDONLY, 0444)
	if err != nil {
		return common.IpEvidences{}, err
	}

	defer func() {
		if err := file.Close(); err != nil {
			log.Fatalf("Failed to close file \"%s\".\n", evidenceFilePath)
		}
	}()

	// Create a new scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	evidences := make(common.IpEvidences, 0)

	// Loop through the file and read each line
	for scanner.Scan() {
		line := scanner.Text() // Get the line as a string

		var doc map[string]string
		if err := yaml.Unmarshal([]byte(line), &doc); err != nil {
			log.Printf("Failed to unmarshal line \"%s\".\n", line)
			continue
		}

		if doc == nil {
			continue
		}

		evidences.Add(doc["server.client-ip"])
	}

	return evidences, nil
}

// processEvidenceBatch processes a batch of evidence IP addresses using the provided engine over multiple iterations.
// The function updates the evidence processing count in the report and logs errors or missing data as encountered.
// TODO: actually use or remove this:
func processEvidenceBatch(engine *ipi_onpremise.Engine, wg *sync.WaitGroup, evidenceBatch []string, report *common.Report, iterations int) {
	defer wg.Done()

	for iter := 0; iter < iterations; iter++ {
		for _, ipAddress := range evidenceBatch {
			atomic.AddUint64(&report.EvidenceProcessed, 1)

			result, err := engine.Process(ipAddress)
			if err != nil {
				log.Fatalln(err)
			}

			for _, property := range []string{"RegisteredName"} {
				// don't use the property in the current step, only processing data
				if _, found := result.GetValueByProperty(property); !found {
					log.Printf("Not found values for the next property %s for address %s", property, ipAddress)
				}
			}
		}
	}
}

// runPerformance executes a performance test by processing evidence data with the given engine and parameters.
// It disables garbage collection during execution and processes evidence using configured concurrency.
// Returns a performance report and an error if the execution fails.
func runPerformance(engine *ipi_onpremise.Engine, params *common.ExampleParams, numThreads int) (*common.Report, error) {
	// Getting evidences from the file (this can use GC)
	evidences, err := readYaml(params)
	if err != nil {
		log.Fatalf("Failed to read yaml files.\n")
		return nil, err
	}

	evidenceSlice := []string(evidences)

	const totalDetections = 120000
	log.Printf("Loaded %d IP addresses from evidence file", len(evidenceSlice))
	log.Printf("Will process %d total detections for performance test with %d threads", totalDetections, numThreads)

	actReport := &common.Report{
		IterationCount:    1, // Single iteration of totalDetections
		EvidenceCount:     uint64(len(evidenceSlice)),
		EvidenceProcessed: 0,
	}

	// Force garbage collection before the test
	log.Printf("Running GC before performance test...")
	runtime.GC()
	runtime.GC() // Run twice to ensure thorough cleanup

	// Disable garbage collection during performance test
	log.Printf("Disabling GC for performance test...")
	debug.SetGCPercent(-1)

	if numThreads == 1 {
		// Single-threaded mode with one reusable ResultsIpi
		log.Printf("Creating reusable ResultsIpi object for single-threaded performance...")
		reusableResults := engine.NewResultsIpi()
		defer reusableResults.Free()

		log.Printf("Starting single-threaded performance test...")

		// Start timing after GC is disabled and objects are created
		startProcessTime := time.Now()

		// Process evidence like the single-threaded test: one continuous loop
		for i := 0; i < totalDetections; i++ {
			ip := evidenceSlice[i%len(evidenceSlice)]
			atomic.AddUint64(&actReport.EvidenceProcessed, 1)

			result, err := engine.ProcessWithResults(ip, reusableResults)
			if err != nil {
				log.Fatalln(err)
			}

			// Access property to ensure full processing
			if _, found := result.GetValueByProperty("RegisteredName"); !found {
				log.Printf("RegisteredName not found for IP %s", ip)
			}
		}

		actReport.ProcessingTime = time.Since(startProcessTime).Milliseconds()
	} else {
		// Multi-threaded mode with thread-local reusable ResultsIpi objects
		log.Printf("Starting multi-threaded performance test with %d threads...", numThreads)

		// Start timing before creating workers
		startProcessTime := time.Now()

		// Create channels for work distribution
		workCh := make(chan int, numThreads*2)
		var wg sync.WaitGroup

		// Start worker goroutines
		for t := 0; t < numThreads; t++ {
			wg.Add(1)
			go func(threadID int) {
				defer wg.Done()

				// Each goroutine gets its own reusable ResultsIpi object
				reusableResults := engine.NewResultsIpi()
				defer reusableResults.Free()

				// Process work items
				for i := range workCh {
					ip := evidenceSlice[i%len(evidenceSlice)]
					atomic.AddUint64(&actReport.EvidenceProcessed, 1)

					result, err := engine.ProcessWithResults(ip, reusableResults)
					if err != nil {
						log.Printf("Thread %d: error processing IP %s: %v", threadID, ip, err)
						continue
					}

					// Access property to ensure full processing
					if _, found := result.GetValueByProperty("RegisteredName"); !found {
						log.Printf("Thread %d: RegisteredName not found for IP %s", threadID, ip)
					}
				}
			}(t)
		}

		// Distribute work
		for i := 0; i < totalDetections; i++ {
			workCh <- i
		}
		close(workCh)

		// Wait for all workers to complete
		wg.Wait()

		actReport.ProcessingTime = time.Since(startProcessTime).Milliseconds()
	}

	// Re-enable garbage collection after test
	log.Printf("Re-enabling GC after performance test...")
	debug.SetGCPercent(100) // Default GC target percentage

	return actReport, nil
}

func main() {
	// Parse command-line flags
	singleThreaded := flag.Bool("single", false, "Run in single-threaded mode (default is multi-threaded)")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nIP Intelligence Performance Test\n")
		fmt.Fprintf(os.Stderr, "By default, runs in multi-threaded mode using all CPU cores for optimal performance.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	common.RunExample(
		func(params *common.ExampleParams) error {
			// Determine number of threads - default to multi-threaded
			numThreads := runtime.NumCPU()
			if *singleThreaded {
				numThreads = 1
			}

			log.Printf("Starting IP Intelligence Performance Test (with GC control)")
			log.Printf("Using data file: %s", params.DataFile)
			log.Printf("Running with %d threads", numThreads)

			//Create config
			config := ipi_interop.NewConfigIpi(ipi_interop.InMemory)
			config.SetConcurrency(uint16(numThreads))

			//Create on-premise engine
			engine, err := ipi_onpremise.New(
				// Optimized config provided
				ipi_onpremise.WithConfigIpi(config),
				// Path to your data file
				ipi_onpremise.WithDataFile(params.DataFile),
				// Enable automatic updates.
				ipi_onpremise.WithAutoUpdate(false),
				ipi_onpremise.WithTempDataCopy(false),
				// Set only 1 parameter for getting data
				ipi_onpremise.WithProperties([]string{"RegisteredName"}),
			)
			if err != nil {
				log.Fatalf("Failed to create engine: %v", err)
			}

			// Run example
			report, err := runPerformance(engine, params, numThreads)
			if err != nil {
				log.Fatalf("Failed to run performance: %v", err)
			}

			// Validation to make sure all detections have been processed
			if report.EvidenceProcessed != 120000 {
				log.Fatalln("Not all detections have been processed.")
			}

			// print report to the file
			if err := report.PrintReport(reportFile); err != nil {
				log.Fatalf("Failed to print report: %v", err)
			}

			engine.Stop()

			return nil
		})
}
