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

package requestengine

import (
	"strings"
	"testing"
)

// The error for each combination Build refuses explains why the combination
// cannot work, being the remote server's answer to every request made with
// it, rather than only restating the rule.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/pipeline-elements/cloud-request-engine.md#accepted-combinations
func TestRefusedCombinationsSayWhyTheyCannotWork(t *testing.T) {
	cases := []struct {
		name  string
		build func(*Builder) *Builder
		why   []string
	}{
		{"no credential", func(b *Builder) *Builder { return b },
			[]string{"answers 401", "carries neither"}},
		{"License Key with no Requested Properties",
			func(b *Builder) *Builder { return b.SetLicenseKey(testLicenseKey) },
			[]string{"answers 400", "names no Properties of its own"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cloud := newCredentialCloud()
			_, err := c.build(credentialBuilder(cloud.serve(t),
				&logRecorder{})).Build()
			if err == nil {
				t.Fatal("the combination was not refused")
			}
			for _, why := range c.why {
				if !strings.Contains(err.Error(), why) {
					t.Errorf("the error %q does not say why the "+
						"combination cannot work (%q)", err, why)
				}
			}
		})
	}
}
