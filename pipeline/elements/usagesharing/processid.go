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

package usagesharing

import (
	"crypto/rand"
	"fmt"
	mathrand "math/rand"
	"time"
)

// newProcessID returns a new version 4 UUID, made from random numbers, in
// the string format of RFC 9562 section 4, which is 32 hexadecimal digits
// in groups of 8, 4, 4, 4 and 12 separated by hyphens. It is never stored,
// so an element built again, in this process or another, has a new one.
// See https://github.com/51Degrees/specifications/blob/07e1fcedc2/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L119-L131
func newProcessID() string {
	var u [16]byte
	// From Go 1.24 crypto/rand.Read never fails. Before it, a failure is
	// met with pseudorandom numbers, which the specification also allows.
	if _, err := rand.Read(u[:]); err != nil {
		r := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
		r.Read(u[:])
	}
	u[6] = u[6]&0x0f | 0x40 // version 4
	u[8] = u[8]&0x3f | 0x80 // the variant of RFC 9562
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}
