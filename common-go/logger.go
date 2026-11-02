package common_go

import (
	"log"
	"os"
)

// LogWriter is an interface that defines a logging mechanism with formatted output through the Printf method.
type LogWriter interface {
	Printf(format string, v ...interface{})
}

var DefaultLogger LogWriter = log.New(os.Stdout, "\r\n", log.LstdFlags)

// LogWrapper provides a logger struct with an enabled state and a logger implementing the LogWriter interface.
type LogWrapper struct {
	enabled bool
	logger  LogWriter
}

// Printf logs a formatted message using the underlying logger if logging is enabled.
func (l LogWrapper) Printf(format string, v ...interface{}) {
	if !l.enabled {
		return
	}
	l.logger.Printf(format, v...)
}
