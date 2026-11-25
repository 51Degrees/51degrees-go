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

// Human Confidence demo server, using only the Go standard library.
//
// Human Confidence says how likely it is that a person is looking at the
// page. The page shows it three ways.
//
//  1. Passive. This server asks the cloud, passing on the visitor's request
//     headers and IP address, and reads HumanConfidenceSource and
//     HumanConfidence from the answer.
//  2. Active. The page loads the 51Degrees client script. When the script
//     completes, HumanConfidenceSource is Active and HumanConfidence holds
//     a sealed value the browser can't read.
//  3. Redeem. The page hands the sealed value to this server, which sends
//     it as sealedResult with the License Key to hc/redeem and returns the
//     cloud's answer as it is. The License Key stays on this server.
//
// Environment variables
//
//	_51DEGREES_RESOURCE_KEY    the Resource Key, required
//	_51DEGREES_LICENSE_KEY     a License Key of the same account, needed
//	                           to redeem
//	_51DEGREES_CLOUD_ENDPOINT  the API base including /api/v4/, defaults
//	                           to https://cloud.51degrees.com/api/v4/
//	PORT                       the port to listen on, defaults to 5151
//
// Run from examples/pipeline with
//
//	go run ./human_confidence
//
// then open http://localhost:5151/ in a browser.
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// contactUs is where to ask for a Resource Key with HumanConfidence.
const contactUs = "https://51degrees.com/contact-us?utm_source=code&utm_medium=example&utm_campaign=51degrees-go&utm_content=human-confidence&utm_term=contact-us"

// defaultEndpoint is the cloud API base used when none is set.
const defaultEndpoint = "https://cloud.51degrees.com/api/v4/"

//go:embed page.html
var page string

// config holds the settings the example reads from the environment.
type config struct {
	API      string
	Resource string
	License  string
}

// loadConfig reads the settings through getenv, which is os.Getenv outside
// the tests.
func loadConfig(getenv func(string) string) config {
	api := strings.TrimSpace(getenv("_51DEGREES_CLOUD_ENDPOINT"))
	if api == "" {
		api = defaultEndpoint
	}
	return config{
		API:      strings.TrimRight(api, "/") + "/",
		Resource: strings.TrimSpace(getenv("_51DEGREES_RESOURCE_KEY")),
		License:  strings.TrimSpace(getenv("_51DEGREES_LICENSE_KEY")),
	}
}

// listenPort gives the port to listen on, from PORT or 5151.
func listenPort(getenv func(string) string) string {
	if port := strings.TrimSpace(getenv("PORT")); port != "" {
		return port
	}
	return "5151"
}

// example serves the page and the two calls the page makes.
type example struct {
	cfg    config
	client *http.Client
	notice string

	mu      sync.Mutex
	headers map[string]bool
}

// newExample checks that the Resource Key has HumanConfidence, writing to
// console when it doesn't, and returns the handler.
func newExample(cfg config, client *http.Client, console io.Writer) http.Handler {
	e := &example{cfg: cfg, client: client}
	has, err := e.hasHumanConfidence()
	switch {
	case err != nil:
		fmt.Fprintf(console, "Could not check the Resource Key's properties. %v\n", err)
	case !has:
		fmt.Fprintf(console, "The Resource Key does not include HumanConfidence. "+
			"Contact us for a key that does, at %s\n", contactUs)
		e.notice = `<p class="notice">The Resource Key does not include ` +
			`HumanConfidence. <a href="` + contactUs +
			`">Contact us</a> for a key that does.</p>`
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", e.servePage)
	mux.HandleFunc("GET /passive", e.passive)
	mux.HandleFunc("POST /redeem", e.redeem)
	return mux
}

// hasHumanConfidence asks the cloud which properties the Resource Key has.
func (e *example) hasHumanConfidence() (bool, error) {
	response, err := e.client.Get(e.cfg.API + "accessibleproperties?resource=" +
		url.QueryEscape(e.cfg.Resource))
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	var answer any
	if err := json.NewDecoder(response.Body).Decode(&answer); err != nil {
		return false, err
	}
	return hasName(answer, "HumanConfidence"), nil
}

// hasName says whether any object in value has a Name equal to name.
func hasName(value any, name string) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if s, ok := item.(string); ok && strings.EqualFold(key, "name") &&
				strings.EqualFold(s, name) {
				return true
			}
			if hasName(item, name) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if hasName(item, name) {
				return true
			}
		}
	}
	return false
}

