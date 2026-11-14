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

package derivedproperty

import (
	"embed"
	"strings"
)

// builtInFiles is a copy of the scripts folder of the derived-properties
// submodule, because a Go module is built from git's archive of a tag, which
// leaves a submodule empty. ci/check-derived-properties.ps1 fails when the
// copy differs from the submodule.
//
//go:embed scripts
var builtInFiles embed.FS

// BuiltInScript names a script shipped in this package.
type BuiltInScript string

const (
	// HumanConfidence is the confidence that the request came from a device
	// being used by a human who is viewing the page, being High, Medium or
	// Low. It reads device.IsCrawler, device.BrowserReleaseYear,
	// device.BrowserReleaseAge and ip.HumanProbability.
	HumanConfidence BuiltInScript = "HumanConfidence"
	// IsGdpr is whether the General Data Protection Regulation, or the United
	// Kingdom's version of it, is likely to apply to the person making the
	// request, judged from ip.CountryCode.
	IsGdpr BuiltInScript = "IsGdpr"
)

// BuiltInScripts returns every script shipped in this package.
func BuiltInScripts() []BuiltInScript {
	return []BuiltInScript{HumanConfidence, IsGdpr}
}

// scriptExtensions are the file extensions a script is read from.
var scriptExtensions = []string{".yaml", ".yml", ".json"}

// text returns the script's text, and false for a name no script has.
func (s BuiltInScript) text() (string, bool) {
	for _, ext := range scriptExtensions {
		if data, err := builtInFiles.ReadFile("scripts/" + string(s) + ext); err == nil {
			return string(data), true
		}
	}
	return "", false
}

// builtInNames lists the built-in scripts for a message.
func builtInNames() string {
	names := make([]string, 0, len(BuiltInScripts()))
	for _, s := range BuiltInScripts() {
		names = append(names, string(s))
	}
	return strings.Join(names, ", ")
}
