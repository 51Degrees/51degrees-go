package usagesharing

import "testing"

// Only the query values session-id and sequence are left out of the repeat
// comparison. A header, cookie or other value with either name is compared
// like any other value, so a call where it changed is not a repeat.
func TestRepeatHashLeavesOutOnlyTheQuerySessionIDAndSequence(t *testing.T) {
	with := func(key, value string) map[string]string {
		return map[string]string{"header.user-agent": "agent", key: value}
	}
	for _, key := range []string{"query.session-id", "query.sequence"} {
		if hashEvidence(with(key, "1")) != hashEvidence(with(key, "2")) {
			t.Errorf("%s is compared, so every call in a session is new", key)
		}
	}
	for _, key := range []string{
		"header.sequence", "header.session-id",
		"cookie.session-id", "server.sequence",
	} {
		if hashEvidence(with(key, "1")) == hashEvidence(with(key, "2")) {
			t.Errorf("%s is not compared, so a call where only it "+
				"changed is dropped as a repeat", key)
		}
	}
}
