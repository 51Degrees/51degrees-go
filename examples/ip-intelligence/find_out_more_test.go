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

package main

import (
	"os"
	"strings"
	"testing"
)

// repositoryURL is the repository every example's Find out more section
// names.
const repositoryURL = "https://github.com/51Degrees/51degrees-go"

// checkFindOutMore checks that an example's output ends with a Find out more
// section naming this repository.
func checkFindOutMore(t *testing.T, name, output string) {
	at := strings.LastIndex(output, "Find out more")
	if at < 0 || !strings.Contains(output[at:], repositoryURL) {
		t.Errorf("%s does not end with a Find out more section naming "+
			"this repository", name)
	}
}

// Each of these examples, run as a fresh clone runs it, ends with a Find out
// more section naming this repository. The Metadata and cloud examples' own
// tests check the same.
func TestEachExampleEndsWithFindOutMore(t *testing.T) {
	for _, name := range []string{"getting_started", "offline_processing",
		"reload_from_file", "performance"} {
		checkFindOutMore(t, name, run(t, name, withoutSettings(os.Environ())))
	}
}
