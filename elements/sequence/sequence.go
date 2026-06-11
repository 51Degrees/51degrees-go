// Package sequence implements the Sequence Element, which tracks how many times
// the client-side JavaScript has called back to the server within a session and
// maintains the session id used to key client-side caching.
//
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/sequence-element.md
package sequence

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/51Degrees/pipeline-go/core"
)

// DataKey is the Element Data key for the Sequence Element.
const DataKey = "sequence"

// Property names produced by the element.
const (
	SessionIDProperty = "session-id"
	SequenceProperty  = "sequence"
)

// Evidence keys consumed by the element.
const (
	EvidenceSessionID = "query.session-id"
	EvidenceSequence  = "query.sequence"
)

// Element is the Sequence Element.
type Element struct {
	*core.BaseElement
}

// New creates a Sequence Element.
func New(logger *slog.Logger) *Element {
	e := &Element{BaseElement: core.NewBaseElement(DataKey, core.NewWhitelistFilter(EvidenceSessionID, EvidenceSequence), logger)}
	e.SetProperties([]core.PropertyMetadata{
		{Name: SessionIDProperty, Element: e, Category: "Sequence", Available: true},
		{Name: SequenceProperty, Element: e, Category: "Sequence", Available: true},
	})
	return e
}

// Process generates or increments the session id and sequence counter.
func (e *Element) Process(fd core.FlowData) error {
	data := fd.GetOrAdd(DataKey, func(core.FlowData) core.ElementData { return core.NewBaseElementData() })

	sessionID, ok := fd.Evidence().Get(EvidenceSessionID)
	if !ok || fmt.Sprintf("%v", sessionID) == "" {
		id, err := newGUID()
		if err != nil {
			return err
		}
		sessionID = id
	}
	data.Set(SessionIDProperty, fmt.Sprintf("%v", sessionID))

	seq := 1
	if raw, ok := fd.Evidence().Get(EvidenceSequence); ok {
		if n, err := toInt(raw); err == nil {
			seq = n + 1
		}
	}
	data.Set(SequenceProperty, seq)
	return nil
}

// Factory builds a Sequence Element from configuration for core.BuilderRegistry.
func Factory(_ map[string]any, logger *slog.Logger) (core.FlowElement, error) {
	return New(logger), nil
}

func toInt(v any) (int, error) {
	switch t := v.(type) {
	case int:
		return t, nil
	case int64:
		return int(t), nil
	case float64:
		return int(t), nil
	case string:
		return strconv.Atoi(t)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

// newGUID returns a random RFC 4122 version 4 UUID string, matching the
// reference implementations' use of a GUID for the session id.
func newGUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
