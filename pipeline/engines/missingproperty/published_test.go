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

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// The three builders below were published with these parameters, and a patch
// version does not change the public API of a Go module, so a program
// written against them still builds.
// See https://go.dev/doc/modules/version-numbers
var (
	_ func(property, element string,
		dataTiers []string) *core.PropertyMissingError = ForDataFileTier
	_ func(property, element,
		product string) *core.PropertyMissingError = ForProductNotInResourceKey
	_ func(property, element, product string,
		includedProperties []string) *core.PropertyMissingError = ForPropertyNotInResourceKey
)

// same compares the reason and the whole message of two errors.
func same(t *testing.T, got, want *core.PropertyMissingError) {
	t.Helper()
	if got.Reason != want.Reason || got.Error() != want.Error() {
		t.Errorf("\n got: %s, %s\nwant: %s, %s",
			got.Reason, got.Error(), want.Reason, want.Error())
	}
}

// ForDataFileTier is not given the Engine's type, so it names the Engine by
// its Data Key, and otherwise gives the message ForDataFileUpgrade gives.
func TestPublishedDataFileBuilder(t *testing.T) {
	tiers := []string{"Premium", "Enterprise"}
	same(t, ForDataFileTier("IsMobile", "device", tiers),
		ForDataFileUpgrade("IsMobile", "device", "device", tiers))
	same(t, ForDataFileTier("IsMobile", "device", nil),
		ForDataFileUpgrade("IsMobile", "device", "device", nil))
}

// The published Resource Key builders name the product they are given where
// the specification's message names the Data Key a second time, so a caller
// that gives the Data Key as the product gets the specification's message.
func TestPublishedResourceKeyBuilders(t *testing.T) {
	same(t, ForProductNotInResourceKey("IsMobile", "device", "device"),
		ForProductNotAccessible("IsMobile", "device"))
	included := []string{"IsMobile", "HardwareVendor"}
	same(t,
		ForPropertyNotInResourceKey("ScreenWidth", "device", "device", included),
		ForPropertyNotAccessible("ScreenWidth", "device", included))

	product := ForProductNotInResourceKey("IsMobile", "device", "hardware")
	if !strings.HasPrefix(product.Error(), "Property 'IsMobile' not found in "+
		"data for element 'device'. ") ||
		!strings.Contains(product.Error(), "any properties under 'hardware'.") {
		t.Errorf("the product is not named: %s", product.Error())
	}
	property := ForPropertyNotInResourceKey("ScreenWidth", "device",
		"hardware", included)
	if !strings.Contains(property.Error(),
		"under 'hardware' are IsMobile,HardwareVendor.") {
		t.Errorf("the product is not named: %s", property.Error())
	}
}
