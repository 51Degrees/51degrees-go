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

// Package ipi_onpremise is the on-premise IP Intelligence and IP geolocation
// engine for Go. It reads a 51Degrees IP Intelligence data file on your server
// to find an IP address's likely location (country, region, town, and
// coordinates), connection type, and network. An alternative to MaxMind and
// IPinfo.
package ipi_onpremise

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

// Engine is an implementation of the on-premise (based on a local data file) IP intelligence. It encapsulates
// the automatic data file updates feature - to periodically fetch and reload the new data file.
// File system watcher feature allows to monitor for changes to the local data file and reload it when it changes.
// Custom URL can be used to fetch data files, the polling interval is configurable
// The 51degrees distributor service can also be used with a licenseKey
// For more information see With... options and examples
type Engine struct {
	*common_go.FileUpdater
	logger *common_go.LogWrapper

	manager *ipi_interop.ResourceManager
	config  *ipi_interop.ConfigIpi

	stopCh           chan *sync.WaitGroup
	reloadFileEvents chan struct{}
	// reloadLoop is done once reloadFileEvent has returned.
	reloadLoop sync.WaitGroup
	// reloadMu makes each reload of the data file run on its own, whether
	// update polling, the file watcher or RefreshData asked for it.
	reloadMu sync.Mutex
	// loadedFileTime is the modification time, in Unix nanoseconds, of the
	// data file when it was last loaded, so a watched file that is not newer
	// is not loaded again.
	loadedFileTime atomic.Int64

	product                   string
	dataFileLastUsedByManager string
	licenseKey                string

	maxRetries int

	isStopped bool

	// stopping is set as Stop begins, so a check waiting for the watched
	// file ends at once rather than holding up Stop.
	stopping atomic.Bool

	managerProperties []string
	// names holds the property name at each required property index. It is
	// made once, when the first data file is loaded, and kept across reloads,
	// because the C library keeps the names the engine was built with.
	names atomic.Pointer[ipi_interop.PropertyNames]
	// positions holds the position of each required property name. It is
	// made with names.
	positions atomic.Pointer[map[string]int]

	// profile and concurrency are the Performance Profile and Concurrency
	// options. nil leaves the option at its default.
	profile     *ipi_interop.PerformanceProfile
	concurrency *uint16
	// defaultConcurrency is the Concurrency used when none is set and no
	// native configuration was given. 0 leaves the native code's own.
	defaultConcurrency uint16

	// weighted holds, by lower case name, the Properties the data file
	// weights, once ProcessData has read them.
	weighted atomic.Pointer[map[string]bool]

	// useMu is held for reading by each call that reads the manager, and for
	// writing by Stop while it frees the manager, so Stop waits for those
	// calls and each later call finds freed set.
	useMu sync.RWMutex
	freed bool
}

// errEngineStopped is the error a call that reads the data gives once Stop
// has been called.
var errEngineStopped = errors.New("the engine has stopped, so its data has been freed")

// acquire holds the manager for one call until release, and reports false,
// holding nothing, once Stop has freed it.
func (e *Engine) acquire() bool {
	e.useMu.RLock()
	if e.freed {
		e.useMu.RUnlock()
		return false
	}
	return true
}

// release ends a hold that acquire gave.
func (e *Engine) release() { e.useMu.RUnlock() }

const (
	defaultDataFileUrl = "" // TODO: set default file path url (when it will be available)

	// engineTypeName names the engine in the data update log messages.
	engineTypeName = "IpiOnPremiseEngine"
)

