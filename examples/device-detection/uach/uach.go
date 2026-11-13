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
 This example illustrates how to perform device detection on evidence extracted
 from a web request.

 To run this example, perform the following command:
 ```
 go run uach.go
 ```
 This will start the application at "localhost:3001". From a browser of your choice,
 enter "localhost:3001" in the URL input. Follow the instructions to see how
 the evidence are used.

 Use curl to run a simple request with a User-Agent that support User Agent
 Client Hints such as below to observe how 'Accept-CH' header is set in the
 http response:

 ```
 curl -A "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/95.0.4638.69 Safari/537.36"
 localhost:3001 -I
 ```

 Response header should include `Accept-CH` set to:
 ```
 Accept-Ch: Sec-CH-UA-Arch, Sec-CH-UA-Full-Version, Sec-CH-UA-Mobile, Sec-CH-UA-Model, Sec-CH-UA-Platform-Version, Sec-CH-UA-Platform, Sec-CH-UA
 ```

 NOTE: To see how User Agent Client Hints, run the following command:
 ```
 curl --header "Sec-CH-UA-Platform: Windows" --header "Sec-CH-UA-Platform-Version: 14.0.0" localhost:3001
 ``
 You should see the html text returned with `Platform Name` set to `Windows`, and
 `Platform Version` set to `11.0`.

 With the free Lite data file, HardwareVendor, HardwareName and DeviceType,
 which only the Enterprise data file has, are shown as needing it, with a link
 to ask for a copy.
*/

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	examples "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
)

// Evidence where all fields are in string format
type stringEvidence struct {
	Prefix string
	Key    string
	Value  string
}

// Properties required for a response page.
type Page struct {
	Keys            []stringEvidence
	HardwareVendor  string
	HardwareName    string
	DeviceType      string
	PlatformVendor  string
	PlatformName    string
	PlatformVersion string
	BrowserVendor   string
	BrowserName     string
	BrowserVersion  string
	// ContactUs is set when the data file lacks a property only the
	// Enterprise data file has.
	ContactUs string
}

// contactUs is where to ask for a copy of the Enterprise data file, which has
// the properties the free data file leaves out.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=examples-device-detection-uach-uach.go&utm_term=enterprise-data-file"

var manager *dd.ResourceManager
var config *dd.ConfigHash

// Template for the response HTML page.

var templ = `<!DOCTYPE HTML>
 <html>
   <head>
	 <meta charset="utf-8">
	 <title>Web Integration Example</title>
   </head>
   <body>
 	 <h2>User Agent Client Hints Example</h2>
	 <p>
	 By default, the user-agent, sec-ch-ua and sec-ch-ua-mobile HTTP headers
	 are sent.
	 <br />
	 This means that on the first request, the server can determine the
	 browser from sec-ch-ua while other details must be derived from the
	 user-agent.
	 <br />
	 If the server determines that the browser supports client hints, then
	 it may request additional client hints headers by setting the
	 Accept-CH header in the response.
	 <br />
	 Select the <strong>Make second request</strong> button below,
	 to use send another request to the server. This time, any
	 additional client hints headers that have been requested
	 will be included.
	 </p>
 
	 <button type="button" onclick="redirect()">Make second request</button>

	 <script>
 
		 // This script will run when button will be clicked and device detection request will again 
		 // be sent to the server with all additional client hints that was requested in the previous
		 // response by the server.
		 // Following sequence will be followed.
		 // 1. User will send the first request to the web server for detection.
		 // 2. Web Server will return the properties in response based on the headers sent in the request. Along 
		 // with the properties, it will also send a new header field Accept-CH in response indicating the additional
		 // evidence it needs. It builds the new response header using SetHeader[Component name]Accept-CH properties 
		 // where Component Name is the name of the component for which properties are required.
		 // 3. When "Make second request" button will be clicked, device detection request will again 
		 // be sent to the server with all additional client hints that was requested in the previous
		 // response by the server.
		 // 4. Web Server will return the properties based on the new User Agent Client Hint headers 
		 // being used as evidence.
 
		 function redirect() {
			 sessionStorage.reloadAfterPageLoad = true;
			 window.location.reload(true);
			 }
 
		 window.onload = function () { 
			 if ( sessionStorage.reloadAfterPageLoad ) {
			 document.getElementById('description').innerHTML = "<p>The information shown below is determined using <strong>User Agent Client Hints</strong> that was sent in the request to obtain additional evidence. If no additional information appears then it may indicate an external problem such as <strong>User Agent Client Hints</strong> being disabled in your browser.</p>";
			 sessionStorage.reloadAfterPageLoad = false;
			 }
			 else{
			 document.getElementById('description').innerHTML = "<p>The following values are determined by sever-side device detection on the first request.</p>";
			 }
		 }

	 </script>

	   <div id="evidence">
	      <strong></br>Evidence values used: </strong>
	      <table>
   	         <tr>
   	            <th>Key</th>
   	            <th>Value</th>
   	         </tr>
			 {{range .Keys}}
			  	<tr>
				   <td>{{.Prefix}}{{.Key}}</td>
				   <td>{{.Value}}</td>
				</tr>
			 {{end}}
	      </table>
	   </div>
	   <div id=description></div>
	   <div id="content">
	      <strong>Detection results:</strong></br></br>
	      <b>HardwareVendor:</b> {{.HardwareVendor}}<br />
	      <b>HardwareName:</b> {{.HardwareName}}<br />
	      <b>DeviceType:</b> {{.DeviceType}}<br />
	      <b>Platform Vendor:</b> {{.PlatformVendor}}<br />
	      <b>Platform Name:</b> {{.PlatformName}}<br />
	      <b>Platform Version:</b> {{.PlatformVersion}}<br />
	      <b>Browser Vendor:</b> {{.BrowserVendor}}<br />
	      <b>Browser Name:</b> {{.BrowserName}}<br />
	      <b>Browser Version:</b> {{.BrowserVersion}}<br />
	      {{if .ContactUs}}<p>Some of these properties need the Enterprise data file. <a href="{{.ContactUs}}">Contact us</a> for a copy.</p>{{end}}
	   </div>
   </body>
