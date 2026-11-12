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

package common_go

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// oldName finds an old variable name where a variable is named, which is
// after a quote, a backtick, an equals sign, a dollar sign, env: or env., or
// at the start of a line past any spaces, tabs and a list dash. The last of
// those is how a workflow declares one, as a key in an env: block, and how a
// readme lists one, neither of which puts any of the other characters in
// front of the name.
//
// Two families are matched. A name that starts FIFTYONE_, because the
// 51Degrees environment variables start _51DEGREES_, which is shorter, and the
// names the examples, the CI scripts and the cloud request engine read before
// each variable took one _51DEGREES_ name. The names of the C library all
// start FIFTYONE_DEGREES_ and are left out.
//
// Every name in this pattern is preceded by a bracket or a bar rather than by
// one of the characters above, so this file does not match itself.
var oldName = regexp.MustCompile("(?:[\"'`=$]|\\$\\{|env[:.]|^[\t -]*)" +
	"(FIFTYONE_[A-Z0-9_]+|DD_DATA_FILE|DATA_FILE|EVIDENCE_YAML" +
	"|LICENSE_KEY|IPI_KEY|IPI_DATA_FILE_URL|DEVICE_DETECTION_KEY" +
	"|FOD_CLOUD_API_URL)")

// No tracked file names one of those variables. The cxx submodule is not read.
// Outside a git checkout, such as the module cache, there are no tracked files
// to read.
func TestNoFileNamesAnOldVariable(t *testing.T) {
	top, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not in a git checkout: %v", err)
	}
	root := strings.TrimSpace(string(top))
	listed, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	files := strings.Split(strings.TrimRight(string(listed), "\x00"), "\x00")
	if len(files) == 0 || files[0] == "" {
		t.Fatal("git ls-files listed no files")
	}
	var found []string
	for _, file := range files {
		if strings.HasPrefix(file, "cxx/") || file == "cxx" {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(file))
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			continue
		}
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		for number, line := range bytes.Split(text, []byte("\n")) {
			for _, name := range oldVariables(line) {
				found = append(found,
					file+":"+strconv.Itoa(number+1)+" "+name)
			}
		}
	}
	if len(found) > 0 {
		t.Errorf("give these variables their one _51DEGREES_ name:\n%s",
			strings.Join(found, "\n"))
	}
}

// oldVariables returns the old names in the line, leaving out the C library's.
func oldVariables(line []byte) []string {
	var names []string
	for _, match := range oldName.FindAllSubmatch(line, -1) {
		if name := match[1]; !bytes.HasPrefix(name, []byte("FIFTYONE_DEGREES_")) {
			names = append(names, string(name))
		}
	}
	return names
}

// Each name below is written in two parts so that this file does not name a
// variable itself.
const (
	fiftyOne  = "FIFTY" + "ONE_"
	dataFile  = "DATA" + "_FILE"
	licence   = "LICENSE" + "_KEY"
	ipiKey    = "IPI" + "_KEY"
	deviceKey = "DEVICE" + "_DETECTION_KEY"
	cloudURL  = "FOD" + "_CLOUD_API_URL"
)

// A name counts only where a variable is named, after a quote, a backtick, an
// equals sign, a dollar sign, env: or env., so a constant, a type or a GitHub
// secret of the same name is left alone, as is a longer name that only ends in
// one of these words.
func TestOldVariablesAreOnlyVariables(t *testing.T) {
	const name = fiftyOne + "DATA"
	for line, want := range map[string]string{
		`os.Getenv("` + name + `")`:              name,
		"$env:" + name + " = 1":                  name,
		"${" + name + ":-x}":                     name,
		"${{ env." + name + " }}":                name,
		"cmd.Env = append(env, `" + name + "=1`)": name,
		"const " + fiftyOne + "LIMIT = 4":        "",
		"type " + fiftyOne + "THING struct{}":    "",
		`"` + fiftyOne + `DEGREES_STRING"`:       "",

		`os.Getenv("` + dataFile + `")`:         dataFile,
		"$env:DD_" + dataFile + " = $path":      "DD_" + dataFile,
		`os.Getenv("` + licence + `")`:          licence,
		`os.Getenv("` + ipiKey + `")`:           ipiKey,
		`os.Getenv("IPI_` + dataFile + `_URL")`: "IPI_" + dataFile + "_URL",
		`os.Getenv("` + deviceKey + `")`:        deviceKey,
		`os.Getenv("` + cloudURL + `")`:         cloudURL,

		// A workflow declares a variable as a key in an env: block, and a
		// readme lists one, so a name at the start of a line counts.
		"  " + licence + ": ${{ secrets.DEVICE" + "_DETECTION_KEY }}": licence,
		"- " + dataFile + " names the data file": dataFile,
		dataFile + "=/data/file.ipi go run ./x":  dataFile,

		// The one name each of those variables now carries.
		`os.Getenv("_51DEGREES_DD_PATH")`:         "",
		`os.Getenv("_51DEGREES_` + licence + `")`: "",
		`os.Getenv("_51DEGREES_CLOUD_ENDPOINT")`:  "",
		// A longer name that ends in one of these words.
		`os.Getenv("_51DEGREES_TEST_` + licence + `")`: "",
		// A GitHub secret is not an environment variable, and it is read
		// through secrets. rather than env., so a workflow line whose key is
		// not an old name is left alone whatever the secret is called.
		"  DeviceDetection: ${{ secrets." + deviceKey + " }}": "",
	} {
		got := strings.Join(oldVariables([]byte(line)), ",")
		if got != want {
			t.Errorf("%s gave %q, want %q", line, got, want)
		}
	}
}
