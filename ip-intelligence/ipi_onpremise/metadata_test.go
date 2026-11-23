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

package ipi_onpremise

import (
	"testing"
	"time"
)

// The engine gives the metadata of every Property and component of its
// data file, with those it was made with available, and the date the data
// file was published.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/required-examples.md?plain=1#L167-L173
func TestEngineGivesItsMetadata(t *testing.T) {
	e := newReloadTestEngine(t,
		WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithAutoUpdate(false),
		WithFileWatch(false),
		WithProperties([]string{"AsnName"}),
	)
	all, err := e.Properties()
	if err != nil {
		t.Fatalf("Properties failed: %v", err)
	}
	available := 0
	for _, m := range all {
		if m.Available {
			available++
			if m.Name != "AsnName" {
				t.Errorf("%s is available, want only AsnName", m.Name)
			}
		}
	}
	if available != 1 || len(all) < 2 {
		t.Errorf("the engine gave %d Properties, %d available, want every "+
			"Property of the data file with AsnName alone available",
			len(all), available)
	}
	components, err := e.Components()
	if err != nil {
		t.Fatalf("Components failed: %v", err)
	}
	if len(components) == 0 {
		t.Error("the engine gave no components")
	}
	if published := e.PublishedDate(); published.IsZero() ||
		published.After(time.Now()) {
		t.Errorf("the data file was published %v, want a date before now",
			published)
	}
}
