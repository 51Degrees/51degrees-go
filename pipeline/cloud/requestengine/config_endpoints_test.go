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

import (
	"log/slog"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// Factory takes each endpoint the configuration options table names from its
// own parameter, so every request goes to the endpoint the configuration
// names, and none to the ones made from EndPoint.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/pipeline-elements/cloud-request-engine.md?plain=1#L499-L502
func TestFactoryTakesEachEndpoint(t *testing.T) {
	base, named := newCredentialCloud(), newCredentialCloud()
	baseURL, namedURL := base.serve(t).URL, named.serve(t).URL
	element, err := Factory(map[string]any{
		"ResourceKey":          testResourceKey,
		"EndPoint":             baseURL + "/",
		"DataEndPoint":         namedURL + "/json",
		"PropertiesEndPoint":   namedURL + "/accessibleproperties",
		"EvidenceKeysEndPoint": namedURL + "/evidencekeys",
	}, slog.New(&logRecorder{}))
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	p, err := core.NewPipelineBuilder().AddFlowElement(element).Build()
	if err != nil {
		t.Fatalf("build the Pipeline: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	if err := processWith(t, p, nil); err != nil {
		t.Fatalf("process: %v", err)
	}
	for _, path := range []string{"accessibleproperties", "evidencekeys",
		"json"} {
		if _, n := named.last(path); n == 0 {
			t.Errorf("no request reached the %s endpoint the configuration "+
				"named", path)
		}
	}
	if n := base.total(); n != 0 {
		t.Errorf("%d requests went to the endpoints made from EndPoint, "+
			"want none, as the configuration names each one", n)
	}
}
