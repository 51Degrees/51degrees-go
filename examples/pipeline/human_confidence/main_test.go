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

	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/derivedproperty"
	"github.com/51Degrees/51degrees-go/pipeline/v4/engines"
)

const (
	testResource = "test-resource-key"
	testLicense  = "test-license-key"
)

// fakeCloud stands in for the cloud. It answers the calls the Cloud Request
// Engine makes and keeps what it was sent. Its JSON call answers a sealed
// result as the cloud does: 200 for every status, with HumanConfidenceSource
// Active, and HumanConfidence the sealed Value on success and Medium on any
// other status. A sealed result named sealed-<Value> redeems once and is
// replayed after that. sealed-expired and sealed-unconfirmed give those
// statuses, and anything else is unreadable.
type fakeCloud struct {
	server      *httptest.Server
	properties  []string
	passiveBody string
	// jsonStatus and jsonBody, where set, replace the JSON call's answer.
	jsonStatus int
	jsonBody   string

	mu       sync.Mutex
	evidence url.Values
	redeemed map[string]bool
	// hcRedeemCalls counts calls to hc/redeem, which the example never makes.
	hcRedeemCalls int
	// keysFailures is how many more times evidencekeys answers 503.
	keysFailures int
}

func newFakeCloud(t *testing.T, properties ...string) *fakeCloud {
	cloud := &fakeCloud{
		properties:  properties,
		passiveBody: `{"derived":{"humanconfidencesource":"Passive","humanconfidence":"Medium"}}`,
		redeemed:    map[string]bool{},
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
		if r.FormValue("resource") != testResource {
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
	mux.HandleFunc("/api/v4/json", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		cloud.mu.Lock()
		cloud.evidence = r.PostForm
		status, body := cloud.jsonStatus, cloud.jsonBody
		cloud.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
			io.WriteString(w, body)
			return
		}
		sealed := r.PostForm.Get("humanconfidence")
		if sealed == "" {
			io.WriteString(w, cloud.passiveBody)
			return
		}
		io.WriteString(w, `{"derived":{"humanconfidencesource":"Active",`+
			`"humanconfidence":"`+cloud.redeem(sealed, r.PostForm.Get("license"))+`"}}`)
	})
	mux.HandleFunc("/api/v4/hc/redeem", func(w http.ResponseWriter, r *http.Request) {
		cloud.mu.Lock()
		cloud.hcRedeemCalls++
		cloud.mu.Unlock()
		http.Error(w, "not expected", http.StatusTeapot)
	})
	cloud.server = httptest.NewServer(mux)
	t.Cleanup(cloud.server.Close)
	return cloud
}

// redeem gives the Value the cloud gives for sealed, which needs the
// License Key.
func (c *fakeCloud) redeem(sealed, license string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	level, ok := strings.CutPrefix(sealed, "sealed-")
	if !ok || license != testLicense {
		return "Medium" // unreadable
	}
	switch level {
	case "High", "Medium", "Low":
		if c.redeemed[sealed] {
			return "Medium" // replayed
		}
		c.redeemed[sealed] = true
		return level
	}
	return "Medium" // expired, unconfirmed
}

func (c *fakeCloud) config() config {
	return config{
		API:      c.server.URL + "/api/v4/",
		Resource: testResource,
		License:  testLicense,
	}
}

// startExample starts the example against the fake cloud.
func startExample(t *testing.T, cloud *fakeCloud) *httptest.Server {
	t.Helper()
	return startExampleWith(t, cloud, cloud.config())
}

func startExampleWith(t *testing.T, cloud *fakeCloud, cfg config) *httptest.Server {
	t.Helper()
	e, err := newExample(cfg, cloud.server.Client(), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close() })
	server := httptest.NewServer(e)
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

