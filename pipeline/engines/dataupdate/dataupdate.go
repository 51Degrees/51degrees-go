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
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md
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
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#update-polling
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
	// If-Modified-Since header so an unchanged file returns 304 rather than
	// being re-downloaded. The date is the publication date from PublishedDate,
	// then the Last-Modified the server last reported, and only then the
	// operational file's modification time. Disable it for a server that
	// mishandles conditional requests.
	UseIfModifiedSince bool
	// PublishedDate, if set, returns the publication date of the data the
	// Engine has loaded, or the zero time when the data has none.
	PublishedDate func() time.Time
	// NextUpdate, if set, returns the date after which the loaded data expects
	// an update to be available, or the zero time when the data gives none.
	// Polling does not start before that date.
	NextUpdate func() time.Time

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

	// OnComplete, if set, is told how each check for an update on this data
	// source ended, the update on start-up included. It is the only way to
	// hear the check made on start-up, because the service's OnComplete can
	// only be called once the data source is registered.
	OnComplete func(Complete)
}

// The settings a data source has when its configuration says nothing else.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#defaults
const (
	// DefaultPollingInterval is the time between checks for an update.
	DefaultPollingInterval = 30 * time.Minute
	// DefaultMaxRandomization is the most that is added at random to each
	// interval, so several instances do not all ask at once.
	DefaultMaxRandomization = 10 * time.Minute
)

// DefaultConfig returns the settings of the Defaults table, which the file
// updater the native Engines use starts with too. Set Identifier,
// DataFilePath and, where the data is downloaded, DataUpdateURL on the result.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#defaults
func DefaultConfig() Config {
	return Config{
		AutomaticUpdatesEnabled:  true,
		FileSystemWatcherEnabled: true,
		UpdateOnStartup:          false,
		UseIfModifiedSince:       true,
		PollingInterval:          DefaultPollingInterval,
		MaxRandomization:         DefaultMaxRandomization,
	}
}

// Status is how a check for an update ended. The names are those of the .NET
// data update service, so a status means the same in every language.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#reporting-each-check
type Status string

const (
	// StatusSuccess is a check that refreshed the Engine with new data.
	StatusSuccess Status = "AUTO_UPDATE_SUCCESS"
	// StatusNotNeeded is a check that applied no update because none was
	// needed, such as a 304 (Not modified) response.
	StatusNotNeeded Status = "AUTO_UPDATE_NOT_NEEDED"
	// StatusHTTPSError is a request to the update URL that failed, or a
	// response that was neither a success nor a 304 (Not modified).
	StatusHTTPSError Status = "AUTO_UPDATE_HTTPS_ERR"
	// StatusTooManyRequests is a 429 (Too many requests) response.
	StatusTooManyRequests Status = "AUTO_UPDATE_ERR_429_TOO_MANY_ATTEMPTS"
	// StatusForbidden is a 403 (Forbidden) response.
	StatusForbidden Status = "AUTO_UPDATE_ERR_403_FORBIDDEN"
	// StatusMD5ValidationFailed is downloaded data whose MD5 hash does not
	// match the one the response gave.
	StatusMD5ValidationFailed Status = "AUTO_UPDATE_ERR_MD5_VALIDATION_FAILED"
	// StatusCannotRename is downloaded data that could not replace the
	// operational data file.
	StatusCannotRename Status = "AUTO_UPDATE_NEW_FILE_CANT_RENAME"
	// StatusRefreshFailed is new data the Engine would not load.
	StatusRefreshFailed Status = "AUTO_UPDATE_REFRESH_FAILED"
	// StatusUnknownError is a failure none of the others name.
	StatusUnknownError Status = "AUTO_UPDATE_UNKNOWN_ERROR"
)

// Complete is the report a check makes once it ends.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#reporting-each-check
type Complete struct {
	// Identifier names the data source the check was for.
	Identifier string
	// Status is how the check ended.
	Status Status
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

	updateMu   sync.Mutex // serializes a whole HTTP update (download + replace + refresh)
	refreshMu  sync.Mutex // serializes refresh; keeps refresh thread-safe
	callbackMu sync.Mutex // guards onComplete
	watcher    *fsnotify.Watcher
	stop       chan struct{}
	wg         sync.WaitGroup

	// lastModified is the server's Last-Modified for the current data; used for
	// the next conditional request. Accessed only under updateMu (httpUpdate).
	lastModified time.Time

	// loadedFileTime is the modification time of the data file the Engine
	// was last refreshed from, which a watched file has to be newer than to
	// be loaded. Guarded by refreshMu.
	loadedFileTime time.Time

	onComplete []func(Complete)
}

