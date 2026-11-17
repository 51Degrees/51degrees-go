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

package usagesharing

import (
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// Query string and form parameters follow the cookie rule: only a name that
// starts 51D_ is shared, whatever the configuration, so a name a site uses
// for itself, such as password, never is.
func TestOnlyParameterNamesStarting51DAreShared(t *testing.T) {
	configs := map[string]*Builder{
		"default": NewBuilder(),
		"names included": NewBuilder().
			SetIncludedQueryStringParameters("password", "other"),
		"all query string parameters": NewBuilder().
			SetShareAllQueryStringParameters(true),
		"all Evidence": NewBuilder().SetShareAllEvidence(true),
	}
	for name, b := range configs {
		e := b.Build()
		for key, want := range map[string]bool{
			"query.password": false,
			"query.other":    false,
			"query.51D_x":    true,
			"query.51d_x":    true,
		} {
			if got := e.isShared(key); got != want {
				t.Errorf("%s: isShared(%q) = %v, want %v", name, key, got,
					want)
			}
		}
		e.Close()
	}
}

// A GET request's query string shares only its 51D_ names.
func TestAQueryStringSharesOnly51DNames(t *testing.T) {
	p, e := newSharingPipeline(t, NewBuilder())
	r := httptest.NewRequest("GET",
		"/page?password=secret&51D_x=1&other=2", nil)
	fd := p.CreateFlowData()
	fd.AddEvidence(httpRequestKey, core.NewWebRequest(r))
	got := e.shareableEvidence(fd)
	want := map[string]string{"query.51d_x": "1"}
	if q := queryOf(got); !reflect.DeepEqual(q, want) {
		t.Errorf("shared the parameters %v, want %v", q, want)
	}
}

// A POST request's form body shares only its 51D_ names, whether the form
// comes in the request or as query Evidence, as the web integration adds it.
func TestAFormBodySharesOnly51DNames(t *testing.T) {
	p, e := newSharingPipeline(t, NewBuilder())
	form := url.Values{"password": {"secret"}, "51D_y": {"2"},
		"other": {"3"}}
	r := httptest.NewRequest("POST", "/page",
		strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	fd := p.CreateFlowData()
	for k, v := range form {
		fd.AddEvidence("query."+k, v[0])
	}
	fd.AddEvidence(httpRequestKey, core.NewWebRequest(r))
	got := e.shareableEvidence(fd)
	want := map[string]string{"query.51d_y": "2"}
	if q := queryOf(got); !reflect.DeepEqual(q, want) {
		t.Errorf("shared the parameters %v, want %v", q, want)
	}
}

// A parameter whose name does not start 51D_ is still shared when it is
// Evidence the Pipeline uses, being a key an element in the Pipeline accepts,
// from a query string and from a form body alike.
func TestAParameterThePipelineUsesIsShared(t *testing.T) {
	e := NewBuilder().SetSharePercentage(0).
		SetShareUsageURL("http://127.0.0.1:1/").Build()
	engine := &userAgentEngine{BaseElement: core.NewBaseElement("device",
		core.NewWhitelistFilter("header.user-agent", "query.user-agent"), nil)}
	p, err := core.NewPipelineBuilder().AddFlowElement(engine).
		AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	for key, want := range map[string]bool{
		"query.user-agent": true,
		"query.User-Agent": true,
		"query.51D_x":      true,
		"query.password":   false,
	} {
		if got := e.isShared(key); got != want {
			t.Errorf("isShared(%q) = %v, want %v", key, got, want)
		}
	}

	get := httptest.NewRequest("GET",
		"/page?user-agent=ua&password=secret&51D_x=1", nil)
	fd := p.CreateFlowData()
	fd.AddEvidence(httpRequestKey, core.NewWebRequest(get))
	want := map[string]string{"query.user-agent": "ua", "query.51d_x": "1"}
	if q := queryOf(e.shareableEvidence(fd)); !reflect.DeepEqual(q, want) {
		t.Errorf("from a query string, shared the parameters %v, want %v",
			q, want)
	}

	form := url.Values{"user-agent": {"ua"}, "password": {"secret"},
		"51D_y": {"2"}}
	post := httptest.NewRequest("POST", "/page",
		strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := post.ParseForm(); err != nil {
		t.Fatal(err)
	}
	fd = p.CreateFlowData()
	for k, v := range form {
		fd.AddEvidence("query."+k, v[0])
	}
	fd.AddEvidence(httpRequestKey, core.NewWebRequest(post))
	want = map[string]string{"query.user-agent": "ua", "query.51d_y": "2"}
	if q := queryOf(e.shareableEvidence(fd)); !reflect.DeepEqual(q, want) {
		t.Errorf("from a form body, shared the parameters %v, want %v",
			q, want)
	}
}

// queryOf gives the query and form parameters among shared values.
func queryOf(shared map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range shared {
		if strings.HasPrefix(k, "query.") {
			out[k] = v
		}
	}
	return out
}
