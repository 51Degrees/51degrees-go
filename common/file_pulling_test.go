package common_go

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var mockHashMutex sync.Mutex

func newMockDataFileServer(timeout time.Duration) *httptest.Server {
	s := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				// Open the file for reading
				mockHashMutex.Lock()
				file, err := os.Open("./mock_hash.gz")
				if err != nil {
					log.Printf("Failed to open file: %v", err)
					return
				}
				defer mockHashMutex.Unlock()
				defer file.Close()

				buffer := bytes.NewBuffer(make([]byte, 0))
				_, err = io.Copy(buffer, file)
				if err != nil {
					log.Printf("Failed to read file: %v", err)
					return
				}

				w.Header().Add("Content-MD5", "daebfa89ddefac8e6c4325c38f129504")
				w.Header().Add("Content-Length", strconv.Itoa(buffer.Len()))
				w.Header().Add("Content-Type", "application/octet-stream")

				w.WriteHeader(http.StatusOK)

				// Write the file to response
				_, err = io.Copy(w, buffer)
				if err != nil {
					log.Printf("Failed to write file: %v", err)
					return
				}

			},
		),
	)

	s.URL = strings.ReplaceAll(s.URL, "127.0.0.1", "localhost")

	return awaitServer(s, timeout)
}

func awaitServer(s *httptest.Server, timeout time.Duration) *httptest.Server {
	readyChan := make(chan struct{}, 1)
	go func(s *httptest.Server) {
		for {
			resp, err := http.Get(s.URL)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				readyChan <- struct{}{}
				close(readyChan)
				return
			}
			resp.Body.Close()
			time.Sleep(50 * time.Millisecond)
		}
	}(s)

	select {
	case <-readyChan:
		return s
	case <-time.After(timeout):
		log.Fatalf("Failed to start server in %v", timeout)
	}

	return nil
}

func newMockUncompressedDataFileServer(timeout time.Duration) *httptest.Server {
	s := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				// Open the file for reading
				mockHashMutex.Lock()
				file, err := os.Open("./mock_hash.gz")
				if err != nil {
					log.Printf("Failed to open file: %v", err)
					return
				}
				defer mockHashMutex.Unlock()
				defer file.Close()
				w.Header().Add("Content-Type", "application/octet-stream")

				// uncompressed the file
				fileBytes, err := gzip.NewReader(file)
				if err != nil {
					log.Printf("Failed to decompress file: %v", err)
					return
				}

				w.WriteHeader(http.StatusOK)

				// Write the file to response
				_, err = io.Copy(w, fileBytes)
				if err != nil {
					log.Printf("Failed to write file: %v", err)
					return
				}

			},
		),
	)

	s.URL = strings.ReplaceAll(s.URL, "127.0.0.1", "localhost")

	return awaitServer(s, timeout)
}

// Mock implementations for testing
type mockFileUpdater struct {
	dataFile               string
	dataFileUrl            string
	dataFilePullEveryMs    int
	isUpdateOnStartEnabled bool
	isFileWatcherEnabled   bool
	logger                 *mockLogWrapper
	totalFilePulls         int
	fileSynced             bool
	lastModificationTime   *time.Time
	randomization          int
}

func (m *mockFileUpdater) GetDataFile() string {
	return m.dataFile
}

func (m *mockFileUpdater) GetDataFileUrl() string {
	return m.dataFileUrl
}

func (m *mockFileUpdater) GetDataFilePullEveryMs() int {
	return m.dataFilePullEveryMs
}

func (m *mockFileUpdater) SetLastModificationTimestamp(t *time.Time) {
	m.lastModificationTime = t
}

func (m *mockFileUpdater) getFilePath() string {
	return m.dataFile
}

// FileUpdater interface for testing
type FileUpdaterI interface {
	GetDataFile() string
	GetDataFileUrl() string
	GetDataFilePullEveryMs() int
	SetLastModificationTimestamp(*time.Time)
	getFilePath() string
}

// Test data creation helpers
func createTestFile(t *testing.T, content string) string {
	tempFile, err := os.CreateTemp("", "test_data_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tempFile.Close()

	_, err = tempFile.WriteString(content)
	if err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	return tempFile.Name()
}

func createGzipFile(t *testing.T, content string) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, err := gw.Write([]byte(content))
	if err != nil {
		t.Fatalf("Failed to write to gzip: %v", err)
	}
	gw.Close()
	return buf.Bytes()
}

func readTempFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func calculateMD5(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// Mock HTTP servers for testing
func createMockServer(statusCode int, headers map[string]string, body []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, value := range headers {
			w.Header().Set(key, value)
		}
		w.WriteHeader(statusCode)
		if body != nil {
			w.Write(body)
		}
	}))
}

func TestIsGzip(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{"gzip content type", "application/gzip", true},
		{"x-gzip content type", "application/x-gzip", true},
		{"plain text", "text/plain", false},
		{"json", "application/json", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isGzip(tt.contentType)
			if result != tt.expected {
				t.Errorf("isGzip(%s) = %v, want %v", tt.contentType, result, tt.expected)
			}
		})
	}
}

func TestValidateMd5(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		hash     string
		expected bool
		hasError bool
	}{
		{
			name:     "valid hash",
			data:     []byte("test data"),
			hash:     calculateMD5([]byte("test data")),
			expected: true,
			hasError: false,
		},
		{
			name:     "invalid hash",
			data:     []byte("test data"),
			hash:     "invalid_hash",
			expected: false,
			hasError: false,
		},
		{
			name:     "empty data",
			data:     []byte(""),
			hash:     calculateMD5([]byte("")),
			expected: true,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateMd5(tt.data, tt.hash)

			if tt.hasError && err == nil {
				t.Errorf("validateMd5() expected error but got none")
			}
			if !tt.hasError && err != nil {
				t.Errorf("validateMd5() unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("validateMd5() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDoDataFileRequest_Success(t *testing.T) {
	testData := "test file content"
	headers := map[string]string{
		"Content-Type": "application/octet-stream",
		"Content-MD5":  calculateMD5([]byte(testData)),
	}

	server := createMockServer(http.StatusOK, headers, []byte(testData))
	defer server.Close()

	logger := &LogWrapper{}

	result, err := doDataFileRequest(context.Background(), server.URL, logger, nil, filepath.Join(t.TempDir(), "data"))

	if err != nil {
		t.Errorf("doDataFileRequest() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("doDataFileRequest() returned nil result")
	}
	if got := readTempFile(t, result.path); got != testData {
		t.Errorf("doDataFileRequest() content = %s, want %s", got, testData)
	}
	if result.retryAfter != 0 {
		t.Errorf("doDataFileRequest() retryAfter = %d, want 0", result.retryAfter)
	}
}

func TestDoDataFileRequest_GzipContent(t *testing.T) {
	testData := "compressed test content"
	gzipData := createGzipFile(t, testData)

	headers := map[string]string{
		"Content-Type": "application/gzip",
		"Content-MD5":  calculateMD5(gzipData),
	}

	server := createMockServer(http.StatusOK, headers, gzipData)
	defer server.Close()

	logger := &LogWrapper{}

	result, err := doDataFileRequest(context.Background(), server.URL, logger, nil, filepath.Join(t.TempDir(), "data"))

	if err != nil {
		t.Errorf("doDataFileRequest() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("doDataFileRequest() returned nil result")
	}
	if got := readTempFile(t, result.path); got != testData {
		t.Errorf("doDataFileRequest() content = %s, want %s", got, testData)
	}
}

func TestDoDataFileRequest_NotModified(t *testing.T) {
	server := createMockServer(http.StatusNotModified, nil, nil)
	defer server.Close()

	logger := &LogWrapper{}
	timestamp := time.Now()

	result, err := doDataFileRequest(context.Background(), server.URL, logger, &timestamp, filepath.Join(t.TempDir(), "data"))

	if !errors.Is(err, ErrFileNotModified) {
		t.Errorf("doDataFileRequest() error = %v, want ErrFileNotModified", err)
	}
	if result != nil {
		t.Errorf("doDataFileRequest() result should be nil for not modified")
	}
}
func TestDoDataFileRequest_InvalidRetryAfter(t *testing.T) {
	headers := map[string]string{
		"Retry-After": "invalid",
	}

	server := createMockServer(http.StatusTooManyRequests, headers, nil)
	defer server.Close()

	logger := &LogWrapper{}

	result, err := doDataFileRequest(context.Background(), server.URL, logger, nil, filepath.Join(t.TempDir(), "data"))

	if err == nil {
		t.Errorf("doDataFileRequest() expected error for invalid Retry-After")
	}
	if result != nil {
		t.Errorf("doDataFileRequest() result should be nil for invalid Retry-After")
	}
}

func TestDoDataFileRequest_ServerError(t *testing.T) {
	server := createMockServer(http.StatusInternalServerError, nil, nil)
	defer server.Close()

	logger := &LogWrapper{}

	result, err := doDataFileRequest(context.Background(), server.URL, logger, nil, filepath.Join(t.TempDir(), "data"))

	if err == nil {
		t.Errorf("doDataFileRequest() expected error for server error")
	}
	if result != nil {
		t.Errorf("doDataFileRequest() result should be nil for server error")
	}
}

func TestDoDataFileRequest_InvalidMD5(t *testing.T) {
	testData := "test content"
	headers := map[string]string{
		"Content-Type": "application/octet-stream",
		"Content-MD5":  "invalid_md5_hash",
	}

	server := createMockServer(http.StatusOK, headers, []byte(testData))
	defer server.Close()

	logger := &LogWrapper{}

	result, err := doDataFileRequest(context.Background(), server.URL, logger, nil, filepath.Join(t.TempDir(), "data"))

	if err == nil {
		t.Errorf("doDataFileRequest() expected error for invalid MD5")
	}
	if result != nil {
		t.Errorf("doDataFileRequest() result should be nil for invalid MD5")
	}
}

func TestDoDataFileRequest_NoMD5Header(t *testing.T) {
	testData := "test content"
	headers := map[string]string{
		"Content-Type": "application/octet-stream",
	}

	server := createMockServer(http.StatusOK, headers, []byte(testData))
	defer server.Close()

	logger := &LogWrapper{}

	result, err := doDataFileRequest(context.Background(), server.URL, logger, nil, filepath.Join(t.TempDir(), "data"))

	if err != nil {
		t.Errorf("doDataFileRequest() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("doDataFileRequest() returned nil result")
	}
}

func TestDoDataFileRequest_InvalidURL(t *testing.T) {
	logger := &LogWrapper{}

	result, err := doDataFileRequest(context.Background(), "invalid://url", logger, nil, filepath.Join(t.TempDir(), "data"))

	if err == nil {
		t.Errorf("doDataFileRequest() expected error for invalid URL")
	}
	if result != nil {
		t.Errorf("doDataFileRequest() result should be nil for invalid URL")
	}
}

// A failed pull must leave the current data file untouched and no temp file behind.
func TestDoFilePulling_KeepsDataFileOnFailure(t *testing.T) {
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.hash")
	if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}

	for name, server := range map[string]*httptest.Server{
		"server error": createMockServer(http.StatusInternalServerError, nil, nil),
		"bad md5":      createMockServer(http.StatusOK, map[string]string{"Content-MD5": "wrong"}, []byte("new")),
	} {
		t.Run(name, func(t *testing.T) {
			defer server.Close()
			updater := NewFileUpdater(server.URL)
			updater.SetDataFile(dataFile)
			updater.SetLoggerEnabled(false)

			if got := readTempFile(t, dataFile); got != "current" {
				t.Errorf("data file content = %q, want unchanged", got)
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 1 {
				t.Errorf("directory has %d entries, want only the data file", len(entries))
			}
		})
	}
}

// failedPullIntervalMs is a polling interval no default uses, so a test can
// tell it from any other wait.
const failedPullIntervalMs = 47 * 60 * 1000

// A failed pull is tried again after the polling interval, as a check that
// finds nothing is, so a failing server is not asked again every second.
// See https://github.com/51Degrees/specifications/blob/67ead69/pipeline-specification/features/data-updates.md#update-polling
func TestDoFilePulling_TriesAgainAfterThePollingInterval(t *testing.T) {
	refused := httptest.NewServer(http.NotFoundHandler())
	refused.Close()
	for name, test := range map[string]struct {
		server  *httptest.Server
		missing bool
	}{
		"connection refused": {server: refused},
		"server error":       {server: createMockServer(http.StatusInternalServerError, nil, nil)},
		"bad md5":            {server: createMockServer(http.StatusOK, map[string]string{"Content-MD5": "wrong"}, []byte("new"))},
		"no data file":       {server: createMockServer(http.StatusOK, nil, []byte("new")), missing: true},
		"429 without a time": {server: createMockServer(http.StatusTooManyRequests, nil, nil)},
	} {
		t.Run(name, func(t *testing.T) {
			defer test.server.Close()
			dataFile := filepath.Join(t.TempDir(), "data.hash")
			if !test.missing {
				if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			updater := NewFileUpdater(test.server.URL)
			updater.SetDataFile(dataFile)
			updater.SetLoggerEnabled(false)
			updater.SetDataFilePullEveryMs(failedPullIntervalMs)

			if next, _, _ := updater.pull(make(chan struct{}, 1)); next != failedPullIntervalMs {
				t.Errorf("next pull in %d ms, want the polling interval of %d ms", next, failedPullIntervalMs)
			}
		})
	}
}

// A successful pull replaces the data file with the uncompressed download in one rename.
func TestDoFilePulling_ReplacesDataFile(t *testing.T) {
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.hash")
	if err := os.WriteFile(dataFile, []byte("current"), 0640); err != nil {
		t.Fatal(err)
	}
	// a pull of this file interrupted earlier left this behind; the other file belongs to another
	// updater sharing the directory and may be mid-download, so it must survive
	for _, name := range []string{".data.hash.pull-stale", ".other.ipi.pull-inprogress"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("partial"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	gzipData := createGzipFile(t, "new content")
	server := createMockServer(http.StatusOK, map[string]string{"Content-MD5": calculateMD5(gzipData)}, gzipData)
	defer server.Close()

	updater := NewFileUpdater(server.URL)
	updater.SetDataFile(dataFile)
	updater.SetLoggerEnabled(false)
	updater.SetIsFileWatcherEnabled(false) // the pull itself must announce the new file
	reload := make(chan struct{}, 1)

	if next, _, _ := updater.pull(reload); next != updater.GetDataFilePullEveryMs() {
		t.Errorf("next pull in %d ms, want %d", next, updater.GetDataFilePullEveryMs())
	}
	if got := readTempFile(t, dataFile); got != "new content" {
		t.Errorf("data file content = %q, want the download", got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".data.hash.pull-stale")); !os.IsNotExist(err) {
		t.Error("own stale temp file was not removed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".other.ipi.pull-inprogress")); err != nil {
		t.Error("another updater's temp file was removed")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Errorf("directory has %d entries, want the data file and the other updater's temp file", len(entries))
	}
	if info, _ := os.Stat(dataFile); runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		t.Errorf("data file mode = %v, want the previous 0640", info.Mode().Perm())
	}
	select {
	case <-reload:
	default:
		t.Error("no reload event sent")
	}
}

// The engines copy the data file for each reload in their own goroutine while
// the puller runs. Run with -race, which reports any field the two use without
// synchronisation.
func TestScheduleFilePulling_CopiesWhilePulling(t *testing.T) {
	dir := t.TempDir()
	dataFile := filepath.Join(dir, "data.hash")
	if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	pulls := 0
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			pulls++
			body := "version " + strconv.Itoa(pulls)
			mu.Unlock()
			w.Write([]byte(body))
		}))
	defer server.Close()

	updater := NewFileUpdater(server.URL)
	updater.SetDataFile(dataFile)
	updater.SetTempDataDir(t.TempDir())
	updater.SetIsFileWatcherEnabled(false)
	updater.SetDataFilePullEveryMs(10)
	updater.SetRandomization(0)
	updater.SetLoggerEnabled(false)

	stopCh := make(chan *sync.WaitGroup)
	reload := make(chan struct{})
	go updater.ScheduleFilePulling(stopCh, reload)

	for i := 0; i < 5; i++ {
		<-reload
		path, err := updater.GetReloadFilePath()
		if err != nil {
			t.Fatal(err)
		}
		os.Remove(path)
	}

	stopPuller(stopCh, reload)
}

// stopPuller stops a puller started with ScheduleFilePulling. The puller may
// be announcing another pull when it is asked to stop, so reload events are
// read until it has stopped.
func stopPuller(stopCh chan *sync.WaitGroup, reload chan struct{}) {
	stopped := make(chan struct{})
	go func() {
		for {
			select {
			case <-reload:
			case <-stopped:
				return
			}
		}
	}()
	var wg sync.WaitGroup
	wg.Add(1)
	stopCh <- &wg
	wg.Wait()
	close(stopped)
}

// Test that the random time added to a polling interval stays from 0 up to
// and including the maximum, differs between draws, and is 0 when the
// maximum is 0 or less.
func TestRandomizationMs(t *testing.T) {
	const maxMs = 600000
	const draws = 1000
	seen := map[int]bool{}
	for i := 0; i < draws; i++ {
		got := randomizationMs(maxMs)
		if got < 0 || got > maxMs {
			t.Fatalf("randomizationMs(%d) = %d, want 0 to %d",
				maxMs, got, maxMs)
		}
		seen[got] = true
	}
	if len(seen) < draws/2 {
		t.Fatalf("%d draws gave only %d different times", draws, len(seen))
	}

	// With a maximum of 1 both 0 and 1 come up, so the maximum is included.
	ends := map[int]bool{}
	for i := 0; i < draws; i++ {
		ends[randomizationMs(1)] = true
	}
	if !ends[0] || !ends[1] || len(ends) != 2 {
		t.Fatalf("randomizationMs(1) gave %v, want 0 and 1", ends)
	}

	if got := randomizationMs(math.MaxInt); got < 0 {
		t.Fatalf("randomizationMs(math.MaxInt) = %d, want 0 or more", got)
	}

	for _, limit := range []int{0, -1} {
		for i := 0; i < draws; i++ {
			if got := randomizationMs(limit); got != 0 {
				t.Fatalf("randomizationMs(%d) = %d, want 0", limit, got)
			}
		}
	}
}

// Test that two seeds differ, because with a fixed seed every server would
// draw the same times and poll in step.
func TestRandomSeed(t *testing.T) {
	if a, b := randomSeed(), randomSeed(); a == b {
		t.Fatalf("randomSeed() gave %d twice", a)
	}
}

// Test that the scheduler gives the randomization to the draw as the maximum
// and waits only for the time drawn. The stub draws 0, so with a maximum of an
// hour the first pull still comes after the few milliseconds of the interval.
func TestScheduleFilePulling_DrawsRandomization(t *testing.T) {
	const pullEveryMs = 5
	const maxRandomizationMs = 3600000
	const pullTimeoutSeconds = 3

	var mu sync.Mutex
	var given []int
	drawRandomizationMs = func(maxMs int) int {
		mu.Lock()
		defer mu.Unlock()
		given = append(given, maxMs)
		return 0
	}
	t.Cleanup(func() { drawRandomizationMs = randomizationMs })

	dataFile := filepath.Join(t.TempDir(), "data.hash")
	if err := os.WriteFile(dataFile, []byte("current"), 0644); err != nil {
		t.Fatal(err)
	}
	server := createMockServer(http.StatusOK, nil, []byte("new"))
	defer server.Close()

	updater := NewFileUpdater(server.URL)
	updater.SetDataFile(dataFile)
	updater.SetIsFileWatcherEnabled(false)
	updater.SetUpdateOnStartEnabled(false)
	updater.SetDataFilePullEveryMs(pullEveryMs)
	updater.SetRandomization(maxRandomizationMs)
	updater.SetLoggerEnabled(false)

	stopCh := make(chan *sync.WaitGroup)
	reload := make(chan struct{})
	go updater.ScheduleFilePulling(stopCh, reload)
	defer stopPuller(stopCh, reload)

	select {
	case <-reload:
	case <-time.After(pullTimeoutSeconds * time.Second):
		t.Fatalf("no pull within %d seconds with a %d ms interval",
			pullTimeoutSeconds, pullEveryMs)
	}

	mu.Lock()
	got := append([]int(nil), given...)
	mu.Unlock()
	if len(got) == 0 {
		t.Fatal("the scheduler did not draw a random time")
	}
	for _, maxMs := range got {
		if maxMs != maxRandomizationMs {
			t.Fatalf("the draw was given a maximum of %d, want %d",
				maxMs, maxRandomizationMs)
		}
	}
}
