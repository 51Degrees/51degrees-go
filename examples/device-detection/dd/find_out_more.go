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

package dd

import (
	"path/filepath"
	"runtime"
	"strings"
)

// FindOutMoreLinks gives the pages to follow up and this repository, each
// page tagged with content, which names the example.
func FindOutMoreLinks(content string) []Property {
	tag := "utm_source=code&utm_medium=example&utm_campaign=51degrees-go" +
		"&utm_content=" + content + "&utm_term=find-out-more"
	return []Property{
		{"Device Detection", "https://51degrees.com/device-detection?" +
			tag},
		{"Property Dictionary", "https://51degrees.com/developers/property-dictionary?" +
			tag},
		{"Repository", repositoryURL},
	}
}

// FindOutMore returns the section every example ends with, giving the links
// FindOutMoreLinks gives, one to a line.
func FindOutMore(content string) string {
	section := "\nFind out more\n"
	for _, link := range FindOutMoreLinks(content) {
		section += "  " + link.Name + ": " + link.Value + "\n"
	}
	return section
}

// ExampleContent gives the utm_content that names the example whose source
// file is path, being its path from the examples folder with each / as a -,
// such as examples-device-detection-dd-match_metrics-match_metrics.go.
func ExampleContent(path string) string {
	path = filepath.ToSlash(path)
	if at := strings.LastIndex(path, "examples/"); at >= 0 {
		path = path[at:]
	}
	return strings.ReplaceAll(path, "/", "-")
}

// callerContent gives the utm_content of the example that called the function
// calling this one.
func callerContent() string {
	_, file, _, _ := runtime.Caller(2)
	return ExampleContent(file)
}
