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

import "errors"

// ProfileIdsForPropertyValue gives the ids of the profiles that hold value for
// property, such as the devices with a TAC or a native model name. Both are
// matched exactly as the data file holds them, and the ids are in ascending
// order with each once. A Property named with WithIndexedProperties is found
// through the native code's index, and any other by reading every profile.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/device-detection-specification/pipeline-elements/device-detection-on-premise.md?plain=1#L477-L499
func (e *Engine) ProfileIdsForPropertyValue(property, value string) ([]uint32, error) {
	if !e.acquire() {
		return nil, errEngineStopped
	}
	defer e.release()
	if e.manager == nil {
		return nil, errors.New("the engine has no data loaded")
	}
	return e.manager.ProfileIdsForPropertyValue(property, value)
}
