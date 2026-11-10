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

// Package dataupdate implements the Data Update Service, which keeps an
// On-Premise Aspect Engine's data file current by downloading updates over HTTP
// and/or watching the file system for replacements.
//
// All update work happens on background goroutines; per the specification,
// errors there are logged and never crash the process, and refresh is performed
// in a thread-safe manner so the Engine can keep processing during an update.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/data-updates.md
package dataupdate

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/51Degrees/51degrees-go/pipeline/v4/core"
	"github.com/fsnotify/fsnotify"
)

// Refreshable is implemented by Engines whose data can be reloaded. The Data
// Update Service calls these methods when new data becomes available. Refresh
// MUST be thread-safe; the service serializes calls per registration to help.
type Refreshable interface {
	// RefreshFromFile reloads the Engine's data from the file at path.
	RefreshFromFile(path string) error
	// RefreshFromBytes reloads the Engine's data from an in-memory buffer.
	RefreshFromBytes(data []byte) error
}

// URLFormatter customizes the data update URL, for example by appending query
// parameters required by the 51Degrees Distributor.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/data-updates.md#update-polling
type URLFormatter func(cfg Config) string

// Config describes a single data source and how it should be updated. It maps
// onto the configuration groups in the data-updates specification.
type Config struct {
	// Identifier names this data source (used for programmatic updates and logs).
	Identifier string
	// DataFilePath is the operational data file location.
	DataFilePath string

	// HTTP config.

	// DataUpdateURL is the URL to check for and download data file updates from.
	DataUpdateURL string
	// URLFormatter, if set, builds the final update URL (for example by adding
	// the query-string parameters required by the 51Degrees Distributor).
	URLFormatter URLFormatter
	// UseIfModifiedSince controls whether the update request sends an
	// If-Modified-Since header (based on the server's last reported
	// Last-Modified, falling back to the operational file's modification time)
	// so an unchanged file returns 304 rather than being re-downloaded. Disable
	// it for a server that mishandles conditional requests.
	UseIfModifiedSince bool

	// Update behavior.

	// UpdateOnStartup indicates whether an update check should be performed as
	// soon as the data source is registered, before normal polling begins.
	UpdateOnStartup bool
	// AutomaticUpdatesEnabled indicates whether updates to the data file are
	// checked for and applied to the Engine automatically.
	AutomaticUpdatesEnabled bool
	// FileSystemWatcherEnabled indicates whether a watcher is created for the
	// file at DataFilePath. When the file is modified the Engine is notified so
	// it can refresh its internal data structures.
	FileSystemWatcherEnabled bool

	// Polling config.

	// PollingInterval is the time between checks for updates for this data source.
	PollingInterval time.Duration
	// MaxRandomization is the maximum extra random time added to the polling
	// interval. It lets multiple instances stagger their update requests to
	// reduce the chance of conflicts or slow downloads.
	MaxRandomization time.Duration

	// Operational file config.
	TempDir string
}

// Service manages data updates for one or more registered Engines.
type Service struct {
	logger     *slog.Logger
	httpClient *http.Client

	mu            sync.Mutex
	registrations map[string]*registration
	rand          *rand.Rand
}

