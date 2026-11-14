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

import "runtime/debug"

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
// with.
func packagesIn(info *debug.BuildInfo) []packageVersion {
	return nil
}
