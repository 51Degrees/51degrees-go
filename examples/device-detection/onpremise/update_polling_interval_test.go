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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	dd_example "github.com/51Degrees/51degrees-go/examples/device-detection/dd"
)

// standInLicenseKey is the made-up license key the update example is given.
// The stand-in for the Distributor answers whatever key it is sent.
const standInLicenseKey = "STANDINLICENSEKEY"

// otherProduct is a product the stand-in lists beside the data file's own,
// as the Distributor does for a license key that holds a Device Detection
// and an IP Intelligence product.
const otherProduct = "IPIV4Enterprise"

// The update example, with no product named, asks the stand-in for the
// Distributor for none first and gets the list of products. It then asks
// for the product of the data file it started from, and gets the newer data
// file.
func TestUpdatePollingInterval(t *testing.T) {
	product := dataFileProduct(t, updateDataFile())
	requests := runUpdateExample(t, product, "")
	if len(requests) < 2 ||
		requests[0] != (dataUpdateRequest{"", 300}) ||
		requests[1] != (dataUpdateRequest{product, 200}) {
		t.Errorf("the stand-in received %v, want first a request that "+
			"names no product and gets 300, then one for %s that gets the "+
			"newer data file", requests, product)
	}
}

// The update example, with a product given as an argument, asks the
// stand-in for that product in its first request and in every other, and
// gets the newer data file.
func TestUpdatePollingIntervalProductArgument(t *testing.T) {
	product := dataFileProduct(t, updateDataFile())
	requests := runUpdateExample(t, product, product)
	if len(requests) < 1 || requests[0] != (dataUpdateRequest{product, 200}) {
		t.Errorf("the stand-in received %v, want first a request for %s "+
			"that gets the newer data file", requests, product)
	}
	for _, request := range requests {
		if request.product == "" {
			t.Errorf("the stand-in received %v, want every request to "+
				"name %s", requests, product)
			break
		}
	}
}

// updateDataFile is the data file the update example starts from, being the
// one the settings name, which CI fetches, or the free data file.
func updateDataFile() string {
	if named := namedDataFile(os.Environ()); named != "" {
		return named
	}
	return dd_example.GetDataFile(dd_example.LiteDataFile)
}

// productPrefix starts the name the Distributor gives each product of a
// Device Detection Hash data file.
const productPrefix = "V4"

// dataFileProduct gives the name the Distributor gives the product of the
// data file at path, such as V4TAC for TAC-HashV41.hash, which is the
// prefix followed by the tier the data file names.
func dataFileProduct(t *testing.T, path string) string {
	t.Helper()
	manager := dd.NewResourceManager()
	defer manager.Free()
	config := dd.NewConfigHash(dd.LowMemory)
	if err := dd.InitManagerFromFile(manager, *config, "", path); err != nil {
		t.Fatalf("%s cannot be loaded: %v", path, err)
	}
	tier := dd.GetDataSetName(manager)
	if tier == "" {
		t.Fatalf("%s names no tier", path)
	}
	if strings.HasPrefix(strings.ToUpper(tier), productPrefix) {
		return tier
	}
	return productPrefix + tier
}

// runUpdateExample runs the update example on a copy of the data file, with
// its updates pointed at a stand-in for the Distributor that lists
// otherProduct and product, and returns the requests the stand-in received.
// The stand-in serves another copy whose published date is a day later, so
// the update has completed when the example's data file holds that date.
// The example is given productArgument as its product unless that is "".
func runUpdateExample(t *testing.T, product, productArgument string) []dataUpdateRequest {
	source := updateDataFile()
	folder := t.TempDir()
	own := filepath.Join(folder, filepath.Base(source))
	served := filepath.Join(folder, "served"+filepath.Ext(source))
	for _, path := range []string{own, served} {
		if err := copyFile(source, path); err != nil {
			t.Fatal(err)
		}
	}
	published := readDate(t, own, publishedOffset)
	// The engine polls only once the date the data file expects the next
	// one has passed, so the example's copy expects it on its published
	// date, and the example polls within the time it waits.
	writeDate(t, own, nextUpdateOffset, published)
	newer := published.AddDate(0, 0, 1)
	writeDate(t, served, publishedOffset, newer)

	standIn := newDataUpdateStandIn(t, []string{otherProduct, product}, served)
	environment := append(withoutSettings(os.Environ()),
		dataFileVariable+"="+own,
		"_51DEGREES_LICENSE_KEY="+standInLicenseKey)
	arguments := []string{"-url", standIn.url()}
	if productArgument != "" {
		arguments = append(arguments, "-product", productArgument)
	}
	run(t, "update_polling_interval", environment, arguments...)

	if got := readDate(t, own, publishedOffset); !got.Equal(newer) {
		t.Errorf("the example's data file was published on %s, want %s, "+
			"the date of the newer data file the stand-in served",
			got.Format("2006-01-02"), newer.Format("2006-01-02"))
	}
	return standIn.received()
}
