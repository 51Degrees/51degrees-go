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

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
)

const (
	testResource = "test-resource-key"
	testLicense  = "test-license-key"
)

// fakeCloud stands in for the cloud. It answers the four calls the example
// makes and keeps what it was sent.
type fakeCloud struct {
	server       *httptest.Server
	properties   []string
	passiveBody  string
	redeemStatus int
	redeemBody   string

	mu       sync.Mutex
	evidence url.Values
	redeemed url.Values
	// keysFailures is how many more times evidencekeys answers 503.
	keysFailures int
}

func newFakeCloud(t *testing.T, properties ...string) *fakeCloud {
	cloud := &fakeCloud{
		properties:   properties,
		passiveBody:  `{"derived":{"humanconfidencesource":"Passive","humanconfidence":"Medium"}}`,
		redeemStatus: http.StatusOK,
		redeemBody:   rcAnswers[0].body,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		cloud.mu.Lock()
		fail := cloud.keysFailures > 0
		if fail {
			cloud.keysFailures--
		}
		cloud.mu.Unlock()
		if fail {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, `{"error":"unavailable"}`)
			return
		}
		json.NewEncoder(w).Encode([]string{
			"header.user-agent", "header.Sec-CH-UA", "query.fod", "cookie.x"})
	})
	mux.HandleFunc("/api/v4/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("resource") != testResource {
			http.Error(w, "unknown key", http.StatusBadRequest)
			return
		}
		list := make([]map[string]string, 0)
		for _, name := range cloud.properties {
			list = append(list, map[string]string{"Name": name})
		}
		json.NewEncoder(w).Encode(map[string]any{"Products": map[string]any{
			"device": map[string]any{"Properties": list}}})
	})
	mux.HandleFunc("/api/v4/"+testResource+".json", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		cloud.mu.Lock()
		cloud.evidence = r.PostForm
		cloud.mu.Unlock()
		io.WriteString(w, cloud.passiveBody)
	})
	mux.HandleFunc("/api/v4/hc/redeem", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		cloud.mu.Lock()
		cloud.redeemed = r.PostForm
		cloud.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(cloud.redeemStatus)
		io.WriteString(w, cloud.redeemBody)
	})
	cloud.server = httptest.NewServer(mux)
	t.Cleanup(cloud.server.Close)
	return cloud
}

func (c *fakeCloud) config() config {
	return config{
		API:      c.server.URL + "/api/v4/",
		Resource: testResource,
		License:  testLicense,
	}
}

// startExample starts the example's handler against the fake cloud.
func startExample(t *testing.T, cloud *fakeCloud) *httptest.Server {
	t.Helper()
	handler := newExample(cloud.config(), cloud.server.Client(), io.Discard)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestPageHasKeysAndNoStore(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence", "HumanConfidenceSource")
	example := startExample(t, cloud)
	response, err := http.Get(example.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, response)
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control is %q", response.Header.Get("Cache-Control"))
	}
	if !strings.Contains(body, `"`+testResource+`"`) ||
		!strings.Contains(body, `"`+cloud.config().API+`"`) {
		t.Error("the page lacks the Resource Key or the API base")
	}
	if strings.Contains(body, "__RESOURCE__") || strings.Contains(body, "__API__") {
		t.Error("the page still has a placeholder")
	}
	if strings.Contains(body, testLicense) {
		t.Error("the page holds the License Key")
	}
	if strings.Contains(body, "contact-us") {
		t.Error("the page says the key lacks HumanConfidence when it has it")
	}
}

func TestPassiveSendsOnlyCloudHeaders(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	example := startExample(t, cloud)
	request, _ := http.NewRequest(http.MethodGet, example.URL+"/passive", nil)
	request.Header.Set("User-Agent", "test agent")
	request.Header.Set("Sec-CH-UA", `"Test";v="1"`)
	request.Header.Set("Authorization", "Bearer not-a-real-token")
	request.Header.Set("X-Other", "other")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	var answer map[string]any
	if err := json.Unmarshal([]byte(readBody(t, response)), &answer); err != nil {
		t.Fatal(err)
	}
	if answer["source"] != "Passive" || answer["humanConfidence"] != "Medium" {
		t.Errorf("the passive answer is %v", answer)
	}
	cloud.mu.Lock()
	defer cloud.mu.Unlock()
	if cloud.evidence.Get("user-agent") != "test agent" ||
		cloud.evidence.Get("sec-ch-ua") != `"Test";v="1"` {
		t.Errorf("the cloud's headers were not passed on, got %v", cloud.evidence)
	}
	for name := range cloud.evidence {
		if name != "user-agent" && name != "sec-ch-ua" && name != "client-ip" {
			t.Errorf("%s was passed on but is not in the cloud's list", name)
		}
	}
	if cloud.evidence.Get("client-ip") != "127.0.0.1" {
		t.Errorf("client-ip is %q", cloud.evidence.Get("client-ip"))
	}
}

