package common_go

import (
	"bufio"
	"compress/gzip"
	"crypto/md5"
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// pullRandom draws the random time added to each polling interval. It has
// its own source, seeded from crypto/rand, because the global source is not
// seeded by every Go version this module supports, and unseeded servers
// would draw the same times and poll in step.
var (
	pullRandomMu sync.Mutex
	pullRandom   = rand.New(rand.NewSource(randomSeed()))
)

// randomSeed returns a seed read from crypto/rand, or the time if that fails.
func randomSeed() int64 {
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	return int64(binary.LittleEndian.Uint64(b[:]))
}

// randomizationMs returns a random number of milliseconds from 0 up to and
// including maxMs, or 0 when maxMs is 0 or less. A new time is drawn for each
// interval so that servers with the same setting do not poll in step.
func randomizationMs(maxMs int) int {
	if maxMs <= 0 {
		return 0
	}
	pullRandomMu.Lock()
	defer pullRandomMu.Unlock()
	if int64(maxMs) == math.MaxInt64 {
		return int(pullRandom.Int63())
	}
	return int(pullRandom.Int63n(int64(maxMs) + 1))
}

// drawRandomizationMs is the draw the scheduler uses. It is a variable so that
// a test can see the maximum the scheduler passes to it.
var drawRandomizationMs = randomizationMs

// ScheduleFilePulling schedules periodic file pulling based on specified intervals and handles stop signals gracefully.
// It performs an immediate pull on startup if the update-on-start feature is enabled and PullOnStart has not
// already done so. When the loaded data file expects an update after a later date, the first pull waits until then.
func (e *FileUpdater) ScheduleFilePulling(stopCh chan *sync.WaitGroup, reloadFileEvents chan struct{}) {
	nextIterationInMs := e.dataFilePullEveryMs

	// if update on start is enabled and has not run yet, perform the pull immediately
	if e.isUpdateOnStartEnabled && !e.startUpPullDone {
		e.logger.Infof("Updating on start-up")
		nextIterationInMs = e.pullRecovering(reloadFileEvents)
	}

	nextIterationInMs = e.delayBeforeFirstPull(nextIterationInMs)

	for {
		select {
		case wg := <-stopCh:
			wg.Done()
			return
		// interval to perform the pull of updated data, plus a random time
		// up to the randomization maximum
		case <-time.After(time.Duration(nextIterationInMs+drawRandomizationMs(e.randomization)) * time.Millisecond):
			nextIterationInMs = e.pullRecovering(reloadFileEvents)
		}
	}
}

// PullOnStart runs the start-up update, which blocks until the pull has finished and does not announce the new
// file, as the caller loads the data file afterwards. It returns true when a new data file replaced the old one.
// A data file that is already current is not an error, and a failed pull falls back to the local data file. It
// returns an error only when the pull failed and there is no local data file to fall back to.
func (e *FileUpdater) PullOnStart() (bool, error) {
	e.startUpPullDone = true
	e.logger.Infof("Updating on start-up")
	_, downloaded, err := e.pull(nil)
	if err == nil || errors.Is(err, ErrFileNotModified) || e.HasLocalDataFile() {
		return downloaded, nil
	}
	return false, fmt.Errorf("the update on start-up failed and there is no local data file: %w", err)
}

// CheckDataFile checks that the data file exists. A missing data file is allowed when update on start-up is on,
// as the start-up update downloads it.
func (e *FileUpdater) CheckDataFile() error {
	_, err := os.Stat(e.GetDataFile())
	if err != nil && !(e.isUpdateOnStartEnabled && os.IsNotExist(err)) {
		return fmt.Errorf("failed to get file path: %w", err)
	}
	return nil
}

// HasLocalDataFile reports whether the data file exists.
func (e *FileUpdater) HasLocalDataFile() bool {
	if e.GetDataFile() == "" {
		return false
	}
	_, err := os.Stat(e.GetDataFile())
	return err == nil
}

// earliestDate is the first year taken as a real date. A data file that carries no date reads as year 0.
const earliestDate = 1970

// validDate reports whether a date read from a data file is set.
func validDate(t time.Time) bool {
	return !t.IsZero() && t.Year() >= earliestDate
}

// delayBeforeFirstPull returns the wait before the first scheduled pull. Polling does not start before the date
// the data file expects an update, so that date replaces the regular interval. A longer wait that the server asked
// for is kept.
func (e *FileUpdater) delayBeforeFirstPull(delayMs int) int {
	untilMs, ok := e.msUntilNextUpdate()
	if ok && (delayMs == e.dataFilePullEveryMs || untilMs > delayMs) {
		return untilMs
	}
	return delayMs
}

// msUntilNextUpdate returns the milliseconds until the date the data file expects an update, and false when the
// data file gives no date or the date has passed.
func (e *FileUpdater) msUntilNextUpdate() (int, bool) {
	if e.nextUpdate == nil {
		return 0, false
	}
	next := e.nextUpdate()
	if !validDate(next) {
		return 0, false
	}
	until := time.Until(next)
	if until <= 0 {
		return 0, false
	}
	ms := until / time.Millisecond
	if ms > math.MaxInt32 {
		return math.MaxInt32, true
	}
	return int(ms), true
}

// ifModifiedSince returns the date sent as If-Modified-Since. It is the publication date of the loaded data file,
// or the file system modification time when the data file has no publication date. It returns nil when there is no
// data file yet, and an error when the file cannot be read.
func (e *FileUpdater) ifModifiedSince() (*time.Time, error) {
	if e.publishedDate != nil {
		if published := e.publishedDate(); validDate(published) {
			published = published.UTC()
			return &published, nil
		}
	}
	if len(e.GetDataFile()) == 0 {
		return nil, nil
	}
	file, err := os.Stat(e.GetDataFile())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	modTime := file.ModTime().UTC()
	return &modTime, nil
}

// maskedDataFileUrl is the data file URL with its license keys masked, for log
// messages.
func (e *FileUpdater) maskedDataFileUrl() string {
	return MaskLicenseKeysIn(e.GetDataFileUrl(), LicenseKeysInURL(e.GetDataFileUrl()))
}

// engineLabel names the engine in log messages.
func (e *FileUpdater) engineLabel() string {
	if e.engineName == "" {
		return "Engine"
	}
	return e.engineName
}

// pullRecovering makes one pull and returns the time in milliseconds before
// the next. A panic in the pull is logged and the next pull comes after the
// polling interval, so one failed pull does not end automatic updates.
func (e *FileUpdater) pullRecovering(reloadFileEvents chan struct{}) (nextMs int) {
	defer func() {
		if r := recover(); r != nil {
			e.logUnhandledError(fmt.Errorf("panic: %v", r))
			nextMs = e.dataFilePullEveryMs
		}
	}()
	return e.doFilePulling(reloadFileEvents)
}

// doFilePulling handles the process of pulling a data file, saving it locally, and triggering reload events if necessary.
// It checks for file modifications and makes an HTTP request to fetch the file.
// The method returns the time in milliseconds before the next pull attempt, which is the polling interval unless the
// server gave a Retry-After time.
func (e *FileUpdater) doFilePulling(reloadFileEvents chan struct{}) int {
	next, _, _ := e.pull(reloadFileEvents)
	return next
}

// pull does one pull and returns the time in milliseconds before the next one, whether a new data file replaced
// the old one, and the error that stopped it. The new file is announced on reloadFileEvents, unless the file
// watcher does that or the channel is nil.
func (e *FileUpdater) pull(reloadFileEvents chan struct{}) (int, bool, error) {
	e.logger.Infof("Checking for update")

	// The download is written next to the data file, so without a data file
	// path there is nowhere to put it.
	if e.GetDataFile() == "" {
		e.logCheckError(fmt.Sprintf("The data file '%s' is checking for updates but does not have a "+
			"temporary file path configured.", e.engineLabel()))
		return e.GetDataFilePullEveryMs(), false, ErrNoDataFileProvided
	}

	lastModificationTimestamp, err := e.ifModifiedSince()
	if err != nil {
		e.logUnhandledError(fmt.Errorf("failed to get file info: %w", err))
		return e.GetDataFilePullEveryMs(), false, err
	}

	e.logger.Infof("Checking for update from '%s' for Engine '%s'", e.maskedDataFileUrl(), e.engineLabel())

	// The download goes to a temp file next to the data file and is renamed over it, so the
	// engine only ever sees the complete old file or the complete new one.
	fileResponse, err := doDataFileRequest(e.dataFileUrl, e.logger, lastModificationTimestamp, e.GetDataFile())
	if err != nil {
		if errors.Is(err, ErrFileNotModified) {
			e.logger.Infof("No data newer than %s found at '%s' for Engine '%s'",
				formatSince(lastModificationTimestamp), e.maskedDataFileUrl(), e.engineLabel())
			return e.GetDataFilePullEveryMs(), false, err
		}
		e.logPullError(err)
		if fileResponse != nil && fileResponse.retryAfter > 0 {
			e.logger.Printf("received retry-after, retrying after %d seconds", fileResponse.retryAfter)
			// retry after the specified time
			return fileResponse.retryAfter * 1000, false, err
		}
		e.logger.Printf("retrying after the polling interval")
		return e.GetDataFilePullEveryMs(), false, err
	}

	e.logger.Infof("Downloaded new data from '%s' for Engine '%s'", e.maskedDataFileUrl(), e.engineLabel())

	err = os.Rename(fileResponse.path, e.GetDataFile())
	if err != nil {
		os.Remove(fileResponse.path)
		e.logCheckError(fmt.Sprintf("An error occurred when copying a data file to replace the existing one "+
			"at '%s'. Error detail: %v", e.GetDataFile(), err))
		return e.GetDataFilePullEveryMs(), false, err
	}
	e.logger.Printf("data file written successfully: %d bytes", fileResponse.size)

	if reloadFileEvents != nil && !e.isFileWatcherEnabled {
		// use the chan for reload the file and reload manager
		reloadFileEvents <- struct{}{}
	}

	if !e.fileSynced {
		e.fileSynced = true
	}

	timestamp := time.Now().UTC()
	e.SetLastModificationTimestamp(&timestamp)
	e.totalFilePulls++

	// reset nextIterationInMs
	return e.dataFilePullEveryMs, true, nil
}

// formatSince writes the date a pull asked for newer data than, for the log.
func formatSince(since *time.Time) string {
	if since == nil {
		return "the current data"
	}
	return since.UTC().Format(time.RFC3339)
}

// logPullError logs a failed pull as the specification's error table gives it,
// which follows .NET. A failure the table has a row for is logged at Error as
// "Error during check for data update", followed by the row's message, and any
// other failure at Error with the table's last row.
func (e *FileUpdater) logPullError(err error) {
	if message, ok := e.pullErrorMessage(err); ok {
		e.logCheckError(message)
		return
	}
	e.logUnhandledError(err)
}

// logCheckError logs a failure the error table has a row for.
func (e *FileUpdater) logCheckError(message string) {
	e.logger.Errorf("Error during check for data update. %s", message)
}

// logUnhandledError logs a failure the error table has no row for.
func (e *FileUpdater) logUnhandledError(err error) {
	e.logger.Errorf("An unhandled error occurred while checking for automatic updates for engine '%s'. "+
		"Error detail: %v", e.engineLabel(), err)
}

// pullErrorMessage gives the error table's message for a failed pull, and
// false for a failure the table has no row for.
func (e *FileUpdater) pullErrorMessage(err error) (string, bool) {
	var pe *pullError
	if !errors.As(err, &pe) {
		return "", false
	}
	url, engine := e.maskedDataFileUrl(), e.engineLabel()
	switch pe.kind {
	case pullConnect:
		return fmt.Sprintf("Error accessing data update service at '%s' for engine '%s'. Error detail: %v",
			url, engine, pe.err), true
	case pullTooManyRequests:
		return fmt.Sprintf("Too many requests to '%s' for engine '%s'", url, engine), true
	case pullForbidden:
		return fmt.Sprintf("Access denied to data update service at '%s' for engine '%s'", url, engine), true
	case pullStatus:
		return fmt.Sprintf("HTTP status code '%d' from data update service at '%s' for engine '%s'",
			pe.status, url, engine), true
	case pullIntegrity:
		// The table, as .NET, has no space after the engine's name.
		return fmt.Sprintf("Integrity check failed. MD5 hash in HTTP response '%s' for '%s'data update does "+
			"not match calculated hash for the downloaded file '%s'.", pe.expectedMD5, engine, pe.actualMD5), true
	}
	return "", false
}

// pullErrorKind says which step of a pull failed, which decides the log message.
type pullErrorKind int

const (
	pullConnect pullErrorKind = iota
	pullDownload
	pullTooManyRequests
	pullForbidden
	pullStatus
	pullIntegrity
	pullWrite
)

// pullError is an error from a pull tagged with the step that failed. Its text is the text of the error it wraps.
type pullError struct {
	kind pullErrorKind
	err  error
	// status is the HTTP status code of a pullStatus failure.
	status int
	// expectedMD5 and actualMD5 are the hashes of a pullIntegrity failure,
	// the first as the response gave it and the second as read from the file.
	expectedMD5 string
	actualMD5   string
}

func (p *pullError) Error() string { return p.err.Error() }
func (p *pullError) Unwrap() error { return p.err }

func newPullError(kind pullErrorKind, err error) error {
	return &pullError{kind: kind, err: err}
}

// FileResponse represents the response obtained from a file request: the path of the temp file
// holding the uncompressed data and its size, or the retry interval when the server asked to wait.
type FileResponse struct {
	path       string
	size       int64
	retryAfter int
}

// tempFilePattern names the file a pull is streamed into, next to the data file. It carries the data
// file's name so that updaters sharing a directory never touch each other's downloads.
func tempFilePattern(dataFile string) string {
	return "." + filepath.Base(dataFile) + ".pull-*"
}

// doDataFileRequest performs an HTTP GET request to fetch a data file and handles retries, caching, and decompression.
// It accepts a URL, a logger for logging messages, an optional timestamp for conditional requests, and the data file
// path: the body is streamed into a temp file in that file's directory, decompressed on the fly while the MD5 of the
// raw body is computed, so a multi-gigabyte data file needs no extra RAM. The directory needs free space for a second
// copy of the data file until the caller renames the temp file over it.
// Returns a FileResponse with the temp file path or an error if the request fails or conditions are not met.
func doDataFileRequest(url string, logger *LogWrapper, timestamp *time.Time, dataFile string) (*FileResponse, error) {
	removeStaleTempFiles(dataFile, logger)

	// An error net/http raises names the URL, so its license keys are masked.
	keys := LicenseKeysInURL(url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, newPullError(pullConnect, MaskLicenseKeysInError(err, keys))
	}

	if timestamp != nil {
		req.Header.Set("If-Modified-Since", timestamp.UTC().Format(http.TimeFormat))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, newPullError(pullConnect, MaskLicenseKeysInError(err, keys))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, ErrFileNotModified
	}

	//handle retry after
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		retryAfterSeconds, err := strconv.Atoi(retryAfter)
		if err != nil || len(retryAfter) == 0 {
			return nil, newPullError(pullTooManyRequests,
				fmt.Errorf("failed to parse Retry-After header: %v", err))
		}

		return &FileResponse{
			retryAfter: retryAfterSeconds,
		}, newPullError(pullTooManyRequests, fmt.Errorf("retrying after %d seconds", retryAfterSeconds))
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, newPullError(pullForbidden, errors.New(resp.Status))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &pullError{
			kind:   pullStatus,
			err:    errors.New(resp.Status),
			status: resp.StatusCode,
		}
	}

	// the MD5 header covers the raw body, compressed or not, so hash everything read from it
	hash := md5.New()
	body := bufio.NewReader(io.TeeReader(resp.Body, hash))

	// check if the response is compressed
	// if it is, decompress it
	head, _ := body.Peek(512)
	var fileBytes io.Reader = body
	if isGzip(http.DetectContentType(head)) {
		fileBytes, err = gzip.NewReader(body)
		if err != nil {
			return nil, newPullError(pullDownload, fmt.Errorf("failed to decompress file: %v", err))
		}
	}

	tempFile, err := os.CreateTemp(filepath.Dir(dataFile), tempFilePattern(dataFile))
	if err != nil {
		return nil, newPullError(pullWrite, fmt.Errorf("failed to create temp file: %v", err))
	}
	size, err := writeDataFile(tempFile, fileBytes, body, dataFile)
	if err == nil {
		contentMD5 := resp.Header.Get("Content-MD5")
		if len(contentMD5) > 0 {
			if actual := hex.EncodeToString(hash.Sum(nil)); actual != contentMD5 {
				err = &pullError{
					kind:        pullIntegrity,
					err:         errors.New("MD5 validation failed"),
					expectedMD5: contentMD5,
					actualMD5:   actual,
				}
			}
		} else {
			logger.Warnf("No MD5 hash included in data update response for '%s'. "+
				"Unable to verify data integrity", MaskLicenseKeysIn(url, keys))
		}
	}
	if err != nil {
		os.Remove(tempFile.Name())
		return nil, err
	}

	return &FileResponse{
		path:       tempFile.Name(),
		size:       size,
		retryAfter: 0,
	}, nil
}