// cloudHeaders gives the request headers in the cloud's evidence keys, in
// lower case. Only these are passed on, so headers such as Authorization
// stay on this server. A good answer is kept. After any other answer the
// next call asks the cloud again.
func (e *example) cloudHeaders() (map[string]bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.headers != nil {
		return e.headers, nil
	}
	response, err := e.client.Get(e.cfg.API + "evidencekeys")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}
	var keys []string
	if err := json.NewDecoder(response.Body).Decode(&keys); err != nil {
		return nil, err
	}
	headers := make(map[string]bool)
	for _, key := range keys {
		if strings.HasPrefix(strings.ToLower(key), "header.") {
			headers[strings.ToLower(key[len("header."):])] = true
		}
	}
	e.headers = headers
	return headers, nil
}

// servePage serves the page with the Resource Key and the API base. The
// page must never come from a cache, as each load runs the tests.
func (e *example) servePage(w http.ResponseWriter, r *http.Request) {
	body := strings.NewReplacer(
		"__RESOURCE__", e.cfg.Resource,
		"__API__", e.cfg.API,
		"__NOTICE__", e.notice).Replace(page)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, body)
}

// passive passes the visitor's headers and IP address to the cloud and
// returns the source and level from the answer.
func (e *example) passive(w http.ResponseWriter, r *http.Request) {
	names, err := e.cloudHeaders()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"error": err.Error()})
		return
	}
	evidence := url.Values{}
	for name, values := range r.Header {
		if lower := strings.ToLower(name); names[lower] {
			evidence.Set(lower, strings.Join(values, ", "))
		}
	}
	// Behind a load balancer or CDN, use the visitor's address it passes
	// on. Set last, so no header can replace it.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		evidence.Set("client-ip", host)
	}
	response, err := e.client.PostForm(e.cfg.API+e.cfg.Resource+".json", evidence)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"error": err.Error()})
		return
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusOK, map[string]string{"error": fmt.Sprintf(
			"HTTP %d: %s", response.StatusCode, trim(string(body)))})
		return
	}
	var answer struct {
		Derived map[string]any `json:"derived"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source":          stringOrNil(answer.Derived["humanconfidencesource"]),
		"humanConfidence": stringOrNil(answer.Derived["humanconfidence"]),
	})
}

// redeem sends the sealed value with the License Key to the cloud and
// returns its answer unchanged.
//
// The sealed value is sent as sealedResult, and again as value, its obsolete
// name, which a cloud that predates sealedResult reads. A current cloud reads
// sealedResult, so the redeem works against both.
func (e *example) redeem(w http.ResponseWriter, r *http.Request) {
	// The page sends sealedResult. A page that predates it sends value.
	sealedResult := r.FormValue("sealedResult")
	if sealedResult == "" {
		sealedResult = r.FormValue("value")
	}
	fields := url.Values{
		"sealedResult": {sealedResult},
		"value":        {sealedResult},
	}
	if e.cfg.License != "" {
		fields.Set("license", e.cfg.License)
	}
	response, err := e.client.PostForm(e.cfg.API+"hc/redeem", fields)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		writeJSON(w, http.StatusOK, map[string]string{"error": fmt.Sprintf(
			"The service at %s doesn't redeem Human Confidence.", e.cfg.API)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	io.Copy(w, response.Body)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func stringOrNil(value any) any {
	if s, ok := value.(string); ok {
		return s
	}
	return nil
}

func trim(text string) string {
	if len(text) > 200 {
		return text[:200]
	}
	return text
}

func main() {
	cfg := loadConfig(os.Getenv)
	if cfg.Resource == "" {
		log.Fatal("Set _51DEGREES_RESOURCE_KEY to the Resource Key of the page.")
	}
	if cfg.License == "" {
		fmt.Println("No _51DEGREES_LICENSE_KEY set. The page still shows " +
			"Passive and Active, but redeeming needs a License Key.")
	}
	// A short limit, so a slow cloud can't hang the demo.
	client := &http.Client{Timeout: 10 * time.Second}
	handler := newExample(cfg, client, os.Stdout)
	port := listenPort(os.Getenv)
	fmt.Printf("Human Confidence demo on http://localhost:%s/, using %s\n", port, cfg.API)
	log.Fatal(http.ListenAndServe("localhost:"+port, handler))
}
