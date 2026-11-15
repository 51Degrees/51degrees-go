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
	"testing"
)

// metadataManager is a manager made from the Asn data file with the
// properties named, freed when the test ends.
func metadataManager(t *testing.T, properties string) *ResourceManager {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.ipi")
	writeReloadTestFile(t, path, readReloadTestFile(t))
	manager := NewResourceManager()
	if err := InitManagerFromFile(manager, *NewConfigIpi(InMemory),
		properties, path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(manager.Free)
	return manager
}

// The manager gives the metadata of every Property the data file defines,
// those it was not made with included, each with its category, description,
// type, the data tiers that hold it, and whether it is available.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/required-examples.md?plain=1#L167-L173
func TestPropertiesGiveEachPropertysMetadata(t *testing.T) {
	every := GetAvailablePropertyNames(metadataManager(t, ""))
	all, err := metadataManager(t, "AsnName").Properties()
	if err != nil {
		t.Fatalf("Properties failed: %v", err)
	}
	if len(all) < len(every) || len(every) == 0 {
		t.Fatalf("Properties gave %d, want every one of the data file's %d",
			len(all), len(every))
	}
	byName := make(map[string]PropertyMetadata, len(all))
	for _, m := range all {
		byName[m.Name] = m
	}
	for _, c := range []struct {
		name      string
		available bool
	}{
		{"AsnName", true},
		{"Asn", false},
	} {
		m, ok := byName[c.name]
		if !ok {
			t.Errorf("no metadata for %s", c.name)
			continue
		}
		if m.Category == "" || m.Description == "" {
			t.Errorf("%s has category %q and description %q, want both",
				c.name, m.Category, m.Description)
		}
		if m.Type < StringValueType || m.Type > WeightedWkbRValueType {
			t.Errorf("%s has type %d, which is no value type", c.name,
				m.Type)
		}
		if len(m.DataTiers) == 0 {
			t.Errorf("%s names no data tier that holds it", c.name)
		}
		if m.Available != c.available {
			t.Errorf("%s available is %v, want %v", c.name, m.Available,
				c.available)
		}
	}
}

// The manager gives every component of the data file, each with its name
// and the names of its Properties, and every Property of the data file
// belongs to the one component its metadata names.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/required-examples.md?plain=1#L167-L173
func TestComponentsGiveEachComponent(t *testing.T) {
	manager := metadataManager(t, "")
	components, err := manager.Components()
	if err != nil {
		t.Fatalf("Components failed: %v", err)
	}
	if len(components) == 0 {
		t.Fatal("Components gave none")
	}
	all, err := manager.Properties()
	if err != nil {
		t.Fatalf("Properties failed: %v", err)
	}
	componentOf := make(map[string]uint8, len(all))
	for _, m := range all {
		componentOf[m.Name] = m.ComponentId
	}
	seen := make(map[string]int)
	for _, c := range components {
		if c.Name == "" {
			t.Errorf("component %d has no name", c.Id)
		}
		for _, name := range c.Properties {
			seen[name]++
			if id, ok := componentOf[name]; !ok || id != c.Id {
				t.Errorf("%s names %s, whose metadata names component %d",
					c.Name, name, id)
			}
		}
	}
	for name := range componentOf {
		if seen[name] != 1 {
			t.Errorf("%s is in %d components, want 1", name, seen[name])
		}
	}
}
