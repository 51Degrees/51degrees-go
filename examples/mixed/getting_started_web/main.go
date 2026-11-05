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
//   - clientIp           — the detected client IP address
//   - deviceDetection    — device properties detected from the request headers
//   - ipIntelligence     — IP intelligence properties for the client IP
//   - enterpriseDataFile — where to ask for a copy of the Enterprise data
//     files, given when a property needs them
//
// Both engines are queried concurrently for each request.
//
// It runs with the free Lite data files, downloading one that is missing, and
// gives the properties that need the Enterprise data files as needing them.
// These environment variables give other data files:
//
//	DATA_FILE    Path to an IP Intelligence .ipi data file
//	DD_DATA_FILE Path to a Device Detection .hash data file
//
// Run (from the examples/mixed directory — the mixed examples are a separate
// Go module, so they cannot be run from the repository root):
//
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
	"strings"
	"sync"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	ddOnpremise "github.com/51Degrees/51degrees-go/device-detection/v4/onpremise"
	"github.com/51Degrees/51degrees-go/examples/mixed/common"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_onpremise"
)

// contactUs is where to ask for a copy of the Enterprise data files, which
// have the properties the free data files leave out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-mixed-getting_started_web-main.go&utm_term=enterprise-data-file"

// ddResponseProperties lists device detection properties included in the JSON
// response. The free data file has the platform and browser ones, and the
// hardware ones need the Enterprise data file.
var ddResponseProperties = []string{
	"HardwareName",
	"HardwareModel",
	"PlatformName",
	"PlatformVersion",
	"BrowserName",
	"BrowserVersion",
}

// ipiResponseProperties lists IP intelligence properties included in the JSON
// response. The free data file has the registered ones, and the location ones
// need the Enterprise data file.
var ipiResponseProperties = []string{
	"RegisteredCountry",
	"RegisteredName",
	"RegisteredOwner",
	"Country",
	"Town",
	"Latitude",
	"Longitude",
}

// MixedResponse is the JSON structure returned by the HTTP handler.
type MixedResponse struct {
	ClientIP           string            `json:"clientIp"`
	DeviceDetection    map[string]string `json:"deviceDetection"`
	IpIntelligence     map[string]string `json:"ipIntelligence"`
	EnterpriseDataFile string            `json:"enterpriseDataFile,omitempty"`
}

// mixedHandler holds references to both engines and implements http.Handler.
type mixedHandler struct {
	ddEngine  *ddOnpremise.Engine
	ipiEngine *ipi_onpremise.Engine
	// ipiFree is whether the IP intelligence data file is a free one.
	ipiFree bool
}

// newMixedHandler returns a handler for both engines.
func newMixedHandler(
	ddEngine *ddOnpremise.Engine,
	ipiEngine *ipi_onpremise.Engine) *mixedHandler {
	return &mixedHandler{
		ddEngine:  ddEngine,
		ipiEngine: ipiEngine,
		ipiFree:   common.IsFreeIpData(ipiEngine, ipiResponseProperties),
	}
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
		ddResults = nil
	} else {
		defer ddResults.Free()
	}
	if ipiErr != nil {
		log.Printf("IP Intelligence error: %v", ipiErr)
	}

	resp := buildResponse(clientIP, ddResults, ipiValues, h.ipiFree)

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

// buildResponse assembles the MixedResponse from both engines' results. A
// property the data file does not have is given as needing the Enterprise
// data file, and the response then says where to ask for a copy.
func buildResponse(
	clientIP string,
	results *dd.ResultsHash,
	values ipi_interop.Values,
	ipiFree bool) MixedResponse {
	missing := false

	ddMap := make(map[string]string)
	if results != nil {
		for _, prop := range ddResponseProperties {
			if !common.HasDeviceProperty(results, prop) {
				ddMap[prop] = common.NeedsEnterprise
				missing = true
				continue
			}
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
	for _, prop := range ipiResponseProperties {
		val, weight, found := values.GetValueWeightByProperty(prop)
		switch {
		case found:
			ipiMap[prop] = fmt.Sprintf("%v (weight: %.2f)", val, weight)
		case ipiFree && !common.FreeIpProperties[prop]:
			ipiMap[prop] = common.NeedsEnterprise
			missing = true
		}
	}

	resp := MixedResponse{
		ClientIP:        clientIP,
		DeviceDetection: ddMap,
		IpIntelligence:  ipiMap,
	}
	if missing {
		resp.EnterpriseDataFile = contactUs
	}
	return resp
}

// newEngines creates both engines from the data files the environment
// variables name, or from the free ones.
func newEngines() (*ddOnpremise.Engine, *ipi_onpremise.Engine, error) {
	ipiDataFile := common.DataFile(
		"DATA_FILE", common.IpIntelligenceFolder, common.IpIntelligenceDataFile)
	ddDataFile := common.DataFile(
		"DD_DATA_FILE", common.DeviceDetectionFolder, common.DeviceDetectionDataFile)

	ddConfig := dd.NewConfigHash(dd.LowMemory)
	ddEngine, err := ddOnpremise.New(
		ddOnpremise.WithConfigHash(ddConfig),
		ddOnpremise.WithDataFile(ddDataFile),
		ddOnpremise.WithAutoUpdate(false),
		ddOnpremise.WithTempDataCopy(false),
		ddOnpremise.WithProperties(ddResponseProperties),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Device Detection engine: %w", err)
	}

	ipiConfig := ipi_interop.NewConfigIpi(ipi_interop.LowMemory)
	ipiEngine, err := ipi_onpremise.New(
		ipi_onpremise.WithConfigIpi(ipiConfig),
		ipi_onpremise.WithDataFile(ipiDataFile),
		ipi_onpremise.WithAutoUpdate(false),
		ipi_onpremise.WithTempDataCopy(false),
		ipi_onpremise.WithProperties(ipiResponseProperties),
	)
	if err != nil {
		ddEngine.Stop()
		return nil, nil, fmt.Errorf("failed to create IP Intelligence engine: %w", err)
	}

	log.Printf("Device Detection data: %s", ddDataFile)
	log.Printf("IP Intelligence data:  %s", ipiDataFile)
	return ddEngine, ipiEngine, nil
}

func main() {
	ddEngine, ipiEngine, err := newEngines()
	if err != nil {
		log.Fatal(err)
	}
	defer ddEngine.Stop()
	defer ipiEngine.Stop()

	http.Handle("/", newMixedHandler(ddEngine, ipiEngine))

	const addr = ":8080"
	log.Printf("Mixed Getting Started Web Example — listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
