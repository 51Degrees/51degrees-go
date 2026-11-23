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

package requestengine

import (
	"net/http"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// panicText is what panickingTransport panics with.
const panicText = "the transport panicked"

// panickingTransport panics on the data request, and passes the discovery
// requests to next.
type panickingTransport struct{ next http.RoundTripper }

func (p panickingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.HasSuffix(r.URL.Path, "/json") {
		panic(panicText)
	}
	return p.next.RoundTrip(r)
}

// A panic in the request's task fails the task, as any other failure does,
// so the program carries on, the failure is recorded once as the Cloud
// Request Engine's error, and a reader of json-response gets an error.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L178-L183
func TestAPanicInTheRequestFailsItsTask(t *testing.T) {
	_, srv := newHeldServer(t, http.StatusOK)
	client := srv.Client()
	client.Transport = panickingTransport{client.Transport}
	e, err := NewBuilder().
		SetResourceKey("test-key").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(client).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, err := core.NewPipelineBuilder().
		AddFlowElement(e).
		AddFlowElement(newFailingReader("a")).
		Build()
	if err != nil {
		t.Fatalf("build the Pipeline: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err == nil {
		t.Error("Process gave no error")
	}
	errs := fd.Errors()
	if len(errs) != 1 || errs[0].Element != core.FlowElement(e) ||
		!strings.Contains(errs[0].Err.Error(), panicText) {
		t.Errorf("the errors are %v, want one, the Cloud Request Engine's, "+
			"naming the panic", errs)
	}
	if _, err := responseOf(t, fd).Get(JSONResponseProperty); err == nil {
		t.Error("json-response gave no error after the request panicked")
	}
}
