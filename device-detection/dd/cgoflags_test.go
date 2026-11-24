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
	"regexp"
	"strings"
	"testing"
)

// cgoDirective is one #cgo CFLAGS or CPPFLAGS line in the package's Go files.
type cgoDirective struct {
	file       string
	constraint string
	flags      []string
}

// cgoCompilerDirectives gives every #cgo CFLAGS and CPPFLAGS line in the
// package's Go files, with the build constraint each one has, if any.
func cgoCompilerDirectives(t *testing.T) []cgoDirective {
	t.Helper()
	directive := regexp.MustCompile(
		`^\s*#cgo\s+(.*?)\s*\b(CFLAGS|CPPFLAGS):(.*)$`)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var directives []cgoDirective
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(text), "\n") {
			match := directive.FindStringSubmatch(strings.TrimRight(line, "\r"))
			if match != nil {
				directives = append(directives, cgoDirective{
					file:       file,
					constraint: match[1],
					flags:      strings.Fields(match[3])})
			}
		}
	}
	return directives
}

// The C sources build with the warnings of -Wall -Wextra on every system, and
// any warning stops the build.
func TestCgoFlagsTreatWarningsAsErrors(t *testing.T) {
	directives := cgoCompilerDirectives(t)
	for _, want := range []string{"-Wall", "-Wextra", "-Werror"} {
		found := false
		for _, d := range directives {
			for _, flag := range d.flags {
				if flag == want && d.constraint == "" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("no #cgo compiler flags line without a build constraint "+
				"includes %s", want)
		}
	}
	// -isystem makes the headers in a folder system headers, which hides
	// their warnings.
	for _, d := range directives {
		for _, flag := range d.flags {
			if flag == "-w" ||
				strings.HasPrefix(flag, "-Wno-") ||
				flag == "-fcommon" ||
				flag == "-fpermissive" ||
				strings.HasPrefix(flag, "-isystem") {
				t.Errorf("%s: the #cgo compiler flags include %s", d.file, flag)
			}
		}
	}
}

// No C code the package compiles turns a GCC or Clang warning off, turns it
// back from an error into a warning, or marks itself a system header, which
// hides every warning in it. That covers the C and header files and the C in
// the Go files' preambles.
func TestCSourcesKeepEveryWarning(t *testing.T) {
	pragma := regexp.MustCompile(
		`^\s*#\s*pragma\s+(GCC|clang)\s+` +
			`(diagnostic\s+(ignored|warning)|system_header)\b|` +
			`_Pragma\s*\(\s*"\s*(GCC|clang)\s+` +
			`(diagnostic\s+(ignored|warning)|system_header)\b`)
	var files []string
	for _, pattern := range []string{"*.c", "*.h", "*.go"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(text), "\n") {
			if pragma.MatchString(line) {
				t.Errorf("%s:%d turns a warning off: %s",
					file, i+1, strings.TrimSpace(line))
			}
		}
	}
}
