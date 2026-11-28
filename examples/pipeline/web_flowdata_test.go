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

package examples

import (
	"os/exec"
	"strings"
	"testing"
)

// "Every web integration example, and the documentation of every web
// integration, MUST say that a Flow Data cannot be used by two threads at
// once, and MUST show it." The web example's handler copies the star sign
// out of the Flow Data and hands the copy, not the Flow Data, to another
// goroutine, which prints it.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/features/web-integration.md?plain=1#L36-L40
func TestTheWebExampleHandsValuesNotTheFlowDataToAnotherGoroutine(t *testing.T) {
	output, err := exec.Command("go", "run", "./3_clientside_evidence").CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	want := "Star sign handed to another goroutine: Aries"
	if !strings.Contains(string(output), want) {
		t.Errorf("the web example does not print %q, so it does not show "+
			"work handed to another goroutine with values copied from the "+
			"Flow Data:\n%s", want, output)
	}
}
