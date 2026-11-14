/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, Davidson House,
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

/*
This example illustrates how to perform simple device detections on given
User-Agent strings.
*/

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	examples "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
)

func TestMain(m *testing.M) {
	// Initialise manager
	manager = dd.NewResourceManager()
	config = dd.NewConfigHash(dd.Balanced)
	// Found as the example finds it, so a fresh clone downloads it.
	filePath := examples.GetFilePathByName([]string{examples.LiteDataFile})

	err := dd.InitManagerFromFile(
		manager,
		*config,
		"",
		filePath)
	if err != nil {
		log.Fatalln("ERROR: Failed to initialize resource manager.")
	}

	// Execute the test
	code := m.Run()

	// Make sure manager object will be freed after the function execution
	manager.Free()
	os.Exit(code)
}

// Test if the web integration handler handles the request
// correctly.
func TestHandler(t *testing.T) {
	// Expected response body, with the Lite data file, which has none of the
	// properties only the Enterprise data file has.
	p := &Page{
		BrowserName:       "Mobile Safari",
		ScreenPixelsWidth: "640",
		ContactUs:         contactUs,
	}
	for _, property := range examples.EnterpriseProperties {
		p.Enterprise = append(p.Enterprise,
			examples.Property{Name: property, Value: examples.NeedsEnterprise})
	}

	// Construct the template
	var buf bytes.Buffer
	expectedTempl := template.Must(template.New("dd").Parse(templ))
	if err := expectedTempl.Execute(&buf, p); err != nil {
		log.Fatalln("ERROR: Failed to construct expected template.")
	}

	// Create http request for testing
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		log.Fatalln("ERROR: Failed to create new http request.")
	}
	r.Header.Add(
		"User-Agent",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) "+
			"AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 "+
			"Safari/9537.53")

	// Create a ResponseRecorder to capture the response
	rr := httptest.NewRecorder()
	h := http.HandlerFunc(handler)

	// Serve the http request
	h.ServeHTTP(rr, r)

	// Check if status code is as expected
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("ERROR: Expected status code %v but got %v",
			http.StatusOK, status)
	}

	// Check the response body is what we expect.
	exp := buf.String()
	act := rr.Body.String()
	if rr.Body.String() != exp {
		t.Errorf("ERROR: Expected:\n"+
			"\"\n"+
			"%s\n"+
			"\"\n"+
			"Got:\n"+
			"\"\n"+
			"%s\n"+
			"\"\n", exp, act)
	}
}

// TestHandlerShowsEnterprise checks the page shows the properties only the
// Enterprise data file has, with the link to ask for a copy when the data file
// lacks them.
func TestHandlerShowsEnterprise(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("User-Agent",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 7_1 like Mac OS X) "+
			"AppleWebKit/537.51.2 (KHTML, like Gecko) Version/7.0 Mobile/11D167 "+
			"Safari/9537.53")
	rr := httptest.NewRecorder()
	http.HandlerFunc(handler).ServeHTTP(rr, r)
	if err := examples.CheckEnterprise(rr.Body.String()); err != nil {
		t.Error(err)
	}
}
