// Package onpremise is the on-premise Device Detection engine and User-Agent
// parser for Go. It reads a 51Degrees Hash data file on your server to detect
// the device type and model, browser, operating system, and crawlers from the
// User-Agent and User-Agent Client Hints. An alternative to UAParser.js,
// DeviceAtlas, and WURFL.
package onpremise

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
	"github.com/51Degrees/51degrees-go/device-detection/v4/dd"
)

// Engine is an implementation of the on-premise (based on a local data file) device detection. It encapsulates
// the automatic data file updates feature - to periodically fetch and reload the new data file.
// File system watcher feature allows to monitor for changes to the local data file and reload it when it changes.
// Custom URL can be used to fetch data files, the polling interval is configurable
// The 51degrees distributor service can also be used with a licenseKey
// For more information see With... options and examples
type Engine struct {
	*common_go.FileUpdater
	logger *common_go.LogWrapper

	manager *dd.ResourceManager
	config  *dd.ConfigHash

	stopCh           chan *sync.WaitGroup
	reloadFileEvents chan struct{}
	// reloadLoop is done once reloadFileEvent has returned.
	reloadLoop sync.WaitGroup

	licenseKey                string
	product                   string
	dataFileLastUsedByManager string
	managerProperties         string

	maxRetries int

	isStopped bool
}

const (
	defaultDataFileUrl = "https://distributor.51degrees.com/api/v2/download?Type=HashV41&Download=True&Product=V4TAC"

	// engineTypeName names the engine in the data update log messages.
	engineTypeName = "DeviceDetectionHashEngine"
)

// New creates an instance of the on-premise device detection engine.  WithDataFile must be provided
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
		managerProperties: "",
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

// handleFileExternallyChanged handles events where a file has been externally modified, ensuring proper processing and logging.
func (e *Engine) handleFileExternallyChanged() {
	if err := e.processFileExternallyChanged(); err != nil {
		e.logger.Errorf("An error occurred while applying a data file update to %s. "+
			"Error detail: %v", engineTypeName, err)
	}

	e.IncreaseFileExternallyChangedCount()
}

