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

// These tests follow the start-up rules of the specification.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#revised-design-eager-discovery-at-build-time-transient-failure-fallback

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// discoveryKey is the Resource Key the discovery tests use, which no error
// may contain.
const discoveryKey = "discovery-resource-key"

// discoveryKeys are the Evidence keys the stand-in cloud accepts, sorted.
var discoveryKeys = []string{
	"header.sec-ch-ua-platform", "header.user-agent", "query.user-agent",
}

// discoveryCloud is a stand-in cloud service whose discovery endpoints can
// be made to fail, counting the requests each endpoint receives.
type discoveryCloud struct {
	// propertiesStatus and evidenceStatus are the status each discovery
	// endpoint answers with, where 0 means a good answer.
	propertiesStatus atomic.Int64
	evidenceStatus   atomic.Int64
	properties       atomic.Int64
	evidence         atomic.Int64
	json             atomic.Int64
}

func (c *discoveryCloud) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		c.properties.Add(1)
		if status := c.propertiesStatus.Load(); status != 0 {
			w.WriteHeader(int(status))
			io.WriteString(w, `{"errors":["refused"]}`)
			return
		}
		io.WriteString(w, `{"Products":{"device":{"DataTier":"CloudV4",`+
			`"Properties":[{"Name":"ismobile","Type":"Bool","Category":"Device"}]}}}`)
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		c.evidence.Add(1)
		if status := c.evidenceStatus.Load(); status != 0 {
			w.WriteHeader(int(status))
			io.WriteString(w, `{"errors":["refused"]}`)
			return
		}
		io.WriteString(w, `["header.user-agent","query.user-agent",`+
			`"header.sec-ch-ua-platform"]`)
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		c.json.Add(1)
		io.WriteString(w, `{"device":{"ismobile":true}}`)
	})
	return mux
}

// counts returns the requests to accessibleproperties, evidencekeys and
// json, in that order.
func (c *discoveryCloud) counts() [3]int64 {
	return [3]int64{c.properties.Load(), c.evidence.Load(), c.json.Load()}
}

// serve starts the stand-in cloud for the test.
func (c *discoveryCloud) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(c.handler())
	t.Cleanup(srv.Close)
	return srv
}

// logRecorder keeps every log record made through it.
type logRecorder struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *logRecorder) Enabled(context.Context, slog.Level) bool { return true }

func (h *logRecorder) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *logRecorder) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *logRecorder) WithGroup(string) slog.Handler      { return h }

func (h *logRecorder) all() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]slog.Record(nil), h.records...)
}

// discoveryBuilder is a builder for the stand-in cloud at srv, logging to
// log.
func discoveryBuilder(srv *httptest.Server, log *logRecorder) *Builder {
	return NewBuilder().
		SetResourceKey(discoveryKey).
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		SetLogger(slog.New(log))
}

