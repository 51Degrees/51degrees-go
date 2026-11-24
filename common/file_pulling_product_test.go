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

package common_go

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// productTestKey is the made-up license key the product tests put in the
// data file URL.
const productTestKey = "TESTKEY"

// fakeDistributor answers as the Distributor does for a license key that
// holds the products in offered. A request that names none of them gets 300
// (Multiple Choices) with a URI for each product, and a request that names
// one gets the data file of that product.
type fakeDistributor struct {
	server *httptest.Server
	// offered are the products the license key holds.
	offered []string
	// host, when set, is written in the offered URIs in place of the address
	// of the server.
	host string

	mu       sync.Mutex
	requests []*http.Request
}

func newFakeDistributor(offered ...string) *fakeDistributor {
	d := &fakeDistributor{offered: offered}
	d.server = httptest.NewServer(http.HandlerFunc(d.serve))
	return d
}

// url is the first request of an update, which names no product.
func (d *fakeDistributor) url() string {
	return d.server.URL + "/api/v2/download?LicenseKeys=" + productTestKey +
		"&Type=HashV41&Download=True"
}

// productData is the data file the fake distributor gives for a product.
func productData(product string) string {
	return "data file of " + product
}

func (d *fakeDistributor) serve(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	d.requests = append(d.requests, r)
	d.mu.Unlock()
	asked := r.URL.Query().Get("Product")
	for _, product := range d.offered {
		if product == asked || len(d.offered) == 1 {
			body := []byte(productData(product))
			w.Header().Set("Content-MD5", calculateMD5(body))
			w.Write(body)
			return
		}
	}
	host := d.server.URL
	if d.host != "" {
		host = d.host
	}
	var uris []string
	for _, product := range d.offered {
		uris = append(uris, host+"/api/v2/download?LicenseKeys="+
			productTestKey+"&LicenceKeys=&Type=HashV41&Product="+product)
	}
	body, _ := json.Marshal(map[string]interface{}{
		"AvailableUris": uris,
		"Error": "There are multiple products associated with this license " +
			"key. Please use the URI returned in the Location header.",
	})
	if len(uris) > 0 {
		w.Header().Set("Location", uris[0])
	}
	w.WriteHeader(http.StatusMultipleChoices)
	w.Write(body)
}

// products are the Product each request asked for, in order.
func (d *fakeDistributor) products() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var asked []string
	for _, r := range d.requests {
		asked = append(asked, r.URL.Query().Get("Product"))
	}
	return asked
}

// productUpdater is a file updater that pulls from the fake distributor into
// a data file holding "current", for an engine whose data file is of the
// product given. Its messages are kept in the log returned.
func productUpdater(t *testing.T, d *fakeDistributor, product string) (*FileUpdater, *licenseKeyLog) {
	t.Helper()
	dataFile := filepath.Join(t.TempDir(), "data.hash")
	if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
		t.Fatalf("write the data file: %v", err)
	}
	updater := NewFileUpdater(d.url())
	updater.SetDataFile(dataFile)
	updater.SetIsFileWatcherEnabled(false)
	updater.SetProductFunc(func() string { return product })
	log := &licenseKeyLog{}
	updater.SetCustomLogger(log)
	return updater, log
}

// noFullKey fails the test when a message or the error holds the license key
// in full.
func noFullKey(t *testing.T, log *licenseKeyLog, err error) {
	t.Helper()
	for _, line := range log.all() {
		if strings.Contains(line, productTestKey) {
			t.Errorf("%q holds the license key in full", line)
		}
	}
	if err != nil && strings.Contains(err.Error(), productTestKey) {
		t.Errorf("the error %q holds the license key in full", err)
	}
}

// When the license key holds several products, the Distributor answers 300
// with a URI for each, and the update repeats its request with the product
// of the data file the engine has loaded added to it.
// See https://github.com/51Degrees/specifications/blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/features/data-updates.md#license-keys-with-several-products
func TestPull_FollowsTheProductOfTheDataFile(t *testing.T) {
	for _, product := range []string{"V4TAC", "IPIV4Enterprise"} {
		t.Run(product, func(t *testing.T) {
			d := newFakeDistributor("IPIV4Enterprise", "V4TAC")
			defer d.server.Close()
			updater, log := productUpdater(t, d, product)

			_, downloaded, err := updater.pull(make(chan struct{}, 1))

			if err != nil || !downloaded {
				t.Fatalf("pull gave downloaded %v and error %v, want the data file "+
					"of %s, the messages being\n%s", downloaded, err, product,
					strings.Join(log.all(), "\n"))
			}
			if got := readTempFile(t, updater.GetDataFile()); got != productData(product) {
				t.Errorf("data file holds %q, want %q", got, productData(product))
			}
			if asked := d.products(); len(asked) != 2 || asked[0] != "" || asked[1] != product {
				t.Errorf("the requests asked for the products %q, want none and then %s",
					asked, product)
			}
			d.mu.Lock()
			first := d.requests[0].Header.Get("If-Modified-Since")
			second := d.requests[len(d.requests)-1].Header.Get("If-Modified-Since")
			d.mu.Unlock()
			if first == "" || second != first {
				t.Errorf("If-Modified-Since was %q and then %q, want the same date", first, second)
			}
			d.mu.Lock()
			query := d.requests[len(d.requests)-1].URL.Query()
			d.mu.Unlock()
			if query.Get("Download") != "True" || query.Has("LicenceKeys") {
				t.Errorf("the second request had the query %q, want the query of "+
					"the first with the product added", query.Encode())
			}
			if !logHas(log, "Requesting product '"+product+"' from '", "' for Engine 'Engine'") {
				t.Errorf("no message says the product is requested, the messages being\n%s",
					strings.Join(log.all(), "\n"))
			}
			noFullKey(t, log, nil)
		})
	}
}

