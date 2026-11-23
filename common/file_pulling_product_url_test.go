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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// countingServer answers every request with 404 and counts them. It stands
// for the address a 300 response names in its Location header.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
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

// queries are the raw queries of the requests made, in order.
func (d *scriptedDistributor) queries() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var queries []string
	for _, r := range d.requests {
		queries = append(queries, r.URL.RawQuery)
	}
	return queries
}

// products are the Product values of each request, in order.
func (d *scriptedDistributor) products() [][]string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var products [][]string
	for _, r := range d.requests {
		products = append(products, r.URL.Query()["Product"])
	}
	return products
}

// maskedSecondURL is the second request, asking for the product given, as a
// message shows it. It is the first with the product added to the end of its
// query.
func (d *scriptedDistributor) maskedSecondURL(product string) string {
	return d.maskedURL() + "&Product=" + product
}

// productLines are the messages, for a test failure.
func productLines(log *licenseKeyLog) string {
	return strings.Join(log.all(), "\n")
}

// The Location header of a 300 response names the first URI of its list,
// which can be of another product. Neither the update nor its HTTP client
// follows it, so only the first request's address is asked, twice.
// See https://github.com/51Degrees/specifications/blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/features/data-updates.md#license-keys-with-several-products
func TestPull_DoesNotFollowTheLocationOfA300(t *testing.T) {
	location, followed := countingServer(t)
	offered := func(product string) string {
		return location.URL + "/api/v2/download?LicenseKeys=" + productTestKey +
			"&Type=HashV41&Product=" + product
	}
	d := newScriptedDistributor(
		severalProductsAt(offered("IPIV4Enterprise"), fmt.Sprintf(
			`{"AvailableUris":[%q,%q]}`,
			offered("IPIV4Enterprise"), offered("V4TAC"))),
		dataFile)
	defer d.server.Close()
	updater, log := d.updater(t, "V4TAC")

	_, downloaded, err := updater.pull(make(chan struct{}, 1))

	if err != nil || !downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want the data file\n%s",
			downloaded, err, productLines(log))
	}
	if n := followed.Load(); n != 0 {
		t.Errorf("%d requests went to the address of the Location header, want none", n)
	}
	got := d.products()
	if len(got) != 2 || len(got[0]) != 0 || len(got[1]) != 1 || got[1][0] != "V4TAC" {
		t.Errorf("the requests asked for the products %q, want none and then V4TAC", got)
	}
	noFullKey(t, log, nil)
}

// A Product with a value in the caller's own data update URL is a configured
// product, whatever the case of the parameter's name. The URL is requested as
// it is, a 300 response is the error of any other response, and the product
// of the loaded data file is not looked up.
// See https://github.com/51Degrees/specifications/blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/features/data-updates.md#configured-product
func TestPull_ProductInTheURLIsNotReplacedOnA300(t *testing.T) {
	for _, parameter := range []string{"Product", "product", "PRODUCT"} {
		t.Run(parameter, func(t *testing.T) {
			d := newScriptedDistributor(
				severalProducts(availableUris(
					"LicenseKeys="+productTestKey+"&Product=V4Enterprise",
					"LicenseKeys="+productTestKey+"&Product=V4TAC")),
				dataFile)
			defer d.server.Close()
			updater, log := d.updater(t, "V4TAC")
			configured := d.url() + "&" + parameter + "=V4Enterprise"
			updater.SetDataFileUrl(configured)
			var lookups atomic.Int32
			updater.SetProductFunc(func() string {
				lookups.Add(1)
				return "V4TAC"
			})

			_, downloaded, err := updater.pull(make(chan struct{}, 1))

			if err == nil || downloaded {
				t.Fatalf("pull gave downloaded %v and error %v, want an error", downloaded, err)
			}
			queries := d.queries()
			want := configured[strings.IndexByte(configured, '?')+1:]
			if len(queries) != 1 || queries[0] != want {
				t.Errorf("the requests had the queries %q, want %q once", queries, want)
			}
			line := "ERROR Error during check for data update. HTTP status code '300' from " +
				"data update service at '" + MaskLicenseKeysIn(configured, []string{productTestKey}) +
				"' for engine 'Engine'"
			if !hasLine(log, line) {
				t.Errorf("no message is %q, the messages being\n%s", line, productLines(log))
			}
			if logHas(log, "Requesting product") {
				t.Error("a message says a product of the response is requested")
			}
			if n := lookups.Load(); n != 0 {
				t.Errorf("the product of the data file was looked up %d times, want none", n)
			}
			if status := pullUpdateStatus(err); status != UpdateHTTPSError {
				t.Errorf("status = %s, want %s", status, UpdateHTTPSError)
			}
			noFullKey(t, log, err)
		})
	}
}

