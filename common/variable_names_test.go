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

// The list of the environment variables this repository reads and sets, from
// the top of the repository.
const listFile = "environment-variables.md"

// The start of every name 51Degrees defines.
const prefix = "_51DEGREES_"

// A variable's name.
const variableName = "([A-Za-z_][A-Za-z0-9_]*)"

// readByName finds a name where Go or PowerShell reads or sets a variable by name,
// which is in a call to one of the functions of the os package or of .NET's
// Environment class, and after the env drive.
var readByName = regexp.MustCompile("(?:(?:Getenv|LookupEnv|Setenv|Unsetenv)\\([\"`]" +
	"|EnvironmentVariable\\([\"']" +
	"|\\b[Ee][Nn][Vv]:)" + variableName)

// assigned finds a name given a value, as a command line, a readme and an
// environment handed to a command write it. That is a name in capitals then
// an equals sign, after a quote or at the start of a line.
var assigned = regexp.MustCompile("(?:[\"'`]|^[\t >$]*(?:export +|set +)?)" +
	"([A-Z_][A-Z0-9_]+)=")

// held finds a name a Go constant or variable holds to read later, where the
// constant says so by having Env or Variable in its own name.
var held = regexp.MustCompile(
	"\\b[A-Za-z0-9]*(?:Env|Variable)[A-Za-z0-9]*\\s*=\\s*\"" + variableName)

// inContext finds a name a workflow reads from the env, secrets or vars
// contexts.
var inContext = regexp.MustCompile("\\b(?:env|secrets|vars)\\." + variableName)

// envBlock finds the line that opens a workflow's env block, and envKey a
// name that block sets.
var (
	envBlock = regexp.MustCompile("^([ -]*)env:\\s*(?:#.*)?$")
	envKey   = regexp.MustCompile("^( +)" + variableName + ":")
)

// platform holds the names that Go, Git, GitHub, PowerShell and the operating
// system define, which are read and set as each requires.
var platform = map[string]bool{
	"CI": true, "PATH": true, "GOWORK": true, "GOPRIVATE": true,
	"CGO_ENABLED": true, "CGO_LDFLAGS": true, "GIT_CONFIG_COUNT": true,
	"PSModulePath": true, "ProgramFiles": true,
}

// platformPrefixes start the names GitHub defines, and the numbered names
// Git reads its settings from.
var platformPrefixes = []string{"GITHUB_", "GIT_CONFIG_KEY_", "GIT_CONFIG_VALUE_"}

// use is a variable named on a line of a file.
type use struct {
	line int
	name string
}

// variablesIn returns each variable the text of the file reads or sets, with
// its line.
func variablesIn(file string, text []byte) []use {
	source := strings.HasSuffix(file, ".go")
	workflow := strings.HasSuffix(file, ".yml") || strings.HasSuffix(file, ".yaml")
	var uses []use
	block := -1
	for number, line := range bytes.Split(text, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		patterns := []*regexp.Regexp{readByName, assigned}
		if source {
			patterns = append(patterns, held)
		}
		if workflow {
			patterns = append(patterns, inContext)
			indent := len(line) - len(bytes.TrimLeft(line, " "))
			if opened := envBlock.FindSubmatch(line); opened != nil {
				block = len(opened[1])
			} else if len(bytes.TrimSpace(line)) > 0 && indent <= block {
				block = -1
			} else if block >= 0 {
				patterns = append(patterns, envKey)
			}
		}
		for _, pattern := range patterns {
			for _, match := range pattern.FindAllSubmatch(line, -1) {
				uses = append(uses,
					use{number + 1, string(match[len(match)-1])})
			}
		}
	}
	return uses
}

// allowed reports whether a variable may be read or set, which is when
// 51Degrees defines it and the list holds it, or the platform defines it.
func allowed(name string, listed map[string]bool) bool {
	if strings.HasPrefix(name, prefix) {
		return listed[name]
	}
	if platform[name] {
		return true
	}
	for _, start := range platformPrefixes {
		if strings.HasPrefix(name, start) {
			return true
		}
	}
	return false
}

// listedName finds a name in the first column of the list's tables.
var listedName = regexp.MustCompile("(?m)^\\| `(" + prefix + "[A-Z0-9_]+)` \\|")

// namesListed returns the names the list holds.
func namesListed(text []byte) map[string]bool {
	listed := map[string]bool{}
	for _, match := range listedName.FindAllSubmatch(text, -1) {
		listed[string(match[1])] = true
	}
	return listed
}

// trackedFiles returns the root of the checkout and the files git tracks in
// it. Outside a git checkout, such as the module cache, there are none to
// read, and the test is skipped.
func trackedFiles(t *testing.T) (string, []string) {
	top, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skipf("not in a git checkout: %v", err)
	}
	root := strings.TrimSpace(string(top))
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	files := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	if len(files) == 0 || files[0] == "" {
		t.Fatal("git ls-files listed no files")
	}
	return root, files
}

// scripts are the kinds of file that read or set a variable: Go, PowerShell,
// shell, workflows and other YAML, JSON settings, and the Markdown that
// shows a reader how.
var scripts = map[string]bool{
	".go": true, ".ps1": true, ".sh": true, ".yml": true, ".yaml": true,
	".json": true, ".md": true,
}

// readTracked returns the text of a tracked file, or nil for a submodule,
// a file that is not text or anything else that is not a regular file.
func readTracked(t *testing.T, root, file string) []byte {
	path := filepath.Join(root, filepath.FromSlash(file))
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return nil
	}
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", file, err)
	}
	if bytes.IndexByte(text, 0) >= 0 {
		return nil
	}
	return text
}

