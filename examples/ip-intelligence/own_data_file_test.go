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

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// dataFileVariable names the data file the examples use, and
// deprecatedDataFile its deprecated name, written in two parts so that this
// file does not name an old variable.
const (
	dataFileVariable   = "_51DEGREES_IPI_PATH"
	deprecatedDataFile = "DATA" + "_FILE"
)

// onOwnDataFile runs tests on the data file the settings name.
func onOwnDataFile(tests func() int) int {
	return tests()
}

// ownDataFile returns environment with dataFileVariable naming a hard link
// to the data file it names, or a copy where a link cannot be made, in a new
// folder beside that file, and without the deprecated name, with a function
// that removes the folder. Environment is returned as it is when it names no
// data file.
func ownDataFile(environment []string) ([]string, func(), error) {
	var named, old string
	kept := make([]string, 0, len(environment)+1)
	for _, variable := range environment {
		name, value, _ := strings.Cut(variable, "=")
		switch strings.ToUpper(name) {
		case dataFileVariable:
			named = value
			continue
		case deprecatedDataFile:
			old = value
			continue
		}
		kept = append(kept, variable)
	}
	source := named
	if source == "" {
		source = old
	}
	if source == "" {
		return environment, func() {}, nil
	}
	folder, err := os.MkdirTemp(filepath.Dir(source), "own-data-file-")
	if err != nil {
		return nil, nil, err
	}
	remove := func() { os.RemoveAll(folder) }
	own := filepath.Join(folder, filepath.Base(source))
	if err := os.Link(source, own); err != nil {
		if err := copyFile(source, own); err != nil {
			remove()
			return nil, nil, fmt.Errorf("the tests need a data file of "+
				"their own, made from %s: %w", source, err)
		}
	}
	return append(kept, dataFileVariable+"="+own), remove, nil
}

// copyFile copies the file at from to a new file at to.
func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
