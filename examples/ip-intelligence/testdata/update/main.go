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

// The stand-in for the update example in the tests. It is given its data file
// as the examples are, and replaces it at once as an update does, writing a
// newer file next to it and renaming that over it, with no download.
package main

import (
	"os"
	"path/filepath"

	"github.com/51Degrees/51degrees-go/examples/ip-intelligence/common"
)

// downloaded is what the stand-in writes in place of the data file.
const downloaded = "a newer data file"

func main() {
	common.RunExample(func(params *common.ExampleParams) error {
		download, err := os.CreateTemp(filepath.Dir(params.DataFile), "download-")
		if err != nil {
			return err
		}
		if _, err := download.WriteString(downloaded); err != nil {
			download.Close()
			return err
		}
		if err := download.Close(); err != nil {
			return err
		}
		return os.Rename(download.Name(), params.DataFile)
	})
}