</html>`

// Prefixes in literal format
const queryPrefix = "query."
const headerPrefix = "header."

func extractEvidenceStrings(r *http.Request, keys []dd.EvidenceKey) []stringEvidence {
	evidence := make([]stringEvidence, 0)
	for _, e := range keys {
		lowerKey := strings.ToLower(e.Key)
		switch e.Prefix {
		case dd.HttpEvidenceQuery:
			// Get evidence from query parameter
			queryVal := r.URL.Query().Get(lowerKey)
			if queryVal != "" {
				evidence = append(
					evidence, stringEvidence{queryPrefix, e.Key, queryVal})
			}
		default:
			// Get evidence from headers
			headerVal := r.Header.Get(lowerKey)
			if headerVal != "" {
				evidence = append(
					evidence, stringEvidence{headerPrefix, e.Key, headerVal})
			}
		}
	}
	return evidence
}

// extractEvidence looks into a list of required evidence keys and extract
// them from a http request.
func extractEvidence(strEvidence []stringEvidence) *dd.Evidence {
	evidence := dd.NewEvidenceHash(uint32(len(strEvidence)))
	for _, e := range strEvidence {
		prefix := dd.HttpHeaderString
		if e.Prefix == queryPrefix {
			prefix = dd.HttpEvidenceQuery
		}
		evidence.Add(prefix, e.Key, e.Value)
	}
	return evidence
}

// function match performs a match on an input User-Agent string and determine
// if the device is a mobile device.
func match(
	results *dd.ResultsHash,
	evidence *dd.Evidence) {
	err := results.MatchEvidence(evidence)
	if err != nil {
		log.Fatal("ERROR: Failed to perform detection.")
	}
}

// function getValue return a value results for a property
func getValue(
	results *dd.ResultsHash,
	propertyName string) string {
	// Get the values in string
	value, err := results.ValuesString(
		propertyName,
		",")
	if err != nil {
		log.Fatalln("ERROR: Failed to get results values string.")
	}

	hasValues, err := results.HasValues(propertyName)
	if err != nil {
		log.Fatalf(
			"ERROR: Failed to check if a matched value exists for property "+
				"%s.\n", propertyName)
	}

	if !hasValues {
		log.Printf("Property %s does not have a matched value.\n", propertyName)
		return "Unknown"
	}

	return value
}

// Handler for web request
func handler(w http.ResponseWriter, r *http.Request) {
	filteredEvidence := extractEvidenceStrings(r, manager.HttpHeaderKeys)
	// Extract evidence
	evidence := extractEvidence(filteredEvidence)
	// Make sure evidence is freed at the end
	defer evidence.Free()

	// Create results
	results := dd.NewResultsHash(manager, uint32(evidence.Count()), 0)

	// Make sure results object is freed after function execution.
	defer results.Free()

	// Perform detection on mobile User-Agent
	match(results, evidence)

	// NOTE: Add response headers to request User-Agent Client Hints
	// from client. This is IMPORTANT so that User-Agent Client Hints
	// required by Device Detection engine are returned in the subsequence
	// requests.
	results.SetResponseHeaders(w, manager)

	// The properties only the Enterprise data file has, each with its value
	// or as needing that file.
	enterprise, missing := examples.Enterprise(results)
	shown := make(map[string]string)
	for _, property := range enterprise {
		shown[property.Name] = property.Value
	}
	hardwareVendor := shown["HardwareVendor"]
	hardwareName := shown["HardwareName"]
	deviceType := shown["DeviceType"]
	platformVendor := getValue(results, "PlatformVendor")
	platformName := getValue(results, "PlatformName")
	platformVersion := getValue(results, "PlatformVersion")
	browserVendor := getValue(results, "BrowserVendor")
	browserName := getValue(results, "BrowserName")
	browserVersion := getValue(results, "BrowserVersion")
	p := &Page{
		filteredEvidence,
		hardwareVendor,
		hardwareName,
		deviceType,
		platformVendor,
		platformName,
		platformVersion,
		browserVendor,
		browserName,
		browserVersion,
		"",
	}
	if missing {
		p.ContactUs = contactUs
	}

	// Construct the template
	t := template.Must(template.New("dd").Parse(templ))
	// Return the constructed template in a response
	t.Execute(w, p)
}

func main() {
	// Initialise manager
	manager = dd.NewResourceManager()
	config = dd.NewConfigHash(dd.Balanced)
	config.SetUseUpperPrefixHeaders(false)
	// The data file is found by searching up from here, and when it is
	// missing it is downloaded from the data repository, which links to it.
	filePath := examples.GetFilePathByName([]string{"51Degrees-LiteV4.1.hash"})
	// Init manager
	err := dd.InitManagerFromFile(
		manager,
		*config,
		"",
		filePath)
	if err != nil {
		log.Fatalln("ERROR: Failed to initialize resource manager.")
	}

	// Make sure manager object will be freed after the function execution
	defer manager.Free()

	http.HandleFunc("/", handler)
	const port = 3001
	fmt.Printf("Server listening on port: %d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil))
}
