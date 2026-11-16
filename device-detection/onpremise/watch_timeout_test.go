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

package onpremise

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
)

// watchTimeoutTestWaitMilliseconds is the wait for a readable watched file
// in the test, in place of ten minutes.
const watchTimeoutTestWaitMilliseconds = 500

// A watched file that cannot be opened for reading is waited for, and when it
// cannot be opened in time the check is reported as a timeout, with nothing
// logged and the data in use kept, where the engine loaded or failed to load
// the file and logged the failure.
// See https://github.com/51Degrees/specifications/blob/706de1f0cb3edd560eb7fd7ef3bb19b93a4883dc/pipeline-specification/features/data-updates.md?plain=1#L512
func TestWatchedFileThatCannotBeOpenedTimesOut(t *testing.T) {
	good := readGoodFile(t)
	wait, open := watchedFileReadableWait, openWatchedFile
	watchedFileReadableWait = watchTimeoutTestWaitMilliseconds *
		time.Millisecond
	openWatchedFile = func(string) error {
		return errors.New("held by another process")
	}
	// Cleanups run last first, so this one runs once the engine has stopped.
	t.Cleanup(func() { watchedFileReadableWait, openWatchedFile = wait, open })

	reports := &reportLog{}
	logs := &reloadTestLog{}
	data := writeDataFile(t, good)
	e := newReloadTestEngine(t,
		WithDataFile(data),
		WithAutoUpdate(false),
		WithFileWatch(true),
		WithCustomLogger(logs),
		WithUpdateCallback(reports.add))
	reports.watch(e)
	published := e.published()

	replaceFile(t, data, markedGoodFile(t, good, published))
	waitFor(t, "the timeout to be reported", func() bool {
		return reports.count(common_go.UpdateTimeout) > 0
	})
	if n := reports.count(common_go.UpdateSuccess); n != 0 {
		t.Errorf("%d checks reported success, want the file left alone", n)
	}
	if got := e.published(); !got.Equal(published) {
		t.Errorf("the data in use was published %v, want %v as before",
			got, published)
	}
	if n := logs.count("Error during check for data update"); n != 0 {
		t.Errorf("%d failures were logged, want the timeout not logged", n)
	}
	reports.check(t, data)
}

// Stopping the engine while a check waits for a watched file to become
// readable ends the wait, so Stop does not wait the ten minutes the check
// would.
func TestStopEndsTheWaitForAWatchedFile(t *testing.T) {
	good := readGoodFile(t)
	wait, open := watchedFileReadableWait, openWatchedFile
	var tries atomic.Int32
	watchedFileReadableWait = common_go.WatchedFileReadableWaitMinutes *
		time.Minute
	openWatchedFile = func(string) error {
		tries.Add(1)
		return errors.New("held by another process")
	}
	// Cleanups run last first, so this one runs once the engine has stopped.
	t.Cleanup(func() { watchedFileReadableWait, openWatchedFile = wait, open })

	data := writeDataFile(t, good)
	e := newReloadTestEngine(t,
		WithDataFile(data),
		WithAutoUpdate(false),
		WithFileWatch(true),
		WithCustomLogger(&reloadTestLog{}))
	replaceFile(t, data, markedGoodFile(t, good, e.published()))
	waitFor(t, "the check to start waiting", func() bool {
		return tries.Load() > 0
	})
	e.stop(t)
}
