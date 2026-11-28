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

package setheaders

import (
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The Set Headers Element accepts no Evidence, so its Evidence key filter is empty and it adds
// no key to the Pipeline's cache keys, ETag or Vary.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/pipeline-elements/set-headers-element.md?plain=1#L20-L23
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/advertize-accepted-evidence.md?plain=1#L25-L27
func TestAcceptsTheEvidenceTheSpecificationGives(t *testing.T) {
	filter := New(nil).EvidenceKeyFilter()
	lister, ok := filter.(core.KeyLister)
	if !ok {
		t.Fatalf("the filter %T cannot list its keys", filter)
	}
	if keys := lister.Keys(); len(keys) != 0 {
		t.Errorf("accepts %v, want no Evidence", keys)
	}
	for _, key := range []string{"header.user-agent", "query.sequence",
		"header.host", "cookie.51d_x", "server.client-ip"} {
		if filter.Include(key) {
			t.Errorf("accepts %s", key)
		}
	}
}
