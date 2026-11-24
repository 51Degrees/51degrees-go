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

package ipi_interop

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// cgoCompilerFlags gives every flag the #cgo CFLAGS and CPPFLAGS directives
// in the package's Go files pass to the C compiler, whatever their build
// constraints.
func cgoCompilerFlags(t *testing.T) []string {
	t.Helper()
	directive := regexp.MustCompile(`#cgo\s+[^:]*\b(CFLAGS|CPPFLAGS):(.*)`)
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var flags []string
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(text), "\n") {
			if match := directive.FindStringSubmatch(line); match != nil {
				flags = append(flags, strings.Fields(match[2])...)
			}
		}
	}
	return flags
}

// The C sources build with every warning the cxx build turns on, and any
// warning stops the build.
func TestCgoFlagsTreatWarningsAsErrors(t *testing.T) {
	flags := cgoCompilerFlags(t)
	for _, want := range []string{"-Wall", "-Wextra", "-Werror"} {
		found := false
		for _, flag := range flags {
			if flag == want {
				found = true
			}
		}
		if !found {
			t.Errorf("the #cgo compiler flags %v do not include %s",
				flags, want)
		}
	}
	for _, flag := range flags {
		if flag == "-w" ||
			strings.HasPrefix(flag, "-Wno-") ||
			flag == "-fcommon" ||
			flag == "-fpermissive" {
			t.Errorf("the #cgo compiler flags include %s", flag)
		}
	}
}
