package ipi_onpremise

import (
	"os"
	"strings"
	"sync"
	"testing"

	common_go "github.com/51Degrees/51degrees-go/common/v4"
	"github.com/51Degrees/51degrees-go/ip-intelligence/v4/ipi_interop"
)

// Mock implementations for testing
type mockResourceManager struct {
	freed           bool
	initError       error
	reloadError     error
	reloadOrigError error
}

func (m *mockResourceManager) Free() {
	m.freed = true
}

func (m *mockResourceManager) ReloadFromFile(config ipi_interop.ConfigIpi, properties string, filePath string) error {
	return m.reloadError
}

func (m *mockResourceManager) ReloadFromOriginalFile() error {
	return m.reloadOrigError
}

type mockResultsIpi struct {
	hasValues     bool
	propertyIndex int
	processError  error
	valuesError   error
	freed         bool
	mockValues    ipi_interop.Values
}

func (m *mockResultsIpi) Free() {
	m.freed = true
}

func (m *mockResultsIpi) HasValues() bool {
	return m.hasValues
}

func (m *mockResultsIpi) ResultsIpiFromIpAddress(ip string) error {
	return m.processError
}

func (m *mockResultsIpi) GetPropertyIndexByName(name string) int {
	return m.propertyIndex
}

func (m *mockResultsIpi) GetWeightedValuesByIndexes(indexes []int, nameFunc func(int) string) (ipi_interop.Values, error) {
	return m.mockValues, m.valuesError
}

func TestNew(t *testing.T) {
	// Create temporary file for testing
	tmpFile := createTestDataFile(t)
	defer os.Remove(tmpFile)

	tests := []struct {
		name        string
		options     []EngineOptions
		expectError bool
		errorMsg    string
	}{
		{
			name:        "missing data file",
			options:     []EngineOptions{},
			expectError: true,
			errorMsg:    "no data file provided",
		},
		{
			name: "invalid data file path",
			options: []EngineOptions{
				WithDataFile("non_existent_file.dat"),
			},
			expectError: true,
			errorMsg:    "failed to get file path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := New(tt.options...)
			if engine != nil {
				defer engine.Stop()
			}

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if engine == nil {
				t.Error("Expected engine to be created")
				return
			}

			// Verify engine is properly initialized
			if engine.FileUpdater == nil {
				t.Error("FileUpdater should not be nil")
			}
			if engine.logger == nil {
				t.Error("Logger should not be nil")
			}
			if engine.stopCh == nil {
				t.Error("Stop channel should not be nil")
			}
			if engine.reloadFileEvents == nil {
				t.Error("Reload events channel should not be nil")
			}
		})
	}
}

func TestEngine_isDefaultDataFileUrl(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "default URL",
			url:      defaultDataFileUrl,
			expected: true,
		},
		{
			name:     "custom URL",
			url:      "https://example.com/data.dat",
			expected: false,
		},
		{
			name:     "empty URL matches default",
			url:      "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &Engine{
				FileUpdater: common_go.NewFileUpdater(tt.url),
			}

			result := engine.isDefaultDataFileUrl()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEngine_hasDefaultDistributorParams(t *testing.T) {
	tests := []struct {
		name       string
		licenseKey string
		expected   bool
	}{
		{
			name:       "with license key",
			licenseKey: "test-license-key",
			expected:   true,
		},
		{
			name:       "empty license key",
			licenseKey: "",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &Engine{
				licenseKey: tt.licenseKey,
			}

			result := engine.hasDefaultDistributorParams()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEngine_appendLicenceKey(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		licenseKey string
		expected   string
		expectErr  bool
	}{
		{
			name:       "append to URL without query params",
			baseURL:    "https://example.com/data",
			licenseKey: "test-key",
			expected:   "https://example.com/data?LicenseKeys=test-key",
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &Engine{
				FileUpdater: common_go.NewFileUpdater(tt.baseURL),
				licenseKey:  tt.licenseKey,
			}

			err := engine.appendLicenceKey()

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if engine.GetDataFileUrl() != tt.expected {
				t.Errorf("Expected URL %q, got %q", tt.expected, engine.GetDataFileUrl())
			}
		})
	}
}

func TestEngine_appendProduct(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		product   string
		expected  string
		expectErr bool
	}{
		{
			name:      "append product to URL",
			baseURL:   "https://example.com/data",
			product:   "test-product",
			expected:  "https://example.com/data?Product=test-product",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &Engine{
				FileUpdater: common_go.NewFileUpdater(tt.baseURL),
				product:     tt.product,
			}

			err := engine.appendProduct()

			if tt.expectErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if engine.GetDataFileUrl() != tt.expected {
				t.Errorf("Expected URL %q, got %q", tt.expected, engine.GetDataFileUrl())
			}
		})
	}
}

