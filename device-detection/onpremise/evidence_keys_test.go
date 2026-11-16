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
	"reflect"
	"sort"
	"testing"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
)

// engineKeyNames gives the keys as sorted text, so two lists can be compared.
func engineKeyNames(keys []dd.EvidenceKey) []string {
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = fmt.Sprintf("%d.%s", k.Prefix, k.Key)
	}
	sort.Strings(names)
	return names
}

// An Engine used on its own, outside a Pipeline, gives the Evidence keys the
// data in use accepts, being the list the native getKeys function gives. They
// are worked out when the Engine starts and again each time it refreshes its
// data, whether it loads a temporary copy or the file itself. A list that no
// longer matches the data, here a placeholder, is replaced by the data's own
// list on each refresh.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L78-L86
func TestEngineEvidenceKeysAreWorkedOutAtStartAndOnEachRefresh(t *testing.T) {
	special := fmt.Sprintf("%d.%s", dd.HttpEvidenceQuery, "51D_ProfileIds")
	for _, temp := range []bool{true, false} {
		t.Run(fmt.Sprintf("temporary copy %v", temp), func(t *testing.T) {
			e := newReloadTestEngine(t,
				WithDataFile(writeDataFile(t, readGoodFile(t))),
				WithAutoUpdate(false), WithFileWatch(false),
				WithTempDataCopy(temp))

			want := engineKeyNames(e.GetHttpHeaderKeys())
			if i := sort.SearchStrings(want, special); i == len(want) ||
				want[i] != special {
				t.Fatalf("at start the Engine gives the keys %v, which are "+
					"not the data's own list, as %s is not among them",
					want, special)
			}

			for i := 0; i < 2; i++ {
				e.manager.HttpHeaderKeys = []dd.EvidenceKey{
					{Prefix: dd.HttpHeaderString, Key: "placeholder"}}
				if err := e.RefreshData(); err != nil {
					t.Fatalf("refresh %d failed: %v", i+1, err)
				}
				if got := engineKeyNames(e.GetHttpHeaderKeys()); !reflect.
					DeepEqual(got, want) {
					t.Errorf("after refresh %d the Evidence keys are %v, "+
						"want the data's %v", i+1, got, want)
				}
			}
		})
	}
}