// recoverGoroutine logs and swallows a panic in a background goroutine so a
// corrupt data file or a buggy update callback cannot crash the host process.
func (r *registration) recoverGoroutine(what string) {
	if rec := recover(); rec != nil {
		r.svc.logger.Error(r.unhandledMessage(), "panic", rec, "in", what)
	}
}

// Register adds an Engine to the service and runs the configured start-up
// process (update-on-start-up, file watcher, polling). engineName is used in
// log messages.
//
// When UpdateOnStartup is set, Register performs the start-up update check
// synchronously and so blocks for up to the HTTP client's timeout; the default
// client sets a bounded timeout, and a custom client should do the same. If
// that update fails, Register falls back to the local data file, and returns
// an error when there is no local data file.
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
	if cfg.OnComplete != nil {
		reg.onComplete = append(reg.onComplete, cfg.OnComplete)
	}

	if cfg.UpdateOnStartup && cfg.DataUpdateURL != "" {
		s.logger.Info("Updating on start-up")
		status, err := reg.httpUpdate()
		// The update on start-up is a check, so it is reported like any other,
		// whether or not start-up carries on, and an error a callback raises
		// reaches the caller rather than the log.
		reported := reg.report(status)
		if err != nil {
			// A failed start-up update falls back to the local data file, and
			// fails when there is none.
			if cfg.DataFilePath != "" {
				if _, statErr := os.Stat(cfg.DataFilePath); statErr != nil {
					return errors.Join(fmt.Errorf("data update: start-up update failed and "+
						"there is no local data file at %q: %w", cfg.DataFilePath, err), reported)
				}
			}
			// The caller does not get this error, as the start-up carries on
			// with the local data file, so it is logged.
			reg.logCheckFailure(err)
		}
		if reported != nil {
			return reported
		}
	}

	// Start the background goroutines (which call wg.Add and assign the watcher)
	// before publishing the registration, so a concurrent Close cannot observe a
	// half-initialized registration and race wg.Add/Wait or the watcher field.
	//
	// AutomaticUpdatesEnabled is the master switch for every automatic update
	// mechanism, so the watcher runs only when it is on, as polling does.
	if cfg.AutomaticUpdatesEnabled && cfg.FileSystemWatcherEnabled {
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

// OnComplete registers a callback that every check for an update on the named
// data source reports to once the check ends, with the status it ended with.
// An update from memory gives its result to its caller alone and reports
// nothing here.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#reporting-each-check
func (s *Service) OnComplete(identifier string, fn func(Complete)) {
	s.mu.Lock()
	reg, ok := s.registrations[identifier]
	s.mu.Unlock()
	if !ok {
		return
	}
	reg.callbackMu.Lock()
	reg.onComplete = append(reg.onComplete, fn)
	reg.callbackMu.Unlock()
}

// OnUpdate registers a callback that is called after a check that refreshed
// the Engine of the named data source with new data. It hears no other check
// and is told no status, which OnComplete gives.
func (s *Service) OnUpdate(identifier string, fn func()) {
	s.OnComplete(identifier, func(complete Complete) {
		if complete.Status == StatusSuccess {
			fn()
		}
	})
}

// CheckForUpdate triggers an immediate update for the named data source. If HTTP
// polling is configured it polls the remote server; otherwise it refreshes from
// the configured file if it is newer. The check is reported to the callbacks
// and its error is passed to the caller rather than logged.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#programmatic-update
func (s *Service) CheckForUpdate(identifier string) error {
	s.mu.Lock()
	reg, ok := s.registrations[identifier]
	s.mu.Unlock()
	if !ok {
		// A data file the service has no configuration for is the caller's to
		// deal with, and nothing is reported.
		return fmt.Errorf("data update: no registration for identifier %q", identifier)
	}
	status := StatusSuccess
	var err error
	if reg.cfg.DataUpdateURL != "" {
		status, err = reg.httpUpdate()
	} else if err = reg.refreshFromFile(); err != nil {
		status = StatusRefreshFailed
	}
	reported := reg.report(status)
	if err != nil {
		return err
	}
	return reported
}

// UpdateFromMemory refreshes the named data source's Engine from data held in
// memory, with no download. It is the programmatic update for a memory data
// source.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#programmatic-update
func (s *Service) UpdateFromMemory(identifier string, data []byte) error {
	s.mu.Lock()
	reg, ok := s.registrations[identifier]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("data update: no registration for identifier %q", identifier)
	}
	if len(data) == 0 {
		return errors.New("data update: empty in-memory data")
	}
	return reg.refreshFromBytes(data)
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
		first := true
		for {
			delay := r.nextInterval()
			if first {
				delay = r.firstDelay(delay)
				first = false
			}
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
			status, err := r.httpUpdate()
			if err != nil {
				// A check the poller starts is logged here, and is never fatal.
				r.logCheckFailure(err)
				var rle *RateLimitError
				if errors.As(err, &rle) && rle.RetryAfter > 0 {
					// Reset polling to respect the server's rate limit.
					override = rle.RetryAfter
				}
			}
			if err := r.report(status); err != nil {
				r.logCheckFailure(err)
			}
		}
	}()
}

