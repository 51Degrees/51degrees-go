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

package dataupdate

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// updateLicenseKey and otherUpdateLicenseKey are the license keys the
// masking tests put in the update URL.
const (
	updateLicenseKey      = "UPDATELICENSEKEY0123WXYZ"
	otherUpdateLicenseKey = "OTHERLICENSEKEY4567ABCD"
)

// maskedForTest is key as the logging specification lets a message show it.
func maskedForTest(key string) string {
	return strings.Repeat("*", len(key)-4) + key[len(key)-4:]
}

// licenseKeyLog keeps each log record as one line of text, holding the
// message and every attribute.
type licenseKeyLog struct {
	mu    sync.Mutex
	lines []string
}

func (h *licenseKeyLog) Enabled(context.Context, slog.Level) bool { return true }

func (h *licenseKeyLog) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lines = append(h.lines, b.String())
	return nil
}

func (h *licenseKeyLog) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *licenseKeyLog) WithGroup(string) slog.Handler      { return h }

func (h *licenseKeyLog) all() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.lines...)
}

// deadUpdateURL is the address of a server that has stopped, so a
// connection to it is refused.
func deadUpdateURL() string {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

// An update request whose URL carries license keys logs and returns only
// their masked form, whether the request succeeds, finds nothing new, is
// refused, cannot connect or cannot be made, as the specification asks.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/logging.md#license-keys
func TestUpdateMessagesMaskLicenseKeys(t *testing.T) {
	answer := func(status int, header, value string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if header != "" {
				w.Header().Set(header, value)
			}
			w.WriteHeader(status)
			if status == http.StatusOK {
				w.Write([]byte("new data"))
			}
		}
	}
	cases := []struct {
		name string
		// handler answers the request, or nil where no server listens.
		handler http.HandlerFunc
		// base replaces the server's address when it is set.
		base string
		// keys is the LicenseKeys value.
		keys []string
		// errNamesURL is whether the returned error names the URL.
		errNamesURL bool
	}{
		{"200", answer(http.StatusOK, "", ""), "",
			[]string{updateLicenseKey}, false},
		{"304", answer(http.StatusNotModified, "", ""), "",
			[]string{updateLicenseKey}, false},
		{"500", answer(http.StatusInternalServerError, "", ""), "",
			[]string{updateLicenseKey}, true},
		{"429", answer(http.StatusTooManyRequests, "Retry-After", "60"), "",
			[]string{updateLicenseKey}, true},
		{"MD5 mismatch", answer(http.StatusOK, "Content-MD5", "00"), "",
			[]string{updateLicenseKey}, false},
		{"two keys", answer(http.StatusInternalServerError, "", ""), "",
			[]string{updateLicenseKey, otherUpdateLicenseKey}, true},
		{"connection refused", nil, deadUpdateURL(),
			[]string{updateLicenseKey}, true},
		{"URL that cannot be parsed", nil, "http://example.invalid/\x7f",
			[]string{updateLicenseKey}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := c.base
			if c.handler != nil {
				srv := httptest.NewServer(c.handler)
				defer srv.Close()
				base = srv.URL
			}
			url := base + "/?LicenseKeys=" + strings.Join(c.keys, "%7C") +
				"&Type=HashV41"
			log := &licenseKeyLog{}
			svc := New(slog.New(log), nil)
			defer svc.Close()
			if err := svc.Register(&fakeEngine{}, "masked", Config{
				Identifier: "masked", DataUpdateURL: url}); err != nil {
				t.Fatalf("register: %v", err)
			}
			errText := ""
			if err := svc.CheckForUpdate("masked"); err != nil {
				errText = err.Error()
			}

			lines := log.all()
			for _, text := range append(lines, errText) {
				for _, key := range c.keys {
					if strings.Contains(text, key) {
						t.Errorf("%q holds the license key %s in full", text, key)
					}
				}
			}
			all := strings.Join(lines, "\n")
			for _, key := range c.keys {
				if !strings.Contains(all, maskedForTest(key)) {
					t.Errorf("no log entry names the masked key %s, the "+
						"entries being\n%s", maskedForTest(key), all)
				}
				if c.errNamesURL && !strings.Contains(errText, maskedForTest(key)) {
					t.Errorf("the error %q does not name the masked key %s",
						errText, maskedForTest(key))
				}
			}
		})
	}
}
