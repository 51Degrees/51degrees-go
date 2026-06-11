package requestengine

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/pipeline-go/core"
)

// cloudServer is a fake 51Degrees cloud service for tests.
type cloudServer struct {
	jsonStatus   int
	jsonBody     string
	lastForm     url.Values
	lastOrigin   string
	jsonRequests atomic.Int64
}

func (c *cloudServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/accessibleproperties", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Products":{"device":{"DataTier":"CloudV4","Properties":[{"Name":"ismobile","Type":"Bool","Category":"Device"}]}}}`))
	})
	mux.HandleFunc("/evidencekeys", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["header.user-agent","query.user-agent"]`))
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		c.jsonRequests.Add(1)
		c.lastOrigin = r.Header.Get("Origin")
		_ = r.ParseForm()
		c.lastForm = r.PostForm
		status := c.jsonStatus
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		io.WriteString(w, c.jsonBody)
	})
	return mux
}

func buildEngine(t *testing.T, srv *httptest.Server) *CloudRequestEngine {
	t.Helper()
	e, err := NewBuilder().
		SetResourceKey("test-key").
		SetEndpoint(srv.URL + "/").
		SetCloudRequestOrigin("example.com").
		SetHTTPClient(srv.Client()).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return e
}

func TestCloudRequestSuccess(t *testing.T) {
	cs := &cloudServer{jsonBody: `{"device":{"ismobile":true}}`}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()

	e := buildEngine(t, srv)
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()

	fd := p.CreateFlowData()
	fd.AddEvidence("query.user-agent", "ua-query")
	fd.AddEvidence("header.user-agent", "ua-header")
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}

	data, err := fd.Get(DataKey)
	if err != nil {
		t.Fatalf("get response: %v", err)
	}
	crd := data.(*CloudRequestData)
	if !strings.Contains(crd.JSONResponse(), "ismobile") {
		t.Fatalf("unexpected response: %s", crd.JSONResponse())
	}

	// Origin header set.
	if cs.lastOrigin != "example.com" {
		t.Fatalf("origin not set, got %q", cs.lastOrigin)
	}
	// Prefix removed and precedence applied: query outranks header.
	if got := cs.lastForm.Get("user-agent"); got != "ua-query" {
		t.Fatalf("expected query value to win precedence, got %q", got)
	}
	if cs.lastForm.Get("resource") != "test-key" {
		t.Fatalf("resource key not sent, got %q", cs.lastForm.Get("resource"))
	}
}

func TestCloudRequestErrorsArray(t *testing.T) {
	cs := &cloudServer{jsonStatus: http.StatusBadRequest, jsonBody: `{"errors":["bad resource key"]}`}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()
	e := buildEngine(t, srv)
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()
	fd := p.CreateFlowData()
	err := fd.Process()
	if err == nil {
		t.Fatal("expected error")
	}
	var cre *CloudRequestError
	if !errors.As(err, &cre) {
		t.Fatalf("expected CloudRequestError, got %v", err)
	}
	if !strings.Contains(cre.Error(), "bad resource key") {
		t.Fatalf("error message should carry server error: %v", cre)
	}
}

func TestCloudRequestEmptyBody(t *testing.T) {
	cs := &cloudServer{jsonStatus: http.StatusOK, jsonBody: ""}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()
	e := buildEngine(t, srv)
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()
	err := p.CreateFlowData().Process()
	if err == nil || !strings.Contains(err.Error(), "No data in response") {
		t.Fatalf("expected empty-body error, got %v", err)
	}
}

func TestCloudRequestNon200(t *testing.T) {
	cs := &cloudServer{jsonStatus: http.StatusInternalServerError, jsonBody: `{"foo":1}`}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()
	e := buildEngine(t, srv)
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()
	err := p.CreateFlowData().Process()
	if err == nil || !strings.Contains(err.Error(), "status code '500'") {
		t.Fatalf("expected status code error, got %v", err)
	}
}

func TestRecoveryMode(t *testing.T) {
	cs := &cloudServer{jsonStatus: http.StatusInternalServerError, jsonBody: `{}`}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()

	e, err := NewBuilder().
		SetResourceKey("k").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		SetFailuresToEnterRecovery(2).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()

	// Two failures trip recovery.
	_ = p.CreateFlowData().Process()
	_ = p.CreateFlowData().Process()
	jsonReqs := cs.jsonRequests.Load()

	// Third request should be bypassed with a TemporarilyUnavailableError.
	err = p.CreateFlowData().Process()
	var tue *TemporarilyUnavailableError
	if !errors.As(err, &tue) {
		t.Fatalf("expected TemporarilyUnavailableError, got %v", err)
	}
	if cs.jsonRequests.Load() != jsonReqs {
		t.Fatal("request should have been bypassed during recovery")
	}
}

func TestRecoveryResetsAfterPeriod(t *testing.T) {
	cs := &cloudServer{jsonStatus: http.StatusInternalServerError, jsonBody: `{}`}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()

	now := time.Unix(1000, 0)
	clock := func() time.Time { return now }
	e, err := NewBuilder().
		SetResourceKey("k").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		SetFailuresToEnterRecovery(1).
		SetRecoveryDuration(60 * time.Second).
		setClock(clock).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()

	// One failure trips recovery (threshold 1).
	_ = p.CreateFlowData().Process()
	// Still within recovery: bypassed.
	if err := p.CreateFlowData().Process(); err == nil {
		t.Fatal("expected error during recovery")
	} else {
		var tue *TemporarilyUnavailableError
		if !errors.As(err, &tue) {
			t.Fatalf("expected TemporarilyUnavailableError, got %v", err)
		}
	}
	// Advance past the recovery period and let the server succeed.
	now = now.Add(61 * time.Second)
	cs.jsonStatus = http.StatusOK
	cs.jsonBody = `{"device":{"ismobile":true}}`
	if err := p.CreateFlowData().Process(); err != nil {
		t.Fatalf("request should be allowed after recovery, got %v", err)
	}
}

func TestRecoveryDisabled(t *testing.T) {
	cs := &cloudServer{jsonStatus: http.StatusInternalServerError, jsonBody: `{}`}
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()
	e, _ := NewBuilder().
		SetResourceKey("k").
		SetEndpoint(srv.URL + "/").
		SetHTTPClient(srv.Client()).
		SetRecoveryDuration(0). // disabled
		Build()
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).Build()
	defer p.Close()
	// Many failures, but recovery never engages, so each still attempts a request.
	for i := 0; i < 5; i++ {
		_ = p.CreateFlowData().Process()
	}
	if cs.jsonRequests.Load() != 5 {
		t.Fatalf("expected 5 attempts with recovery disabled, got %d", cs.jsonRequests.Load())
	}
}

func TestSuppressedMetadataFailure(t *testing.T) {
	// Server that fails the metadata endpoints; with suppression enabled the
	// pipeline should not return an error but should collect one.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	e := buildEngine(t, srv)
	p, _ := core.NewPipelineBuilder().AddFlowElement(e).SetSuppressProcessExceptions(true).Build()
	defer p.Close()
	fd := p.CreateFlowData()
	if err := fd.Process(); err != nil {
		t.Fatalf("expected suppressed error to be nil, got %v", err)
	}
	if !fd.HasErrors() {
		t.Fatal("expected error to be collected")
	}
}