// pipelineOf builds a Pipeline holding e alone.
func pipelineOf(t *testing.T, e *CloudRequestEngine, suppress bool) core.Pipeline {
	t.Helper()
	p, err := core.NewPipelineBuilder().
		SetSuppressProcessExceptions(suppress).
		AddFlowElement(e).
		Build()
	if err != nil {
		t.Fatalf("build the Pipeline: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// process runs one request through p.
func process(p core.Pipeline) (core.FlowData, error) {
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	return fd, fd.Process()
}

// wantCounts fails the test unless the stand-in cloud has received the
// given requests to accessibleproperties, evidencekeys and json.
func wantCounts(t *testing.T, c *discoveryCloud, when string, want [3]int64) {
	t.Helper()
	if got := c.counts(); got != want {
		t.Fatalf("%s: requests to accessibleproperties, evidencekeys and "+
			"json are %v, want %v", when, got, want)
	}
}

// wantKeys fails the test unless filter lists exactly want.
func wantKeys(t *testing.T, filter core.EvidenceKeyFilter, want []string) {
	t.Helper()
	lister, ok := filter.(core.KeyLister)
	if !ok {
		t.Fatalf("the filter %T cannot list its keys", filter)
	}
	got := lister.Keys()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the filter lists %v, want %v", got, want)
	}
}

// A healthy cloud service gives both discovery results when the engine is
// built, so the engine and a Pipeline holding it advertise the accepted
// Evidence and Properties before any request, and nothing is fetched again.
func TestBuildDiscoversPropertiesAndEvidenceKeys(t *testing.T) {
	cloud := &discoveryCloud{}
	log := &logRecorder{}
	e, err := discoveryBuilder(cloud.serve(t), log).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wantCounts(t, cloud, "after the build", [3]int64{1, 1, 0})

	wantKeys(t, e.EvidenceKeyFilter(), discoveryKeys)
	if e.EvidenceKeyFilter().Include("query.extra") {
		t.Error("the engine accepts query.extra, which the cloud does not")
	}
	p := pipelineOf(t, e, false)
	if !p.EvidenceKeyFilter().Include("header.sec-ch-ua-platform") {
		t.Error("the Pipeline does not accept the cloud's Evidence keys")
	}
	products, err := e.PublicProperties()
	if err != nil || len(products["device"].Properties) != 1 {
		t.Fatalf("PublicProperties gives %v, %v, want the device product",
			products, err)
	}
	wantCounts(t, cloud, "after reading the Properties", [3]int64{1, 1, 0})

	for i := 0; i < 2; i++ {
		if _, err := process(p); err != nil {
			t.Fatalf("process: %v", err)
		}
	}
	wantCounts(t, cloud, "after two requests", [3]int64{1, 1, 2})
	if records := log.all(); len(records) != 0 {
		t.Errorf("a healthy build and two requests logged %d records, "+
			"want none, the first being %q", len(records), records[0].Message)
	}
}

// A 4xx answer to either discovery request, such as an invalid Resource Key,
// fails the build with a configuration error that carries the cloud's
// status, after both requests have been made.
func TestBuildFailsOnDefinitiveDiscoveryError(t *testing.T) {
	cases := []struct {
		name                 string
		properties, evidence int64
		want                 int
	}{
		{"accessibleproperties 400", 400, 0, 400},
		{"accessibleproperties 401", 401, 0, 401},
		{"accessibleproperties 403", 403, 0, 403},
		{"evidencekeys 404", 0, 404, 404},
		{"evidencekeys 429", 0, 429, 429},
		{"accessibleproperties 401, evidencekeys 503", 401, 503, 401},
		{"accessibleproperties 503, evidencekeys 403", 503, 403, 403},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cloud := &discoveryCloud{}
			cloud.propertiesStatus.Store(c.properties)
			cloud.evidenceStatus.Store(c.evidence)
			e, err := discoveryBuilder(cloud.serve(t), &logRecorder{}).Build()
			if err == nil || e != nil {
				t.Fatalf("build gives an engine: %v, and the error %v, want an "+
					"error and no engine", e != nil, err)
			}
			var config *core.ConfigurationError
			if !errors.As(err, &config) {
				t.Errorf("the error %T is not a *core.ConfigurationError", err)
			}
			var cloudErr *CloudRequestError
			if !errors.As(err, &cloudErr) || cloudErr.StatusCode != c.want {
				t.Errorf("the error %v does not carry the status %d", err, c.want)
			}
			if strings.Contains(err.Error(), discoveryKey) {
				t.Errorf("the error %q gives away the Resource Key", err)
			}
			wantCounts(t, cloud, "after the build", [3]int64{1, 1, 0})
		})
	}
}

// blockingCloud answers every request only when the client gives up.
func blockingCloud(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	return srv
}

// deadEndpoint is the address of a server that has stopped, so a connection
// to it is refused.
func deadEndpoint() string {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL + "/"
}

