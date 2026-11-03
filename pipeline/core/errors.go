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

package core

import (
	"errors"
	"fmt"
	"strings"
)

// This file defines the custom error types used throughout the Pipeline API.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/exception-handling.md

// ConfigurationError indicates that something in the provided configuration is
// preventing the creation or execution of a Pipeline or Element. It is the
// analogue of PipelineConfigurationException in the reference implementations.
type ConfigurationError struct {
	// Message describes what is wrong with the configuration.
	Message string
	// Cause is the underlying error that triggered this one, if any.
	Cause error
}

// Error returns the human-readable error message, including the cause if present.
func (e *ConfigurationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("pipeline configuration error: %s: %v", e.Message, e.Cause)
	}
	return "pipeline configuration error: " + e.Message
}

// Unwrap returns the underlying cause, enabling errors.Is/As traversal.
func (e *ConfigurationError) Unwrap() error { return e.Cause }

// NewConfigurationError creates a ConfigurationError, optionally wrapping a cause.
func NewConfigurationError(message string, cause error) *ConfigurationError {
	return &ConfigurationError{Message: message, Cause: cause}
}

// DataError indicates the API was used incorrectly with respect to Flow Data,
// for example accessing results before Process has been called, or requesting
// an element's data that is not part of the Pipeline. It is the analogue of
// PipelineDataException.
type DataError struct {
	// Message describes how the Flow Data API was misused.
	Message string
}

// Error returns the human-readable error message.
func (e *DataError) Error() string { return "pipeline data error: " + e.Message }

// NewDataError creates a DataError.
func NewDataError(message string) *DataError { return &DataError{Message: message} }

// NoValueError is thrown when accessing a Property value that an Engine has
// chosen not to set ('no value'). The message explains why the value is absent.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/properties.md#null-values
type NoValueError struct {
	// Message explains why the value is absent.
	Message string
	// Cause is the underlying error that prevented the value being set, if any.
	Cause error
}

// Error returns the explanation of why the value is absent.
func (e *NoValueError) Error() string { return e.Message }

// Unwrap returns the underlying cause, enabling errors.Is/As traversal.
func (e *NoValueError) Unwrap() error { return e.Cause }

// NewNoValueError creates a NoValueError.
func NewNoValueError(message string, cause error) *NoValueError {
	return &NoValueError{Message: message, Cause: cause}
}

// PropertyMissingError is thrown when a Property that is defined for an Element
// is not present in the result set. The message explains why.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/properties.md#missing-properties
type PropertyMissingError struct {
	// PropertyName is the name of the Property that could not be found.
	PropertyName string
	// ElementName is the data key of the Element the Property belongs to.
	ElementName string
	// Message explains why the Property is missing.
	Message string
	// Reason is a short machine-readable code for why the Property is missing
	// (for example "datafiletier" or "cloudrequestfailed"), so a caller can
	// branch on it without parsing Message. Empty when unspecified.
	Reason string
}

// Error returns the explanation of why the Property is missing.
func (e *PropertyMissingError) Error() string { return e.Message }

// ProcessError aggregates the errors that occurred while a Pipeline processed a
// Flow Data instance. It is returned from Process when exceptions are not
// suppressed. Use errors.Is/As against the wrapped element errors.
type ProcessError struct {
	// Errors holds one entry per Flow Element error encountered during processing.
	Errors []FlowError
}

// Error returns a combined message describing all the errors that occurred.
func (e *ProcessError) Error() string {
	switch len(e.Errors) {
	case 0:
		return "error processing flow data"
	case 1:
		return fmt.Sprintf("error processing flow data: %v", e.Errors[0].Err)
	default:
		parts := make([]string, len(e.Errors))
		for i, fe := range e.Errors {
			parts[i] = fe.Err.Error()
		}
		return fmt.Sprintf("%d errors processing flow data: %s", len(e.Errors), strings.Join(parts, "; "))
	}
}

// Unwrap exposes the underlying errors so errors.Is/As can match against them.
// It returns nil when there are no wrapped errors.
func (e *ProcessError) Unwrap() []error {
	if len(e.Errors) == 0 {
		return nil
	}
	out := make([]error, len(e.Errors))
	for i, fe := range e.Errors {
		out[i] = fe.Err
	}
	return out
}

// FlowError associates an error with the Flow Element that produced it (which
// may be nil for errors not attributable to a specific element).
type FlowError struct {
	// Element is the Flow Element that produced the error. It may be nil for
	// errors not attributable to a specific element.
	Element FlowElement
	// Err is the error that occurred.
	Err error
}

// Error implements the error interface so a FlowError can be used directly.
func (e FlowError) Error() string {
	if e.Element != nil {
		return fmt.Sprintf("error in element '%s': %v", e.Element.DataKey(), e.Err)
	}
	return fmt.Sprintf("%v", e.Err)
}

// Unwrap exposes the wrapped error so errors.Is/As can match against it.
func (e FlowError) Unwrap() error { return e.Err }

// Common sentinel errors.
var (
	// ErrAlreadyProcessed is returned when Process is called more than once on a
	// Flow Data instance. A Flow Data can only be processed once.
	ErrAlreadyProcessed = errors.New("flow data has already been processed")
	// ErrNotProcessed is returned when result data is accessed before Process has
	// been called.
	ErrNotProcessed = errors.New("flow data has not been processed yet")
	// ErrPipelineClosed is returned when an operation is attempted on a closed Pipeline.
	ErrPipelineClosed = errors.New("pipeline has been closed")
)
