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
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeEngine struct {
	mu        sync.Mutex
	fromBytes [][]byte
	fromFiles []string
}

func (f *fakeEngine) RefreshFromBytes(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	f.fromBytes = append(f.fromBytes, cp)
	return nil
}

func (f *fakeEngine) RefreshFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fromFiles = append(f.fromFiles, string(data))
	return nil
}

func (f *fakeEngine) bytesCount() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.fromBytes) }
func (f *fakeEngine) lastFile() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.fromFiles) == 0 {
		return ""
	}
	return f.fromFiles[len(f.fromFiles)-1]
}

func TestUpdateOnStartupMemory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("DATA1"))
	}))
	defer srv.Close()

	eng := &fakeEngine{}
	svc := New(nil, srv.Client())
	defer svc.Close()
	err := svc.Register(eng, "memEngine", Config{
		Identifier:      "mem",
		DataUpdateURL:   srv.URL,
		UpdateOnStartup: true,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if eng.bytesCount() != 1 || string(eng.fromBytes[0]) != "DATA1" {
		t.Fatalf("expected in-memory refresh with DATA1, got %v", eng.fromBytes)
	}
}

func TestUpdateOnStartupFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DATA2"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	eng := &fakeEngine{}
	svc := New(nil, srv.Client())
	defer svc.Close()
	if err := svc.Register(eng, "fileEngine", Config{
		Identifier:      "file",
		DataFilePath:    path,
		DataUpdateURL:   srv.URL,
		UpdateOnStartup: true,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	// File written atomically and engine refreshed from it.
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "DATA2" {
		t.Fatalf("data file not written: %q %v", content, err)
	}
	if eng.lastFile() != "DATA2" {
		t.Fatalf("engine not refreshed from file: %q", eng.lastFile())
	}
}

func TestIfModifiedSince304(t *testing.T) {
	var sawIfModifiedSince bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-Modified-Since") != "" {
			sawIfModifiedSince = true
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Write([]byte("NEW"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(path, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := &fakeEngine{}
	svc := New(nil, srv.Client())
	defer svc.Close()
	if err := svc.Register(eng, "ims", Config{
		Identifier:         "ims",
		DataFilePath:       path,
		DataUpdateURL:      srv.URL,
		UseIfModifiedSince: true,
		UpdateOnStartup:    true,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if !sawIfModifiedSince {
		t.Fatal("expected If-Modified-Since header to be sent")
	}
	// 304 => file unchanged, no refresh.
	if content, _ := os.ReadFile(path); string(content) != "OLD" {
		t.Fatalf("file should be unchanged on 304, got %q", content)
	}
	if len(eng.fromFiles) != 0 {
		t.Fatal("engine should not refresh on 304")
	}
}

func TestRateLimit429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	eng := &fakeEngine{}
	svc := New(nil, srv.Client())
	defer svc.Close()
	_ = svc.Register(eng, "rl", Config{Identifier: "rl", DataUpdateURL: srv.URL})
	err := svc.CheckForUpdate("rl")
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("expected RateLimitError, got %v", err)
	}
	if rle.RetryAfter != 120*time.Second {
		t.Fatalf("expected 120s retry-after, got %v", rle.RetryAfter)
	}
}

func TestProgrammaticFileUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(path, []byte("V1"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := &fakeEngine{}
	svc := New(nil, nil)
	defer svc.Close()
	if err := svc.Register(eng, "prog", Config{Identifier: "prog", DataFilePath: path}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := svc.CheckForUpdate("prog"); err != nil {
		t.Fatalf("check for update: %v", err)
	}
	if eng.lastFile() != "V1" {
		t.Fatalf("expected refresh from file, got %q", eng.lastFile())
	}
}

func TestGzipResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		gz.Write([]byte("DECOMPRESSED"))
		gz.Close()
	}))
	defer srv.Close()

	eng := &fakeEngine{}
	svc := New(nil, srv.Client())
	defer svc.Close()
	if err := svc.Register(eng, "gz", Config{Identifier: "gz", DataUpdateURL: srv.URL, UpdateOnStartup: true}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if eng.bytesCount() != 1 || string(eng.fromBytes[0]) != "DECOMPRESSED" {
		t.Fatalf("gzip not decompressed: %v", eng.fromBytes)
	}
}

// gzipBytes returns the gzip-compressed form of b.
func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestGzipBodyWithoutContentEncoding covers the real Distributor case: a gzip
// body served as application/gzip with no Content-Encoding header (so the HTTP
// transport does not transparently decompress it). The service must detect the
// gzip from the body itself and decompress it.
func TestGzipBodyWithoutContentEncoding(t *testing.T) {
	payload := []byte("DECOMPRESSED-BODY")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		w.Write(gzipBytes(t, payload))
	}))
	defer srv.Close()

	eng := &fakeEngine{}
	svc := New(nil, srv.Client())
	defer svc.Close()
	if err := svc.Register(eng, "gzbody", Config{Identifier: "gzbody", DataUpdateURL: srv.URL, UpdateOnStartup: true}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if eng.bytesCount() != 1 || string(eng.fromBytes[0]) != string(payload) {
		t.Fatalf("gzip body not decompressed: %v", eng.fromBytes)
	}
}

// TestContentMD5Valid confirms a matching Content-MD5 header allows the update.
func TestContentMD5Valid(t *testing.T) {
	payload := []byte("VALID-DATA")
	sum := md5.Sum(payload)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-MD5", hex.EncodeToString(sum[:]))
		w.Write(payload)
	}))
	defer srv.Close()

	eng := &fakeEngine{}
	svc := New(nil, srv.Client())
	defer svc.Close()
	if err := svc.Register(eng, "md5ok", Config{Identifier: "md5ok", DataUpdateURL: srv.URL, UpdateOnStartup: true}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if eng.bytesCount() != 1 || string(eng.fromBytes[0]) != string(payload) {
		t.Fatalf("expected refresh with valid MD5, got %v", eng.fromBytes)
	}
}

