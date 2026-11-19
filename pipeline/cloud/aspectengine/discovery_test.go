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

package aspectengine

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/cloud/requestengine"
	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// After a transient failure when the Cloud Request Engine is built, the
// Property metadata is absent until a request fetches it, and is present
// from then on, because AspectProperties derives it on each call rather than
// keeping the first answer.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-aspect-engine.md#start-up-activity
func TestAspectPropertiesFollowRetriedDiscovery(t *testing.T) {
	var healthy atomic.Bool
	cloud := fakeCloud()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !healthy.Load() && r.URL.Path != "/json" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		cloud.ServeHTTP(w, r)
	}))
	defer srv.Close()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	re, err := requestengine.NewBuilder().
		SetResourceKey("k").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		SetLogger(quiet).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, _ := core.NewPipelineBuilder().
		SetLogger(quiet).
		SetSuppressProcessExceptions(true).
		AddFlowElement(re).
		Build()
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	_ = fd.Process()
	if props, err := AspectProperties(fd, nil, "device"); err == nil {
		t.Fatalf("AspectProperties gives %v while the cloud fails, want an "+
			"error", props)
	}

	healthy.Store(true)
	fd = p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	props, err := AspectProperties(fd, nil, "device")
	if err != nil || len(props) != 2 {
		t.Fatalf("AspectProperties gives %v, %v, want two Properties", props, err)
	}
}
