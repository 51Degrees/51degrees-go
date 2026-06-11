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
	Message string
	Cause   error
}

func (e *ConfigurationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("pipeline configuration error: %s: %v", e.Message, e.Cause)
	}
	return "pipeline configuration error: " + e.Message
}

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
	Message string
}

func (e *DataError) Error() string { return "pipeline data error: " + e.Message }

// NewDataError creates a DataError.
func NewDataError(message string) *DataError { return &DataError{Message: message} }

// NoValueError is thrown when accessing a Property value that an Engine has
// chosen not to set ('no value'). The message explains why the value is absent.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/properties.md#null-values
type NoValueError struct {
	Message string
	Cause   error
}

func (e *NoValueError) Error() string { return e.Message }
func (e *NoValueError) Unwrap() error { return e.Cause }

// NewNoValueError creates a NoValueError.
func NewNoValueError(message string, cause error) *NoValueError {
	return &NoValueError{Message: message, Cause: cause}
}

// PropertyMissingError is thrown when a Property that is defined for an Element
// is not present in the result set. The message explains why.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/properties.md#missing-properties
type PropertyMissingError struct {
	PropertyName string
	ElementName  string
	Message      string
}

func (e *PropertyMissingError) Error() string { return e.Message }

// ProcessError aggregates the errors that occurred while a Pipeline processed a
// Flow Data instance. It is returned from Process when exceptions are not
// suppressed. Use errors.Is/As against the wrapped element errors.
type ProcessError struct {
	Errors []FlowError
}

func (e *ProcessError) Error() string {
	if len(e.Errors) == 1 {
		return fmt.Sprintf("error processing flow data: %v", e.Errors[0].Err)
	}
	parts := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		parts[i] = fe.Err.Error()
	}
	return fmt.Sprintf("%d errors processing flow data: %s", len(e.Errors), strings.Join(parts, "; "))
}

// Unwrap exposes the underlying errors so errors.Is/As can match against them.
func (e *ProcessError) Unwrap() []error {
	out := make([]error, len(e.Errors))
	for i, fe := range e.Errors {
		out[i] = fe.Err
	}
	return out
}

// FlowError associates an error with the Flow Element that produced it (which
// may be nil for errors not attributable to a specific element).
type FlowError struct {
	Element FlowElement
	Err     error
}

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
