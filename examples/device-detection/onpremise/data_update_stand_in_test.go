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

// This file is the same in examples/device-detection/onpremise and
// examples/ip-intelligence, apart from the package name, so change both.
// Each folder of examples is a Go module of its own, and a package that both
// could import would have to be published with the modules, which test code
// must not be.

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The header of a Hash or an IP Intelligence data file holds the date it was
// published at publishedOffset, and the date the next data file is expected
// at nextUpdateOffset. Each is a 16 bit year, little endian, then a byte
// each for the month and the day.
const (
	publishedOffset  = 66
	nextUpdateOffset = 70
)

// The bodies the Distributor answers with when it is asked for a product the
// license key does not hold, and when it is asked for no product by a
// license key that holds several.
const (
	notOffered = "The license key provided does not contain the " +
		"requested product."
	severalProducts = "There are multiple products associated with this " +
		"license key. Please use the URI returned in the Location header."
)

// dataUpdateRequest is a request the stand-in received, with the product it
// named, or "" for none, and the status the stand-in answered with.
type dataUpdateRequest struct {
	product string
	status  int
}

// dataUpdateStandIn stands in for the Distributor on this machine, so that a
// test of an update example never reaches the Distributor. It answers as the
// Distributor does for a license key that holds several products, whatever
// key it is sent:
//   - a request that names no product gets 300 (Multiple Choices) with the
//     list of products, and a Location header that names the first
//   - a request for a product not on the list gets 403 (Forbidden)
//   - a request for a product on the list gets the data file to serve,
//     compressed and with the lower case hexadecimal MD5 hash of what is
//     sent, unless there is no file to serve or the request's
//     If-Modified-Since is not earlier than the file's published date, when
//     it gets 304 (Not Modified)
//
// Every request is recorded, so a test can check what the example asked for.
type dataUpdateStandIn struct {
	server    *httptest.Server
	products  []string
	gzipped   string
	md5       string
	published time.Time

	mu       sync.Mutex
	requests []dataUpdateRequest
}

// newDataUpdateStandIn starts a stand-in that lists products, in that order,
// and serves dataFile, or answers 304 to every request for a product on the
// list when dataFile is "". It stops when the test ends.
func newDataUpdateStandIn(t *testing.T, products []string,
	dataFile string) *dataUpdateStandIn {
	t.Helper()
	s := &dataUpdateStandIn{products: products}
	if dataFile != "" {
		s.published = readDate(t, dataFile, publishedOffset)
		s.gzipped = filepath.Join(t.TempDir(), filepath.Base(dataFile)+".gz")
		s.md5 = gzipFile(t, dataFile, s.gzipped)
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.answer))
	t.Cleanup(s.server.Close)
	return s
}

// url is the data update URL to give an example in place of the
// Distributor's.
func (s *dataUpdateStandIn) url() string {
	return s.server.URL + "/api/v2/download"
}

// received gives every request so far, in the order received.
func (s *dataUpdateStandIn) received() []dataUpdateRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]dataUpdateRequest(nil), s.requests...)
}

func (s *dataUpdateStandIn) record(product string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, dataUpdateRequest{product, status})
}

