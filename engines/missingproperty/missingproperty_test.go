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

package missingproperty

import (
	"strings"
	"testing"
)

func TestDataFileTierMessage(t *testing.T) {
	err := ForDataFileTier("IsMobile", "device", []string{"Enterprise", "Premium"})
	if err.PropertyName != "IsMobile" || err.ElementName != "device" {
		t.Fatalf("fields not set: %+v", err)
	}
	if !strings.Contains(err.Message, "license and/or data file does not include") {
		t.Fatalf("unexpected message: %s", err.Message)
	}
	if !strings.Contains(err.Message, "Enterprise, Premium") {
		t.Fatalf("data tiers not listed: %s", err.Message)
	}
}

func TestExcludedMessage(t *testing.T) {
	err := ForExcludedInConfig("IsMobile", "device")
	if !strings.Contains(err.Message, "excluded when configuring the Engine") {
		t.Fatalf("unexpected message: %s", err.Message)
	}
}

func TestProductNotInResourceKeyMessage(t *testing.T) {
	err := ForProductNotInResourceKey("IsMobile", "device", "device")
	if !strings.Contains(err.Message, "does not include access to any Properties under 'device'") {
		t.Fatalf("unexpected message: %s", err.Message)
	}
	if !strings.Contains(err.Message, "resource_keys.html") {
		t.Fatalf("missing explainer link: %s", err.Message)
	}
}

func TestPropertyNotInResourceKeyMessage(t *testing.T) {
	err := ForPropertyNotInResourceKey("IsMobile", "device", "device", []string{"HardwareVendor", "PlatformName"})
	if !strings.Contains(err.Message, "does not include access to this Property") {
		t.Fatalf("unexpected message: %s", err.Message)
	}
	if !strings.Contains(err.Message, "HardwareVendor, PlatformName") {
		t.Fatalf("included properties not listed: %s", err.Message)
	}
}
