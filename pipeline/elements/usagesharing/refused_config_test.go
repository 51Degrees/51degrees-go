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

// These tests follow the queue size and timeout options, which a
// configuration file can set.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L444-L446

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// refuses reports the error the Pipeline build gives for the element b makes,
// and checks that Factory refuses the same option in a configuration file.
func refuses(t *testing.T, b *Builder, params map[string]any) error {
	t.Helper()
	e := b.Build()
	defer e.Close()
	_, err := core.NewPipelineBuilder().AddFlowElement(e).Build()
	var refusal *core.ConfigurationError
	if !errors.As(err, &refusal) {
		t.Fatalf("the Pipeline build gives %v, want a configuration error", err)
	}
	if !strings.Contains(err.Error(), "usage sharing") {
		t.Errorf("the message does not say which element refused it: %v", err)
	}
	if _, ferr := Factory(params, nil); !errors.As(ferr, &refusal) {
		t.Errorf("Factory gives %v, want a configuration error", ferr)
	}
	return err
}

// The specification sets no lower limit on the queue size, so a size below
// the minimum entries per message builds, zero included, with a queue of that
// size.
func TestAQueueSizeBelowTheMinimumEntriesBuilds(t *testing.T) {
	// 20 and zero against the default minimum of 50.
	for _, n := range []int{20, 0} {
		e := NewBuilder().SetMaximumQueueSize(n).Build()
		if _, err := core.NewPipelineBuilder().AddFlowElement(e).Build(); err != nil {
			t.Errorf("a queue size of %d is refused: %v", n, err)
		}
		if got := cap(e.queue); got != n {
			t.Errorf("a queue size of %d makes a queue of %d", n, got)
		}
		e.Close()
		f, err := Factory(map[string]any{"MaximumQueueSize": n}, nil)
		if err != nil {
			t.Errorf("Factory refuses a queue size of %d: %v", n, err)
			continue
		}
		f.Close()
	}
}

// A negative queue size, which no queue can have, is refused, and does not
// panic where the queue is made.
func TestANegativeQueueSizeIsRefusedWithoutAPanic(t *testing.T) {
	refuses(t, NewBuilder().SetMaximumQueueSize(-1),
		map[string]any{"MaximumQueueSize": -1})
}

// An add timeout of zero or less discards an entry whenever the queue has no
// room at that moment, so it is refused.
func TestAnAddTimeoutOfZeroOrLessIsRefused(t *testing.T) {
	refuses(t, NewBuilder().SetAddTimeout(0),
		map[string]any{"AddTimeoutMilliseconds": 0})
	refuses(t, NewBuilder().SetAddTimeout(-time.Millisecond),
		map[string]any{"AddTimeoutMilliseconds": -1})
}

// A queue size equal to the minimum entries per message holds a whole
// message, so it is accepted, and so is a take timeout of zero, which means
// take only what is queued.
func TestAQueueSizeAtTheMinimumEntriesIsAccepted(t *testing.T) {
	e := NewBuilder().
		SetMinimumEntriesPerMessage(10).
		SetMaximumQueueSize(10).
		SetTakeTimeout(0).
		Build()
	defer e.Close()
	if _, err := core.NewPipelineBuilder().AddFlowElement(e).Build(); err != nil {
		t.Fatalf("a queue size equal to the minimum entries is refused: %v", err)
	}
	if _, err := Factory(map[string]any{
		"MinimumEntriesPerMessage": 10,
		"MaximumQueueSize":         10,
		"TakeTimeoutMilliseconds":  0,
	}, nil); err != nil {
		t.Fatalf("Factory refuses the same configuration in a file: %v", err)
	}
}
