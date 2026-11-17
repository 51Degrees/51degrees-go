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

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// valueOf gives the value of the named variable in environment, and whether
// it is there.
func valueOf(environment []string, name string) (string, bool) {
	for _, variable := range environment {
		if n, value, _ := strings.Cut(variable, "="); strings.EqualFold(n, name) {
			return value, true
		}
	}
	return "", false
}

// readText gives the content of the file at path, failing the test when it
// cannot be read.
func readText(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// An example is given its own data file, a hard link to or a copy of the one
// the settings name, under either name, in a new folder beside it. Other
// variables are kept, and renaming a file over the example's own file leaves
// the named file as it was.
func TestAnExampleIsGivenItsOwnDataFile(t *testing.T) {
	deprecated := "DATA" + "_FILE"
	for _, name := range []string{"_51DEGREES_DD_PATH", deprecated} {
		t.Run(name, func(t *testing.T) {
			folder := t.TempDir()
			shared := filepath.Join(folder, "51Degrees-LiteV4.1.hash")
			if err := os.WriteFile(shared, []byte("the free data file"), 0o644); err != nil {
				t.Fatal(err)
			}
			environment, err := ownDataFile(t,
				[]string{"PATH=kept", name + "=" + shared})
			if err != nil {
				t.Fatal(err)
			}
			own, ok := valueOf(environment, "_51DEGREES_DD_PATH")
			if !ok || own == shared ||
				filepath.Dir(filepath.Dir(own)) != folder {
				t.Fatalf("the example is given %q, want its own file in a "+
					"new folder in %s", own, folder)
			}
			if _, ok := valueOf(environment, deprecated); ok {
				t.Errorf("the example is still given %s", deprecated)
			}
			if path, _ := valueOf(environment, "PATH"); path != "kept" {
				t.Errorf("PATH is %q, want it kept", path)
			}
			if got := readText(t, own); got != "the free data file" {
				t.Errorf("the example's own file holds %q", got)
			}

			// An update writes the download next to the data file and
			// renames it over the data file.
			download := filepath.Join(filepath.Dir(own), "download")
			if err := os.WriteFile(download, []byte("the Enterprise data file"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(download, own); err != nil {
				t.Fatal(err)
			}
			if got := readText(t, shared); got != "the free data file" {
				t.Errorf("an update of the example's own file changed the "+
					"named file to %q", got)
			}
		})
	}
}

// A data file that the settings name and that is not there is an error, so
// an example is never run on a file other tests read.
func TestAnExampleNeedsADataFileOfItsOwn(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.hash")
	if _, err := ownDataFile(t,
		[]string{"_51DEGREES_DD_PATH=" + missing}); err == nil {
		t.Errorf("a missing data file gave no error")
	}
}

// With no data file named, the environment is passed on as it is, and the
// example finds the free data file itself.
func TestAnExampleWithNoDataFileNamedIsGivenTheEnvironment(t *testing.T) {
	environment := []string{"PATH=kept"}
	got, err := ownDataFile(t, environment)
	if err != nil || len(got) != 1 || got[0] != "PATH=kept" {
		t.Errorf("ownDataFile gave %q and %v, want the environment as it was",
			got, err)
	}
}
