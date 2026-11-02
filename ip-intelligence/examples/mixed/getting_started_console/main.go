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

// Package main demonstrates using 51Degrees Device Detection and IP Intelligence
// engines together in a console application.
//
// The example processes four hardcoded evidence records (User-Agent header + client IP)
// and prints the detected device properties alongside IP intelligence data.
//
// Required environment variables:
//
//	DATA_FILE    Path to the IP Intelligence .ipi data file
//	DD_DATA_FILE Path to the Device Detection .hash data file
package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"sync"

	"github.com/51Degrees/device-detection-go/v4/dd"
	ddOnpremise "github.com/51Degrees/device-detection-go/v4/onpremise"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_interop"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_onpremise"
)

// testCase holds a single set of evidence for the mixed example.
type testCase struct {
	name            string
	userAgent       string
	secChUa         string
	secChUaMobile   string
	secChUaPlatform string
	clientIP        string
}

// hardcodedCases provides representative evidence records covering mobile,
// desktop, tablet, and IPv6 scenarios.
var hardcodedCases = []testCase{
	{
		name:            "iPhone (iOS, Mobile Safari) - UK IP",
		userAgent:       "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1.2 Mobile/15E148 Safari/604.1",
		secChUaMobile:   "?1",
		secChUaPlatform: `"iOS"`,
		clientIP:        "185.28.167.77",
	},
	{
		name:            "Windows desktop (Chrome) - Chile IP",
		userAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		secChUa:         `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		secChUaMobile:   "?0",
		secChUaPlatform: `"Windows"`,
		clientIP:        "190.53.100.1",
	},
	{
		name:            "iPad (iOS, Mobile Safari) - IPv6",
		userAgent:       "Mozilla/5.0 (iPad; CPU OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
		secChUaMobile:   "?0",
		secChUaPlatform: `"iPadOS"`,
		clientIP:        "fdaa:bbcc:ddee:0:995f:d63a:f2a1:f189",
	},
	{
		name:            "Android Chrome - 8.8.8.8",
		userAgent:       "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.144 Mobile Safari/537.36",
		secChUa:         `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		secChUaMobile:   "?1",
		secChUaPlatform: `"Android"`,
		clientIP:        "8.8.8.8",
	},
}

// ddProperties lists the device detection properties to display.
var ddProperties = []string{
	"HardwareName",
	"HardwareModel",
	"PlatformName",
	"PlatformVersion",
	"BrowserName",
	"BrowserVersion",
}

// buildDDEvidence constructs the DD evidence slice for a test case,
// omitting any header whose value is empty.
func buildDDEvidence(tc testCase) []ddOnpremise.Evidence {
	ev := []ddOnpremise.Evidence{
		{Prefix: dd.HttpHeaderString, Key: "User-Agent", Value: tc.userAgent},
	}
	if tc.secChUa != "" {
		ev = append(ev, ddOnpremise.Evidence{Prefix: dd.HttpHeaderString, Key: "Sec-CH-UA", Value: tc.secChUa})
	}
	if tc.secChUaMobile != "" {
		ev = append(ev, ddOnpremise.Evidence{Prefix: dd.HttpHeaderString, Key: "Sec-CH-UA-Mobile", Value: tc.secChUaMobile})
	}
	if tc.secChUaPlatform != "" {
		ev = append(ev, ddOnpremise.Evidence{Prefix: dd.HttpHeaderString, Key: "Sec-CH-UA-Platform", Value: tc.secChUaPlatform})
	}
	return ev
}

// printDDResults writes device detection property values to stdout.
func printDDResults(results *dd.ResultsHash) {
	fmt.Println("  --- Device Detection ---")
	for _, prop := range ddProperties {
		hasValues, err := results.HasValues(prop)
		if err != nil || !hasValues {
			continue
		}
		value, err := results.ValuesString(prop, ",")
		if err != nil {
			continue
		}
		fmt.Printf("  %-20s %s\n", prop+":", value)
	}
}

// printIPIResults writes all IP intelligence properties to stdout,
// sorted alphabetically for consistent output.
func printIPIResults(values ipi_interop.Values) {
	fmt.Println("\n  --- IP Intelligence ---")
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, prop := range keys {
		if prop == "Mcc" {
			// Only Mcc property has weight
			val, weight, found := values.GetValueWeightByProperty(prop)
			if !found {
				continue
			}
			fmt.Printf("  %-20s %v (weight: %.2f)\n", prop+":", val, weight)
		} else {
			// All other properties are non-weighted
			val, found := values.GetValueByProperty(prop)
			if !found {
				continue
			}
			fmt.Printf("  %-20s %v\n", prop+":", val)
		}
	}
}

// processCase runs one evidence record through both engines concurrently
// and prints the combined results.
func processCase(tc testCase, ddEngine *ddOnpremise.Engine, ipiEngine *ipi_onpremise.Engine) {
	fmt.Printf("\n[%s]\n", tc.name)
	fmt.Printf("  User-Agent: %s\n", tc.userAgent)
	fmt.Printf("  Client IP:  %s\n\n", tc.clientIP)

	var (
		ddResults     *dd.ResultsHash
		ipiValues     ipi_interop.Values
		ddErr, ipiErr error
		wg            sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		ddResults, ddErr = ddEngine.Process(buildDDEvidence(tc))
	}()
	go func() {
		defer wg.Done()
		ipiValues, ipiErr = ipiEngine.Process(tc.clientIP)
	}()
	wg.Wait()

	if ddErr != nil {
		log.Printf("Device Detection error: %v", ddErr)
	} else {
		defer ddResults.Free()
		printDDResults(ddResults)
	}

	if ipiErr != nil {
		log.Printf("IP Intelligence error: %v", ipiErr)
	} else {
		printIPIResults(ipiValues)
	}
}

func main() {
	ipiDataFile := os.Getenv("DATA_FILE")
	if ipiDataFile == "" {
		ipiDataFile = "51Degrees-EnterpriseIpiV41.ipi"
	}
	ddDataFile := os.Getenv("DD_DATA_FILE")
	if ddDataFile == "" {
		ddDataFile = "51Degrees-LiteV4.1.hash"
	}

	ddConfig := dd.NewConfigHash(dd.InMemory)
	ddEngine, err := ddOnpremise.New(
		ddOnpremise.WithConfigHash(ddConfig),
		ddOnpremise.WithDataFile(ddDataFile),
		ddOnpremise.WithAutoUpdate(false),
		ddOnpremise.WithTempDataCopy(false),
		ddOnpremise.WithProperties(ddProperties),
	)
	if err != nil {
		log.Fatalf("Failed to create Device Detection engine: %v", err)
	}
	defer ddEngine.Stop()

	ipiConfig := ipi_interop.NewConfigIpi(ipi_interop.LowMemory)
	ipiEngine, err := ipi_onpremise.New(
		ipi_onpremise.WithConfigIpi(ipiConfig),
		ipi_onpremise.WithDataFile(ipiDataFile),
		ipi_onpremise.WithAutoUpdate(false),
		ipi_onpremise.WithTempDataCopy(false),
	)
	if err != nil {
		log.Fatalf("Failed to create IP Intelligence engine: %v", err)
	}
	defer ipiEngine.Stop()

	log.Println("Mixed Getting Started Console Example")
	log.Printf("Device Detection data: %s", ddDataFile)
	log.Printf("IP Intelligence data:  %s", ipiDataFile)

	for _, tc := range hardcodedCases {
		processCase(tc, ddEngine, ipiEngine)
	}
}
