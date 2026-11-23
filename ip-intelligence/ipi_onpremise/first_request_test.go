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

package ipi_onpremise

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
)

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

// A product the product option sets wins. The first request asks for it, and
// a 300 (Multiple Choices) response to that request is not followed, so the
// product of the loaded data file is never asked for in its place. The update
// ends with the error of any other response.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/data-updates.md#configured-product
func TestProductSetIsNotReplacedOnA300(t *testing.T) {
	var mu sync.Mutex
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			asked = append(asked, r.URL.Query().Get("Product"))
			mu.Unlock()
			w.WriteHeader(http.StatusMultipleChoices)
			w.Write([]byte(`{"AvailableUris":[` +
				`"https://distributor.example/api/v2/download?Product=IPIV4Basic",` +
				`"https://distributor.example/api/v2/download?Product=IPIV4Enterprise"]}`))
		}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "data.file")
	if err := os.WriteFile(path, []byte("current"), 0644); err != nil {
		t.Fatalf("write the data file: %v", err)
	}
	engine := &Engine{
		FileUpdater: common_go.NewFileUpdater(defaultDataFileUrl),
		licenseKey:  productTestKey,
		product:     "IPIV4Enterprise",
	}
	if err := engine.validateAndAppendUrlParams(); err != nil {
		t.Fatalf("the data update URL could not be completed: %v", err)
	}
	parsed, err := url.Parse(engine.GetDataFileUrl())
	if err != nil {
		t.Fatalf("the data update URL could not be parsed: %v", err)
	}
	// The test server stands in for the Distributor. It gets the query the
	// engine made, and the loaded data file is of a product it offers.
	engine.SetDataFileUrl(server.URL + "/api/v2/download?" + parsed.RawQuery)
	engine.SetDataFile(path)
	engine.SetProductFunc(func() string { return "IPIV4Basic" })
	log := &reloadTestLog{}
	engine.SetCustomLogger(log)

	if _, err := engine.PullOnStart(); err != nil {
		t.Fatalf("the update on start-up failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(asked) != 1 || asked[0] != "IPIV4Enterprise" {
		t.Errorf("the requests asked for the products %q, want IPIV4Enterprise once", asked)
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
