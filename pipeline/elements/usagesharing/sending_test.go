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

// These tests follow the specification's rules for filling and sending
// messages, pausing after failed sends, and repeated Evidence.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// usageService stands in for the usage sharing service. It records how many
// entries each message holds and answers with status, or 200 when status is
// 0.
type usageService struct {
	status  atomic.Int64
	mu      sync.Mutex
	entries []int
}

func (u *usageService) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The answer is chosen before the message is recorded, so a test that
		// sees the message can change the status for the next one.
		status := int(u.status.Load())
		if status == 0 {
			status = http.StatusOK
		}
		n := -1
		if gz, err := gzip.NewReader(r.Body); err == nil {
			data, _ := io.ReadAll(gz)
			n = strings.Count(string(data), "<Device>")
		}
		u.mu.Lock()
		u.entries = append(u.entries, n)
		u.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// messages returns the number of entries in each message received so far.
func (u *usageService) messages() []int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int(nil), u.entries...)
}

// waitForMessages waits up to within for n messages in all.
func (u *usageService) waitForMessages(t *testing.T, n int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if len(u.messages()) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("received the messages %v, want %d", u.messages(), n)
}

// usageLog keeps every log record made through it, with its attributes as
// text.
type usageLog struct {
	mu      sync.Mutex
	records []string
	levels  []slog.Level
}

func (h *usageLog) Enabled(context.Context, slog.Level) bool { return true }

func (h *usageLog) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, b.String())
	h.levels = append(h.levels, r.Level)
	return nil
}

func (h *usageLog) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *usageLog) WithGroup(string) slog.Handler      { return h }

// matching returns the records at level whose text holds part.
func (h *usageLog) matching(level slog.Level, part string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for i, r := range h.records {
		if h.levels[i] == level && strings.Contains(r, part) {
			out = append(out, r)
		}
	}
	return out
}

// usagePipeline builds a Pipeline holding the element b builds, sending to
// the stand-in service and logging to log.
func usagePipeline(t *testing.T, b *Builder, u *usageService, log *usageLog) core.Pipeline {
	t.Helper()
	srv := u.serve(t)
	e := b.SetHTTPClient(srv.Client()).
		SetShareUsageURL(srv.URL).
		SetLogger(slog.New(log)).
		Build()
	p, err := core.NewPipelineBuilder().
		SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil))).
		AddFlowElement(e).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// share processes one request whose Evidence differs from every other
// request's, so it is never taken for a repeat.
func share(t *testing.T, p core.Pipeline, i int) {
	t.Helper()
	fd := p.CreateFlowData()
	fd.AddEvidence("header.user-agent", fmt.Sprintf("agent-%d", i))
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
}

// A message is not started until the queue holds the minimum entries per
// message, however long the entries wait.
func TestMessageWaitsForMinimumEntries(t *testing.T) {
	u := &usageService{}
	p := usagePipeline(t, NewBuilder().
		SetMinimumEntriesPerMessage(3).
		SetTakeTimeout(20*time.Millisecond), u, &usageLog{})
	share(t, p, 1)
	share(t, p, 2)
	time.Sleep(300 * time.Millisecond)
	if got := u.messages(); len(got) != 0 {
		t.Fatalf("sent %v with fewer than the minimum queued", got)
	}
	share(t, p, 3)
	u.waitForMessages(t, 1, 2*time.Second)
	if got := u.messages(); got[0] != 3 {
		t.Errorf("the message holds %d entries, want 3", got[0])
	}
}

// Once started, a message takes the entries that arrive within the take
// timeout of each other, up to twice the minimum entries per message, and
// what is left waits for the minimum again.
func TestMessageHoldsUpToTwiceMinimum(t *testing.T) {
	u := &usageService{}
	p := usagePipeline(t, NewBuilder().
		SetMinimumEntriesPerMessage(2).
		SetTakeTimeout(300*time.Millisecond), u, &usageLog{})
	for i := 1; i <= 5; i++ {
		share(t, p, i)
	}
	u.waitForMessages(t, 1, 2*time.Second)
	time.Sleep(500 * time.Millisecond)
	if got := u.messages(); len(got) != 1 || got[0] != 4 {
		t.Errorf("received the messages %v, want one of 4 entries", got)
	}
}

