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
// sends it, pausing after repeated failed sends. Identical Evidence is shared
// at most once in each repeat Evidence interval, and all errors are logged
// without disrupting the Pipeline.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md
package usagesharing

import (
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand"
	"net"
	"net/http"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/51Degrees/51degrees-go/pipeline/v4/elements/sequence"
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
	apiVersion             = "4.0" // fallback when the build version is unavailable
	productName            = "Pipeline"
	maxValueLength         = 10000
	truncatedMarker        = "[TRUNCATED BY USAGE SHARING] "
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
	recovery            recoveryConfig
	// err refuses a configuration that cannot work, when the element is
	// added to a Pipeline.
	err error
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

	// ready wakes the sending goroutine when the queue may hold enough
	// entries to start a message.
	ready chan struct{}
	// discarded counts the entries discarded since the queue was last found
	// full, so the log has one entry for each time it fills.
	discarded atomic.Int64
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
	if e.cfg.err != nil {
		return e.cfg.err
	}
	e.startOne.Do(e.startConsumer)

	if !e.sample() {
		return nil
	}

	ev := e.shareableEvidence(fd)
	if len(ev) == 0 {
		return nil
	}

	// Share identical Evidence at most once in each repeat Evidence interval.
	// The hash excludes values designed to make a request unique (e.g. the
	// sequence).
	shared, undo := e.tracker.share(hashEvidence(ev))
	if !shared {
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

	// Evidence the full queue discards was not shared, so it is shared as
	// new when next seen.
	if !e.enqueue(rec) {
		undo()
	}
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

// enqueue adds rec to the queue, waiting up to the add timeout for room, and
// reports whether it was added. A full queue discards the entry. The log
// says when discarding starts and how many were discarded once there is room
// again, rather than once for each entry, which on a busy server would flood
// it.
func (e *Element) enqueue(rec *record) bool {
	timer := time.NewTimer(e.cfg.addTimeout)
	defer timer.Stop()
	select {
	case e.queue <- rec:
	case <-timer.C:
		if e.discarded.Add(1) == 1 {
			e.Logger().Warn("usage sharing queue is full, so entries are " +
				"discarded until there is room")
		}
		return false
	}
	if n := e.discarded.Swap(0); n > 0 {
		e.Logger().Info("usage sharing queue has room again", "discarded", n)
	}
	if len(e.queue) >= e.minimum() {
		select {
		case e.ready <- struct{}{}:
		default:
		}
	}
	return true
}

// OnPipelineAdded records the fully-qualified Flow Element names used in the
// invariant portion of each shared record.
func (e *Element) OnPipelineAdded(p core.Pipeline) error {
	if e.cfg.err != nil {
		return e.cfg.err
	}
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
	e.ready = make(chan struct{}, 1)
	e.wg.Add(1)
	go e.consume()
}

// minimum returns the minimum entries per message, being at least 1.
func (e *Element) minimum() int {
	return max(1, e.cfg.minEntries)
}

// consume sends one message at a time until the element is closed. A
// message is started once the queue holds the minimum entries per message,
// and the queue is sent from until it holds fewer, so a queue that filled
// during a pause drains at once. Repeated failed sends pause sending, and
// processing goes on adding to the queue meanwhile.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md#error-handling
func (e *Element) consume() {
	defer e.wg.Done()
	b := &backOff{cfg: e.cfg.recovery}
	for {
		select {
		case <-e.stop:
			e.finish(b)
			return
		default:
		}
		if wait := b.remaining(time.Now()); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-e.stop:
				timer.Stop()
				e.finish(b)
				return
			case <-timer.C:
			}
			continue
		}
		if len(e.queue) < e.minimum() {
			select {
			case <-e.stop:
				e.finish(b)
				return
			case <-e.ready:
			}
			continue
		}
		ok := e.send(e.takeMessage())
		switch b.record(ok, time.Now()) {
		case backOffPaused:
			e.Logger().Warn("usage sharing paused sending after failed sends",
				"delay", b.delay)
		case backOffLengthened:
			e.Logger().Warn("usage sharing paused sending again after the "+
				"trial send failed", "delay", b.delay)
		case backOffResumed:
			e.Logger().Info("usage sharing resumed sending")
		}
	}
}