// New creates a Data Update Service. If logger is nil, slog.Default is used; if
// client is nil, a client with a sensible timeout is used.
func New(logger *slog.Logger, client *http.Client) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Service{
		logger:        logger,
		httpClient:    client,
		registrations: make(map[string]*registration),
		rand:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

type registration struct {
	svc        *Service
	engine     Refreshable
	engineName string
	cfg        Config

	updateMu  sync.Mutex // serializes a whole HTTP update (download + replace + refresh)
	refreshMu sync.Mutex // serializes refresh; keeps refresh thread-safe
	watcher   *fsnotify.Watcher
	stop      chan struct{}
	wg        sync.WaitGroup

	// lastModified is the server's Last-Modified for the current data; used for
	// the next conditional request. Accessed only under updateMu (httpUpdate).
	lastModified time.Time

	onUpdate []func()
}

// recoverGoroutine logs and swallows a panic in a background goroutine so a
// corrupt data file or a buggy update callback cannot crash the host process.
func (r *registration) recoverGoroutine(what string) {
	if rec := recover(); rec != nil {
		r.svc.logger.Error("recovered panic in "+what, "engine", r.engineName, "panic", rec)
	}
}

// Register adds an Engine to the service and runs the configured start-up
// process (update-on-start-up, file watcher, polling). engineName is used in
// log messages.
//
// When UpdateOnStartup is set, Register performs the start-up update check
// synchronously and so blocks for up to the HTTP client's timeout; the default
// client sets a bounded timeout, and a custom client should do the same.
func (s *Service) Register(engine Refreshable, engineName string, cfg Config) error {
	if cfg.Identifier == "" {
		cfg.Identifier = engineName
	}
	reg := &registration{
		svc:        s,
		engine:     engine,
		engineName: engineName,
		cfg:        cfg,
		stop:       make(chan struct{}),
	}

	if cfg.UpdateOnStartup && cfg.DataUpdateURL != "" {
		s.logger.Info("Updating on start-up", "engine", engineName)
		if err := reg.httpUpdate(); err != nil {
			// Start-up update failures fall back to existing local data.
			s.logger.Warn("start-up update failed; using existing data if available",
				"engine", engineName, "error", err)
		}
	}

	// Start the background goroutines (which call wg.Add and assign the watcher)
	// before publishing the registration, so a concurrent Close cannot observe a
	// half-initialized registration and race wg.Add/Wait or the watcher field.
	if cfg.FileSystemWatcherEnabled {
		if err := reg.startWatcher(); err != nil {
			s.logger.Warn("could not create file system watcher", "engine", engineName, "error", err)
		}
	}
	if cfg.AutomaticUpdatesEnabled && cfg.DataUpdateURL != "" {
		reg.startPolling()
	}

	s.mu.Lock()
	if _, exists := s.registrations[cfg.Identifier]; exists {
		s.mu.Unlock()
		// Roll back the goroutines started above for this duplicate.
		reg.stopAndWait()
		return fmt.Errorf("data update: identifier %q is already registered", cfg.Identifier)
	}
	s.registrations[cfg.Identifier] = reg
	s.mu.Unlock()
	return nil
}

// stopAndWait signals the registration's goroutines to stop and waits for them,
// closing the watcher if present.
func (r *registration) stopAndWait() {
	close(r.stop)
	if r.watcher != nil {
		_ = r.watcher.Close()
	}
	r.wg.Wait()
}

// OnUpdate registers a callback invoked after a successful refresh of the named
// data source.
func (s *Service) OnUpdate(identifier string, fn func()) {
	s.mu.Lock()
	reg, ok := s.registrations[identifier]
	s.mu.Unlock()
	if !ok {
		return
	}
	// onUpdate is read by fireCallbacks under refreshMu, so append under the same
	// lock to avoid a data race with a concurrent refresh.
	reg.refreshMu.Lock()
	reg.onUpdate = append(reg.onUpdate, fn)
	reg.refreshMu.Unlock()
}

// CheckForUpdate triggers an immediate update for the named data source. If HTTP
// polling is configured it polls the remote server; otherwise it refreshes from
// the configured file if it is newer.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/data-updates.md#programmatic-update
func (s *Service) CheckForUpdate(identifier string) error {
	s.mu.Lock()
	reg, ok := s.registrations[identifier]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("data update: no registration for identifier %q", identifier)
	}
	if reg.cfg.DataUpdateURL != "" {
		return reg.httpUpdate()
	}
	return reg.refreshFromFile()
}

// Close stops all background activity (watchers and polling).
func (s *Service) Close() error {
	s.mu.Lock()
	regs := make([]*registration, 0, len(s.registrations))
	for _, r := range s.registrations {
		regs = append(regs, r)
	}
	s.registrations = make(map[string]*registration)
	s.mu.Unlock()

	for _, r := range regs {
		r.stopAndWait()
	}
	return nil
}

func (r *registration) startPolling() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.recoverGoroutine("update poller")
		// override, when non-zero, forces the next poll delay (used to honor a
		// server's Retry-After after a 429 response).
		var override time.Duration
		for {
			delay := r.nextInterval()
			if override > 0 {
				delay = override
				override = 0
			}
			timer := time.NewTimer(delay)
			select {
			case <-r.stop:
				timer.Stop()
				return
			case <-timer.C:
			}
			if err := r.httpUpdate(); err != nil {
				// Background errors are logged, never fatal.
				r.svc.logger.Warn("automatic update failed; will retry later",
					"engine", r.engineName, "error", err)
				var rle *RateLimitError
				if errors.As(err, &rle) && rle.RetryAfter > 0 {
					// Reset polling to respect the server's rate limit.
					override = rle.RetryAfter
				}
			}
		}
	}()
}

func (r *registration) nextInterval() time.Duration {
	d := r.cfg.PollingInterval
	if d <= 0 {
		d = 30 * time.Minute
	}
	if r.cfg.MaxRandomization > 0 {
		r.svc.mu.Lock()
		extra := time.Duration(r.svc.rand.Int63n(int64(r.cfg.MaxRandomization)))
		r.svc.mu.Unlock()
		d += extra
	}
	return d
}

