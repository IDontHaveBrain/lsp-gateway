package installer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"lsp-gateway/src/config"
)

// MockInstaller implements LanguageInstaller for testing
type MockInstaller struct {
	language         string
	installed        bool
	version          string
	installError     error
	validationError  error
	versionError     error
	uninstallError   error
	installDelay     time.Duration
	validationDelay  time.Duration
	forceReinstall   bool
	installCallCount int
	mutex            sync.Mutex
}

func NewMockInstaller(language string) *MockInstaller {
	return &MockInstaller{
		language: language,
		version:  "1.0.0",
	}
}

func (m *MockInstaller) Install(ctx context.Context, options InstallOptions) error {
	m.mutex.Lock()
	m.installCallCount++
	m.mutex.Unlock()

	if m.installDelay > 0 {
		select {
		case <-time.After(m.installDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.installError != nil {
		return m.installError
	}

	m.installed = true
	return nil
}

func (m *MockInstaller) IsInstalled() bool {
	return m.installed
}

func (m *MockInstaller) GetVersion() (string, error) {
	if m.versionError != nil {
		return "", m.versionError
	}
	if !m.installed {
		return "", errors.New("not installed")
	}
	return m.version, nil
}

func (m *MockInstaller) Uninstall() error {
	if m.uninstallError != nil {
		return m.uninstallError
	}
	m.installed = false
	return nil
}

func (m *MockInstaller) GetLanguage() string {
	return m.language
}

func (m *MockInstaller) GetServerConfig() *config.ServerConfig {
	return &config.ServerConfig{
		Command: m.language + "-lsp",
		Args:    []string{"serve"},
	}
}

func (m *MockInstaller) ValidateInstallation() error {
	if m.validationDelay > 0 {
		time.Sleep(m.validationDelay)
	}

	if m.validationError != nil {
		return m.validationError
	}

	if !m.installed {
		return errors.New("validation failed - not installed")
	}

	return nil
}

func (m *MockInstaller) SetInstalled(installed bool) {
	m.installed = installed
}

func (m *MockInstaller) SetInstallError(err error) {
	m.installError = err
}

func (m *MockInstaller) SetValidationError(err error) {
	m.validationError = err
}

func (m *MockInstaller) SetVersionError(err error) {
	m.versionError = err
}

func (m *MockInstaller) SetInstallDelay(delay time.Duration) {
	m.installDelay = delay
}

func (m *MockInstaller) GetInstallCallCount() int {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.installCallCount
}

func TestNewLSPInstallManager(t *testing.T) {
	manager := NewLSPInstallManager()
	if manager == nil {
		t.Fatal("NewLSPInstallManager returned nil")
	}

	if manager.installers == nil {
		t.Fatal("installers map not initialized")
	}

	languages := manager.GetSupportedLanguages()
	if len(languages) != 0 {
		t.Errorf("Expected 0 languages initially, got %d", len(languages))
	}
}

func TestRegisterInstaller(t *testing.T) {
	manager := NewLSPInstallManager()
	mockInstaller := NewMockInstaller("go")

	manager.RegisterInstaller("go", mockInstaller)

	languages := manager.GetSupportedLanguages()
	if len(languages) != 1 {
		t.Errorf("Expected 1 language after registration, got %d", len(languages))
	}

	if languages[0] != "go" {
		t.Errorf("Expected 'go' language, got '%s'", languages[0])
	}
}

func TestGetInstaller(t *testing.T) {
	manager := NewLSPInstallManager()
	mockInstaller := NewMockInstaller("go")
	manager.RegisterInstaller("go", mockInstaller)

	tests := []struct {
		name        string
		language    string
		expectError bool
	}{
		{
			name:        "existing installer",
			language:    "go",
			expectError: false,
		},
		{
			name:        "non-existing installer",
			language:    "python",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installer, err := manager.GetInstaller(tt.language)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if installer != nil {
					t.Error("Expected nil installer on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if installer == nil {
					t.Error("Expected installer but got nil")
				}
			}
		})
	}
}

func TestInstallLanguage(t *testing.T) {
	tests := []struct {
		name            string
		setupMock       func(*MockInstaller)
		options         InstallOptions
		expectError     bool
		expectInstalled bool
	}{
		{
			name: "successful installation",
			setupMock: func(m *MockInstaller) {
				m.SetInstalled(false)
			},
			options:         InstallOptions{},
			expectError:     false,
			expectInstalled: true,
		},
		{
			name: "already installed without force",
			setupMock: func(m *MockInstaller) {
				m.SetInstalled(true)
			},
			options:         InstallOptions{Force: false},
			expectError:     false,
			expectInstalled: true,
		},
		{
			name: "already installed with force",
			setupMock: func(m *MockInstaller) {
				m.SetInstalled(true)
			},
			options:         InstallOptions{Force: true},
			expectError:     false,
			expectInstalled: true,
		},
		{
			name: "installation failure",
			setupMock: func(m *MockInstaller) {
				m.SetInstalled(false)
				m.SetInstallError(errors.New("install failed"))
			},
			options:         InstallOptions{},
			expectError:     true,
			expectInstalled: false,
		},
		{
			name: "validation failure",
			setupMock: func(m *MockInstaller) {
				m.SetInstalled(false)
				m.SetValidationError(errors.New("validation failed"))
			},
			options:         InstallOptions{},
			expectError:     true,
			expectInstalled: true,
		},
		{
			name: "skip validation",
			setupMock: func(m *MockInstaller) {
				m.SetInstalled(false)
				m.SetValidationError(errors.New("validation failed"))
			},
			options:         InstallOptions{SkipValidation: true},
			expectError:     false,
			expectInstalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLSPInstallManager()
			mockInstaller := NewMockInstaller("go")
			tt.setupMock(mockInstaller)
			manager.RegisterInstaller("go", mockInstaller)

			ctx := context.Background()
			err := manager.InstallLanguage(ctx, "go", tt.options)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if mockInstaller.IsInstalled() != tt.expectInstalled {
				t.Errorf("Expected installed=%v, got %v", tt.expectInstalled, mockInstaller.IsInstalled())
			}
		})
	}
}

func TestInstallLanguageNonExistentLanguage(t *testing.T) {
	manager := NewLSPInstallManager()
	ctx := context.Background()

	err := manager.InstallLanguage(ctx, "nonexistent", InstallOptions{})
	if err == nil {
		t.Error("Expected error for non-existent language")
	}
}

func TestInstallAll(t *testing.T) {
	tests := []struct {
		name            string
		setupMocks      func(map[string]*MockInstaller)
		expectedSuccess int
		expectError     bool
	}{
		{
			name: "all successful",
			setupMocks: func(mocks map[string]*MockInstaller) {
				// All installers succeed by default
			},
			expectedSuccess: 3,
			expectError:     false,
		},
		{
			name: "partial failure",
			setupMocks: func(mocks map[string]*MockInstaller) {
				mocks["python"].SetInstallError(errors.New("python install failed"))
			},
			expectedSuccess: 2,
			expectError:     true,
		},
		{
			name: "all failures",
			setupMocks: func(mocks map[string]*MockInstaller) {
				for _, mock := range mocks {
					mock.SetInstallError(errors.New("install failed"))
				}
			},
			expectedSuccess: 0,
			expectError:     true,
		},
		{
			name: "validation failures",
			setupMocks: func(mocks map[string]*MockInstaller) {
				mocks["go"].SetValidationError(errors.New("validation failed"))
				mocks["rust"].SetValidationError(errors.New("validation failed"))
			},
			expectedSuccess: 1,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLSPInstallManager()
			mocks := map[string]*MockInstaller{
				"go":     NewMockInstaller("go"),
				"python": NewMockInstaller("python"),
				"rust":   NewMockInstaller("rust"),
			}

			for language, mock := range mocks {
				manager.RegisterInstaller(language, mock)
			}

			tt.setupMocks(mocks)

			ctx := context.Background()
			err := manager.InstallAll(ctx, InstallOptions{})

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			actualSuccess := 0
			for _, mock := range mocks {
				if mock.IsInstalled() {
					actualSuccess++
				}
			}

			if actualSuccess != tt.expectedSuccess {
				t.Errorf("Expected %d successful installations, got %d", tt.expectedSuccess, actualSuccess)
			}
		})
	}
}