// passiveAnswer calls /passive and returns the decoded answer.
func passiveAnswer(t *testing.T, example *httptest.Server) map[string]any {
	t.Helper()
	response, err := http.Get(example.URL + "/passive")
	if err != nil {
		t.Fatal(err)
	}
	var answer map[string]any
	if err := json.Unmarshal([]byte(readBody(t, response)), &answer); err != nil {
		t.Fatal(err)
	}
	return answer
}

// The answer for a key without the properties has a null source and a null
// level, and the example reports neither rather than making one up.
func TestPassiveWithoutTheSourceReportsNone(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	cloud.passiveBody = `{"derived":{` +
		`"humanconfidence":null,"humanconfidencenullreason":"Not in this key.",` +
		`"humanconfidencesource":null,` +
		`"humanconfidencesourcenullreason":"Not in this key."}}`
	example := startExample(t, cloud)
	answer := passiveAnswer(t, example)
	if _, failed := answer["error"]; failed {
		t.Fatalf("the passive answer is an error, %v", answer)
	}
	if answer["source"] != nil || answer["humanConfidence"] != nil {
		t.Errorf("the passive answer is %v, want no source and no level", answer)
	}
}

func TestPassiveAsksForTheHeadersAgainAfterAFailure(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	cloud.keysFailures = 1
	example := startExample(t, cloud)
	first := passiveAnswer(t, example)
	if message, _ := first["error"].(string); !strings.Contains(message, "HTTP 503") {
		t.Errorf("the first answer is %v, want an error with HTTP 503", first)
	}
	second := passiveAnswer(t, example)
	if second["source"] != "Passive" || second["humanConfidence"] != "Medium" {
		t.Errorf("the second answer is %v", second)
	}
}

// rcAnswers are the answers hc/redeem gives, one for each status. Every one
// is 200 but unconfirmed, which is 503 to invite a retry. humanConfidence is
// in every answer, and is Medium unless the status is success. The times
// come with success and expired only.
var rcAnswers = []struct {
	status int
	body   string
	shown  shown
}{
	{http.StatusOK, `{"humanConfidence":"High","status":"success",` +
		`"verifiedAt":"2026-11-28T09:00:00Z","secondsSinceVerified":2}`,
		shown{"success", cell{"redeemed 2 s after the check", "pass"}, cell{"High", "pass"}}},
	{http.StatusOK, `{"humanConfidence":"Medium","status":"expired",` +
		`"verifiedAt":"2026-11-28T09:00:00Z","secondsSinceVerified":11}`,
		shown{"expired", cell{"expired", "fail"}, cell{"no level", "none"}}},
	{http.StatusOK, `{"humanConfidence":"Medium","status":"replayed"}`,
		shown{"replayed", cell{"replayed", "fail"}, cell{"no level", "none"}}},
	{http.StatusOK, `{"humanConfidence":"Medium","status":"unreadable"}`,
		shown{"unreadable", cell{"unreadable", "fail"}, cell{"no level", "none"}}},
	{http.StatusServiceUnavailable, `{"humanConfidence":"Medium","status":"unconfirmed"}`,
		shown{"unconfirmed", cell{"unconfirmed", "fail"}, cell{"no level", "none"}}},
}

// redeemAnswer posts a sealed result to /redeem and decodes what comes back.
func redeemAnswer(t *testing.T, example *httptest.Server) (int, shown) {
	t.Helper()
	response, err := http.PostForm(example.URL+"/redeem",
		url.Values{"sealedResult": {"sealed-value"}})
	if err != nil {
		t.Fatal(err)
	}
	var got shown
	if err := json.Unmarshal([]byte(readBody(t, response)), &got); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, got
}

func TestRedeemPassesTheAnswerOn(t *testing.T) {
	for _, rc := range rcAnswers {
		cloud := newFakeCloud(t, "HumanConfidence")
		cloud.redeemStatus = rc.status
		cloud.redeemBody = rc.body
		example := startExample(t, cloud)
		code, got := redeemAnswer(t, example)
		if code != rc.status || got != rc.shown {
			t.Errorf("redeem of %s gave %d %+v, want %d %+v",
				rc.body, code, got, rc.status, rc.shown)
		}
		cloud.mu.Lock()
		if cloud.redeemed.Get("sealedResult") != "sealed-value" ||
			cloud.redeemed.Get("license") != testLicense {
			t.Error("the cloud was not sent the sealed result and the License Key")
		}
		cloud.mu.Unlock()
	}
}

// Success is decided from status alone. A success that gives a reason in
// place of a level shows the reason, not a failure.
func TestRedeemSuccessWithNoLevelShowsTheReason(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	cloud.redeemBody = `{"status":"success","secondsSinceVerified":1,` +
		`"humanConfidenceNullReason":"Not in this key."}`
	example := startExample(t, cloud)
	_, got := redeemAnswer(t, example)
	want := shown{"success", cell{"redeemed 1 s after the check", "pass"},
		cell{"Not in this key.", "none"}}
	if got != want {
		t.Errorf("the page is told %+v, want %+v", got, want)
	}
}