func (r *registration) startWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	r.svc.logger.Info("Creating file system watcher", "engine", r.engineName)
	dir := filepath.Dir(r.cfg.DataFilePath)
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		return err
	}
	r.watcher = w
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.recoverGoroutine("file watcher")
		target := filepath.Clean(r.cfg.DataFilePath)
		// A single logical change (an atomic rename, or an editor writing
		// incrementally) surfaces as several events; debounce so the Engine
		// reloads once per burst rather than on every event.
		const debounce = 500 * time.Millisecond
		var timer *time.Timer
		var timerC <-chan time.Time
		defer func() {
			if timer != nil {
				timer.Stop()
			}
		}()
		for {
			select {
			case <-r.stop:
				return
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) != target {
					continue
				}
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					if timer == nil {
						timer = time.NewTimer(debounce)
					} else {
						timer.Reset(debounce)
					}
					timerC = timer.C
				}
			case <-timerC:
				timerC = nil
				if err := r.refreshFromFile(); err != nil {
					r.svc.logger.Warn("refresh from watched file failed",
						"engine", r.engineName, "error", err)
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				r.svc.logger.Warn("file system watcher error", "engine", r.engineName, "error", err)
			}
		}
	}()
	return nil
}

// refreshFromFile reloads the Engine from its configured data file. The watcher
// and polling are implicitly paused for the duration because refresh is
// serialized by refreshMu.
func (r *registration) refreshFromFile() error {
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	r.svc.logger.Info("Attempting to refresh Engine with new data", "engine", r.engineName)
	if err := r.engine.RefreshFromFile(r.cfg.DataFilePath); err != nil {
		r.svc.logger.Error("error applying data file update", "engine", r.engineName, "error", err)
		return err
	}
	r.fireCallbacks()
	return nil
}

// httpUpdate checks the remote server for a newer data file and, if found,
// downloads it, replaces the operational file, and refreshes the Engine. A 304
// response means no newer data; a 429 response is reported with its Retry-After
// value.
//
// The whole operation (download, replace, refresh) is serialized per
// registration so a programmatic CheckForUpdate cannot race the background
// poller and load an older of two concurrent downloads.
func (r *registration) httpUpdate() error {
	r.updateMu.Lock()
	defer r.updateMu.Unlock()

	target := r.cfg.DataUpdateURL
	if r.cfg.URLFormatter != nil {
		target = r.cfg.URLFormatter(r.cfg)
	}
	// url names the request in log entries and errors, with each license key
	// in it masked.
	keys := core.LicenseKeysInURL(target)
	url := core.MaskLicenseKeysIn(target, keys)
	r.svc.logger.Info("Checking for update", "engine", r.engineName, "url", url)

	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return core.MaskLicenseKeysInError(err, keys)
	}
	// When enabled, send If-Modified-Since so a server that honors it returns
	// 304 instead of re-sending an unchanged file. Prefer the Last-Modified the
	// server previously reported (persisted below) over the operational file's
	// modification time, which after an atomic rename is the download time and
	// so could be at or after data that is actually newer.
	if r.cfg.UseIfModifiedSince {
		if since, ok := r.conditionalSince(); ok {
			req.Header.Set("If-Modified-Since", since.UTC().Format(http.TimeFormat))
		}
	}

	resp, err := r.svc.httpClient.Do(req)
	if err != nil {
		err = core.MaskLicenseKeysInError(err, keys)
		r.svc.logger.Warn("error connecting to update server", "engine", r.engineName, "url", url, "error", err)
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		r.svc.logger.Info("No newer data found", "engine", r.engineName, "url", url)
		return nil
	case http.StatusTooManyRequests:
		retry := resp.Header.Get("Retry-After")
		return &RateLimitError{URL: url, RetryAfter: parseRetryAfter(retry)}
	case http.StatusOK:
		// proceed
	default:
		return fmt.Errorf("data update: unexpected status %d from %s", resp.StatusCode, url)
	}

	raw, err := readBody(resp)
	if err != nil {
		r.svc.logger.Warn("error downloading data file", "engine", r.engineName, "url", url, "error", err)
		return err
	}
	// Validate integrity against the server's Content-MD5 (the 51Degrees
	// Distributor sends it) before the bytes are written or loaded, so a corrupt
	// or truncated download is rejected rather than handed to the Engine.
	if err := verifyContentMD5(resp, raw); err != nil {
		r.svc.logger.Warn("data file integrity check failed", "engine", r.engineName, "url", url, "error", err)
		return err
	}
	body, err := maybeDecompress(raw)
	if err != nil {
		r.svc.logger.Warn("error decompressing data file", "engine", r.engineName, "url", url, "error", err)
		return err
	}

	if r.cfg.DataFilePath != "" {
		if err := r.writeDataFile(body); err != nil {
			r.svc.logger.Error("error writing data file", "engine", r.engineName, "path", r.cfg.DataFilePath, "error", err)
			return err
		}
		if err := r.refreshFromFile(); err != nil {
			return err
		}
	} else if err := r.refreshFromBytes(body); err != nil {
		return err
	}

	// Persist the server's Last-Modified so the next conditional request is
	// based on the data's real timestamp rather than the local file's mtime.
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if t, perr := http.ParseTime(lm); perr == nil {
			r.lastModified = t
		}
	}
	r.svc.logger.Info("Downloaded new data", "engine", r.engineName, "url", url)
	return nil
}

