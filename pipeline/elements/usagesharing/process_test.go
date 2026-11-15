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
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// processCalls is how many times the test processes, each starting a message
// of its own.
const processCalls = 20

// processWaitSeconds guards against a hang. The stand-in service never
// answers, so Process calls that waited for a send would never return, and
// Process calls that do not wait return in milliseconds, which the test logs.
const processWaitSeconds = 10

// Process queues what it shares and returns, leaving the send to the
// element's sending goroutine, so a service that does not answer holds up no
// request.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L120-L121
func TestProcessDoesNotWaitForTheSend(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}))
	t.Cleanup(srv.Close)

	p, err := core.NewPipelineBuilder().
		SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil))).
		AddFlowElement(NewBuilder().
			SetMinimumEntriesPerMessage(1).
			SetHTTPClient(srv.Client()).
			SetShareUsageURL(srv.URL).
			SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil))).
			Build()).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	// Cleanups run last first, so the service answers before the Pipeline
	// closes, and the Pipeline closes before the service stops.
	t.Cleanup(func() { p.Close() })
	t.Cleanup(func() { close(release) })

	started := time.Now()
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < processCalls; i++ {
			share(t, p, i)
		}
	}()
	select {
	case <-done:
		t.Logf("%d Process calls took %v with the service not answering",
			processCalls, time.Since(started))
	case <-time.After(processWaitSeconds * time.Second):
		t.Fatalf("%d Process calls had not returned after %d seconds with "+
			"the service not answering, so Process waits for the send",
			processCalls, processWaitSeconds)
	}
}
