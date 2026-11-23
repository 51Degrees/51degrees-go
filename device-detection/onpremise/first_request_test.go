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

package onpremise

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
)

// configuredProductRules is the specification text these tests follow.
const configuredProductRules = "https://github.com/51Degrees/specifications/" +
	"blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/" +
	"features/data-updates.md#configured-product"

// firstRequestQuery is the query of the first request of an update that an
// engine with the default data update URL, the made-up license key and the
// product given makes.
func firstRequestQuery(t *testing.T, product string) url.Values {
	t.Helper()
	engine := &Engine{
		FileUpdater: common_go.NewFileUpdater(defaultDataFileUrl),
		licenseKey:  productTestKey,
		product:     product,
	}
	if err := engine.validateAndAppendUrlParams(); err != nil {
		t.Fatalf("the data update URL could not be completed: %v", err)
	}
	parsed, err := url.Parse(engine.GetDataFileUrl())
	if err != nil {
		t.Fatalf("the data update URL could not be parsed: %v", err)
	}
	return parsed.Query()
}

// The first request of an update names no product unless the product option
// is set, so the Distributor answers with the product the license key holds.
func TestFirstRequestNamesNoProductByDefault(t *testing.T) {
	query := firstRequestQuery(t, "")

	if query.Has("Product") {
		t.Errorf("the first request has the query %q, want no Product parameter",
			query.Encode())
	}
	if got := query.Get("LicenseKeys"); got != productTestKey {
		t.Errorf("LicenseKeys = %q, want %q", got, productTestKey)
	}
}

// The first request of an update names the product the product option gives.
func TestFirstRequestNamesTheProductSet(t *testing.T) {
	query := firstRequestQuery(t, "V4TAC")

	if got := query["Product"]; len(got) != 1 || got[0] != "V4TAC" {
		t.Errorf("Product = %q, want V4TAC once", got)
	}
	if got := query.Get("LicenseKeys"); got != productTestKey {
		t.Errorf("LicenseKeys = %q, want %q", got, productTestKey)
	}
}

// requestLog keeps the query of each request a test server is sent.
type requestLog struct {
	mu      sync.Mutex
	queries []string
}

func (l *requestLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries = append(l.queries, r.URL.RawQuery)
}

// raw is the query of each request as it was sent.
func (l *requestLog) raw() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.queries...)
}

// products are the Product values of each request, in order.
func (l *requestLog) products() [][]string {
	var products [][]string
	for _, query := range l.raw() {
		values, _ := url.ParseQuery(query)
		products = append(products, values["Product"])
	}
	return products
}

// onlyProduct reports whether products is one request that asked for product
// once.
func onlyProduct(products [][]string, product string) bool {
	return len(products) == 1 && len(products[0]) == 1 && products[0][0] == product
}

// locationServer answers every request with 404 and counts them. It stands
// for the address a 300 response names in its Location header.
func locationServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			count.Add(1)
			http.NotFound(w, r)
		}))
	t.Cleanup(server.Close)
	return server, &count
}

// productSetEngine is an engine with the default data update URL, completed
// for the made-up license key and the product given, that sends the query it
// made to server, which stands in for the Distributor. engineWithDataFile
// gives it the rest.
func productSetEngine(t *testing.T, server *httptest.Server, product string,
	loaded string) (*Engine, *reloadTestLog, *atomic.Int32) {
	t.Helper()
	engine := &Engine{
		FileUpdater: common_go.NewFileUpdater(defaultDataFileUrl),
		licenseKey:  productTestKey,
		product:     product,
	}
	if err := engine.validateAndAppendUrlParams(); err != nil {
		t.Fatalf("the data update URL could not be completed: %v", err)
	}
	parsed, err := url.Parse(engine.GetDataFileUrl())
	if err != nil {
		t.Fatalf("the data update URL could not be parsed: %v", err)
	}
	engine.SetDataFileUrl(server.URL + "/api/v2/download?" + parsed.RawQuery)
	return engineWithDataFile(t, engine, loaded)
}

// engineWithDataFile gives engine a data file holding "current" and the log
// returned, with loaded as the product of its data file, counting each time
// that product is looked up.
func engineWithDataFile(t *testing.T, engine *Engine,
	loaded string) (*Engine, *reloadTestLog, *atomic.Int32) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.file")
	if err := os.WriteFile(path, []byte("current"), 0644); err != nil {
		t.Fatalf("write the data file: %v", err)
	}
	engine.SetDataFile(path)
	engine.SetEngineName(engineTypeName)
	var lookups atomic.Int32
	engine.SetProductFunc(func() string {
		lookups.Add(1)
		return loaded
	})
	log := &reloadTestLog{}
	engine.SetCustomLogger(log)
	return engine, log, &lookups
}