// takeMessage takes the entries for one message, waiting up to the take
// timeout for each one, until a wait ends with no entry, the message holds
// twice the minimum entries per message or the element is closed. With a take
// timeout of zero it takes only the entries already queued.
func (e *Element) takeMessage() []*record {
	limit := 2 * e.minimum()
	batch := make([]*record, 0, limit)
	for len(batch) < limit {
		select {
		case rec := <-e.queue:
			batch = append(batch, rec)
			continue
		default:
		}
		if e.cfg.takeTimeout <= 0 {
			return batch
		}
		timer := time.NewTimer(e.cfg.takeTimeout)
		select {
		case rec := <-e.queue:
			timer.Stop()
			batch = append(batch, rec)
		case <-timer.C:
			return batch
		case <-e.stop:
			// The element is closing, so the entries taken are sent now
			// rather than after a take timeout.
			timer.Stop()
			return batch
		}
	}
	return batch
}

// finish ends sending when the element is closed. While sending is paused it
// sends nothing and discards what is queued, logging how many. Otherwise it
// sends whatever is left in one final message.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md#cleanup
func (e *Element) finish(b *backOff) {
	var rest []*record
	for drained := false; !drained; {
		select {
		case rec := <-e.queue:
			rest = append(rest, rec)
		default:
			drained = true
		}
	}
	if len(rest) == 0 {
		return
	}
	if b.remaining(time.Now()) > 0 {
		e.Logger().Warn("usage sharing closed while sending was paused, so "+
			"the queued entries were discarded", "discarded", len(rest))
		return
	}
	e.send(rest)
}

// send sends one message and reports whether it succeeded. It fails when the
// request cannot be made, has no answer within the HTTP client's timeout or
// is answered with anything other than 200, whose status code and content are
// logged.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md#request-detail
func (e *Element) send(batch []*record) bool {
	xmlBytes, err := e.buildXML(batch)
	if err != nil {
		e.Logger().Error("usage sharing: failed to build XML", "error", err)
		return false
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(xmlBytes); err != nil {
		e.Logger().Error("usage sharing: failed to compress payload", "error", err)
		return false
	}
	if err := gz.Close(); err != nil {
		e.Logger().Error("usage sharing: failed to finalize payload", "error", err)
		return false
	}
	req, err := http.NewRequest(http.MethodPost, e.cfg.shareURL, &buf)
	if err != nil {
		e.Logger().Error("usage sharing: failed to create request", "error", err)
		return false
	}
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "text/xml")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		e.Logger().Warn("usage sharing: request failed", "error", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		content, _ := io.ReadAll(io.LimitReader(resp.Body, maxLoggedResponseBytes))
		e.Logger().Warn("usage sharing: the service refused the message",
			"status", resp.StatusCode, "content", string(content))
		return false
	}
	return true
}

// maxLoggedResponseBytes limits how much of a refused message's response is
// read for the log.
const maxLoggedResponseBytes = 64 << 10

