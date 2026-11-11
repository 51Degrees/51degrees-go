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

package common_go

// A license key never appears in full in a log entry or an error message.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/logging.md#license-keys

import (
	"errors"
	"net/url"
	"sort"
	"strings"
)

// licenseKeyVisible is how many of a license key's last characters a message
// may show.
const licenseKeyVisible = 4

// licenseKeyMask replaces each hidden character of a license key.
const licenseKeyMask = "*"

// licenseKeyParameters are the query parameters, compared without regard to
// case, whose values are license keys.
var licenseKeyParameters = map[string]bool{
	"licensekeys": true,
	"licensekey":  true,
	"license":     true,
}

// licenseKeySeparator separates several license keys in one parameter.
const licenseKeySeparator = "|"

// MaskLicenseKey returns key with every character before its last 4 replaced
// by '*', so two keys can be told apart but neither can be used. A key of 4
// characters or fewer is replaced in full.
func MaskLicenseKey(key string) string {
	r := []rune(key)
	hidden := len(r)
	if hidden > licenseKeyVisible {
		hidden -= licenseKeyVisible
	}
	return strings.Repeat(licenseKeyMask, hidden) + string(r[hidden:])
}

// LicenseKeysInURL returns the license keys in rawURL's query, being the
// values of its LicenseKeys, LicenseKey and License parameters, split where
// one holds several separated by '|'. The query is read by splitting it, so
// the keys of a URL that cannot be parsed are found too.
func LicenseKeysInURL(rawURL string) []string {
	start := strings.IndexByte(rawURL, '?')
	if start < 0 {
		return nil
	}
	query := rawURL[start+1:]
	if end := strings.IndexByte(query, '#'); end >= 0 {
		query = query[:end]
	}
	var keys []string
	for _, pair := range strings.Split(query, "&") {
		name, value, found := strings.Cut(pair, "=")
		if !found || !licenseKeyParameters[strings.ToLower(unescape(name))] {
			continue
		}
		for _, key := range strings.Split(unescape(value), licenseKeySeparator) {
			if key != "" {
				keys = append(keys, key)
			}
		}
	}
	return keys
}

// MaskLicenseKeysIn returns text, such as a URL or an error message, with
// every occurrence of each key masked by MaskLicenseKey, both as the key is
// written and as it is encoded in a URL. Longer keys are masked first, so a
// key that holds another is masked whole.
func MaskLicenseKeysIn(text string, keys []string) string {
	for _, key := range longestFirst(keys) {
		text = strings.ReplaceAll(text, key, MaskLicenseKey(key))
		if encoded := url.QueryEscape(key); encoded != key {
			text = strings.ReplaceAll(text, encoded, MaskLicenseKey(encoded))
		}
	}
	return text
}

// MaskLicenseKeysInError returns err with no key in its message. Each URL it
// names through a *url.Error, which is how net/http reports a request it
// could not make or a URL it could not parse, is masked in place, so the
// error keeps its type. An error whose message still holds a key is returned
// as a new error carrying only the masked message.
func MaskLicenseKeysInError(err error, keys []string) error {
	if err == nil || len(keys) == 0 {
		return err
	}
	var urlErr *url.Error
	for next := err; errors.As(next, &urlErr); next = urlErr.Err {
		urlErr.URL = MaskLicenseKeysIn(urlErr.URL, keys)
	}
	text := err.Error()
	if masked := MaskLicenseKeysIn(text, keys); masked != text {
		return errors.New(masked)
	}
	return err
}

// unescape decodes a query component, or returns it as it is when it is not
// valid encoding.
func unescape(s string) string {
	if decoded, err := url.QueryUnescape(s); err == nil {
		return decoded
	}
	return s
}

// longestFirst returns the keys that are not empty, longest first.
func longestFirst(keys []string) []string {
	sorted := make([]string, 0, len(keys))
	for _, key := range keys {
		if key != "" {
			sorted = append(sorted, key)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})
	return sorted
}
