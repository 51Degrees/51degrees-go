package requestengine

import (
	"fmt"
	"time"
)

// CloudRequestError is returned following failed requests to the cloud service.
// It carries the underlying cause (for example a JSON parse error) so callers
// can inspect or log it.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/features/exception-handling.md#cloud-request-exception
type CloudRequestError struct {
	Message string
	Cause   error
}

func (e *CloudRequestError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("cloud request error: %s: %v", e.Message, e.Cause)
	}
	return "cloud request error: " + e.Message
}

func (e *CloudRequestError) Unwrap() error { return e.Cause }

// NewCloudRequestError creates a CloudRequestError.
func NewCloudRequestError(message string, cause error) *CloudRequestError {
	return &CloudRequestError{Message: message, Cause: cause}
}

// TemporarilyUnavailableError is returned while the engine is in its recovery
// period, signalling that the element (and Pipeline) are temporarily
// unavailable so that requests are bypassed rather than piling up.
// See https://github.com/51Degrees/specifications/blob/main/pipeline-specification/pipeline-elements/cloud-request-engine.md#recovery-mode
type TemporarilyUnavailableError struct {
	RecoveryUntil time.Time
}

func (e *TemporarilyUnavailableError) Error() string {
	return fmt.Sprintf("cloud request engine is temporarily unavailable (recovering until %s)",
		e.RecoveryUntil.Format(time.RFC3339))
}
