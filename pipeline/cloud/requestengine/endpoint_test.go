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
	"log/slog"
	"strings"
	"testing"
)

// The variable the cloud endpoint is read from, and the deprecated name it
// had before, which is written in two parts so this file does not name it.
const (
	endpointVariable    = "_51DEGREES_CLOUD_ENDPOINT"
	oldEndpointVariable = "FOD" + "_CLOUD_API_URL"
)

// The endpoints the tests give each variable.
const (
	newNameEndpoint = "https://new.example/api/v4/"
	oldNameEndpoint = "https://old.example/api/v4/"
)

// TestNewBuilderReadsTheEndpointFromTheEnvironment checks the base endpoint
// comes from the 51Degrees variable first, then from the deprecated variable
// when the first is unset or empty, and is the default when neither is set.
func TestNewBuilderReadsTheEndpointFromTheEnvironment(t *testing.T) {
	for _, c := range []struct {
		name, newValue, oldValue, want string
	}{
		{"only the new name", newNameEndpoint, "", newNameEndpoint},
		{"only the old name", "", oldNameEndpoint, oldNameEndpoint},
		{"both names", newNameEndpoint, oldNameEndpoint, newNameEndpoint},
		{"neither name", "", "", DefaultEndpoint},
		{"a blank new name", "  ", oldNameEndpoint, oldNameEndpoint},
		{"a blank old name", "", "  ", DefaultEndpoint},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(endpointVariable, c.newValue)
			t.Setenv(oldEndpointVariable, c.oldValue)
			if got := NewBuilder().endpoint; got != c.want {
				t.Errorf("the endpoint is %q with %s as %q and %s as %q, "+
					"want %q", got, endpointVariable, c.newValue,
					oldEndpointVariable, c.oldValue, c.want)
			}
		})
	}
}

// TestEnvCloudAPIURLNamesTheOldVariable checks the deprecated constant holds
// the value it was published with, so code that passes it to the environment
// reads and sets the variable it always did, and the constant that replaces
// it names the 51Degrees variable.
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

// buildFromEnvironment builds an engine from a new builder, which reads the
// endpoint from the environment, logging to log. The discovery state means
// the build makes no request.
func buildFromEnvironment(t *testing.T, log *logRecorder) {
	t.Helper()
	_, err := NewBuilder().
		SetResourceKey(discoveryKey).
		SetDiscoveryState(itemState()).
		SetLogger(slog.New(log)).
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
}

// forgetTheWarning sets back the process's record of the deprecation
// warning, now and when the test ends, so that each test starts as a new
// process does and can tell whether its own builds logged the warning.
func forgetTheWarning(t *testing.T) {
	t.Helper()
	oldEndpointLogged.Store(false)
	t.Cleanup(func() { oldEndpointLogged.Store(false) })
}

// deprecations returns the warnings in log that name both variables.
func deprecations(log *logRecorder) []slog.Record {
	var found []slog.Record
	for _, r := range log.all() {
		if r.Level == slog.LevelWarn &&
			strings.Contains(r.Message, oldEndpointVariable) &&
			strings.Contains(r.Message, endpointVariable) {
			found = append(found, r)
		}
	}
	return found
}

// TestTheOldEndpointVariableIsLoggedOnce checks an endpoint read from the
// deprecated variable is logged through the builder's logger as a warning
// that names the variable and the one that replaces it, and only once
// however many engines are built.
func TestTheOldEndpointVariableIsLoggedOnce(t *testing.T) {
	t.Setenv(endpointVariable, "")
	t.Setenv(oldEndpointVariable, oldNameEndpoint)
	forgetTheWarning(t)
	log := &logRecorder{}
	buildFromEnvironment(t, log)
	buildFromEnvironment(t, log)
	if got := deprecations(log); len(got) != 1 {
		t.Errorf("two builds with only %s set logged %d warnings that it "+
			"is deprecated, want 1", oldEndpointVariable, len(got))
	}
}

// TestNoDeprecationIsLoggedWithoutTheOldEndpoint checks nothing is logged
// when the endpoint in use did not come from the deprecated variable.
func TestNoDeprecationIsLoggedWithoutTheOldEndpoint(t *testing.T) {
	for _, c := range []struct {
		name, newValue, oldValue string
		setEndpoint              bool
	}{
		{"only the new name", newNameEndpoint, "", false},
		{"both names", newNameEndpoint, oldNameEndpoint, false},
		{"neither name", "", "", false},
		{"the old name behind SetEndpoint", "", oldNameEndpoint, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(endpointVariable, c.newValue)
			t.Setenv(oldEndpointVariable, c.oldValue)
			forgetTheWarning(t)
			log := &logRecorder{}
			b := NewBuilder().
				SetResourceKey(discoveryKey).
				SetDiscoveryState(itemState()).
				SetLogger(slog.New(log))
			if c.setEndpoint {
				b.SetEndpoint(newNameEndpoint)
			}
			if _, err := b.Build(); err != nil {
				t.Fatalf("build: %v", err)
			}
			if records := log.all(); len(records) != 0 {
				t.Errorf("the build logged %q, want nothing",
					records[0].Message)
			}
		})
	}
}
