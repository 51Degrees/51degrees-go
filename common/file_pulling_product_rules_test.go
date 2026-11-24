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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// scriptedDistributor answers each request with the next of its answers, and
// the last of them once they run out.
type scriptedDistributor struct {
	server  *httptest.Server
	answers []func(w http.ResponseWriter)

	mu       sync.Mutex
	requests []*http.Request
}

func newScriptedDistributor(answers ...func(w http.ResponseWriter)) *scriptedDistributor {
	d := &scriptedDistributor{answers: answers}
	d.server = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			d.mu.Lock()
			i := len(d.requests)
			d.requests = append(d.requests, r)
			d.mu.Unlock()
			if i >= len(d.answers) {
				i = len(d.answers) - 1
			}
			d.answers[i](w)
		}))
	return d
}

// count is how many requests were made.
func (d *scriptedDistributor) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.requests)
}

// url is the first request of an update, which names no product.
func (d *scriptedDistributor) url() string {
	return d.server.URL + "/api/v2/download?LicenseKeys=" + productTestKey +
		"&Type=HashV41&Download=True"
}

// maskedURL is the first request as a message shows it.
func (d *scriptedDistributor) maskedURL() string {
	return d.server.URL + "/api/v2/download?LicenseKeys=" +
		MaskLicenseKey(productTestKey) + "&Type=HashV41&Download=True"
}

// updater is a file updater that pulls from the distributor into a data file
// holding "current", for an engine whose data file is of the product given.
func (d *scriptedDistributor) updater(t *testing.T, product string) (*FileUpdater, *licenseKeyLog) {
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

// severalProducts answers 300 with the body given, as the Distributor does,
// with a Location header that names the first URI of its list.
func severalProducts(body string) func(w http.ResponseWriter) {
	var choice struct{ AvailableUris []string }
	location := ""
	if json.Unmarshal([]byte(body), &choice) == nil && len(choice.AvailableUris) > 0 {
		location = choice.AvailableUris[0]
	}
	return severalProductsAt(location, body)
}

// severalProductsAt answers 300 with the body given and, when location is
// set, a Location header that names it.
func severalProductsAt(location, body string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		if location != "" {
			w.Header().Set("Location", location)
		}
		w.WriteHeader(http.StatusMultipleChoices)
		w.Write([]byte(body))
	}
}

// availableUris is the body of a 300 response that lists a URI with each of
// the queries given.
func availableUris(queries ...string) string {
	uris := make([]string, 0, len(queries))
	for _, query := range queries {
		uris = append(uris, "https://distributor.example/api/v2/download?"+query)
	}
	body, _ := json.Marshal(map[string]interface{}{"AvailableUris": uris})
	return string(body)
}

// dataFile answers 200 with a data file.
func dataFile(w http.ResponseWriter) {
	body := []byte("new data")
	w.Header().Set("Content-MD5", calculateMD5(body))
	w.Write(body)
}

// hasLine reports whether one message is exactly the line, which starts
// with its level.
func hasLine(log *licenseKeyLog, line string) bool {
	for _, logged := range log.all() {
		if logged == line {
			return true
		}
	}
	return false
}

// A URI with no Product, or an empty one, is ignored, and the product is
// requested with the message the specification gives.
// See https://github.com/51Degrees/specifications/blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/features/data-updates.md#license-keys-with-several-products
func TestPull_IgnoresAURIWithNoProduct(t *testing.T) {
	d := newScriptedDistributor(
		severalProducts(availableUris(
			"LicenseKeys="+productTestKey,
			"LicenseKeys="+productTestKey+"&Product=",
			"LicenseKeys="+productTestKey+"&Product=V4TAC")),
		dataFile)
	defer d.server.Close()
	updater, log := d.updater(t, "V4TAC")

	_, downloaded, err := updater.pull(make(chan struct{}, 1))

	if err != nil || !downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want the data file", downloaded, err)
	}
	if d.count() != 2 {
		t.Errorf("%d requests were made, want 2", d.count())
	}
	want := "INFO Requesting product 'V4TAC' from '" + d.maskedURL() + "' for Engine 'Engine'"
	if !hasLine(log, want) {
		t.Errorf("no message is %q, the messages being\n%s", want,
			strings.Join(log.all(), "\n"))
	}
	noFullKey(t, log, nil)
}

