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

// countWarnings gives how many records in log hold text, and fails the test
// for one that is not at warning level.
func countWarnings(t *testing.T, log *logRecorder, text string) int {
	t.Helper()
	n := 0
	for _, r := range log.all() {
		if !strings.Contains(recordText(r), text) {
			continue
		}
		n++
		if r.Level != slog.LevelWarn {
			t.Errorf("%q was logged at %s, want %s", text, r.Level,
				slog.LevelWarn)
		}
	}
	return n
}

// Warnings in the answer to the accessible Properties request made at build
// are logged as warnings, once each, as those in the answer to a data
// request are, under a Resource Key (a GET request) and under a License Key
// (a POST request).
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/pipeline-elements/cloud-request-engine.md#http-requests
func TestWarningsInEveryAnswerAreLogged(t *testing.T) {
	const discoveryWarning = "This Resource Key is close to its limit"
	const dataWarning = "This request was slow"
	credentials := []struct {
		name string
		set  func(*Builder) *Builder
	}{
		{"resource key", func(b *Builder) *Builder {
			return b.SetResourceKey(testResourceKey)
		}},
		{"license key", func(b *Builder) *Builder {
			return b.SetLicenseKey(testLicenseKey).
				SetRequestedProperties("device.ismobile")
		}},
	}
	for _, c := range credentials {
		t.Run(c.name, func(t *testing.T) {
			cloud := newCredentialCloud()
			cloud.propertiesBody = `{"Products":{"device":{` +
				`"DataTier":"CloudV4","Properties":[{"Name":"ismobile",` +
				`"Type":"Bool","Category":"Device"}]}},` +
				`"warnings":["` + discoveryWarning + `"]}`
			cloud.jsonBody = `{"device":{"ismobile":true},` +
				`"warnings":["` + dataWarning + `"]}`
			log := &logRecorder{}
			e, err := c.set(credentialBuilder(cloud.serve(t), log)).Build()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if n := countWarnings(t, log, discoveryWarning); n != 1 {
				t.Errorf("the warning in the accessible Properties answer "+
					"was logged %d times when the engine was built, want "+
					"once", n)
			}
			if err := processWith(t, pipelineOf(t, e, false), nil); err != nil {
				t.Fatalf("process: %v", err)
			}
			if n := countWarnings(t, log, dataWarning); n != 1 {
				t.Errorf("the warning in the data answer was logged %d "+
					"times, want once", n)
			}
		})
	}
}