// passiveAnswer calls /passive and returns the decoded answer.
func passiveAnswer(t *testing.T, example *httptest.Server) reading {
	t.Helper()
	response, err := http.Get(example.URL + "/passive")
	if err != nil {
		t.Fatal(err)
	}
	var answer reading
	if err := json.Unmarshal([]byte(readBody(t, response)), &answer); err != nil {
		t.Fatal(err)
	}
	return answer
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
	var answer reading
	if err := json.Unmarshal([]byte(readBody(t, response)), &answer); err != nil {
		t.Fatal(err)
	}
	if answer.Source.Text != "Passive" || answer.HumanConfidence != (cell{"Medium", "part"}) {
		t.Errorf("the passive answer is %+v", answer)
	}
	cloud.mu.Lock()
	defer cloud.mu.Unlock()
	if cloud.evidence.Get("user-agent") != "test agent" ||
		cloud.evidence.Get("sec-ch-ua") != `"Test";v="1"` {
		t.Errorf("the cloud's headers were not passed on, got %v", cloud.evidence)
	}
	for name := range cloud.evidence {
		switch name {
		case "user-agent", "sec-ch-ua", "client-ip", "resource", "license":
		default:
			t.Errorf("%s was passed on but is not in the cloud's list", name)
		}
	}
	if cloud.evidence.Get("client-ip") != "127.0.0.1" {
		t.Errorf("client-ip is %q", cloud.evidence.Get("client-ip"))
	}
}

// The answer for a key without the properties has a null source and a null
// level, each with a reason, and the page is given the reasons, as for any
// other Property with no value, rather than a value made up.
func TestPassiveWithoutTheSourceShowsTheReason(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	cloud.passiveBody = `{"derived":{` +
		`"humanconfidence":null,"humanconfidencenullreason":"Not in this key.",` +
		`"humanconfidencesource":null,` +
		`"humanconfidencesourcenullreason":"Nor this."}}`
	example := startExample(t, cloud)
	want := reading{cell{"Nor this.", "none"}, cell{"Not in this key.", "none"}}
	if answer := passiveAnswer(t, example); answer != want {
		t.Errorf("the passive answer is %+v, want %+v", answer, want)
	}
}

// The Evidence keys are asked for when the Pipeline is built, and again by
// the next request after a failure. While the cloud still fails, the two
// Properties have no value, and the page shows the reason the Pipeline gives.
func TestPassiveAsksForTheHeadersAgainAfterAFailure(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	cloud.keysFailures = 2
	example := startExample(t, cloud)
	first := passiveAnswer(t, example)
	if !strings.Contains(first.HumanConfidence.Text, "cloud request failed") {
		t.Errorf("the first answer is %+v, want the reason a failed request gives", first)
	}
	second := passiveAnswer(t, example)
	if second.Source.Text != "Passive" || second.HumanConfidence.Text != "Medium" {
		t.Errorf("the second answer is %+v", second)
	}
}