// When none of the products the license key holds is the product of the data
// file, or the engine names no product, the update fails with an error that
// names the products offered and never the license key.
func TestPull_NamesTheProductsOffered(t *testing.T) {
	for name, product := range map[string]string{
		"another product": "OtherProduct",
		"no product":      "",
	} {
		t.Run(name, func(t *testing.T) {
			d := newFakeDistributor("IPIV4Enterprise", "V4TAC")
			defer d.server.Close()
			updater, log := productUpdater(t, d, product)

			_, downloaded, err := updater.pull(make(chan struct{}, 1))

			if err == nil || downloaded {
				t.Fatalf("pull gave downloaded %v and error %v, want an error", downloaded, err)
			}
			if status := pullUpdateStatus(err); status != UpdateHTTPSError {
				t.Errorf("status = %s, want %s", status, UpdateHTTPSError)
			}
			for _, offered := range d.offered {
				if !strings.Contains(err.Error(), offered) {
					t.Errorf("the error %q does not name the product %s", err, offered)
				}
			}
			if !logHas(log, "Product '"+product+"' is not offered by data update service at '",
				"' for engine 'Engine'. Products offered: 'IPIV4Enterprise,V4TAC'") {
				t.Errorf("no message names the products offered, the messages being\n%s",
					strings.Join(log.all(), "\n"))
			}
			if product != "" && !strings.Contains(err.Error(), product) {
				t.Errorf("the error %q does not name the product %s asked for", err, product)
			}
			if got := readTempFile(t, updater.GetDataFile()); got != "current" {
				t.Errorf("data file holds %q, want it left as it was", got)
			}
			if asked := d.products(); len(asked) != 1 {
				t.Errorf("%d requests were made, want 1", len(asked))
			}
			noFullKey(t, log, err)
		})
	}
}

// A license key that holds one product gets its data file from the first
// request, whatever product the engine names.
func TestPull_OneProductNeedsNoChoice(t *testing.T) {
	d := newFakeDistributor("V4TAC")
	defer d.server.Close()
	updater, log := productUpdater(t, d, "V4TAC")

	_, downloaded, err := updater.pull(make(chan struct{}, 1))

	if err != nil || !downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want the data file", downloaded, err)
	}
	if got := readTempFile(t, updater.GetDataFile()); got != productData("V4TAC") {
		t.Errorf("data file holds %q, want %q", got, productData("V4TAC"))
	}
	if asked := d.products(); len(asked) != 1 || asked[0] != "" {
		t.Errorf("the requests asked for the products %q, want one request naming none", asked)
	}
	checks := 0
	for _, line := range log.all() {
		if strings.HasPrefix(line, "INFO Checking for update from '") {
			checks++
		}
	}
	if checks != 1 {
		t.Errorf("%d messages say a request checks for an update, want 1", checks)
	}
	noFullKey(t, log, nil)
}

// Only the product is taken from a URI the response offers, so the second
// request goes to the data update URL and never to the host of the URI.
func TestPull_NeverRequestsTheOfferedURI(t *testing.T) {
	var elsewhereMu sync.Mutex
	elsewhereRequests := 0
	elsewhere := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			elsewhereMu.Lock()
			elsewhereRequests++
			elsewhereMu.Unlock()
		}))
	defer elsewhere.Close()
	d := newFakeDistributor("IPIV4Enterprise", "V4TAC")
	defer d.server.Close()
	d.host = elsewhere.URL
	updater, log := productUpdater(t, d, "v4tac")

	_, downloaded, err := updater.pull(make(chan struct{}, 1))

	if err != nil || !downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want the data file", downloaded, err)
	}
	if got := readTempFile(t, updater.GetDataFile()); got != productData("V4TAC") {
		t.Errorf("data file holds %q, want %q", got, productData("V4TAC"))
	}
	elsewhereMu.Lock()
	defer elsewhereMu.Unlock()
	if elsewhereRequests != 0 {
		t.Errorf("%d requests went to the host of the offered URI, want none", elsewhereRequests)
	}
	noFullKey(t, log, err)
}

// logHas reports whether one message holds every part.
func logHas(log *licenseKeyLog, parts ...string) bool {
	for _, line := range log.all() {
		found := true
		for _, part := range parts {
			found = found && strings.Contains(line, part)
		}
		if found {
			return true
		}
	}
	return false
}
