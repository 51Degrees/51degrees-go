package ipi_onpremise

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

const (
	// enterpriseTestDataFile is the Enterprise data file, which CI links into
	// the module folder. It has many more properties than the ASN file, so a
	// reload from one to the other changes the required properties.
	enterpriseTestDataFile = "51Degrees-EnterpriseIpiV41.ipi"
	// propertiesTestShownDifferences is how many differences a failure lists.
	propertiesTestShownDifferences = 20
	// duringReloadsLookups is how many goroutines look up while reloads run.
	duringReloadsLookups = 8
	// duringReloadsCount is how many times the data file is reloaded.
	duringReloadsCount = 20
)

var propertiesTestAddresses = []string{
	"185.28.167.77",
	"8.8.8.8",
	"81.2.69.142",
	"2001:4860:4860::8888",
}

// testDataPath returns the absolute path of a data file in the module folder.
func testDataPath(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(filepath.Join("..", name))
	if err == nil {
		path, err = filepath.Abs(path)
	}
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal(err)
		}
		t.Skipf("put %s in the module folder to run this test: %v", name, err)
	}
	return path
}

// pointAt makes path give the content of target, replacing what was there
// with a rename, as a pull does. It links rather than copies, as the
// Enterprise file is large, and copies only where links cannot be made.
func pointAt(t *testing.T, path, target string) {
	t.Helper()
	next := path + ".next"
	if err := os.Symlink(target, next); err != nil {
		copyFile(t, target, next)
	}
	if err := os.Rename(next, path); err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	in, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

// propertiesTestEngine starts an engine that reloads only when the test asks.
func propertiesTestEngine(
	t *testing.T,
	path string,
	properties []string,
	temp bool) *Engine {
	t.Helper()
	opts := []EngineOptions{
		WithDataFile(path),
		WithAutoUpdate(false),
		WithFileWatch(false),
		WithTempDataCopy(temp),
	}
	if temp {
		opts = append(opts, WithTempDataDir(t.TempDir()))
	}
	if properties != nil {
		opts = append(opts, WithProperties(properties))
	}
	e, err := New(opts...)
	if err != nil {
		t.Fatalf("New on %s: %v", filepath.Base(path), err)
	}
	t.Cleanup(func() { stopWithin(e) })
	return e
}

// requiredNames returns the names of the properties a file has.
func requiredNames(t *testing.T, path string) []string {
	t.Helper()
	e, err := New(
		WithDataFile(path),
		WithAutoUpdate(false),
		WithFileWatch(false),
		WithTempDataCopy(false))
	if err != nil {
		t.Fatalf("New on %s: %v", filepath.Base(path), err)
	}
	defer e.Stop()
	return ipi_interop.GetAvailablePropertyNames(e.manager)
}

// namedProperties returns a property only the Enterprise file has, then two
// that both files have, so a reload between them adds or drops the first.
func namedProperties(t *testing.T, asn, enterprise string) []string {
	t.Helper()
	inAsn := make(map[string]bool)
	for _, name := range requiredNames(t, asn) {
		inAsn[name] = true
	}
	only := ""
	var shared []string
	for _, name := range requiredNames(t, enterprise) {
		if !inAsn[name] && only == "" {
			only = name
		} else if inAsn[name] && len(shared) < 2 {
			shared = append(shared, name)
		}
	}
	if only == "" || len(shared) < 2 {
		t.Fatalf("%s needs a property the ASN file lacks and two it has",
			enterpriseTestDataFile)
	}
	return append([]string{only}, shared...)
}

// lookupAt returns the values found for an address by property name. A failed
// lookup is kept under "(error)", so it counts as a difference.
func lookupAt(e *Engine, address string) map[string]string {
	values, err := e.Process(address)
	if err != nil {
		return map[string]string{"(error)": err.Error()}
	}
	formatted := make(map[string]string, len(values))
	for name, list := range values {
		parts := make([]string, 0, len(list))
		for _, v := range list {
			parts = append(parts, fmt.Sprintf("%v@%.4f", v.Value, v.Weight))
		}
		formatted[name] = strings.Join(parts, "|")
	}
	return formatted
}

func lookups(e *Engine) map[string]map[string]string {
	all := make(map[string]map[string]string)
	for _, address := range propertiesTestAddresses {
		all[address] = lookupAt(e, address)
	}
	return all
}

func shorten(value string) string {
	if len(value) > 60 {
		return value[:57] + "..."
	}
	return value
}

// sameAnswers fails the test for each property whose values differ between
// the reloaded engine and a fresh one.
func sameAnswers(t *testing.T, reloaded, fresh map[string]map[string]string) {
	t.Helper()
	differences := 0
	for _, address := range propertiesTestAddresses {
		names := make(map[string]bool)
		for name := range reloaded[address] {
			names[name] = true
		}
		for name := range fresh[address] {
			names[name] = true
		}
		sorted := make([]string, 0, len(names))
		for name := range names {
			sorted = append(sorted, name)
		}
		sort.Strings(sorted)
		for _, name := range sorted {
			got, inReloaded := reloaded[address][name]
			want, inFresh := fresh[address][name]
			if got == want && inReloaded == inFresh {
				continue
			}
			differences++
			if differences <= propertiesTestShownDifferences {
				t.Errorf("%s %s: after the reload %q, fresh engine %q",
					address, name, shorten(got), shorten(want))
			}
		}
	}
	if differences > propertiesTestShownDifferences {
		t.Errorf("%d differences in all", differences)
	}
}

// checkReload loads the first file, reloads the second through the path the
// temporary copy setting chooses, and checks the engine afterwards. It answers
// as a fresh engine on the second file does for every property the second
// file has. Its names stay those it was created with, being the names given,
// or with none given, the properties of the first file, so each index names
// the same property as before the reload, and a property the second file
// lacks gives no value.
// See https://github.com/51Degrees/specifications/blob/cd74d0cdb3ebe208a59369ca79b40288dd54e278/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md#finding-properties
func checkReload(
	t *testing.T,
	first string,
	second string,
	properties []string,
	temp bool) {
	t.Helper()
	master := filepath.Join(t.TempDir(), "data.ipi")
	pointAt(t, master, first)
	e := propertiesTestEngine(t, master, properties, temp)
	before := make([]string, 0)
	for i := 0; e.GetPropertyNameByIndex(i) != ""; i++ {
		before = append(before, e.GetPropertyNameByIndex(i))
	}
	for address, values := range lookups(e) {
		if message, failed := values["(error)"]; failed {
			t.Errorf("%s failed before the reload: %s", address, message)
		}
	}
	pointAt(t, master, second)
	if err := e.processFileExternallyChanged(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	fresh := propertiesTestEngine(t, second, properties, false)
	reloaded, answers := lookups(e), lookups(fresh)
	inSecond := make(map[string]bool)
	for _, name := range ipi_interop.GetAvailablePropertyNames(fresh.manager) {
		inSecond[name] = true
	}
	for _, address := range propertiesTestAddresses {
		for name, got := range reloaded[address] {
			if !inSecond[name] && name != "(error)" {
				if got != "" {
					t.Errorf("%s %s, which the second file lacks, after the reload %q, want no value",
						address, name, shorten(got))
				}
				delete(reloaded[address], name)
			}
		}
		for name := range answers[address] {
			if name != "(error)" && (!inSecond[name] || !contains(before, name)) {
				delete(answers[address], name)
			}
		}
	}
	sameAnswers(t, reloaded, answers)
	// Each index names what it named before the reload, and one past the last
	// has no name.
	for i := 0; i <= len(before); i++ {
		want := ""
		if i < len(before) {
			want = before[i]
		}
		if got := e.GetPropertyNameByIndex(i); got != want {
			t.Errorf("GetPropertyNameByIndex(%d) after the reload %q, before it %q",
				i, got, want)
		}
	}
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func TestReloadToDifferentProperties(t *testing.T) {
	asn := testDataPath(t, reloadTestDataFile)
	enterprise := testDataPath(t, enterpriseTestDataFile)
	named := namedProperties(t, asn, enterprise)
	cases := []struct {
		name          string
		first, second string
		properties    []string
		temp          bool
	}{
		{"every property, temporary copy, ASN then Enterprise",
			asn, enterprise, nil, true},
		{"every property, original file, Enterprise then ASN",
			enterprise, asn, nil, false},
		{"named properties, temporary copy, ASN then Enterprise",
			asn, enterprise, named, true},
		{"named properties, original file, Enterprise then ASN",
			enterprise, asn, named, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !c.temp && runtime.GOOS == "windows" {
				t.Skip("Windows cannot replace a file the data set has open")
			}
			checkReload(t, c.first, c.second, c.properties, c.temp)
		})
	}
}

// TestLookupsDuringReloads looks up while reloads replace the data set again
// and again, so a data set can be freed and another loaded at its address,
// and checks that every lookup names its values from its own data set.
func TestLookupsDuringReloads(t *testing.T) {
	asn := testDataPath(t, reloadTestDataFile)
	master := filepath.Join(t.TempDir(), "data.ipi")
	pointAt(t, master, asn)
	e := propertiesTestEngine(t, master, nil, true)
	want := lookupAt(e, reloadTestAddress)

	done := make(chan struct{})
	failures := make(chan string, duringReloadsLookups)
	var wg sync.WaitGroup
	wg.Add(duringReloadsLookups)
	for i := 0; i < duringReloadsLookups; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				if got := lookupAt(e, reloadTestAddress); !sameValues(got, want) {
					failures <- fmt.Sprintf("during the reloads %v, before them %v",
						got, want)
					return
				}
			}
		}()
	}
	for i := 0; i < duringReloadsCount; i++ {
		pointAt(t, master, asn)
		if err := e.processFileExternallyChanged(); err != nil {
			t.Errorf("reload %d: %v", i, err)
			break
		}
	}
	close(done)
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}