func (s *dataUpdateStandIn) answer(w http.ResponseWriter, r *http.Request) {
	product := productOf(r.URL)
	switch {
	case product == "":
		uris := make([]string, 0, len(s.products))
		for _, offered := range s.products {
			uris = append(uris, withProduct(r, offered))
		}
		s.record(product, http.StatusMultipleChoices)
		if len(uris) > 0 {
			w.Header().Set("Location", uris[0])
		}
		writeJSON(w, http.StatusMultipleChoices, map[string]interface{}{
			"AvailableUris": uris,
			"Error":         severalProducts,
		})
	case !s.offers(product):
		s.record(product, http.StatusForbidden)
		writeJSON(w, http.StatusForbidden, map[string]string{
			"Error": notOffered,
		})
	case s.gzipped == "" || s.notModified(r):
		s.record(product, http.StatusNotModified)
		w.WriteHeader(http.StatusNotModified)
	default:
		file, err := os.Open(s.gzipped)
		if err != nil {
			s.record(product, http.StatusInternalServerError)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()
		s.record(product, http.StatusOK)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-MD5", s.md5)
		// An example that ends before the file is sent ends only this
		// answer.
		_, _ = io.Copy(w, file)
	}
}

// offers reports whether product is on the list, ignoring case.
func (s *dataUpdateStandIn) offers(product string) bool {
	for _, offered := range s.products {
		if strings.EqualFold(offered, product) {
			return true
		}
	}
	return false
}

// notModified reports whether the request's If-Modified-Since is not earlier
// than the published date of the file served, so the file is not newer.
func (s *dataUpdateStandIn) notModified(r *http.Request) bool {
	since, err := http.ParseTime(r.Header.Get("If-Modified-Since"))
	return err == nil && !since.Before(s.published)
}

// productOf gives the value of the Product parameter of a URL, its name
// matched ignoring case, or "" when there is none or it is empty.
func productOf(u *url.URL) string {
	for name, values := range u.Query() {
		if strings.EqualFold(name, "Product") && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// withProduct gives the URL requested with Product set to product, as each
// URI the Distributor lists is.
func withProduct(r *http.Request, product string) string {
	var kept []string
	for _, pair := range strings.Split(r.URL.RawQuery, "&") {
		name, _, _ := strings.Cut(pair, "=")
		if pair != "" && !strings.EqualFold(name, "Product") {
			kept = append(kept, pair)
		}
	}
	kept = append(kept, "Product="+url.QueryEscape(product))
	return "http://" + r.Host + r.URL.Path + "?" + strings.Join(kept, "&")
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// gzipFile writes a compressed copy of the file at from to the file at to,
// and gives the lower case hexadecimal MD5 hash of the copy.
func gzipFile(t *testing.T, from, to string) string {
	t.Helper()
	in, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	hash := md5.New()
	zipped, err := gzip.NewWriterLevel(io.MultiWriter(out, hash),
		gzip.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(zipped, in); err != nil {
		t.Fatal(err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// readDate gives the date in the header of the data file at path that starts
// at offset.
func readDate(t *testing.T, path string, offset int64) time.Time {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var date [4]byte
	if _, err := file.ReadAt(date[:], offset); err != nil {
		t.Fatalf("%s has no date at %d: %v", path, offset, err)
	}
	return time.Date(int(int16(binary.LittleEndian.Uint16(date[:2]))),
		time.Month(date[2]), int(date[3]), 0, 0, 0, 0, time.UTC)
}

// writeDate sets the date in the header of the data file at path that starts
// at offset, so that a copy of a data file can stand for another.
func writeDate(t *testing.T, path string, offset int64, date time.Time) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	var bytes [4]byte
	binary.LittleEndian.PutUint16(bytes[:2], uint16(date.Year()))
	bytes[2] = byte(date.Month())
	bytes[3] = byte(date.Day())
	if _, err := file.WriteAt(bytes[:], offset); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestDataUpdateStandInAnswersAsTheDistributor checks each answer the
// stand-in gives, and that it records each request.
func TestDataUpdateStandInAnswersAsTheDistributor(t *testing.T) {
	file := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(file, make([]byte, 128), 0o644); err != nil {
		t.Fatal(err)
	}
	published := time.Date(2026, time.November, 25, 0, 0, 0, 0, time.UTC)
	writeDate(t, file, publishedOffset, published)
	if got := readDate(t, file, publishedOffset); !got.Equal(published) {
		t.Fatalf("the date read is %v, want %v", got, published)
	}
	served := newDataUpdateStandIn(t, []string{"Other", "Mine"}, file)
	notModified := newDataUpdateStandIn(t, []string{"Mine"}, "")
	get := func(address string, since time.Time) *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, address, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !since.IsZero() {
			request.Header.Set("If-Modified-Since",
				since.Format(http.TimeFormat))
		}
		client := &http.Client{CheckRedirect: func(*http.Request,
			[]*http.Request) error {
			return http.ErrUseLastResponse
		}}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { response.Body.Close() })
		return response
	}
	key := "?LicenseKeys=KEY&Type=HashV41"

	several := get(served.url()+key, time.Time{})
	var list struct{ AvailableUris []string }
	if err := json.NewDecoder(several.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	want := served.url() + key + "&Product=Other"
	if several.StatusCode != http.StatusMultipleChoices ||
		len(list.AvailableUris) != 2 || list.AvailableUris[0] != want ||
		several.Header.Get("Location") != want {
		t.Errorf("no product gave %d, %q and Location %q, want 300 and "+
			"%q first", several.StatusCode, list.AvailableUris,
			several.Header.Get("Location"), want)
	}
	if got := get(served.url()+key+"&Product=None", time.Time{}); got.StatusCode != http.StatusForbidden {
		t.Errorf("a product not offered gave %d, want 403", got.StatusCode)
	}
	file200 := get(served.url()+key+"&product=mine", published.AddDate(0, 0, -1))
	body, err := io.ReadAll(file200.Body)
	if err != nil {
		t.Fatal(err)
	}
	sum := md5.Sum(body)
	if file200.StatusCode != http.StatusOK ||
		file200.Header.Get("Content-MD5") != hex.EncodeToString(sum[:]) {
		t.Errorf("an older file gave %d with Content-MD5 %q, want 200 with "+
			"the hash of the body", file200.StatusCode,
			file200.Header.Get("Content-MD5"))
	}
	if unzipped, err := gzip.NewReader(bytes.NewReader(body)); err != nil {
		t.Errorf("the file sent is not compressed: %v", err)
	} else if content, err := io.ReadAll(unzipped); err != nil || len(content) != 128 {
		t.Errorf("the file sent unpacks to %d bytes and %v, want 128",
			len(content), err)
	}
	if got := get(served.url()+key+"&Product=Mine", published); got.StatusCode != http.StatusNotModified {
		t.Errorf("a file that is not newer gave %d, want 304", got.StatusCode)
	}
	if got := get(notModified.url()+key+"&Product=Mine", time.Time{}); got.StatusCode != http.StatusNotModified {
		t.Errorf("no file to serve gave %d, want 304", got.StatusCode)
	}

	wantRequests := []dataUpdateRequest{{"", 300}, {"None", 403},
		{"mine", 200}, {"Mine", 304}}
	got := served.received()
	if len(got) != len(wantRequests) {
		t.Fatalf("the stand-in recorded %v, want %v", got, wantRequests)
	}
	for i := range got {
		if got[i] != wantRequests[i] {
			t.Errorf("request %d was recorded as %v, want %v", i, got[i],
				wantRequests[i])
		}
	}
}
