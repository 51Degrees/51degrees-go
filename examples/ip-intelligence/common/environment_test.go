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

package common

import (
	"os"
	"strings"
	"testing"
)

// The variables the examples read, and the deprecated names they are also
// read from, each of which is written in two parts so that this file does not
// name an old variable.
const (
	licenceVariable  = "_51DEGREES_LICENSE_KEY"
	dataVariable     = "_51DEGREES_IPI_PATH"
	evidenceVariable = "_51DEGREES_EVIDENCE_YAML"
	urlVariable      = "_51DEGREES_IPI_DATA_FILE_URL"
	oldLicence       = "LICENSE" + "_KEY"
	oldIpLicence     = "IPI" + "_KEY"
	oldData          = "DATA" + "_FILE"
	oldEvidence      = "EVIDENCE" + "_YAML"
	oldURL           = "IPI_DATA" + "_FILE_URL"
)

// setVariables gives each variable the examples read the value given for it,
// and makes it empty when none is given, in an empty working folder.
func setVariables(t *testing.T, values map[string]string) {
	t.Helper()
	for _, name := range []string{licenceVariable, dataVariable,
		evidenceVariable, urlVariable, oldLicence, oldIpLicence, oldData,
		oldEvidence, oldURL} {
		t.Setenv(name, values[name])
	}
	inAnEmptyFolder(t)
}

// parameters returns the parameters RunExample gives an example.
func parameters() ExampleParams {
	var got ExampleParams
	RunExample(func(params *ExampleParams) error {
		got = *params
		return nil
	})
	return got
}

// TestRunExampleReadsTheEnvironment checks each parameter comes from its
// 51Degrees variable first, then from the deprecated names in the order they
// were read before, and is the default when none is set.
func TestRunExampleReadsTheEnvironment(t *testing.T) {
	for _, c := range []struct {
		name   string
		values map[string]string
		want   ExampleParams
	}{
		{"only the new names", map[string]string{
			licenceVariable: "new-key", dataVariable: "new.ipi",
			evidenceVariable: "new.yml", urlVariable: "https://new.example/",
		}, ExampleParams{LicenseKey: "new-key", DataFile: "new.ipi",
			EvidenceYaml: "new.yml", DataFileUrl: "https://new.example/"}},
		{"only the old names", map[string]string{
			oldLicence: "old-key", oldData: "old.ipi",
			oldEvidence: "old.yml", oldURL: "https://old.example/",
		}, ExampleParams{LicenseKey: "old-key", DataFile: "old.ipi",
			EvidenceYaml: "old.yml", DataFileUrl: "https://old.example/"}},
		{"only the old IP Intelligence key", map[string]string{
			oldIpLicence: "ip-key",
		}, ExampleParams{LicenseKey: "ip-key", DataFile: LiteDataFile,
			EvidenceYaml: EvidenceFile}},
		{"both old keys", map[string]string{
			oldLicence: "old-key", oldIpLicence: "ip-key",
		}, ExampleParams{LicenseKey: "old-key", DataFile: LiteDataFile,
			EvidenceYaml: EvidenceFile}},
		{"the new and the old names", map[string]string{
			licenceVariable: "new-key", dataVariable: "new.ipi",
			evidenceVariable: "new.yml", urlVariable: "https://new.example/",
			oldLicence: "old-key", oldIpLicence: "ip-key",
			oldData: "old.ipi", oldEvidence: "old.yml",
			oldURL: "https://old.example/",
		}, ExampleParams{LicenseKey: "new-key", DataFile: "new.ipi",
			EvidenceYaml: "new.yml", DataFileUrl: "https://new.example/"}},
		{"no names", nil, ExampleParams{DataFile: LiteDataFile,
			EvidenceYaml: EvidenceFile}},
	} {
		t.Run(c.name, func(t *testing.T) {
			setVariables(t, c.values)
			if got := parameters(); got != c.want {
				t.Errorf("RunExample gives %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestAnOldNameIsReportedOnce checks a value read from a deprecated name is
// reported on the standard error stream, naming the variable that replaces
// it, and only once however often the name is read.
func TestAnOldNameIsReportedOnce(t *testing.T) {
	setVariables(t, map[string]string{oldURL: "https://old.example/"})
	printed := standardError(t, func() {
		parameters()
		parameters()
	})
	if got := reportsOf(printed, oldURL, urlVariable); got != 1 {
		t.Errorf("two runs with only %s set reported it %d times, want 1, "+
			"in %q", oldURL, got, printed)
	}
}

// inAnEmptyFolder makes a new empty folder the working folder for the test,
// so that no data file is looked for in the data repository.
func inAnEmptyFolder(t *testing.T) {
	t.Helper()
	before, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(before); err != nil {
			t.Fatal(err)
		}
	})
}

// standardError returns what f writes to the standard error stream.
func standardError(t *testing.T, f func()) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	before := os.Stderr
	os.Stderr = file
	defer func() { os.Stderr = before }()
	f()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(written)
}

// reportsOf returns how many lines of printed say that the variable old is
// deprecated and that the variable name replaces it.
func reportsOf(printed, old, name string) int {
	count := 0
	for _, line := range strings.Split(printed, "\n") {
		if strings.Contains(line, " "+old+" is deprecated") &&
			strings.Contains(line, " "+name+" replaces it") {
			count++
		}
	}
	return count
}
