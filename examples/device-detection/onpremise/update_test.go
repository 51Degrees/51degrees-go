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
	"time"
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

// modified gives the modification time of the file at path, failing the
// test when it cannot be read.
func modified(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime()
}

// An example is given its own data file, a copy of the one the settings name,
// under either name, in a new folder beside it, and the folder is removed
// afterwards. Other variables are kept. Setting the modification time of the
// example's own file, as the reload example does, or renaming a file over it,
// as an update does, leaves the named file as it was.
func TestAnExampleIsGivenItsOwnDataFile(t *testing.T) {
	for _, name := range []string{dataFileVariable, deprecatedDataFile} {
		t.Run(name, func(t *testing.T) {
			folder := t.TempDir()
			shared := filepath.Join(folder, "51Degrees-LiteV4.1.hash")
			if err := os.WriteFile(shared, []byte("the free data file"), 0o644); err != nil {
				t.Fatal(err)
			}
			environment, remove, err := ownDataFile(
				[]string{"PATH=kept", name + "=" + shared})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(remove)
			own, ok := valueOf(environment, dataFileVariable)
			if !ok || own == shared ||
				filepath.Dir(filepath.Dir(own)) != folder {
				t.Fatalf("the example is given %q, want its own file in a "+
					"new folder in %s", own, folder)
			}
			if _, ok := valueOf(environment, deprecatedDataFile); ok {
				t.Errorf("the example is still given %s", deprecatedDataFile)
			}
			if path, _ := valueOf(environment, "PATH"); path != "kept" {
				t.Errorf("PATH is %q, want it kept", path)
			}
			if got := readText(t, own); got != "the free data file" {
				t.Errorf("the example's own file holds %q", got)
			}

			// The reload example sets the modification time of its data
			// file so that the engine reloads it.
			before := modified(t, shared)
			later := before.Add(time.Hour)
			if err := os.Chtimes(own, later, later); err != nil {
				t.Fatal(err)
			}
			if got := modified(t, shared); !got.Equal(before) {
				t.Errorf("setting the modification time of the example's "+
					"own file changed that of the named file from %v to %v",
					before, got)
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

			remove()
			if _, err := os.Stat(filepath.Dir(own)); !os.IsNotExist(err) {
				t.Errorf("the folder of the example's own file is still "+
					"there after it is removed: %v", err)
			}
		})
	}
}

// A data file that the settings name and that is not there is an error, so
// an example is never run on a file other tests read.
func TestAnExampleNeedsADataFileOfItsOwn(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.hash")
	if _, _, err := ownDataFile(
		[]string{dataFileVariable + "=" + missing}); err == nil {
		t.Errorf("a missing data file gave no error")
	}
}

// With no data file named, the environment is passed on as it is, and the
// example finds the free data file itself.
func TestAnExampleWithNoDataFileNamedIsGivenTheEnvironment(t *testing.T) {
	environment := []string{"PATH=kept"}
	got, remove, err := ownDataFile(environment)
	if err != nil || len(got) != 1 || got[0] != "PATH=kept" {
		t.Errorf("ownDataFile gave %q and %v, want the environment as it was",
			got, err)
	}
	if remove != nil {
		remove()
	}
}
