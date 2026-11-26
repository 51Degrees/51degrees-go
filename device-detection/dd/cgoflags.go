/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, Davidson House,
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

/*
* Cgo concatenate all C linker and compiler directives so create this file to
* centralize the specifications of these flags.
*
* The C math library is part of the system library on macOS, which every
* program links, so -lm is given only on the other systems.
*
* The C code builds with the warnings of -Wall -Wextra, each one an error, and
* no pragma in it turns a warning off. A warning in the amalgamated sources is
* fixed in 51degrees-cxx, and the amalgamation made again.
*
* -Werror applies to every build: CI, the tests, the examples and every program
* that imports this package. If a newer compiler adds a warning, a program can
* build with CGO_CFLAGS="-g -O2 -Wno-error=<warning>" until a release fixes it.
* Setting CGO_CFLAGS replaces the go command's default of "-g -O2", so keep
* those two. The go command puts CGO_CFLAGS before these flags, and
* -Wno-error=<warning> holds against a later -Werror, so the warning is still
* printed.
 */

/*
#cgo CFLAGS: -Wall -Wextra -Werror
#cgo !darwin LDFLAGS: -lm -latomic
*/
import "C"
