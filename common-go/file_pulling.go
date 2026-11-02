package common_go

import (
	"bufio"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const retryMs = 1000

// ScheduleFilePulling schedules periodic file pulling based on specified intervals and handles stop signals gracefully.
// It optionally performs an immediate pull on startup if the update-on-start feature is enabled.
func (e *FileUpdater) ScheduleFilePulling(stopCh chan *sync.WaitGroup, reloadFileEvents chan struct{}) {
	nextIterationInMs := e.dataFilePullEveryMs

	// if update on start is enabled, perform the pull immediately
	if e.isUpdateOnStartEnabled {
		e.logger.Printf("Doing pull on start")
		nextIterationInMs = e.doFilePulling(reloadFileEvents)
	}

	for {
		select {
		case wg := <-stopCh:
			wg.Done()
			return
		// interval to perform the pull of updated data
		case <-time.After(time.Duration(nextIterationInMs+e.randomization) * time.Millisecond):
			nextIterationInMs = e.doFilePulling(reloadFileEvents)
		}
	}
}

// doFilePulling handles the process of pulling a data file, saving it locally, and triggering reload events if necessary.
// It checks for file modifications, makes an HTTP request to fetch the file, and handles retry logic for failures.
// The method returns the time in milliseconds before the next pull attempt.
func (e *FileUpdater) doFilePulling(reloadFileEvents chan struct{}) int {
	e.logger.Printf("Pulling data from %s", e.GetDataFileUrl())

	var lastModificationTimestamp *time.Time

	if len(e.getFilePath()) > 0 {
		file, err := os.Stat(e.GetDataFile())
		if err != nil {
			e.logger.Printf("failed to get file info: %v", err)
			// retry after 1 second, since we have unhandled error
			// this can happen from file info error or something else
			return retryMs
		}
		modTime := file.ModTime().UTC()
		lastModificationTimestamp = &modTime
	}

	// The download goes to a temp file next to the data file and is renamed over it, so the
	// engine only ever sees the complete old file or the complete new one.
	fileResponse, err := doDataFileRequest(e.dataFileUrl, e.logger, lastModificationTimestamp, e.GetDataFile())
	if err != nil {
		e.logger.Printf("failed to pull data file: %v", err)
		if errors.Is(err, ErrFileNotModified) {
			e.logger.Printf("skipping pull, file not modified")
			return e.GetDataFilePullEveryMs()
		} else if fileResponse != nil && fileResponse.retryAfter > 0 {
			e.logger.Printf("received retry-after, retrying after %d seconds", fileResponse.retryAfter)
			// retry after the specified time
			return fileResponse.retryAfter * 1000
		} else {
			e.logger.Printf("retrying after 1 second")
			// retry after 1 second, since we have unhandled error
			// this can happen from network stutter or something else
			return retryMs
		}
	}

	e.logger.Printf("data file pulled successfully: %d bytes", fileResponse.size)

	err = os.Rename(fileResponse.path, e.GetDataFile())
	if err != nil {
		os.Remove(fileResponse.path)
		e.logger.Printf("failed to write data file: %v", err)
		// retry after 1 second, since we have unhandled error
		// this can happen from disk write error or something else
		return retryMs
	}
	e.logger.Printf("data file written successfully: %d bytes", fileResponse.size)

	if !e.isFileWatcherEnabled {
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
	return e.dataFilePullEveryMs
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

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if timestamp != nil {
		req.Header.Set("If-Modified-Since", timestamp.Format(http.TimeFormat))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
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
			return nil, fmt.Errorf("failed to parse Retry-After header: %v", err)
		}

		return &FileResponse{
			retryAfter: retryAfterSeconds,
		}, fmt.Errorf("received 429, retrying after %d seconds", retryAfterSeconds)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to pull data file: %s", resp.Status)
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
			return nil, fmt.Errorf("failed to decompress file: %v", err)
		}
	}

	tempFile, err := os.CreateTemp(filepath.Dir(dataFile), tempFilePattern(dataFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	size, err := writeDataFile(tempFile, fileBytes, body, dataFile)
	if err == nil {
		contentMD5 := resp.Header.Get("Content-MD5")
		if len(contentMD5) > 0 {
			if hex.EncodeToString(hash.Sum(nil)) != contentMD5 {
				err = errors.New("MD5 validation failed")
			}
		} else {
			logger.Printf("MD5 header not found in response, skipping validation")
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
		return 0, fmt.Errorf("failed to write temp file: %v", err)
	}
	// the gzip reader stops before any trailing bytes
	if _, err = io.Copy(io.Discard, body); err != nil {
		tempFile.Close()
		return 0, fmt.Errorf("failed to read response: %v", err)
	}
	// a rename after a power loss must not leave an empty data file behind
	if err = tempFile.Sync(); err != nil {
		tempFile.Close()
		return 0, fmt.Errorf("failed to flush temp file: %v", err)
	}
	if err = tempFile.Close(); err != nil {
		return 0, fmt.Errorf("failed to close temp file: %v", err)
	}
	mode := os.FileMode(0644)
	if info, statErr := os.Stat(dataFile); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err = os.Chmod(tempFile.Name(), mode); err != nil {
		return 0, fmt.Errorf("failed to set temp file permissions: %v", err)
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
