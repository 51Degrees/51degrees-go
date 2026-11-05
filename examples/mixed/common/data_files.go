/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence (EUPL)
 * v.1.2 and is subject to its terms as set out below.
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

// Package common finds the data files the mixed examples use, downloading a
// free one that is missing, and says which properties need the Enterprise
// data file.
package common

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// The free data files the examples use when DD_DATA_FILE and DATA_FILE name
// no others.
const (
	DeviceDetectionDataFile = "51Degrees-LiteV4.1.hash"
	IpIntelligenceDataFile  = "51Degrees-LiteV41.ipi"
)

// The data repository's folders, in the data submodule of cxx, which hold a
// link file for each data file its scripts download.
const (
	DeviceDetectionFolder = "device-detection"
	IpIntelligenceFolder  = "ip-intelligence"
)

// downloadSource is sent with each download an example starts, so that they
// can be counted. In GitHub Actions the scripts send their own source.
const downloadSource = "examples-51degrees-go"

// DataFile returns the path of the data file the environment variable names,
// or of the free file when it is not set. A name that is not the path of a
// file is looked for in the data repository's folder, from the working folder
// up, and downloaded when the data repository links to it. Otherwise the name
// is returned, so that the engine reports the file missing.
func DataFile(variable string, folder string, free string) string {
	name := os.Getenv(variable)
	if name == "" {
		name = free
	}
	if _, err := os.Stat(name); err == nil {
		return name
	}
	if path, found := dataRepositoryFile(folder, filepath.Base(name)); found {
		return path
	}
	return name
}

// dataRepositoryFile returns the data repository's copy of the file called
// name, running the repository's script first to download it when it is
// missing. It returns false when the folder has no link file for name.
func dataRepositoryFile(folder string, name string) (string, bool) {
	link := findAbove(filepath.Join("cxx", "data", folder, name+".link"))
	if link == "" {
		return "", false
	}
	path := strings.TrimSuffix(link, ".link")
	if _, err := os.Stat(path); err == nil {
		return path, true
	}

	// The data repository is the folder above the data folder.
	data := filepath.Dir(filepath.Dir(link))
	var args []string
	var command string
	if runtime.GOOS == "windows" {
		script := filepath.Join(data, "get-data-files.ps1")
		args = []string{"powershell", "-NoProfile", "-ExecutionPolicy",
			"Bypass", "-File", script, name}
		command = fmt.Sprintf("powershell -NoProfile -ExecutionPolicy Bypass "+
			"-File \"%s\" \"%s\"", script, name)
		if source := exampleSource(); source != "" {
			args = append(args, "-Source", source)
			command += " -Source " + source
		}
	} else {
		script := filepath.Join(data, "get-data-files.sh")
		args = []string{"sh", script}
		command = fmt.Sprintf("sh \"%s\"", script)
		if source := exampleSource(); source != "" {
			args = append(args, "-s", source)
			command += " -s " + source
		}
		args = append(args, name)
		command += fmt.Sprintf(" \"%s\"", name)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Printf("%s is missing, so it is being downloaded by running\n%s\n",
		name, command)
	if cmd.Run() == nil {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	log.Fatalf("%s could not be downloaded. Run this command to try again.\n%s\n",
		name, command)
	return "", false
}

// exampleSource returns the source to send with a download, or nothing in
// GitHub Actions, where the data repository's scripts send their own so that
// test runs are not counted as people running the examples.
func exampleSource() string {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return ""
	}
	return downloadSource
}

// findAbove returns the working folder, or the first folder above it, joined
// with rel where that names a file. It returns "" when there is none.
func findAbove(rel string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