func TestInstallAllNoInstallers(t *testing.T) {
	manager := NewLSPInstallManager()
	ctx := context.Background()

	err := manager.InstallAll(ctx, InstallOptions{})
	if err == nil {
		t.Error("Expected error when no installers registered")
	}
}

func TestInstallAllWithForce(t *testing.T) {
	manager := NewLSPInstallManager()
	mockInstaller := NewMockInstaller("go")
	mockInstaller.SetInstalled(true) // Already installed
	manager.RegisterInstaller("go", mockInstaller)

	ctx := context.Background()

	// Without force - should not reinstall
	err := manager.InstallAll(ctx, InstallOptions{Force: false})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if mockInstaller.GetInstallCallCount() != 0 {
		t.Error("Should not call Install when already installed without force")
	}

	// With force - should reinstall
	err = manager.InstallAll(ctx, InstallOptions{Force: true})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if mockInstaller.GetInstallCallCount() != 1 {
		t.Error("Should call Install when force is true")
	}
}

func TestGetStatus(t *testing.T) {
	manager := NewLSPInstallManager()

	// Setup mock installers with different states
	goMock := NewMockInstaller("go")
	goMock.SetInstalled(true)
	goMock.version = "1.19.0"

	pythonMock := NewMockInstaller("python")
	pythonMock.SetInstalled(false)

	rustMock := NewMockInstaller("rust")
	rustMock.SetInstalled(true)
	rustMock.SetVersionError(errors.New("version check failed"))

	manager.RegisterInstaller("go", goMock)
	manager.RegisterInstaller("python", pythonMock)
	manager.RegisterInstaller("rust", rustMock)

	status := manager.GetStatus()

	if len(status) != 3 {
		t.Errorf("Expected 3 status entries, got %d", len(status))
	}

	// Check Go status
	goStatus, exists := status["go"]
	if !exists {
		t.Error("Go status not found")
	} else {
		if !goStatus.Installed {
			t.Error("Go should be marked as installed")
		}
		if !goStatus.Available {
			t.Error("Go should be marked as available")
		}
		if goStatus.Version != "1.19.0" {
			t.Errorf("Expected Go version '1.19.0', got '%s'", goStatus.Version)
		}
		if goStatus.Error != nil {
			t.Errorf("Go status should not have error, got: %v", goStatus.Error)
		}
	}

	// Check Python status
	pythonStatus, exists := status["python"]
	if !exists {
		t.Error("Python status not found")
	} else {
		if pythonStatus.Installed {
			t.Error("Python should not be marked as installed")
		}
		if !pythonStatus.Available {
			t.Error("Python should be marked as available")
		}
		if pythonStatus.Version != "" {
			t.Errorf("Python version should be empty, got '%s'", pythonStatus.Version)
		}
	}

	// Check Rust status (installed but version error)
	rustStatus, exists := status["rust"]
	if !exists {
		t.Error("Rust status not found")
	} else {
		if !rustStatus.Installed {
			t.Error("Rust should be marked as installed")
		}
		if !rustStatus.Available {
			t.Error("Rust should be marked as available")
		}
		if rustStatus.Error == nil {
			t.Error("Rust status should have version error")
		}
	}
}

