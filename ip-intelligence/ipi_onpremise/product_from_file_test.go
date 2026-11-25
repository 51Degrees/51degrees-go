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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
)

// productNamesRules is the specification text these tests follow.
const productNamesRules = "https://github.com/51Degrees/specifications/" +
	"blob/a5699da253d13661478f5923964c22d1010c1df3/pipeline-specification/" +
	"features/data-updates.md#product-names"

// productsServer stands in for the Distributor for a license key that holds
// the products offered. A request that names no product gets 300 (Multiple
// Choices) with a URI for each product, one that names a product offered
// gets 304 (Not Modified), and any other gets 403 (Forbidden). Every request
// is kept in the log returned.
func productsServer(t *testing.T, offered ...string) (*httptest.Server, *requestLog) {
	t.Helper()
	requests := &requestLog{}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requests.add(r)
			product := r.URL.Query().Get("Product")
			if product == "" {
				var uris []string
				for _, name := range offered {
					uris = append(uris, server.URL+"/api/v2/download?LicenseKeys="+
						productTestKey+"&Type=IPIV41&Product="+name)
				}
				body, _ := json.Marshal(map[string]interface{}{"AvailableUris": uris})
				if len(uris) > 0 {
					w.Header().Set("Location", uris[0])
				}
				w.WriteHeader(http.StatusMultipleChoices)
				w.Write(body)
				return
			}
			for _, name := range offered {
				if name == product {
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"Error":"The license key provided does not ` +
				`contain the requested product."}`))
		}))
	t.Cleanup(server.Close)
	return server, requests
}

// unloadedEngine is an engine that has not loaded a data file, with the data
// file at path and the default data update URL, completed for the made-up
// license key, sending its query to server. Its product is the one the
// engine gives, as New sets it.
func unloadedEngine(t *testing.T, server *httptest.Server, path string) (*Engine, *reloadTestLog) {
	t.Helper()
	engine := &Engine{
		FileUpdater: common_go.NewFileUpdater(defaultDataFileUrl),
		licenseKey:  productTestKey,
	}
	if err := engine.validateAndAppendUrlParams(); err != nil {
		t.Fatalf("the data update URL could not be completed: %v", err)
	}
	parsed, err := url.Parse(engine.GetDataFileUrl())
	if err != nil {
		t.Fatalf("the data update URL could not be parsed: %v", err)
	}
	engine.SetDataFileUrl(server.URL + "/api/v2/download?" + parsed.RawQuery)
	engine.SetDataFile(path)
	engine.SetEngineName(engineTypeName)
	engine.SetProductFunc(engine.dataFileProduct)
	log := &reloadTestLog{}
	engine.SetCustomLogger(log)
	return engine, log
}

// Before the engine has loaded a data file, the product of the data file is
// read from its header, without loading it, and is the product the engine
// gives once it has loaded the same file.
func TestProductFromTheHeaderBeforeTheEngineLoads(t *testing.T) {
	path := writeDataFile(t, readGoodFile(t))
	server, _ := productsServer(t)
	engine, _ := unloadedEngine(t, server, path)

	if product := engine.dataFileProduct(); product != "IPIV4Asn" {
		t.Errorf("the data file is of the product %q before it is loaded, "+
			"want IPIV4Asn (%s)", product, productNamesRules)
	}

	loaded := newReloadTestEngine(t,
		WithDataFile(path),
		WithAutoUpdate(false),
		WithFileWatch(false))
	if product := loaded.dataFileProduct(); product != "IPIV4Asn" {
		t.Errorf("the loaded data file is of the product %q, want IPIV4Asn", product)
	}
}

// The update on start-up of an engine that has not loaded its data file asks
// for the product the header of the data file names, when the license key
// holds several products. The first request names no product, and the second
// asks for that product.
func TestUpdateOnStartAsksForTheProductInTheHeader(t *testing.T) {
	for name, file := range map[string]struct {
		path    func(t *testing.T) string
		product string
	}{
		"the free data file": {func(t *testing.T) string {
			return writeDataFile(t, readGoodFile(t))
		}, "IPIV4Asn"},
		"the Enterprise data file": {func(t *testing.T) string {
			return testDataPath(t, enterpriseTestDataFile)
		}, "IPIV4Enterprise"},
	} {
		t.Run(name, func(t *testing.T) {
			server, requests := productsServer(t, "V4TAC", file.product)
			engine, log := unloadedEngine(t, server, file.path(t))

			if _, err := engine.PullOnStart(); err != nil {
				t.Fatalf("the update on start-up failed: %v", err)
			}

			got := requests.products()
			if len(got) != 2 || len(got[0]) != 0 ||
				len(got[1]) != 1 || got[1][0] != file.product {
				t.Errorf("the requests asked for the products %q, want none "+
					"and then %s (%s)", got, file.product, productNamesRules)
			}
			if !log.has("Requesting product '" + file.product + "' from '") {
				t.Errorf("no message says %s is requested", file.product)
			}
			if log.has(productTestKey) {
				t.Error("a message holds the license key in full")
			}
		})
	}
}

// No product is known when there is no data file at the configured location
// or its header cannot be read, so the update on start-up makes only the
// first request and asks for no product.
func TestNoProductWithoutAReadableDataFile(t *testing.T) {
	folder := t.TempDir()
	notData := filepath.Join(folder, "not-data.file")
	if err := os.WriteFile(notData, []byte("not a data file"), 0644); err != nil {
		t.Fatalf("write the file: %v", err)
	}
	for name, path := range map[string]string{
		"no file":      filepath.Join(folder, "missing.file"),
		"no data file": notData,
	} {
		t.Run(name, func(t *testing.T) {
			server, requests := productsServer(t, "V4TAC", "IPIV4Asn")
			engine, log := unloadedEngine(t, server, path)

			if product := engine.dataFileProduct(); product != "" {
				t.Errorf("the product is %q, want none (%s)", product,
					productNamesRules)
			}
			// The error is the caller's when there is no data file, and
			// is logged otherwise, so only the requests are checked.
			engine.PullOnStart()

			if got := requests.products(); len(got) != 1 || len(got[0]) != 0 {
				t.Errorf("the requests asked for the products %q, want one "+
					"request that names none", got)
			}
			if log.has("Requesting product") {
				t.Error("a message says a product is requested")
			}
		})
	}
}

// Once the engine has loaded a data file, the product is the one the engine
// gives, even when the data file at the configured location has changed
// since and its header cannot be read.
func TestLoadedEngineProductWins(t *testing.T) {
	path := writeDataFile(t, readGoodFile(t))
	e := newReloadTestEngine(t,
		WithDataFile(path),
		WithTempDataCopy(true),
		WithAutoUpdate(false),
		WithFileWatch(false))
	if err := os.WriteFile(path, []byte("not a data file"), 0644); err != nil {
		t.Fatalf("replace the data file: %v", err)
	}

	if product := e.dataFileProduct(); product != "IPIV4Asn" {
		t.Errorf("the loaded data file is of the product %q, want IPIV4Asn (%s)",
			product, productNamesRules)
	}
}
