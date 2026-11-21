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
	"testing"
)

// The variable the cloud endpoint is read from, and the deprecated name it
// had before, which is written in two parts so this file does not name it.
const (
	endpointVariable    = "_51DEGREES_CLOUD_ENDPOINT"
	oldEndpointVariable = "FOD" + "_CLOUD_API_URL"
)

// The endpoint the tests give the variable.
const newNameEndpoint = "https://new.example/api/v4/"

// TestNewBuilderReadsTheEndpointFromTheEnvironment checks the base endpoint
// comes from the 51Degrees variable, and is the default when it is unset or
// blank.
func TestNewBuilderReadsTheEndpointFromTheEnvironment(t *testing.T) {
	for _, c := range []struct {
		name, value, want string
	}{
		{"the name set", newNameEndpoint, newNameEndpoint},
		{"the name unset", "", DefaultEndpoint},
		{"a blank name", "  ", DefaultEndpoint},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(endpointVariable, c.value)
			if got := NewBuilder().endpoint; got != c.want {
				t.Errorf("the endpoint is %q with %s as %q, want %q",
					got, endpointVariable, c.value, c.want)
			}
		})
	}
}

// TestEnvCloudAPIURLNamesTheOldVariable checks the deprecated constant holds
// the value it was published with, and the constant that replaces it names
// the 51Degrees variable.
func TestEnvCloudAPIURLNamesTheOldVariable(t *testing.T) {
	if EnvCloudAPIURL != oldEndpointVariable {
		t.Errorf("EnvCloudAPIURL is %q, want %q",
			EnvCloudAPIURL, oldEndpointVariable)
	}
	if EnvCloudEndpoint != endpointVariable {
		t.Errorf("EnvCloudEndpoint is %q, want %q",
			EnvCloudEndpoint, endpointVariable)
	}
}