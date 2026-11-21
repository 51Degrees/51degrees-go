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
	"sync"
	"testing"
	"time"
)

// fakeClock moves only when a test advances it, so a test decides when a
// pause, a take timeout or a repeat Evidence interval ends, rather than
// sleeping until it does.
type fakeClock struct {
	mu     sync.Mutex
	at     time.Time
	timers []*fakeTimer
}

// fakeTimer is a timer of a fakeClock, waiting until due.
type fakeTimer struct {
	due   time.Time
	fired chan time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{at: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

// newTimer starts a timer that fires once the clock is advanced by d, or at
// once when d is not more than 0.
func (c *fakeClock) newTimer(d time.Duration) (<-chan time.Time, func() bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	timer := &fakeTimer{due: c.at.Add(d), fired: make(chan time.Time, 1)}
	if d <= 0 {
		timer.fired <- c.at
		return timer.fired, func() bool { return false }
	}
	c.timers = append(c.timers, timer)
	return timer.fired, func() bool { return c.stop(timer) }
}

// stop removes timer and reports whether it was waiting.
func (c *fakeClock) stop(timer *fakeTimer) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, waiting := range c.timers {
		if waiting == timer {
			c.timers = append(c.timers[:i], c.timers[i+1:]...)
			return true
		}
	}
	return false
}

// advance moves the clock on by d and fires every timer then due.
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
	waiting := c.timers[:0]
	for _, timer := range c.timers {
		if timer.due.After(c.at) {
			waiting = append(waiting, timer)
		} else {
			timer.fired <- c.at
		}
	}
	c.timers = waiting
}

// waiting returns the number of timers waiting.
func (c *fakeClock) waiting() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

// waitForTimer waits up to two seconds for a timer to be waiting, which is
// when the sending goroutine has paused or is waiting for an entry to take.
// Until the clock is advanced, the goroutine then sends nothing.
func (c *fakeClock) waitForTimer(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for c.waiting() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the sending goroutine did not start waiting on a timer")
		}
		time.Sleep(time.Millisecond)
	}
}

// withClock sets the clock that the element b builds uses.
func withClock(b *Builder, c clock) *Builder {
	b.clock = c
	return b
}

// closeWithin closes p and reports whether it returned within two seconds.
// If it has not, the clock is advanced by release so a wait that Close
// should not have made ends, and the test can finish.
func closeWithin(t *testing.T, p interface{ Close() error }, c *fakeClock,
	release time.Duration) bool {
	t.Helper()
	done := make(chan struct{})
	go func() {
		p.Close()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(2 * time.Second):
		c.advance(release)
		<-done
		return false
	}
}
