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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
)

// productTestKey is the made-up license key the product tests put in the
// data update URL.
const productTestKey = "TESTKEY"

// The product of a data file is the prefix followed by its data tier, or the
// tier itself when it already starts with the prefix, and no tier gives no
// product.
func TestDistributorProduct(t *testing.T) {
	for tier, want := range map[string]string{
		"TAC":   "V4TAC",
		"Lite":  "V4Lite",
		"V4TAC": "V4TAC",
		"v4tac": "v4tac",
		"":      "",
	} {
		if got := distributorProduct(tier); got != want {
			t.Errorf("distributorProduct(%q) = %q, want %q", tier, got, want)
		}
	}
}

// severalProductsServer answers every request as the Distributor answers a
// request that names no product for a license key that holds the products.
func severalProductsServer(products ...string) *httptest.Server {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			var uris []string
			for _, product := range products {
				uris = append(uris, server.URL+"/api/v2/download?LicenseKeys="+
					productTestKey+"&LicenceKeys=&Type=HashV41&Product="+product)
			}
			body, _ := json.Marshal(map[string]interface{}{"AvailableUris": uris})
			w.WriteHeader(http.StatusMultipleChoices)
			w.Write(body)
		}))
	return server
}

// The free data file is of a product the license key does not hold, so an
// update with a license key that holds several products reports an HTTP error
// and logs the products offered, without the license key.
func TestUpdateNamesTheProductsOffered(t *testing.T) {
	good := withPastNextUpdate(t, readGoodFile(t))
	server := severalProductsServer("IPIV4Enterprise", "V4TAC")
	defer server.Close()
	reports := &reportLog{}
	log := &reloadTestLog{}
	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, good)),
		WithDataUpdateUrl(server.URL+"/api/v2/download?LicenseKeys="+
			productTestKey+"&Type=HashV41"),
		WithPollingInterval(1),
		WithRandomization(0),
		WithFileWatch(false),
		WithCustomLogger(log),
		WithUpdateCallback(reports.add))
	reports.watch(e)

	if product := e.dataFileProduct(); product != "V4Lite" {
		t.Errorf("the free data file is of the product %q, want V4Lite", product)
	}
	waitFor(t, "the update to report an HTTP error", func() bool {
		return reports.count(common_go.UpdateHTTPSError) > 0
	})
	if !log.has("Product 'V4Lite' is not offered by data update service at '",
		"' for engine '"+engineTypeName+"'. Products offered: 'IPIV4Enterprise,V4TAC'") {
		t.Error("no message names the products offered")
	}
	if log.has(productTestKey) {
		t.Error("a message holds the license key in full")
	}
}