// New creates an instance of the on-premise IP intelligence engine.  WithDataFile must be provided
// to specify the path to the data file, otherwise initialization will fail
func New(opts ...EngineOptions) (*Engine, error) {
	fileUpdater := common_go.NewFileUpdater(defaultDataFileUrl)
	logger := fileUpdater.GetLogger()

	engine := &Engine{
		FileUpdater: fileUpdater,
		logger:      logger,

		config:            nil,
		stopCh:            make(chan *sync.WaitGroup),
		reloadFileEvents:  make(chan struct{}),
		managerProperties: nil, // nil means "all properties"
	}

	fileUpdater.SetEngineName(engineTypeName)
	fileUpdater.SetPublishedDateFunc(engine.publishedDate)
	fileUpdater.SetNextUpdateFunc(engine.nextUpdateDate)
	fileUpdater.SetProductFunc(engine.dataFileProduct)

	for _, opt := range opts {
		err := opt(engine)
		if err != nil {
			engine.Stop()
			return nil, err
		}
	}

	if !engine.IsDataFileProvided() {
		return nil, common_go.ErrNoDataFileProvided
	}

	if err := engine.CheckDataFile(); err != nil {
		return nil, err
	}

	if err := engine.InitCreateTempDataCopy(); err != nil {
		return nil, err
	}
	err := engine.run()
	if err != nil {
		engine.Stop()
		return nil, err
	}

	// if file watcher is enabled, start the watcher. The data set is loaded
	// by now, so a failure stops the engine to free it.
	if engine.IsFileWatcherEnabled() {
		if err := engine.InitFileWatcher(engine.logger, engine.stopCh); err != nil {
			engine.Stop()
			return nil, err
		}

		if err := engine.Watch(engine.handleFileExternallyChanged); err != nil {
			engine.Stop()
			return nil, err
		}

		engine.SetFileWatcherStarted(true)
		engine.RunWatcher()
	}

	return engine, nil
}

// handleFileExternallyChanged reloads the data file the file watcher saw
// change, once it can be opened for reading, and reports the check. A file
// another process still holds is waited for, and one that cannot be opened in
// time is reported as a timeout and not logged, as the error table gives it.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md?plain=1#L512
func (e *Engine) handleFileExternallyChanged() {
	if !common_go.WaitUntilReadable(openWatchedFile, e.GetDataFile(),
		watchedFileReadableWait, e.stopped) {
		if !e.stopped() {
			e.reportTimeout()
		}
		e.IncreaseFileExternallyChangedCount()
		return
	}
	switch tries, err := e.reloadIfNewer(); {
	case errors.Is(err, errNotNewer):
		e.reportCheck(common_go.UpdateNotNeeded)
	case errors.Is(err, errLoadedMeanwhile):
		e.reportCheck(common_go.UpdateInProgress)
	default:
		if err != nil {
			e.logReloadError(tries, err)
		}
		e.reportReload(err)
	}

	e.IncreaseFileExternallyChangedCount()
}

// run starts the engine
func (e *Engine) run() error {
	e.reloadLoop.Add(1)
	go e.reloadFileEvent()

	// The update URL is completed first, as the update on start-up uses it
	// and runs before the data file is loaded.
	if err := e.validateAndAppendUrlParams(); err != nil {
		return err
	}

	// A data file that is already there is loaded first, so that the update
	// asks for data newer than its published date, and is loaded again only if
	// the update replaced it.
	loaded := false
	if e.IsUpdateOnStartEnabled() {
		if e.HasLocalDataFile() {
			if err := e.processFileExternallyChanged(); err != nil {
				return err
			}
			loaded = true
		}
		downloaded, err := e.PullOnStart()
		if err != nil {
			return err
		}
		if downloaded {
			// The update on start-up ends once the new file has loaded or
			// failed to, so it is reported here.
			loadErr := e.processFileExternallyChanged()
			callbackErr := e.ReportUpdateComplete(reloadStatus(loadErr))
			if loadErr != nil {
				return loadErr
			}
			if callbackErr != nil {
				return callbackErr
			}
			loaded = true
		}
	}

	if !loaded {
		if err := e.processFileExternallyChanged(); err != nil {
			return err
		}
	}

	if e.IsAutoUpdateEnabled() {
		e.SetFilePullerStarted(true)
		go e.ScheduleFilePulling(e.stopCh, e.reloadFileEvents)
	}

	return nil
}

// errStopped is the error RefreshData gives once the engine has stopped.
var errStopped = errors.New("the engine has stopped, so it has no data to refresh")

// RefreshData loads the Engine's data file again now, for an application
// that has replaced the file and does not leave it to the file watcher. It
// is safe to call while the Engine processes and while an update runs, as
// each reload runs on its own. When the file cannot be used it returns the
// error, after the tries a watched file gets, and the Engine goes on with the
// data it had. The caller gets the error, so it is not logged, and nothing is
// reported to the update callbacks, which hear the checks for an update.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md?plain=1#L36-L41
func (e *Engine) RefreshData() error {
	_, err := e.reload()
	return err
}

