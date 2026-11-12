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

package dataupdate

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// OnUpdate takes a callback with no arguments and calls it only for a check
// that refreshed the Engine, so a program written against it still builds and
// still hears what it heard, while OnComplete hears every check.
func TestOnUpdateIsCalledOnlyForACheckThatRefreshedTheEngine(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusNotModified)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(int(status.Load()))
		w.Write([]byte("DATA"))
	}))
	defer srv.Close()
	svc := New(nil, srv.Client())
	defer svc.Close()
	if err := svc.Register(&fakeEngine{}, "Eng", Config{Identifier: "a", DataUpdateURL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	updates := 0
	svc.OnUpdate("a", func() { updates++ })
	got := newReports()
	svc.OnComplete("a", got.record)

	if err := svc.CheckForUpdate("a"); err != nil {
		t.Fatal(err)
	}
	if updates != 0 {
		t.Errorf("OnUpdate was called %d times for a check with no newer data, want none", updates)
	}

	status.Store(http.StatusOK)
	if err := svc.CheckForUpdate("a"); err != nil {
		t.Fatal(err)
	}
	if updates != 1 {
		t.Errorf("OnUpdate was called %d times after one refresh, want once", updates)
	}

	if err := svc.UpdateFromMemory("a", []byte("NEW DATA")); err != nil {
		t.Fatal(err)
	}
	if updates != 1 {
		t.Errorf("OnUpdate was called %d times after an update from memory, want it left at one", updates)
	}

	list := got.list()
	if len(list) != 2 || list[0].Status != StatusNotNeeded || list[1].Status != StatusSuccess {
		t.Errorf("OnComplete heard %v, want %q then %q", list, StatusNotNeeded, StatusSuccess)
	}
}
