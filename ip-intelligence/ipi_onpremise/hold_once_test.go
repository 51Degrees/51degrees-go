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

package ipi_onpremise

import (
	"testing"
	"time"
)

// A Data is made while its caller holds the engine, so making one must not
// ask for a second hold. A sync.RWMutex blocks a second read lock while Stop
// waits for the write lock, so a second hold would never be given.
func TestMakingADataTakesNoSecondHoldOnTheEngine(t *testing.T) {
	e := &Engine{}
	if !e.acquire() {
		t.Fatal("a new engine refused a hold")
	}
	stopped := make(chan struct{})
	go func() {
		e.useMu.Lock()
		e.useMu.Unlock()
		close(stopped)
	}()
	// Give the writer time to start waiting, as Stop does.
	time.Sleep(100 * time.Millisecond)

	made := make(chan struct{})
	go func() {
		e.newData(noResults{}, noAddressReason)
		close(made)
	}()
	select {
	case <-made:
	case <-time.After(5 * time.Second):
		t.Fatal("making a Data waited for a second hold on the engine")
	}
	e.release()
	<-stopped
}