// TestContentMD5Mismatch confirms a bad Content-MD5 header rejects the update so
// a corrupt download is never loaded into the Engine.
func TestContentMD5Mismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-MD5", hex.EncodeToString(md5.New().Sum(nil))) // md5 of empty, won't match
		w.Write([]byte("CORRUPT-DATA"))
	}))
	defer srv.Close()

	eng := &fakeEngine{}
	svc := New(nil, srv.Client())
	defer svc.Close()
	_ = svc.Register(eng, "md5bad", Config{Identifier: "md5bad", DataUpdateURL: srv.URL})
	if err := svc.CheckForUpdate("md5bad"); err == nil {
		t.Fatal("expected Content-MD5 mismatch to fail the update")
	}
	if eng.bytesCount() != 0 {
		t.Fatalf("engine must not be refreshed when MD5 mismatches, got %v", eng.fromBytes)
	}
}

func TestParseRetryAfter(t *testing.T) {
	if d := parseRetryAfter("30"); d != 30*time.Second {
		t.Fatalf("delta-seconds: got %v", d)
	}
	future := time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)
	if d := parseRetryAfter(future); d <= 0 {
		t.Fatalf("http-date: got %v", d)
	}
	if d := parseRetryAfter(""); d != 0 {
		t.Fatalf("empty: got %v", d)
	}
}

func TestFileWatcher(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(path, []byte("INITIAL"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng := &fakeEngine{}
	svc := New(nil, nil)
	defer svc.Close()

	updated := make(chan struct{}, 1)
	if err := svc.Register(eng, "watch", Config{
		Identifier:               "watch",
		DataFilePath:             path,
		AutomaticUpdatesEnabled:  true,
		FileSystemWatcherEnabled: true,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	svc.OnUpdate("watch", func(Complete) {
		select {
		case updated <- struct{}{}:
		default:
		}
	})

	// Replace the file atomically (write a temp file then rename over the
	// target), which is how data files are replaced in practice. The watcher
	// should trigger a refresh that sees the complete new content.
	tmp := filepath.Join(dir, "incoming.bin")
	if err := os.WriteFile(tmp, []byte("UPDATED"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
	select {
	case <-updated:
	case <-time.After(3 * time.Second):
		t.Fatal("file watcher did not trigger refresh within timeout")
	}
	// Poll briefly: a single os.Rename can surface as more than one event, so
	// allow the final refresh to settle on the complete content.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if eng.lastFile() == "UPDATED" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected refresh with UPDATED, got %q", eng.lastFile())
}
