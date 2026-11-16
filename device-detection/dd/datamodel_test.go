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
	"strconv"
	"strings"
	"testing"
)

// matchedProfileIds gives the profile ids in the device id the manager gives
// for the mobile User-Agent, which is an iPhone on iOS, one for each
// component in component order, hardware first.
func matchedProfileIds(t *testing.T, manager *ResourceManager) []uint32 {
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
	var ids []uint32
	for _, part := range strings.Split(deviceId, "-") {
		id, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			t.Fatalf("the device id %q holds %q, not a profile id: %v",
				deviceId, part, err)
		}
		ids = append(ids, uint32(id))
	}
	if len(ids) < 2 || ids[0] == 0 || ids[1] == 0 {
		t.Fatalf("the device id %q has no hardware and platform profiles",
			deviceId)
	}
	return ids
}

// componentOfEachProfile gives the component id of each profile by its id,
// failing the test on an error.
func componentOfEachProfile(t *testing.T,
	manager *ResourceManager) map[uint32]uint8 {
	t.Helper()
	profiles, err := manager.Profiles()
	if err != nil {
		t.Fatalf("Profiles failed: %v", err)
	}
	byId := make(map[uint32]uint8, len(profiles))
	for _, p := range profiles {
		byId[p.Id] = p.ComponentId
	}
	return byId
}

// componentOfEachProperty gives the component id of each Property by its
// name, failing the test on an error.
func componentOfEachProperty(t *testing.T,
	manager *ResourceManager) map[string]uint8 {
	t.Helper()
	all, err := manager.Properties()
	if err != nil {
		t.Fatalf("Properties failed: %v", err)
	}
	byName := make(map[string]uint8, len(all))
	for _, m := range all {
		byName[m.Name] = m.ComponentId
	}
	return byName
}

// holds says whether the values hold the value of the Property.
func holds(values []ValueMetadata, property, value string) bool {
	for _, v := range values {
		if v.Property == property && v.Name == value {
			return true
		}
	}
	return false
}

// The manager gives every component of the data file, each with its name,
// the names of its Properties and a default profile of the component, and
// every Property of the data file belongs to the one component its metadata
// names.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/data-model-specification/README.md?plain=1#L16-L31
func TestComponentsGiveEachComponent(t *testing.T) {
	manager := initResourceManager("", t)
	defer freeResourceManager(manager, t)
	components, err := manager.Components()
	if err != nil {
		t.Fatalf("Components failed: %v", err)
	}
	if len(components) == 0 {
		t.Fatal("Components gave none")
	}
	profiles := componentOfEachProfile(t, manager)
	properties := componentOfEachProperty(t, manager)

	seen := make(map[string]int)
	byName := make(map[string]ComponentMetadata)
	for _, c := range components {
		byName[c.Name] = c
		if c.Name == "" {
			t.Errorf("component %d has no name", c.Id)
		}
		if id, ok := profiles[c.DefaultProfileId]; !ok || id != c.Id {
			t.Errorf("%s's default profile %d is not a profile of the "+
				"component", c.Name, c.DefaultProfileId)
		}
		for _, name := range c.Properties {
			seen[name]++
			if id, ok := properties[name]; !ok || id != c.Id {
				t.Errorf("%s names %s, whose metadata names component %d",
					c.Name, name, id)
			}
		}
	}
	for name := range properties {
		if seen[name] != 1 {
			t.Errorf("%s is in %d components, want 1", name, seen[name])
		}
	}
	for _, c := range []struct{ component, property string }{
		{"HardwarePlatform", "IsMobile"},
		{"SoftwarePlatform", "PlatformName"},
	} {
		found := false
		for _, name := range byName[c.component].Properties {
			found = found || name == c.property
		}
		if !found {
			t.Errorf("no component %s holding %s", c.component, c.property)
		}
	}
}

