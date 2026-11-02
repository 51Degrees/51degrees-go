package common_go

import (
	"bytes"
	"log"
	"testing"
)

// mockLogger implements LogWriter interface for testing
type mockLogger struct {
	buffer bytes.Buffer
}

func (m *mockLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func TestLogWrapper_Printf(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		format  string
		args    []interface{}
		wantLog bool
	}{
		{
			name:    "enabled logger",
			enabled: true,
			format:  "test message %s",
			args:    []interface{}{"arg1"},
			wantLog: true,
		},
		{
			name:    "disabled logger",
			enabled: false,
			format:  "test message %s",
			args:    []interface{}{"arg1"},
			wantLog: false,
		},
		{
			name:    "empty message enabled",
			enabled: true,
			format:  "",
			args:    []interface{}{},
			wantLog: true,
		},
		{
			name:    "multiple arguments",
			enabled: true,
			format:  "test %s %d %f",
			args:    []interface{}{"string", 42, 3.14},
			wantLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockLogger{}
			wrapper := &LogWrapper{
				enabled: tt.enabled,
				logger:  mock,
			}

			// Call Printf
			wrapper.Printf(tt.format, tt.args...)

			// Verify logging behavior
			if tt.enabled != tt.wantLog {
				t.Errorf("Printf() logging state = %v, want %v", tt.enabled, tt.wantLog)
			}
		})
	}
}

func TestDefaultLogger(t *testing.T) {
	t.Run("default logger initialization", func(t *testing.T) {
		if DefaultLogger == nil {
			t.Error("DefaultLogger should not be nil")
		}
	})
}

func TestLogWrapper_WithCustomLogger(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() LogWriter
		wantErr bool
	}{
		{
			name: "valid custom logger",
			setup: func() LogWriter {
				return &mockLogger{}
			},
			wantErr: false,
		},
		{
			name: "nil logger",
			setup: func() LogWriter {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := tt.setup()
			wrapper := &LogWrapper{
				enabled: true,
				logger:  logger,
			}

			if tt.wantErr {
				if wrapper.logger != nil {
					t.Error("Expected nil logger, got non-nil")
				}
			} else {
				if wrapper.logger == nil && logger != nil {
					t.Error("Expected non-nil logger, got nil")
				}
			}
		})
	}
}

func TestLogWrapper_MultipleMessages(t *testing.T) {
	mock := &mockLogger{}
	wrapper := &LogWrapper{
		enabled: true,
		logger:  mock,
	}

	messages := []struct {
		format string
		args   []interface{}
	}{
		{
			format: "message 1 %s",
			args:   []interface{}{"arg1"},
		},
		{
			format: "message 2 %d",
			args:   []interface{}{42},
		},
		{
			format: "message 3 %s %d",
			args:   []interface{}{"arg3", 123},
		},
	}

	for _, msg := range messages {
		wrapper.Printf(msg.format, msg.args...)
	}
}

func TestLogWrapper_EnableDisable(t *testing.T) {
	mock := &mockLogger{}
	wrapper := &LogWrapper{
		enabled: true,
		logger:  mock,
	}

	// Test enabled state
	wrapper.Printf("test message")

	// Test disabled state
	wrapper.enabled = false
	wrapper.Printf("should not log")

	// Re-enable and test
	wrapper.enabled = true
	wrapper.Printf("should log again")
}

func TestLogWrapper_Concurrency(t *testing.T) {
	mock := &mockLogger{}
	wrapper := &LogWrapper{
		enabled: true,
		logger:  mock,
	}

	// Test concurrent logging
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			wrapper.Printf("concurrent message %d", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkLogWrapper_Printf(b *testing.B) {
	mock := &mockLogger{}
	wrapper := &LogWrapper{
		enabled: true,
		logger:  mock,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wrapper.Printf("benchmark message %d", i)
	}
}
