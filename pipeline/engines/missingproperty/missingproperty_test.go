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
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// wantPrefix is the prefix every message in the specification's table begins
// with, for the Property and Engine these tests use.
const wantPrefix = "Property 'IsMobile' not found in data for element 'device'. "

// Each message below is a row of the table in the missing Properties section
// of the Pipeline specification, with the Property name and the Data Key of
// the Engine filled in. A message is compared whole, because a difference of
// one word or one capital letter is a difference between languages.
func TestMessagesMatchSpecification(t *testing.T) {
	cases := []struct {
		name string
		err  *core.PropertyMissingError
		want string
	}{
		{
			name: "data file upgrade required",
			err: ForDataFileUpgrade("IsMobile", "device", "HashEngine",
				[]string{"Premium", "Enterprise"}),
			want: wantPrefix +
				"This is because your license or data file does not include " +
				"this property. It is available with the Premium,Enterprise " +
				"license/data for the HashEngine. To upgrade, see " +
				"https://51degrees.com/pricing.",
		},
		{
			name: "data file upgrade required, data tiers not known",
			err:  ForDataFileUpgrade("IsMobile", "device", "HashEngine", nil),
			want: wantPrefix +
				"This is because your license or data file does not include " +
				"this property. To see which licenses include it, see " +
				"https://51degrees.com/pricing.",
		},
		{
			name: "property excluded in configuration",
			err:  ForExcludedInConfig("IsMobile", "device"),
			want: wantPrefix +
				"This is because the property has been excluded when " +
				"configuring the engine.",
		},
		{
			name: "product not included with Resource Key",
			err:  ForProductNotAccessible("IsMobile", "device"),
			want: wantPrefix +
				"This is because your resource key does not include access " +
				"to any properties under 'device'. Check or create a " +
				"resource key with the properties you need in the " +
				"configurator at https://configure.51degrees.com.",
		},
		{
			name: "property not included with Resource Key",
			err: ForPropertyNotAccessible("IsMobile", "device",
				[]string{"HardwareVendor", "PlatformName"}),
			want: wantPrefix +
				"This is because your resource key does not include access " +
				"to this property. The properties included for your key " +
				"under 'device' are HardwareVendor,PlatformName. Check or " +
				"change your key in the configurator at " +
				"https://configure.51degrees.com.",
		},
		{
			name: "request to the remote server failed",
			err:  ForCloudRequestFailed("IsMobile", "device"),
			want: wantPrefix +
				"This is because an upstream cloud request failed, so no " +
				"data was available to populate the property.",
		},
		{
			name: "unknown",
			err:  Generic("IsMobile", "device"),
			want: wantPrefix +
				"The reason for this is unknown. Please check that the " +
				"aspect and property name are correct.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Message != tc.want {
				t.Fatalf("message\n got %q\nwant %q", tc.err.Message, tc.want)
			}
			if tc.err.Error() != tc.want {
				t.Fatalf("Error() = %q, want %q", tc.err.Error(), tc.want)
			}
			if tc.err.PropertyName != "IsMobile" ||
				tc.err.ElementName != "device" {
				t.Fatalf("fields not set: %+v", tc.err)
			}
		})
	}
}

// A Property with no Data Key to name, which happens when a cloud request
// fails before any Engine has run, is named Unknown.
func TestCloudRequestFailedWithNoEngine(t *testing.T) {
	err := ForCloudRequestFailed("IsMobile", "")
	want := "Property 'IsMobile' not found in data for element 'Unknown'. " +
		"This is because an upstream cloud request failed, so no data was " +
		"available to populate the property."
	if err.Message != want {
		t.Fatalf("message\n got %q\nwant %q", err.Message, want)
	}
	if err.ElementName != "Unknown" {
		t.Fatalf("ElementName = %q, want %q", err.ElementName, "Unknown")
	}
}

// An empty list of Properties included with the Resource Key leaves the names
// out rather than writing anything in their place.
func TestPropertyNotInResourceKeyWithNoNames(t *testing.T) {
	err := ForPropertyNotAccessible("IsMobile", "device", nil)
	want := wantPrefix +
		"This is because your resource key does not include access to this " +
		"property. The properties included for your key under 'device' are " +
		". Check or change your key in the configurator at " +
		"https://configure.51degrees.com."
	if err.Message != want {
		t.Fatalf("message\n got %q\nwant %q", err.Message, want)
	}
}

// TestReasonTagging verifies each builder tags its error with the matching
// machine-readable reason code.
func TestReasonTagging(t *testing.T) {
	cases := []struct {
		reason string
		err    *core.PropertyMissingError
	}{
		{ReasonDataFileTier, ForDataFileUpgrade("p", "e", "Engine", nil)},
		{ReasonExcludedInConfig, ForExcludedInConfig("p", "e")},
		{ReasonProductNotInKey, ForProductNotAccessible("p", "e")},
		{ReasonPropertyNotInKey, ForPropertyNotAccessible("p", "e", nil)},
		{ReasonUnknown, Generic("p", "e")},
		{ReasonCloudRequestFailed, ForCloudRequestFailed("p", "e")},
	}
	for _, tc := range cases {
		if got := tc.err.Reason; got != tc.reason {
			t.Fatalf("reason = %q, want %q", got, tc.reason)
		}
	}
}
