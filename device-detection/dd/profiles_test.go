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
	"strconv"
	"strings"
	"testing"
)

// platformProfileId gives the platform profile in the device id the manager
// gives for the mobile User-Agent, which is an iPhone on iOS. The device id
// holds a profile of each component in component order, hardware first.
func platformProfileId(t *testing.T, manager *ResourceManager) uint32 {
	t.Helper()
	results := NewResultsHash(manager, 1, 0)
	defer results.Free()
	if err := results.MatchUserAgent(uaMobile); err != nil {
		t.Fatalf("detection failed: %v", err)
	}
	deviceId, err := results.DeviceId()
	if err != nil {
		t.Fatalf("reading the device id failed: %v", err)
	}
	parts := strings.Split(deviceId, "-")
	if len(parts) < 2 {
		t.Fatalf("the device id %q has no platform profile", deviceId)
	}
	id, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		t.Fatalf("the device id %q has no platform profile: %v", deviceId, err)
	}
	return uint32(id)
}

// profileIds gives the ids for the value, failing the test on an error.
func profileIds(t *testing.T, manager *ResourceManager,
	property, value string) []uint32 {
	t.Helper()
	ids, err := manager.ProfileIdsForPropertyValue(property, value)
	if err != nil {
		t.Fatalf("ProfileIdsForPropertyValue(%q, %q) failed: %v", property,
			value, err)
	}
	return ids
}

// The manager gives the ids of the profiles holding a value of a Property,
// in ascending id with each once, and they include the platform profile a
// detection of an iPhone found for PlatformName iOS.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L477-L499
func TestProfileIdsForPropertyValue(t *testing.T) {
	manager := initResourceManager("", t)
	defer freeResourceManager(manager, t)
	ids := profileIds(t, manager, "PlatformName", "iOS")
	if len(ids) == 0 {
		t.Fatal("no profile holds PlatformName iOS")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Fatalf("id %d is %d after %d, want each id once in ascending "+
				"order", i, ids[i], ids[i-1])
		}
	}
	platform := platformProfileId(t, manager)
	found := false
	for _, id := range ids {
		found = found || id == platform
	}
	if !found {
		t.Errorf("the ids %v do not hold %d, the platform profile of an "+
			"iPhone", ids, platform)
	}
}

// The Property name and the value are matched exactly, so either in another
// case finds no profile.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L485-L488
func TestProfileIdsMatchExactly(t *testing.T) {
	manager := initResourceManager("", t)
	defer freeResourceManager(manager, t)
	if len(profileIds(t, manager, "PlatformName", "iOS")) == 0 {
		t.Fatal("no profile holds PlatformName iOS")
	}
	for _, c := range [][2]string{
		{"PlatformName", "ios"},
		{"platformname", "iOS"},
	} {
		if ids := profileIds(t, manager, c[0], c[1]); len(ids) != 0 {
			t.Errorf("%s %s found %d profiles, want none, as names and "+
				"values are matched exactly", c[0], c[1], len(ids))
		}
	}
}

// A manager that indexes a Property gives the same ids for it as one that
// reads every profile, and keeps the index across a reload.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L491-L494
func TestIndexedProfileIdsMatchTheFullRead(t *testing.T) {
	plain := initResourceManager("", t)
	defer freeResourceManager(plain, t)
	want := profileIds(t, plain, "PlatformName", "iOS")
	if len(want) == 0 {
		t.Fatal("no profile holds PlatformName iOS")
	}

	indexed := NewResourceManager()
	if err := InitManagerFromFileWithIndexedProperties(indexed,
		*NewConfigHash(Balanced), "", testDataFilePath,
		[]string{"PlatformName"}); err != nil {
		t.Fatalf("InitManagerFromFileWithIndexedProperties failed: %v", err)
	}
	defer indexed.Free()
	if got := profileIds(t, indexed, "PlatformName", "iOS"); !reflect.DeepEqual(
		got, want) {
		t.Errorf("the indexed manager gave %d ids, want the %d of a full "+
			"read", len(got), len(want))
	}
	if err := indexed.ReloadFromOriginalFile(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if got := profileIds(t, indexed, "PlatformName", "iOS"); !reflect.DeepEqual(
		got, want) {
		t.Errorf("after a reload the indexed manager gave %d ids, want %d",
			len(got), len(want))
	}
}