// A transient failure, being the service unreachable, a timeout, a 5xx
// answer or a body that cannot be read, does not fail the build. The build
// logs one warning and completes, and the engine advertises no Evidence
// keys that it could not fetch.
func TestBuildToleratesTransientDiscoveryFailure(t *testing.T) {
	cases := []struct {
		name string
		// builder gives the builder for the case and the stand-in cloud,
		// which is nil where no request can arrive.
		builder func(t *testing.T, log *logRecorder) (*Builder, *discoveryCloud)
		// keys are the Evidence keys the engine has after the build.
		keys []string
	}{
		{"accessibleproperties 500", func(t *testing.T, log *logRecorder) (*Builder, *discoveryCloud) {
			cloud := &discoveryCloud{}
			cloud.propertiesStatus.Store(500)
			return discoveryBuilder(cloud.serve(t), log), cloud
		}, discoveryKeys},
		{"evidencekeys 503", func(t *testing.T, log *logRecorder) (*Builder, *discoveryCloud) {
			cloud := &discoveryCloud{}
			cloud.evidenceStatus.Store(503)
			return discoveryBuilder(cloud.serve(t), log), cloud
		}, nil},
		{"both 502", func(t *testing.T, log *logRecorder) (*Builder, *discoveryCloud) {
			cloud := &discoveryCloud{}
			cloud.propertiesStatus.Store(502)
			cloud.evidenceStatus.Store(502)
			return discoveryBuilder(cloud.serve(t), log), cloud
		}, nil},
		{"unreadable body", func(t *testing.T, log *logRecorder) (*Builder, *discoveryCloud) {
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `{"Products":`)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			return NewBuilder().SetResourceKey(discoveryKey).
				SetEndpoint(srv.URL + "/").SetLogger(slog.New(log)), nil
		}, nil},
		{"unreachable", func(t *testing.T, log *logRecorder) (*Builder, *discoveryCloud) {
			return NewBuilder().SetResourceKey(discoveryKey).
				SetEndpoint(deadEndpoint()).SetLogger(slog.New(log)), nil
		}, nil},
		{"timeout", func(t *testing.T, log *logRecorder) (*Builder, *discoveryCloud) {
			// The engine's own client applies the timeout.
			return NewBuilder().SetResourceKey(discoveryKey).
				SetEndpoint(blockingCloud(t).URL + "/").
				SetTimeout(100 * time.Millisecond).
				SetLogger(slog.New(log)), nil
		}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			log := &logRecorder{}
			b, cloud := c.builder(t, log)
			e, err := buildWithin(t, b, 10*time.Second)
			if err != nil || e == nil {
				t.Fatalf("build gives an engine: %v, and the error %v, want an "+
					"engine", e != nil, err)
			}
			if cloud != nil {
				wantCounts(t, cloud, "after the build", [3]int64{1, 1, 0})
			}
			wantKeys(t, e.EvidenceKeyFilter(), c.keys)
			records := log.all()
			if len(records) != 1 || records[0].Level != slog.LevelWarn ||
				!strings.Contains(records[0].Message, "retry") {
				for _, r := range records {
					t.Logf("%s %s", r.Level, r.Message)
				}
				t.Fatalf("logged %d records, want one warning that the "+
					"engine will retry", len(records))
			}
			var text strings.Builder
			records[0].Attrs(func(a slog.Attr) bool {
				fmt.Fprintf(&text, "%s=%v ", a.Key, a.Value)
				return true
			})
			if strings.Contains(text.String(), discoveryKey) {
				t.Errorf("the warning %q gives away the Resource Key", text.String())
			}
		})
	}
}

// buildWithin builds b, failing the test if the build takes longer than
// limit.
func buildWithin(t *testing.T, b *Builder, limit time.Duration) (*CloudRequestEngine, error) {
	t.Helper()
	type result struct {
		e   *CloudRequestEngine
		err error
	}
	done := make(chan result, 1)
	go func() {
		e, err := b.Build()
		done <- result{e, err}
	}()
	select {
	case r := <-done:
		return r.e, r.err
	case <-time.After(limit):
		t.Fatalf("the build took longer than %s", limit)
		return nil, nil
	}
}

