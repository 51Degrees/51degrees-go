package usagesharing

import (
	"strings"
	"testing"
)

func evidenceXML(key, value string) string {
	var b strings.Builder
	writeEvidence(&b, key, value)
	return b.String()
}

// The page marks a value with replaced characters escaped="true", puts
// escaped before truncated before Name, and uses neither CDATA nor BadSchema.
func TestEvidenceEntryFormat(t *testing.T) {
	long := strings.Repeat("a", maxValueLength+5)
	cases := map[string]struct {
		key, value, want string
	}{
		"plain": {"header.user-agent", "agent",
			`<header Name="user-agent">agent</header>`},
		"special characters are escaped as text": {"query.51D_x", "a<b&c]]>d",
			`<query Name="51D_x">a&lt;b&amp;c]]&gt;d</query>`},
		"invalid character": {"header.x", "a\x01b",
			`<header escaped="true" Name="x">a\x0001b</header>`},
		"invalid character in the field": {"header.x\x02", "v",
			`<header escaped="true" Name="x\x0002">v</header>`},
		"truncated": {"header.x", long,
			`<header truncated="true" Name="x">` + strings.Repeat("a", maxValueLength) + `</header>`},
		"escaped and truncated": {"header.x", "\x01" + long,
			`<header escaped="true" truncated="true" Name="x">\x0001` +
				strings.Repeat("a", maxValueLength-1) + `</header>`},
		"bare key": {"flag", "\x01",
			`<flag escaped="true">\x0001</flag>`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := evidenceXML(c.key, c.value); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}

func TestRecordHasNoBadSchemaOrCDATA(t *testing.T) {
	e := &Element{}
	rec := &record{sessionID: "s", evidence: map[string]string{"header.x": "a\x01b"}}
	var b strings.Builder
	e.writeDevice(&b, rec, nil)
	out := b.String()
	if strings.Contains(out, "BadSchema") || strings.Contains(out, "CDATA") ||
		strings.Contains(out, "TRUNCATED BY USAGE SHARING") {
		t.Errorf("record uses the other form: %s", out)
	}
	if !strings.Contains(out, `escaped="true"`) {
		t.Errorf("record does not mark the escaped value: %s", out)
	}
}