// run starts the engine
func (e *Engine) run() error {
	e.recoverEngine()

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

// recoverEngine handles recovery from a panic during file pulling, logs the error, and restarts the file pulling process if needed.
func (e *Engine) recoverEngine() {
	// recover from panic
	// if panic occurs, we will log the error and restart the file pulling
	defer func() {
		if r := recover(); r != nil {
			e.logger.Printf("error occurred when pulling data: %v", r)
			if !e.isStopped {
				go e.ScheduleFilePulling(e.stopCh, e.reloadFileEvents)
			}
		}
	}()
}

// reloadFileEvent reloads the data file each time the file puller has pulled a
// new one, until the engine stops. A failed reload is logged and the engine
// keeps the data it has, so a later file is still loaded.
func (e *Engine) reloadFileEvent() {
	defer e.reloadLoop.Done()
	for range e.reloadFileEvents {
		if err := e.processFileExternallyChanged(); err != nil {
			e.logger.Errorf("An error occurred while applying a data file update to %s. "+
				"Error detail: %v", engineTypeName, err)
		}
	}
}

// validateAndAppendUrlParams validates and appends necessary URL parameters based on the engine's configuration and state.
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

// appendProduct appends a "Product" query parameter to the Engine's data file URL using the provided product value.
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

// isDefaultDataFileUrl checks if the engine's data file URL matches the predefined default data file URL.
func (e *Engine) isDefaultDataFileUrl() bool {
	return e.GetDataFileUrl() == defaultDataFileUrl
}

// hasDefaultDistributorParams checks whether the engine has default distributor parameters set via the license key.
func (e *Engine) hasDefaultDistributorParams() bool {
	return len(e.licenseKey) > 0
}

// processFileExternallyChanged handles updates when a file is changed externally by reloading its content.
func (e *Engine) processFileExternallyChanged() error {
	reloadFilePath, err := e.GetReloadFilePath()
	if err != nil {
		return err
	}

	if err := e.reloadManager(reloadFilePath); err != nil {
		return err
	}

	return nil
}

// this function will be called when the engine is started or the is new file available
// it will create and initialize a new manager from the new file if it does not exist
// if the manager exists, it will create a new manager from the new file and replace the existing manager thus freeing memory of the old manager
func (e *Engine) reloadManager(filePath string) (err error) {
	if e.isStopped {
		return nil
	}
	// Only a load that worked is logged, as a manager whose first load failed
	// has no data set to read the published date from.
	defer func() {
		if err != nil {
			return
		}
		year, month, day := e.getPublishedDate().Date()
		e.logger.Printf("data file loaded from " + filePath + " published on: " + fmt.Sprintf("%d-%d-%d", year, month, day))
	}()

	if e.manager == nil {
		e.manager = dd.NewResourceManager()
		// init manager from file
		if e.config == nil {
			e.config = dd.NewConfigHash(dd.Balanced)
		}
		err = dd.InitManagerFromFile(e.manager, *e.config, e.managerProperties, filePath)
		if err != nil {
			if e.IsCreateTempDataCopyEnabled() {
				e.removeCopy(filePath)
			}
			return fmt.Errorf("failed to init manager from file: %w", err)
		}
		e.dataFileLastUsedByManager = filePath
		// return nil is created for the first time
		return nil
	} else if !e.IsCreateTempDataCopyEnabled() {
		e.logger.Infof("Attempting to refresh Engine '%s' with new data", engineTypeName)
		err = e.manager.ReloadFromOriginalFile()
		if err != nil {
			return fmt.Errorf("failed to reload manager from original file: %w", err)
		}
		return nil
	}

	e.logger.Infof("Attempting to refresh Engine '%s' with new data", engineTypeName)
	err = e.manager.ReloadFromFile(filePath)
	if err != nil {
		// The data set in use is kept, so the copy made for this reload is
		// not needed.
		e.removeCopy(filePath)
		return fmt.Errorf("failed to reload manager from file: %w", err)
	}

	e.removeCopy(e.dataFileLastUsedByManager)
	e.dataFileLastUsedByManager = filePath

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

// getPublishedDate retrieves the published date of the data file used by the engine.
func (e *Engine) getPublishedDate() time.Time {
	return dd.GetPublishedDate(e.manager)
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
	return dd.GetNextUpdateDate(e.manager)
}

// Create a new raw ResultsHash object.
func (e *Engine) NewResultsHash(evidenceCapacity uint32, overridesCapacity uint32) *dd.ResultsHash {
	return dd.NewResultsHash(e.manager, evidenceCapacity, overridesCapacity)
}

// Process detects the device from the provided evidence list
// returns the dd.ResultsHash object from which various device properties
// are retrieved
func (e *Engine) Process(evidenceList []Evidence) (*dd.ResultsHash, error) {
	evidenceHash, err := mapEvidence(evidenceList)
	if err != nil {
		return nil, err
	}
	defer evidenceHash.Free()

	results := dd.NewResultsHash(e.manager, uint32(evidenceHash.Count()), 0)
	err = results.MatchEvidence(evidenceHash)
	if err != nil {
		// Unfreed results panic in their finalizer.
		results.Free()
		return nil, fmt.Errorf("failed to match evidence: %w", err)
	}

	return results, nil
}

// GetHttpHeaderKeys returns all HTTP headers that can be used as evidence for device detection
func (e *Engine) GetHttpHeaderKeys() []dd.EvidenceKey {
	return e.manager.HttpHeaderKeys
}

// appendLicenceKey updates the data file URL by appending a license key to its query parameters and returns an error if it fails.
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

// hasSomeDistributorParams checks if the engine contains distributor-related parameters such as licenseKey or product.
func (e *Engine) hasSomeDistributorParams() bool {
	return len(e.licenseKey) > 0 || len(e.product) > 0
}

// Evidence struct encapsulates the evidence provided as input to the Process function
// Evidence is usually an HTTP header, thus Prefix would be dd.HttpHeaderString,
// but can also be a query param (dd.HttpEvidenceQuery) or a cookie (dd.HttpEvidenceCookie)
type Evidence struct {
	Prefix dd.EvidencePrefix
	Key    string
	Value  string
}

// mapEvidence converts a list of Evidence into a dd.Evidence object for further processing, handling errors if any occur.
func mapEvidence(evidenceList []Evidence) (*dd.Evidence, error) {
	evidenceHash := dd.NewEvidenceHash(uint32(len(evidenceList)))

	for _, evidence := range evidenceList {
		err := evidenceHash.Add(evidence.Prefix, evidence.Key, evidence.Value)
		if err != nil {
			evidenceHash.Free()
			return nil, fmt.Errorf("failed to add evidence: %w", err)
		}
	}

	return evidenceHash, nil
}
