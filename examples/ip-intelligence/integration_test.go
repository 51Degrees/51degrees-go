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
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func test(t *testing.T, name string) {
	run(t, name, nil)
}

// run runs an example in the environment given, or in this process's when
// that is nil, prints what it printed, and returns what it printed.
func run(t *testing.T, name string, environment []string) string {
	cmd := exec.Command("go", "run", "./"+name)
	cmd.Env = environment
	output, err := cmd.CombinedOutput()
	os.Stdout.Write(output)
	if err != nil {
		t.Error(err)
	}
	return string(output)
}

// needsEnterprise matches a line naming a property as needing the
// Enterprise data file, as the examples print it.
var needsEnterprise = regexp.MustCompile(
	`(?m)\b[A-Z][A-Za-z]+: needs the Enterprise data file$`)

// contactUsLink is the start of every example's link to ask for a copy of the
// Enterprise data file, which each example ends with its own utm_content and
// utm_term.
const contactUsLink = "https://51degrees.com/contact-us?" +
	"utm_source=code&utm_medium=example&utm_campaign=51degrees-go"

// checkEnterprise checks that an example run on a free data file says which
// of the properties it asks for need the Enterprise data file, and gives the
// link to ask for a copy once.
func checkEnterprise(t *testing.T, name, output string) {
	if !needsEnterprise.MatchString(output) {
		t.Errorf("%s does not name a property as needing the Enterprise "+
			"data file", name)
	}
	if count := strings.Count(output, contactUsLink); count != 1 {
		t.Errorf("%s gives the contact link %d times, not once", name,
			count)
	}
}

func TestGettingStarted(t *testing.T) {
	test(t, "getting_started")
}

// weightedLine matches a line giving a value with its weighting, as the
// getting started example prints it.
var weightedLine = regexp.MustCompile(`(?m)^\t[A-Za-z]+: .+ \(weight [01]\.[0-9]{2}\)\r?$`)

// The getting started example runs on the free data file, shows what only
// the Enterprise file has, and shows each value with its weighting.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/required-examples.md?plain=1#L107
func TestGettingStartedFreeDataFile(t *testing.T) {
	output := run(t, "getting_started", withoutSettings(os.Environ()))
	checkEnterprise(t, "getting_started", output)
	if !weightedLine.MatchString(output) {
		t.Error("getting_started shows no value with its weighting")
	}
}

func TestMetadata(t *testing.T) {
	test(t, "metadata")
}

// evidenceKeyLine matches an Evidence key as the Metadata Console example
// lists it, being its prefix and its name.
var evidenceKeyLine = regexp.MustCompile(`(?m)^\t(query|server)\.[A-Za-z0-9-]+\r?$`)

// The Metadata Console example runs on the free data file and lists the
// data file's published date, the Evidence keys the engine reads, its
// components and each Property with its description, then shows what only
// the Enterprise file has.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/ip-intelligence-specification/required-examples.md?plain=1#L167-L173
func TestMetadataFreeDataFile(t *testing.T) {
	output := run(t, "metadata", withoutSettings(os.Environ()))
	checkEnterprise(t, "metadata", output)
	checkFindOutMore(t, "metadata", output)
	for _, want := range []string{"\tPublished: ", "\nComponent "} {
		if !strings.Contains(output, want) {
			t.Errorf("metadata prints no %q", strings.TrimSpace(want))
		}
	}
	if !evidenceKeyLine.MatchString(output) {
		t.Error("metadata lists no Evidence key")
	}
}

func TestOfflineProcessingFreeDataFile(t *testing.T) {
	testFree(t, "offline_processing")
}

func TestReloadFromFileFreeDataFile(t *testing.T) {
	testFree(t, "reload_from_file")
}

func TestPerformanceFreeDataFile(t *testing.T) {
	testFree(t, "performance")
}

func TestUpdatePollingIntervalFreeDataFile(t *testing.T) {
	testFree(t, "update_polling_interval")
}

func TestOfflineProcessing(t *testing.T) {
	test(t, "offline_processing")
}

func TestReloadFromFile(t *testing.T) {
	test(t, "reload_from_file")
}

func TestUpdatePollingInterval(t *testing.T) {
	test(t, "update_polling_interval")
}

// resourceKeyVariable is the variable the cloud example reads its Resource
// Key from.
const resourceKeyVariable = "_51DEGREES_RESOURCE_KEY"

// configureLink matches the tagged link to the page where a Resource Key is
// created, on either address of that page.
var configureLink = regexp.MustCompile(
	`https://(configure\.51degrees\.com/|51degrees\.com/configure/?)\?` +
		`utm_source=code&utm_medium=example&utm_campaign=51degrees-go&` +
		`utm_content=[a-z0-9._-]+&utm_term=[a-z0-9-]+`)

