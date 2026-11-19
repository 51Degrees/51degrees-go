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

// "It then starts the HTTP request, adds its Element Data with json-response
// as a lazy property whose task is that request, and returns without waiting
// for the response. Every reader of json-response ... waits on that task.
// The task holds only the request it was given, and never reads the Flow
// Data."
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/cloud-request-engine.md?plain=1#L364-L382
// and https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/thread-safety.md?plain=1#L178-L188

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

const (
	// lazyTestHoldMilliseconds is how long the fake service holds a response
	// the test has not released, below the engine's 2 second timeout.
	lazyTestHoldMilliseconds = 1000
	// lazyTestWaitMilliseconds is how long a test watches for a reader or a
	// Close that must still be waiting.
	lazyTestWaitMilliseconds = 100
	// lazyTestReaders is how many goroutines read the response at once.
	lazyTestReaders = 3
	lazyTestBody    = `{"device":{"ismobile":true}}`
)

// heldServer is a fake cloud service that holds each data response until the
// test releases it, or for lazyTestHoldMilliseconds.
type heldServer struct {
	status    int
	release   chan struct{}
	once      sync.Once
	responded atomic.Bool
	requests  atomic.Int32
}

func newHeldServer(t *testing.T, status int) (*heldServer, *httptest.Server) {
	t.Helper()
	h := &heldServer{status: status, release: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"device":{"DataTier":"CloudV4","Properties":[{"Name":"ismobile","Type":"Bool","Category":"Device"}]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["header.user-agent"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		h.requests.Add(1)
		select {
		case <-h.release:
		case <-time.After(lazyTestHoldMilliseconds * time.Millisecond):
		}
		h.responded.Store(true)
		w.WriteHeader(h.status)
		io.WriteString(w, lazyTestBody)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		h.let()
		srv.Close()
	})
	return h, srv
}

// let releases the held response.
func (h *heldServer) let() { h.once.Do(func() { close(h.release) }) }

// responseOf gives the Cloud Request Engine's Element Data in fd.
func responseOf(t *testing.T, fd core.FlowData) core.ElementData {
	t.Helper()
	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get %s: %v", DataKey, err)
	}
	return data
}

// The engine starts the request and returns without waiting for the
// response, and a read of json-response then gives the response.
func TestProcessReturnsBeforeTheResponse(t *testing.T) {
	h, srv := newHeldServer(t, http.StatusOK)
	p, err := core.NewPipelineBuilder().AddFlowElement(buildEngine(t, srv)).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	if h.responded.Load() {
		t.Error("Process waited for the HTTP response")
	}
	h.let()
	v, err := responseOf(t, fd).Get(JSONResponseProperty)
	if err != nil || v != lazyTestBody {
		t.Errorf("json-response is %v, %v, want the response", v, err)
	}
}

// Every reader of json-response waits for the one request, and all of them
// get its response.
func TestEveryReaderWaitsForTheResponse(t *testing.T) {
	h, srv := newHeldServer(t, http.StatusOK)
	p, err := core.NewPipelineBuilder().AddFlowElement(buildEngine(t, srv)).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	data := responseOf(t, fd)
	var returned atomic.Int32
	values := make([]any, lazyTestReaders)
	var wg sync.WaitGroup
	for i := 0; i < lazyTestReaders; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			values[i], _ = data.Get(JSONResponseProperty)
			returned.Add(1)
		}(i)
	}
	time.Sleep(lazyTestWaitMilliseconds * time.Millisecond)
	if n := returned.Load(); n != 0 {
		t.Errorf("%d readers returned before the response arrived", n)
	}
	h.let()
	wg.Wait()
	for i, v := range values {
		if v != lazyTestBody {
			t.Errorf("reader %d got %v, want the response", i, v)
		}
	}
	if n := h.requests.Load(); n != 1 {
		t.Errorf("the service had %d requests, want 1", n)
	}
}

// watchedFlowData counts the reads of its Evidence made once watching
// starts.
type watchedFlowData struct {
	core.FlowData
	watching atomic.Bool
	late     atomic.Int32
}

func (w *watchedFlowData) Evidence() core.Evidence {
	if w.watching.Load() {
		w.late.Add(1)
	}
	return w.FlowData.Evidence()
}