func (r *registration) nextInterval() time.Duration {
	d := r.cfg.PollingInterval
	if d <= 0 {
		d = DefaultPollingInterval
	}
	return d + r.randomExtra()
}

// randomExtra returns a random time below the configured maximum, or zero when
// there is no maximum.
func (r *registration) randomExtra() time.Duration {
	if r.cfg.MaxRandomization <= 0 {
		return 0
	}
	r.svc.mu.Lock()
	defer r.svc.mu.Unlock()
	return time.Duration(r.svc.rand.Int63n(int64(r.cfg.MaxRandomization)))
}

// firstDelay returns the wait before the first poll. When the loaded data
// expects an update after a later date, polling does not start until then, and
// the random time is added to the wait.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#automatic-update-via-http
func (r *registration) firstDelay(regular time.Duration) time.Duration {
	if r.cfg.NextUpdate == nil {
		return regular
	}
	next := r.cfg.NextUpdate()
	if !validDate(next) {
		return regular
	}
	until := time.Until(next)
	if until <= 0 {
		return regular
	}
	return until + r.randomExtra()
}

// earliestDate is the first year taken as a real date. Data that carries no
// date reads as year 0.
const earliestDate = 1970

// validDate reports whether a date read from a data file is set.
func validDate(t time.Time) bool {
	return !t.IsZero() && t.Year() >= earliestDate
}

