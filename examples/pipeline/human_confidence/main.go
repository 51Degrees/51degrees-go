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

// Human Confidence demo server, using the Pipeline API.
//
// Human Confidence says how likely it is that a person is looking at the
// page. The page shows it three ways, each time from the two Properties the
// Pipeline gives, HumanConfidenceSource and HumanConfidence.
//
//  1. Passive. This server processes the visitor's request headers and IP
//     address, and reads the two Properties.
//  2. Active. The page loads the 51Degrees client script. When the script
//     completes, HumanConfidenceSource is Active and HumanConfidence holds
//     a sealed value the browser can't read.
//  3. Redeemed. The page sends the sealed value back to this server as
//     humanconfidence. The server processes that request and reads the two
//     Properties again. The Pipeline redeems the sealed value with the
//     License Key, so this server never redeems anything itself, and the
//     License Key stays on it.
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
	"os"
	"strings"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/humanconfidence"
	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/derivedproperty"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
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
	http.Handler
	cfg      config
	pipeline core.Pipeline
	notice   string
}

// newExample builds the Pipeline, being a Cloud Request Engine followed by
// the Human Confidence Engine, checks that the Resource Key has
// HumanConfidence, writing to console when it doesn't, and returns the
// example. Close it to close the Pipeline.
func newExample(cfg config, client *http.Client, console io.Writer) (*example, error) {
	cre, err := requestengine.NewBuilder().
		SetEndpoint(cfg.API).
		SetResourceKey(cfg.Resource).
		SetLicenseKey(cfg.License).
		SetHTTPClient(client).
		Build()
	if err != nil {
		return nil, err
	}
	pipeline, err := core.NewPipelineBuilder().
		AddFlowElement(cre).
		AddFlowElement(humanconfidence.New(nil)).
		Build()
	if err != nil {
		cre.Close()
		return nil, err
	}
	e := &example{cfg: cfg, pipeline: pipeline}
	products, err := cre.PublicProperties()
	switch {
	case err != nil:
		fmt.Fprintf(console, "Could not check the Resource Key's properties. %v\n", err)
	case !hasHumanConfidence(products):
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
	e.Handler = mux
	return e, nil
}

// Close closes the Pipeline.
func (e *example) Close() error { return e.pipeline.Close() }

// hasHumanConfidence says whether any product holds HumanConfidence.
func hasHumanConfidence(products map[string]requestengine.ProductMeta) bool {
	for _, product := range products {
		for _, property := range product.Properties {
			if strings.EqualFold(property.Name, humanconfidence.HumanConfidence) {
				return true
			}
		}
	}
	return false
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

// passive reads the two Properties for the visitor's request alone.
func (e *example) passive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, e.read(r, ""))
}

// redeem reads the two Properties for a request that carries the sealed
// value the page sent as humanconfidence. The Pipeline redeems it.
func (e *example) redeem(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, e.read(r, r.FormValue("humanconfidence")))
}

// read processes the visitor's request and gives what the page shows for
// the two Properties. Only the request headers the cloud lists in its
// Evidence keys are passed on, so headers such as Authorization stay on this
// server. A sealed value goes in as the Evidence query.humanconfidence,
// which is what the web integration makes of the page's humanconfidence
// parameter.
func (e *example) read(r *http.Request, sealed string) reading {
	fd := e.pipeline.CreateFlowData()
	filter := e.pipeline.EvidenceKeyFilter()
	for name, values := range r.Header {
		if key := "header." + strings.ToLower(name); filter.Include(key) {
			fd.AddEvidence(key, strings.Join(values, ", "))
		}
	}
	// Behind a load balancer or CDN, use the visitor's address it passes
	// on.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		fd.AddEvidence("server.client-ip", host)
	}
	if sealed != "" {
		fd.AddEvidence(requestengine.HumanConfidenceEvidence, sealed)
	}
	// A failed request to the cloud is an error here. The two Properties
	// are read all the same, as the Pipeline gives them where it can.
	failure := ""
	if err := fd.Process(); err != nil {
		failure = err.Error()
	}
	data, err := core.Get[*derivedproperty.Data](fd)
	if err != nil {
		data = derivedproperty.NewData()
	}
	return reading{
		Source:          shownProperty(data, humanconfidence.HumanConfidenceSource, failure),
		HumanConfidence: shownProperty(data, humanconfidence.HumanConfidence, failure),
	}
}

// cell is the text of one cell of the page's table, and its tone, being
// pass, fail, part or none, which the page turns into a color.
type cell struct {
	Text string `json:"text"`
	Tone string `json:"tone"`
}

// reading is what the page shows for one read of the two Properties.
type reading struct {
	Source          cell `json:"source"`
	HumanConfidence cell `json:"humanConfidence"`
}

// shownProperty decides what the page shows for a Property: its value, or,
// where it has none, the reason the Pipeline gives, as for any other
// Property with no value. A Property the Pipeline didn't give shows the
// failure, where there was one.
func shownProperty(data *derivedproperty.Data, name, failure string) cell {
	value, err := derivedproperty.GetAs[engines.AspectPropertyValue[string]](data, name)
	if err != nil {
		if failure != "" {
			return cell{failure, "fail"}
		}
		return cell{"not returned", "none"}
	}
	text, err := value.Value()
	if err != nil {
		return cell{value.NoValueMessage(), "none"}
	}
	if name == humanconfidence.HumanConfidenceSource {
		if text == humanconfidence.Active {
			return cell{text, "pass"}
		}
		return cell{text, "none"}
	}
	return cell{text, levelTone(text)}
}

// levelTone gives the tone for a Human Confidence Value.
func levelTone(level string) string {
	switch level {
	case "High":
		return "pass"
	case "Low":
		return "fail"
	case "Medium":
		return "part"
	}
	return "none"
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(value)
}

func main() {
	cfg := loadConfig(os.Getenv)
	if cfg.Resource == "" {
		log.Fatal("Set _51DEGREES_RESOURCE_KEY to the Resource Key of the page.")
	}
	if cfg.License == "" {
		fmt.Println("No _51DEGREES_LICENSE_KEY set. The page still shows " +
			"Passive and Active, but redeeming needs a License Key, so the " +
			"third row shows Medium.")
	}
	// A short limit, so a slow cloud can't hang the demo.
	client := &http.Client{Timeout: 10 * time.Second}
	e, err := newExample(cfg, client, os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
	defer e.Close()
	port := listenPort(os.Getenv)
	fmt.Printf("Human Confidence demo on http://localhost:%s/, using %s\n", port, cfg.API)
	log.Fatal(http.ListenAndServe("localhost:"+port, e))
}
