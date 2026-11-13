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

package onpremise

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"
)

const (
	leakCheckCollections    = 3
	leakCheckPollMs         = 50
	leakCheckTimeoutSeconds = 10
)

// TestMain fails the run when a test leaves native resources behind. A running
// engine keeps its data set in native memory until the process ends, so every
// engine a test creates must be stopped. Collecting garbage first makes the
// finalizer of any results, evidence or manager a test did not free panic,
// which also fails the run.
func TestMain(m *testing.M) {
	code := m.Run()

	for i := 0; i < leakCheckCollections; i++ {
		runtime.GC()
		time.Sleep(leakCheckPollMs * time.Millisecond)
	}

	deadline := time.Now().Add(leakCheckTimeoutSeconds * time.Second)
	running := engineGoroutines()
	for running > 0 && time.Now().Before(deadline) {
		time.Sleep(leakCheckPollMs * time.Millisecond)
		running = engineGoroutines()
	}
	if running > 0 {
		fmt.Fprintf(os.Stderr,
			"%d engine goroutines are still running, so a test did not "+
				"stop an engine it created.\n", running)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