// A product that is not offered gives the whole message of the specification,
// with the license key masked and the products in the order of the response.
func TestPull_NotOfferedMessage(t *testing.T) {
	for name, c := range map[string]struct {
		queries []string
		offered string
	}{
		"two products": {[]string{
			"LicenseKeys=" + productTestKey + "&Product=IPIV4Enterprise",
			"LicenseKeys=" + productTestKey,
			"LicenseKeys=" + productTestKey + "&Product=V4TAC"},
			"IPIV4Enterprise,V4TAC"},
	} {
		t.Run(name, func(t *testing.T) {
			d := newScriptedDistributor(severalProducts(availableUris(c.queries...)))
			defer d.server.Close()
			updater, log := d.updater(t, "V4Lite")

			_, downloaded, err := updater.pull(make(chan struct{}, 1))

			if err == nil || downloaded {
				t.Fatalf("pull gave downloaded %v and error %v, want an error", downloaded, err)
			}
			if d.count() != 1 {
				t.Errorf("%d requests were made, want 1", d.count())
			}
			want := "ERROR Error during check for data update. Product 'V4Lite' is not " +
				"offered by data update service at '" + d.maskedURL() +
				"' for engine 'Engine'. Products offered: '" + c.offered + "'"
			if !hasLine(log, want) {
				t.Errorf("no message is %q, the messages being\n%s", want,
					strings.Join(log.all(), "\n"))
			}
			noFullKey(t, log, err)
		})
	}
}

// A 300 response to the second request is not followed. It is the error of
// any other response, and its message names the second request.
func TestPull_DoesNotFollowASecond300(t *testing.T) {
	d := newScriptedDistributor(severalProducts(availableUris(
		"LicenseKeys=" + productTestKey + "&Product=V4TAC")))
	defer d.server.Close()
	updater, log := d.updater(t, "V4TAC")

	_, downloaded, err := updater.pull(make(chan struct{}, 1))

	if err == nil || downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want an error", downloaded, err)
	}
	if d.count() != 2 {
		t.Errorf("%d requests were made, want 2", d.count())
	}
	want := "ERROR Error during check for data update. HTTP status code '300' from " +
		"data update service at '" + d.maskedSecondURL("V4TAC") + "' for engine 'Engine'"
	if !hasLine(log, want) {
		t.Errorf("no message is %q, the messages being\n%s", want,
			strings.Join(log.all(), "\n"))
	}
	if status := pullUpdateStatus(err); status != UpdateHTTPSError {
		t.Errorf("status = %s, want %s", status, UpdateHTTPSError)
	}
	noFullKey(t, log, err)
}