// After a transient failure at build time, the first request retries
// discovery. While the service still fails, the request fails, and
// SuppressProcessExceptions decides whether the failure is returned or only
// recorded. Once the service answers, the retry completes, the Pipeline's
// filter accepts the Evidence keys and nothing is fetched again.
func TestFirstUseRetriesDiscoveryAfterTransientFailure(t *testing.T) {
	for _, suppress := range []bool{false, true} {
		t.Run(fmt.Sprintf("suppress=%v", suppress), func(t *testing.T) {
			cloud := &discoveryCloud{}
			cloud.propertiesStatus.Store(503)
			cloud.evidenceStatus.Store(503)
			e, err := discoveryBuilder(cloud.serve(t), &logRecorder{}).Build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			wantCounts(t, cloud, "after the build", [3]int64{1, 1, 0})
			p := pipelineOf(t, e, suppress)
			if p.EvidenceKeyFilter().Include("header.user-agent") {
				t.Fatal("the Pipeline accepts Evidence the engine has not fetched")
			}

			fd, err := process(p)
			if suppress {
				if err != nil || !fd.HasErrors() {
					t.Fatalf("process gives %v with errors %v, want the "+
						"failure recorded and not returned", err, fd.Errors())
				}
			} else {
				var cloudErr *CloudRequestError
				if !errors.As(err, &cloudErr) || cloudErr.StatusCode != 503 {
					t.Fatalf("process gives %v, want the 503", err)
				}
			}
			// The retry stops at the first failure and sends no data request.
			wantCounts(t, cloud, "after a request while failing", [3]int64{1, 2, 0})

			cloud.propertiesStatus.Store(0)
			cloud.evidenceStatus.Store(0)
			fd, err = process(p)
			if err != nil || fd.HasErrors() {
				t.Fatalf("process once the service answers: %v %v", err, fd.Errors())
			}
			wantCounts(t, cloud, "after the retry", [3]int64{2, 3, 1})
			if !p.EvidenceKeyFilter().Include("header.sec-ch-ua-platform") {
				t.Error("the Pipeline does not accept the Evidence keys the " +
					"retry fetched")
			}
			wantKeys(t, e.EvidenceKeyFilter(), discoveryKeys)

			if _, err := process(p); err != nil {
				t.Fatalf("process: %v", err)
			}
			wantCounts(t, cloud, "after a later request", [3]int64{2, 3, 2})
		})
	}
}

// A discovery result that arrived at build time is kept when the other
// failed, so it is read with no request and only the other is retried.
func TestPartialDiscoveryKeepsWhatArrived(t *testing.T) {
	cloud := &discoveryCloud{}
	cloud.evidenceStatus.Store(503)
	e, err := discoveryBuilder(cloud.serve(t), &logRecorder{}).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	products, err := e.PublicProperties()
	if err != nil || len(products["device"].Properties) != 1 {
		t.Fatalf("PublicProperties gives %v, %v, want the device product",
			products, err)
	}
	wantCounts(t, cloud, "after reading the Properties", [3]int64{1, 1, 0})

	cloud.evidenceStatus.Store(0)
	if _, err := process(pipelineOf(t, e, false)); err != nil {
		t.Fatalf("process: %v", err)
	}
	wantCounts(t, cloud, "after the first request", [3]int64{1, 2, 1})
}

// The discovery requests the build makes count toward the recovery period,
// so a build that fails often enough starts in recovery. Requests are then
// refused as temporarily unavailable with nothing sent, until the period
// ends and the first request retries discovery.
func TestBuildFailuresCountTowardRecovery(t *testing.T) {
	cloud := &discoveryCloud{}
	cloud.propertiesStatus.Store(503)
	cloud.evidenceStatus.Store(503)
	now := time.Unix(1000, 0)
	e, err := discoveryBuilder(cloud.serve(t), &logRecorder{}).
		SetFailuresToEnterRecovery(2).
		SetRecoveryDuration(60 * time.Second).
		setClock(func() time.Time { return now }).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p := pipelineOf(t, e, false)

	var unavailable *TemporarilyUnavailableError
	if _, err := process(p); !errors.As(err, &unavailable) {
		t.Fatalf("process gives %v, want TemporarilyUnavailableError", err)
	}
	if _, err := e.PublicProperties(); !errors.As(err, &unavailable) {
		t.Fatalf("PublicProperties gives %v, want TemporarilyUnavailableError", err)
	}
	wantCounts(t, cloud, "during the recovery period", [3]int64{1, 1, 0})

	now = now.Add(61 * time.Second)
	cloud.propertiesStatus.Store(0)
	cloud.evidenceStatus.Store(0)
	if _, err := process(p); err != nil {
		t.Fatalf("process after the recovery period: %v", err)
	}
	wantCounts(t, cloud, "after the recovery period", [3]int64{2, 2, 1})
}

