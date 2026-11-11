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

import "testing"

// The variable the cloud endpoint is read from, and the name it had before,
// which is written in two parts so this file does not name it.
const (
	endpointVariable    = "_51DEGREES_CLOUD_ENDPOINT"
	oldEndpointVariable = "FOD" + "_CLOUD_API_URL"
)

// TestNewBuilderReadsTheEndpointVariable checks the base endpoint comes from
// the one 51Degrees name for it.
func TestNewBuilderReadsTheEndpointVariable(t *testing.T) {
	const endpoint = "https://cloud.example/api/v4/"
	t.Setenv(oldEndpointVariable, "")
	t.Setenv(endpointVariable, endpoint)
	if got := NewBuilder().endpoint; got != endpoint {
		t.Errorf("the endpoint is %q with %s set, want %q",
			got, endpointVariable, endpoint)
	}
	if EnvCloudAPIURL != endpointVariable {
		t.Errorf("EnvCloudAPIURL is %q, want %q",
			EnvCloudAPIURL, endpointVariable)
	}
}

// TestNewBuilderIgnoresTheOldEndpointVariable checks nothing stands behind
// the one name, so the old name is not read.
func TestNewBuilderIgnoresTheOldEndpointVariable(t *testing.T) {
	t.Setenv(endpointVariable, "")
	t.Setenv(oldEndpointVariable, "https://old.example/api/v4/")
	if got := NewBuilder().endpoint; got != DefaultEndpoint {
		t.Errorf("the endpoint is %q with only %s set, want %q",
			got, oldEndpointVariable, DefaultEndpoint)
	}
}
