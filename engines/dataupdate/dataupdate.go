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
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

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
	DataUpdateURL      string
	URLFormatter       URLFormatter
	UseIfModifiedSince bool

	// Update behavior.
	UpdateOnStartup          bool
	AutomaticUpdatesEnabled  bool
	FileSystemWatcherEnabled bool

	// Polling config.
	PollingInterval  time.Duration
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

	refreshMu sync.Mutex // serializes refresh; keeps refresh thread-safe
	watcher   *fsnotify.Watcher
	stop      chan struct{}
	wg        sync.WaitGroup

	onUpdate []func()
}

// Register adds an Engine to the service and runs the configured start-up
// process (update-on-start-up, file watcher, polling). engineName is used in
// log messages.
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

	s.mu.Lock()
	if _, exists := s.registrations[cfg.Identifier]; exists {
		s.mu.Unlock()
		return fmt.Errorf("data update: identifier %q is already registered", cfg.Identifier)
	}
	s.registrations[cfg.Identifier] = reg
	s.mu.Unlock()

	if cfg.UpdateOnStartup && cfg.DataUpdateURL != "" {
		s.logger.Info("Updating on start-up", "engine", engineName)
		if err := reg.httpUpdate(true); err != nil {
			// Start-up update failures fall back to existing local data.
			s.logger.Warn("start-up update failed; using existing data if available",
				"engine", engineName, "error", err)
		}
	}

	if cfg.FileSystemWatcherEnabled {
		if err := reg.startWatcher(); err != nil {
			s.logger.Warn("could not create file system watcher", "engine", engineName, "error", err)
		}
	}
	if cfg.AutomaticUpdatesEnabled && cfg.DataUpdateURL != "" {
		reg.startPolling()
	}
	return nil
}

// OnUpdate registers a callback invoked after a successful refresh of the named
// data source.
func (s *Service) OnUpdate(identifier string, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if reg, ok := s.registrations[identifier]; ok {
		reg.onUpdate = append(reg.onUpdate, fn)
	}
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
		return reg.httpUpdate(false)
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
		close(r.stop)
		if r.watcher != nil {
			_ = r.watcher.Close()
		}
		r.wg.Wait()
	}
	return nil
}

func (r *registration) startPolling() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		for {
			delay := r.nextInterval()
			timer := time.NewTimer(delay)
			select {
			case <-r.stop:
				timer.Stop()
				return
			case <-timer.C:
			}
			if err := r.httpUpdate(false); err != nil {
				// Background errors are logged, never fatal.
				r.svc.logger.Warn("automatic update failed; will retry later",
					"engine", r.engineName, "error", err)
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
		target := filepath.Clean(r.cfg.DataFilePath)
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
					if err := r.refreshFromFile(); err != nil {
						r.svc.logger.Warn("refresh from watched file failed",
							"engine", r.engineName, "error", err)
					}
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
func (r *registration) httpUpdate(startup bool) error {
	url := r.cfg.DataUpdateURL
	if r.cfg.URLFormatter != nil {
		url = r.cfg.URLFormatter(r.cfg)
	}
	r.svc.logger.Info("Checking for update", "engine", r.engineName, "url", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if r.cfg.UseIfModifiedSince {
		if t, ok := r.currentDataTime(); ok {
			req.Header.Set("If-Modified-Since", t.UTC().Format(http.TimeFormat))
		}
	}

	resp, err := r.svc.httpClient.Do(req)
	if err != nil {
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

	body, err := readMaybeGzip(resp)
	if err != nil {
		r.svc.logger.Warn("error downloading data file", "engine", r.engineName, "url", url, "error", err)
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
	} else {
		r.refreshMu.Lock()
		err := r.engine.RefreshFromBytes(body)
		if err == nil {
			r.fireCallbacks()
		}
		r.refreshMu.Unlock()
		if err != nil {
			r.svc.logger.Error("error applying in-memory data update", "engine", r.engineName, "error", err)
			return err
		}
	}
	r.svc.logger.Info("Downloaded new data", "engine", r.engineName, "url", url)
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
		fn()
	}
}

// RateLimitError is returned when the update server responds with HTTP 429.
type RateLimitError struct {
	URL        string
	RetryAfter time.Duration
}

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

func readMaybeGzip(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("data update: empty response body")
	}
	return data, nil
}
