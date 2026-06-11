package core

import (
	"log/slog"
	"math"
	"testing"
)

func TestComparePrefixesPrecedence(t *testing.T) {
	// header has higher precedence than query? No: query is highest.
	if ComparePrefixes("query", "header") >= 0 {
		t.Fatal("query should outrank header")
	}
	if ComparePrefixes("header", "cookie") >= 0 {
		t.Fatal("header should outrank cookie")
	}
	// Known beats unknown.
	if ComparePrefixes("location", "custom") >= 0 {
		t.Fatal("known prefix should outrank unknown")
	}
	// Two unknowns ordered alphabetically.
	if ComparePrefixes("alpha", "beta") >= 0 {
		t.Fatal("unknown prefixes should be alphabetical")
	}
	if ComparePrefixes("header", "header") != 0 {
		t.Fatal("equal prefixes should compare equal")
	}
}

func TestSplitEvidenceKey(t *testing.T) {
	if p, f := SplitEvidenceKey("query.user-agent"); p != "query" || f != "user-agent" {
		t.Fatalf("got %q %q", p, f)
	}
	if p, f := SplitEvidenceKey("noprefix"); p != "" || f != "noprefix" {
		t.Fatalf("got %q %q", p, f)
	}
}

func TestEvidenceImmutableAfterProcessing(t *testing.T) {
	p, _ := NewPipelineBuilder().AddFlowElement(newStarSignElement()).SetLogger(slog.New(slog.NewTextHandler(discard{}, nil))).Build()
	defer p.Close()
	fd := p.CreateFlowData()
	fd.AddEvidence("query.month", "march")
	_ = fd.Process()
	fd.AddEvidence("query.month", "april") // should be ignored
	if v, _ := fd.Evidence().Get("query.month"); v != "march" {
		t.Fatalf("evidence mutated after processing: %v", v)
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func TestAzimuthDeclination(t *testing.T) {
	if got := AzimuthToDegrees(32767); math.Abs(got-180.0) > 1e-6 {
		t.Fatalf("azimuth max: got %v", got)
	}
	if got := DeclinationToDegrees(32767); math.Abs(got-90.0) > 1e-6 {
		t.Fatalf("declination max: got %v", got)
	}
	if got := AzimuthToDegrees(0); got != 0 {
		t.Fatalf("azimuth zero: got %v", got)
	}
}

func TestWeightedValue(t *testing.T) {
	w := NewWeightedValue("london", 32768)
	if w.Value() != "london" {
		t.Fatalf("value: %v", w.Value())
	}
	if math.Abs(w.Weighting()-0.5) > 0.001 {
		t.Fatalf("weighting: %v", w.Weighting())
	}
}

func TestDecodeWKBRPolygon(t *testing.T) {
	// A triangle ring with 3 points, big-endian, type 3 (polygon), 1 ring.
	// Build bytes: order=0, type=3, numRings=1, ring{numPoints=3, points...}
	data := []byte{
		0x00,       // big-endian
		0x03,       // polygon
		0x00, 0x01, // 1 ring
		0x00, 0x03, // 3 points
		0x00, 0x00, 0x00, 0x00, // (0,0)
		0x7F, 0xFF, 0x00, 0x00, // (32767 -> 180 lon, 0 lat)
		0x00, 0x00, 0x7F, 0xFF, // (0 lon, 32767 -> 90 lat)
	}
	wkt, err := DecodeWKBRPolygon(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := "POLYGON ((0 0, 180 0, 0 90))"
	if wkt.String() != want {
		t.Fatalf("got %q, want %q", wkt.String(), want)
	}
}

func TestBuildFromConfiguration(t *testing.T) {
	registry := NewBuilderRegistry()
	registry.Register("StarSignBuilder", func(params map[string]any, logger *slog.Logger) (FlowElement, error) {
		return newStarSignElement(), nil
	})
	opts := PipelineOptions{
		Elements: []ElementOptions{{BuilderName: "StarSign"}}, // convention match -> StarSignBuilder
		BuildParameters: map[string]any{
			"suppressProcessExceptions": true,
		},
	}
	p, err := registry.BuildFromConfiguration(opts, nil)
	if err != nil {
		t.Fatalf("build from config: %v", err)
	}
	defer p.Close()
	if !p.IsSuppressProcessExceptions() {
		t.Fatal("suppressProcessExceptions not applied from config")
	}
	if p.GetElement("starsign") == nil {
		t.Fatal("element not built from config")
	}
}

func TestStringListParam(t *testing.T) {
	params := map[string]any{"props": "a, b ,c"}
	got, ok := StringListParam(params, "props")
	if !ok || len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("got %v ok=%v", got, ok)
	}
}