// A cloud example run as a fresh clone runs it, with no Resource Key, ends
// without an error, names the variable to set and links to where a Resource
// Key is created, so it runs in CI with no settings.
// See https://github.com/51Degrees/specifications/blob/20173fa1186b2c3c51fb1e4588cbc4c2658edfd4/device-detection-specification/required-examples.md?plain=1#L140-L150
func TestGettingStartedCloudWithNoResourceKey(t *testing.T) {
	output := run(t, "getting_started_cloud",
		withoutResourceKey(os.Environ()))
	checkFindOutMore(t, "getting_started_cloud", output)
	if !strings.Contains(output, resourceKeyVariable) {
		t.Errorf("getting_started_cloud does not name %s", resourceKeyVariable)
	}
	if !configureLink.MatchString(output) {
		t.Errorf("getting_started_cloud does not link to where a Resource " +
			"Key is created, tagged for this example")
	}
}

// withoutResourceKey returns the environment without the Resource Key.
func withoutResourceKey(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, variable := range environment {
		name := strings.SplitN(variable, "=", 2)[0]
		if !strings.EqualFold(name, resourceKeyVariable) {
			kept = append(kept, variable)
		}
	}
	return kept
}

// bespokeKeyVariable holds the Resource Key the cloud example is checked
// with, which includes every property. The tests of this example in the
// other languages use the same key.
const bespokeKeyVariable = "_51DEGREES_RESOURCE_KEY_BESPOKE"

// withResourceKey returns the environment with the Resource Key the example
// reads set to the value of the named variable, or false when that variable
// is unset or blank.
func withResourceKey(environment []string, name string) ([]string, bool) {
	key := ""
	for _, variable := range environment {
		parts := strings.SplitN(variable, "=", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], name) {
			key = strings.TrimSpace(parts[1])
		}
	}
	if key == "" {
		return nil, false
	}
	return append(withoutResourceKey(environment),
		resourceKeyVariable+"="+key), true
}

// TestWithResourceKeyGivesTheExampleTheBespokeKey checks the example is
// given the Bespoke key and no other, and that a blank key is reported.
func TestWithResourceKeyGivesTheExampleTheBespokeKey(t *testing.T) {
	environment := []string{
		"PATH=kept",
		resourceKeyVariable + "=mine",
		bespokeKeyVariable + "=bespoke",
	}
	got, ok := withResourceKey(environment, bespokeKeyVariable)
	want := []string{"PATH=kept", bespokeKeyVariable + "=bespoke",
		resourceKeyVariable + "=bespoke"}
	if !ok || strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the environment is %q and %v, want %q", got, ok, want)
	}
	blank := []string{"PATH=kept", bespokeKeyVariable + "= "}
	if got, ok := withResourceKey(blank, bespokeKeyVariable); ok {
		t.Errorf("%s is blank, but the environment is %q", bespokeKeyVariable, got)
	}
}

// knownAddress is the address the cloud example looks up whose values the
// tests require, the one the .NET tests of the same example check.
const knownAddress = "82.12.34.23"

// knownValue is a value the cloud service gives for the known address: the
// text printed for one of the properties must contain one of the values, or,
// when exact is set, be one of them, with or without its weight.
type knownValue struct {
	properties []string
	values     []string
	exact      bool
}

// registeredName is the network the known address is registered to.
var registeredName = knownValue{
	[]string{"RegisteredName"}, []string{"VMCBBUK"}, false}

// country is the country of the known address.
var country = knownValue{
	[]string{"Country", "CountryCode"}, []string{"United Kingdom", "GB"}, false}

// ipRange is the range the known address belongs to. An address is compared
// whole, so a longer address does not count.
var ipRange = knownValue{
	[]string{"IpRangeStart", "IpRangeEnd"},
	[]string{"82.12.34.0", "82.12.34.255"}, true}

// knownValues are the values the Bespoke key gives for the known address,
// which the tests of this example in the other languages require.
var knownValues = []knownValue{registeredName, country, ipRange}

// resultsFor returns what the cloud example printed for the address, from
// its heading to the next one.
func resultsFor(output, address string) string {
	heading := "Results for " + address + ":"
	start := strings.Index(output, heading)
	if start < 0 {
		return ""
	}
	results := output[start+len(heading):]
	if end := strings.Index(results, "Results for "); end >= 0 {
		results = results[:end]
	}
	return results
}

// printedValue returns the text printed for the property in the results, or
// "" when no line names it.
func printedValue(results, property string) string {
	for _, line := range strings.Split(results, "\n") {
		line = strings.TrimRight(line, "\r")
		if value, ok := strings.CutPrefix(line, "\t"+property+": "); ok {
			return value
		}
	}
	return ""
}

// in reports whether the results give one of the values for one of the
// properties.
func (k knownValue) in(results string) bool {
	for _, property := range k.properties {
		text := printedValue(results, property)
		if k.exact {
			text, _, _ = strings.Cut(text, " (")
		}
		for _, value := range k.values {
			if text == value || (!k.exact && strings.Contains(text, value)) {
				return true
			}
		}
	}
	return false
}