func TestGetSupportedLanguages(t *testing.T) {
	manager := NewLSPInstallManager()

	// Initially empty
	languages := manager.GetSupportedLanguages()
	if len(languages) != 0 {
		t.Errorf("Expected 0 languages initially, got %d", len(languages))
	}

	// Add installers
	expectedLanguages := []string{"go", "python", "rust", "typescript"}
	for _, lang := range expectedLanguages {
		manager.RegisterInstaller(lang, NewMockInstaller(lang))
	}

	languages = manager.GetSupportedLanguages()
	if len(languages) != len(expectedLanguages) {
		t.Errorf("Expected %d languages, got %d", len(expectedLanguages), len(languages))
	}

	// Check all expected languages are present
	languageMap := make(map[string]bool)
	for _, lang := range languages {
		languageMap[lang] = true
	}

	for _, expected := range expectedLanguages {
		if !languageMap[expected] {
			t.Errorf("Expected language '%s' not found in supported languages", expected)
		}
	}
}

func TestConcurrentOperations(t *testing.T) {
	manager := NewLSPInstallManager()

	// Test concurrent installer registration
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			language := "lang" + string(rune('0'+index))
			installer := NewMockInstaller(language)
			manager.RegisterInstaller(language, installer)
		}(i)
	}

	wg.Wait()

	languages := manager.GetSupportedLanguages()
	if len(languages) != numGoroutines {
		t.Errorf("Expected %d languages after concurrent registration, got %d", numGoroutines, len(languages))
	}

	// Test concurrent status access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := manager.GetStatus()
			if len(status) != numGoroutines {
				t.Errorf("Status check failed during concurrent access")
			}
		}()
	}

	wg.Wait()
}