// An empty Product in the caller's own data update URL is no product. The
// URL is requested as it is, the 300 response is followed, and the second
// request has the product of the loaded data file as its only Product.
func TestPull_EmptyProductInTheURLIsNotConfigured(t *testing.T) {
	d := newScriptedDistributor(
		severalProducts(availableUris(
			"LicenseKeys="+productTestKey+"&Product=V4Enterprise",
			"LicenseKeys="+productTestKey+"&Product=V4TAC")),
		dataFile)
	defer d.server.Close()
	updater, log := d.updater(t, "V4TAC")
	configured := d.url() + "&Product="
	updater.SetDataFileUrl(configured)

	_, downloaded, err := updater.pull(make(chan struct{}, 1))

	if err != nil || !downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want the data file\n%s",
			downloaded, err, productLines(log))
	}
	queries := d.queries()
	if len(queries) != 2 || queries[0] != configured[strings.IndexByte(configured, '?')+1:] {
		t.Errorf("the requests had the queries %q, want the configured one first", queries)
	}
	if got := d.products(); len(got) != 2 || len(got[1]) != 1 || got[1][0] != "V4TAC" {
		t.Errorf("the requests asked for the products %q, want V4TAC once in the second", got)
	}
	noFullKey(t, log, nil)
}

// A 300 response can come after a 301, 302, 307 or 308 redirect. The address
// that answered 300 is then the URL of the first request, so the second
// request goes there and the messages name it.
// See https://github.com/51Degrees/specifications/blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/features/data-updates.md#license-keys-with-several-products
func TestPull_SecondRequestGoesToTheAddressThatAnswered300(t *testing.T) {
	for _, status := range []int{
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect,
	} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			d := newScriptedDistributor(
				severalProducts(availableUris(
					"LicenseKeys="+productTestKey+"&Product=IPIV4Enterprise",
					"LicenseKeys="+productTestKey+"&Product=V4TAC")),
				dataFile)
			defer d.server.Close()
			var moved atomic.Int32
			redirect := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					moved.Add(1)
					http.Redirect(w, r, d.server.URL+r.URL.RequestURI(), status)
				}))
			defer redirect.Close()
			updater, log := d.updater(t, "V4TAC")
			updater.SetDataFileUrl(redirect.URL + "/api/v2/download?LicenseKeys=" +
				productTestKey + "&Type=HashV41&Download=True")

			_, downloaded, err := updater.pull(make(chan struct{}, 1))

			if err != nil || !downloaded {
				t.Fatalf("pull gave downloaded %v and error %v, want the data file\n%s",
					downloaded, err, productLines(log))
			}
			if n := moved.Load(); n != 1 {
				t.Errorf("%d requests went to the configured address, want 1", n)
			}
			if got := d.products(); len(got) != 2 || len(got[1]) != 1 || got[1][0] != "V4TAC" {
				t.Errorf("the address that answered 300 was asked for the products %q, "+
					"want none and then V4TAC", got)
			}
			for _, line := range []string{
				"INFO Requesting product 'V4TAC' from '" + d.maskedURL() + "' for Engine 'Engine'",
				"INFO Checking for update from '" + d.maskedSecondURL("V4TAC") + "' for Engine 'Engine'",
				"INFO Downloaded new data from '" + d.maskedSecondURL("V4TAC") + "' for Engine 'Engine'",
			} {
				if !hasLine(log, line) {
					t.Errorf("no message is %q, the messages being\n%s", line, productLines(log))
				}
			}
			noFullKey(t, log, nil)
		})
	}
}

