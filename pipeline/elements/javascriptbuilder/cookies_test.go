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

package javascriptbuilder

import (
	"strings"
	"testing"
)

// whiteSpace is every character with the Unicode White_Space property, as the
// JavaScript Builder specification lists them.
var whiteSpace = string([]rune{
	0x0009, 0x000a, 0x000b, 0x000c, 0x000d, 0x0020, 0x0085, 0x00a0, 0x1680,
	0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008,
	0x2009, 0x200a, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000,
})

// Cookie evidence that is absent, blank, meaning empty or only white space, or
// neither true nor false, leaves the configured default in place. True and
// false decide, in any letter case and with any white space around them. The
// script reads document.cookie when cookies are on, and never when they are
// off.
func TestEnableCookiesEvidence(t *testing.T) {
	// The script either follows the configured default, or is on or off
	// whatever the default is.
	const followsDefault, on, off = "default", "on", "off"
	data := []struct {
		what     string
		value    string
		present  bool
		expected string
	}{
		{"no cookie evidence", "", false, followsDefault},
		{"an empty value", "", true, followsDefault},
		{"a single space", " ", true, followsDefault},
		{"only white space", whiteSpace, true, followsDefault},
		{"true", "true", true, on},
		{"TRUE", "TRUE", true, on},
		{"true with spaces around it", " true ", true, on},
		{"false", "false", true, off},
		{"FALSE with spaces around it", " FALSE ", true, off},
		{"a word that is not a boolean", "yes", true, followsDefault},
		{"a number", "1", true, followsDefault},
	}
	for _, d := range data {
		for _, configured := range []bool{true, false} {
			want := configured
			switch d.expected {
			case on:
				want = true
			case off:
				want = false
			}
			evidence := map[string]string{}
			if d.present {
				evidence[EvidenceEnableCookies] = d.value
			}
			js := renderElement(
				t, NewBuilder().SetEnableCookies(configured).Build(), evidence)
			got := strings.Contains(js, "document.cookie")
			if got != want {
				t.Errorf("with %s and cookies configured %v, the script "+
					"reads document.cookie %v, want %v",
					d.what, configured, got, want)
			}
		}
	}
}