// writeDataFile copies the uncompressed data into the temp file, drains the rest of the body so the MD5 hash
// covers it, flushes the file to disk and gives it the data file's permissions (0644 for a new data file).
func writeDataFile(tempFile *os.File, data io.Reader, body io.Reader, dataFile string) (int64, error) {
	size, err := io.Copy(tempFile, data)
	if err != nil {
		tempFile.Close()
		return 0, newPullError(pullWrite, fmt.Errorf("failed to write temp file: %v", err))
	}
	// the gzip reader stops before any trailing bytes
	if _, err = io.Copy(io.Discard, body); err != nil {
		tempFile.Close()
		return 0, newPullError(pullDownload, fmt.Errorf("failed to read response: %v", err))
	}
	// a rename after a power loss must not leave an empty data file behind
	if err = tempFile.Sync(); err != nil {
		tempFile.Close()
		return 0, newPullError(pullWrite, fmt.Errorf("failed to flush temp file: %v", err))
	}
	if err = tempFile.Close(); err != nil {
		return 0, newPullError(pullWrite, fmt.Errorf("failed to close temp file: %v", err))
	}
	mode := os.FileMode(0644)
	if info, statErr := os.Stat(dataFile); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err = os.Chmod(tempFile.Name(), mode); err != nil {
		return 0, newPullError(pullWrite, fmt.Errorf("failed to set temp file permissions: %v", err))
	}
	return size, nil
}

// removeStaleTempFiles deletes temp files a previous pull of this data file left behind, for example after
// the process was killed mid-download; otherwise every interrupted pull keeps a copy of the data file on disk.
func removeStaleTempFiles(dataFile string, logger *LogWrapper) {
	stale, _ := filepath.Glob(filepath.Join(filepath.Dir(dataFile), tempFilePattern(dataFile)))
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			logger.Printf("failed to remove stale temp file %s: %v", path, err)
		} else {
			logger.Printf("removed stale temp file %s", path)
		}
	}
}

// isGzip checks if the given content type corresponds to a Gzip-compressed format. Returns true if it matches.
func isGzip(contentType string) bool {
	return contentType == "application/gzip" || contentType == "application/x-gzip"
}

// validateMd5 compares a given MD5 hash for a byte slice and returns true if they match or an error if hashing fails.
func validateMd5(val []byte, hash string) (bool, error) {
	h := md5.New()
	_, err := h.Write(val)
	if err != nil {
		return false, err
	}

	strVal := hex.EncodeToString(h.Sum(nil))

	return strVal == hash, nil
}
