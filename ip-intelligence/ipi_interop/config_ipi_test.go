/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, 5 Charlotte Close,
 * Caversham, Reading, Berkshire, United Kingdom RG4 7BY.
 *
 * This Original Work is licensed under the European Union Public Licence (EUPL)
 * v.1.2 and is subject to its terms as set out below.
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

//func TestConfigIpi_SetConcurrency(t *testing.T) {
//	tests := []struct {
//		name        string
//		concurrency uint16
//	}{
//		{
//			name:        "Set concurrency to 0",
//			concurrency: 0,
//		},
//		{
//			name:        "Set concurrency to 1",
//			concurrency: 1,
//		},
//		{
//			name:        "Set concurrency to max uint16",
//			concurrency: 65535,
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			config := NewConfigIpi(Default)
//			config.SetConcurrency(tt.concurrency)
//
//			// Verify all concurrency fields are set correctly
//			if uint16(config.CPtr.strings.concurrency) != tt.concurrency {
//				t.Errorf("strings.concurrency = %v, want %v", config.CPtr.strings.concurrency, tt.concurrency)
//			}
//			if uint16(config.CPtr.components.concurrency) != tt.concurrency {
//				t.Errorf("components.concurrency = %v, want %v", config.CPtr.components.concurrency, tt.concurrency)
//			}
//			if uint16(config.CPtr.maps.concurrency) != tt.concurrency {
//				t.Errorf("maps.concurrency = %v, want %v", config.CPtr.maps.concurrency, tt.concurrency)
//			}
//			if uint16(config.CPtr.properties.concurrency) != tt.concurrency {
//				t.Errorf("properties.concurrency = %v, want %v", config.CPtr.properties.concurrency, tt.concurrency)
//			}
//			if uint16(config.CPtr.values.concurrency) != tt.concurrency {
//				t.Errorf("values.concurrency = %v, want %v", config.CPtr.values.concurrency, tt.concurrency)
//			}
//			if uint16(config.CPtr.profiles.concurrency) != tt.concurrency {
//				t.Errorf("profiles.concurrency = %v, want %v", config.CPtr.profiles.concurrency, tt.concurrency)
//			}
//			if uint16(config.CPtr.graph.concurrency) != tt.concurrency {
//				t.Errorf("graph.concurrency = %v, want %v", config.CPtr.graph.concurrency, tt.concurrency)
//			}
//			if uint16(config.CPtr.profileGroups.concurrency) != tt.concurrency {
//				t.Errorf("profileGroups.concurrency = %v, want %v", config.CPtr.profileGroups.concurrency, tt.concurrency)
//			}
//			if uint16(config.CPtr.profileOffsets.concurrency) != tt.concurrency {
//				t.Errorf("profileOffsets.concurrency = %v, want %v", config.CPtr.profileOffsets.concurrency, tt.concurrency)
//			}
//		})
//	}
//}
//
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