func sameValues(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for name, value := range a {
		if other, ok := b[name]; !ok || other != value {
			return false
		}
	}
	return true
}

// TestReloadKeepsTheNameTable checks the table of property names is made once,
// when the engine is built, and that a reload keeps that same table. Each
// index then names the property it named before the reload, and the reloaded
// data set reads that property at that index. The "same file" cases need only
// the ASN file, and the others need the Enterprise file as well, whose
// properties and their order differ from the ASN file's.
// See https://github.com/51Degrees/specifications/blob/d72eb074feb014a22e67d19802e1158a27658664/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md#finding-properties
func TestReloadKeepsTheNameTable(t *testing.T) {
	asn := testDataPath(t, reloadTestDataFile)
	// Two of the ASN file's names in the reverse of the data set's order, so
	// an index is a place among the names given and not among the data set's.
	asnNames := requiredNames(t, asn)
	if len(asnNames) < 2 {
		t.Fatalf("%s needs two properties", reloadTestDataFile)
	}
	reversed := []string{asnNames[1], asnNames[0]}
	enterprise := func(t *testing.T) string {
		return testDataPath(t, enterpriseTestDataFile)
	}
	cases := []struct {
		name          string
		first, second func(t *testing.T) string
		properties    func(t *testing.T) []string
	}{
		{"same file, every property",
			func(*testing.T) string { return asn },
			func(*testing.T) string { return asn },
			func(*testing.T) []string { return nil }},
		{"same file, named properties",
			func(*testing.T) string { return asn },
			func(*testing.T) string { return asn },
			func(*testing.T) []string { return reversed }},
		{"ASN then Enterprise, every property",
			func(*testing.T) string { return asn },
			enterprise,
			func(*testing.T) []string { return nil }},
		{"Enterprise then ASN, named properties",
			enterprise,
			func(*testing.T) string { return asn },
			func(t *testing.T) []string {
				return namedProperties(t, asn, enterprise(t))
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			first, second := c.first(t), c.second(t)
			master := filepath.Join(t.TempDir(), "data.ipi")
			pointAt(t, master, first)
			e := propertiesTestEngine(t, master, c.properties(t), true)
			table := e.names.Load()
			if table == nil {
				t.Fatal("the engine was built with no table of names")
			}
			var before []string
			for i := 0; e.GetPropertyNameByIndex(i) != ""; i++ {
				before = append(before, e.GetPropertyNameByIndex(i))
			}
			if len(before) == 0 {
				t.Fatal("the table names no property")
			}

			pointAt(t, master, second)
			if err := e.processFileExternallyChanged(); err != nil {
				t.Fatalf("reload: %v", err)
			}

			if e.names.Load() != table {
				t.Error("the reload replaced the table of names made " +
					"when the engine was built")
			}
			for i, want := range before {
				if got := e.GetPropertyNameByIndex(i); got != want {
					t.Errorf("index %d names %q after the reload, %q before it",
						i, got, want)
				}
			}
			checkIndexesRead(t, e, before)
		})
	}
}

// checkIndexesRead fails the test for each index at which the engine's data
// set reads a property other than the one names gives for that index.
func checkIndexesRead(t *testing.T, e *Engine, names []string) {
	t.Helper()
	results := ipi_interop.NewResultsIpi(e.manager)
	defer results.Free()
	for _, address := range propertiesTestAddresses {
		if err := results.ResultsIpiFromIpAddress(address); err != nil {
			t.Fatalf("%s: %v", address, err)
		}
		if !results.HasValues() {
			continue
		}
		for i, want := range names {
			values, err := results.GetWeightedValuesByIndexes([]int{i}, nil)
			if err != nil {
				t.Fatalf("%s index %d: %v", address, i, err)
			}
			for got := range values {
				if got != want {
					t.Errorf("%s index %d reads %q, the table names %q",
						address, i, got, want)
				}
			}
		}
	}
}
