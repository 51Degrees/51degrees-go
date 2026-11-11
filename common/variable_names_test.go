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

// fiftyOneName finds a name that starts FIFTYONE_ where a variable is named,
// after a quote, a backtick, an equals sign, a dollar sign, env: or env. The
// names of the C library all start FIFTYONE_DEGREES_ and are left out.
var fiftyOneName = regexp.MustCompile("(?:[\"'`=$]|\\$\\{|env[:.])(FIFTYONE_[A-Z0-9_]+)")

// No tracked file names a variable that starts FIFTYONE_, because the
// 51Degrees environment variables start _51DEGREES_, which is shorter. The cxx
// submodule is not read. Outside a git checkout, such as the module cache,
// there are no tracked files to read.
func TestNoFileNamesAFiftyOneVariable(t *testing.T) {
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
			for _, name := range fiftyOneVariables(line) {
				found = append(found,
					file+":"+strconv.Itoa(number+1)+" "+name)
			}
		}
	}
	if len(found) > 0 {
		t.Errorf("name these variables _51DEGREES_ instead:\n%s",
			strings.Join(found, "\n"))
	}
}

// fiftyOneVariables returns the names in the line that start FIFTYONE_ and are
// not the C library's.
func fiftyOneVariables(line []byte) []string {
	var names []string
	for _, match := range fiftyOneName.FindAllSubmatch(line, -1) {
		if name := match[1]; !bytes.HasPrefix(name, []byte("FIFTYONE_DEGREES_")) {
			names = append(names, string(name))
		}
	}
	return names
}

// fiftyOne starts each example below. It is written in two parts so that this
// file does not name a variable itself.
const fiftyOne = "FIFTY" + "ONE_"

// A name counts only where a variable is named, after a quote, a backtick, an
// equals sign, a dollar sign, env: or env., so a constant or a type with such
// a name is left alone.
func TestFiftyOneVariablesAreOnlyVariables(t *testing.T) {
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
	} {
		got := strings.Join(fiftyOneVariables([]byte(line)), ",")
		if got != want {
			t.Errorf("%s gave %q, want %q", line, got, want)
		}
	}
}
