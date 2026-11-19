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

import (
	"reflect"
	"sync"
	"testing"
)

// "Aspect Data that reads its values from a ResultsIpi instance MUST NOT
// lock it, and is not safe to use from several threads at once", so neither
// the AspectData the AspectEngine adds nor the Data it reads from holds a
// lock of its own. The caller is responsible for thread-safe access.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L520-L525
func TestAspectDataTakesNoLock(t *testing.T) {
	locks := []reflect.Type{
		reflect.TypeOf(sync.Mutex{}), reflect.TypeOf(sync.RWMutex{}),
	}
	for _, held := range []reflect.Type{
		reflect.TypeOf(AspectData{}), reflect.TypeOf(Data{}),
	} {
		for i := 0; i < held.NumField(); i++ {
			field := held.Field(i)
			kind := field.Type
			if kind.Kind() == reflect.Pointer {
				kind = kind.Elem()
			}
			for _, lock := range locks {
				if kind == lock {
					t.Errorf("%s holds the lock %s, and Aspect Data that "+
						"reads from a ResultsIpi instance MUST NOT lock it",
						held.Name(), field.Name)
				}
			}
		}
	}
}
