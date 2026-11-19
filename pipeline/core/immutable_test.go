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

package core

import "testing"

// A base Flow Element given no filter accepts no Evidence, so an Element that
// declares nothing adds no key to the Pipeline's cache keys, ETag or Vary.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/advertize-accepted-evidence.md?plain=1#L19-L23
func TestABaseElementGivenNoFilterAcceptsNoEvidence(t *testing.T) {
	filter := NewBaseElement("element", nil, nil).EvidenceKeyFilter()
	lister, ok := filter.(KeyLister)
	if !ok {
		t.Fatalf("the default filter %T cannot list its keys", filter)
	}
	if keys := lister.Keys(); len(keys) != 0 {
		t.Errorf("the default filter accepts %v, want no Evidence", keys)
	}
	for _, key := range []string{"header.user-agent", "query.sequence",
		"cookie.51d_x", "server.client-ip"} {
		if filter.Include(key) {
			t.Errorf("the default filter accepts %s", key)
		}
	}
}

// A Pipeline cannot be changed once built, so the elements it gives are a
// copy, and changing that copy leaves the Pipeline as it was.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/conceptual-overview.md?plain=1#L118-L119
func TestFlowElementsGivesACopy(t *testing.T) {
	first := &propElement{BaseElement: NewBaseElement("first", NewWhitelistFilter(), nil)}
	second := &propElement{BaseElement: NewBaseElement("second", NewWhitelistFilter(), nil)}
	p, err := NewPipelineBuilder().AddFlowElement(first).AddFlowElement(second).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	elements := p.FlowElements()
	elements[0], elements[1] = elements[1], elements[0]
	got := p.FlowElements()
	if len(got) != 2 || got[0] != FlowElement(first) || got[1] != FlowElement(second) {
		t.Errorf("the Pipeline's elements changed with the copy it gave: %v", got)
	}
}

// A Pipeline cannot be changed once built, so using its builder again, which
// a program may do to build a second Pipeline, leaves the first as it was.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/pipeline-specification/conceptual-overview.md?plain=1#L118-L119
func TestABuilderUsedAgainLeavesItsPipelineAsItWas(t *testing.T) {
	first := &propElement{BaseElement: NewBaseElement("first", NewWhitelistFilter(), nil)}
	second := &propElement{BaseElement: NewBaseElement("second", NewWhitelistFilter(), nil)}
	b := NewPipelineBuilder().AddFlowElement(first)
	p, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	b.AddFlowElement(second)
	later, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	defer later.Close()

	got := p.FlowElements()
	if len(got) != 1 || got[0] != FlowElement(first) {
		t.Errorf("the first Pipeline holds %v, want the first element alone", got)
	}
	if p.GetElement("second") != nil {
		t.Error("the first Pipeline found an element added to its builder after it was built")
	}
	if elements := later.FlowElements(); len(elements) != 2 {
		t.Errorf("the second Pipeline holds %d elements, want 2", len(elements))
	}
}