// Stop has to be called to free all the resources of the engine before the
// instance goes out of scope. It waits for the refreshes and the calls that
// read the data already running, then frees the data, and each such call
// made later gives an error or nothing. Results and Data already returned
// keep the data they were made from until they are freed.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/resource-cleanup.md?plain=1#L33-L40
func (e *Engine) Stop() {
	e.stopping.Store(true)
	num := 0
	if e.IsAutoUpdateEnabled() && e.IsFilePullerStarted() {
		num++ // file puller is enabled and started
	}
	if e.IsFileWatcherEnabled() && e.IsFileWatcherStarted() {
		num++ // file watcher is enabled and started
	}

	if num > 0 {
		var wg sync.WaitGroup
		wg.Add(num)
		for i := 0; i < num; i++ {
			e.stopCh <- &wg
		}
		// make sure that all routines finished processing current work, only after that free the manager
		wg.Wait()
	}

	// Closing the channel ends the reload loop once a reload in progress has
	// finished, so the manager is not freed while it is being reloaded.
	close(e.reloadFileEvents)
	e.reloadLoop.Wait()

	// A reload in progress, such as one RefreshData asked for, ends before the
	// manager is freed, and one asked for later refuses.
	e.reloadMu.Lock()
	e.isStopped = true
	e.reloadMu.Unlock()
	close(e.stopCh)

	e.useMu.Lock()
	e.freed = true
	if e.manager != nil {
		e.manager.Free()
	} else {
		e.logger.Printf("stopping engine, manager is nil")
	}
	e.useMu.Unlock()

	if e.IsCreateTempDataCopyEnabled() {
		dir := filepath.Dir(e.dataFileLastUsedByManager)
		os.RemoveAll(dir)
	}
}

// How often a reload of the data file is tried, as .NET and Java do, because
// the file can be held by another process for a moment.
const (
	// reloadTries is how many times a reload is tried before it is reported
	// as failed.
	reloadTries = 10
	// reloadRetryWaitMilliseconds is the wait between tries.
	reloadRetryWaitMilliseconds = 200
)

// reload loads the data file again, trying up to reloadTries times. It
// returns the tries made and the last error. A panic ends the tries, being a
// fault rather than a file another process holds, and is returned as a
// reloadPanic so the goroutine that asked for the reload carries on and a
// later file is loaded.
// See https://github.com/51Degrees/specifications/blob/e89679e/pipeline-specification/features/data-updates.md#errors
func (e *Engine) reload() (int, error) {
	return e.reloadTrying(false)
}

// errNotNewer and errLoadedMeanwhile say why reloadIfNewer did not load the
// data file, the second for a check that waited while another reload ran.
var (
	errNotNewer        = errors.New("the data file is not newer than the one loaded")
	errLoadedMeanwhile = errors.New("another reload loaded the data file while this one waited")
)

// reloadIfNewer reloads the watched data file as reload does, but only when
// it is newer than the one loaded.
// See https://github.com/51Degrees/specifications/blob/d79bbe3b2aef86273570742f6b660f29bfe72816/pipeline-specification/features/data-updates.md#newfileavailable-via-remote-or-file-watcher
func (e *Engine) reloadIfNewer() (int, error) {
	return e.reloadTrying(true)
}

// reloadTrying makes the tries of reload and reloadIfNewer.
func (e *Engine) reloadTrying(onlyNewer bool) (int, error) {
	for tries := 1; ; tries++ {
		err := e.reloadOnce(onlyNewer)
		var p *reloadPanic
		if err == nil || errors.As(err, &p) || errors.Is(err, errStopped) ||
			errors.Is(err, errNotNewer) || errors.Is(err, errLoadedMeanwhile) ||
			tries == reloadTries {
			return tries, err
		}
		time.Sleep(reloadRetryWaitMilliseconds * time.Millisecond)
	}
}

// reloadOnce loads the data file again, on its own, returning a panic as a
// reloadPanic. With onlyNewer, a data file no newer than the one loaded is
// not loaded.
func (e *Engine) reloadOnce(onlyNewer bool) (err error) {
	waited := !e.reloadMu.TryLock()
	if waited {
		e.reloadMu.Lock()
	}
	defer e.reloadMu.Unlock()
	if e.isStopped {
		return errStopped
	}
	if onlyNewer && !e.dataFileIsNewer() {
		if waited {
			return errLoadedMeanwhile
		}
		return errNotNewer
	}
	defer func() {
		if r := recover(); r != nil {
			err = &reloadPanic{r}
		}
	}()
	return e.processFileExternallyChanged()
}

// reloadPanic is a panic in a reload, returned as an error.
type reloadPanic struct{ value interface{} }

func (p *reloadPanic) Error() string { return fmt.Sprintf("panic: %v", p.value) }

