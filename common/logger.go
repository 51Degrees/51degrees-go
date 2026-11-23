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

// LevelLogWriter can be implemented by a LogWriter that has log levels. The
// data update messages are written through it when it is present.
type LevelLogWriter interface {
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

// Infof logs an information message.
func (l LogWrapper) Infof(format string, v ...interface{}) {
	l.leveled("INFO", format, v, func(w LevelLogWriter) func(string, ...interface{}) { return w.Infof })
}

// Warnf logs a warning message.
func (l LogWrapper) Warnf(format string, v ...interface{}) {
	l.leveled("WARN", format, v, func(w LevelLogWriter) func(string, ...interface{}) { return w.Warnf })
}

// Errorf logs an error message.
func (l LogWrapper) Errorf(format string, v ...interface{}) {
	l.leveled("ERROR", format, v, func(w LevelLogWriter) func(string, ...interface{}) { return w.Errorf })
}

// leveled writes through the level method of a LevelLogWriter, or through
// Printf with the level name in front when the writer has no levels.
func (l LogWrapper) leveled(
	level string,
	format string,
	v []interface{},
	pick func(LevelLogWriter) func(string, ...interface{}),
) {
	if !l.enabled {
		return
	}
	if w, ok := l.logger.(LevelLogWriter); ok {
		pick(w)(format, v...)
		return
	}
	l.logger.Printf(level+" "+format, v...)
}
