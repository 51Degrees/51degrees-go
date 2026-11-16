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
	"reflect"
	"testing"
)

// The table from each Property name to its position is built once, when the
// manager is created, and each reload keeps it rather than building another
// from the new data set. Results made after a reload read through it too.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L209-L213
func TestThePropertyTableIsBuiltOnce(t *testing.T) {
	manager := initResourceManager("", t)
	defer freeResourceManager(manager, t)
	table := reflect.ValueOf(manager.indexes).Pointer()

	for _, reload := range []struct {
		name string
		run  func() error
	}{
		{"ReloadFromOriginalFile", manager.ReloadFromOriginalFile},
		{"ReloadFromFile", func() error {
			return manager.ReloadFromFile(testDataFilePath)
		}},
	} {
		if err := reload.run(); err != nil {
			t.Fatalf("%s failed: %v", reload.name, err)
		}
		if reflect.ValueOf(manager.indexes).Pointer() != table {
			t.Errorf("%s built the Property table again, want the one "+
				"built when the manager was created", reload.name)
		}
	}

	results := NewResultsHash(manager, 1, 0)
	defer results.Free()
	if reflect.ValueOf(results.indexes).Pointer() != table {
		t.Error("results made after the reloads read through another " +
			"Property table, want the one built when the manager was created")
	}
}