// Every environment variable a tracked file reads or sets starts _51DEGREES_
// and is in the list, or is one the platform defines. The submodules and the
// copies of the C library are not read.
func TestEveryVariableHasA51DegreesName(t *testing.T) {
	root, files := trackedFiles(t)
	list, err := os.ReadFile(filepath.Join(root, listFile))
	if err != nil {
		t.Fatalf("reading the list of names: %v", err)
	}
	listed := namesListed(list)
	if len(listed) == 0 {
		t.Fatalf("%s lists no names", listFile)
	}
	var found []string
	for _, file := range files {
		if !scripts[strings.ToLower(filepath.Ext(file))] {
			continue
		}
		for _, used := range variablesIn(file, readTracked(t, root, file)) {
			if !allowed(used.name, listed) {
				found = append(found,
					file+":"+strconv.Itoa(used.line)+" "+used.name)
			}
		}
	}
	if len(found) > 0 {
		t.Errorf("give each of these variables a %s name that %s lists:\n%s",
			prefix, listFile, strings.Join(found, "\n"))
	}
}

// Each name below is written in parts, so that this file reads no variable
// itself.
const (
	other  = "EXAMPLE" + "_SETTING"
	ours   = prefix + "DD_PATH"
	github = "GITHUB" + "_JOB"
)

// A name is found wherever a file reads or sets a variable, and nowhere
// else.
func TestVariablesAreFoundWhereTheyAreNamed(t *testing.T) {
	for _, c := range []struct{ file, line, want string }{
		{"a.go", `os.Getenv("` + other + `")`, other},
		{"a.go", `value, ok := os.LookupEnv("` + other + `")`, other},
		{"a_test.go", `t.Setenv("` + other + `", "1")`, other},
		{"a.go", "os.Unsetenv(`" + other + "`)", other},
		{"a.go", "cmd.Env = append(env, `" + other + "=1`)", other},
		{"a.go", `const settingEnv = "` + other + `"`, other},
		{"a.go", `const settingVariable = "` + other + `"`, other},
		{"a.go", `const EnvSetting = "` + other + `"`, other},
		{"a.ps1", "$env" + ":" + other + " = $path", other},
		{"a.ps1", "${env" + ":" + other + "}", other},
		{"a.ps1", "Remove-Item Env" + ":" + other, other},
		{"a.ps1", "[Environment]::GetEnvironmentVariable('" + other + "')", other},
		{"a.yml", "run: echo ${{ env" + "." + other + " }}", other},
		{"a.yml", "key: ${{ secrets" + "." + other + " }}", other},
		{"a.yml", "key: ${{ vars" + "." + other + " }}", other},
		{"README.md", other + "=/data/file go run ./x", other},
		{"README.md", "    export " + other + "=/data/file", other},
		{"README.md", "$ " + other + "=1 go test ./...", other},

		// A constant, a type and a word in a sentence are not variables.
		{"a.go", "const " + other + " = 4", ""},
		{"a.go", `const name = "` + other + `"`, ""},
		{"a.go", "type " + other + " struct{}", ""},
		{"README.md", "The " + other + " setting is not read.", ""},
		// The env, secrets and vars contexts are a workflow's.
		{"a.go", "return env" + "." + other, ""},
	} {
		var got []string
		for _, used := range variablesIn(c.file, []byte(c.line)) {
			got = append(got, used.name)
		}
		if strings.Join(got, ",") != c.want {
			t.Errorf("%s in %s gave %q, want %q", c.line, c.file, got, c.want)
		}
	}
}

// A workflow sets a variable as a key of an env block, at any level, and the
// keys of other blocks are not variables.
func TestAWorkflowEnvBlockSetsItsKeys(t *testing.T) {
	workflow := strings.Join([]string{
		"jobs:",
		"  build:",
		"    env:",
		"      " + other + ": 1",
		"",
		"      " + ours + ": 2",
		"    steps:",
		"      - name: Build",
		"        with:",
		"          " + github + ": 3",
		"      - env:",
		"          " + github + ": 4",
		"        run: go build ./...",
	}, "\n")
	want := []use{{4, other}, {6, ours}, {12, github}}
	got := variablesIn(".github/workflows/a.yml", []byte(workflow))
	if len(got) != len(want) {
		t.Fatalf("found %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("found %v, want %v", got[i], want[i])
		}
	}
}

// A 51Degrees name is allowed when the list holds it, a platform name is
// allowed as it is, and no other name is.
func TestOnlyListedAndPlatformNamesAreAllowed(t *testing.T) {
	listed := namesListed([]byte("| Name | Holds |\n| --- | --- |\n" +
		"| `" + ours + "` | The path of a data file |\n"))
	for name, want := range map[string]bool{
		ours:                    true,
		prefix + "NOT_LISTED":   false,
		other:                   false,
		github:                  true,
		"CI":                    true,
		"PATH":                  true,
		"GIT_CONFIG" + "_KEY_0": true,
		"51DEGREES_DD_PATH":     false,
		"":                      false,
	} {
		if got := allowed(name, listed); got != want {
			t.Errorf("allowed(%q) is %v, want %v", name, got, want)
		}
	}
}

// Evidence is YAML, so no tracked file names a file of comma separated
// values. The extension is written in two parts so that this file names none.
func TestEvidenceIsYAML(t *testing.T) {
	extension := []byte(".c" + "sv")
	root, files := trackedFiles(t)
	var found []string
	for _, file := range files {
		text := bytes.ToLower(readTracked(t, root, file))
		for number, line := range bytes.Split(text, []byte("\n")) {
			if bytes.Contains(line, extension) {
				found = append(found, file+":"+strconv.Itoa(number+1))
			}
		}
	}
	if len(found) > 0 {
		t.Errorf("read the evidence from a YAML file here:\n%s",
			strings.Join(found, "\n"))
	}
}