// Close sends any remaining data, unless sending is paused, then stops the
// background goroutine.
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
	// Field order and always-present fields are fixed so the payload validates
	// against the server-side schema.
	writeElement(b, "SessionId", rec.sessionID)
	writeElement(b, "Sequence", strconv.Itoa(rec.sequence))
	writeElement(b, "DateSent", rec.dateSent.Format("2006-01-02T15:04:05"))
	writeElement(b, "ClientIP", rec.clientIP)
	writeElement(b, "Version", coreVersion())
	writeElement(b, "Product", productName)
	for _, name := range flowNames {
		writeElement(b, "FlowElement", name)
	}
	writeElement(b, "Language", "Go")
	writeElement(b, "LanguageVersion", runtime.Version())
	writeElement(b, "ServerIP", serverIP())
	writeElement(b, "Platform", runtime.GOOS)

	// Evidence entries, deterministically ordered. A single BadSchema element is
	// added at the end if any value contained characters that had to be encoded.
	keys := make([]string, 0, len(rec.evidence))
	for k := range rec.evidence {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	badSchema := false
	for _, key := range keys {
		if writeEvidence(b, key, rec.evidence[key]) {
			badSchema = true
		}
	}
	if badSchema {
		writeElement(b, "BadSchema", "true")
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

// writeEvidence writes a single Evidence value using the usage-sharing schema:
// a prefixed key becomes <prefix Name="field"><![CDATA[value]]></prefix> with
// the prefix (category) lower-case as-is, and a bare key becomes
// <field>value</field>. Over-long values are truncated with a marker and
// invalid XML characters are hex-encoded. It returns true when any character
// was encoded, so the caller can emit a single BadSchema element.
func writeEvidence(b *strings.Builder, key, value string) bool {
	prefix, field := core.SplitEvidenceKey(key)

	if r := []rune(value); len(r) > maxValueLength {
		value = truncatedMarker + string(r[:maxValueLength])
	}
	encodedValue, badValue := encodeInvalidXMLChars(value)
	encodedField, badField := encodeInvalidXMLChars(field)

	if prefix == "" {
		b.WriteByte('<')
		b.WriteString(encodedField)
		b.WriteByte('>')
		xml.EscapeText(b, []byte(encodedValue))
		b.WriteString("</")
		b.WriteString(encodedField)
		b.WriteByte('>')
		return badValue || badField
	}

	b.WriteByte('<')
	b.WriteString(prefix)
	b.WriteString(` Name="`)
	xml.EscapeText(b, []byte(encodedField))
	b.WriteString(`">`)
	writeCDATA(b, encodedValue)
	b.WriteString("</")
	b.WriteString(prefix)
	b.WriteByte('>')
	return badValue || badField
}

// writeCDATA writes value inside a CDATA section, splitting any "]]>" so it
// cannot close the section early.
func writeCDATA(b *strings.Builder, value string) {
	b.WriteString("<![CDATA[")
	b.WriteString(strings.ReplaceAll(value, "]]>", "]]]]><![CDATA[>"))
	b.WriteString("]]>")
}

// encodeInvalidXMLChars replaces characters not valid in XML with a \xNNNN
// escape and reports whether any were replaced.
func encodeInvalidXMLChars(text string) (string, bool) {
	valid := true
	for _, r := range text {
		if !isValidXMLChar(r) {
			valid = false
			break
		}
	}
	if valid {
		return text, false
	}
	var b strings.Builder
	for _, r := range text {
		if isValidXMLChar(r) {
			b.WriteRune(r)
		} else {
			fmt.Fprintf(&b, `\x%04x`, r)
		}
	}
	return b.String(), true
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

var (
	serverIPOnce  sync.Once
	serverIPValue string
)

// serverIP returns this machine's non-loopback IP address, preferring IPv4, or
// "" if none can be determined. It is resolved once and cached.
func serverIP() string {
	serverIPOnce.Do(func() {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return
		}
		var fallback string
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() {
				continue
			}
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				serverIPValue = ip4.String()
				return
			}
			if fallback == "" {
				fallback = ipNet.IP.String()
			}
		}
		serverIPValue = fallback
	})
	return serverIPValue
}

// coreVersion returns the Pipeline module version from the build info, or the
// fallback apiVersion when it is unavailable (for example in tests or a dev
// build), so shared usage reports which Pipeline version produced it.
func coreVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, d := range info.Deps {
			if d.Path == "github.com/51Degrees/51degrees-go/pipeline/v4" && d.Version != "" {
				return d.Version
			}
		}
		if info.Main.Path == "github.com/51Degrees/51degrees-go/pipeline/v4" &&
			info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}
	return apiVersion
}

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

// tracker holds the time each Evidence hash was last shared.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/usage-sharing-element.md#session-tracking
type tracker struct {
	mu       sync.Mutex
	interval time.Duration
	seen     map[uint64]time.Time
}

func newTracker(interval time.Duration) *tracker {
	return &tracker{interval: interval, seen: make(map[uint64]time.Time)}
}

// share reports whether Evidence with hash h is to be shared now, being when
// it has not been shared within the interval, and records the time it is
// shared. Evidence seen again within the interval leaves the time as it is,
// so Evidence seen continuously is still shared once in each interval. undo
// restores the entry as it was, for Evidence that was then not shared.
// Entries whose interval has passed are removed once there are many.
func (t *tracker) share(h uint64) (shared bool, undo func()) {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	last, existed := t.seen[h]
	if existed && now.Sub(last) < t.interval {
		return false, func() {}
	}
	t.seen[h] = now
	if len(t.seen) > 10000 {
		for k, ts := range t.seen {
			if now.Sub(ts) >= t.interval {
				delete(t.seen, k)
			}
		}
	}
	return true, func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if current, ok := t.seen[h]; ok && current.Equal(now) {
			if existed {
				t.seen[h] = last
			} else {
				delete(t.seen, h)
			}
		}
	}
}
