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

package engines

import (
	"errors"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// Reading a Property that has no value gives an error carrying the message
// the Engine set when it assigned no value, so each Engine says why.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/properties.md?plain=1#L57-L58
func TestNoValueGivesTheMessageItWasGiven(t *testing.T) {
	for _, message := range []string{
		"no match was found for this request",
		"the Resource Key does not include this property",
	} {
		_, err := NoValue[string](message).Value()
		var noValue *core.NoValueError
		if !errors.As(err, &noValue) {
			t.Fatalf("Value gave %v, want a NoValueError", err)
		}
		if noValue.Error() != message {
			t.Errorf("Value gave the message %q, want %q", noValue.Error(), message)
		}
	}
}
