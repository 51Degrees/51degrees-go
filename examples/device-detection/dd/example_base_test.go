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

package dd

import (
	"os"
	"path/filepath"
	"testing"
)

// Stand in for the data repository's scripts. Each writes the source it is
// given into the file it is asked for.
const (
	testGetDataFilesSh = `while getopts s: opt; do
    case $opt in s) source=$OPTARG ;; esac
done
shift $((OPTIND - 1))
printf %s "$source" > "$(dirname "$0")/device-detection/$1"
`
	testGetDataFilesPs1 = `param([string]$Name, [string]$Source)
Set-Content -NoNewline -Value $Source ` +
		`-LiteralPath (Join-Path $PSScriptRoot "device-detection/$Name")
`
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// Test that a file the examples can't find is downloaded with the data
// repository's script when the data folder has a link file for it.
func TestGetFilePathByNameDownloads(t *testing.T) {
	// Outside GitHub Actions the examples send their own source.
	t.Setenv("GITHUB_ACTIONS", "")
	root := t.TempDir()
	data := filepath.Join(root, "cxx", "data")
	name := "test-download.hash"
	writeTestFile(t, filepath.Join(data, "device-detection", name+".link"),
		"https://example.com/test-download.hash\n")
	writeTestFile(t, filepath.Join(data, "get-data-files.sh"),
		testGetDataFilesSh)
	writeTestFile(t, filepath.Join(data, "get-data-files.ps1"),
		testGetDataFilesPs1)
	work := filepath.Join(root, "examples", "device-detection", "dd")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	})

	found := GetFilePathByName([]string{name})

	want, err := filepath.EvalSymlinks(
		filepath.Join(data, "device-detection", name))
	if err != nil {
		t.Fatal(err)
	}
	got, err := filepath.EvalSymlinks(found)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Expected %s, but got %s instead.", want, got)
	}
	source, err := os.ReadFile(found)
	if err != nil {
		t.Fatal(err)
	}
	if string(source) != downloadSource {
		t.Errorf("Expected the source %s, but got %s instead.",
			downloadSource, source)
	}

	// In GitHub Actions the data repository's scripts send their own source,
	// so that test runs are not counted as people running the examples.
	if err := os.Remove(found); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_ACTIONS", "true")
	found = GetFilePathByName([]string{name})
	source, err = os.ReadFile(found)
	if err != nil {
		t.Fatal(err)
	}
	if len(source) != 0 {
		t.Errorf("Expected no source in GitHub Actions, but got %s.", source)
	}
}
