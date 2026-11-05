/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, 5 Charlotte Close,
 * Caversham, Reading, Berkshire, United Kingdom RG4 7BY.
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

package dd

/*
This example illustrates how to perform simple device detections on given
User-Agent strings.
*/

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
	"gopkg.in/yaml.v3"
)

// Constants
const LiteDataFile = "51Degrees-LiteV4.1.hash"
const EnterpriseDataFile = "Enterprise-HashV41.hash"
const UaFileCSV = "20000 User Agents.csv"
const EvidenceFileYaml = "20000 Evidence Records.yml"

// EnterpriseProperties are properties that only the Enterprise data file
// has, which the examples show beside the free ones.
var EnterpriseProperties = []string{"HardwareVendor", "HardwareName", "DeviceType"}

// NeedsEnterprise is shown in place of the value of a property that the data
// file in use does not have, because only the Enterprise data file has it.
const NeedsEnterprise = "needs the Enterprise data file"

// HasProperty reports whether the data file has the property, which the
// results must have been asked for.
func HasProperty(results *dd.ResultsHash, property string) bool {
	return results.RequiredPropertyIndexFromName(property) >= 0
}

// EnterpriseNote returns where to ask for a copy of the Enterprise data file,
// for an example to print when the data file lacks a property it shows.
func EnterpriseNote(contactUs string) string {
	return fmt.Sprintf("Some of these properties need the Enterprise data "+
		"file. Contact us for a copy at %s\n", contactUs)
}

// The data repository, the data submodule of cxx, has a link file in
// dataFolder for each file its scripts download.
var dataFolder = filepath.Join("cxx", "data", "device-detection")

// Sent with each download the examples start, so that they can be counted.
// In GitHub Actions the scripts send their own source.
const downloadSource = "examples-51degrees-go"

// Evidence where all fields are in string format
type stringEvidence struct {
	Prefix string
	Key    string
	Value  string
}

// Convert Evidence Records entries from file to struct
func ConvertEvidenceMap(values map[string]string) []stringEvidence {
	evidence := make([]stringEvidence, 0)
	for k, v := range values {
		strSplit := strings.SplitN(k, ".", 2)
		prefixStr := strSplit[0]
		keyStr := strSplit[1]
		evidence = append(
			evidence, stringEvidence{prefixStr, keyStr, v})
	}
	return evidence
}

// ExtractEvidence looks into a list of required evidence keys and extract
// them.
func ExtractEvidence(strEvidence []stringEvidence) *dd.Evidence {
	evidence := dd.NewEvidenceHash(uint32(len(strEvidence)))
	for _, e := range strEvidence {
		prefix := dd.HttpHeaderString
		if e.Prefix == "query" {
			prefix = dd.HttpEvidenceQuery
		}
		evidence.Add(prefix, e.Key, e.Value)
	}
	return evidence
}

// Type take a performance profile, run the code and get the return output
type ExampleFunc func(p dd.PerformanceProfile) string
type ExampleOptFunc func(p dd.PerformanceProfile, o Options) string

// Returns a full path to a file to be used for examples by file name. A file
// that isn't found is downloaded when the data repository links to it.
func GetFilePathByName(names []string) string {
	filePath, err := dd.GetFilePath(
		"../..",
		names,
	)
	if err == nil {
		return filePath
	}
	for _, name := range names {
		if filePath, found := getDataFile(name); found {
			return filePath
		}
	}
	log.Fatalf("Could not find any file that matches any of \"%s\".\n",
		strings.Join(names, ", "))
	return ""
}

// Returns a full path to a file to be used for examples by path to a file. A
// file that isn't found is downloaded when the data repository links to it.
func GetFilePathByPath(path string) string {
	dir, file := filepath.Split(path)
	filePath, err := dd.GetFilePath(
		dir,
		[]string{file},
	)
	if err == nil {
		return filePath
	}
	if filePath, found := getDataFile(file); found {
		return filePath
	}
	log.Fatalf("Could not find any file that matches \"%s\" at path \"%s\".\n",
		file,
		dir)
	return ""
}

// GetDataFile returns path when the file is there, and otherwise the data
// repository's copy of a file with the same name. It returns path when the
// data repository has no such file, so that the caller reports it missing.
func GetDataFile(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if filePath, found := getDataFile(filepath.Base(path)); found {
		return filePath
	}
	return path
}

