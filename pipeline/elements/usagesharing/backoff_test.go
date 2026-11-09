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
	"errors"
	"log/slog"
	"math"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// fastBackOff is a builder that pauses after threshold failed sends, first
// for initial, doubling up to two seconds, sending each entry as it comes.
func fastBackOff(threshold int, initial time.Duration) *Builder {
	return NewBuilder().
		SetMinimumEntriesPerMessage(1).
		SetTakeTimeout(0).
		SetFailuresToEnterRecovery(threshold).
		SetFailuresWindow(10 * time.Second).
		SetInitialRecoveryDelay(initial).
		SetRecoveryMultiplier(2).
		SetMaximumRecoveryDelay(2 * time.Second)
}

// The first send after a pause is a trial. When it fails the pause doubles,
// and when it succeeds sending resumes, each with its log entry.
func TestBackOffTrialLengthensThenResumes(t *testing.T) {
	u := &usageService{}
	log := &usageLog{}
	p := usagePipeline(t, fastBackOff(2, 300*time.Millisecond), u, log)
	failSends(t, p, u, 2)
	for i := 3; i <= 5; i++ {
		share(t, p, i)
	}
	time.Sleep(150 * time.Millisecond)
	if got := len(u.messages()); got != 2 {
		t.Fatalf("received %d messages during the first pause, want 2", got)
	}
	// The trial after 300ms takes two of the three queued entries and fails.
	u.waitForMessages(t, 3, time.Second)
	time.Sleep(300 * time.Millisecond)
	if got := len(u.messages()); got != 3 {
		t.Fatalf("received %d messages during the second pause, want 3", got)
	}
	// The trial after 600ms succeeds and sends the entry left.
	u.status.Store(http.StatusOK)
	u.waitForMessages(t, 4, 2*time.Second)
	if got := u.messages(); !reflect.DeepEqual(got, []int{1, 1, 2, 1}) {
		t.Errorf("received the messages %v, want [1 1 2 1]", got)
	}
	for _, want := range []struct {
		level slog.Level
		part  string
	}{
		{slog.LevelWarn, "delay=300ms"},
		{slog.LevelWarn, "delay=600ms"},
		{slog.LevelInfo, "resumed"},
	} {
		if got := log.matching(want.level, want.part); len(got) != 1 {
			t.Errorf("logged %v, want one %s entry holding %q", got,
				want.level, want.part)
		}
	}
}

// When a pause ends with the minimum entries queued, sending starts at once
// and goes on until fewer than the minimum are queued, with no further
// requests.
func TestBackOffDrainsQueueWhenPauseEnds(t *testing.T) {
	u := &usageService{}
	p := usagePipeline(t, fastBackOff(1, 300*time.Millisecond).
		SetMinimumEntriesPerMessage(2), u, &usageLog{})
	u.status.Store(http.StatusInternalServerError)
	share(t, p, 1)
	share(t, p, 2)
	u.waitForMessages(t, 1, 2*time.Second)
	u.status.Store(http.StatusOK)
	for i := 3; i <= 9; i++ {
		share(t, p, i)
	}
	u.waitForMessages(t, 3, 2*time.Second)
	time.Sleep(100 * time.Millisecond)
	if got := u.messages(); !reflect.DeepEqual(got, []int{2, 4, 3}) {
		t.Errorf("received the messages %v, want [2 4 3]", got)
	}
}

// A success between failures leaves the count as it is, so a high rate of
// failures pauses sending even when some sends succeed.
func TestSuccessDoesNotResetFailureCount(t *testing.T) {
	u := &usageService{}
	p := usagePipeline(t, fastBackOff(2, time.Second), u, &usageLog{})
	for i, status := range []int{500, 200, 500} {
		u.status.Store(int64(status))
		share(t, p, i)
		u.waitForMessages(t, i+1, 2*time.Second)
	}
	share(t, p, 3)
	time.Sleep(300 * time.Millisecond)
	if got := len(u.messages()); got != 3 {
		t.Errorf("received %d messages, want 3 while sending is paused", got)
	}
}

