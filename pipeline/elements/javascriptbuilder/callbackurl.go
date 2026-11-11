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
	"net/netip"
	"strconv"
	"strings"
)

// userInfoMarker is what the log shows in place of anything in a refused
// host value before its last "@".
const userInfoMarker = "***"

// buildCallbackURL builds the callback URL the script posts to for updated
// JSON from its parts and checks it. The protocol and host usually come
// from the request, and the URL is written inside a single-quoted
// JavaScript string, so it is checked and encoded first.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/pipeline-elements/javascript-builder.md#callback-url
//
// valid is the URL for the script, or "" when there is none. refused is set
// when the URL fails the rules, being the URL as built for the log, with
// anything in the host value before its last "@" shown as "***".
func buildCallbackURL(protocol, host, endpoint string) (valid, refused string) {
	if isBlank(protocol) || isBlank(host) || isBlank(endpoint) {
		return "", ""
	}
	hostSlash := strings.HasSuffix(host, "/")
	endpointSlash := strings.HasPrefix(endpoint, "/")
	if !hostSlash && !endpointSlash {
		endpoint = "/" + endpoint
	} else if hostSlash && endpointSlash {
		endpoint = endpoint[1:]
	}
	authority, rest := host, ""
	if i := strings.IndexByte(host, '/'); i >= 0 {
		authority, rest = host[:i], host[i:]
	}
	if isScheme(protocol) && !strings.ContainsAny(host, "?#@") {
		if a, ok := checkedAuthority(authority); ok {
			return strings.ToLower(protocol) + "://" + a + encoded(rest) +
				encoded(endpoint), ""
		}
	}
	if i := strings.LastIndexByte(host, '@'); i >= 0 {
		host = userInfoMarker + host[i:]
	}
	return "", protocol + "://" + host + endpoint
}

// isBlank reports whether s is empty or only characters with the Unicode
// White_Space property, which is what unicode.IsSpace tests.
func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

// isScheme reports whether the protocol is http or https, ignoring ASCII
// case only, so that a letter outside ASCII never matches one inside it.
func isScheme(protocol string) bool {
	lower := []byte(protocol)
	for i, c := range lower {
		if 'A' <= c && c <= 'Z' {
			lower[i] = c + 'a' - 'A'
		}
	}
	return string(lower) == "http" || string(lower) == "https"
}

// checkedAuthority checks a host and an optional port, and returns them with
// the host in lower case. The host is an IPv6 address in brackets with no
// zone, or a name of non-empty labels separated by dots, with a dot allowed
// at the end. A name that ends in a number, ignoring a dot at its end, must
// be an IPv4 address with no dot at its end, because a browser reads it as
// one.
func checkedAuthority(authority string) (string, bool) {
	host, port := authority, ""
	if inner, ok := strings.CutPrefix(authority, "["); ok {
		address, _, found := strings.Cut(inner, "]")
		if !found || !isIPv6(address) {
			return "", false
		}
		host, port = authority[:len(address)+2], authority[len(address)+2:]
	} else {
		if i := strings.IndexByte(authority, ':'); i >= 0 {
			host, port = authority[:i], authority[i:]
		}
		name := strings.TrimSuffix(host, ".")
		for _, label := range strings.Split(name, ".") {
			if !isLabel(label) {
				return "", false
			}
		}
		if endsInANumber(name) && !isIPv4(host) {
			return "", false
		}
	}
	if port != "" && !isPort(port) {
		return "", false
	}
	return strings.ToLower(host) + port, true
}

// isIPv6 reports whether s is an IPv6 address with no zone. A browser's URL
// parser fails on a "%" inside the brackets.
func isIPv6(s string) bool {
	addr, err := netip.ParseAddr(s)
	return err == nil && addr.Is6() && !strings.Contains(s, "%")
}

// isLabel reports whether s is a label of a host name, being ASCII letters,
// digits, "-" and "_". An internationalized name is given as A-labels.
func isLabel(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !isDigit(c) && !('a' <= c|0x20 && c|0x20 <= 'z') &&
			c != '-' && c != '_' {
			return false
		}
	}
	return s != ""
}

// endsInANumber reports whether the last label of a name is all digits, or
// "0x" or "0X" followed by zero or more hexadecimal digits, which is when
// the URL Standard's host parser reads the name as an IPv4 address.
func endsInANumber(name string) bool {
	last := name[strings.LastIndexByte(name, '.')+1:]
	if len(last) >= 2 && last[0] == '0' && last[1]|0x20 == 'x' {
		return allBytes(last[2:], isHex)
	}
	return last != "" && allBytes(last, isDigit)
}

// isIPv4 reports whether s is four decimal numbers from 0 to 255 separated
// by dots, each starting with 0 only when it is 0 alone.
func isIPv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		// Base 10 takes no sign and no "_", and a value above 255 is a
		// range error.
		_, err := strconv.ParseUint(part, 10, 8)
		if err != nil || (part[0] == '0' && len(part) > 1) {
			return false
		}
	}
	return true
}

// isPort reports whether s is ":" followed by one or more digits whose
// value is no more than 65535. Leading zeros do not change the value.
func isPort(s string) bool {
	digits, ok := strings.CutPrefix(s, ":")
	if !ok {
		return false
	}
	// Base 10 takes no sign and no "_", an empty string is a syntax error
	// and a value above 65535 is a range error.
	_, err := strconv.ParseUint(digits, 10, 16)
	return err == nil
}

// encoded percent-encodes each byte that is a control character, is outside
// ASCII, is a space, or is one of the characters rule 8 of the callback URL
// lists, and a "%" that does not start an escape.
func encoded(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		keep := c > ' ' && c < 0x7f && !strings.ContainsRune("\"'<>\\^`{|}", rune(c))
		if c == '%' {
			keep = i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2])
		}
		if keep {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0xf])
		}
	}
	return b.String()
}

func allBytes(s string, f func(byte) bool) bool {
	for i := 0; i < len(s); i++ {
		if !f(s[i]) {
			return false
		}
	}
	return true
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

func isHex(c byte) bool { return isDigit(c) || ('a' <= c|0x20 && c|0x20 <= 'f') }
