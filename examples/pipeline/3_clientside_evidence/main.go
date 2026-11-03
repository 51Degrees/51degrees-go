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

// Example 3: a Flow Element whose input is gathered by client-side JavaScript.
//
// When no date of birth is available, the element returns a JavaScript Property
// containing a snippet that the client executes to collect it (here, it reads a
// browser cookie). The generated client-side script (from the JavaScript
// Builder) posts the value back to the JSON endpoint, and that request can then
// populate the star sign.
//
// This example stands up the real web.Middleware over an httptest server and
// exercises the actual round-trip: it fetches /51Degrees.core.js (printing the
// generated script and its callback URL), then POSTs to /51dpipeline/json as the
// browser would once it has gathered the date of birth.
//
// Run with: go run ./examples/3_clientside_evidence
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/required-examples.md
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
	"github.com/51Degrees/51degrees-go/examples/pipeline/starsign"
	"github.com/51Degrees/51degrees-go/pipeline/v4/web"
)

const (
	evidenceDateOfBirth = "query.date-of-birth"
	dobJavaScript       = `var d = (document.cookie.match(/(?:^|; )51D_date-of-birth=([^;]*)/)||[])[1];` +
		`if (d) { document.cookie = "51D_date-of-birth=" + d; }`
)

// ClientSideStarSignElement determines a star sign, or asks the client for the
// date of birth via a JavaScript Property when it is not yet available.
type ClientSideStarSignElement struct {
	*core.BaseElement
}

// NewClientSideStarSignElement creates the element.
func NewClientSideStarSignElement() *ClientSideStarSignElement {
	e := &ClientSideStarSignElement{BaseElement: core.NewBaseElement(
		"starsign", core.NewWhitelistFilter(evidenceDateOfBirth), nil)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: "starsign", Element: e, Category: "Astrology", Available: true,
			EvidenceProperties: []string{"starsign.dobjavascript"}},
		{Name: "dobjavascript", Element: e, Category: "Astrology", Available: true, DelayExecution: false},
	})
	return e
}

// Process either returns the star sign or a JavaScript snippet to gather the
// date of birth from the client.
func (e *ClientSideStarSignElement) Process(fd core.FlowData) error {
	data := fd.GetOrAdd(e.DataKey(), func(core.FlowData) core.ElementData { return core.NewBaseElementData() })
	raw, ok := fd.Evidence().Get(evidenceDateOfBirth)
	if !ok {
		// Ask the client to gather the value; star sign has no value yet. Using
		// an AspectPropertyValue lets the JSON Builder emit the nullreason for us.
		data.Set("dobjavascript", core.JavaScript(dobJavaScript))
		data.Set("starsign", engines.NoValue[string]("Awaiting date of birth from client-side JavaScript."))
		return nil
	}
	dob, err := starsign.ParseDate(fmt.Sprintf("%v", raw))
	if err != nil {
		return err
	}
	data.Set("starsign", starsign.SignForDate(dob.Month(), dob.Day()))
	return nil
}

func main() {
	pipeline, err := web.AssemblePipeline(web.PipelineConfig{}, NewClientSideStarSignElement())
	if err != nil {
		log.Fatal(err)
	}
	defer pipeline.Close()

	// Mount the web integration middleware on a real HTTP server. The host
	// application handler can read results from the Flow Data via web.FromContext.
	mw := web.NewMiddleware(pipeline)
	app := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mw.Handler(app))
	defer srv.Close()

	// 1) The browser includes /51Degrees.core.js. The served script contains the
	//    JavaScript snippet to gather the date of birth and the callback URL.
	fmt.Println("=== GET /51Degrees.core.js (served to the browser) ===")
	js := httpGet(srv.URL + web.DefaultJavaScriptURL)
	fmt.Println(firstLines(js, 6))
	fmt.Printf("... callback endpoint present: %v\n", strings.Contains(js, web.DefaultJSONURL))

	// 2) Once the client has the date of birth it posts back to the JSON endpoint.
	//    That request now carries the evidence, so the star sign is resolved.
	fmt.Println("\n=== POST /51dpipeline/json (client callback with the gathered date) ===")
	json := httpPostForm(srv.URL+web.DefaultJSONURL, url.Values{"date-of-birth": {"1992-04-15"}})
	fmt.Println(json)
}

func httpGet(u string) string {
	resp, err := http.Get(u)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

func httpPostForm(u string, form url.Values) string {
	resp, err := http.PostForm(u, form)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = append(lines[:n], "...")
	}
	return strings.Join(lines, "\n")
}
