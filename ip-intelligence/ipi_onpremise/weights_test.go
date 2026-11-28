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

package ipi_onpremise

import "testing"

// Every Property read from a data file is given as weighted values, each
// with a weight above 0 and no more than 1, so a caller can read each value
// with its weighting.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/data-model.md?plain=1#L37
func TestEveryPropertyIsAWeightedValue(t *testing.T) {
	e := newReloadTestEngine(t, WithDataFile(writeDataFile(t, readGoodFile(t))),
		WithAutoUpdate(false), WithFileWatch(false))
	values, err := e.Process(reloadTestAddress)
	if err != nil {
		t.Fatal(err)
	}
	weighted := 0
	for name, list := range values {
		for _, value := range list {
			if value.Weight <= 0 || value.Weight > 1 {
				t.Errorf("%s gives %v with the weight %v, want a weight above 0 "+
					"and no more than 1", name, value.Value, value.Weight)
			}
			weighted++
		}
	}
	if weighted == 0 {
		t.Errorf("%s gave no values from the data file", reloadTestAddress)
	}
}
