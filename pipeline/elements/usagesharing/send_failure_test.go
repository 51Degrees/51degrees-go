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
	"reflect"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// A failed send logs the status code and the content of the response.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md#request-detail
func TestFailedSendLogsStatusAndContent(t *testing.T) {
	const content = "the service is down for maintenance"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, content)
	}))
	defer srv.Close()
	log := &usageLog{}
	e := NewBuilder().
		SetHTTPClient(srv.Client()).
		SetShareUsageURL(srv.URL).
		SetLogger(slog.New(log)).
		SetMinimumEntriesPerMessage(1).
		SetTakeTimeout(0).
		Build()
	p, err := core.NewPipelineBuilder().
		SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil))).
		AddFlowElement(e).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer p.Close()
	share(t, p, 1)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(log.matching(slog.LevelWarn, content)) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	got := log.matching(slog.LevelWarn, content)
	if len(got) != 1 || len(log.matching(slog.LevelWarn, "503")) == 0 {
		t.Errorf("logged %v, want one warning with the status 503 and the "+
			"content %q", got, content)
	}
}

// Closing while a message is being filled stops the wait for further
// entries, so Close does not wait a take timeout, and the entries taken are
// sent.
func TestCloseDuringTakeReturnsPromptly(t *testing.T) {
	u := &usageService{}
	clk := newFakeClock()
	p := usagePipeline(t, withClock(NewBuilder().
		SetMinimumEntriesPerMessage(1).
		SetTakeTimeout(10*time.Second), clk), u, &usageLog{})
	share(t, p, 1)
	// The sending goroutine has taken the entry and waits for another.
	clk.waitForTimer(t)
	if !closeWithin(t, p, clk, 10*time.Second) {
		t.Error("close waited for the take timeout while a message was " +
			"being filled")
	}
	if got := u.messages(); !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("received the messages %v, want [1]", got)
	}
}

// With a take timeout of zero a message takes only the entries already
// queued, so an entry that arrives once the message has started is left for
// the next message.
func TestTakeTimeoutZeroTakesOnlyQueued(t *testing.T) {
	u := &usageService{}
	p := usagePipeline(t, NewBuilder().
		SetMinimumEntriesPerMessage(2).
		SetTakeTimeout(0), u, &usageLog{})
	share(t, p, 1)
	share(t, p, 2)
	u.waitForMessages(t, 1, 2*time.Second)
	share(t, p, 3)
	share(t, p, 4)
	u.waitForMessages(t, 2, 2*time.Second)
	if got := u.messages(); !reflect.DeepEqual(got, []int{2, 2}) {
		t.Errorf("received the messages %v, want [2 2]", got)
	}
}

// A pause is never longer than the maximum recovery delay, whether it is the
// first pause or one lengthened after a failed trial.
func TestBackOffDelayCappedAtMaximum(t *testing.T) {
	now := time.Unix(1000, 0)
	b := &backOff{cfg: recoveryConfig{
		failuresToEnterRecovery: 1,
		failuresWindow:          time.Minute,
		initialDelay:            200 * time.Millisecond,
		maxDelay:                300 * time.Millisecond,
		multiplier:              10,
	}}
	want := []struct {
		event backOffEvent
		delay time.Duration
	}{
		{backOffPaused, 200 * time.Millisecond},
		{backOffLengthened, 300 * time.Millisecond},
		{backOffLengthened, 300 * time.Millisecond},
	}
	for i, w := range want {
		now = now.Add(time.Second)
		if event := b.record(false, now); event != w.event || b.delay != w.delay {
			t.Errorf("failure %d gives event %d and a pause of %s, want %d "+
				"and %s", i+1, event, b.delay, w.event, w.delay)
		}
	}

	first := &backOff{cfg: recoveryConfig{
		failuresToEnterRecovery: 1,
		failuresWindow:          time.Minute,
		initialDelay:            time.Second,
		maxDelay:                300 * time.Millisecond,
		multiplier:              2,
	}}
	first.record(false, now)
	if first.delay != 300*time.Millisecond {
		t.Errorf("an initial delay above the maximum gives a pause of %s, "+
			"want 300ms", first.delay)
	}
}
