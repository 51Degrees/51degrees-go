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
	"strings"
	"testing"
)

// specialEvidenceKeys are the keys a data set accepts, beyond those of its
// headers and overridable Properties, when it processes special Evidence, as
// a data set does unless told not to.
var specialEvidenceKeys = []EvidenceKey{
	{HttpEvidenceQuery, "51D_deviceId"},
	{HttpEvidenceQuery, "51D_gethighentropyvalues"},
	{HttpEvidenceQuery, "51D_structureduseragent"},
	{HttpEvidenceCookie, "51D_gethighentropyvalues"},
	{HttpEvidenceCookie, "51D_structureduseragent"},
	{HttpEvidenceQuery, "51D_ProfileIds"},
	{HttpEvidenceCookie, "51D_ProfileIds"},
}

// The accepted Evidence keys are the list the native getKeys function gives
// for the data. So they hold the keys of the special Evidence the data set
// reads, name no pseudo header, which no request carries, and give each key
// once.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L78-L86
func TestEvidenceKeysAreTheDataSetsOwnList(t *testing.T) {
	manager := initResourceManager("", t)
	defer freeResourceManager(manager, t)
	keys := manager.HeaderKeys()

	listed := make(map[string]int, len(keys))
	for _, k := range keys {
		name := fmt.Sprintf("%d.%s", k.Prefix, k.Key)
		listed[name]++
		if listed[name] == 2 {
			t.Errorf("the key %s is listed more than once", name)
		}
		if strings.ContainsRune(k.Key, '\x1f') {
			t.Errorf("the key %q names a pseudo header", k.Key)
		}
	}
	for _, k := range specialEvidenceKeys {
		if listed[fmt.Sprintf("%d.%s", k.Prefix, k.Key)] == 0 {
			t.Errorf("the special Evidence key %d.%s is not listed",
				k.Prefix, k.Key)
		}
	}
}

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
