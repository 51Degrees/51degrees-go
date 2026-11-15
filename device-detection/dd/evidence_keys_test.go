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

package dd

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// keyNames gives the keys as sorted text, so two lists can be compared.
func keyNames(keys []EvidenceKey) []string {
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = fmt.Sprintf("%d.%s", k.Prefix, k.Key)
	}
	sort.Strings(names)
	return names
}

// The accepted Evidence keys are worked out from the data at start-up and
// again whenever the data is refreshed. A list that no longer matches the
// data, here a placeholder, is replaced by the data's own list on each
// reload.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L78-L86
func TestEvidenceKeysAreWorkedOutAgainOnEachRefresh(t *testing.T) {
	manager := initResourceManager("", t)
	defer freeResourceManager(manager, t)
	want := keyNames(manager.HttpHeaderKeys)
	if len(want) == 0 {
		t.Fatal("the data file gave no Evidence keys")
	}
	placeholder := []EvidenceKey{{HttpHeaderString, "placeholder"}}

	for _, reload := range []struct {
		name string
		run  func() error
	}{
		{"ReloadFromOriginalFile", manager.ReloadFromOriginalFile},
		{"ReloadFromFile", func() error {
			return manager.ReloadFromFile(testDataFilePath)
		}},
	} {
		manager.HttpHeaderKeys = placeholder
		if err := reload.run(); err != nil {
			t.Fatalf("%s failed: %v", reload.name, err)
		}
		if got := keyNames(manager.HeaderKeys()); !reflect.DeepEqual(got,
			want) {
			t.Errorf("after %s the Evidence keys are %v, want the data's "+
				"%d keys", reload.name, got, len(want))
		}
	}
}
