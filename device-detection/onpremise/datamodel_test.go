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

// The engine gives the components, profiles and values of its data file,
// and the values of a profile.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/usage-examples.md?plain=1#L166-L184
func TestEngineGivesTheDataModel(t *testing.T) {
	engine, err := New(
		WithDataFile("../51Degrees-LiteV4.1.hash"),
		WithAutoUpdate(false),
		WithProperties([]string{"IsMobile"}),
	)
	if err != nil {
		t.Fatalf("Error creating engine: %v", err)
	}
	defer engine.Stop()
	components, err := engine.Components()
	if err != nil {
		t.Fatalf("Components failed: %v", err)
	}
	profiles, err := engine.Profiles()
	if err != nil {
		t.Fatalf("Profiles failed: %v", err)
	}
	values, err := engine.Values()
	if err != nil {
		t.Fatalf("Values failed: %v", err)
	}
	if len(components) == 0 || len(profiles) == 0 || len(values) == 0 {
		t.Fatalf("the engine gave %d components, %d profiles and %d values, "+
			"want some of each", len(components), len(profiles), len(values))
	}
	held, err := engine.ProfileValues(components[0].DefaultProfileId)
	if err != nil {
		t.Fatalf("ProfileValues failed: %v", err)
	}
	if len(held) == 0 {
		t.Errorf("the default profile of %s gave no values",
			components[0].Name)
	}
}
