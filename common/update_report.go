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

package common_go

import "fmt"

// UpdateStatus is how a check for an update of a data file ended. The names
// are those of the .NET data update service, so a status means the same in
// every language.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/features/data-updates.md#reporting-each-check
type UpdateStatus string

const (
	// UpdateSuccess is a check that refreshed the engine with new data.
	UpdateSuccess UpdateStatus = "AUTO_UPDATE_SUCCESS"
	// UpdateNotNeeded is a check that applied no update because none was
	// needed, such as a 304 (Not modified) response.
	UpdateNotNeeded UpdateStatus = "AUTO_UPDATE_NOT_NEEDED"
	// UpdateHTTPSError is a request to the update URL that failed, a download
	// that ended before the whole file arrived, or a response that was
	// neither a success nor a 304 (Not modified).
	UpdateHTTPSError UpdateStatus = "AUTO_UPDATE_HTTPS_ERR"
	// UpdateTooManyRequests is a 429 (Too many requests) response.
	UpdateTooManyRequests UpdateStatus = "AUTO_UPDATE_ERR_429_TOO_MANY_ATTEMPTS"
	// UpdateForbidden is a 403 (Forbidden) response.
	UpdateForbidden UpdateStatus = "AUTO_UPDATE_ERR_403_FORBIDDEN"
	// UpdateMD5ValidationFailed is downloaded data whose MD5 hash does not
	// match the one the response gave.
	UpdateMD5ValidationFailed UpdateStatus = "AUTO_UPDATE_ERR_MD5_VALIDATION_FAILED"
	// UpdateTempPathNotSet is a check with no data file path, so nowhere to
	// write the download.
	UpdateTempPathNotSet UpdateStatus = "AUTO_UPDATE_TEMP_PATH_NOT_SET"
	// UpdateCannotRename is downloaded data that could not replace the data
	// file.
	UpdateCannotRename UpdateStatus = "AUTO_UPDATE_NEW_FILE_CANT_RENAME"
	// UpdateRefreshFailed is new data the engine would not load.
	UpdateRefreshFailed UpdateStatus = "AUTO_UPDATE_REFRESH_FAILED"
	// UpdateUnknownError is a failure none of the others name.
	UpdateUnknownError UpdateStatus = "AUTO_UPDATE_UNKNOWN_ERROR"
)

// UpdateComplete is the report a check for an update makes once it ends.
type UpdateComplete struct {
	// DataFile is the path of the data file the check was for.
	DataFile string
	// Status is how the check ended.
	Status UpdateStatus
}

// OnUpdateComplete registers a callback that each check for an update of the
// data file reports to once it ends: the update on start-up, each pull, and
// each change the file watcher sees. A check that finds new data ends once
// the engine has loaded it or failed to, so the engine reports that one.
// Register before the engine starts to hear the update on start-up.
func (e *FileUpdater) OnUpdateComplete(fn func(UpdateComplete)) {
	e.callbackMu.Lock()
	defer e.callbackMu.Unlock()
	e.onUpdateComplete = append(e.onUpdateComplete, fn)
}

// ReportUpdateComplete tells each callback registered with OnUpdateComplete
// that a check ended with the status given. It is for the engine, which
// reports the checks that end with its load. A callback that panics stops the
// callbacks after it, and the panic is returned as an error.
func (e *FileUpdater) ReportUpdateComplete(status UpdateStatus) error {
	e.callbackMu.Lock()
	callbacks := e.onUpdateComplete
	e.callbackMu.Unlock()
	complete := UpdateComplete{DataFile: e.GetDataFile(), Status: status}
	for _, fn := range callbacks {
		if err := callUpdateCallback(fn, complete); err != nil {
			return err
		}
	}
	return nil
}

// callUpdateCallback calls fn, returning the panic it raised as an error.
func callUpdateCallback(fn func(UpdateComplete), complete UpdateComplete) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &UpdateCallbackPanic{Value: r}
		}
	}()
	fn(complete)
	return nil
}

// UpdateCallbackPanic is a panic an update callback raised, as an error.
type UpdateCallbackPanic struct {
	// Value is the value the callback panicked with.
	Value any
}

func (p *UpdateCallbackPanic) Error() string {
	return fmt.Sprintf("panic in an update callback: %v", p.Value)
}
