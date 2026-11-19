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

package usagesharing

import (
	"encoding/xml"
	"runtime/debug"
	"sort"
	"strings"
)

// modulePrefix starts the path of every Go module 51Degrees publishes.
const modulePrefix = "github.com/51Degrees/"

// develVersion is the version Go gives a module built from its folder rather
// than from a release.
const develVersion = "(devel)"

// packageVersion is a 51Degrees module in use and the version of it in use.
type packageVersion struct {
	path    string
	version string
}

// linkedPackages gives the 51Degrees modules linked into the program, from
// its build information.
func linkedPackages() []packageVersion {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	return packagesIn(info)
}

// packagesIn gives the 51Degrees modules among those a program was built
// with, each once and in path order, with the version of the one in use.
// Go's build information lists the modules a program was built with but not
// which requires which, so every 51Degrees module built in is named. The
// program's own module is left out, as it is not a package the program uses.
// See https://github.com/51Degrees/specifications/blob/e36d54620caa31745059208b9c09d0ae91e86f05/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L199-L207
func packagesIn(info *debug.BuildInfo) []packageVersion {
	seen := make(map[string]bool)
	var out []packageVersion
	for _, d := range info.Deps {
		if d == nil || !strings.HasPrefix(d.Path, modulePrefix) || seen[d.Path] {
			continue
		}
		seen[d.Path] = true
		out = append(out, packageVersion{path: d.Path, version: inUse(d)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

// inUse gives the version of a module that the program was built with. A
// module replaced by another release is that release, and one replaced by a
// folder, or built from its folder in a workspace, is (devel).
func inUse(d *debug.Module) string {
	version := d.Version
	if d.Replace != nil {
		version = d.Replace.Version
	}
	if version == "" {
		return develVersion
	}
	return version
}

// writePackage writes a module in use as a Package element, named by its
// module path, holding its version.
func writePackage(b *strings.Builder, p packageVersion) {
	b.WriteString(`<Package Name="`)
	xml.EscapeText(b, []byte(p.path))
	b.WriteString(`">`)
	xml.EscapeText(b, []byte(p.version))
	b.WriteString("</Package>")
}
