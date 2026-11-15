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

import "testing"

// The engine gives the metadata of every Property of its data file, with
// those it was made with available.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/usage-examples.md?plain=1#L148-L151
func TestEngineGivesPropertyMetadata(t *testing.T) {
	engine, err := New(
		WithDataFile("../51Degrees-LiteV4.1.hash"),
		WithAutoUpdate(false),
		WithProperties([]string{"IsMobile"}),
	)
	if err != nil {
		t.Fatalf("Error creating engine: %v", err)
	}
	defer engine.Stop()
	all, err := engine.Properties()
	if err != nil {
		t.Fatalf("Properties failed: %v", err)
	}
	available := 0
	for _, m := range all {
		if m.Available {
			available++
			if m.Name != "IsMobile" {
				t.Errorf("%s is available, want only IsMobile", m.Name)
			}
		}
	}
	if available != 1 || len(all) < 2 {
		t.Errorf("the engine gave %d Properties, %d available, want every "+
			"Property of the data file with IsMobile alone available",
			len(all), available)
	}
}