// logReloadError logs a reload of a pulled or watched file that failed, as
// the specification's error table gives it, which follows .NET. A panic is
// logged with the table's last row, and any other failure as a refresh that
// failed after the tries made.
func (e *Engine) logReloadError(tries int, err error) {
	var p *reloadPanic
	if errors.As(err, &p) {
		e.logUnhandledError(p.value)
		return
	}
	e.logger.Errorf("Error during check for data update. An error occurred when applying a data update to "+
		"engine '%s' after %d tries. Error detail: %v", engineTypeName, tries, err)
}

// logUnhandledError logs a failure the error table has no row for, with its
// last row.
func (e *Engine) logUnhandledError(detail interface{}) {
	e.logger.Errorf("An unhandled error occurred while checking for automatic updates for engine '%s'. "+
		"Error detail: %v", engineTypeName, detail)
}

// reportReload reports a reload made by update polling or the file watcher to
// the update callbacks, and logs a callback's panic with the error table's
// last row.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/features/data-updates.md#reporting-each-check
func (e *Engine) reportReload(err error) {
	if callbackErr := e.ReportUpdateComplete(reloadStatus(err)); callbackErr != nil {
		e.logUnhandledError(callbackErr)
	}
}

// stopped says whether the engine is stopping or has stopped.
func (e *Engine) stopped() bool {
	return e.stopping.Load()
}

// reportTimeout reports a watched file that could not be opened in time to
// the update callbacks, and logs an error a callback gives back.
func (e *Engine) reportTimeout() {
	e.reportCheck(common_go.UpdateTimeout)
}

// reportCheck reports a check that loaded nothing to the update callbacks,
// and logs an error a callback gives back.
func (e *Engine) reportCheck(status common_go.UpdateStatus) {
	if err := e.ReportUpdateComplete(status); err != nil {
		e.logUnhandledError(err)
	}
}

// dataFileIsNewer says whether the data file was modified after the one last
// loaded. A file that cannot be read counts as newer, so the load is tried
// and its failure reported.
func (e *Engine) dataFileIsNewer() bool {
	info, err := os.Stat(e.GetDataFile())
	return err != nil || info.ModTime().UnixNano() > e.loadedFileTime.Load()
}

// reloadStatus gives the status a reload that ended with err reports: success,
// the refresh failure, or the unknown error for a panic.
func reloadStatus(err error) common_go.UpdateStatus {
	var p *reloadPanic
	switch {
	case err == nil:
		return common_go.UpdateSuccess
	case errors.As(err, &p):
		return common_go.UpdateUnknownError
	default:
		return common_go.UpdateRefreshFailed
	}
}

// reloadFileEvent reloads the data file each time the file puller has pulled a
// new one, until the engine stops. A failed reload is logged and the engine
// keeps the data it has, so a later file is still loaded.
func (e *Engine) reloadFileEvent() {
	defer e.reloadLoop.Done()
	for range e.reloadFileEvents {
		tries, err := e.reload()
		if err != nil {
			e.logReloadError(tries, err)
		}
		e.reportReload(err)
	}
}

// validateAndAppendUrlParams completes the data file URL when the engine
// downloads data files. The default URL is given the license key, which it
// needs, and any URL is given the product option, as appendProduct says.
func (e *Engine) validateAndAppendUrlParams() error {
	if !e.IsAutoUpdateEnabled() && !e.IsUpdateOnStartEnabled() {
		return nil
	}
	if e.isDefaultDataFileUrl() {
		if !e.hasDefaultDistributorParams() {
			return common_go.ErrLicenseKeyRequired
		}
		if err := e.appendLicenceKey(); err != nil {
			return err
		}
	}
	return e.appendProduct()
}

// appendProduct gives the data file URL the product option as its Product
// parameter, in place of an empty one, so every request asks for that
// product and a 300 (Multiple Choices) response is not followed. A URL that
// names a product already, which only a caller's own URL can, is left as it
// is, and so is any URL when the option is not set, so the first request of
// an update then names no product.
// See https://github.com/51Degrees/specifications/blob/5f4a2ee6c435bd78c3ae283756400317864e42f9/pipeline-specification/features/data-updates.md#configured-product
func (e *Engine) appendProduct() error {
	if e.product == "" {
		return nil
	}
	urlParsed, err := url.Parse(e.GetDataFileUrl())
	if err != nil {
		return fmt.Errorf("failed to parse data file url: %w", err)
	}
	query := urlParsed.Query()
	if namesProduct(query) {
		return nil
	}
	for name := range query {
		if strings.EqualFold(name, productParameter) {
			query.Del(name)
		}
	}
	query.Set(productParameter, e.product)
	urlParsed.RawQuery = query.Encode()

	e.SetDataFileUrl(urlParsed.String())

	return nil
}

