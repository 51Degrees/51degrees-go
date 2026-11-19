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

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The response gives the links to find out more, naming this repository.
func TestHandlerGivesFindOutMore(t *testing.T) {
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}
	r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_2 "+
		"like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) "+
		"Version/17.1.2 Mobile/15E148 Safari/604.1")
	r.Header.Set("X-Forwarded-For", "185.28.167.77")

	rr := httptest.NewRecorder()
	testHandler.ServeHTTP(rr, r)

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &fields); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}
	if !strings.Contains(string(fields["findOutMore"]),
		"https://github.com/51Degrees/51degrees-go") {
		t.Errorf("Expected findOutMore to name the repository, got %s",
			fields["findOutMore"])
	}
}