func (r *registration) startWatcher() error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	r.svc.logger.Info("Creating file system watcher")
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
				status := StatusSuccess
				if loaded, err := r.refreshFromNewerFile(); err != nil {
					status = StatusRefreshFailed
					r.logCheckFailure(err)
				} else if !loaded {
					status = StatusNotNeeded
				}
				if err := r.report(status); err != nil {
					r.logCheckFailure(err)
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

// How often a refresh from the data file is tried, as .NET and Java do,
// because the file can be held by another process for a moment.
const (
	// refreshTries is how many times the refresh is tried before it is
	// reported as failed.
	refreshTries = 10
	// refreshRetryWaitMilliseconds is the wait between tries.
	refreshRetryWaitMilliseconds = 200
)

// refreshFromFile reloads the Engine from its configured data file, trying up
// to refreshTries times. The watcher and polling are implicitly paused for the
// duration because refresh is serialized by refreshMu.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#errors
func (r *registration) refreshFromFile() error {
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	return r.loadDataFile()
}

// refreshFromNewerFile reloads the Engine from its data file when the file is
// newer than the one last loaded, and says whether it was loaded. A watched
// file that is not newer is left alone, as .NET leaves it.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#newfileavailable-via-remote-or-file-watcher
func (r *registration) refreshFromNewerFile() (bool, error) {
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	if modified, ok := r.currentDataTime(); ok && !modified.After(r.loadedFileTime) {
		return false, nil
	}
	return true, r.loadDataFile()
}

// loadDataFile makes the tries of a refresh from the data file. It records the
// file's modification time first, so a file that would not load is not tried
// again by the watcher and one written later is. The caller holds refreshMu.
func (r *registration) loadDataFile() error {
	if modified, ok := r.currentDataTime(); ok {
		r.loadedFileTime = modified
	}
	r.svc.logger.Info(fmt.Sprintf("Attempting to refresh Engine '%s' with new data", r.engineName))
	var err error
	for tries := 1; tries <= refreshTries; tries++ {
		if err = r.engine.RefreshFromFile(r.cfg.DataFilePath); err == nil {
			return nil
		}
		if tries < refreshTries {
			time.Sleep(refreshRetryWaitMilliseconds * time.Millisecond)
		}
	}
	return &CheckError{fmt.Sprintf("An error occurred when applying a data update to engine '%s' "+
		"after %d tries.", r.engineName, refreshTries), err}
}

// httpUpdate checks the remote server for a newer data file and, if found,
// downloads it, replaces the operational file, and refreshes the Engine. A 304
// response means no newer data; a 429 response is reported with its Retry-After
// value.
//
// The whole operation (download, replace, refresh) is serialized per
// registration so a programmatic CheckForUpdate cannot race the background
// poller and load an older of two concurrent downloads.
//
// A failure the error table has a row for is returned as a CheckError with
// the row's message, and with the status of that row. Nothing is logged here
// for a failure, so a check the caller starts passes its error to the caller,
// and the poller logs its own.
func (r *registration) httpUpdate() (Status, error) {
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
	r.svc.logger.Info("Checking for update")
	r.svc.logger.Info(fmt.Sprintf("Checking for update from '%s' for Engine '%s'", url, r.engineName))

	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return StatusHTTPSError, core.MaskLicenseKeysInError(err, keys)
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
		return StatusHTTPSError, &CheckError{fmt.Sprintf(
			"Error accessing data update service at '%s' for engine '%s'",
			url, r.engineName), core.MaskLicenseKeysInError(err, keys)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		r.svc.logger.Info(fmt.Sprintf("No data newer than %s found at '%s' for Engine '%s'",
			r.sinceText(), url, r.engineName))
		return StatusNotNeeded, nil
	case http.StatusTooManyRequests:
		retry := resp.Header.Get("Retry-After")
		return StatusTooManyRequests, &RateLimitError{URL: url, Engine: r.engineName,
			RetryAfter: parseRetryAfter(retry)}
	case http.StatusForbidden:
		return StatusForbidden, &CheckError{fmt.Sprintf(
			"Access denied to data update service at '%s' for engine '%s'", url, r.engineName), nil}
	case http.StatusOK:
		// proceed
	default:
		return StatusHTTPSError, &CheckError{fmt.Sprintf(
			"HTTP status code '%d' from data update service at '%s' for engine '%s'",
			resp.StatusCode, url, r.engineName), nil}
	}

	raw, err := readBody(resp)
	if err != nil {
		return StatusUnknownError, err
	}
	// Validate integrity against the server's Content-MD5 (the 51Degrees
	// Distributor sends it) before the bytes are written or loaded, so a corrupt
	// or truncated download is rejected rather than handed to the Engine.
	if resp.Header.Get("Content-MD5") == "" {
		r.svc.logger.Warn(fmt.Sprintf("No MD5 hash included in data update response for '%s'. "+
			"Unable to verify data integrity", url))
	} else if err := verifyContentMD5(resp, raw, r.engineName); err != nil {
		return StatusMD5ValidationFailed, err
	}
	body, err := maybeDecompress(raw)
	if err != nil {
		return StatusUnknownError, err
	}

	if r.cfg.DataFilePath != "" {
		if status, err := r.writeDataFile(body); err != nil {
			return status, err
		}
		if err := r.refreshFromFile(); err != nil {
			return StatusRefreshFailed, err
		}
	} else if err := r.refreshFromBytes(body); err != nil {
		return StatusRefreshFailed, err
	}

	// Persist the server's Last-Modified so the next conditional request is
	// based on the data's real timestamp rather than the local file's mtime.
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if t, perr := http.ParseTime(lm); perr == nil {
			r.lastModified = t
		}
	}
	r.svc.logger.Info(fmt.Sprintf("Downloaded new data from '%s' for Engine '%s'", url, r.engineName))
	return StatusSuccess, nil
}

// conditionalSince returns the timestamp to use for If-Modified-Since: the
// publication date of the loaded data if it has one, then the server's last
// reported Last-Modified if known, and only then the operational file's
// modification time.
func (r *registration) conditionalSince() (time.Time, bool) {
	if r.cfg.PublishedDate != nil {
		if published := r.cfg.PublishedDate(); validDate(published) {
			return published, true
		}
	}
	if !r.lastModified.IsZero() {
		return r.lastModified, true
	}
	return r.currentDataTime()
}

// refreshFromBytes reloads the Engine from an in-memory buffer, serialized by
// refreshMu. The refresh is made once, as the error table's row for data held
// in memory names no tries.
func (r *registration) refreshFromBytes(body []byte) error {
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	r.svc.logger.Info(fmt.Sprintf("Attempting to refresh Engine '%s' with new data", r.engineName))
	if err := r.engine.RefreshFromBytes(body); err != nil {
		return &CheckError{fmt.Sprintf("An error occurred when applying a data update to engine '%s'.",
			r.engineName), err}
	}
	return nil
}

