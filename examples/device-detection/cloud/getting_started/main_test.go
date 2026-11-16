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

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// standInCloud answers as the cloud service does for a Resource Key that
// includes IsMobile, PlatformName and BrowserName.
func standInCloud() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"device":{"DataTier":"CloudV4Free","Properties":[` +
			`{"Name":"IsMobile","Type":"Boolean","Category":"Device"},` +
			`{"Name":"PlatformName","Type":"String","Category":"Software"},` +
			`{"Name":"BrowserName","Type":"String","Category":"Browser"}]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["header.user-agent"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"device":{"ismobile":true,"platformname":"iOS","browsername":"Mobile Safari"}}`))
	})
	return httptest.NewServer(mux)
}

// optionsAt writes the example's options file with the cloud end point set
// to url, and gives its path.
func optionsAt(t *testing.T, url string) string {
	t.Helper()
	body, err := os.ReadFile(optionsFile)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(body), `"ResourceKey"`,
		`"EndPoint": "`+url+`/", "ResourceKey"`, 1)
	path := filepath.Join(t.TempDir(), "options.json")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The Device Detection console example builds its Pipeline from an options
// file and gives the Properties of the device it detects in the cloud.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/required-examples.md?plain=1#L237-L243
func TestCloudExampleIsBuiltFromAnOptionsFile(t *testing.T) {
	srv := standInCloud()
	defer srv.Close()
	var out strings.Builder
	if err := run(optionsAt(t, srv.URL), "test-resource-key", &out); err != nil {
		t.Fatalf("the example gave %v", err)
	}
	for _, want := range []string{"IsMobile: true", "PlatformName: iOS", "BrowserName: Mobile Safari"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the output lacks %q:\n%s", want, out.String())
		}
	}
}

// With no Resource Key, the example names the variable to set, gives the
// link to where a Resource Key is created, ends with Find out more and gives
// no error, so it runs with no settings.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/required-examples.md?plain=1#L140-L150
func TestCloudExampleWithNoResourceKey(t *testing.T) {
	var out strings.Builder
	if err := run(optionsFile, "", &out); err != nil {
		t.Fatalf("the example gave %v", err)
	}
	if !strings.Contains(out.String(), resourceKeyVariable) {
		t.Errorf("the output does not name %s:\n%s", resourceKeyVariable, out.String())
	}
	if !strings.Contains(out.String(), configuratorURL) {
		t.Errorf("the output does not link to %s:\n%s", configuratorURL, out.String())
	}
	checkFindOutMore(t, out.String())
}

// checkFindOutMore fails the test unless output ends with a Find out more
// section that names the repository and links to 51degrees.com with the
// example's campaign tags.
func checkFindOutMore(t *testing.T, output string) {
	t.Helper()
	at := strings.LastIndex(output, "Find out more")
	if at < 0 {
		t.Fatalf("the output has no Find out more section:\n%s", output)
	}
	section := output[at:]
	for _, want := range []string{
		"https://github.com/51Degrees/51degrees-go",
		"https://51degrees.com/",
		"utm_source=code&utm_medium=example&utm_campaign=51degrees-go",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("the Find out more section lacks %q:\n%s", want, section)
		}
	}
}