// failSends makes the stand-in service refuse every message and shares
// requests one at a time until n messages have failed.
func failSends(t *testing.T, p core.Pipeline, u *usageService, n int) {
	t.Helper()
	u.status.Store(http.StatusInternalServerError)
	for i := 1; i <= n; i++ {
		share(t, p, i)
		u.waitForMessages(t, i, 2*time.Second)
	}
}

// With the default options, ten failed sends pause sending for two seconds,
// while processing goes on adding to the queue.
func TestSendingPausesAfterFailedSends(t *testing.T) {
	u := &usageService{}
	p := usagePipeline(t, NewBuilder().
		SetMinimumEntriesPerMessage(1).
		SetTakeTimeout(time.Millisecond), u, &usageLog{})
	failSends(t, p, u, 10)
	for i := 11; i <= 13; i++ {
		share(t, p, i)
	}
	time.Sleep(time.Second)
	if got := u.messages(); len(got) != 10 {
		t.Errorf("received %d messages, want 10 while sending is paused", len(got))
	}
}

// Closing while sending is paused does not wait for the pause to end and
// sends nothing. It discards what is queued and logs how many.
func TestCloseWhilePausedSendsNothing(t *testing.T) {
	u := &usageService{}
	log := &usageLog{}
	p := usagePipeline(t, NewBuilder().
		SetMinimumEntriesPerMessage(1).
		SetTakeTimeout(time.Millisecond), u, log)
	failSends(t, p, u, 10)
	for i := 11; i <= 13; i++ {
		share(t, p, i)
	}
	start := time.Now()
	p.Close()
	if took := time.Since(start); took > time.Second {
		t.Errorf("close took %s while sending was paused", took)
	}
	if got := u.messages(); len(got) != 10 {
		t.Errorf("received %d messages, want 10 after closing while paused",
			len(got))
	}
	if got := log.matching(slog.LevelWarn, "discarded=3"); len(got) != 1 {
		t.Errorf("logged %v, want one warning that 3 entries were discarded", got)
	}
}

// A full queue discards entries without a log entry for each one.
func TestFullQueueLogsOnce(t *testing.T) {
	u := &usageService{}
	log := &usageLog{}
	p := usagePipeline(t, NewBuilder().
		SetMinimumEntriesPerMessage(100).
		SetMaximumQueueSize(2).
		SetAddTimeout(time.Millisecond), u, log)
	for i := 1; i <= 10; i++ {
		share(t, p, i)
	}
	if got := log.matching(slog.LevelWarn, "full"); len(got) != 1 {
		t.Errorf("logged %d warnings for 8 discarded entries, want 1: %v",
			len(got), got)
	}
}

// The queue holds 1000 entries, or ten times the minimum entries per message
// if that is larger, unless its size is set.
func TestDefaultQueueSize(t *testing.T) {
	cases := []struct {
		name string
		b    *Builder
		want int
	}{
		{"defaults", NewBuilder(), 1000},
		{"minimum 200", NewBuilder().SetMinimumEntriesPerMessage(200), 2000},
		{"size set", NewBuilder().SetMinimumEntriesPerMessage(200).
			SetMaximumQueueSize(300), 300},
	}
	for _, c := range cases {
		e := c.b.Build()
		if got := cap(e.queue); got != c.want {
			t.Errorf("%s: the queue holds %d entries, want %d", c.name, got, c.want)
		}
		e.Close()
	}
}

// Identical Evidence seen again within the repeat Evidence interval is not
// shared, and seeing it does not start the interval again, so Evidence seen
// continuously is still shared once the interval since it was shared ends.
func TestRepeatEvidenceSharedOncePerInterval(t *testing.T) {
	u := &usageService{}
	p := usagePipeline(t, NewBuilder().
		SetMinimumEntriesPerMessage(1).
		SetTakeTimeout(time.Millisecond).
		SetRepeatEvidenceInterval(400*time.Millisecond), u, &usageLog{})
	start := time.Now()
	for time.Since(start) < 600*time.Millisecond {
		share(t, p, 0)
		time.Sleep(50 * time.Millisecond)
	}
	u.waitForMessages(t, 2, 2*time.Second)
	time.Sleep(100 * time.Millisecond)
	if got := u.messages(); len(got) != 2 {
		t.Errorf("received the messages %v, want 2 in 600ms with an "+
			"interval of 400ms", got)
	}
}
