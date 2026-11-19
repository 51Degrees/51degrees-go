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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
)

// dataFileVariable names the data file the examples use, and
// deprecatedDataFile its deprecated name, written in two parts so that this
// file does not name an old variable.
const (
	dataFileVariable   = "_51DEGREES_DD_PATH"
	deprecatedDataFile = "DATA" + "_FILE"
)

// TestMain runs the package's tests on a data file of their own, as
// onOwnDataFile gives them.
func TestMain(m *testing.M) {
	os.Exit(onOwnDataFile(m.Run))
}

// onOwnDataFile runs tests with dataFileVariable naming a copy of the data
// file the settings name, made by ownDataFile, and removes the copy
// afterwards. The update example renames a new data file over its own and
// the reload example sets its modification time, whilst the tests of other
// packages read the file the settings name, so these tests must not reach
// that file. The result is that of tests, or 1 when the file the settings
// name changed size or modification time while they ran.
func onOwnDataFile(tests func() int) int {
	named := namedDataFile(os.Environ())
	if named == "" {
		return tests()
	}
	before, err := os.Stat(named)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	start := time.Now()
	environment, remove, err := ownDataFile(os.Environ())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	own, _ := valueOf(environment, dataFileVariable)
	fmt.Printf("The tests use %s, a copy of %s made in %v.\n", own, named,
		time.Since(start).Round(time.Millisecond))
	restore := useDataFile(environment)
	code := tests()
	restore()
	remove()
	after, err := os.Stat(named)
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "%s, the data file the settings name, "+
			"cannot be read after the tests: %v\n", named, err)
	case after.Size() != before.Size() ||
		!after.ModTime().Equal(before.ModTime()):
		fmt.Fprintf(os.Stderr, "%s, the data file the settings name and "+
			"the tests of other packages read, changed from %d bytes "+
			"modified at %v to %d bytes modified at %v while these tests "+
			"ran\n", named, before.Size(), before.ModTime(), after.Size(),
			after.ModTime())
	default:
		return code
	}
	return 1
}

// testFreeOnOwnCopy runs an example as testFree does, with no arguments or
// settings, but builds it first and runs it in a folder of its own holding
// copies of the free data file and the evidence. The example looks in its
// working folder first, so it finds those copies. It is for an example that
// sets its data file's modification time, because run from its package the
// example finds the free file the tests of other packages read.
func testFreeOnOwnCopy(t *testing.T, name string) {
	folder, environment := freeOnOwnCopy(t)
	program := filepath.Join(folder, name)
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	build := exec.Command("go", "build", "-o", program, "./"+name)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("%s cannot be built: %v\n%s", name, err, output)
	}
	cmd := exec.Command(program)
	cmd.Dir = folder
	cmd.Env = environment
	output, err := cmd.CombinedOutput()
	os.Stdout.Write(output)
	if err != nil {
		t.Error(err)
	}
	if err := dd_example.CheckFindOutMore(string(output)); err != nil {
		t.Errorf("%s: %v", name, err)
	}
	checkEnterprise(t, name, string(output))
}

// freeOnOwnCopy makes a folder holding copies of the free data file and the
// evidence, found as the examples find them, and returns it with the
// environment testFreeOnOwnCopy gives the example, which has no settings.
func freeOnOwnCopy(t *testing.T) (string, []string) {
	folder := t.TempDir()
	for _, file := range []string{
		dd_example.GetDataFile(dd_example.LiteDataFile),
		dd_example.GetFilePathByPath(dd_example.EvidenceFileYaml),
	} {
		own := filepath.Join(folder, filepath.Base(file))
		if err := copyFile(file, own); err != nil {
			t.Fatalf("a run on its own copy needs %s: %v", file, err)
		}
	}
	return folder, withoutSettings(os.Environ())
}

// TestFreeOnOwnCopyGivesNoSettings checks that the run of an example on its
// own copy of the free data file is given no settings, as someone would run
// it from a fresh clone, and that its folder holds the copy.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/device-detection-specification/required-examples.md?plain=1#L20-L23
func TestFreeOnOwnCopyGivesNoSettings(t *testing.T) {
	folder, environment := freeOnOwnCopy(t)
	for _, variable := range environment {
		if len(withoutSettings([]string{variable})) == 0 {
			name, _, _ := strings.Cut(variable, "=")
			t.Errorf("the run on its own copy is given the setting %s", name)
		}
	}
	own := filepath.Join(folder, dd_example.LiteDataFile)
	if _, err := os.Stat(own); err != nil {
		t.Errorf("the run's folder holds no copy of the free data file: %v",
			err)
	}
}

// namedDataFile gives the data file environment names under
// dataFileVariable, or else under its deprecated name, as the examples read
// them, or "" when it names none.
func namedDataFile(environment []string) string {
	named, _ := valueOf(environment, dataFileVariable)
	if named == "" {
		named, _ = valueOf(environment, deprecatedDataFile)
	}
	return named
}

// ownDataFile returns environment with dataFileVariable naming a copy of the
// data file it names, with the same modification time, in a new folder
// beside that file, and without the deprecated name, with a function that
// removes the folder. A copy and not a hard link, because a link shares the
// modification time the reload example sets. Environment is returned as it
// is when it names no data file.
func ownDataFile(environment []string) ([]string, func(), error) {
	named := namedDataFile(environment)
	if named == "" {
		return environment, func() {}, nil
	}
	info, err := os.Stat(named)
	if err != nil {
		return nil, nil, err
	}
	folder, err := os.MkdirTemp(filepath.Dir(named), "own-data-file-")
	if err != nil {
		return nil, nil, err
	}
	remove := func() { os.RemoveAll(folder) }
	own := filepath.Join(folder, filepath.Base(named))
	err = copyFile(named, own)
	if err == nil {
		err = os.Chtimes(own, info.ModTime(), info.ModTime())
	}
	if err != nil {
		remove()
		return nil, nil, fmt.Errorf("the tests need a data file of their "+
			"own, copied from %s: %w", named, err)
	}
	kept := make([]string, 0, len(environment)+1)
	for _, variable := range environment {
		name, _, _ := strings.Cut(variable, "=")
		switch strings.ToUpper(name) {
		case dataFileVariable, deprecatedDataFile:
			continue
		}
		kept = append(kept, variable)
	}
	return append(kept, dataFileVariable+"="+own), remove, nil
}

// useDataFile sets dataFileVariable and its deprecated name in this process
// as environment has them, unsetting those it lacks, and returns a function
// that puts them back as they were.
func useDataFile(environment []string) func() {
	names := []string{dataFileVariable, deprecatedDataFile}
	previous := make(map[string]string, len(names))
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			previous[name] = value
		}
		if value, ok := valueOf(environment, name); ok {
			os.Setenv(name, value)
		} else {
			os.Unsetenv(name)
		}
	}
	return func() {
		for _, name := range names {
			if value, ok := previous[name]; ok {
				os.Setenv(name, value)
			} else {
				os.Unsetenv(name)
			}
		}
	}
}

// copyFile copies the file at from to a new file at to.
func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
