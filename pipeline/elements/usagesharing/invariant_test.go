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

package usagesharing

// These tests follow the process identifier and the invariant values of the
// usage sharing record.
// See https://github.com/51Degrees/specifications/blob/07e1fcedc2/pipeline-specification/pipeline-elements/usage-sharing-element.md?plain=1#L108-L166

import (
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
)

// invariantTags are the opening tags of the invariant values, as the
// specification lists them at lines 138 to 145: Version, Product, Flow
// Elements, Packages, Language, LanguageVersion, ServerIP and Platform.
var invariantTags = []string{
	"<Version>", "<Product>", "<FlowElement>", "<Package ", "<Language>",
	"<LanguageVersion>", "<ServerIP>", "<Platform>",
}

// processIDTag finds the process identifier of a record.
var processIDTag = regexp.MustCompile(`<ProcessId>([^<]*)</ProcessId>`)

// uuidV4 is the string format of RFC 9562 section 4 for a version 4 UUID,
// in either case, as lines 119 to 126 ask.
var uuidV4 = regexp.MustCompile(
	`^(?i)[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// invariantSharer is a Pipeline whose usage sharing element sends each
// record on its own as soon as it is queued, timed by a clock the test
// moves, so the hour passes without waiting for it.
type invariantSharer struct {
	p     core.Pipeline
	c     *capture
	clock *fakeClock
	n     int
}

func newInvariantSharer(t *testing.T) *invariantSharer {
	t.Helper()
	c := &capture{}
	srv := c.server()
	t.Cleanup(srv.Close)
	clk := newFakeClock()
	b := withClock(NewBuilder().
		SetShareUsageURL(srv.URL).
		SetMinimumEntriesPerMessage(1).
		SetTakeTimeout(0).
		SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil))), clk)
	b.packages = func() []packageVersion {
		return []packageVersion{{pipelineModule, "v4.5.174"}}
	}
	p, err := core.NewPipelineBuilder().AddFlowElement(b.Build()).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return &invariantSharer{p: p, c: c, clock: clk}
}

// share processes Evidence not seen before, so it is not a repeat, waits
// for its record to be sent and returns it.
func (s *invariantSharer) share(t *testing.T) string {
	t.Helper()
	s.n++
	fd := s.p.CreateFlowData()
	fd.AddEvidence("header.user-agent", fmt.Sprintf("agent %d", s.n))
	if err := fd.Process(); err != nil {
		t.Fatalf("process: %v", err)
	}
	s.c.waitFor(t, s.n)
	records := devices(s.c)
	if len(records) != s.n {
		t.Fatalf("%d records sent, want %d", len(records), s.n)
	}
	return records[s.n-1]
}

// carriesInvariants reports which invariant values a record holds.
func carriesInvariants(record string) (present, absent []string) {
	for _, tag := range invariantTags {
		if strings.Contains(record, tag) {
			present = append(present, tag)
		} else {
			absent = append(absent, tag)
		}
	}
	return present, absent
}

func processID(t *testing.T, record string) string {
	t.Helper()
	m := processIDTag.FindStringSubmatch(record)
	if m == nil {
		t.Fatalf("the record has no ProcessId: %s", record)
	}
	return m[1]
}

// "The invariant values MUST be written to the first record the element
// writes." (line 150)
func TestTheFirstRecordCarriesTheInvariantValues(t *testing.T) {
	s := newInvariantSharer(t)
	if _, absent := carriesInvariants(s.share(t)); len(absent) > 0 {
		t.Errorf("the first record leaves out %v", absent)
	}
}

// "They MUST NOT be written to any other record." (line 152) Records within
// the hour after the first leave them out.
func TestRecordsWithinTheHourLeaveOutTheInvariantValues(t *testing.T) {
	s := newInvariantSharer(t)
	s.share(t)
	for _, after := range []time.Duration{0, time.Minute, 58 * time.Minute} {
		s.clock.advance(after)
		if present, _ := carriesInvariants(s.share(t)); len(present) > 0 {
			t.Errorf("a record written within the hour carries %v", present)
		}
	}
	// The values are left out, and the rest of the record is still written.
	last := devices(s.c)[s.n-1]
	for _, tag := range []string{"<ProcessId>", "<SessionId>", "<Sequence>",
		"<DateSent>", "<ClientIP>", `<header Name="user-agent">`} {
		if !strings.Contains(last, tag) {
			t.Errorf("a record that leaves out the invariant values has no %s: %s",
				tag, last)
		}
	}
}

// "They MUST be written again to the first record the element writes once
// an hour or more has passed since it last wrote them." (lines 151 to 152)
// The hour starts again from that record, and an hour that has passed
// several times over still writes them once.
func TestTheInvariantValuesAreWrittenAgainOnceAnHourHasPassed(t *testing.T) {
	s := newInvariantSharer(t)
	s.share(t)
	s.clock.advance(time.Hour - time.Second)
	if present, _ := carriesInvariants(s.share(t)); len(present) > 0 {
		t.Errorf("a record a second before the hour carries %v", present)
	}
	s.clock.advance(time.Second)
	if _, absent := carriesInvariants(s.share(t)); len(absent) > 0 {
		t.Errorf("the first record once the hour has passed leaves out %v", absent)
	}
	if present, _ := carriesInvariants(s.share(t)); len(present) > 0 {
		t.Errorf("the record after the one that wrote them again carries %v",
			present)
	}
	s.clock.advance(59 * time.Minute)
	if present, _ := carriesInvariants(s.share(t)); len(present) > 0 {
		t.Errorf("the hour was measured from the first record, not from "+
			"when the values were last written, so a record carries %v", present)
	}
	s.clock.advance(3 * time.Hour)
	if _, absent := carriesInvariants(s.share(t)); len(absent) > 0 {
		t.Errorf("the first record after three hours leaves out %v", absent)
	}
	if present, _ := carriesInvariants(s.share(t)); len(present) > 0 {
		t.Errorf("the values were written twice after three hours: %v", present)
	}
}

// "The element MUST create a process identifier when it is built, and MUST
// write it to every record as the ProcessId element." (lines 115 to 116) It
// is a version 4 UUID in the string format of RFC 9562 section 4 (lines 119
// to 125), and the same in every record, whether or not the record carries
// the invariant values.
func TestOneProcessKeepsOneIdentifierOnEveryRecord(t *testing.T) {
	s := newInvariantSharer(t)
	first := processID(t, s.share(t))
	if !uuidV4.MatchString(first) {
		t.Errorf("the process identifier %q is not a version 4 UUID", first)
	}
	for _, after := range []time.Duration{time.Minute, time.Hour, time.Minute} {
		s.clock.advance(after)
		if id := processID(t, s.share(t)); id != first {
			t.Errorf("a record has the process identifier %q, want %q", id, first)
		}
	}
	// The identifier comes first, as in the specification's record (line 214).
	if !strings.HasPrefix(devices(s.c)[0], "<ProcessId>") {
		t.Errorf("the record does not start with its ProcessId: %s",
			devices(s.c)[0])
	}
}

// "The identifier is never stored, so a process that starts again creates
// a new one. A process that builds more than one Usage Sharing Element, for
// example one for each of two Pipelines, creates an identifier for each"
// (lines 128 to 130). A fresh element is a new start, so it has a new
// identifier and writes the invariant values to its own first record.
func TestARestartGivesANewIdentifier(t *testing.T) {
	before := newInvariantSharer(t)
	beforeID := processID(t, before.share(t))
	before.share(t)
	before.p.Close()

	after := newInvariantSharer(t)
	record := after.share(t)
	afterID := processID(t, record)
	if !uuidV4.MatchString(afterID) {
		t.Errorf("the process identifier %q is not a version 4 UUID", afterID)
	}
	if strings.EqualFold(afterID, beforeID) {
		t.Errorf("the element built again kept the identifier %q", beforeID)
	}
	if _, absent := carriesInvariants(record); len(absent) > 0 {
		t.Errorf("the first record of the element built again leaves out %v",
			absent)
	}

	// Two elements alive at once, one for each of two Pipelines, have one
	// identifier each.
	other := newInvariantSharer(t)
	if id := processID(t, other.share(t)); strings.EqualFold(id, afterID) {
		t.Errorf("two elements share the identifier %q", id)
	}
}
