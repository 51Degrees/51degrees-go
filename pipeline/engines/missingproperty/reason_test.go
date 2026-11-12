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

package missingproperty

import (
	"testing"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// These tests follow "Deciding the reason" in the missing Properties section
// of the Pipeline specification, with a test for each step.
// See https://github.com/51Degrees/specifications/blob/a0a7008a6875b1b8aae4f947acd5252466c777fe/pipeline-specification/features/properties.md#deciding-the-reason

// onPremise is an On-premise Engine as Decide sees one.
type onPremise struct {
	key   string
	tier  string
	props []core.PropertyMetadata
}

func (e *onPremise) DataKey() string                     { return e.key }
func (e *onPremise) DataSourceTier() string              { return e.tier }
func (e *onPremise) Properties() []core.PropertyMetadata { return e.props }

// lateOnPremise is an On-premise Engine that has not loaded its metadata.
type lateOnPremise struct{ onPremise }

func (e *lateOnPremise) PropertiesLoaded() bool { return false }

// cloud is a Cloud Engine as Decide sees one.
type cloud struct {
	onPremise
	loaded     bool
	listFailed bool
}

func (e *cloud) PropertiesLoaded() bool          { return e.loaded }
func (e *cloud) PropertyListRequestFailed() bool { return e.listFailed }

// engineType is this implementation's own name for the onPremise Engine.
const engineType = "missingproperty.onPremise"

// property is the metadata of a Property present in tiers.
func property(name string, available bool, tiers ...string) core.PropertyMetadata {
	return core.PropertyMetadata{
		Name: name, Available: available, DataTiersWherePresent: tiers,
	}
}

// check compares the reason and the whole message with those of want, which
// one of the builders gives. The text each builder gives is tested against
// the specification's table in missingproperty_test.go.
func check(t *testing.T, got, want *core.PropertyMissingError) {
	t.Helper()
	if got == nil {
		t.Fatal("Decide gave no error")
	}
	if got.Reason != want.Reason {
		t.Errorf("reason\n got: %s\nwant: %s", got.Reason, want.Reason)
	}
	if got.Error() != want.Error() {
		t.Errorf("message\n got: %s\nwant: %s", got.Error(), want.Error())
	}
}

// Step 1. A cloud request that failed for this Flow Data decides the reason
// before any Engine is asked, and the message names the first Engine.
func TestFailedCloudRequestDecidesFirst(t *testing.T) {
	first := &cloud{onPremise: onPremise{key: "device",
		props: []core.PropertyMetadata{property("IsMobile", true)}},
		loaded: true}
	second := &cloud{onPremise: onPremise{key: "location"}, loaded: true}
	got := Decide("ScreenWidth", []Engine{first, second},
		Context{CloudRequestFailed: true})
	check(t, got, ForCloudRequestFailed("ScreenWidth", "device"))
}

// Step 1. With no Engine to name, the message names Unknown.
func TestFailedCloudRequestWithNoEngine(t *testing.T) {
	got := Decide("ScreenWidth", nil, Context{CloudRequestFailed: true})
	check(t, got, ForCloudRequestFailed("ScreenWidth", ""))
}

// Step 2. The first Engine to give a reason other than Unknown decides.
func TestFirstEngineWithAReasonDecides(t *testing.T) {
	unknown := &onPremise{key: "first", tier: "Lite"}
	excluded := &onPremise{key: "second", tier: "Lite",
		props: []core.PropertyMetadata{property("IsMobile", false, "Lite")}}
	upgrade := &onPremise{key: "third", tier: "Lite",
		props: []core.PropertyMetadata{property("IsMobile", true, "Premium")}}
	got := Decide("IsMobile", []Engine{unknown, excluded, upgrade}, Context{})
	check(t, got, ForExcludedInConfig("IsMobile", "second"))
}

// Step 2. When every Engine gives Unknown, the message names the last.
func TestEveryEngineUnknownNamesTheLast(t *testing.T) {
	got := Decide("IsMobile", []Engine{
		&onPremise{key: "first"}, &onPremise{key: "last"}}, Context{})
	check(t, got, Generic("IsMobile", "last"))
}

// Step 3.1. An On-premise Engine whose data file tier is not among the
// Property's tiers gives DataFileUpgradeRequired, naming the tiers and the
// Engine's type. The name is matched whatever its letter case.
func TestOnPremiseTierNotAmongThePropertyTiers(t *testing.T) {
	e := &onPremise{key: "device", tier: "Lite", props: []core.PropertyMetadata{
		property("IsMobile", false, "Premium", "Enterprise")}}
	got := Decide("ISMOBILE", []Engine{e}, Context{})
	check(t, got, ForDataFileUpgrade("ISMOBILE", "device", engineType,
		[]string{"Premium", "Enterprise"}))
}

// Step 3.1. A listed Property with no tiers is not in the Engine's tier, and
// the message is the one for tiers that are not known.
func TestOnPremiseListedWithNoTiers(t *testing.T) {
	e := &onPremise{key: "device", tier: "Lite",
		props: []core.PropertyMetadata{property("IsMobile", true)}}
	got := Decide("IsMobile", []Engine{e}, Context{})
	check(t, got, ForDataFileUpgrade("IsMobile", "device", engineType, nil))
}

// Step 3.1. A listed Property in the Engine's tier that is not available
// gives PropertyExcludedFromEngineConfiguration.
func TestListedPropertyNotAvailableIsExcluded(t *testing.T) {
	e := &onPremise{key: "device", tier: "Lite", props: []core.PropertyMetadata{
		property("IsMobile", false, "Lite", "Premium")}}
	got := Decide("ismobile", []Engine{e}, Context{})
	check(t, got, ForExcludedInConfig("ismobile", "device"))
}

// Step 3.1. A Cloud Engine has no data file tier, so a listed Property that
// is not available is excluded, whatever tiers its metadata names.
func TestCloudListedPropertyNotAvailableIsExcluded(t *testing.T) {
	e := &cloud{onPremise: onPremise{key: "device", tier: "cloud",
		props: []core.PropertyMetadata{property("IsMobile", false)}},
		loaded: true}
	got := Decide("IsMobile", []Engine{e}, Context{})
	check(t, got, ForExcludedInConfig("IsMobile", "device"))
}

// Step 3.2. A Cloud Engine whose loaded metadata lists no Properties gives
// ProductNotAccessibleWithResourceKey.
func TestCloudWithNoPropertiesIsProductNotAccessible(t *testing.T) {
	e := &cloud{onPremise: onPremise{key: "device", tier: "cloud"},
		loaded: true}
	got := Decide("IsMobile", []Engine{e}, Context{})
	check(t, got, ForProductNotAccessible("IsMobile", "device"))
}

// Step 3.2. A Cloud Engine whose loaded metadata lists other Properties
// gives PropertyNotAccessibleWithResourceKey, naming them.
func TestCloudWithOtherPropertiesIsPropertyNotAccessible(t *testing.T) {
	e := &cloud{onPremise: onPremise{key: "device", tier: "cloud",
		props: []core.PropertyMetadata{
			property("IsMobile", true), property("HardwareVendor", true)}},
		loaded: true}
	got := Decide("ScreenWidth", []Engine{e}, Context{})
	check(t, got, ForPropertyNotAccessible("ScreenWidth", "device",
		[]string{"IsMobile", "HardwareVendor"}))
}

// Step 3.2. A Cloud Engine that has not loaded its metadata gives
// CloudRequestFailed when its request for the property list failed.
func TestCloudNotLoadedAfterFailedListRequest(t *testing.T) {
	e := &cloud{onPremise: onPremise{key: "device", tier: "cloud"},
		listFailed: true}
	got := Decide("IsMobile", []Engine{e}, Context{})
	check(t, got, ForCloudRequestFailed("IsMobile", "device"))
}

// Step 3.2. A Cloud Engine that has not loaded its metadata, and whose
// request for the property list has not failed, gives Unknown. A typed
// accessor makes no difference to a Cloud Engine.
func TestCloudNotLoadedIsUnknown(t *testing.T) {
	e := &cloud{onPremise: onPremise{key: "device", tier: "cloud"}}
	got := Decide("IsMobile", []Engine{e},
		Context{Accessors: []string{"IsMobile"}})
	check(t, got, Generic("IsMobile", "device"))
}

// Step 3.3. An On-premise Engine whose metadata does not list the Property,
// and whose Element Data has a typed accessor of that name, gives
// DataFileUpgradeRequired with no data tiers.
func TestOnPremiseTypedAccessorWithoutMetadata(t *testing.T) {
	e := &onPremise{key: "device", tier: "Lite", props: []core.PropertyMetadata{
		property("HardwareVendor", true, "Lite")}}
	got := Decide("ismobile", []Engine{e},
		Context{Accessors: []string{"HardwareVendor", "IsMobile"}})
	check(t, got, ForDataFileUpgrade("ismobile", "device", engineType, nil))
}

// Step 3.3. The same holds for an On-premise Engine that has not loaded its
// metadata, because no loaded metadata lists the Property.
func TestOnPremiseNotLoadedTypedAccessor(t *testing.T) {
	e := &lateOnPremise{onPremise{key: "device", tier: "Lite",
		props: []core.PropertyMetadata{property("IsMobile", false, "Lite")}}}
	got := Decide("IsMobile", []Engine{e},
		Context{Accessors: []string{"IsMobile"}})
	check(t, got, ForDataFileUpgrade("IsMobile", "device", engineType, nil))
}

// Step 3.4. A Property the metadata lists as available in the Engine's tier
// gives Unknown, typed accessor or not, and so does one with neither
// metadata nor an accessor.
func TestOtherwiseUnknown(t *testing.T) {
	e := &onPremise{key: "device", tier: "Lite", props: []core.PropertyMetadata{
		property("IsMobile", true, "Lite")}}
	got := Decide("IsMobile", []Engine{e},
		Context{Accessors: []string{"IsMobile"}})
	check(t, got, Generic("IsMobile", "device"))

	got = Decide("ScreenWidth", []Engine{e},
		Context{Accessors: []string{"IsMobile"}})
	check(t, got, Generic("ScreenWidth", "device"))
}
