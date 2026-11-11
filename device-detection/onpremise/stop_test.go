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
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Stop does not wait for a download in progress, which for a large data file
// can take many minutes.
func TestStopDoesNotWaitForADownload(t *testing.T) {
	const startTimeoutSeconds = 10
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		once.Do(func() { close(started) })
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithDataUpdateUrl(server.URL),
		WithPollingInterval(1),
		WithRandomization(0),
		WithFileWatch(false))
	select {
	case <-started:
	case <-time.After(startTimeoutSeconds * time.Second):
		t.Fatal("the download did not start")
	}
	e.stop(t)
}
