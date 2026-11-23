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

// Live tests of a passive call like the example's, against the staging cloud
// named by CLOUD_STAGING_ENDPOINT, because the production cloud does not
// serve HumanConfidenceSource or a passive level other than Unknown yet. The
// request sends a browser's User-Agent and names HumanConfidence and
// HumanConfidenceSource, so the answer carries both whatever the key's own
// property list holds. A test without its endpoint or key skips and names
// the variable it needs. Failure messages hide the key, the staging API base
// and its host.

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
	// stagingEndpointVariable holds the staging cloud's API base. It has no
	// default, so these tests never call another service.
	stagingEndpointVariable = "CLOUD_STAGING_ENDPOINT"
	// stagingBespokeKeyVariable holds a Resource Key with the Bespoke
	// product that the staging cloud knows, which gets the source.
	stagingBespokeKeyVariable = "RESOURCE_KEY_BESPOKE_STAGING"
	// basicKeyVariable holds a Resource Key outside the Bespoke product,
	// which gets the source as null.
	basicKeyVariable = "_51DEGREES_RESOURCE_KEY_BASIC"
	liveValues       = "derived.humanconfidence,derived.humanconfidencesource"
	liveUserAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36"
	liveTimeoutSeconds = 20
)

// liveSetting returns the value of the variable, or skips the test naming
// it.
func liveSetting(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Skipf("%s is not set, so this live test against the staging "+
			"cloud is skipped.", name)
	}
	return value
}

// redactor hides the API base, its host and the key, because a failed call's
// error names the URL, and a failed lookup or connection names the host
// alone.
func redactor(api, key string) *strings.Replacer {
	pairs := []string{api, "<" + stagingEndpointVariable + ">"}
	if base, err := url.Parse(api); err == nil && base.Hostname() != "" {
		pairs = append(pairs,
			base.Hostname(), "<"+stagingEndpointVariable+" host>")
	}
	return strings.NewReplacer(append(pairs, key, "<key>")...)
}

// livePassive posts a browser's User-Agent to the JSON path of the key in
// keyName, on the staging cloud, naming the two properties. It returns the
// answer's derived object, empty when the answer has none.
func livePassive(t *testing.T, keyName string) map[string]any {
	t.Helper()
	api := strings.TrimRight(liveSetting(t, stagingEndpointVariable), "/") + "/"
	key := liveSetting(t, keyName)
	redact := redactor(api, key)
	client := &http.Client{Timeout: liveTimeoutSeconds * time.Second}
	response, err := client.PostForm(api+key+".json",
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
	derived := livePassive(t, stagingBespokeKeyVariable)
	if derived["humanconfidencesource"] != "Passive" {
		t.Errorf("a request with no browser tests is passive, got %v", derived)
	}
	switch derived["humanconfidence"] {
	case "High", "Medium", "Low":
	default:
		t.Errorf("a passive level is one of the plain levels, got %v", derived)
	}
}

// A key outside Bespoke gets the source named, as null with a reason. The
// production cloud leaves the source out of every answer, so this fails
// against any cloud that does not serve the source.
func TestLiveKeyOutsideBespokeGetsNoSource(t *testing.T) {
	derived := livePassive(t, basicKeyVariable)
	if source, named := derived["humanconfidencesource"]; !named ||
		source != nil {
		t.Errorf("a key outside Bespoke gets the source as null, got %v",
			derived)
	}
	if _, ok := derived["humanconfidencesourcenullreason"].(string); !ok {
		t.Errorf("a null source comes with a reason, got %v", derived)
	}
}

// The redactor hides the API base, a host named on its own and the key.
func TestRedactorHidesTheAddressAndTheKey(t *testing.T) {
	api, key := "https://staging.example.com/api/v4/", "test-key"
	got := redactor(api, key).Replace(`Post "` + api + key +
		`.json": dial tcp: lookup staging.example.com: no such host`)
	want := `Post "<` + stagingEndpointVariable + `><key>.json": ` +
		`dial tcp: lookup <` + stagingEndpointVariable + ` host>: no such host`
	if got != want {
		t.Errorf("the redacted error is %q, want %q", got, want)
	}
}
