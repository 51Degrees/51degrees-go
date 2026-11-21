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
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/cloud-request-engine.md#resilience

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
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

// A discovery state that lacks either result, such as one read from an
// empty JSON file, fails the build with a configuration error and no
// request, because the engine would never fetch what is missing. A result
// that is present and empty is kept, as it is when the cloud service gives
// one.
func TestIncompleteDiscoveryStateFailsTheBuild(t *testing.T) {
	var fromEmptyJSON DiscoveryState
	if err := json.Unmarshal([]byte(`{}`), &fromEmptyJSON); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cases := []struct {
		name  string
		state *DiscoveryState
	}{
		{"empty JSON", &fromEmptyJSON},
		{"no Evidence keys", &DiscoveryState{Products: map[string]ProductMeta{}}},
		{"no Products", &DiscoveryState{EvidenceKeys: []string{"header.user-agent"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cloud := &discoveryCloud{}
			b := discoveryBuilder(cloud.serve(t), &logRecorder{}).
				SetDiscoveryState(c.state)
			e, err := b.Build()
			var configErr *core.ConfigurationError
			if e != nil || !errors.As(err, &configErr) {
				t.Fatalf("build gives %v, %v, want a configuration error", e, err)
			}
			wantCounts(t, cloud, "after the build", [3]int64{0, 0, 0})
			if s := b.DiscoveryState(); s != nil {
				t.Errorf("after a failed build the state is %+v, want none", s)
			}
		})
	}

	t.Run("present and empty", func(t *testing.T) {
		cloud := &discoveryCloud{}
		e, err := discoveryBuilder(cloud.serve(t), &logRecorder{}).
			SetDiscoveryState(&DiscoveryState{
				EvidenceKeys: []string{},
				Products:     map[string]ProductMeta{},
			}).
			Build()
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if products, err := e.PublicProperties(); err != nil || len(products) != 0 {
			t.Fatalf("PublicProperties gives %v, %v, want none", products, err)
		}
		wantCounts(t, cloud, "after the build and a read", [3]int64{0, 0, 0})
	})
}

// itemState is a discovery state whose one Property has item Properties.
func itemState() *DiscoveryState {
	return &DiscoveryState{
		EvidenceKeys: []string{"header.user-agent"},
		Products: map[string]ProductMeta{"device": {
			DataTier: "CloudV4",
			Properties: []PropertyMeta{{
				Name:     "profiles",
				Type:     "Array",
				Category: "Device",
				ItemProperties: []PropertyMeta{
					{Name: "profileid", Type: "String", Category: "Device"},
				},
			}},
		}},
	}
}

// change alters every part of s that a caller could reach.
func change(s *DiscoveryState) {
	s.EvidenceKeys[0] = "header.changed"
	device := s.Products["device"]
	device.Properties[0].ItemProperties[0].Name = "changed"
	device.Properties[0].Name = "changed"
	s.Products["other"] = ProductMeta{}
}

// An engine shares nothing with the discovery state it was built from or
// the one its builder gives back, item Properties included, so changing
// either state, or what PublicProperties gives, does not change the engine.
func TestDiscoveryStateIsCopied(t *testing.T) {
	given := itemState()
	cloud := &discoveryCloud{}
	b := discoveryBuilder(cloud.serve(t), &logRecorder{}).SetDiscoveryState(given)
	e, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	change(given)
	change(b.DiscoveryState())
	products, err := e.PublicProperties()
	if err != nil {
		t.Fatalf("PublicProperties: %v", err)
	}
	products["device"].Properties[0].ItemProperties[0].Name = "changed"

	want := itemState()
	wantKeys(t, e.EvidenceKeyFilter(), want.EvidenceKeys)
	if got, err := e.PublicProperties(); err != nil ||
		!reflect.DeepEqual(got, want.Products) {
		t.Fatalf("after the states and a copy changed, PublicProperties "+
			"gives %+v, %v, want %+v", got, err, want.Products)
	}
}

// The Evidence Properties of a Property are copied with the rest, so
// changing them in a discovery state, or in what PublicProperties gives,
// does not change the engine.
func TestEvidencePropertiesAreCopied(t *testing.T) {
	newState := func() *DiscoveryState {
		s := itemState()
		s.Products["device"].Properties[0].EvidenceProperties =
			[]string{"device.javascriptprofile"}
		return s
	}
	given := newState()
	cloud := &discoveryCloud{}
	b := discoveryBuilder(cloud.serve(t), &logRecorder{}).SetDiscoveryState(given)
	e, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	given.Products["device"].Properties[0].EvidenceProperties[0] = "changed"
	b.DiscoveryState().Products["device"].Properties[0].EvidenceProperties[0] = "changed"
	products, err := e.PublicProperties()
	if err != nil {
		t.Fatalf("PublicProperties: %v", err)
	}
	products["device"].Properties[0].EvidenceProperties[0] = "changed"

	want := newState().Products
	if got, err := e.PublicProperties(); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("after the states and a copy changed, PublicProperties "+
			"gives %+v, %v, want %+v", got, err, want)
	}
}
