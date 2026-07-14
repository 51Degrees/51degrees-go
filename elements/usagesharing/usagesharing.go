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

// Package usagesharing implements the Usage Sharing Element, which shares
// anonymized request data with 51Degrees to help improve its products.
//
// Processing is fast: each request's data is placed on a bounded queue and a
// background goroutine batches entries into a GZip-compressed XML payload and
// sends it. Duplicate requests are suppressed via a sliding-window tracker, and
// all errors are logged without disrupting the Pipeline.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md
package usagesharing

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"math/rand"
	"net/http"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/51Degrees/pipeline-go/core"
	"github.com/51Degrees/pipeline-go/elements/sequence"
)

// DataKey is the identifier for the Usage Sharing Element. It produces no
// Element Data.
const DataKey = "share-usage"

// Defaults from the usage-sharing specification.
const (
	DefaultShareURL        = "https://devices-v4.51degrees.com/new.ashx"
	DefaultMinEntries      = 50
	DefaultMaxQueueSize    = 1000
	DefaultAddTimeout      = 5 * time.Millisecond
	DefaultTakeTimeout     = 100 * time.Millisecond
	DefaultSharePercentage = 1.0
	DefaultRepeatInterval  = 20 * time.Minute
	apiVersion             = "4.0"
	productName            = "Pipeline"
	maxValueLength         = 8192
)

type config struct {
	includedQueryParams map[string]struct{}
	shareAllQueryParams bool
	shareAllEvidence    bool
	blockedHeaders      map[string]struct{}
	sharePercentage     float64
	minEntries          int
	maxQueueSize        int
	addTimeout          time.Duration
	takeTimeout         time.Duration
	shareURL            string
	repeatInterval      time.Duration
}

// Element is the Usage Sharing Element.
type Element struct {
	*core.BaseElement
	cfg        config
	httpClient *http.Client

	queue chan *record
	stop  chan struct{}
	wg    sync.WaitGroup

	tracker  *tracker
	rngMu    sync.Mutex
	rng      *rand.Rand
	startOne sync.Once

	staticMu  sync.Mutex
	staticSet bool
	flowNames []string

	closeOnce sync.Once
}

type record struct {
	sessionID string
	sequence  int
	dateSent  time.Time
	clientIP  string
	evidence  map[string]string // key: full evidence key, value: raw value
}

// Process samples and enqueues the request's shareable Evidence.
func (e *Element) Process(fd core.FlowData) error {
	e.startOne.Do(e.startConsumer)

	if !e.sample() {
		return nil
	}

	ev := e.shareableEvidence(fd)
	if len(ev) == 0 {
		return nil
	}

	// Suppress duplicates seen within the repeat-evidence window. The hash
	// excludes values designed to make a request unique (e.g. the sequence).
	if e.tracker.seenRecently(hashEvidence(ev)) {
		return nil
	}

	rec := &record{
		dateSent: time.Now().UTC(),
		evidence: ev,
	}
	if ip, ok := fd.Evidence().Get("server.client-ip"); ok {
		rec.clientIP = fmt.Sprintf("%v", ip)
	}
	if data, err := fd.Get(sequence.DataKey); err == nil {
		if v, err := data.Get(sequence.SessionIDProperty); err == nil {
			rec.sessionID, _ = v.(string)
		}
		if v, err := data.Get(sequence.SequenceProperty); err == nil {
			rec.sequence, _ = v.(int)
		}
	}

	e.enqueue(rec)
	return nil
}

func (e *Element) sample() bool {
	if e.cfg.sharePercentage >= 1.0 {
		return true
	}
	if e.cfg.sharePercentage <= 0 {
		return false
	}
	e.rngMu.Lock()
	defer e.rngMu.Unlock()
	return e.rng.Float64() < e.cfg.sharePercentage
}

// shareableEvidence returns the Evidence values that pass the share-usage
// filter, keyed by their full Evidence key.
func (e *Element) shareableEvidence(fd core.FlowData) map[string]string {
	out := map[string]string{}
	filter := e.EvidenceKeyFilter()
	for key, val := range fd.Evidence().AsMap() {
		if filter.Include(key) {
			out[key] = fmt.Sprintf("%v", val)
		}
	}
	return out
}

