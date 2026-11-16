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

package ipi_onpremise

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

type EngineOptions func(cfg *Engine) error

// WithDataFile sets the path to the local data file, this parameter is required to start the engine.
// The file must exist unless WithUpdateOnStart is enabled, which downloads it before the engine loads it.
func WithDataFile(path string) EngineOptions {
	return func(cfg *Engine) error {
		cfg.SetDataFile(filepath.Join(path))
		return nil
	}
}

// WithConfigIpi allows to configure the Ipi matching algorithm.
// See ipi_interopt.ConfigIpi type for all available settings:
// PerformanceProfile, Drift, Difference, Concurrency
// By default initialized with ipi_interopt.Balanced performance profile
// ipi_interopt.NewConfigIpi(ipi_interopt.Balanced)
//
// It cannot be used with WithPerformanceProfile, because the configuration
// given here already has a performance profile.
func WithConfigIpi(configIpi *ipi_interop.ConfigIpi) EngineOptions {
	return func(cfg *Engine) error {
		if configIpi != nil && cfg.profile != nil {
			return errProfileAndConfig
		}
		cfg.config = configIpi
		return nil
	}
}

// errProfileAndConfig refuses a Performance Profile given two ways.
var errProfileAndConfig = errors.New("WithPerformanceProfile and " +
	"WithConfigIpi were both given, and each sets the performance " +
	"profile, so which is meant is not clear. Give one of them. " +
	"WithConcurrency can be used with either")

// performanceProfileNames are the names a Performance Profile is given by
// in a configuration file, in lower case. MaxPerformance is the
// specification's name for the profile that holds the whole data file in
// memory, which this package calls InMemory.
var performanceProfileNames = map[string]ipi_interop.PerformanceProfile{
	"default":         ipi_interop.Default,
	"lowmemory":       ipi_interop.LowMemory,
	"balancedtemp":    ipi_interop.BalancedTemp,
	"balanced":        ipi_interop.Balanced,
	"highperformance": ipi_interop.HighPerformance,
	"maxperformance":  ipi_interop.InMemory,
	"inmemory":        ipi_interop.InMemory,
}

// performanceProfileList names the Performance Profiles in a message.
const performanceProfileList = "LowMemory, Balanced, BalancedTemp, " +
	"HighPerformance, MaxPerformance (also called InMemory) or Default"

// ParsePerformanceProfile gives the Performance Profile with the given name,
// ignoring case, or an error that lists the names when there is none.
func ParsePerformanceProfile(name string) (ipi_interop.PerformanceProfile, error) {
	profile, ok := performanceProfileNames[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return ipi_interop.Default, fmt.Errorf("'%s' is not a Performance "+
			"Profile. Use %s", name, performanceProfileList)
	}
	return profile, nil
}

// WithPerformanceProfile sets the Performance Profile the Engine is created
// with, which decides how much of the data file is held in memory. The
// default is Balanced. A value that is not a Performance Profile is refused.
// It cannot be used with WithConfigIpi.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L619
func WithPerformanceProfile(profile ipi_interop.PerformanceProfile) EngineOptions {
	return func(cfg *Engine) error {
		if profile < ipi_interop.Default || profile > ipi_interop.InMemory {
			return fmt.Errorf("%d is not a Performance Profile. Use %s",
				int(profile), performanceProfileList)
		}
		if cfg.config != nil {
			return errProfileAndConfig
		}
		cfg.profile = &profile
		return nil
	}
}

// maxConcurrency is the highest Concurrency the native code can be given.
const maxConcurrency = math.MaxUint16

// WithConcurrency sets the expected number of operations that use the Engine
// at the same time, which the native code uses for every collection it reads
// from the data file. It has no effect when the whole data file is held in
// memory. A number below 1 or above 65535 is refused.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L620
func WithConcurrency(concurrency int) EngineOptions {
	return func(cfg *Engine) error {
		if concurrency < 1 || concurrency > maxConcurrency {
			return fmt.Errorf("Concurrency is %d, and it has to be from 1 "+
				"to %d. Leave it out to use the default", concurrency,
				maxConcurrency)
		}
		value := uint16(concurrency)
		cfg.concurrency = &value
		return nil
	}
}

// WithReuseOperationalFile says whether a copy of the data file left by an
// earlier run is used again. When this Engine works from a copy it makes a
// new one each time it starts and removes it when it stops, so there is never
// an earlier copy to use again, and turning the option on is refused. Off is
// the default, and is accepted.
// See https://github.com/51Degrees/specifications/blob/dbaf099c712e2bd3a2a80bfb4eaf284b079f05eb/ip-intelligence-specification/pipeline-elements/ip-intelligence-on-premise.md?plain=1#L621
func WithReuseOperationalFile(reuse bool) EngineOptions {
	return func(cfg *Engine) error {
		if reuse {
			return errors.New("ReuseOperationalFile cannot be turned on. " +
				"When this Engine works from a copy of the data file it " +
				"makes a new one each time it starts and removes it when " +
				"it stops, so there is no earlier copy to use again. Leave " +
				"ReuseOperationalFile out or set it to false")
		}
		return nil
	}
}

// nativeConfig gives the native configuration the data set is created with.
// It is the one given with WithConfigIpi, or otherwise one for the
// Performance Profile set, which is Balanced by default. The Concurrency set
// is applied to either.
func (e *Engine) nativeConfig() *ipi_interop.ConfigIpi {
	config := e.config
	concurrency := e.concurrency
	if config == nil {
		profile := ipi_interop.Balanced
		if e.profile != nil {
			profile = *e.profile
		}
		config = ipi_interop.NewConfigIpi(profile)
		if concurrency == nil && e.defaultConcurrency != 0 {
			concurrency = &e.defaultConcurrency
		}
	}
	if concurrency != nil {
		config.SetConcurrency(*concurrency)
	}
	return config
}