// isDefaultDataFileUrl checks if the current data file URL matches the predefined defaultDataFileUrl constant.
func (e *Engine) isDefaultDataFileUrl() bool {
	return e.GetDataFileUrl() == defaultDataFileUrl
}

// hasDefaultDistributorParams checks if the default distributor parameters are set based on the presence of a license key.
func (e *Engine) hasDefaultDistributorParams() bool {
	return len(e.licenseKey) > 0
}

// processFileExternallyChanged reloads the file if it detects external changes by invoking the reload manager with the file path.
func (e *Engine) processFileExternallyChanged() error {
	// The time is read before the load, so a file written during it is newer.
	info, statErr := os.Stat(e.GetDataFile())
	reloadFilePath, err := e.GetReloadFilePath()
	if err != nil {
		return err
	}

	if err := e.reloadManager(reloadFilePath); err != nil {
		return err
	}
	if statErr == nil {
		e.loadedFileTime.Store(info.ModTime().UnixNano())
	}
	return nil
}

// reloadManager loads the data file at filePath. The first call, when the
// engine starts, makes the manager and with it the tables of names and
// positions. Every later call, for a new file or a refresh, reloads the
// manager's data set and keeps both tables, because the C library keeps the
// properties the manager was made with.
func (e *Engine) reloadManager(filePath string) (err error) {
	if e.isStopped {
		return nil
	}
	// Only a load that worked is logged, as a manager whose first load failed
	// has no data set to read the published date from. Each return that loaded
	// sets loaded first, because err is still nil while a panic unwinds.
	loaded := false
	defer func() {
		if !loaded {
			return
		}
		year, month, day := e.getPublishedDate().Date()
		e.logger.Printf("data file loaded from " + filePath + " published on: " + fmt.Sprintf("%d-%d-%d", year, month, day))
	}()

	if e.manager == nil {
		e.manager = ipi_interop.NewResourceManager()
		// init manager from file
		e.config = e.nativeConfig()

		if err := ipi_interop.InitManagerFromFile(e.manager, *e.config, strings.Join(e.managerProperties, ","), filePath); err != nil {
			if e.IsCreateTempDataCopyEnabled() {
				e.removeCopy(filePath)
			}
			return fmt.Errorf("failed to init manager from file: %+v", err)
		}
		// The table of names is made once, here, and never from a later
		// data set.
		names := ipi_interop.NewPropertyNames(e.manager)
		e.names.Store(names)
		e.setPositions(namesOf(names))
		e.dataFileLastUsedByManager = filePath
		loaded = true
		return nil
	} else if !e.IsCreateTempDataCopyEnabled() {
		e.logger.Infof("Attempting to refresh Engine '%s' with new data", engineTypeName)
		err := e.manager.ReloadFromOriginalFile()
		if err != nil {
			return fmt.Errorf("failed to reload manager from original file: %w", err)
		}
		loaded = true
		return nil
	}

	e.logger.Infof("Attempting to refresh Engine '%s' with new data", engineTypeName)
	err = e.manager.ReloadFromFile(*e.config, strings.Join(e.managerProperties, ","), filePath)
	if err != nil {
		// The data set in use is kept, so the copy made for this reload is
		// not needed.
		e.removeCopy(filePath)
		return fmt.Errorf("failed to reload manager from file: %w", err)
	}

	e.removeCopy(e.dataFileLastUsedByManager)
	e.dataFileLastUsedByManager = filePath
	loaded = true

	return nil
}

// removeCopy deletes a temporary copy of the data file. Windows cannot delete
// a copy that a data set still has open, so a failure is logged and the copy is
// left for Stop, which removes the temporary directory.
func (e *Engine) removeCopy(path string) {
	if err := os.Remove(path); err != nil {
		e.logger.Printf("failed to remove data file copy: %v", err)
	}
}

// getPublishedDate retrieves the published date of the data file being used by the engine.
func (e *Engine) getPublishedDate() time.Time {
	return ipi_interop.GetPublishedDate(e.manager)
}