// Each message after the second request names the URL of the second request,
// whatever its response.
// See https://github.com/51Degrees/specifications/blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/features/data-updates.md#errors
func TestPull_MessagesAfterTheSecondRequestNameIt(t *testing.T) {
	notModified := func(w http.ResponseWriter) { w.WriteHeader(http.StatusNotModified) }
	forbidden := func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"Error":"The license key provided does not contain the requested product."}`))
	}
	serverError := func(w http.ResponseWriter) { w.WriteHeader(http.StatusInternalServerError) }
	noMD5 := func(w http.ResponseWriter) { w.Write([]byte("new data")) }
	for name, c := range map[string]struct {
		answer func(w http.ResponseWriter)
		lines  func(second string) []string
	}{
		"304": {notModified, func(second string) []string {
			return []string{" found at '" + second + "' for Engine 'Engine'"}
		}},
		"403": {forbidden, func(second string) []string {
			return []string{"ERROR Error during check for data update. Access denied to data " +
				"update service at '" + second + "' for engine 'Engine'"}
		}},
		"500": {serverError, func(second string) []string {
			return []string{"ERROR Error during check for data update. HTTP status code " +
				"'500' from data update service at '" + second + "' for engine 'Engine'"}
		}},
		"200 with no MD5": {noMD5, func(second string) []string {
			return []string{
				"WARN No MD5 hash included in data update response for '" + second +
					"'. Unable to verify data integrity",
				"INFO Downloaded new data from '" + second + "' for Engine 'Engine'",
			}
		}},
	} {
		t.Run(name, func(t *testing.T) {
			d := newScriptedDistributor(
				severalProducts(availableUris(
					"LicenseKeys="+productTestKey+"&Product=IPIV4Enterprise",
					"LicenseKeys="+productTestKey+"&Product=V4TAC")),
				c.answer)
			defer d.server.Close()
			updater, log := d.updater(t, "V4TAC")

			_, _, err := updater.pull(make(chan struct{}, 1))

			if d.count() != 2 {
				t.Fatalf("%d requests were made, want 2\n%s", d.count(), productLines(log))
			}
			for _, part := range c.lines(d.maskedSecondURL("V4TAC")) {
				if !logHas(log, part) {
					t.Errorf("no message holds %q, the messages being\n%s", part, productLines(log))
				}
			}
			first := "'" + d.maskedURL() + "'"
			after := false
			for _, line := range log.all() {
				if strings.Contains(line, "Checking for update from '"+d.maskedSecondURL("V4TAC")) {
					after = true
					continue
				}
				if after && strings.Contains(line, first) {
					t.Errorf("%q, logged after the second request, names the first", line)
				}
			}
			noFullKey(t, log, err)
		})
	}
}

// The two requests of a followed 300 are one check, which update polling
// reports once, with the status the second response gives.
// See https://github.com/51Degrees/specifications/blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/features/data-updates.md#license-keys-with-several-products
func TestPull_TwoRequestsAreReportedOnce(t *testing.T) {
	notModified := func(w http.ResponseWriter) { w.WriteHeader(http.StatusNotModified) }
	for name, c := range map[string]struct {
		second func(w http.ResponseWriter)
		want   UpdateStatus
	}{
		"304": {notModified, UpdateNotNeeded},
		"a second 300": {severalProducts(availableUris(
			"LicenseKeys=" + productTestKey + "&Product=V4TAC")), UpdateHTTPSError},
	} {
		t.Run(name, func(t *testing.T) {
			d := newScriptedDistributor(
				severalProducts(availableUris(
					"LicenseKeys="+productTestKey+"&Product=IPIV4Enterprise",
					"LicenseKeys="+productTestKey+"&Product=V4TAC")),
				c.second)
			defer d.server.Close()
			updater, log := d.updater(t, "V4TAC")
			var mu sync.Mutex
			var reports []UpdateStatus
			updater.OnUpdateComplete(func(u UpdateComplete) {
				mu.Lock()
				defer mu.Unlock()
				reports = append(reports, u.Status)
			})

			updater.pullRecovering(context.Background(), make(chan struct{}, 1))

			if d.count() != 2 {
				t.Errorf("%d requests were made, want 2\n%s", d.count(), productLines(log))
			}
			mu.Lock()
			defer mu.Unlock()
			if len(reports) != 1 || reports[0] != c.want {
				t.Errorf("the check was reported as %v, want %v once", reports, c.want)
			}
		})
	}
}

// The name of the Product parameter of each offered URI is matched ignoring
// case, so a URI that writes it in another case still offers its product.
// See https://github.com/51Degrees/specifications/blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/features/data-updates.md#license-keys-with-several-products
func TestPull_MatchesTheProductParameterIgnoringCase(t *testing.T) {
	for _, parameter := range []string{"product", "PRODUCT", "pRoDuCt"} {
		t.Run(parameter, func(t *testing.T) {
			d := newScriptedDistributor(
				severalProducts(availableUris(
					"LicenseKeys="+productTestKey+"&"+parameter+"=IPIV4Enterprise",
					"LicenseKeys="+productTestKey+"&"+parameter+"=V4TAC")),
				dataFile)
			defer d.server.Close()
			updater, log := d.updater(t, "V4TAC")

			_, downloaded, err := updater.pull(make(chan struct{}, 1))

			if err != nil || !downloaded {
				t.Fatalf("pull gave downloaded %v and error %v, want the data file\n%s",
					downloaded, err, productLines(log))
			}
			if got := d.products(); len(got) != 2 || len(got[1]) != 1 || got[1][0] != "V4TAC" {
				t.Errorf("the requests asked for the products %q, want none and then V4TAC", got)
			}
			line := "INFO Requesting product 'V4TAC' from '" + d.maskedURL() + "' for Engine 'Engine'"
			if !hasLine(log, line) {
				t.Errorf("no message is %q, the messages being\n%s", line, productLines(log))
			}
		})
	}
}

// A redirect to an address that names a product, which answers 300, is not
// followed further. The request named a product, so the 300 is the error of
// any other response, and the product of the loaded data file is not asked
// for in place of the one named.
func TestPull_ARedirectToAProductIsNotReplacedOnA300(t *testing.T) {
	d := newScriptedDistributor(
		severalProducts(availableUris(
			"LicenseKeys="+productTestKey+"&Product=V4Enterprise",
			"LicenseKeys="+productTestKey+"&Product=V4TAC")),
		dataFile)
	defer d.server.Close()
	redirect := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, d.server.URL+r.URL.RequestURI()+"&Product=V4Enterprise",
				http.StatusTemporaryRedirect)
		}))
	defer redirect.Close()
	updater, log := d.updater(t, "V4TAC")
	updater.SetDataFileUrl(redirect.URL + "/api/v2/download?LicenseKeys=" +
		productTestKey + "&Type=HashV41&Download=True")

	_, downloaded, err := updater.pull(make(chan struct{}, 1))

	if err == nil || downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want an error", downloaded, err)
	}
	if got := d.products(); len(got) != 1 || len(got[0]) != 1 || got[0][0] != "V4Enterprise" {
		t.Errorf("the address that answered 300 was asked for the products %q, "+
			"want V4Enterprise once", got)
	}
	line := "ERROR Error during check for data update. HTTP status code '300' from " +
		"data update service at '" + d.maskedURL() + "&Product=V4Enterprise' for engine 'Engine'"
	if !hasLine(log, line) {
		t.Errorf("no message is %q, the messages being\n%s", line, productLines(log))
	}
	noFullKey(t, log, err)
}
