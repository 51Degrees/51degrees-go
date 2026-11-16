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

package derivedproperty

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The script format lets a condition name a property "produced by another
// script in the same element", and gives no order the two must be added in.
// See https://github.com/51Degrees/derived-properties/blob/cc30638632050795d9acc80d5118a290ffaba085/docs/format-1.md?plain=1#L274-L276

// A script that reads another script's property gets its value when it was
// added before that script.
func TestDerivedFromDerivedAddedInEitherOrder(t *testing.T) {
	e, err := NewBuilder().AddScript("Kind", kindYAML).AddScript("Mobile", mobileYAML).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := processWith(t, e, map[string]map[string]any{"device": {"IsMobile": "True"}}, "Kind")
	if !got.HasValue() || got.AsInterface() != "Phone" {
		t.Errorf("Kind is %v, %q", got.AsInterface(), got.NoValueMessage())
	}
	// The metadata keeps the order the scripts were added in.
	scripts := e.Scripts()
	if len(scripts) != 2 || scripts[0].Name != "Kind" || scripts[1].Name != "Mobile" {
		t.Errorf("the scripts are listed as %+v, want Kind then Mobile", scripts)
	}
	properties := e.Properties()
	if len(properties) != 2 || properties[0].Name != "Kind" || properties[1].Name != "Mobile" {
		t.Errorf("the properties are listed as %+v, want Kind then Mobile", properties)
	}
}

// Files matched by a wildcard are added by name, so Kind.yaml comes before
// the Mobile.yaml it reads, and Kind still gets its value.
func TestDerivedFromDerivedInFilesMatchedByAWildcard(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{"Kind.yaml": kindYAML, "Mobile.yaml": mobileYAML} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e, err := NewBuilder().AddScriptFile(filepath.Join(dir, "*.yaml")).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := processWith(t, e, map[string]map[string]any{"device": {"IsMobile": "True"}}, "Kind")
	if !got.HasValue() || got.AsInterface() != "Phone" {
		t.Errorf("Kind is %v, %q", got.AsInterface(), got.NoValueMessage())
	}
}

// A chain of three scripts added in reverse runs from the first link.
func TestAChainOfDerivedPropertiesAddedInReverse(t *testing.T) {
	label := strings.NewReplacer("Name: Kind", "Name: Label", "{ Name: Kind,", "{ Name: Label,",
		"{ Property: derived.Mobile, Eq: true }", "{ Property: derived.Kind, Eq: Phone }",
		"Then: Phone", "Then: Handheld").Replace(kindYAML)
	e, err := NewBuilder().AddScript("Label", label).AddScript("Kind", kindYAML).
		AddScript("Mobile", mobileYAML).Build()
	if err != nil {
		t.Fatal(err)
	}
	got := processWith(t, e, map[string]map[string]any{"device": {"IsMobile": "True"}}, "Label")
	if !got.HasValue() || got.AsInterface() != "Handheld" {
		t.Errorf("Label is %v, %q", got.AsInterface(), got.NoValueMessage())
	}
}

// Two scripts that each read the other's property have no order to run in,
// so the element is not built and the fault names both.
func TestScriptsThatReadEachOtherAreRefused(t *testing.T) {
	script := func(name, reads string) string {
		return strings.NewReplacer("device.IsMobile", "derived."+reads, "Mobile", name).Replace(mobileYAML)
	}
	_, err := NewBuilder().AddScript("First", script("First", "Second")).
		AddScript("Second", script("Second", "First")).Build()
	var invalid *ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("Build gave %v, want a ValidationError", err)
	}
	text := invalid.Error()
	for _, want := range []string{"First", "Second", "derived."} {
		if !strings.Contains(text, want) {
			t.Errorf("the fault does not name %q: %s", want, text)
		}
	}
}
