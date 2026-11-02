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

//#include <string.h>
//#include "ip-intelligence-cxx.h"
import "C"

// Performance Profile
type PerformanceProfile int

const (
	Default PerformanceProfile = iota
	LowMemory
	BalancedTemp
	Balanced
	HighPerformance
	InMemory
)

// ConfigIpi wraps around pointer to a value of C ConfigIpi structure
type ConfigIpi struct {
	CPtr *C.ConfigIpi
	perf PerformanceProfile
}

/* Constructor and Destructor */

// NewConfigIpi creates a new ConfigIpi object. Target performance profile
// is required as initial setup and any following adjustments will be done
// on top of this initial performance profile. Any invalid input will result
// in default config to be used. In C the default performance profile is the
// same as balanced profile.
func NewConfigIpi(perf PerformanceProfile) *ConfigIpi {
	var config C.ConfigIpi
	profile := perf
	switch perf {
	case InMemory:
		config = C.IpiInMemoryConfig
	case HighPerformance:
		config = C.IpiHighPerformanceConfig
	case LowMemory:
		config = C.IpiLowMemoryConfig
	case Balanced:
		config = C.IpiBalancedConfig
	case BalancedTemp:
		config = C.IpiBalancedTempConfig
	default:
		config = C.IpiDefaultConfig
		profile = Default
	}
	return &ConfigIpi{&config, profile}
}

// SetConcurrency sets the expected concurrent requests.
func (config *ConfigIpi) SetConcurrency(concurrency uint16) {
	config.CPtr.strings.concurrency = C.ushort(concurrency)
	config.CPtr.components.concurrency = C.ushort(concurrency)
	config.CPtr.maps.concurrency = C.ushort(concurrency)
	config.CPtr.properties.concurrency = C.ushort(concurrency)
	config.CPtr.values.concurrency = C.ushort(concurrency)
	config.CPtr.profiles.concurrency = C.ushort(concurrency)
	config.CPtr.graph.concurrency = C.ushort(concurrency)
	config.CPtr.graphs.concurrency = C.ushort(concurrency)
	config.CPtr.profileGroups.concurrency = C.ushort(concurrency)
	config.CPtr.profileOffsets.concurrency = C.ushort(concurrency)

}

// PerformanceProfile get the configured performance profile
func (config *ConfigIpi) PerformanceProfile() PerformanceProfile {
	return config.perf
}