// A refused first-use retry of discovery answers that one request, so it
// does not count toward the recovery period, which every visitor shares.
// Each request still reaches the cloud service and gets its refusal.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/cloud-request-engine.md#failures-that-count
func TestRefusedDiscoveryRetryDoesNotCountTowardRecovery(t *testing.T) {
	for _, endpoint := range []string{"accessibleproperties", "evidencekeys"} {
		for _, status := range []int{400, 401, 403, 404, 408} {
			t.Run(fmt.Sprintf("%s/%d", endpoint, status), func(t *testing.T) {
				cloud := &discoveryCloud{}
				failing, want := &cloud.evidenceStatus, [3]int64{1, 4, 0}
				if endpoint == "accessibleproperties" {
					failing, want = &cloud.propertiesStatus, [3]int64{4, 1, 0}
				}
				// One transient failure at build, below the threshold.
				failing.Store(503)
				now := time.Unix(1000, 0)
				e, err := discoveryBuilder(cloud.serve(t), &logRecorder{}).
					SetFailuresToEnterRecovery(2).
					SetRecoveryDuration(60 * time.Second).
					setClock(func() time.Time { return now }).
					Build()
				if err != nil {
					t.Fatalf("build: %v", err)
				}
				p := pipelineOf(t, e, false)
				failing.Store(int64(status))
				for i := 1; i <= 3; i++ {
					_, err := process(p)
					var cloudErr *CloudRequestError
					if !errors.As(err, &cloudErr) || cloudErr.StatusCode != status {
						t.Fatalf("request %d gives %v, want the cloud's %d",
							i, err, status)
					}
				}
				wantCounts(t, cloud, "after three refused retries", want)
			})
		}
	}
}

// PublicProperties gives a copy, so a caller that changes what it is given
// changes neither the engine nor what any other caller reads.
func TestPublicPropertiesGivesACopy(t *testing.T) {
	cloud := &discoveryCloud{}
	e, err := discoveryBuilder(cloud.serve(t), &logRecorder{}).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	products, err := e.PublicProperties()
	if err != nil {
		t.Fatalf("PublicProperties: %v", err)
	}
	products["device"].Properties[0].Name = "changed"
	delete(products, "device")
	products["other"] = ProductMeta{}

	again, err := e.PublicProperties()
	if err != nil || len(again) != 1 || len(again["device"].Properties) != 1 ||
		again["device"].Properties[0].Name != "ismobile" {
		t.Fatalf("after a caller changed its copy, PublicProperties gives "+
			"%v, %v, want the device product as the cloud gave it", again, err)
	}
}

// Factory gives no element with its error, so a caller that checks the
// element against nil sees the failure.
func TestFactoryGivesNoElementWhenTheBuildFails(t *testing.T) {
	cloud := &discoveryCloud{}
	cloud.evidenceStatus.Store(401)
	srv := cloud.serve(t)
	cases := []struct {
		name   string
		params map[string]any
	}{
		{"no Resource Key", map[string]any{"EndPoint": srv.URL + "/"}},
		{"refused Resource Key", map[string]any{
			"ResourceKey": discoveryKey, "EndPoint": srv.URL + "/"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			element, err := Factory(c.params, slog.New(&logRecorder{}))
			if err == nil {
				t.Fatal("Factory succeeded")
			}
			if element != nil {
				t.Errorf("Factory gives the element %#v with its error, "+
					"want none", element)
			}
		})
	}
}