// conditionalSince returns the timestamp to use for If-Modified-Since: the
// server's last reported Last-Modified if known, otherwise the operational
// file's modification time.
func (r *registration) conditionalSince() (time.Time, bool) {
	if !r.lastModified.IsZero() {
		return r.lastModified, true
	}
	return r.currentDataTime()
}

// refreshFromBytes reloads the Engine from an in-memory buffer, serialized by
// refreshMu, and fires the update callbacks on success.
func (r *registration) refreshFromBytes(body []byte) error {
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	if err := r.engine.RefreshFromBytes(body); err != nil {
		r.svc.logger.Error("error applying in-memory data update", "engine", r.engineName, "error", err)
		return err
	}
	r.fireCallbacks()
	return nil
}

// writeDataFile writes body to a temporary file and atomically renames it over
// the operational data file, so the Engine never sees a partial file.
func (r *registration) writeDataFile(body []byte) error {
	dir := r.cfg.TempDir
	if dir == "" {
		dir = filepath.Dir(r.cfg.DataFilePath)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(r.cfg.DataFilePath)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, r.cfg.DataFilePath); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func (r *registration) currentDataTime() (time.Time, bool) {
	info, err := os.Stat(r.cfg.DataFilePath)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

func (r *registration) fireCallbacks() {
	for _, fn := range r.onUpdate {
		// Isolate each callback: a panic in user code is logged and the
		// remaining callbacks still run, rather than unwinding the caller.
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					r.svc.logger.Error("recovered panic in update callback",
						"engine", r.engineName, "panic", rec)
				}
			}()
			fn()
		}()
	}
}

// RateLimitError is returned when the update server responds with HTTP 429.
type RateLimitError struct {
	// URL is the update URL that returned the rate-limit response, with each
	// license key in it masked.
	URL string
	// RetryAfter is how long to wait before retrying, taken from the
	// Retry-After response header.
	RetryAfter time.Duration
}

// Error returns a message naming the rate-limited URL and the retry delay.
func (e *RateLimitError) Error() string {
	return fmt.Sprintf("data update: rate limited by %s, retry after %s", e.URL, e.RetryAfter)
}

func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// gzipMagic is the two-byte signature at the start of every gzip stream.
var gzipMagic = []byte{0x1f, 0x8b}

// readBody reads the entire response body, rejecting an empty payload.
func readBody(resp *http.Response) ([]byte, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("data update: empty response body")
	}
	return data, nil
}

// verifyContentMD5 checks the downloaded bytes against the server's Content-MD5
// header when one is present. The 51Degrees Distributor sends this header, so
// validating it guards against a corrupt or truncated download being loaded into
// the Engine. A missing header is not an error: not every server supplies one.
func verifyContentMD5(resp *http.Response, raw []byte) error {
	want := resp.Header.Get("Content-MD5")
	if want == "" {
		return nil
	}
	sum := md5.Sum(raw)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("data update: Content-MD5 mismatch (got %s, want %s)", got, want)
	}
	return nil
}

// maybeDecompress gunzips raw when it is a gzip stream, returning it unchanged
// otherwise. Gzip is detected from the leading magic bytes rather than the
// Content-Encoding header: the Go HTTP transport transparently decompresses and
// strips a gzip Content-Encoding, so by the time the body is read that header is
// never present, whereas the Distributor's payload still carries the gzip magic.
func maybeDecompress(raw []byte) ([]byte, error) {
	if !bytes.HasPrefix(raw, gzipMagic) {
		return raw, nil
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("data update: empty response body after decompression")
	}
	return data, nil
}