// A 300 response whose body is not JSON with a list of AvailableUris, or
// whose list is empty or gives no Product value, is the error of any other
// response, and no second request is made.
func TestPull_300WithoutAListIsAnHTTPError(t *testing.T) {
	tooLong := `{"Padding":"` + strings.Repeat("a", 2*productChoiceBytes) +
		`","AvailableUris":["https://distributor.example/?Product=V4TAC"]}`
	noProduct := availableUris(
		"LicenseKeys="+productTestKey,
		"LicenseKeys="+productTestKey+"&Product=")
	for name, body := range map[string]string{
		"not JSON":        "There are multiple products",
		"no list":         `{"Error":"There are multiple products"}`,
		"an empty list":   `{"AvailableUris":[]}`,
		"a list of other": `{"AvailableUris":"V4TAC"}`,
		"too long a body": tooLong,
		"no product name": noProduct,
		// The name of the list is matched by exact case.
		"a list named in another case": `{"availableUris":["https://distributor.example/?Product=V4TAC"]}`,
		"no string in the list":        `{"AvailableUris":[1,null,{"Product":"V4TAC"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			d := newScriptedDistributor(severalProducts(body))
			defer d.server.Close()
			updater, log := d.updater(t, "V4TAC")

			_, downloaded, err := updater.pull(make(chan struct{}, 1))

			if err == nil || downloaded {
				t.Fatalf("pull gave downloaded %v and error %v, want an error", downloaded, err)
			}
			if d.count() != 1 {
				t.Errorf("%d requests were made, want 1", d.count())
			}
			want := "ERROR Error during check for data update. HTTP status code '300' from " +
				"data update service at '" + d.maskedURL() + "' for engine 'Engine'"
			if !hasLine(log, want) {
				t.Errorf("no message is %q, the messages being\n%s", want,
					strings.Join(log.all(), "\n"))
			}
			noFullKey(t, log, err)
		})
	}
}

// The second request is logged as a request is, so the check for an update is
// logged for the first request and again for the second, with the license
// key masked in both.
func TestPull_LogsTheSecondRequest(t *testing.T) {
	d := newScriptedDistributor(
		severalProducts(availableUris(
			"LicenseKeys="+productTestKey+"&Product=IPIV4Enterprise",
			"LicenseKeys="+productTestKey+"&Product=V4TAC")),
		dataFile)
	defer d.server.Close()
	updater, log := d.updater(t, "V4TAC")

	_, downloaded, err := updater.pull(make(chan struct{}, 1))

	if err != nil || !downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want the data file", downloaded, err)
	}
	if d.count() != 2 {
		t.Errorf("%d requests were made, want 2", d.count())
	}
	second := d.maskedSecondURL("V4TAC")
	for _, masked := range []string{d.maskedURL(), second} {
		want := "INFO Checking for update from '" + masked + "' for Engine 'Engine'"
		if !hasLine(log, want) {
			t.Errorf("no message is %q, the messages being\n%s", want,
				strings.Join(log.all(), "\n"))
		}
	}
	checks := 0
	for _, line := range log.all() {
		if strings.HasPrefix(line, "INFO Checking for update from '") {
			checks++
		}
	}
	if checks != 2 {
		t.Errorf("%d messages say a request checks for an update, want 2", checks)
	}
	noFullKey(t, log, nil)
}

// The second request is the first with its Product set to the product, in
// place of any Product it has whatever the case or escaping of its name, and
// with its other parameters as they were written, in their order.
func TestURLWithProduct(t *testing.T) {
	for name, c := range map[string]struct{ first, query string }{
		"no product": {
			"https://d.example/api?LicenseKeys=A%7CB&Type=HashV41&Download=True",
			"LicenseKeys=A%7CB&Type=HashV41&Download=True&Product=V4TAC"},
		"an empty product": {
			"https://d.example/api?LicenseKeys=A%7CB&Product=&Type=HashV41&Download=True",
			"LicenseKeys=A%7CB&Type=HashV41&Download=True&Product=V4TAC"},
		"another product": {
			"https://d.example/api?LicenseKeys=A%7CB&Type=HashV41&Download=True&product=Other",
			"LicenseKeys=A%7CB&Type=HashV41&Download=True&Product=V4TAC"},
		"an escaped name": {
			"https://d.example/api?Pro%64uct=Other&LicenseKeys=A%7CB&Type=HashV41&Download=True",
			"LicenseKeys=A%7CB&Type=HashV41&Download=True&Product=V4TAC"},
		"a key not escaped": {
			"https://d.example/api?LicenseKeys=A|B&Type=HashV41&Download=True&PRODUCT=Other&" +
				"Product=More",
			"LicenseKeys=A|B&Type=HashV41&Download=True&Product=V4TAC"},
	} {
		t.Run(name, func(t *testing.T) {
			first := c.first
			second, err := urlWithProduct(first, "V4TAC")
			if err != nil {
				t.Fatalf("urlWithProduct failed: %v", err)
			}
			parsed, err := url.Parse(second)
			if err != nil {
				t.Fatalf("%q cannot be parsed: %v", second, err)
			}
			if parsed.Host != "d.example" || parsed.Path != "/api" {
				t.Errorf("the second request goes to %q, want the address of the first", second)
			}
			want := url.Values{
				"LicenseKeys": {"A|B"},
				"Type":        {"HashV41"},
				"Download":    {"True"},
				"Product":     {"V4TAC"},
			}
			if got := parsed.Query(); got.Encode() != want.Encode() {
				t.Errorf("the second request has the query %q, want %q",
					got.Encode(), want.Encode())
			}
			if parsed.RawQuery != c.query {
				t.Errorf("the second request has the query %q as written, want %q",
					parsed.RawQuery, c.query)
			}
		})
	}
}

// Only a product name of letters, digits, '_', '-' and '.', of 64 characters
// at most, is taken from a response, and the first 32 of them are kept, so
// what a message holds of a response is bounded.
func TestPull_BoundsTheProductsOffered(t *testing.T) {
	queries := []string{
		"Product=V4TAC",
		"Product=Two%0ALines",
		"Product=With+Space",
		"Product=" + strings.Repeat("a", productNameCharacters+1),
		"Product=" + strings.Repeat("b", productNameCharacters),
		"Product=IPIV4_Enter-prise.1",
	}
	want := []string{"V4TAC", strings.Repeat("b", productNameCharacters), "IPIV4_Enter-prise.1"}
	for i := 0; i < 2*productsOfferedCount; i++ {
		name := fmt.Sprintf("P%d", i)
		queries = append(queries, "Product="+name)
		if len(want) < productsOfferedCount {
			want = append(want, name)
		}
	}
	d := newScriptedDistributor(severalProducts(availableUris(queries...)))
	defer d.server.Close()
	updater, log := d.updater(t, "V4Lite")

	_, _, err := updater.pull(make(chan struct{}, 1))

	if err == nil {
		t.Fatal("pull gave no error, want the product not offered")
	}
	line := "ERROR Error during check for data update. Product 'V4Lite' is not " +
		"offered by data update service at '" + d.maskedURL() +
		"' for engine 'Engine'. Products offered: '" + strings.Join(want, ",") + "'"
	if !hasLine(log, line) {
		t.Errorf("no message is %q, the messages being\n%s", line,
			strings.Join(log.all(), "\n"))
	}
}