// getDataFile returns the data repository's copy of the file called name,
// running the repository's script first to download it when it is missing.
// It returns false when there is no link file for name in the data folder,
// looked for from the working folder up.
func getDataFile(name string) (string, bool) {
	link := findAbove(filepath.Join(dataFolder, name+".link"))
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

// isFlagOn checks if certain flag is enabled in the test input
// args.
func isFlagOn(value string) bool {
	for _, arg := range os.Args {
		if strings.EqualFold(value, arg) {
			return true
		}
	}
	return false
}

// Count the number of User-Agents in a User-Agents file and return the number
// of user agents found.
func CountUAFromFiles(
	uaFilePath string) uint64 {
	var count uint64 = 0
	// Count the number of User Agents
	f, err := os.OpenFile(uaFilePath, os.O_RDONLY, 0444)
	if err != nil {
		log.Fatalf("ERROR: Failed to open file \"%s\".\n", uaFilePath)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Fatalf("ERROR: Failed to close file \"%s\".\n", uaFilePath)
		}
	}()

	// Count the number of UA.
	s := bufio.NewScanner(f)
	defer func() {
		if err := s.Err(); err != nil {
			log.Fatalf("ERROR: Error during scanning file\"%s\".\n", uaFilePath)
		}
	}()

	// Count the User-Agents
	for s.Scan() {
		count++
	}
	return count
}

// Count the number of Evidence Records in a Evidence Records file and return the number
// of evidence found.
func CountEvidenceFromFiles(
	evidenceFilePath string) uint64 {
	var count uint64 = 0
	// Count the number of Evidence Records
	f, err := os.OpenFile(evidenceFilePath, os.O_RDONLY, 0444)
	if err != nil {
		log.Fatalf("ERROR: Failed to open file \"%s\".\n", evidenceFilePath)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Fatalf("ERROR: Failed to close file \"%s\".\n", evidenceFilePath)
		}
	}()

	dec := yaml.NewDecoder(f)
	// Count the Evidence Records
	for {
		// Decode Evidence file by line
		var doc interface{}
		if err := dec.Decode(&doc); err == io.EOF {
			break
		} else if err != nil {
			// Make sure there is no decoder error
			log.Fatalf("ERROR: Failed during decoding file \"%s\". %v\n", evidenceFilePath, err)
		}
		count++
	}
	return count
}

// This is a wrapper function which execute a function that contains
// example code with an input performance profile or all performance
// profiles if performed under CI.
func PerformExample(perf dd.PerformanceProfile, eFunc ExampleFunc) {
	perfs := []dd.PerformanceProfile{perf}
	// If running under ci, use all performance profiles
	if isFlagOn("ci") {
		perfs = []dd.PerformanceProfile{
			dd.Default,
			dd.LowMemory,
			dd.Balanced,
			dd.BalancedTemp,
			dd.HighPerformance,
			dd.InMemory,
		}
	}

	// Execute the example function with all performance profiles
	for i, p := range perfs {
		output := eFunc(p)
		// This is to support example Output verification
		// so only print once.
		if i == 0 {
			fmt.Print(output)
		}
	}
}

// Same as PerformExample with additional support for command line options
func PerformExampleOptions(perf dd.PerformanceProfile, eFunc ExampleOptFunc) {
	// Get command line options
	options := ParseOptions()
	if options.showHelp {
		flag.Usage()
		return
	}

	perfs := []dd.PerformanceProfile{perf}
	// If running under ci, use all performance profiles
	if isFlagOn("ci") {
		perfs = []dd.PerformanceProfile{
			dd.Default,
			dd.LowMemory,
			dd.Balanced,
			dd.BalancedTemp,
			dd.HighPerformance,
			dd.InMemory,
		}
	}

	// Execute the example function with all performance profiles
	for i, p := range perfs {
		output := eFunc(p, options)
		// This is to support example Output verification
		// so only print once.
		if i == 0 {
			fmt.Print(output)
		}
	}
}

type Options struct {
	DataFilePath     string
	EvidenceFilePath string
	LogOutputPath    string
	// JSONOutputPath is where the performance example writes the results JSON
	// the nightly performance graphs read. Empty leaves it unwritten, which is
	// what an interactive run wants.
	JSONOutputPath string
	Iterations     uint64
	showHelp       bool
}

func ParseOptions() Options {
	options := Options{}

	flag.StringVar(&options.DataFilePath, "data-file", "../"+LiteDataFile, "Path to a 51Degrees Hash data file")
	flag.StringVar(&options.DataFilePath, "d", options.DataFilePath, "Alias for -data-file")

	flag.StringVar(&options.EvidenceFilePath, "evidence-file", "../"+EvidenceFileYaml, "Path to a Evidence Records YAML file")
	flag.StringVar(&options.EvidenceFilePath, "e", options.EvidenceFilePath, "Alias for -evidence-file")

	flag.StringVar(&options.LogOutputPath, "log-output", "", "Path to a output log file")
	flag.StringVar(&options.LogOutputPath, "l", options.LogOutputPath, "Alias for -log-output")

	flag.StringVar(&options.JSONOutputPath, "json-output", "", "Path to write the performance results JSON to")
	flag.StringVar(&options.JSONOutputPath, "j", options.JSONOutputPath, "Alias for -json-output")

	flag.Uint64Var(&options.Iterations, "iterations", 4, "Number of iterations")
	flag.Uint64Var(&options.Iterations, "i", options.Iterations, "Alias for -iterations")

	flag.BoolVar(&options.showHelp, "help", false, "Print help")
	flag.BoolVar(&options.showHelp, "h", options.showHelp, "Alias for -help")

	flag.Parse()
	return options
}
