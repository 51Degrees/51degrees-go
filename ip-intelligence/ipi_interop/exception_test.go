/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2019 51 Degrees Mobile Experts Limited, 5 Charlotte Close,
 * Caversham, Reading, Berkshire, United Kingdom RG4 7BY.
 *
 * This Original Work is licensed under the European Union Public Licence (EUPL)
 * v.1.2 and is subject to its terms as set out below.
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
package ipi_interop

import "testing"

func TestNewException(t *testing.T) {
	t.Run("create new exception", func(t *testing.T) {
		exception := NewException()

		if exception == nil {
			t.Fatal("NewException returned nil")
		}

		if exception.CPtr == nil {
			t.Fatal("NewException created Exception with nil CPtr")
		}

		// Verify initial state after Clear() is called by NewException
		if exception.CPtr.file != nil {
			t.Error("New exception should have nil file")
		}

		if exception.CPtr._func != nil {
			t.Error("New exception should have nil _func")
		}

		if exception.CPtr.line != -1 {
			t.Errorf("New exception should have line = -1, got %d", exception.CPtr.line)
		}

		if int(exception.CPtr.status) != StatusNotSet {
			t.Errorf("New exception should have status = STATUS_NOT_SET, got %d",
				exception.CPtr.status)
		}
	})
}

func TestException_IsOkay(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Exception)
		expected bool
	}{
		{
			name: "nil CPtr",
			setup: func(e *Exception) {
				e.CPtr = nil
			},
			expected: true,
		},
		{
			name: "status not set",
			setup: func(e *Exception) {
				e.CPtr.status = StatusNotSet
			},
			expected: true,
		},
		{
			name: "status set",
			setup: func(e *Exception) {
				e.CPtr.status = 1 // Any non-STATUS_NOT_SET value
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exception := NewException()
			if tt.setup != nil {
				tt.setup(exception)
			}

			if got := exception.IsOkay(); got != tt.expected {
				t.Errorf("IsOkay() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExceptionFullFlow(t *testing.T) {
	t.Run("exception lifecycle", func(t *testing.T) {
		// Create new exception
		exception := NewException()
		if !exception.IsOkay() {
			t.Error("New exception should be okay")
		}

		// Simulate an error condition
		exception.CPtr.status = 1 // Some error status
		if exception.IsOkay() {
			t.Error("Exception should not be okay after setting error status")
		}

		// Clear the exception
		exception.Clear()
		if !exception.IsOkay() {
			t.Error("Exception should be okay after Clear()")
		}
	})
}
