package common_go

import "errors"

var (
	ErrNoDataFileProvided = errors.New("no data file provided")
	ErrTooManyRetries     = errors.New("too many retries to pull data file")
	ErrFileNotModified    = errors.New("data file not modified")
	ErrLicenseKeyRequired = errors.New("auto update set to true, no custom URL specified, license key is required, set it using WithLicenseKey")
)
