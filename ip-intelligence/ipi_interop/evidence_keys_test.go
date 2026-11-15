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

package ipi_interop

import (
	"path/filepath"
	"reflect"
	"testing"
)

// keysManager is a manager made from the Asn data file, freed when the test
// ends.
func keysManager(t *testing.T) (*ResourceManager, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.ipi")
	writeReloadTestFile(t, path, readReloadTestFile(t))
	manager := NewResourceManager()
	if err := InitManagerFromFile(manager, *NewConfigIpi(InMemory), "",
		path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Free)
	return manager, path
}

// The accepted Evidence keys are the data set's, worked out when the data is
// loaded, in the order the native code tries them. That is a query key for
// each header the data set reads the client IP address from, in the data
// set's order, then a server key for each of the same headers in the same
// order.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L75-L84
func TestEvidenceKeysAreTheDataSetsInTheOrderTried(t *testing.T) {
	manager, _ := keysManager(t)
	keys := manager.HeaderKeys()
	if len(keys) == 0 || len(keys)%2 != 0 {
		t.Fatalf("the data set gave %d Evidence keys, want a query and a "+
			"server key for each header: %v", len(keys), keys)
	}
	headers := len(keys) / 2
	for i := 0; i < headers; i++ {
		query, server := keys[i], keys[headers+i]
		if query.Prefix != HttpEvidenceQuery ||
			server.Prefix != HttpEvidenceServer {
			t.Errorf("keys %d and %d are %v and %v, want every query key "+
				"before every server key", i, headers+i, query, server)
		}
		if query.Key == "" || query.Key != server.Key {
			t.Errorf("keys %d and %d are %v and %v, want the same header "+
				"in the same place in both halves", i, headers+i, query,
				server)
		}
	}
}

// The accepted Evidence keys are worked out again whenever the data is
// refreshed. A list that no longer matches the data, here a placeholder, is
// replaced by the data's own list on each reload.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L75-L84
func TestEvidenceKeysAreWorkedOutAgainOnEachRefresh(t *testing.T) {
	manager, path := keysManager(t)
	want := manager.HeaderKeys()
	placeholder := []EvidenceKey{{Key: "placeholder"}}

	for _, reload := range []struct {
		name string
		run  func() error
	}{
		{"ReloadFromOriginalFile", manager.ReloadFromOriginalFile},
		{"ReloadFromFile", func() error {
			return manager.ReloadFromFile(*NewConfigIpi(InMemory), "", path)
		}},
	} {
		manager.HttpHeaderKeys = placeholder
		if err := reload.run(); err != nil {
			t.Fatalf("%s failed: %v", reload.name, err)
		}
		if got := manager.HeaderKeys(); len(got) == 0 ||
			!reflect.DeepEqual(got, want) {
			t.Errorf("after %s the Evidence keys are %v, want the data's %v",
				reload.name, got, want)
		}
	}
}
