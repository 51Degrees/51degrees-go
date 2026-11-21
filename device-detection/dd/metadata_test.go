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

// metadataByName gives the manager's Property metadata keyed by name.
func metadataByName(t *testing.T,
	manager *ResourceManager) map[string]PropertyMetadata {
	t.Helper()
	all, err := manager.Properties()
	if err != nil {
		t.Fatalf("Properties failed: %v", err)
	}
	byName := make(map[string]PropertyMetadata, len(all))
	for _, m := range all {
		byName[m.Name] = m
	}
	return byName
}

// The manager gives the metadata of every Property the data file defines,
// those it was not made with included, each with its category,
// description, type, the data tiers that hold it, and whether it is
// available.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/usage-examples.md?plain=1#L148-L151
func TestPropertiesGiveEachPropertysMetadata(t *testing.T) {
	all := initResourceManager("", t)
	defer freeResourceManager(all, t)
	everyName := NewResultsHash(all, 1, 0)
	names := everyName.AvailableProperties()
	everyName.Free()

	manager := initResourceManager("IsMobile,BrowserName", t)
	defer freeResourceManager(manager, t)
	byName := metadataByName(t, manager)
	if len(byName) < len(names) || len(names) == 0 {
		t.Fatalf("Properties gave %d, want every one of the data file's %d",
			len(byName), len(names))
	}

	for _, c := range []struct {
		name      string
		goType    reflect.Type
		available bool
	}{
		{"IsMobile", reflect.TypeOf(false), true},
		{"BrowserName", reflect.TypeOf(""), true},
		{"PlatformName", reflect.TypeOf(""), false},
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
		if m.Type != c.goType {
			t.Errorf("%s is read as %v, want %v", c.name, m.Type, c.goType)
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

// The metadata gives each Property's display order, whether it is mandatory,
// and whether it and its values are shown, as the data file holds them. The
// data file shows some Properties and the values of some, in more than one
// display order.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/data-model-specification/README.md?plain=1#L55-L58
func TestPropertiesGiveTheDisplayFields(t *testing.T) {
	manager := initResourceManager("", t)
	defer freeResourceManager(manager, t)
	all, err := manager.Properties()
	if err != nil {
		t.Fatalf("Properties failed: %v", err)
	}
	shown, valuesShown, mandatory := 0, 0, 0
	orders := make(map[uint8]bool)
	for _, m := range all {
		if m.Show {
			shown++
		}
		if m.ShowValues {
			valuesShown++
		}
		if m.IsMandatory {
			mandatory++
		}
		orders[m.DisplayOrder] = true
	}
	t.Logf("%d Properties, %d shown, %d with values shown, %d mandatory, "+
		"%d display orders", len(all), shown, valuesShown, mandatory,
		len(orders))
	if shown == 0 || valuesShown == 0 || len(orders) < 2 {
		t.Errorf("of %d Properties %d are shown, %d have their values "+
			"shown and there are %d display orders, want some shown, some "+
			"with values shown and more than one order", len(all), shown,
			valuesShown, len(orders))
	}
}
