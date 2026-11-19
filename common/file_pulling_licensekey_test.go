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

package common_go

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// pullLicenseKey and otherPullLicenseKey are the license keys the masking
// tests put in the data file URL.
const (
	pullLicenseKey      = "PULLLICENSEKEY0123WXYZ"
	otherPullLicenseKey = "OTHERPULLKEY4567ABCD"
)

// maskedPullKey is key as the logging specification lets a message show it.
func maskedPullKey(key string) string {
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}

// licenseKeyLog keeps each message logged through it.
type licenseKeyLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *licenseKeyLog) Printf(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, v...))
}

func (l *licenseKeyLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

// A pull whose URL carries license keys logs only their masked form, whether
// the server refuses the request, no server answers or the URL cannot be
// parsed, as the specification asks.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/logging.md#license-keys
func TestFilePullingMasksLicenseKeys(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	cases := []struct {
		name string
		// status is the server's answer, or 0 where base is used instead.
		status int
		base   string
		keys   []string
	}{
		{"500", http.StatusInternalServerError, "", []string{pullLicenseKey}},
		{"two keys", http.StatusInternalServerError, "",
			[]string{pullLicenseKey, otherPullLicenseKey}},
		{"connection refused", 0, dead.URL, []string{pullLicenseKey}},
		{"URL that cannot be parsed", 0, "http://example.invalid/\x7f",
			[]string{pullLicenseKey}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := c.base
			if c.status != 0 {
				status := c.status
				srv := httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(status)
					}))
				defer srv.Close()
				base = srv.URL
			}
			dataFile := filepath.Join(t.TempDir(), "data.hash")
			if err := os.WriteFile(dataFile, []byte("old data"), 0644); err != nil {
				t.Fatalf("write the data file: %v", err)
			}
			updater := NewFileUpdater(base + "/?LicenseKeys=" +
				strings.Join(c.keys, "%7C") + "&Type=HashV41")
			updater.SetDataFile(dataFile)
			log := &licenseKeyLog{}
			updater.SetCustomLogger(log)

			updater.pull(make(chan struct{}, 1))

			lines := log.all()
			all := strings.Join(lines, "\n")
			for _, key := range c.keys {
				for _, line := range lines {
					if strings.Contains(line, key) {
						t.Errorf("%q holds the license key %s in full", line, key)
					}
				}
				if !strings.Contains(all, maskedPullKey(key)) {
					t.Errorf("no message names the masked key %s, the "+
						"messages being\n%s", maskedPullKey(key), all)
				}
			}
		})
	}
}