// checkKnownAddress checks the cloud example printed each of the values
// for the known address. A property the Resource Key does not include, or
// one with no value, fails.
func checkKnownAddress(t *testing.T, output string, want []knownValue) {
	results := resultsFor(output, knownAddress)
	if results == "" {
		t.Errorf("getting_started_cloud prints no results for %s",
			knownAddress)
		return
	}
	for _, w := range want {
		if !w.in(results) {
			t.Errorf("getting_started_cloud does not give %s as the %s of %s",
				strings.Join(w.values, " or "),
				strings.Join(w.properties, " or "), knownAddress)
		}
	}
}

// TestKnownValuesAreFoundOnlyInTheirOwnResults checks a value is found on
// its property's line of the known address's results, and not on another
// property's line or in another address's results.
func TestKnownValuesAreFoundOnlyInTheirOwnResults(t *testing.T) {
	output := "Results for 1.2.3.4:\n" +
		"\tCountryCode: GB\n" +
		"Results for " + knownAddress + ":\r\n" +
		"\tRegisteredName: VMCBBUK (1.00)\r\n" +
		"\tRegisteredCountry: GB\r\n" +
		"\tCountry: This property is not in the results, so the Resource " +
		"Key may not include it.\r\n" +
		"\tIpRangeEnd: 82.12.34.255 (1.00)\r\n" +
		"Results for 1.2.3.5:\n" +
		"\tCountryCode: GB\n"
	results := resultsFor(output, knownAddress)
	for _, k := range []knownValue{registeredName, ipRange} {
		if !k.in(results) {
			t.Errorf("%s is not found in %q", k.properties, results)
		}
	}
	if k := country; k.in(results) {
		t.Errorf("%s is found in %q", k.properties, results)
	}
	for _, longer := range []string{"82.12.34.023", "82.12.34.0x", "82.12.34.2550"} {
		text := "\tIpRangeStart: " + longer + "\n"
		if ipRange.in(text) {
			t.Errorf("%s is found in %q", ipRange.properties, text)
		}
	}
	if k := registeredName; k.in(resultsFor(output, "1.2.3.6")) {
		t.Errorf("%s is found for an address the example did not print", k.properties)
	}
}

// The cloud example is always built, and is run with the Bespoke key, as a
// customer with that key would run it. The run must give the values that key
// gives for the known address. It is skipped when the key is not set, and
// .github/workflows/cloud-ip-intelligence.yml fails when it is skipped.
func TestGettingStartedCloud(t *testing.T) {
	build := exec.Command("go", "build", "-o", os.DevNull, "./getting_started_cloud")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatal(err)
	}
	t.Run(bespokeKeyVariable, func(t *testing.T) {
		environment, ok := withResourceKey(os.Environ(), bespokeKeyVariable)
		if !ok {
			t.Skipf("%s is not set, so the cloud example is not run "+
				"with it", bespokeKeyVariable)
		}
		output := run(t, "getting_started_cloud", environment)
		if t.Failed() {
			// The example's own error, which run reported, says why.
			return
		}
		checkKnownAddress(t, output, knownValues)
	})
}

// The mixed examples are a separate Go module; their integration test lives
// in mixed/integration_test.go and runs via `go test` inside that module.

// testFree runs an example as a fresh clone would, without the settings CI
// gives the other tests, so that it uses the free data files and evidence,
// and checks it shows what only the Enterprise data file has.
func testFree(t *testing.T, name string) {
	checkEnterprise(t, name, run(t, name, withoutSettings(os.Environ())))
}

// withoutSettings returns the environment without the variables that name a
// data file, evidence or a key.
func withoutSettings(environment []string) []string {
	kept := make([]string, 0, len(environment))
	for _, variable := range environment {
		switch strings.ToUpper(strings.SplitN(variable, "=", 2)[0]) {
		case "_51DEGREES_IPI_PATH", "_51DEGREES_DD_PATH",
			"_51DEGREES_EVIDENCE_YAML", "_51DEGREES_LICENSE_KEY":
			continue
		}
		kept = append(kept, variable)
	}
	return kept
}

// TestWithoutSettingsDropsTheNames checks each of those variables is left
// out, and others are kept.
func TestWithoutSettingsDropsTheNames(t *testing.T) {
	got := withoutSettings([]string{
		"_51DEGREES_DD_PATH=a", "_51DEGREES_LICENSE_KEY=b",
		"_51DEGREES_IPI_PATH=j", "_51DEGREES_EVIDENCE_YAML=k",
		"PATH=kept",
	})
	if len(got) != 1 || got[0] != "PATH=kept" {
		t.Errorf("withoutSettings kept %q, want only PATH", got)
	}
}
