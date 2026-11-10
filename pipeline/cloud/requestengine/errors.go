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

package requestengine

import (
	"fmt"
	"net/http"
	"time"
)

// CloudRequestError is returned following failed requests to the cloud service.
// It carries the underlying cause (for example a JSON parse error) so callers
// can inspect or log it, and, for a response-level failure, the HTTP status
// code and response headers so a caller can inspect them (for example a 429
// Retry-After).
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/features/exception-handling.md#cloud-request-exception
type CloudRequestError struct {
	// Message describes the failure.
	Message string
	// Cause is the underlying error (for example a JSON parse or HTTP error), if any.
	Cause error
	// StatusCode is the HTTP status code of the response, or 0 when the failure
	// was not a response (for example a transport error).
	StatusCode int
	// Headers are the response headers, or nil when not applicable.
	Headers http.Header
}

// Error returns the human-readable error message, including the cause if present.
func (e *CloudRequestError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("cloud request error: %s: %v", e.Message, e.Cause)
	}
	return "cloud request error: " + e.Message
}

// Unwrap returns the underlying cause, enabling errors.Is/As traversal.
func (e *CloudRequestError) Unwrap() error { return e.Cause }

// NewCloudRequestError creates a CloudRequestError.
func NewCloudRequestError(message string, cause error) *CloudRequestError {
	return &CloudRequestError{Message: message, Cause: cause}
}

// NewCloudResponseError creates a CloudRequestError for a response-level failure,
// capturing the HTTP status code and response headers so a caller can inspect
// them (for example a 429 with a Retry-After header).
func NewCloudResponseError(message string, status int, headers http.Header) *CloudRequestError {
	return &CloudRequestError{Message: message, StatusCode: status, Headers: headers}
}

// TemporarilyUnavailableError is returned while the engine is in its recovery
// period, signalling that the element (and Pipeline) are temporarily
// unavailable so that requests are bypassed rather than piling up.
// See https://github.com/51Degrees/specifications/blob/rc/pipeline-specification/pipeline-elements/cloud-request-engine.md#recovery-mode
type TemporarilyUnavailableError struct {
	// RecoveryUntil is the time at which the engine will resume making requests.
	RecoveryUntil time.Time
}

// Error returns a message stating the engine is unavailable until recovery completes.
func (e *TemporarilyUnavailableError) Error() string {
	return fmt.Sprintf("cloud request engine is temporarily unavailable (recovering until %s)",
		e.RecoveryUntil.Format(time.RFC3339))
}