// redeemAnswer posts a sealed result to /redeem as the page does, and
// decodes what comes back.
func redeemAnswer(t *testing.T, example *httptest.Server, sealed string) reading {
	t.Helper()
	response, err := http.PostForm(example.URL+"/redeem",
		url.Values{"humanconfidence": {sealed}})
	if err != nil {
		t.Fatal(err)
	}
	var got reading
	if err := json.Unmarshal([]byte(readBody(t, response)), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

var active = cell{"Active", "pass"}

// The cloud answers every status with 200 through the JSON call, and with
// Medium for every one but success. The page is shown the two Properties.
func TestRedeemShowsTheTwoProperties(t *testing.T) {
	for _, test := range []struct {
		sealed string
		want   cell
	}{
		{"sealed-High", cell{"High", "pass"}},
		{"sealed-Low", cell{"Low", "fail"}},
		{"sealed-Medium", cell{"Medium", "part"}},
		{"sealed-expired", cell{"Medium", "part"}},
		{"sealed-unconfirmed", cell{"Medium", "part"}},
		{"not a sealed result", cell{"Medium", "part"}},
	} {
		cloud := newFakeCloud(t, "HumanConfidence")
		example := startExample(t, cloud)
		want := reading{active, test.want}
		if got := redeemAnswer(t, example, test.sealed); got != want {
			t.Errorf("%s shows %+v, want %+v", test.sealed, got, want)
		}
	}
}

// The second time a sealed result is sent it is replayed, so the Pipeline
// gives Medium.
func TestRedeemTwiceGivesMedium(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	example := startExample(t, cloud)
	if got := redeemAnswer(t, example, "sealed-High"); got != (reading{active, cell{"High", "pass"}}) {
		t.Errorf("the first time shows %+v", got)
	}
	if got := redeemAnswer(t, example, "sealed-High"); got != (reading{active, cell{"Medium", "part"}}) {
		t.Errorf("the second time shows %+v", got)
	}
}

// The example never redeems a sealed result itself. It hands it to the
// Pipeline as Evidence, which sends it in the JSON call with the License
// Key, and nothing calls hc/redeem. The page loads the cloud's script
// itself, so the Pipeline keeps no transaction for it, and the sealed
// result, made with none, goes with none.
func TestRedeemIsThePipelines(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	example := startExample(t, cloud)
	redeemAnswer(t, example, "sealed-High")
	cloud.mu.Lock()
	defer cloud.mu.Unlock()
	if cloud.hcRedeemCalls != 0 {
		t.Errorf("hc/redeem was called %d times", cloud.hcRedeemCalls)
	}
	if cloud.evidence.Get("humanconfidence") != "sealed-High" ||
		cloud.evidence.Get("license") != testLicense {
		t.Errorf("the JSON call did not carry the sealed result and the License Key, %v",
			cloud.evidence)
	}
	if cloud.evidence.Has("transaction") {
		t.Errorf("the JSON call carried transaction %q", cloud.evidence.Get("transaction"))
	}
}

func TestRedeemWithoutALicenseKeyGivesMedium(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	cfg := cloud.config()
	cfg.License = ""
	example := startExampleWith(t, cloud, cfg)
	if got := redeemAnswer(t, example, "sealed-High"); got != (reading{active, cell{"Medium", "part"}}) {
		t.Errorf("redeeming with no License Key shows %+v", got)
	}
}

// A JSON call that fails still gives Medium and Active, as the Pipeline does
// for any sealed result it couldn't redeem.
func TestRedeemWhenTheCloudFailsGivesMedium(t *testing.T) {
	cloud := newFakeCloud(t, "HumanConfidence")
	cloud.jsonStatus, cloud.jsonBody = http.StatusInternalServerError, `{"error":"broken"}`
	example := startExample(t, cloud)
	if got := redeemAnswer(t, example, "sealed-High"); got != (reading{active, cell{"Medium", "part"}}) {
		t.Errorf("a failed call shows %+v", got)
	}
}

// Each cell shows the Property's value, or the reason it has none.
func TestShownProperty(t *testing.T) {
	data := derivedproperty.NewData()
	data.Set("HumanConfidence", engines.NewAspectPropertyValue("Low"))
	data.Set("HumanConfidenceSource", engines.NoValue[string]("Not in this key."))
	if got := shownProperty(data, "HumanConfidence", ""); got != (cell{"Low", "fail"}) {
		t.Errorf("a value shows %+v", got)
	}
	if got := shownProperty(data, "HumanConfidenceSource", ""); got != (cell{"Not in this key.", "none"}) {
		t.Errorf("no value shows %+v", got)
	}
	empty := derivedproperty.NewData()
	if got := shownProperty(empty, "HumanConfidence", ""); got != (cell{"not returned", "none"}) {
		t.Errorf("a missing Property shows %+v", got)
	}
	if got := shownProperty(empty, "HumanConfidence", "HTTP 500"); got != (cell{"HTTP 500", "fail"}) {
		t.Errorf("a missing Property after a failure shows %+v", got)
	}
	data.Set("HumanConfidenceSource", engines.NewAspectPropertyValue("Passive"))
	if got := shownProperty(data, "HumanConfidenceSource", ""); got != (cell{"Passive", "none"}) {
		t.Errorf("Passive shows %+v", got)
	}
}

func TestMissingPropertyLinksToContactUs(t *testing.T) {
	cloud := newFakeCloud(t, "BrowserName")
	var console strings.Builder
	e, err := newExample(cloud.config(), cloud.server.Client(), &console)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	example := httptest.NewServer(e)
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
