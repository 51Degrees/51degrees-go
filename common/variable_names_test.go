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

// fiftyOneName finds a name that starts FIFTYONE_. The names of the C library
// all start FIFTYONE_DEGREES_ and are left out.
var fiftyOneName = regexp.MustCompile(`FIFTYONE_[A-Z0-9_]+`)

// No tracked file names a variable that starts FIFTYONE_, because the
// 51Degrees environment variables start _51DEGREES_, which is shorter. The C
// sources and the cxx submodule hold only the C library's names. Outside a git
// checkout, such as the module cache, there are no tracked files to read.
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
		if strings.HasPrefix(file, "cxx/") || file == "cxx" ||
			strings.HasSuffix(file, ".c") || strings.HasSuffix(file, ".h") {
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
			for _, name := range fiftyOneName.FindAll(line, -1) {
				if !bytes.HasPrefix(name, []byte("FIFTYONE_DEGREES_")) {
					found = append(found,
						file+":"+strconv.Itoa(number+1)+" "+string(name))
				}
			}
		}
	}
	if len(found) > 0 {
		t.Errorf("name these variables _51DEGREES_ instead:\n%s",
			strings.Join(found, "\n"))
	}
}
