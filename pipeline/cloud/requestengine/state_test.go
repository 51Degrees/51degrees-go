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

// These tests follow the specification's rule for building from a kept copy
// of the discovery results.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#resilience

import (
	"encoding/json"
	"reflect"
	"testing"
)

// The discovery state of one build, kept as JSON, lets a later build with the
// same Resource Key make no discovery request, even while the cloud service
// fails, and gives that engine the same Evidence keys and Properties.
func TestDiscoveryStateSkipsDiscovery(t *testing.T) {
	first := &discoveryCloud{}
	b := discoveryBuilder(first.serve(t), &logRecorder{})
	if b.DiscoveryState() != nil {
		t.Fatal("the builder has a discovery state before any build")
	}
	if _, err := b.Build(); err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, err := json.Marshal(b.DiscoveryState())
	if err != nil {
		t.Fatalf("marshal the discovery state: %v", err)
	}
	var kept DiscoveryState
	if err := json.Unmarshal(raw, &kept); err != nil {
		t.Fatalf("unmarshal the discovery state %s: %v", raw, err)
	}
	if !reflect.DeepEqual(kept.EvidenceKeys, discoveryKeys) ||
		len(kept.Products["device"].Properties) != 1 {
		t.Fatalf("the discovery state is %s, want the stand-in's keys and "+
			"device product", raw)
	}

	second := &discoveryCloud{}
	second.propertiesStatus.Store(503)
	second.evidenceStatus.Store(503)
	log := &logRecorder{}
	b2 := discoveryBuilder(second.serve(t), log).SetDiscoveryState(&kept)
	e, err := b2.Build()
	if err != nil {
		t.Fatalf("build from the discovery state: %v", err)
	}
	wantKeys(t, e.EvidenceKeyFilter(), discoveryKeys)
	products, err := e.PublicProperties()
	if err != nil || !reflect.DeepEqual(products, kept.Products) {
		t.Fatalf("PublicProperties gives %v, %v, want %v", products, err,
			kept.Products)
	}
	if _, err := process(pipelineOf(t, e, false)); err != nil {
		t.Fatalf("process: %v", err)
	}
	wantCounts(t, second, "after a build from the state and a request",
		[3]int64{0, 0, 1})
	if records := log.all(); len(records) != 0 {
		t.Errorf("a build from the state logged %q", records[0].Message)
	}
	if got := b2.DiscoveryState(); !reflect.DeepEqual(got, &kept) {
		t.Errorf("the builder gives back %+v, want %+v", got, kept)
	}
}

// A build that could not fetch both discovery results, or that failed, has
// no discovery state, so a host never keeps an incomplete one.
func TestNoDiscoveryStateWithoutBothResults(t *testing.T) {
	cloud := &discoveryCloud{}
	b := discoveryBuilder(cloud.serve(t), &logRecorder{})
	if _, err := b.Build(); err != nil || b.DiscoveryState() == nil {
		t.Fatalf("a healthy build gives %v and the state %v", err,
			b.DiscoveryState())
	}

	cloud.evidenceStatus.Store(503)
	if _, err := b.Build(); err != nil {
		t.Fatalf("build: %v", err)
	}
	if s := b.DiscoveryState(); s != nil {
		t.Errorf("after a transient failure the state is %+v, want none", s)
	}

	cloud.evidenceStatus.Store(401)
	if _, err := b.Build(); err == nil {
		t.Fatal("a build refused with 401 succeeded")
	}
	if s := b.DiscoveryState(); s != nil {
		t.Errorf("after a failed build the state is %+v, want none", s)
	}
}
