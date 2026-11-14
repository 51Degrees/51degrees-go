/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, Davidson House,
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
package ipi_interop

import (
	"testing"
)

func TestNewConfigIpi(t *testing.T) {
	tests := []struct {
		name        string
		profile     PerformanceProfile
		wantProfile PerformanceProfile
	}{
		{
			name:        "Default profile",
			profile:     Default,
			wantProfile: Default,
		},
		{
			name:        "LowMemory profile",
			profile:     LowMemory,
			wantProfile: LowMemory,
		},
		{
			name:        "BalancedTemp profile",
			profile:     BalancedTemp,
			wantProfile: BalancedTemp,
		},
		{
			name:        "Balanced profile",
			profile:     Balanced,
			wantProfile: Balanced,
		},
		{
			name:        "HighPerformance profile",
			profile:     HighPerformance,
			wantProfile: HighPerformance,
		},
		{
			name:        "InMemory profile",
			profile:     InMemory,
			wantProfile: InMemory,
		},
		{
			name:        "Invalid profile",
			profile:     PerformanceProfile(999),
			wantProfile: Default,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewConfigIpi(tt.profile)
			if config == nil {
				t.Fatal("NewConfigIpi returned nil")
			}
			if config.CPtr == nil {
				t.Error("ConfigIpi.CPtr is nil")
			}
			if got := config.PerformanceProfile(); got != tt.wantProfile {
				t.Errorf("NewConfigIpi() profile = %v, want %v", got, tt.wantProfile)
			}
		})
	}
}

// SetConcurrency sets the concurrency of every collection the native
// configuration holds, as the C++ ConfigIpi::setConcurrency does, so each
// collection's file handles and cache parts follow it.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md#L620
func TestConfigIpi_SetConcurrency(t *testing.T) {
	for _, concurrency := range []uint16{1, 7, 65535} {
		config := NewConfigIpi(Default)
		config.SetConcurrency(concurrency)
		for name, got := range map[string]uint16{
			"strings":        uint16(config.CPtr.strings.concurrency),
			"components":     uint16(config.CPtr.components.concurrency),
			"maps":           uint16(config.CPtr.maps.concurrency),
			"properties":     uint16(config.CPtr.properties.concurrency),
			"values":         uint16(config.CPtr.values.concurrency),
			"profiles":       uint16(config.CPtr.profiles.concurrency),
			"graphs":         uint16(config.CPtr.graphs.concurrency),
			"profileGroups":  uint16(config.CPtr.profileGroups.concurrency),
			"profileOffsets": uint16(config.CPtr.profileOffsets.concurrency),
			"propertyTypes":  uint16(config.CPtr.propertyTypes.concurrency),
			"graph":          uint16(config.CPtr.graph.concurrency),
		} {
			if got != concurrency {
				t.Errorf("SetConcurrency(%d) left %s at %d", concurrency, name,
					got)
			}
		}
	}
}

//func TestConfigIpi_PerformanceProfile(t *testing.T) {
//	tests := []struct {
//		name    string
//		profile PerformanceProfile
//	}{
//		{
//			name:    "Get Default profile",
//			profile: Default,
//		},
//		{
//			name:    "Get InMemory profile",
//			profile: InMemory,
//		},
//		{
//			name:    "Get HighPerformance profile",
//			profile: HighPerformance,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			config := NewConfigIpi(tt.profile)
//			if got := config.perf; got != tt.profile {
//				t.Errorf("PerformanceProfile() = %v, want %v", got, tt.profile)
//			}
//		})
//	}
//}