// A failure older than the failures window is not counted.
func TestFailuresOutsideWindowNotCounted(t *testing.T) {
	u := &usageService{}
	p := usagePipeline(t, fastBackOff(2, 2*time.Second).
		SetFailuresWindow(time.Second), u, &usageLog{})
	failSends(t, p, u, 1)
	time.Sleep(1100 * time.Millisecond)
	share(t, p, 2)
	u.waitForMessages(t, 2, 2*time.Second)
	share(t, p, 3)
	u.waitForMessages(t, 3, time.Second)
}

// A back-off option outside the range the specification gives is refused
// when the element is added to a Pipeline and when it is built from
// configuration, and the edges of each range are accepted.
func TestBackOffOptionsRefusedOutsideRange(t *testing.T) {
	refused := map[string]struct {
		set    func(*Builder) *Builder
		params map[string]any
	}{
		"failures 0": {func(b *Builder) *Builder { return b.SetFailuresToEnterRecovery(0) },
			map[string]any{"FailuresToEnterRecovery": 0}},
		"failures 101": {func(b *Builder) *Builder { return b.SetFailuresToEnterRecovery(101) },
			map[string]any{"FailuresToEnterRecovery": 101}},
		"window 0.5s": {func(b *Builder) *Builder { return b.SetFailuresWindow(500 * time.Millisecond) },
			map[string]any{"FailuresWindowSeconds": 0.5}},
		"initial 0": {func(b *Builder) *Builder { return b.SetInitialRecoveryDelay(0) },
			map[string]any{"InitialRecoveryDelaySeconds": 0}},
		"maximum -1s": {func(b *Builder) *Builder { return b.SetMaximumRecoveryDelay(-time.Second) },
			map[string]any{"MaximumRecoveryDelaySeconds": -1}},
		"multiplier 0.5": {func(b *Builder) *Builder { return b.SetRecoveryMultiplier(0.5) },
			map[string]any{"RecoveryMultiplier": 0.5}},
		"multiplier NaN": {func(b *Builder) *Builder { return b.SetRecoveryMultiplier(math.NaN()) },
			nil},
	}
	for name, c := range refused {
		t.Run(name, func(t *testing.T) {
			e := c.set(NewBuilder()).Build()
			defer e.Close()
			_, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
			var config *core.ConfigurationError
			if !errors.As(err, &config) {
				t.Errorf("the Pipeline build gives %v, want a configuration error", err)
			}
			if c.params != nil {
				if _, err := Factory(c.params, nil); !errors.As(err, &config) {
					t.Errorf("Factory gives %v, want a configuration error", err)
				}
			}
		})
	}

	e := NewBuilder().
		SetFailuresToEnterRecovery(100).
		SetFailuresWindow(time.Second).
		SetInitialRecoveryDelay(time.Millisecond).
		SetMaximumRecoveryDelay(time.Millisecond).
		SetRecoveryMultiplier(1).
		Build()
	p, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	if err != nil {
		t.Fatalf("the edges of the ranges were refused: %v", err)
	}
	p.Close()
	el, err := Factory(map[string]any{"FailuresToEnterRecovery": 1,
		"FailuresWindowSeconds": 1, "RecoveryMultiplier": 1}, nil)
	if err != nil {
		t.Fatalf("Factory refused the edges of the ranges: %v", err)
	}
	el.Close()
}

// The tracker shares new Evidence and records when, holds back a repeat
// within the interval, and undo puts an entry back as it was before.
func TestTrackerShareAndUndo(t *testing.T) {
	tr := newTracker(time.Hour)
	if shared, _ := tr.share(1); !shared {
		t.Fatal("new Evidence was not shared")
	}
	if shared, _ := tr.share(1); shared {
		t.Fatal("a repeat within the interval was shared")
	}
	shared, undo := tr.share(2)
	if !shared {
		t.Fatal("new Evidence was not shared")
	}
	undo()
	if shared, _ := tr.share(2); !shared {
		t.Error("Evidence undone was not shared as new when next seen")
	}

	short := newTracker(50 * time.Millisecond)
	short.share(3)
	time.Sleep(60 * time.Millisecond)
	shared, undo = short.share(3)
	if !shared {
		t.Fatal("Evidence was not shared once the interval had passed")
	}
	undo()
	if shared, _ := short.share(3); !shared {
		t.Error("undo did not put back the time the Evidence was last shared")
	}
}
