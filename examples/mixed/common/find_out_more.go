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

package common

import (
	"path/filepath"
	"strings"
)

// FindOutMore returns the section every example ends with, giving the pages
// to follow up and this repository. Each link is tagged with content, which
// names the example.
func FindOutMore(content string) string {
	tag := "utm_source=code&utm_medium=example&utm_campaign=51degrees-go" +
		"&utm_content=" + content + "&utm_term=find-out-more"
	return "\nFind out more\n" +
		"  Device Detection: https://51degrees.com/device-detection?" +
		tag + "\n" +
		"  IP Intelligence: https://51degrees.com/ip-intelligence?" +
		tag + "\n" +
		"  Property Dictionary: https://51degrees.com/developers/property-dictionary?" +
		tag + "\n" +
		"  https://github.com/51Degrees/51degrees-go\n"
}

// ExampleContent gives the utm_content that names the example whose source
// file is path, being its path from the examples folder with each / as a -,
// such as examples-mixed-getting_started-main.go.
func ExampleContent(path string) string {
	path = filepath.ToSlash(path)
	if at := strings.LastIndex(path, "examples/"); at >= 0 {
		path = path[at:]
	}
	return strings.ReplaceAll(path, "/", "-")
}
