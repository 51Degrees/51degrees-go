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

package ipi_onpremise

import (
	"fmt"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

// mappedTestAddresses are IPv4 addresses with a range and values in the
// Enterprise data file.
var mappedTestAddresses = []string{"185.28.167.77", "8.8.8.8", "81.2.69.142"}

// mappedForms returns ways of writing ipv4 as an IPv4-mapped IPv6 address.
func mappedForms(ipv4 string) []string {
	b := netip.MustParseAddr(ipv4).As4()
	high := uint16(b[0])<<8 | uint16(b[1])
	low := uint16(b[2])<<8 | uint16(b[3])
	return []string{
		"::ffff:" + ipv4,
		"::FFFF:" + ipv4,
		fmt.Sprintf("::ffff:%x:%x", high, low),
		fmt.Sprintf("0:0:0:0:0:ffff:%x:%x", high, low),
		"[::ffff:" + ipv4 + "]:8080",
	}
}

// processValues looks up address and fails the test on an error.
func processValues(t *testing.T, e *Engine, address string) ipi_interop.Values {
	t.Helper()
	values, err := e.Process(address)
	if err != nil {
		t.Fatalf("process %s: %v", address, err)
	}
	return values
}

// rangeStart returns the first IpRangeStart value, or nil when there is none.
func rangeStart(values ipi_interop.Values) any {
	for name, list := range values {
		if strings.EqualFold(name, "IpRangeStart") && len(list) > 0 {
			return list[0].Value
		}
	}
	return nil
}

// An IPv4-mapped IPv6 address is its IPv4 address, so every way of writing
// it gets the IPv4 address's values, the range properties included. The
// IPv4-compatible and IPv4-embedded forms are IPv6 addresses.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md#ipv4-mapped-addresses
func TestMappedAddressGetsItsIPv4Values(t *testing.T) {
	e := propertiesTestEngine(
		t, testDataPath(t, enterpriseTestDataFile), nil, false)
	for _, ipv4 := range mappedTestAddresses {
		want := processValues(t, e, ipv4)
		start := rangeStart(want)
		if start == nil {
			t.Fatalf("%s gave no IpRangeStart", ipv4)
		}
		for _, form := range mappedForms(ipv4) {
			if got := processValues(t, e, form); !reflect.DeepEqual(got, want) {
				t.Errorf("%s gave IpRangeStart %v and %d properties, where "+
					"%s gave %v and %d", form, rangeStart(got), len(got),
					ipv4, start, len(want))
			}
		}
		for _, form := range []string{"::" + ipv4, "64:ff9b::" + ipv4} {
			if got := rangeStart(processValues(t, e, form)); reflect.DeepEqual(got, start) {
				t.Errorf("%s was looked up as %s, with IpRangeStart %v",
					form, ipv4, got)
			}
		}
	}
}
