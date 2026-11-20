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

package requestengine

import "github.com/51Degrees/51degrees-go/pipeline/v4/core"

// processAndRead processes fd, then, where the engine started a request,
// reads json-response as a Cloud Aspect Engine after the engine does, and
// gives the error of either. The engine returns before its request has
// answered, so a test that looks at the request, or at what came of it,
// reads the response first.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/pipeline-elements/cloud-request-engine.md?plain=1#L364-L382
func processAndRead(fd core.FlowData) error {
	if err := fd.Process(); err != nil {
		return err
	}
	data, err := fd.Get(DataKey)
	if err != nil {
		return nil
	}
	if crd, ok := data.(*CloudRequestData); ok && crd.task != nil {
		_, err = crd.Response()
		return err
	}
	return nil
}
