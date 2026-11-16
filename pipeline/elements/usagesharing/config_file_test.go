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
	"testing"
	"time"
)

// The queue size and the two timeouts can be set from a configuration file,
// under the names the options table gives them, as numbers decoded from JSON.
// The minimum entries per message is in the file as well, because a queue
// smaller than a message is refused.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L444-L446
func TestFactoryReadsQueueSizeAndTimeouts(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	fe, err := Factory(map[string]any{
		"MinimumEntriesPerMessage": float64(5),
		"MaximumQueueSize":         float64(7),
		"AddTimeoutMilliseconds":   float64(12),
		"TakeTimeoutMilliseconds":  float64(0),
	}, quiet)
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	cfg := fe.(*Element).cfg
	if cfg.minEntries != 5 {
		t.Errorf("minimum entries per message %d, want 5", cfg.minEntries)
	}
	if cfg.maxQueueSize != 7 {
		t.Errorf("maximum queue size %d, want 7", cfg.maxQueueSize)
	}
	if cfg.addTimeout != 12*time.Millisecond {
		t.Errorf("add timeout %v, want 12ms", cfg.addTimeout)
	}
	if cfg.takeTimeout != 0 {
		t.Errorf("take timeout %v, want 0", cfg.takeTimeout)
	}
}

// Options the configuration file leaves out keep their defaults.
func TestFactoryKeepsQueueSizeAndTimeoutDefaults(t *testing.T) {
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	fe, err := Factory(map[string]any{}, quiet)
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	cfg := fe.(*Element).cfg
	if cfg.minEntries != DefaultMinEntries {
		t.Errorf("minimum entries per message %d, want %d",
			cfg.minEntries, DefaultMinEntries)
	}
	if cfg.maxQueueSize != DefaultMaxQueueSize {
		t.Errorf("maximum queue size %d, want %d",
			cfg.maxQueueSize, DefaultMaxQueueSize)
	}
	if cfg.addTimeout != DefaultAddTimeout {
		t.Errorf("add timeout %v, want %v", cfg.addTimeout, DefaultAddTimeout)
	}
	if cfg.takeTimeout != DefaultTakeTimeout {
		t.Errorf("take timeout %v, want %v",
			cfg.takeTimeout, DefaultTakeTimeout)
	}
}
