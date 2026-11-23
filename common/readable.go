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

import (
	"os"
	"time"
)

// How long a check the file watcher starts waits for the watched data file
// to become readable, and how often it tries to open it, because another
// process can still hold a file it is writing.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md?plain=1#L512
const (
	WatchedFileReadableWaitMinutes   = 10
	watchedFileOpenRetryMilliseconds = 200
)

// OpenForReading opens the file at path for reading and closes it again, and
// gives the error when it cannot be opened.
func OpenForReading(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return f.Close()
}

// WaitUntilReadable tries to open the file at path with open until it can,
// for wait at most, trying again every 200 ms, and gives up early once
// stopped says so. It says whether the file could be opened.
func WaitUntilReadable(
	open func(path string) error,
	path string,
	wait time.Duration,
	stopped func() bool) bool {
	deadline := time.Now().Add(wait)
	for {
		if open(path) == nil {
			return true
		}
		if stopped() || !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(watchedFileOpenRetryMilliseconds * time.Millisecond)
	}
}
