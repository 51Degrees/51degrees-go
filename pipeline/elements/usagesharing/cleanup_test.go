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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// wantCleanupLimitSeconds is the cleanup limit the specification gives.
const wantCleanupLimitSeconds = 5

// closeWaitSeconds is how long the test waits for Close, being well over the
// cleanup limit on any runner.
const closeWaitSeconds = 15

// pollIntervalMilliseconds is how often the test looks for the stand-in
// service to see the request stop.
const pollIntervalMilliseconds = 10

// Closing the element while the service does not answer stops sending once
// the cleanup limit has passed, discards the entries still to send and logs
// a warning with how many, so that Close does not hold up the process that
// closes it.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/pipeline-elements/usage-sharing-element.md#cleanup
func TestCloseStopsSendingAfterTheCleanupLimit(t *testing.T) {
	release := make(chan struct{})
	var abandoned atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// The server sees the client stop the request only once the
			// body has been read.
			io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
				abandoned.Store(true)
			case <-release:
			}
		}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	log := &usageLog{}
	p, err := core.NewPipelineBuilder().
		SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil))).
		AddFlowElement(NewBuilder().
			SetMinimumEntriesPerMessage(10).
			SetHTTPClient(srv.Client()).
			SetShareUsageURL(srv.URL).
			SetLogger(slog.New(log)).
			Build()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	// Fewer entries than the minimum per message, so nothing is sent before
	// Close sends what is left.
	for i := 0; i < 3; i++ {
		share(t, p, i)
	}

	started := time.Now()
	done := make(chan struct{})
	go func() {
		p.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(closeWaitSeconds * time.Second):
		t.Fatalf("Close had not returned after %d seconds, with a cleanup "+
			"limit of %d seconds", closeWaitSeconds, wantCleanupLimitSeconds)
	}
	if took := time.Since(started); took < wantCleanupLimitSeconds*time.Second {
		t.Errorf("Close returned after %v, before the cleanup limit of %d "+
			"seconds, while the final message was still being sent", took,
			wantCleanupLimitSeconds)
	}
	// The service sees the request stop a moment after Close returns.
	deadline := time.Now().Add(closeWaitSeconds * time.Second)
	for !abandoned.Load() && time.Now().Before(deadline) {
		time.Sleep(pollIntervalMilliseconds * time.Millisecond)
	}
	if !abandoned.Load() {
		t.Error("the final message's request was not stopped")
	}
	if got := log.matching(slog.LevelWarn, "discarded=3"); len(got) != 1 {
		t.Errorf("the warnings naming 3 entries discarded are %v, want one",
			got)
	}
}