// WithLicenseKey sets the license key to use when pulling the data file
// this option can only be used when using the default data file url from 51Degrees, it will be appended as a query parameter
func WithLicenseKey(key string) EngineOptions {
	return func(cfg *Engine) error {
		if !cfg.isDefaultDataFileUrl() {
			return errors.New("license key can only be set when using default data file url")
		}

		cfg.licenseKey = key
		return nil
	}
}

// WithProduct sets the product to use when pulling the data file when distributor service is used
// licenseKey has to be provided using WithLicenseKey
func WithProduct(product string) EngineOptions {
	return func(cfg *Engine) error {
		if !cfg.isDefaultDataFileUrl() {
			return errors.New("product can only be set when using default data file url")
		}

		cfg.product = product
		return nil
	}
}

// WithDataUpdateUrl sets a custom URL to download the data file from
func WithDataUpdateUrl(urlStr string) EngineOptions {
	return func(cfg *Engine) error {
		if _, err := url.ParseRequestURI(urlStr); err != nil {
			return err
		}

		cfg.SetDataFileUrl(urlStr)

		return nil
	}
}

// WithMaxRetries sets the maximum number of retries to pull the data file if request fails
func WithMaxRetries(retries int) EngineOptions {
	return func(cfg *Engine) error {
		cfg.maxRetries = retries
		return nil
	}
}

// WithPollingInterval sets the interval in seconds to pull the data file
func WithPollingInterval(seconds int) EngineOptions {
	return func(cfg *Engine) error {
		cfg.SetDataFilePullEveryMs(seconds * 1000)
		return nil
	}
}

// WithLogging enables or disables the logger
func WithLogging(enabled bool) EngineOptions {
	return func(cfg *Engine) error {
		cfg.logger = cfg.SetLoggerEnabled(enabled)

		return nil
	}
}

// WithCustomLogger sets a custom logger
func WithCustomLogger(logger common_go.LogWriter) EngineOptions {
	return func(cfg *Engine) error {
		cfg.logger = cfg.SetCustomLogger(logger)

		return nil
	}
}

// WithFileWatch enables or disables file watching in case 3rd party updates the data file
// engine will automatically reload the data file.  Default is true
func WithFileWatch(enabled bool) EngineOptions {
	return func(cfg *Engine) error {
		cfg.SetIsFileWatcherEnabled(enabled)
		return nil
	}
}

// WithUpdateOnStart enables or disables update on start
// if enabled, engine will pull the data file from the distributor (or custom URL) once initialized
// default is false
func WithUpdateOnStart(enabled bool) EngineOptions {
	return func(cfg *Engine) error {
		cfg.SetUpdateOnStartEnabled(enabled)

		return nil
	}
}

// WithUpdateCallback registers a callback that each check for an update of
// the data file reports to once it ends, with the data file's path and how
// the check ended: the update on start-up, each pull and each change the file
// watcher sees. A callback that panics is logged, and the checks carry on,
// apart from the update on start-up, where New returns the panic as an error.
// See https://github.com/51Degrees/specifications/blob/6570727d2af6967390d3c4fbec77b9cf0540d3f3/pipeline-specification/features/data-updates.md#reporting-each-check
func WithUpdateCallback(fn func(common_go.UpdateComplete)) EngineOptions {
	return func(cfg *Engine) error {
		if fn == nil {
			return errors.New("the update callback is nil")
		}
		cfg.OnUpdateComplete(fn)
		return nil
	}
}

// WithAutoUpdate enables or disables auto update
// default is true
// if enabled, engine will automatically pull the data file from the distributor or custom URL
// if disabled options like WithDataUpdateUrl, WithLicenseKey will be ignored
func WithAutoUpdate(enabled bool) EngineOptions {
	return func(cfg *Engine) error {
		cfg.SetIsAutoUpdateEnabled(enabled)

		return nil
	}
}

// WithTempDataCopy enables or disables creating a temp copy of the data file
// default is true
// if enabled, engine will create a temp copy of the data file and use it for detection rather than original data file
// if disabled, engine will use the original data file to initialize the manager
// this is useful when 3rd party updates the data file on the file system
func WithTempDataCopy(enabled bool) EngineOptions {
	return func(cfg *Engine) error {
		cfg.SetIsCreateTempDataCopyEnabled(enabled)

		return nil
	}
}

// WithTempDataDir sets the directory to store the temp data file
// default is system temp directory
func WithTempDataDir(dir string) EngineOptions {
	return func(cfg *Engine) error {
		dirFileInfo, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("failed to get file path: %w", err)
		}

		if !dirFileInfo.IsDir() {
			return fmt.Errorf("path is not a directory: %s", dir)
		}

		cfg.SetTempDataDir(dir)
		return nil
	}
}

// WithRandomization sets the maximum random time in seconds added to each
// interval between data file pulls. A new time from 0 up to this maximum is
// drawn for each interval, so that several servers with the same setting do
// not pull the data file at the same time. The default is 10 minutes, and 0
// adds nothing.
func WithRandomization(seconds int) EngineOptions {
	return func(cfg *Engine) error {
		cfg.SetRandomization(seconds * 1000)
		return nil
	}
}

// WithProperties sets the list of properties the engine will load and return.
// Passing an empty slice (or omitting this option entirely) signals the engine
// to load and return all available properties — the C library interprets an
// empty properties string as "all properties required".
func WithProperties(properties []string) EngineOptions {
	return func(cfg *Engine) error {
		if properties != nil {
			cfg.managerProperties = properties
		}
		return nil
	}
}