// A product the product option sets wins. The first request asks for it, and
// a 300 (Multiple Choices) response to that request is not followed, so the
// product of the loaded data file is never looked up or asked for in its
// place, and the address in the Location header is not requested. The update
// ends with the error of any other response.
func TestProductSetIsNotReplacedOnA300(t *testing.T) {
	location, followed := locationServer(t)
	offered := location.URL + "/api/v2/download?Product="
	requests := &requestLog{}
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requests.add(r)
			w.Header().Set("Location", offered+"V4Enterprise")
			w.WriteHeader(http.StatusMultipleChoices)
			w.Write([]byte(`{"AvailableUris":["` + offered + `V4Enterprise","` +
				offered + `V4TAC"]}`))
		}))
	defer server.Close()
	engine, log, lookups := productSetEngine(t, server, "V4TAC", "V4Enterprise")

	if _, err := engine.PullOnStart(); err != nil {
		t.Fatalf("the update on start-up failed: %v", err)
	}

	if got := requests.products(); !onlyProduct(got, "V4TAC") {
		t.Errorf("the requests asked for the products %q, want V4TAC once (%s)",
			got, configuredProductRules)
	}
	if n := followed.Load(); n != 0 {
		t.Errorf("%d requests went to the address of the Location header, want none", n)
	}
	if n := lookups.Load(); n != 0 {
		t.Errorf("the product of the data file was looked up %d times, want none", n)
	}
	if !log.has("HTTP status code '300' from data update service at '") {
		t.Error("no message gives the error of any other response")
	}
	if log.has("Requesting product") {
		t.Error("a message says a product of the response is requested")
	}
	if log.has(productTestKey) {
		t.Error("a message holds the license key in full")
	}
}

// A 403 (Forbidden) answer to a request for the product the option sets is
// the access denied error, which is what the Distributor answers when the
// license key does not hold that product.
func TestProductSetAnd403IsAccessDenied(t *testing.T) {
	requests := &requestLog{}
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requests.add(r)
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"Error":"The license key provided does not ` +
				`contain the requested product."}`))
		}))
	defer server.Close()
	engine, log, _ := productSetEngine(t, server, "V4TAC", "V4Enterprise")
	var mu sync.Mutex
	var statuses []common_go.UpdateStatus
	engine.OnUpdateComplete(func(u common_go.UpdateComplete) {
		mu.Lock()
		defer mu.Unlock()
		statuses = append(statuses, u.Status)
	})

	if _, err := engine.PullOnStart(); err != nil {
		t.Fatalf("the update on start-up failed: %v", err)
	}

	if got := requests.products(); !onlyProduct(got, "V4TAC") {
		t.Errorf("the requests asked for the products %q, want V4TAC once", got)
	}
	if !log.has("Access denied to data update service at '",
		"' for engine '"+engineTypeName+"'") {
		t.Error("no message says access to the data update service was denied")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(statuses) != 1 || statuses[0] != common_go.UpdateForbidden {
		t.Errorf("the update reported %v, want %v once (%s)", statuses,
			common_go.UpdateForbidden, configuredProductRules)
	}
	if log.has(productTestKey) {
		t.Error("a message holds the license key in full")
	}
}

// The product option is sent with a caller's own data update URL too, given
// before or after the URL. An empty Product in the URL is replaced, and a URL
// that already names a product, whatever the case of the parameter's name, is
// requested as it is.
func TestProductSetWithACallersURLIsSent(t *testing.T) {
	requests := &requestLog{}
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requests.add(r)
			w.WriteHeader(http.StatusNotModified)
		}))
	defer server.Close()
	address := server.URL + "/api/v2/download?"
	query := "LicenseKeys=" + productTestKey + "&Type=HashV41"
	for name, c := range map[string]struct {
		options []EngineOptions
		// product is the Product the request asks for once, or empty when
		// the request is the caller's URL as it is.
		product string
	}{
		"the URL first": {[]EngineOptions{
			WithDataUpdateUrl(address + query), WithProduct("V4TAC")}, "V4TAC"},
		"the product first": {[]EngineOptions{
			WithProduct("V4TAC"), WithDataUpdateUrl(address + query)}, "V4TAC"},
		"an empty product in the URL": {[]EngineOptions{
			WithDataUpdateUrl(address + query + "&Product="),
			WithProduct("V4TAC")}, "V4TAC"},
		"a product in the URL": {[]EngineOptions{
			WithDataUpdateUrl(address + query + "&product=V4Enterprise"),
			WithProduct("V4TAC")}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			*requests = requestLog{}
			engine := &Engine{
				FileUpdater: common_go.NewFileUpdater(defaultDataFileUrl),
			}
			for _, option := range c.options {
				if err := option(engine); err != nil {
					t.Fatalf("the option was refused: %v", err)
				}
			}
			if err := engine.validateAndAppendUrlParams(); err != nil {
				t.Fatalf("the data update URL could not be completed: %v", err)
			}
			engine, log, lookups := engineWithDataFile(t, engine, "V4Enterprise")

			if _, err := engine.PullOnStart(); err != nil {
				t.Fatalf("the update on start-up failed: %v", err)
			}

			if c.product != "" {
				if got := requests.products(); !onlyProduct(got, c.product) {
					t.Errorf("the requests asked for the products %q, want %s "+
						"once (%s)", got, c.product, configuredProductRules)
				}
			} else {
				want := query + "&product=V4Enterprise"
				if got := requests.raw(); len(got) != 1 || got[0] != want {
					t.Errorf("the requests had the queries %q, want %q once (%s)",
						got, want, configuredProductRules)
				}
			}
			if n := lookups.Load(); n != 0 {
				t.Errorf("the product of the data file was looked up %d times, "+
					"want none", n)
			}
			if log.has(productTestKey) {
				t.Error("a message holds the license key in full")
			}
		})
	}
}
