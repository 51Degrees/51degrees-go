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

package web

import (
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/usagesharing"
)

// The web Pipeline puts the usage sharing element last, after the elements
// it adds itself, wherever the caller gives it.
// See https://github.com/51Degrees/specifications/blob/a90ba516a3bfadc58dd7a51535d83af6bdaed291/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L106-L109
func TestAssemblePipelinePutsUsageSharingLast(t *testing.T) {
	sharing := usagesharing.NewBuilder().SetSharePercentage(0).
		SetShareUsageURL("http://127.0.0.1:1/").Build()
	p, err := AssemblePipeline(PipelineConfig{}, sharing, newHeaderEngine())
	if err != nil {
		sharing.Close()
		t.Fatalf("assemble: %v", err)
	}
	defer p.Close()
	elements := p.FlowElements()
	if last := elements[len(elements)-1]; last.DataKey() != usagesharing.DataKey {
		t.Errorf("the last element is %s, want %s", last.DataKey(),
			usagesharing.DataKey)
	}
}
