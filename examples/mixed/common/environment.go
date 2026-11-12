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
	"fmt"
	"os"
	"sync"
)

// oldNames holds, for each environment variable the examples read, the
// deprecated names the value is also read from, in the order they are read.
var oldNames = map[string][]string{
	"_51DEGREES_DD_PATH":  {"DD_DATA_FILE"},
	"_51DEGREES_IPI_PATH": {"DATA_FILE"},
}

var (
	reportedLock sync.Mutex
	// reported holds the deprecated names already reported, so that each is
	// reported once.
	reported = map[string]bool{}
)

// fromEnvironment returns the value of the environment variable name. When
// that is unset or empty it returns the value of the first of the variable's
// deprecated names that is set, and reports that name as deprecated.
func fromEnvironment(name string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	for _, old := range oldNames[name] {
		if value := os.Getenv(old); value != "" {
			reportOldName(old, name)
			return value
		}
	}
	return ""
}

// reportOldName prints to the standard error stream, once in the life of the
// process for each old name, that the variable old is deprecated and that the
// variable name replaces it.
func reportOldName(old, name string) {
	reportedLock.Lock()
	defer reportedLock.Unlock()
	if reported[old] {
		return
	}
	reported[old] = true
	fmt.Fprintf(os.Stderr, "The environment variable %s is deprecated. %s "+
		"replaces it, so set that variable instead.\n", old, name)
}