// The request is built while the engine processes, and the task that sends
// it never reads the Flow Data.
func TestTheRequestDoesNotReadTheFlowData(t *testing.T) {
	h, srv := newHeldServer(t, http.StatusOK)
	e := buildEngine(t, srv)
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := &watchedFlowData{FlowData: p.CreateFlowData()}
	fd.AddEvidence("header.user-agent", "ua")
	if err := e.Process(fd); err != nil {
		t.Fatalf("process: %v", err)
	}
	fd.watching.Store(true)
	h.let()
	data := fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData { return nil })
	if v, err := data.Get(JSONResponseProperty); err != nil || v != lazyTestBody {
		t.Errorf("json-response is %v, %v, want the response", v, err)
	}
	if n := fd.late.Load(); n != 0 {
		t.Errorf("the Evidence was read %d times after Process returned", n)
	}
}

// failingReader reads json-response while it processes, and returns no
// error of its own.
type failingReader struct{ *core.BaseElement }

func (r *failingReader) Process(fd core.FlowData) error {
	if data, err := fd.Get(DataKey); err == nil {
		data.Get(JSONResponseProperty)
	}
	return nil
}

func newFailingReader(key string) *failingReader {
	return &failingReader{core.NewBaseElement(key, nil, nil)}
}

// A failed request met by readers while the Flow Data is processed is
// recorded once, as the Cloud Request Engine's error, whether the readers run
// one after another or in a parallel group.
func TestAFailureMetWhileProcessingIsRecordedOnce(t *testing.T) {
	for _, parallel := range []bool{false, true} {
		h, srv := newHeldServer(t, http.StatusInternalServerError)
		h.let()
		e := buildEngine(t, srv)
		b := core.NewPipelineBuilder().AddFlowElement(e)
		if parallel {
			b.AddParallel(newFailingReader("a"), newFailingReader("b"))
		} else {
			b.AddFlowElement(newFailingReader("a")).
				AddFlowElement(newFailingReader("b"))
		}
		p, err := b.Build()
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		fd := p.CreateFlowData()
		fd.AddEvidence("header.user-agent", "ua")
		if err := fd.Process(); err == nil {
			t.Errorf("parallel %v: Process gave no error", parallel)
		}
		errs := fd.Errors()
		if len(errs) != 1 || errs[0].Element != core.FlowElement(e) ||
			!strings.Contains(errs[0].Err.Error(), "status code '500'") {
			t.Errorf("parallel %v: the errors are %v, want one, the Cloud "+
				"Request Engine's, for the status code", parallel, errs)
		}
		p.Close()
	}
}

// A failed request met only once processing has finished reaches the reader
// alone, because nothing can be added to a finished Flow Data.
func TestAFailureMetAfterProcessingReachesOnlyTheReader(t *testing.T) {
	h, srv := newHeldServer(t, http.StatusInternalServerError)
	h.let()
	p, err := core.NewPipelineBuilder().AddFlowElement(buildEngine(t, srv)).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err != nil {
		t.Errorf("Process gave %v, want no error, as nothing read the "+
			"response while it processed", err)
	}
	if _, err := responseOf(t, fd).Get(JSONResponseProperty); err == nil ||
		!strings.Contains(err.Error(), "status code '500'") {
		t.Errorf("the read gave %v, want the error for the status code", err)
	}
	if errs := fd.Errors(); len(errs) != 0 {
		t.Errorf("the Flow Data holds %v, want no errors", errs)
	}
}

// Closing the Pipeline waits for the request the engine started, and the
// engine refuses to process once it is closed.
func TestCloseWaitsForTheRequest(t *testing.T) {
	h, srv := newHeldServer(t, http.StatusOK)
	e := buildEngine(t, srv)
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", "ua")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	closed := make(chan struct{})
	go func() {
		p.Close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Error("Close returned while the request was running")
	case <-time.After(lazyTestWaitMilliseconds * time.Millisecond):
	}
	h.let()
	<-closed
	if !h.responded.Load() {
		t.Error("Close returned before the response")
	}
	if err := e.Process(p.CreateFlowData()); err == nil {
		t.Error("Process gave no error once the engine was closed")
	}
}
