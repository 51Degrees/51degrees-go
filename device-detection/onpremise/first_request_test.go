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

package onpremise

import (
	"net/url"
	"testing"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
)

// firstRequestQuery is the query of the first request of an update that an
// engine with the default data update URL, the made-up license key and the
// product given makes.
func firstRequestQuery(t *testing.T, product string) url.Values {
	t.Helper()
	engine := &Engine{
		FileUpdater: common_go.NewFileUpdater(defaultDataFileUrl),
		licenseKey:  productTestKey,
		product:     product,
	}
	if err := engine.validateAndAppendUrlParams(); err != nil {
		t.Fatalf("the data update URL could not be completed: %v", err)
	}
	parsed, err := url.Parse(engine.GetDataFileUrl())
	if err != nil {
		t.Fatalf("the data update URL could not be parsed: %v", err)
	}
	return parsed.Query()
}

// The first request of an update names no product unless the product option
// is set, so the Distributor answers with the product the license key holds.
func TestFirstRequestNamesNoProductByDefault(t *testing.T) {
	query := firstRequestQuery(t, "")

	if query.Has("Product") {
		t.Errorf("the first request has the query %q, want no Product parameter",
			query.Encode())
	}
	if got := query.Get("LicenseKeys"); got != productTestKey {
		t.Errorf("LicenseKeys = %q, want %q", got, productTestKey)
	}
}

// The first request of an update names the product the product option gives.
func TestFirstRequestNamesTheProductSet(t *testing.T) {
	query := firstRequestQuery(t, "V4TAC")

	if got := query["Product"]; len(got) != 1 || got[0] != "V4TAC" {
		t.Errorf("Product = %q, want V4TAC once", got)
	}
	if got := query.Get("LicenseKeys"); got != productTestKey {
		t.Errorf("LicenseKeys = %q, want %q", got, productTestKey)
	}
}