// The manager gives every profile of the data file once, in order of id,
// each of a component the data file defines, the profiles a detection
// matched among them.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/data-model-specification/README.md?plain=1#L60-L70
func TestProfilesGiveEachProfile(t *testing.T) {
	manager := initResourceManager("", t)
	defer freeResourceManager(manager, t)
	profiles, err := manager.Profiles()
	if err != nil {
		t.Fatalf("Profiles failed: %v", err)
	}
	if len(profiles) == 0 {
		t.Fatal("Profiles gave none")
	}
	components, err := manager.Components()
	if err != nil {
		t.Fatalf("Components failed: %v", err)
	}
	componentIds := make(map[uint8]bool)
	for _, c := range components {
		componentIds[c.Id] = true
	}
	for i, p := range profiles {
		if i > 0 && p.Id <= profiles[i-1].Id {
			t.Fatalf("profile %d is %d after %d, want each id once in "+
				"ascending order", i, p.Id, profiles[i-1].Id)
		}
		if !componentIds[p.ComponentId] {
			t.Fatalf("profile %d names component %d, which the data file "+
				"does not define", p.Id, p.ComponentId)
		}
	}
	byId := componentOfEachProfile(t, manager)
	matched := make(map[uint8]bool)
	for _, id := range matchedProfileIds(t, manager) {
		if id == 0 {
			continue
		}
		component, ok := byId[id]
		if !ok {
			t.Errorf("the matched profile %d is not among the profiles", id)
		}
		if matched[component] {
			t.Errorf("two matched profiles are of component %d", component)
		}
		matched[component] = true
	}
}

// The manager gives every value of every Property, each naming its
// Property, with at most one default for a Property.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/data-model-specification/README.md?plain=1#L72-L82
func TestValuesGiveEachValue(t *testing.T) {
	manager := initResourceManager("", t)
	defer freeResourceManager(manager, t)
	values, err := manager.Values()
	if err != nil {
		t.Fatalf("Values failed: %v", err)
	}
	for _, c := range [][2]string{
		{"IsMobile", "True"},
		{"IsMobile", "False"},
		{"PlatformName", "iOS"},
	} {
		if !holds(values, c[0], c[1]) {
			t.Errorf("Values gave no %s %s", c[0], c[1])
		}
	}
	properties := componentOfEachProperty(t, manager)
	defaults := make(map[string]int)
	for _, v := range values {
		if _, ok := properties[v.Property]; !ok {
			t.Fatalf("the value %q names %q, which is no Property of the "+
				"data file", v.Name, v.Property)
		}
		if v.IsDefault {
			defaults[v.Property]++
		}
	}
	if len(defaults) == 0 {
		t.Error("no value is a Property's default")
	}
	for name, n := range defaults {
		if n > 1 {
			t.Errorf("%s has %d default values, want 1", name, n)
		}
	}
}

// The manager gives the values a profile holds, each of a Property of the
// profile's component, and an error for a profile the data file does not
// hold.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/data-model-specification/README.md?plain=1#L60-L70
func TestProfileValuesGiveTheProfilesValues(t *testing.T) {
	manager := initResourceManager("", t)
	defer freeResourceManager(manager, t)
	ids := matchedProfileIds(t, manager)
	profiles := componentOfEachProfile(t, manager)
	properties := componentOfEachProperty(t, manager)
	for _, c := range []struct {
		profile         uint32
		property, value string
	}{
		{ids[0], "IsMobile", "True"},
		{ids[1], "PlatformName", "iOS"},
	} {
		values, err := manager.ProfileValues(c.profile)
		if err != nil {
			t.Fatalf("ProfileValues(%d) failed: %v", c.profile, err)
		}
		if !holds(values, c.property, c.value) {
			t.Errorf("profile %d gave %d values and not %s %s", c.profile,
				len(values), c.property, c.value)
		}
		for _, v := range values {
			if properties[v.Property] != profiles[c.profile] {
				t.Errorf("profile %d of component %d gave %s %s, a Property "+
					"of component %d", c.profile, profiles[c.profile],
					v.Property, v.Name, properties[v.Property])
			}
		}
	}
	if _, err := manager.ProfileValues(^uint32(0)); err == nil {
		t.Error("ProfileValues gave no error for a profile the data file " +
			"does not hold")
	}
}