func TestEngine_validateAndAppendUrlParams(t *testing.T) {
	tests := []struct {
		name              string
		setupEngine       func() *Engine
		expectError       bool
		expectedErrorType string
	}{
		{
			name: "valid configuration with license key",
			setupEngine: func() *Engine {
				return &Engine{
					FileUpdater: common_go.NewFileUpdater(defaultDataFileUrl),
					licenseKey:  "test-key",
					product:     "test-product",
				}
			},
			expectError: false,
		},
		{
			name: "custom URL - no validation needed",
			setupEngine: func() *Engine {
				engine := &Engine{
					FileUpdater: common_go.NewFileUpdater("https://custom.example.com/data.dat"),
				}
				engine.SetIsAutoUpdateEnabled(true)
				return engine
			},
			expectError: false,
		},
		{
			name: "auto update disabled - no validation needed",
			setupEngine: func() *Engine {
				engine := &Engine{
					FileUpdater: common_go.NewFileUpdater(defaultDataFileUrl),
				}
				engine.SetIsAutoUpdateEnabled(false)
				return engine
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := tt.setupEngine()
			err := engine.validateAndAppendUrlParams()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.expectedErrorType != "" && !strings.Contains(err.Error(), tt.expectedErrorType) {
					t.Errorf("Expected error containing %q, got %q", tt.expectedErrorType, err.Error())
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestEngine_recoverEngine(t *testing.T) {
	engine := &Engine{
		logger:    &common_go.LogWrapper{},
		isStopped: false,
	}

	// Test that recoverEngine doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Error("recoverEngine should handle panics gracefully")
		}
	}()

	engine.recoverEngine()
}

// TestEngine_GetPropertyNameByIndex checks an engine with no data set loaded.
// TestReloadToDifferentProperties checks engines with data, across reloads.
func TestEngine_GetPropertyNameByIndex(t *testing.T) {
	engine := &Engine{}
	for _, index := range []int{-1, 0, 99} {
		if got := engine.GetPropertyNameByIndex(index); got != "" {
			t.Errorf("GetPropertyNameByIndex(%d) = %q with no data set, want \"\"",
				index, got)
		}
	}
}

func TestConstants(t *testing.T) {
	// Test that defaultDataFileUrl is properly defined
	if defaultDataFileUrl != "" {
		t.Logf("Default data file URL is set to: %s", defaultDataFileUrl)
	} else {
		t.Log("Default data file URL is empty (expected during development)")
	}
}

// Helper functions for testing

func createTestDataFile(t *testing.T) string {
	tmpFile, err := os.CreateTemp("", "test_data_*.dat")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	// Write some test data
	if _, err := tmpFile.WriteString("test data file content"); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}

	return tmpFile.Name()
}

func createTestDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "test_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	return tmpDir
}

// Benchmark tests
func BenchmarkEngine_isDefaultDataFileUrl(b *testing.B) {
	engine := &Engine{
		FileUpdater: common_go.NewFileUpdater(defaultDataFileUrl),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.isDefaultDataFileUrl()
	}
}

func BenchmarkEngine_hasDefaultDistributorParams(b *testing.B) {
	engine := &Engine{
		licenseKey: "test-license-key",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.hasDefaultDistributorParams()
	}
}

// Integration-style tests (these would require actual data files in a real scenario)
func TestEngine_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would test with an actual data file
	// For now, we'll just verify the test structure
	t.Log("Integration test placeholder - would require actual IP intelligence data file")
}

// Test for race conditions. TestLookupsDuringReloads checks lookups and names
// while reloads replace the data set.
func TestEngine_ConcurrentAccess(t *testing.T) {
	engine := &Engine{}

	var wg sync.WaitGroup
	numGoroutines := 10
	numIterations := 100

	// Test concurrent reads of the property names
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				_ = engine.GetPropertyNameByIndex(j % 2)
			}
		}()
	}

	wg.Wait()
}
