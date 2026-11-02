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
// engines together inside an HTTP handler.
//
// The server listens on port 8080 and responds to every request with a JSON
// document containing:
//   - clientIp         — the detected client IP address
//   - deviceDetection  — device properties detected from the request headers
//   - ipIntelligence   — IP intelligence properties for the client IP
//
// Both engines are queried concurrently for each request.
//
// Required environment variables:
//
//	DATA_FILE    Path to the IP Intelligence .ipi data file
//	DD_DATA_FILE Path to the Device Detection .hash data file
//
// Run (from the examples/mixed directory — the mixed examples are a separate
// Go module, so they cannot be run from the repository root):
//
//	DATA_FILE=../../51Degrees-EnterpriseIpiV41.ipi \
//	DD_DATA_FILE=../../51Degrees-LiteV4.1.hash \
//	go run ./getting_started_web
//
// Then in another terminal:
//
//	curl http://localhost:8080/
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/51Degrees/device-detection-go/v4/dd"
	ddOnpremise "github.com/51Degrees/device-detection-go/v4/onpremise"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_interop"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_onpremise"
)

// ddResponseProperties lists device detection properties included in the JSON response.
var ddResponseProperties = []string{
	"HardwareName",
	"HardwareModel",
	"PlatformName",
	"PlatformVersion",
	"BrowserName",
	"BrowserVersion",
}

// MixedResponse is the JSON structure returned by the HTTP handler.
type MixedResponse struct {
	ClientIP        string            `json:"clientIp"`
	DeviceDetection map[string]string `json:"deviceDetection"`
	IpIntelligence  map[string]string `json:"ipIntelligence"`
}

// mixedHandler holds references to both engines and implements http.Handler.
type mixedHandler struct {
	ddEngine  *ddOnpremise.Engine
	ipiEngine *ipi_onpremise.Engine
}

// ServeHTTP extracts evidence from the request, queries both engines concurrently,
// and writes a JSON response.
func (h *mixedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	evidence := extractDDEvidence(r)
	clientIP := extractClientIP(r)

	var (
		ddResults     *dd.ResultsHash
		ipiValues     ipi_interop.Values
		ddErr, ipiErr error
		wg            sync.WaitGroup
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		ddResults, ddErr = h.ddEngine.Process(evidence)
	}()
	go func() {
		defer wg.Done()
		ipiValues, ipiErr = h.ipiEngine.Process(clientIP)
	}()
	wg.Wait()

	if ddErr != nil {
		log.Printf("Device Detection error: %v", ddErr)
	} else {
		defer ddResults.Free()
	}
	if ipiErr != nil {
		log.Printf("IP Intelligence error: %v", ipiErr)
	}

	resp := buildResponse(clientIP, ddResults, ipiValues)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// extractDDEvidence converts all HTTP request headers into a DD evidence slice.
func extractDDEvidence(r *http.Request) []ddOnpremise.Evidence {
	evidence := make([]ddOnpremise.Evidence, 0, len(r.Header))
	for key, values := range r.Header {
		if len(values) > 0 {
			evidence = append(evidence, ddOnpremise.Evidence{
				Prefix: dd.HttpHeaderString,
				Key:    key,
				Value:  values[0],
			})
		}
	}
	return evidence
}

// extractClientIP determines the client IP from the request, consulting
// X-Forwarded-For and X-Real-Ip headers before falling back to RemoteAddr.
func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
	}
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// buildResponse assembles the MixedResponse from both engines' results.
func buildResponse(clientIP string, results *dd.ResultsHash, values ipi_interop.Values) MixedResponse {
	ddMap := make(map[string]string)
	if results != nil {
		for _, prop := range ddResponseProperties {
			hasValues, err := results.HasValues(prop)
			if err != nil || !hasValues {
				continue
			}
			value, err := results.ValuesString(prop, ",")
			if err != nil {
				continue
			}
			ddMap[prop] = value
		}
	}

	ipiMap := make(map[string]string)
	for prop := range values {
		if prop == "Mcc" {
			// Only Mcc property has weight
			val, weight, found := values.GetValueWeightByProperty(prop)
			if !found {
				continue
			}
			ipiMap[prop] = fmt.Sprintf("%v (weight: %.2f)", val, weight)
		} else {
			// All other properties are non-weighted
			val, found := values.GetValueByProperty(prop)
			if !found {
				continue
			}
			ipiMap[prop] = fmt.Sprintf("%v", val)
		}
	}

	return MixedResponse{
		ClientIP:        clientIP,
		DeviceDetection: ddMap,
		IpIntelligence:  ipiMap,
	}
}

func main() {
	ipiDataFile := os.Getenv("DATA_FILE")
	ddDataFile := os.Getenv("DD_DATA_FILE")

	ddConfig := dd.NewConfigHash(dd.LowMemory)
	ddEngine, err := ddOnpremise.New(
		ddOnpremise.WithConfigHash(ddConfig),
		ddOnpremise.WithDataFile(ddDataFile),
		ddOnpremise.WithAutoUpdate(false),
		ddOnpremise.WithTempDataCopy(false),
		ddOnpremise.WithProperties(ddResponseProperties),
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

	h := &mixedHandler{ddEngine: ddEngine, ipiEngine: ipiEngine}
	http.Handle("/", h)

	const addr = ":8080"
	log.Printf("Mixed Getting Started Web Example — listening on %s", addr)
	log.Printf("Device Detection data: %s", ddDataFile)
	log.Printf("IP Intelligence data:  %s", ipiDataFile)
	log.Fatal(http.ListenAndServe(addr, nil))
}
