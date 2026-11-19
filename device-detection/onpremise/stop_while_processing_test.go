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

// "Disposing an Engine or a Pipeline MUST wait for every call to process,
// every lazy task and every data refresh already running on it to finish
// before it frees anything they use."
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L33-L40

import (
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
)

// stopTestProcessors is how many goroutines process while the Engine stops.
const stopTestProcessors = 4

// lookup processes the test User-Agent and reads a value from the results.
func lookup(e *Engine) error {
	r, err := e.Process([]Evidence{
		{dd.HttpHeaderString, "User-Agent", reloadTestUserAgent}})
	if err != nil {
		return err
	}
	defer r.Free()
	_, err = r.ValuesString("IsMobile", ",")
	return err
}

// Stop waits for the lookups other goroutines are making, and each lookup
// they make once it has stopped gives an error rather than reading data
// that has been freed.
func TestStopWhileProcessing(t *testing.T) {
	e := newReloadTestEngine(t, WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithAutoUpdate(false), WithFileWatch(false))
	started := make(chan struct{}, stopTestProcessors)
	var wg sync.WaitGroup
	for i := 0; i < stopTestProcessors; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for first := true; ; first = false {
				err := lookup(e.Engine)
				if first {
					started <- struct{}{}
				}
				if err != nil {
					return
				}
			}
		}()
	}
	for i := 0; i < stopTestProcessors; i++ {
		<-started
	}
	e.stop(t)
	wg.Wait()
}

// Every call that reads the data gives an error, or nothing, once the Engine
// has stopped.
func TestCallsAfterStop(t *testing.T) {
	e := newReloadTestEngine(t, WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithAutoUpdate(false), WithFileWatch(false))
	e.stop(t)
	if err := lookup(e.Engine); err == nil {
		t.Error("Process gave no error after Stop")
	}
	if r := e.NewResultsHash(1, 0); r != nil {
		r.Free()
		t.Error("NewResultsHash gave results after Stop")
	}
	if keys := e.GetHttpHeaderKeys(); keys != nil {
		t.Errorf("GetHttpHeaderKeys gave %d keys after Stop", len(keys))
	}
	if _, err := e.Properties(); err == nil {
		t.Error("Properties gave no error after Stop")
	}
	if _, err := e.Components(); err == nil {
		t.Error("Components gave no error after Stop")
	}
	if _, err := e.Profiles(); err == nil {
		t.Error("Profiles gave no error after Stop")
	}
	if _, err := e.Values(); err == nil {
		t.Error("Values gave no error after Stop")
	}
	if _, err := e.ProfileValues(1); err == nil {
		t.Error("ProfileValues gave no error after Stop")
	}
	if _, err := e.ProfileIdsForPropertyValue("HardwareModel", "iPhone"); err == nil {
		t.Error("ProfileIdsForPropertyValue gave no error after Stop")
	}
}