func TestRedeemShown(t *testing.T) {
	for _, test := range []struct {
		answer string
		want   shown
	}{
		{`{"status":"success","humanConfidence":"Low"}`,
			shown{"success", cell{"redeemed", "pass"}, cell{"Low", "fail"}}},
		{`{"status":"success","humanConfidence":"Medium","secondsSinceVerified":3}`,
			shown{"success", cell{"redeemed 3 s after the check", "pass"}, cell{"Medium", "part"}}},
		{`{"status":"success"}`,
			shown{"success", cell{"redeemed", "pass"}, cell{"no level and no reason given", "none"}}},
		{`{"humanConfidence":"High"}`,
			shown{"no answer", cell{"no answer", "fail"}, cell{"no level", "none"}}},
		{`{"error":"broken"}`,
			shown{"broken", cell{"broken", "fail"}, cell{"no level", "none"}}},
	} {
		var answer map[string]any
		if err := json.Unmarshal([]byte(test.answer), &answer); err != nil {
			t.Fatal(err)
		}
		if got := redeemShown(answer); got != test.want {
			t.Errorf("%s shows %+v, want %+v", test.answer, got, test.want)
		}
	}
}

// hc/redeem reads the sealed result from sealedResult and from no other
// field, so the redeem sends only that and the License Key.
func TestRedeemSendsOnlySealedResult(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	example := startExample(t, cloud)
	response, err := http.PostForm(example.URL+"/redeem",
		url.Values{"sealedResult": {"sealed-value"}})
	if err != nil {
		t.Fatal(err)
	}
	readBody(t, response)
	cloud.mu.Lock()
	defer cloud.mu.Unlock()
	for name := range cloud.redeemed {
		if name != "sealedResult" && name != "license" {
			t.Errorf("the cloud was sent %s, which hc/redeem doesn't read", name)
		}
	}
}

func TestRedeemNotFoundSaysServiceDoesNotRedeem(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	cloud.redeemStatus = http.StatusNotFound
	cloud.redeemBody = "not found"
	example := startExample(t, cloud)
	response, err := http.PostForm(example.URL+"/redeem",
		url.Values{"sealedResult": {"sealed-value"}})
	if err != nil {
		t.Fatal(err)
	}
	var answer shown
	if err := json.Unmarshal([]byte(readBody(t, response)), &answer); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(answer.Detail.Text, "doesn't redeem Human Confidence") {
		t.Errorf("the error is %q", answer.Detail.Text)
	}
}

func TestMissingPropertyLinksToContactUs(t *testing.T) {
	cloud := newFakeCloud(t, "BrowserName")
	var console strings.Builder
	handler := newExample(cloud.config(), cloud.server.Client(), &console)
	example := httptest.NewServer(handler)
	defer example.Close()
	response, err := http.Get(example.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, response)
	if !strings.Contains(body, contactUs) {
		t.Error("the page does not link to contact us")
	}
	if !strings.Contains(console.String(), contactUs) {
		t.Error("the console does not link to contact us")
	}
	if !strings.Contains(contactUs, "utm_campaign=51degrees-go") ||
		!strings.Contains(contactUs, "utm_content=human-confidence") {
		t.Errorf("the contact link is not tagged, %s", contactUs)
	}
}

// taggedLink is the form every link to the website takes. It has the five
// parameters in order, with the repository as the campaign and the example
// as the content.
var taggedLink = regexp.MustCompile(`^https://51degrees\.com/[^?]*` +
	`\?utm_source=(github|code)&utm_medium=(readme|example)` +
	`&utm_campaign=51degrees-go&utm_content=human-confidence` +
	`&utm_term=[a-z0-9][a-z0-9._-]*$`)

// websiteLink finds the links to the website in a file.
var websiteLink = regexp.MustCompile(`https://51degrees\.com[^\s"')>]*`)

func TestLinksAreTagged(t *testing.T) {
	for _, name := range []string{"README.md", "main.go", "page.html"} {
		text, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, link := range websiteLink.FindAllString(string(text), -1) {
			if !taggedLink.MatchString(link) {
				t.Errorf("%s has a link without the five tags, %s", name, link)
			}
		}
	}
}

func TestDefaultPort(t *testing.T) {
	none := func(string) string { return "" }
	if port := listenPort(none); port != "5151" {
		t.Errorf("the default port is %s", port)
	}
	set := func(name string) string {
		if name == "PORT" {
			return "6000"
		}
		return ""
	}
	if port := listenPort(set); port != "6000" {
		t.Errorf("PORT gave %s", port)
	}
}

func TestEndpoint(t *testing.T) {
	cfg := loadConfig(func(string) string { return "" })
	if cfg.API != "https://cloud.51degrees.com/api/v4/" {
		t.Errorf("the default endpoint is %s", cfg.API)
	}
	cfg = loadConfig(func(name string) string {
		if name == "_51DEGREES_CLOUD_ENDPOINT" {
			return "https://cloud.example.com/api/v4"
		}
		return ""
	})
	if cfg.API != "https://cloud.example.com/api/v4/" {
		t.Errorf("the endpoint is %s", cfg.API)
	}
}
