// Package ipi_onpremise is the on-premise IP Intelligence and IP geolocation
// engine for Go. It reads a 51Degrees IP Intelligence data file on your server
// to find an IP address's likely location (country, region, town, and
// coordinates), connection type, and network. An alternative to MaxMind and
// IPinfo.
package ipi_onpremise

import (
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

	product                   string
	dataFileLastUsedByManager string
	licenseKey                string

	maxRetries int

	isStopped bool

	managerProperties []string
	// names holds the property names of the data set loaded last. namesMu
	// stops two loads replacing it at once.
	names   atomic.Pointer[ipi_interop.PropertyNames]
	namesMu sync.Mutex
}

const (
	defaultDataFileUrl = "" // TODO: set default file path url (when it will be available)

	// engineTypeName names the engine in the data update log messages.
	engineTypeName = "IpIntelligenceEngine"
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

// handleFileExternallyChanged handles the logic for processing a file that has been altered externally to ensure consistency.
func (e *Engine) handleFileExternallyChanged() {
	if err := e.reload(); err != nil {
		e.logger.Errorf("An error occurred while applying a data file update to %s. "+
			"Error detail: %v", engineTypeName, err)
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
			loaded = false
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

// Stop has to be called to free all the resources of the engine
// before the instance goes out of scope
func (e *Engine) Stop() {
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

	e.isStopped = true
	close(e.stopCh)

	// The names hold a data set of the manager, so they go first.
	e.namesMu.Lock()
	if names := e.names.Swap(nil); names != nil {
		names.Release()
	}
	e.namesMu.Unlock()

	if e.manager != nil {
		e.manager.Free()
	} else {
		e.logger.Printf("stopping engine, manager is nil")
	}

	if e.IsCreateTempDataCopyEnabled() {
		dir := filepath.Dir(e.dataFileLastUsedByManager)
		os.RemoveAll(dir)
	}
}

// reload loads the data file again. A panic is returned as an error, so the
// goroutine that asked for the reload carries on and a later file is loaded.
func (e *Engine) reload() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return e.processFileExternallyChanged()
}

// reloadFileEvent reloads the data file each time the file puller has pulled a
// new one, until the engine stops. A failed reload is logged and the engine
// keeps the data it has, so a later file is still loaded.
func (e *Engine) reloadFileEvent() {
	defer e.reloadLoop.Done()
	for range e.reloadFileEvents {
		if err := e.reload(); err != nil {
			e.logger.Errorf("An error occurred while applying a data file update to %s. "+
			"Error detail: %v", engineTypeName, err)
		}
	}
}

// validateAndAppendUrlParams validates URL parameters and appends necessary values for default data file URLs.
// Returns an error if required parameters are missing or appending fails.
func (e *Engine) validateAndAppendUrlParams() error {
	downloads := e.IsAutoUpdateEnabled() || e.IsUpdateOnStartEnabled()
	if e.isDefaultDataFileUrl() && !e.hasDefaultDistributorParams() && downloads {
		return common_go.ErrLicenseKeyRequired
	}

	if e.isDefaultDataFileUrl() && downloads {
		if err := e.appendLicenceKey(); err != nil {
			return err
		}

		if err := e.appendProduct(); err != nil {
			return err
		}
	}

	return nil
}

// appendProduct appends the product parameter to the data file URL and updates the URL in the Engine. Returns an error if parsing fails.
func (e *Engine) appendProduct() error {
	urlParsed, err := url.Parse(e.GetDataFileUrl())
	if err != nil {
		return fmt.Errorf("failed to parse data file url: %w", err)
	}
	query := urlParsed.Query()
	query.Set("Product", e.product)
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
	reloadFilePath, err := e.GetReloadFilePath()
	if err != nil {
		return err
	}

	if err := e.reloadManager(reloadFilePath); err != nil {
		return err
	}
	e.refreshPropertyNames()

	return nil
}

// refreshPropertyNames reads the property names of the data set the manager
// has now, in place of those of the data set loaded before it.
func (e *Engine) refreshPropertyNames() {
	e.namesMu.Lock()
	defer e.namesMu.Unlock()
	if e.isStopped || e.manager == nil {
		return
	}
	if old := e.names.Swap(ipi_interop.NewPropertyNames(e.manager)); old != nil {
		old.Release()
	}
}

// this function will be called when the engine is started or the is new file available
// it will create and initialize a new manager from the new file if it does not exist
// if the manager exists, it will create a new manager from the new file and replace the existing manager thus freeing memory of the old manager
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
		if e.config == nil {
			e.config = ipi_interop.NewConfigIpi(ipi_interop.Balanced)
		}

		if err := ipi_interop.InitManagerFromFile(e.manager, *e.config, strings.Join(e.managerProperties, ","), filePath); err != nil {
			if e.IsCreateTempDataCopyEnabled() {
				e.removeCopy(filePath)
			}
			return fmt.Errorf("failed to init manager from file: %+v", err)
		}
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

// publishedDate is the published date of the loaded data file, or the zero
// time before a data file is loaded.
func (e *Engine) publishedDate() time.Time {
	if e.manager == nil {
		return time.Time{}
	}
	return e.getPublishedDate()
}

// nextUpdateDate is the date the loaded data file expects an update after, or
// the zero time before a data file is loaded.
func (e *Engine) nextUpdateDate() time.Time {
	if e.manager == nil {
		return time.Time{}
	}
	return ipi_interop.GetNextUpdateDate(e.manager)
}

// NewResultsIpi creates a new ResultsIpi object using this engine's manager
// Caller is responsible for calling Free() on the returned object
func (e *Engine) NewResultsIpi() *ipi_interop.ResultsIpi {
	return ipi_interop.NewResultsIpi(e.manager)
}

// Process processes the given IP address and retrieves associated values using the default properties.
// If results is nil, creates a new ResultsIpi object for this call (per-call mode).
// If results is provided, reuses the existing object (reuse mode for better performance).
func (e *Engine) Process(ipAddress string) (ipi_interop.Values, error) {
	return e.ProcessWithResults(ipAddress, nil)
}

// ProcessWithResults processes the given IP address with an optional reusable ResultsIpi object.
// If results is nil, creates a new ResultsIpi object for this call.
// If results is provided, reuses it for better performance (caller manages lifecycle).
func (e *Engine) ProcessWithResults(ipAddress string, results *ipi_interop.ResultsIpi) (ipi_interop.Values, error) {
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
		// The values are those of every required property of the results' own
		// data set. The names read at the last load are used only for results
		// from that data set, so results made before a reload, or reused across
		// one, are named from their own data set instead.
		var resolver func(int) string
		if names := e.names.Load().For(results); names != nil {
			resolver = func(index int) string {
				if index >= 0 && index < len(names) {
					return names[index]
				}
				return ""
			}
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
// property index in the data set loaded last, or an empty string for an index
// outside its required properties.
func (e *Engine) GetPropertyNameByIndex(index int) string {
	return e.names.Load().Name(index)
}