// GetHttpHeaderKeys returns the Evidence keys the data in use accepts, worked
// out from the data when it is loaded and each time it is refreshed, or nil
// once the engine has stopped. It is safe to call while the data is
// refreshed.
func (e *Engine) GetHttpHeaderKeys() []ipi_interop.EvidenceKey {
	if !e.acquire() {
		return nil
	}
	defer e.release()
	if e.manager == nil {
		return nil
	}
	return e.manager.HeaderKeys()
}

// publishedDate is the published date of the loaded data file, or the zero
// time before a data file is loaded or once the engine has stopped.
func (e *Engine) publishedDate() time.Time {
	if !e.acquire() {
		return time.Time{}
	}
	defer e.release()
	if e.manager == nil {
		return time.Time{}
	}
	return e.getPublishedDate()
}

// nextUpdateDate is the date the loaded data file expects an update after, or
// the zero time before a data file is loaded or once the engine has stopped.
func (e *Engine) nextUpdateDate() time.Time {
	if !e.acquire() {
		return time.Time{}
	}
	defer e.release()
	if e.manager == nil {
		return time.Time{}
	}
	return ipi_interop.GetNextUpdateDate(e.manager)
}

// NewResultsIpi creates a new ResultsIpi object using this engine's manager,
// or returns nil once the engine has stopped. Caller is responsible for
// calling Free() on the returned object. A ResultsIpi is not safe to use from
// several goroutines at once, so a reused one belongs to one goroutine at a
// time. The caller is responsible for thread-safe access to one it shares.
func (e *Engine) NewResultsIpi() *ipi_interop.ResultsIpi {
	if !e.acquire() {
		return nil
	}
	defer e.release()
	return ipi_interop.NewResultsIpi(e.manager)
}

// Process processes the given IP address and retrieves associated values using the default properties.
// If results is nil, creates a new ResultsIpi object for this call (per-call mode).
// If results is provided, reuses the existing object (reuse mode for better performance).
//
// Deprecated: Process copies the values of every required Property out of
// the native results while it processes, and the specification requires a
// value to be read only when its Property is requested. Use ProcessData,
// which does that, and call Free on the Data it returns.
func (e *Engine) Process(ipAddress string) (ipi_interop.Values, error) {
	return e.ProcessWithResults(ipAddress, nil)
}

// ProcessWithResults processes the given IP address with an optional reusable ResultsIpi object.
// If results is nil, creates a new ResultsIpi object for this call.
// If results is provided, reuses it for better performance (caller manages lifecycle).
// A ResultsIpi is not safe to use from several goroutines at once, so a reused
// one belongs to one goroutine at a time. The caller is responsible for
// thread-safe access to one it shares.
//
// Deprecated: ProcessWithResults copies the values of every required Property
// out of the native results while it processes, and the specification
// requires a value to be read only when its Property is requested. Use
// ProcessData, which does that, and call Free on the Data it returns.
func (e *Engine) ProcessWithResults(ipAddress string, results *ipi_interop.ResultsIpi) (ipi_interop.Values, error) {
	if !e.acquire() {
		return nil, errEngineStopped
	}
	defer e.release()
	var shouldFree bool

	if results == nil {
		// Create a new ResultsIpi object for this call
		results = ipi_interop.NewResultsIpi(e.manager)
		shouldFree = true // We created it, so we should free it
	}

	if shouldFree {
		defer results.Free() // Ensure proper cleanup only if we created it
	}

	if err := results.ResultsIpiFromIpAddress(ipAddress); err != nil {
		return nil, err
	}

	var values ipi_interop.Values
	var err error

	if results.HasValues() {
		// The values are those of every required property. An index names
		// the same property in every data set the manager loads, so the table
		// made at build names the values of results from any of them.
		var resolver func(int) string
		if names := e.names.Load(); names != nil {
			resolver = names.Name
		}
		values, err = results.GetWeightedValuesByIndexes(nil, resolver)
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

// appendLicenceKey appends the license key as a query parameter to the data file URL in the Engine instance.
func (e *Engine) appendLicenceKey() error {
	urlParsed, err := url.Parse(e.GetDataFileUrl())
	if err != nil {
		return err
	}
	query := urlParsed.Query()
	query.Set("LicenseKeys", e.licenseKey)
	urlParsed.RawQuery = query.Encode()

	e.SetDataFileUrl(urlParsed.String())

	return nil
}

// GetPropertyNameByIndex returns the name of the property at the required
// property index, or an empty string for an index outside the required
// properties.
func (e *Engine) GetPropertyNameByIndex(index int) string {
	return e.names.Load().Name(index)
}