func TestContextCancellation(t *testing.T) {
	manager := NewLSPInstallManager()
	mockInstaller := NewMockInstaller("go")
	mockInstaller.SetInstallDelay(2 * time.Second) // Long delay to test cancellation
	manager.RegisterInstaller("go", mockInstaller)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := manager.InstallLanguage(ctx, "go", InstallOptions{})
	if err == nil {
		t.Error("Expected context timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && err != context.DeadlineExceeded {
		t.Errorf("Expected context deadline exceeded error, got: %v", err)
	}
}

func TestInstallAllContextCancellation(t *testing.T) {
	manager := NewLSPInstallManager()

	// Add multiple installers with delays
	for _, lang := range []string{"go", "python", "rust"} {
		mockInstaller := NewMockInstaller(lang)
		mockInstaller.SetInstallDelay(1 * time.Second)
		manager.RegisterInstaller(lang, mockInstaller)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := manager.InstallAll(ctx, InstallOptions{})
	if err == nil {
		t.Error("Expected context timeout error during InstallAll")
	}
}

func TestInstallLanguageConcurrentSafety(t *testing.T) {
	manager := NewLSPInstallManager()
	mockInstaller := NewMockInstaller("go")
	manager.RegisterInstaller("go", mockInstaller)

	var wg sync.WaitGroup
	numGoroutines := 5
	ctx := context.Background()

	// Concurrent installations of the same language
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = manager.InstallLanguage(ctx, "go", InstallOptions{Force: true})
		}()
	}

	wg.Wait()

	if !mockInstaller.IsInstalled() {
		t.Error("Installation should have succeeded")
	}

	// Should have been called multiple times due to Force flag
	if mockInstaller.GetInstallCallCount() != numGoroutines {
		t.Errorf("Expected %d install calls, got %d", numGoroutines, mockInstaller.GetInstallCallCount())
	}
}

func TestErrorAggregation(t *testing.T) {
	manager := NewLSPInstallManager()

	// Setup installers with specific error types
	goMock := NewMockInstaller("go")
	goMock.SetInstallError(errors.New("go installation failed"))

	pythonMock := NewMockInstaller("python")
	pythonMock.SetValidationError(errors.New("python validation failed"))

	rustMock := NewMockInstaller("rust")
	// Rust will succeed

	manager.RegisterInstaller("go", goMock)
	manager.RegisterInstaller("python", pythonMock)
	manager.RegisterInstaller("rust", rustMock)

	ctx := context.Background()
	err := manager.InstallAll(ctx, InstallOptions{})

	if err == nil {
		t.Error("Expected error due to failed installations")
	}

	// Check error message contains information about failures
	errorMsg := err.Error()
	if !contains(errorMsg, "failed") {
		t.Errorf("Error message should mention failures: %s", errorMsg)
	}

	// Verify that Rust succeeded despite other failures
	if !rustMock.IsInstalled() {
		t.Error("Rust should have been installed successfully")
	}

	// Verify Go and Python failed
	if goMock.IsInstalled() {
		t.Error("Go should not have been installed due to error")
	}
	if pythonMock.IsInstalled() {
		t.Error("Python should have been installed but failed validation")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
