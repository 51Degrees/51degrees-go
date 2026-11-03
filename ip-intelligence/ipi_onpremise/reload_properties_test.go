package ipi_onpremise

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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
)

var propertiesTestAddresses = []string{
	"185.28.167.77",
	"8.8.8.8",
	"81.2.69.142",
	"2001:4860:4860::8888",
}

// propertiesTestFiles returns the absolute paths of the ASN and Enterprise
// data files.
func propertiesTestFiles(t *testing.T) (string, string) {
	t.Helper()
	paths := make([]string, 0, 2)
	for _, name := range []string{reloadTestDataFile, enterpriseTestDataFile} {
		path, err := filepath.EvalSymlinks(filepath.Join("..", name))
		if err == nil {
			path, err = filepath.Abs(path)
		}
		if err != nil {
			if os.Getenv("CI") != "" {
				t.Fatal(err)
			}
			t.Skipf("put %s and %s in the module folder to run this test: %v",
				reloadTestDataFile, enterpriseTestDataFile, err)
		}
		paths = append(paths, path)
	}
	return paths[0], paths[1]
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

// lookups returns the values found for each address by property name. A
// failed lookup is kept under "(error)", so it counts as a difference.
func lookups(t *testing.T, e *Engine) map[string]map[string]string {
	t.Helper()
	all := make(map[string]map[string]string)
	for _, address := range propertiesTestAddresses {
		values, err := e.Process(address)
		if err != nil {
			all[address] = map[string]string{"(error)": err.Error()}
			continue
		}
		formatted := make(map[string]string, len(values))
		for name, list := range values {
			parts := make([]string, 0, len(list))
			for _, v := range list {
				parts = append(parts, fmt.Sprintf("%v@%.4f", v.Value, v.Weight))
			}
			formatted[name] = strings.Join(parts, "|")
		}
		all[address] = formatted
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
// temporary copy setting chooses, and checks that the engine then answers as
// a fresh engine on the second file does.
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
	for address, values := range lookups(t, e) {
		if message, failed := values["(error)"]; failed {
			t.Errorf("%s failed before the reload: %s", address, message)
		}
	}
	pointAt(t, master, second)
	if err := e.processFileExternallyChanged(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	fresh := propertiesTestEngine(t, second, properties, false)
	sameAnswers(t, lookups(t, e), lookups(t, fresh))
}

func TestReloadToDifferentProperties(t *testing.T) {
	asn, enterprise := propertiesTestFiles(t)
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
