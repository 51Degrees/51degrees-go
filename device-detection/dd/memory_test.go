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
	"os"
	"reflect"
	"testing"
	"unsafe"
)

// readTestData returns the bytes of the data file the tests use.
func readTestData(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(testDataFilePath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// overwrite fills data with zeros, so anything still reading it would fail.
func overwrite(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

// memoryManager is a manager made from the test data file held in memory. Its
// caller's copy of the data is overwritten once the manager is made.
func memoryManager(t *testing.T) *ResourceManager {
	t.Helper()
	data := readTestData(t)
	manager := NewResourceManager()
	if err := InitManagerFromMemory(manager, *NewConfigHash(Balanced),
		"IsMobile", unsafe.Pointer(&data[0]), uint64(len(data))); err != nil {
		t.Fatalf("InitManagerFromMemory failed: %v", err)
	}
	t.Cleanup(manager.Free)
	if len(manager.HttpHeaderKeys) == 0 {
		t.Fatal("the manager made from memory gave no Evidence keys, so it " +
			"holds no data")
	}
	overwrite(data)
	return manager
}

// isMobile is the IsMobile value the manager gives for a mobile User-Agent.
func isMobile(t *testing.T, manager *ResourceManager) string {
	t.Helper()
	results := NewResultsHash(manager, 1, 0)
	defer results.Free()
	if err := results.MatchUserAgent(uaMobile); err != nil {
		t.Fatalf("detection failed: %v", err)
	}
	value, err := results.ValuesString("IsMobile", ",")
	if err != nil {
		t.Fatalf("reading IsMobile failed: %v", err)
	}
	return value
}

// A manager can be made from a data file held in memory, and detects with it
// once the caller's memory is reused.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md?plain=1#L212-L213
func TestAManagerMadeFromMemoryDetects(t *testing.T) {
	manager := memoryManager(t)
	if got := isMobile(t, manager); got != "True" {
		t.Errorf("IsMobile is %q, want True", got)
	}
}

// For a manager made from memory, a programmatic update takes the new data in
// memory. The reload loads it, works the accepted Evidence keys out again from
// it, and needs none of the caller's memory once it returns.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md?plain=1#L212-L213
func TestReloadFromBytesLoadsTheNewData(t *testing.T) {
	manager := memoryManager(t)
	want := append([]EvidenceKey(nil), manager.HttpHeaderKeys...)
	manager.HttpHeaderKeys = []EvidenceKey{{HttpHeaderString, "placeholder"}}

	data := readTestData(t)
	if err := manager.ReloadFromBytes(data); err != nil {
		t.Fatalf("ReloadFromBytes failed: %v", err)
	}
	overwrite(data)
	if got := manager.HttpHeaderKeys; !reflect.DeepEqual(got, want) {
		t.Errorf("after ReloadFromBytes the Evidence keys are %v, want the "+
			"data's %d keys", got, len(want))
	}
	if got := isMobile(t, manager); got != "True" {
		t.Errorf("after ReloadFromBytes IsMobile is %q, want True", got)
	}
}

// Only a manager made from memory reloads from memory, and ReloadFromBytes
// with no data, or ReloadFromMemory, which takes none, reloads nothing and
// says so.
func TestReloadsFromMemoryThatCannotWorkAreRefused(t *testing.T) {
	fromFile := initResourceManager("IsMobile", t)
	defer freeResourceManager(fromFile, t)
	if err := fromFile.ReloadFromBytes(readTestData(t)); err == nil {
		t.Error("a manager made from a data file reloaded from memory, " +
			"want an error")
	}

	fromMemory := memoryManager(t)
	if err := fromMemory.ReloadFromBytes(nil); err == nil {
		t.Error("ReloadFromBytes with no data gave no error")
	}
	if err := fromMemory.ReloadFromMemory(); err == nil {
		t.Error("ReloadFromMemory, which takes no data, gave no error")
	}
}