// writeDataFile writes body to a temporary file and atomically renames it over
// the operational data file, so the Engine never sees a partial file.
func (r *registration) writeDataFile(body []byte) (Status, error) {
	dir := r.cfg.TempDir
	if dir == "" {
		dir = filepath.Dir(r.cfg.DataFilePath)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(r.cfg.DataFilePath)+".tmp-*")
	if err != nil {
		return StatusUnknownError, err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return StatusUnknownError, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return StatusUnknownError, err
	}
	if err := os.Rename(tmpName, r.cfg.DataFilePath); err != nil {
		os.Remove(tmpName)
		return StatusCannotRename, &CheckError{fmt.Sprintf(
			"An error occurred when copying a data file to replace the existing one at '%s'.",
			r.cfg.DataFilePath), err}
	}
	return StatusSuccess, nil
}

// sinceText writes the date a check asked for newer data than.
func (r *registration) sinceText() string {
	if since, ok := r.conditionalSince(); ok && r.cfg.UseIfModifiedSince {
		return since.UTC().Format(time.RFC3339)
	}
	return "the current data"
}

// CheckError is a failed check that the error table of the specification has a
// row for. Its message is the row's, which follows .NET, with the cause after
// it where there is one.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#errors
type CheckError struct {
	// Message is the error table's message for the failure.
	Message string
	// Cause is the error that led to the failure, or nil.
	Cause error
}

// Error returns the message, and the cause after it as the error detail.
func (e *CheckError) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	if strings.HasSuffix(e.Message, ".") {
		return e.Message + " Error detail: " + e.Cause.Error()
	}
	return e.Message + ". Error detail: " + e.Cause.Error()
}

// Unwrap returns the cause.
func (e *CheckError) Unwrap() error { return e.Cause }

// logCheckFailure logs a failed check the service started itself. A failure
// the error table has a row for is logged at Error as "Error during check for
// data update" with the error attached, and any other at Error with the
// table's last row.
func (r *registration) logCheckFailure(err error) {
	var ce *CheckError
	var rle *RateLimitError
	if errors.As(err, &ce) || errors.As(err, &rle) {
		r.svc.logger.Error("Error during check for data update", "error", err)
		return
	}
	r.svc.logger.Error(r.unhandledMessage(), "error", err)
}

// unhandledMessage is the error table's last row, for a failure it has no row
// of its own for.
func (r *registration) unhandledMessage() string {
	return fmt.Sprintf("An unhandled error occurred while checking for automatic updates for engine '%s'",
		r.engineName)
}

func (r *registration) currentDataTime() (time.Time, bool) {
	info, err := os.Stat(r.cfg.DataFilePath)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}

// report tells each callback that a check has ended with the status given. A
// callback that panics stops the callbacks after it, and its panic is returned
// for the caller to log or to pass on, as the error table's last row says.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#reporting-each-check
func (r *registration) report(status Status) error {
	r.callbackMu.Lock()
	callbacks := r.onComplete
	r.callbackMu.Unlock()
	complete := Complete{Identifier: r.cfg.Identifier, Status: status}
	for _, fn := range callbacks {
		if err := r.callback(fn, complete); err != nil {
			return err
		}
	}
	return nil
}

// callback calls fn, returning the panic it raised, if any.
func (r *registration) callback(fn func(Complete), complete Complete) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = &callbackPanic{rec}
		}
	}()
	fn(complete)
	return nil
}

// callbackPanic is a panic an update callback raised, as an error. The error
// table has no row of its own for it, so logCheckFailure logs it with the
// last row's message.
type callbackPanic struct{ value any }

func (p *callbackPanic) Error() string {
	return fmt.Sprintf("panic in an update callback: %v", p.value)
}

// RateLimitError is returned when the update server responds with HTTP 429.
type RateLimitError struct {
	// URL is the update URL that returned the rate-limit response, with each
	// license key in it masked.
	URL string
	// Engine names the Engine the check was for.
	Engine string
	// RetryAfter is how long to wait before retrying, taken from the
	// Retry-After response header.
	RetryAfter time.Duration
}

// Error returns the error table's message for a 429 response.
func (e *RateLimitError) Error() string {
	return fmt.Sprintf("Too many requests to '%s' for engine '%s'", e.URL, e.Engine)
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
func verifyContentMD5(resp *http.Response, raw []byte, engineName string) error {
	want := resp.Header.Get("Content-MD5")
	if want == "" {
		return nil
	}
	sum := md5.Sum(raw)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		// The table, as .NET, has no space after the engine's name.
		return &CheckError{fmt.Sprintf("Integrity check failed. MD5 hash in HTTP response '%s' for "+
			"'%s'data update does not match calculated hash for the downloaded file '%s'.",
			want, engineName, got), nil}
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
