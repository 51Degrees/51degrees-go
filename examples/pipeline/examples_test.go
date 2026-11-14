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

// Package examples runs each Pipeline example as a fresh clone would, with no
// settings, and checks that it ends with a Find out more section.
package examples

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// examples are the Pipeline examples, each a folder here.
var examples = []string{
	"1_simple_element",
	"2_onpremise_engine",
	"3_clientside_evidence",
	"4_cloud_engine",
	"5_usage_sharing",
	"6_caching",
}

func TestExamplesEndWithFindOutMore(t *testing.T) {
	for _, name := range examples {
		t.Run(name, func(t *testing.T) {
			output, err := exec.Command("go", "run", "./"+name).CombinedOutput()
			os.Stdout.Write(output)
			if err != nil {
				t.Fatal(err)
			}
			at := strings.LastIndex(string(output), "Find out more")
			if at < 0 || !strings.Contains(string(output[at:]),
				"https://github.com/51Degrees/51degrees-go") {
				t.Errorf("%s does not end with a Find out more section "+
					"naming this repository", name)
			}
		})
	}
}
