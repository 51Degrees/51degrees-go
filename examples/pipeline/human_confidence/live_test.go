/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
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

package main

// The passive call the example makes, against the live cloud service named
// by _51DEGREES_CLOUD_ENDPOINT, with keys from the environment. The request
// names HumanConfidence and HumanConfidenceSource, so the answer carries
// both whatever the key's own property list holds. A test with no key skips
// and names the variable it needs. Keys are never printed.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	// bespokeKeyVariable holds a Resource Key with the Bespoke product,
	// which gets the source.
	bespokeKeyVariable = "_51DEGREES_RESOURCE_KEY_BESPOKE"
	// basicKeyVariable holds a Resource Key outside the Bespoke product,
	// which gets no source.
	basicKeyVariable = "_51DEGREES_RESOURCE_KEY_BASIC"
	liveValues       = "derived.humanconfidence,derived.humanconfidencesource"
	liveUserAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36"
	liveTimeoutSeconds = 20
)

// liveKey returns the key in the variable, or skips the test naming it.
func liveKey(t *testing.T, name string) string {
	t.Helper()
	key := strings.TrimSpace(os.Getenv(name))
	if key == "" {
		t.Skipf("%s is not set, so this live cloud test is skipped. "+
			"_51DEGREES_CLOUD_ENDPOINT names the service.", name)
	}
	return key
}

// livePassive posts a browser's User-Agent to the key's JSON path, at the
// API base the example reads, naming the two properties. It returns the
// answer's derived object, empty when the answer has none.
func livePassive(t *testing.T, key string) map[string]any {
	t.Helper()
	redact := strings.NewReplacer(key, "<key>")
	client := &http.Client{Timeout: liveTimeoutSeconds * time.Second}
	response, err := client.PostForm(
		loadConfig(os.Getenv).API+key+".json",
		url.Values{"user-agent": {liveUserAgent}, "values": {liveValues}})
	if err != nil {
		t.Fatalf("the passive call failed, %s", redact.Replace(err.Error()))
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the passive call gave HTTP %d, %s",
			response.StatusCode, redact.Replace(string(body)))
	}
	var answer struct {
		Derived map[string]any `json:"derived"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatalf("the answer is not JSON, %v", err)
	}
	if answer.Derived == nil {
		return map[string]any{}
	}
	return answer.Derived
}

func TestLiveBespokeKeyGetsSourcePassive(t *testing.T) {
	derived := livePassive(t, liveKey(t, bespokeKeyVariable))
	if derived["humanconfidencesource"] != "Passive" {
		t.Errorf("a request with no browser tests is passive, got %v", derived)
	}
	switch derived["humanconfidence"] {
	case "High", "Medium", "Low":
	default:
		t.Errorf("a passive level is one of the plain levels, got %v", derived)
	}
}

func TestLiveKeyOutsideBespokeGetsNoSource(t *testing.T) {
	derived := livePassive(t, liveKey(t, basicKeyVariable))
	if source := derived["humanconfidencesource"]; source != nil {
		t.Errorf("a key outside Bespoke gets no source, got %v", derived)
	}
}
