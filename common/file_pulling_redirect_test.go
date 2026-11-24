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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

// redirectRules is the specification text the redirect tests follow.
const redirectRules = "https://github.com/51Degrees/specifications/blob/" +
	"1651a6449e746bc33260e43c2d15610710f15375/pipeline-specification/features/" +
	"data-updates.md#license-keys-with-several-products"

// redirectChain answers a request for /hop/<n> with a 302 to /hop/<n-1>,
// keeping the query, and passes a request for /hop/0 to next. It counts the
// redirects it gives.
func redirectChain(t *testing.T, next http.HandlerFunc) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var redirects atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			hop, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/hop/"))
			if err != nil || hop <= 0 {
				next(w, r)
				return
			}
			redirects.Add(1)
			target := fmt.Sprintf("/hop/%d", hop-1)
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusFound)
		}))
	t.Cleanup(server.Close)
	return server, &redirects
}

// chainUpdater is a file updater whose data file URL starts the chain with
// the hops given, for an engine whose data file is of the product V4TAC.
func chainUpdater(t *testing.T, server *httptest.Server, hops int) (*FileUpdater, *licenseKeyLog, string) {
	t.Helper()
	d := &scriptedDistributor{server: server}
	updater, log := d.updater(t, "V4TAC")
	url := fmt.Sprintf("%s/hop/%d?LicenseKeys=%s&Type=HashV41&Download=True",
		server.URL, hops, productTestKey)
	updater.SetDataFileUrl(url)
	return updater, log, MaskLicenseKeysIn(url, []string{productTestKey})
}

// Redirects are followed up to 20 times, and a 21st is not followed. The
// redirect not followed is the error of any other response.
func TestPull_FollowsUpTo20Redirects(t *testing.T) {
	for _, c := range []struct {
		hops       int
		downloaded bool
	}{{redirectLimit, true}, {redirectLimit + 1, false}} {
		t.Run(fmt.Sprint(c.hops), func(t *testing.T) {
			server, redirects := redirectChain(t, func(w http.ResponseWriter, r *http.Request) {
				dataFile(w)
			})
			updater, log, shown := chainUpdater(t, server, c.hops)

			_, downloaded, err := updater.pull(make(chan struct{}, 1))

			if downloaded != c.downloaded {
				t.Fatalf("pull gave downloaded %v and error %v, want downloaded %v (%s)\n%s",
					downloaded, err, c.downloaded, redirectRules, productLines(log))
			}
			want := c.hops
			if want > redirectLimit+1 {
				want = redirectLimit + 1
			}
			if n := redirects.Load(); n != int32(want) {
				t.Errorf("%d redirects were given, want %d", n, want)
			}
			if !c.downloaded {
				line := "ERROR Error during check for data update. HTTP status code '302' from " +
					"data update service at '" + shown + "' for engine 'Engine'"
				if !hasLine(log, line) {
					t.Errorf("no message is %q, the messages being\n%s", line, productLines(log))
				}
				if status := pullUpdateStatus(err); status != UpdateHTTPSError {
					t.Errorf("status = %s, want %s", status, UpdateHTTPSError)
				}
			}
			noFullKey(t, log, err)
		})
	}
}

// The 20 redirects are for the whole check, so the redirects of the second
// request count with those of the first, as Java counts them.
func TestPull_RedirectsAreCountedOverTheCheck(t *testing.T) {
	const firstHops = 12
	for _, c := range []struct {
		secondHops int
		downloaded bool
	}{{redirectLimit - firstHops, true}, {redirectLimit - firstHops + 1, false}} {
		t.Run(fmt.Sprint(c.secondHops), func(t *testing.T) {
			hop := func(r *http.Request, prefix string) int {
				n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, prefix))
				return n
			}
			redirect := func(w http.ResponseWriter, r *http.Request, path string) {
				http.Redirect(w, r, path+"?"+r.URL.RawQuery, http.StatusFound)
			}
			server := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					switch {
					case strings.HasPrefix(r.URL.Path, "/first/") && hop(r, "/first/") > 0:
						redirect(w, r, fmt.Sprintf("/first/%d", hop(r, "/first/")-1))
					case strings.HasPrefix(r.URL.Path, "/first/") && r.URL.Query().Get("Product") == "":
						w.WriteHeader(http.StatusMultipleChoices)
						w.Write([]byte(availableUris("Product=V4TAC")))
					case strings.HasPrefix(r.URL.Path, "/first/"):
						// The second request goes to the address that
						// answered 300, which starts its own chain.
						redirect(w, r, fmt.Sprintf("/second/%d", c.secondHops-1))
					case hop(r, "/second/") > 0:
						redirect(w, r, fmt.Sprintf("/second/%d", hop(r, "/second/")-1))
					default:
						dataFile(w)
					}
				}))
			defer server.Close()
			d := &scriptedDistributor{server: server}
			updater, log := d.updater(t, "V4TAC")
			updater.SetDataFileUrl(fmt.Sprintf("%s/first/%d?LicenseKeys=%s&Type=HashV41&Download=True",
				server.URL, firstHops, productTestKey))

			_, downloaded, err := updater.pull(make(chan struct{}, 1))

			if downloaded != c.downloaded {
				t.Fatalf("pull gave downloaded %v and error %v, want downloaded %v (%s)\n%s",
					downloaded, err, c.downloaded, redirectRules, productLines(log))
			}
			if !c.downloaded {
				second := server.URL + "/first/0?LicenseKeys=" + MaskLicenseKey(productTestKey) +
					"&Type=HashV41&Download=True&Product=V4TAC"
				line := "ERROR Error during check for data update. HTTP status code '302' from " +
					"data update service at '" + second + "' for engine 'Engine'"
				if !hasLine(log, line) {
					t.Errorf("no message is %q, the messages being\n%s", line, productLines(log))
				}
			}
			noFullKey(t, log, err)
		})
	}
}

// A redirect from an https address to an http address is not followed. It
// is the error of any other response, and the http address gets no request.
func TestPull_DoesNotFollowARedirectFromHTTPSToHTTP(t *testing.T) {
	var plain atomic.Int32
	insecure := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			plain.Add(1)
			dataFile(w)
		}))
	defer insecure.Close()
	secure := httptest.NewTLSServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, insecure.URL+r.URL.RequestURI(), http.StatusFound)
		}))
	defer secure.Close()
	previous := dataFileTransport
	dataFileTransport = secure.Client().Transport
	defer func() { dataFileTransport = previous }()
	d := &scriptedDistributor{server: secure}
	updater, log := d.updater(t, "V4TAC")

	_, downloaded, err := updater.pull(make(chan struct{}, 1))

	if err == nil || downloaded {
		t.Fatalf("pull gave downloaded %v and error %v, want an error (%s)",
			downloaded, err, redirectRules)
	}
	if n := plain.Load(); n != 0 {
		t.Errorf("%d requests went to the http address, want none", n)
	}
	line := "ERROR Error during check for data update. HTTP status code '302' from " +
		"data update service at '" + d.maskedURL() + "' for engine 'Engine'"
	if !hasLine(log, line) {
		t.Errorf("no message is %q, the messages being\n%s", line, productLines(log))
	}
	noFullKey(t, log, err)
}