func (e *Element) enqueue(rec *record) {
	timer := time.NewTimer(e.cfg.addTimeout)
	defer timer.Stop()
	select {
	case e.queue <- rec:
	case <-timer.C:
		// Queue full within the add timeout: discard and log (never disrupt).
		e.Logger().Warn("usage sharing queue full; discarding entry")
	}
}

// OnPipelineAdded records the fully-qualified Flow Element names used in the
// invariant portion of each shared record.
func (e *Element) OnPipelineAdded(p core.Pipeline) error {
	if err := e.BaseElement.OnPipelineAdded(p); err != nil {
		return err
	}
	e.staticMu.Lock()
	defer e.staticMu.Unlock()
	if e.staticSet {
		return nil
	}
	for _, el := range p.FlowElements() {
		e.flowNames = append(e.flowNames, typeName(el))
	}
	e.staticSet = true
	return nil
}

// --- background consumer -------------------------------------------------

func (e *Element) startConsumer() {
	e.wg.Add(1)
	go e.consume()
}

func (e *Element) consume() {
	defer e.wg.Done()
	batch := make([]*record, 0, e.cfg.minEntries)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		e.send(batch)
		batch = batch[:0]
	}
	for {
		timer := time.NewTimer(e.cfg.takeTimeout)
		select {
		case <-e.stop:
			timer.Stop()
			// Drain anything still queued, then send a final (possibly small) batch.
			for {
				select {
				case rec := <-e.queue:
					batch = append(batch, rec)
				default:
					flush()
					return
				}
			}
		case rec := <-e.queue:
			timer.Stop()
			batch = append(batch, rec)
			if len(batch) >= e.cfg.minEntries {
				flush()
			}
		case <-timer.C:
			// Periodically flush whatever has accumulated so data is not held
			// indefinitely below the minimum batch size.
			flush()
		}
	}
}

func (e *Element) send(batch []*record) {
	xmlBytes, err := e.buildXML(batch)
	if err != nil {
		e.Logger().Error("usage sharing: failed to build XML", "error", err)
		return
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(xmlBytes); err != nil {
		e.Logger().Error("usage sharing: failed to compress payload", "error", err)
		return
	}
	if err := gz.Close(); err != nil {
		e.Logger().Error("usage sharing: failed to finalize payload", "error", err)
		return
	}
	req, err := http.NewRequest(http.MethodPost, e.cfg.shareURL, &buf)
	if err != nil {
		e.Logger().Error("usage sharing: failed to create request", "error", err)
		return
	}
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "text/xml")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		e.Logger().Warn("usage sharing: request failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		e.Logger().Warn("usage sharing: unexpected response", "status", resp.StatusCode)
	}
}

// Close drains and sends any remaining data, then stops the background goroutine.
func (e *Element) Close() error {
	e.closeOnce.Do(func() {
		// Ensure the consumer was started so the final flush runs.
		e.startOne.Do(e.startConsumer)
		close(e.stop)
		e.wg.Wait()
	})
	return nil
}

// --- XML construction ----------------------------------------------------

func (e *Element) buildXML(batch []*record) ([]byte, error) {
	// Snapshot the flow names once under the lock: they are written during
	// pipeline setup and this runs on the background flush goroutine.
	flowNames := e.flowNamesSnapshot()
	var b strings.Builder
	b.WriteString("<Devices>")
	for _, rec := range batch {
		e.writeDevice(&b, rec, flowNames)
	}
	b.WriteString("</Devices>")
	return []byte(b.String()), nil
}

// flowNamesSnapshot returns a copy of the flow names read under staticMu.
func (e *Element) flowNamesSnapshot() []string {
	e.staticMu.Lock()
	defer e.staticMu.Unlock()
	out := make([]string, len(e.flowNames))
	copy(out, e.flowNames)
	return out
}

func (e *Element) writeDevice(b *strings.Builder, rec *record, flowNames []string) {
	b.WriteString("<Device>")
	writeElement(b, "SessionId", rec.sessionID)
	if rec.sequence > 0 {
		writeElement(b, "Sequence", fmt.Sprintf("%d", rec.sequence))
	}
	writeElement(b, "DateSent", rec.dateSent.Format("2006-01-02T15:04:05"))
	writeElement(b, "Version", apiVersion)
	writeElement(b, "Product", productName)
	for _, name := range flowNames {
		writeElement(b, "FlowElement", name)
	}
	writeElement(b, "Language", "Go")
	writeElement(b, "LanguageVersion", runtime.Version())
	if rec.clientIP != "" {
		writeElement(b, "ClientIP", rec.clientIP)
	}
	writeElement(b, "ServerIP", serverIP())
	writeElement(b, "Platform", runtime.GOOS)

	// Evidence entries, deterministically ordered.
	keys := make([]string, 0, len(rec.evidence))
	for k := range rec.evidence {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		writeEvidence(b, key, rec.evidence[key])
	}
	b.WriteString("</Device>")
}

