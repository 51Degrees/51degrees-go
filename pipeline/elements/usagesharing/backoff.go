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

// Repeated failed sends pause sending, so a service that is down is not sent
// a message for every batch and the data waits in the queue.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md#error-handling

import (
	"fmt"
	"math"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// Defaults and limits for the back-off after failed sends, from the
// usage-sharing specification.
const (
	DefaultFailuresToEnterRecovery = 10
	DefaultFailuresWindow          = 100 * time.Second
	DefaultInitialRecoveryDelay    = 2 * time.Second
	DefaultMaximumRecoveryDelay    = 300 * time.Second
	DefaultRecoveryMultiplier      = 2.0

	maxFailuresToEnterRecovery = 100
	minFailuresWindow          = time.Second
)

// queueSizePerMinimum makes the default queue this many times the minimum
// entries per message when that is more than DefaultMaxQueueSize, so a queue
// that fills during a pause still holds several messages.
const queueSizePerMinimum = 10

// recoveryConfig holds the five options that set the back-off.
type recoveryConfig struct {
	failuresToEnterRecovery int
	failuresWindow          time.Duration
	initialDelay            time.Duration
	maxDelay                time.Duration
	multiplier              float64
}

func defaultRecovery() recoveryConfig {
	return recoveryConfig{
		failuresToEnterRecovery: DefaultFailuresToEnterRecovery,
		failuresWindow:          DefaultFailuresWindow,
		initialDelay:            DefaultInitialRecoveryDelay,
		maxDelay:                DefaultMaximumRecoveryDelay,
		multiplier:              DefaultRecoveryMultiplier,
	}
}

// validate refuses a value outside the range the specification gives.
func (r recoveryConfig) validate() error {
	var problem string
	switch {
	case r.failuresToEnterRecovery < 1 ||
		r.failuresToEnterRecovery > maxFailuresToEnterRecovery:
		problem = fmt.Sprintf("failures to enter recovery is %d, and must be "+
			"from 1 to %d", r.failuresToEnterRecovery, maxFailuresToEnterRecovery)
	case r.failuresWindow < minFailuresWindow:
		problem = fmt.Sprintf("the failures window is %s, and must be %s or "+
			"more", r.failuresWindow, minFailuresWindow)
	case r.initialDelay <= 0:
		problem = fmt.Sprintf("the initial recovery delay is %s, and must be "+
			"more than 0", r.initialDelay)
	case r.maxDelay <= 0:
		problem = fmt.Sprintf("the maximum recovery delay is %s, and must be "+
			"more than 0", r.maxDelay)
	case !(r.multiplier >= 1) || math.IsInf(r.multiplier, 1):
		problem = fmt.Sprintf("the recovery multiplier is %v, and must be 1 "+
			"or more", r.multiplier)
	default:
		return nil
	}
	return core.NewConfigurationError("usage sharing: "+problem, nil)
}

// backOff decides when sending pauses. Only the goroutine that sends uses
// it.
type backOff struct {
	cfg recoveryConfig
	// failures are the times of failed sends within the window, oldest
	// first.
	failures []time.Time
	// delay is the length of the latest pause, or 0 when sending has not
	// paused since the last send that ended a back-off.
	delay time.Duration
	// until is when the latest pause ends.
	until time.Time
}

// backOffEvent says what a send's result did to the back-off.
type backOffEvent int

const (
	backOffNone backOffEvent = iota
	backOffPaused
	backOffLengthened
	backOffResumed
)

// remaining returns how much of the pause is left at now, which is 0 or less
// when sending is not paused.
func (b *backOff) remaining(now time.Time) time.Duration {
	return b.until.Sub(now)
}

// record notes the result of a send made at now. A send made while backing
// off is the trial after a pause. Its failure lengthens the pause, and its
// success ends the back-off and starts the count of failures again. Any other
// failure is counted, and enough of them within the window start a pause.
// Other successes leave the count as it is.
func (b *backOff) record(ok bool, now time.Time) backOffEvent {
	trial := b.delay > 0
	switch {
	case ok && trial:
		b.delay = 0
		b.failures = nil
		return backOffResumed
	case ok:
		return backOffNone
	case trial:
		b.delay = b.capped(time.Duration(float64(b.delay) * b.cfg.multiplier))
		b.until = now.Add(b.delay)
		return backOffLengthened
	}
	cutoff := now.Add(-b.cfg.failuresWindow)
	kept := b.failures[:0]
	for _, t := range b.failures {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	b.failures = append(kept, now)
	if len(b.failures) < b.cfg.failuresToEnterRecovery {
		return backOffNone
	}
	b.failures = nil
	b.delay = b.capped(b.cfg.initialDelay)
	b.until = now.Add(b.delay)
	return backOffPaused
}

// capped limits a pause to the maximum recovery delay.
func (b *backOff) capped(d time.Duration) time.Duration {
	if d > b.cfg.maxDelay || d < 0 {
		return b.cfg.maxDelay
	}
	return d
}
