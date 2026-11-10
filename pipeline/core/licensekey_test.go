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

import (
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// A key shows only its last 4 characters, each hidden one replaced by '*',
// and a key of 4 or fewer is replaced in full.
func TestMaskLicenseKey(t *testing.T) {
	cases := map[string]string{
		"":                               "",
		"A":                              "*",
		"ABCD":                           "****",
		"ABCDE":                          "*BCDE",
		"0123456789ABCDEF":               "************CDEF",
		strings.Repeat("K", 32) + "WXYZ": strings.Repeat("*", 32) + "WXYZ",
	}
	for key, want := range cases {
		if got := MaskLicenseKey(key); got != want {
			t.Errorf("MaskLicenseKey(%q) = %q, want %q", key, got, want)
		}
	}
}

// The keys are read from each license key parameter, whatever its case, and
// split where a parameter holds several.
func TestLicenseKeysInURL(t *testing.T) {
	cases := []struct {
		url  string
		want []string
	}{
		{"https://d.example/api?LicenseKeys=ABCDEFGH1234&Type=HashV41",
			[]string{"ABCDEFGH1234"}},
		{"https://d.example/api?licensekeys=ABCDEFGH1234%7CZYXWVUTS5678",
			[]string{"ABCDEFGH1234", "ZYXWVUTS5678"}},
		{"https://d.example/api?Type=HashV41&license=KEY&LicenseKey=SECOND",
			[]string{"KEY", "SECOND"}},
		{"https://d.example/api?LicenseKeys=ABCDEFGH1234#part",
			[]string{"ABCDEFGH1234"}},
		{"http://bad/\x7f?LicenseKeys=ABCDEFGH1234", []string{"ABCDEFGH1234"}},
		{"https://d.example/api?LicenseKeys=", nil},
		{"https://d.example/api?Type=HashV41", nil},
		{"https://d.example/api", nil},
	}
	for _, c := range cases {
		if got := LicenseKeysInURL(c.url); !reflect.DeepEqual(got, c.want) {
			t.Errorf("LicenseKeysInURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

// Each key is masked wherever it appears, as written and as encoded in a
// URL, and a key holding another is masked whole.
func TestMaskLicenseKeysIn(t *testing.T) {
	cases := []struct {
		text string
		keys []string
		want string
	}{
		{"https://d.example/api?LicenseKeys=ABCDEFGH1234&Type=HashV41",
			[]string{"ABCDEFGH1234"},
			"https://d.example/api?LicenseKeys=********1234&Type=HashV41"},
		{"get ABCDEFGH1234|ZYXWVUTS5678 failed",
			[]string{"ABCDEFGH1234", "ZYXWVUTS5678"},
			"get ********1234|********5678 failed"},
		{"key=AB/CD+EF=12 or AB%2FCD%2BEF%3D12",
			[]string{"AB/CD+EF=12"},
			"key=*******F=12 or *************3D12"},
		{"ABCDEFGH1234 and ABCDEFGH", []string{"ABCDEFGH", "ABCDEFGH1234"},
			"********1234 and ****EFGH"},
		{"no keys here", []string{"ABCDEFGH1234"}, "no keys here"},
		{"no keys given", nil, "no keys given"},
	}
	for _, c := range cases {
		if got := MaskLicenseKeysIn(c.text, c.keys); got != c.want {
			t.Errorf("MaskLicenseKeysIn(%q, %q) = %q, want %q",
				c.text, c.keys, got, c.want)
		}
	}
}

// The URL in a net/http error is masked at every level it appears and the
// error keeps its type, another error holding a key is replaced by one with
// the masked message, and an error holding none comes back as it was.
func TestMaskLicenseKeysInError(t *testing.T) {
	const raw = "http://bad/\x7f?LicenseKeys=ABCDEFGH1234"
	keys := []string{"ABCDEFGH1234"}
	_, err := http.NewRequest(http.MethodGet, raw, nil)
	if err == nil {
		t.Fatal("the request was made from a URL that cannot be parsed")
	}
	masked := MaskLicenseKeysInError(err, keys)
	var urlErr *url.Error
	if !errors.As(masked, &urlErr) {
		t.Errorf("the masked error %T is no longer a *url.Error", masked)
	}
	if text := masked.Error(); strings.Contains(text, "ABCDEFGH1234") ||
		!strings.Contains(text, "********1234") {
		t.Errorf("the error reads %q, want only the masked key", text)
	}

	nested := &url.Error{Op: "Get", URL: raw,
		Err: &url.Error{Op: "Get", URL: raw, Err: errors.New("redirect")}}
	if text := MaskLicenseKeysInError(nested, keys).Error(); strings.Contains(text, "ABCDEFGH1234") {
		t.Errorf("the nested error reads %q, want only the masked key", text)
	}

	plain := errors.New("refused ABCDEFGH1234")
	if text := MaskLicenseKeysInError(plain, keys).Error(); text != "refused ********1234" {
		t.Errorf("the plain error reads %q, want the masked key", text)
	}

	other := errors.New("no key here")
	if got := MaskLicenseKeysInError(other, keys); got != other {
		t.Errorf("an error holding no key came back as %v", got)
	}
	if MaskLicenseKeysInError(nil, keys) != nil {
		t.Error("nil came back as an error")
	}
}