func writeElement(b *strings.Builder, name, value string) {
	b.WriteByte('<')
	b.WriteString(name)
	b.WriteByte('>')
	xml.EscapeText(b, []byte(value))
	b.WriteString("</")
	b.WriteString(name)
	b.WriteByte('>')
}

// writeEvidence writes a single Evidence value, sanitizing it and flagging the
// element with escaped/truncated attributes if the value was modified.
func writeEvidence(b *strings.Builder, key, value string) {
	prefix, field := core.SplitEvidenceKey(key)
	elementName := xmlElementName(prefix)

	sanitized, escaped := sanitizeXMLValue(value)
	truncated := false
	if len(sanitized) > maxValueLength {
		sanitized = sanitized[:maxValueLength]
		truncated = true
	}

	b.WriteByte('<')
	b.WriteString(elementName)
	if escaped {
		b.WriteString(` escaped="true"`)
	}
	if truncated {
		b.WriteString(` truncated="true"`)
	}
	b.WriteString(` Name="`)
	xml.EscapeText(b, []byte(field))
	b.WriteString(`">`)
	xml.EscapeText(b, []byte(sanitized))
	b.WriteString("</")
	b.WriteString(elementName)
	b.WriteByte('>')
}

func xmlElementName(prefix string) string {
	if prefix == "" {
		return "Evidence"
	}
	return strings.ToUpper(prefix[:1]) + prefix[1:]
}

// sanitizeXMLValue replaces characters that are not valid in XML, reporting
// whether any replacement was made.
func sanitizeXMLValue(value string) (string, bool) {
	var b strings.Builder
	changed := false
	for _, r := range value {
		if isValidXMLChar(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('�')
			changed = true
		}
	}
	return b.String(), changed
}

// isValidXMLChar reports whether r is permitted in XML 1.0 character data.
func isValidXMLChar(r rune) bool {
	return r == 0x9 || r == 0xA || r == 0xD ||
		(r >= 0x20 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0x10FFFF)
}

func typeName(v any) string {
	t := reflect.TypeOf(v)
	if t == nil {
		return ""
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.PkgPath() != "" {
		return t.PkgPath() + "." + t.Name()
	}
	return t.Name()
}

func serverIP() string { return "" } // best-effort; left blank when undetermined

// hashEvidence builds a deterministic hash of Evidence for duplicate detection,
// excluding values that are designed to make a request unique.
func hashEvidence(ev map[string]string) uint64 {
	keys := make([]string, 0, len(ev))
	for k := range ev {
		if isUniquenessKey(k) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := fnv.New64a()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0x1f})
		h.Write([]byte(ev[k]))
		h.Write([]byte{0x1e})
	}
	return h.Sum64()
}

func isUniquenessKey(key string) bool {
	_, field := core.SplitEvidenceKey(key)
	switch strings.ToLower(field) {
	case "sequence", "session-id":
		return true
	}
	return false
}

// --- duplicate tracker ---------------------------------------------------

type tracker struct {
	mu       sync.Mutex
	interval time.Duration
	seen     map[uint64]time.Time
}

func newTracker(interval time.Duration) *tracker {
	return &tracker{interval: interval, seen: make(map[uint64]time.Time)}
}

// seenRecently reports whether the hash was seen within the interval. It also
// records/refreshes the entry's timestamp and opportunistically prunes expired
// entries.
func (t *tracker) seenRecently(h uint64) bool {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	last, ok := t.seen[h]
	recently := ok && now.Sub(last) < t.interval
	// Only (re)start the window when the entry is new or already expired.
	// Refreshing the timestamp on every hit would turn the fixed window into a
	// sliding one, suppressing continuous identical traffic forever instead of
	// re-sharing it once per interval.
	if !recently {
		t.seen[h] = now
	}
	if len(t.seen) > 10000 {
		for k, ts := range t.seen {
			if now.Sub(ts) > t.interval {
				delete(t.seen, k)
			}
		}
	}
	return recently
}
