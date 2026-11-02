package ipi_onpremise

import (
	"os"
	"strings"
	"sync"
	"testing"

	common_go "github.com/51Degrees/common-go/v4"
	"github.com/51Degrees/ip-intelligence-go/v4/ipi_interop"
)

// testPropertyIndexer is a mock for the resultsPropertyIndexer interface that
// maps property names to fixed indexes without any CGO dependency.
type testPropertyIndexer struct {
	nameToIndex map[string]int
}

func (m *testPropertyIndexer) GetPropertyIndexByName(name string) int {
	if idx, ok := m.nameToIndex[name]; ok {
		return idx
	}
	return -1
}

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

			// Clean up
			engine.Stop()
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

func TestEngine_GetPropertyNameByIndex(t *testing.T) {
	tests := []struct {
		name     string
		index    int
		cache    map[int]string
		expected string
	}{
		{
			name:     "existing index",
			index:    0,
			cache:    map[int]string{0: "IpRangeStart", 1: "Country"},
			expected: "IpRangeStart",
		},
		{
			name:     "non-existing index",
			index:    99,
			cache:    map[int]string{0: "IpRangeStart", 1: "Country"},
			expected: "",
		},
		{
			name:     "empty cache",
			index:    0,
			cache:    map[int]string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &Engine{
				propertyNameCache: tt.cache,
			}

			result := engine.GetPropertyNameByIndex(tt.index)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestInitPropertyIndexesWithIndexer_EmptyProperties confirms that when
// managerProperties is nil or empty the engine:
//   - calls availablePropertyNamesProvider to discover all property names,
//   - populates both name caches from those names, and
//   - leaves propertyIndexes nil so that GetWeightedValuesByIndexes passes NULL
//     to the C layer (which then returns all available properties).
func TestInitPropertyIndexesWithIndexer_EmptyProperties(t *testing.T) {
	// Inject deterministic property names without requiring a real ResourceManager.
	origProvider := availablePropertyNamesProvider
	availablePropertyNamesProvider = func(_ *ipi_interop.ResourceManager) []string {
		return []string{"IpRangeStart", "Country", "City"}
	}
	defer func() { availablePropertyNamesProvider = origProvider }()

	engine := &Engine{
		managerProperties:  nil,
		propertyIndexCache: make(map[string]int),
		propertyNameCache:  make(map[int]string),
	}

	mock := &testPropertyIndexer{
		nameToIndex: map[string]int{
			"IpRangeStart": 0,
			"Country":      3,
			"City":         7,
		},
	}

	engine.initPropertyIndexesWithIndexer(mock)

	// propertyIndexes must remain nil: a nil slice causes GetWeightedValuesByIndexes
	// to pass a NULL index array to the C layer, which returns all properties.
	if engine.propertyIndexes != nil {
		t.Errorf("propertyIndexes should be nil in all-properties mode, got %v", engine.propertyIndexes)
	}

	// Every property provided by the mock dataset must be resolvable by name.
	wantNameCache := map[int]string{0: "IpRangeStart", 3: "Country", 7: "City"}
	for idx, name := range wantNameCache {
		if got := engine.GetPropertyNameByIndex(idx); got != name {
			t.Errorf("GetPropertyNameByIndex(%d) = %q, want %q", idx, got, name)
		}
	}

	// Every property must also be findable by name in the index cache.
	wantIndexCache := map[string]int{"IpRangeStart": 0, "Country": 3, "City": 7}
	for name, idx := range wantIndexCache {
		if got, ok := engine.propertyIndexCache[name]; !ok || got != idx {
			t.Errorf("propertyIndexCache[%q] = %d, want %d", name, got, idx)
		}
	}
}

// TestInitPropertyIndexesWithIndexer_ExplicitProperties confirms that when
// managerProperties is non-empty the engine builds propertyIndexes for each
// named property and populates both caches — preserving the pre-existing
// targeted-query behaviour.
func TestInitPropertyIndexesWithIndexer_ExplicitProperties(t *testing.T) {
	props := []string{"IpRangeStart", "Country"}
	engine := &Engine{
		managerProperties:  props,
		propertyIndexCache: make(map[string]int),
		propertyNameCache:  make(map[int]string),
	}

	mock := &testPropertyIndexer{
		nameToIndex: map[string]int{
			"IpRangeStart": 0,
			"Country":      3,
		},
	}

	engine.initPropertyIndexesWithIndexer(mock)

	if len(engine.propertyIndexes) != len(props) {
		t.Fatalf("expected %d propertyIndexes, got %d", len(props), len(engine.propertyIndexes))
	}
	if engine.propertyIndexes[0] != 0 {
		t.Errorf("propertyIndexes[0] = %d, want 0", engine.propertyIndexes[0])
	}
	if engine.propertyIndexes[1] != 3 {
		t.Errorf("propertyIndexes[1] = %d, want 3", engine.propertyIndexes[1])
	}

	if got := engine.GetPropertyNameByIndex(0); got != "IpRangeStart" {
		t.Errorf("GetPropertyNameByIndex(0) = %q, want IpRangeStart", got)
	}
	if got := engine.GetPropertyNameByIndex(3); got != "Country" {
		t.Errorf("GetPropertyNameByIndex(3) = %q, want Country", got)
	}
	if got, ok := engine.propertyIndexCache["IpRangeStart"]; !ok || got != 0 {
		t.Errorf("propertyIndexCache[IpRangeStart] = %d, want 0", got)
	}
}

// TestAvailablePropertyNamesProviderCalledForEmpty verifies that
// availablePropertyNamesProvider is invoked when managerProperties is empty,
// and is not invoked when an explicit list is provided.
func TestAvailablePropertyNamesProviderCalledForEmpty(t *testing.T) {
	t.Run("called when empty", func(t *testing.T) {
		called := false
		orig := availablePropertyNamesProvider
		availablePropertyNamesProvider = func(_ *ipi_interop.ResourceManager) []string {
			called = true
			return []string{"TestProp"}
		}
		defer func() { availablePropertyNamesProvider = orig }()

		engine := &Engine{
			managerProperties:  nil,
			propertyIndexCache: make(map[string]int),
			propertyNameCache:  make(map[int]string),
		}
		engine.initPropertyIndexesWithIndexer(&testPropertyIndexer{
			nameToIndex: map[string]int{"TestProp": 0},
		})

		if !called {
			t.Error("availablePropertyNamesProvider should be called when managerProperties is nil")
		}
	})

	t.Run("not called when explicit", func(t *testing.T) {
		called := false
		orig := availablePropertyNamesProvider
		availablePropertyNamesProvider = func(_ *ipi_interop.ResourceManager) []string {
			called = true
			return nil
		}
		defer func() { availablePropertyNamesProvider = orig }()

		engine := &Engine{
			managerProperties:  []string{"IpRangeStart"},
			propertyIndexCache: make(map[string]int),
			propertyNameCache:  make(map[int]string),
		}
		engine.initPropertyIndexesWithIndexer(&testPropertyIndexer{
			nameToIndex: map[string]int{"IpRangeStart": 0},
		})

		if called {
			t.Error("availablePropertyNamesProvider should not be called when managerProperties is explicit")
		}
	})
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

func BenchmarkEngine_GetPropertyNameByIndex(b *testing.B) {
	engine := &Engine{
		propertyNameCache: map[int]string{
			0: "IpRangeStart",
			1: "IpRangeEnd",
			2: "Country",
			3: "City",
			4: "Latitude",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.GetPropertyNameByIndex(i % 5)
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

// Test for race conditions
func TestEngine_ConcurrentAccess(t *testing.T) {
	engine := &Engine{
		propertyNameCache: map[int]string{
			0: "IpRangeStart",
			1: "Country",
		},
		propertyIndexCache: map[string]int{
			"IpRangeStart": 0,
			"Country":      1,
		},
	}

	var wg sync.WaitGroup
	numGoroutines := 10
	numIterations := 100

	// Test concurrent reads from property caches
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
