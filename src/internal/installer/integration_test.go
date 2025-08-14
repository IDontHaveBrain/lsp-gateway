package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
)

// mockLanguageInstaller implements LanguageInstaller for testing
type mockLanguageInstaller struct {
	language          string
	installed         bool
	installError      error
	version           string
	versionError      error
	validateError     error
	uninstallError    error
	serverConfig      *config.ServerConfig
	installCallCount  int
	validateCallCount int
	installDelay      time.Duration
	mu                sync.Mutex
}

func newMockLanguageInstaller(language string) *mockLanguageInstaller {
	return &mockLanguageInstaller{
		language: language,
		serverConfig: &config.ServerConfig{
			Command: language + "-lsp",
			Args:    []string{"--stdio"},
		},
		version: "1.0.0",
	}
}

func (m *mockLanguageInstaller) Install(ctx context.Context, options InstallOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.installCallCount++

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

func (m *mockLanguageInstaller) IsInstalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.installed
}

func (m *mockLanguageInstaller) GetVersion() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.versionError != nil {
		return "", m.versionError
	}
	return m.version, nil
}

func (m *mockLanguageInstaller) Uninstall() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.uninstallError != nil {
		return m.uninstallError
	}

	m.installed = false
	return nil
}

func (m *mockLanguageInstaller) GetLanguage() string {
	return m.language
}

func (m *mockLanguageInstaller) GetServerConfig() *config.ServerConfig {
	return m.serverConfig
}

func (m *mockLanguageInstaller) ValidateInstallation() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.validateCallCount++

	if m.validateError != nil {
		// If validation fails, mark as not installed to simulate real behavior
		// where a failed validation means the installation is not usable
		m.installed = false
		return m.validateError
	}

	if !m.installed {
		return fmt.Errorf("%s not installed", m.language)
	}

	return nil
}

func (m *mockLanguageInstaller) setInstalled(installed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.installed = installed
}

func (m *mockLanguageInstaller) setInstallError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.installError = err
}

func (m *mockLanguageInstaller) setValidateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validateError = err
}

func (m *mockLanguageInstaller) getInstallCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.installCallCount
}

func (m *mockLanguageInstaller) getValidateCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.validateCallCount
}

func TestInstallManager_InstallLanguage_Integration(t *testing.T) {
	tests := []struct {
		name               string
		language           string
		installError       error
		validateError      error
		options            InstallOptions
		expectError        bool
		expectInstallCall  bool
		expectValidateCall bool
	}{
		{
			name:               "Successful installation",
			language:           "go",
			options:            InstallOptions{},
			expectError:        false,
			expectInstallCall:  true,
			expectValidateCall: true,
		},
		{
			name:               "Installation fails",
			language:           "go",
			installError:       errors.New("install failed"),
			options:            InstallOptions{},
			expectError:        true,
			expectInstallCall:  true,
			expectValidateCall: false,
		},
		{
			name:               "Installation succeeds but validation fails",
			language:           "go",
			validateError:      errors.New("validation failed"),
			options:            InstallOptions{},
			expectError:        true,
			expectInstallCall:  true,
			expectValidateCall: true,
		},
		{
			name:               "Skip validation",
			language:           "go",
			validateError:      errors.New("validation would fail"),
			options:            InstallOptions{SkipValidation: true},
			expectError:        false,
			expectInstallCall:  true,
			expectValidateCall: false,
		},
		{
			name:               "Force reinstall",
			language:           "go",
			options:            InstallOptions{Force: true},
			expectError:        false,
			expectInstallCall:  true,
			expectValidateCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLSPInstallManager()
			installer := newMockLanguageInstaller(tt.language)

			if tt.installError != nil {
				installer.setInstallError(tt.installError)
			}
			if tt.validateError != nil {
				installer.setValidateError(tt.validateError)
			}

			manager.RegisterInstaller(tt.language, installer)

			ctx, cancel := common.CreateContext(5 * time.Second)
			defer cancel()

			err := manager.InstallLanguage(ctx, tt.language, tt.options)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectInstallCall && installer.getInstallCallCount() == 0 {
				t.Error("Expected install call but it wasn't made")
			}
			if !tt.expectInstallCall && installer.getInstallCallCount() > 0 {
				t.Error("Unexpected install call made")
			}

			if tt.expectValidateCall && installer.getValidateCallCount() == 0 {
				t.Error("Expected validate call but it wasn't made")
			}
			if !tt.expectValidateCall && installer.getValidateCallCount() > 0 {
				t.Error("Unexpected validate call made")
			}
		})
	}
}

func TestInstallManager_InstallAll_ErrorAggregation(t *testing.T) {
	tests := []struct {
		name                 string
		languages            []string
		errorLanguages       []string
		expectOverallError   bool
		expectedSuccessCount int
	}{
		{
			name:                 "All succeed",
			languages:            []string{"go", "python", "typescript"},
			errorLanguages:       []string{},
			expectOverallError:   false,
			expectedSuccessCount: 3,
		},
		{
			name:                 "Some fail",
			languages:            []string{"go", "python", "typescript"},
			errorLanguages:       []string{"python"},
			expectOverallError:   true,
			expectedSuccessCount: 2,
		},
		{
			name:                 "All fail",
			languages:            []string{"go", "python", "typescript"},
			errorLanguages:       []string{"go", "python", "typescript"},
			expectOverallError:   true,
			expectedSuccessCount: 0,
		},
		{
			name:                 "No languages registered",
			languages:            []string{},
			errorLanguages:       []string{},
			expectOverallError:   true,
			expectedSuccessCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLSPInstallManager()

			// Register installers
			for _, lang := range tt.languages {
				installer := newMockLanguageInstaller(lang)

				// Set errors for specified languages
				for _, errorLang := range tt.errorLanguages {
					if lang == errorLang {
						installer.setInstallError(errors.New("install failed for " + lang))
						break
					}
				}

				manager.RegisterInstaller(lang, installer)
			}

			ctx, cancel := common.CreateContext(10 * time.Second)
			defer cancel()

			err := manager.InstallAll(ctx, InstallOptions{})

			if tt.expectOverallError && err == nil {
				t.Error("Expected overall error but got none")
			}
			if !tt.expectOverallError && err != nil {
				t.Errorf("Unexpected overall error: %v", err)
			}

			// Check status of all languages
			status := manager.GetStatus()
			successCount := 0
			for _, s := range status {
				if s.Installed {
					successCount++
				}
			}

			if successCount != tt.expectedSuccessCount {
				t.Errorf("Expected %d successful installations, got %d", tt.expectedSuccessCount, successCount)
			}
		})
	}
}

func TestInstallManager_ConcurrentInstallation(t *testing.T) {
	manager := NewLSPInstallManager()

	languages := []string{"go", "python", "typescript", "java", "rust"}

	// Register installers with delays to simulate real installation time
	for _, lang := range languages {
		installer := newMockLanguageInstaller(lang)
		installer.installDelay = 100 * time.Millisecond
		manager.RegisterInstaller(lang, installer)
	}

	ctx, cancel := common.CreateContext(10 * time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errors := make(chan error, len(languages))

	// Start concurrent installations
	for _, lang := range languages {
		wg.Add(1)
		go func(language string) {
			defer wg.Done()
			err := manager.InstallLanguage(ctx, language, InstallOptions{})
			if err != nil {
				errors <- fmt.Errorf("failed to install %s: %w", language, err)
			}
		}(lang)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	if len(errorList) > 0 {
		t.Errorf("Concurrent installation errors: %v", errorList)
	}

	// Verify all languages were installed
	status := manager.GetStatus()
	for _, lang := range languages {
		if langStatus, exists := status[lang]; !exists || !langStatus.Installed {
			t.Errorf("Language %s was not properly installed", lang)
		}
	}
}

func TestInstallManager_ContextCancellation(t *testing.T) {
	manager := NewLSPInstallManager()

	// Create installer with long delay
	installer := newMockLanguageInstaller("slow-language")
	installer.installDelay = 5 * time.Second
	manager.RegisterInstaller("slow-language", installer)

	// Create context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := manager.InstallLanguage(ctx, "slow-language", InstallOptions{})
	duration := time.Since(start)

	// Should return context error quickly
	if err == nil {
		t.Error("Expected context cancellation error")
	}

	if duration > time.Second {
		t.Errorf("Installation took too long (%v), should have been cancelled", duration)
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context deadline exceeded error, got: %v", err)
	}
}

func TestConfigUpdater_Integration(t *testing.T) {
	// Create temporary directory for test config
	tempDir, err := os.MkdirTemp("", "lsp-gateway-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewLSPInstallManager()
	updater := NewConfigUpdater(manager)

	// Register some installers
	languages := []string{"go", "python", "typescript"}
	for _, lang := range languages {
		installer := newMockLanguageInstaller(lang)
		installer.setInstalled(true)

		// Customize server config paths for testing
		installer.serverConfig.Command = filepath.Join("/custom/path", lang+"-lsp")

		manager.RegisterInstaller(lang, installer)
	}

	// Test config update
	originalConfig := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls", // Will be updated
				Args:    []string{"serve"},
			},
			"existing": {
				Command: "existing-lsp",
				Args:    []string{"--stdio"},
			},
		},
	}

	updatedConfig, err := updater.UpdateConfigWithInstalledServers(originalConfig)
	if err != nil {
		t.Fatalf("Config update failed: %v", err)
	}

	// Verify updates
	if updatedConfig.Servers["go"].Command != filepath.Join("/custom/path", "go-lsp") {
		t.Errorf("Go command not updated correctly: %s", updatedConfig.Servers["go"].Command)
	}

	if updatedConfig.Servers["python"] == nil {
		t.Error("Python config not added")
	}

	if updatedConfig.Servers["typescript"] == nil {
		t.Error("TypeScript config not added")
	}

	// Verify existing config preserved
	if updatedConfig.Servers["existing"] == nil || updatedConfig.Servers["existing"].Command != "existing-lsp" {
		t.Error("Existing config not preserved")
	}

	// Test saving config
	configPath := filepath.Join(tempDir, "config.yaml")
	err = updater.SaveUpdatedConfig(updatedConfig, configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify config file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Verify backup was created if original existed
	err = updater.SaveUpdatedConfig(updatedConfig, configPath) // Save again
	if err != nil {
		t.Fatalf("Failed to save config second time: %v", err)
	}

	backupPath := configPath + ".backup"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("Backup file was not created")
	}
}

func TestPlatformIntegration_WithInstaller(t *testing.T) {
	// Test platform integration with actual installer components
	platform := NewLSPPlatformInfo()

	// Test supported platform detection
	isSupported := platform.IsSupported()

	// Test Java URL generation for current platform
	if isSupported {
		url, extractDir, err := platform.GetJavaDownloadURL("21")
		if err != nil {
			t.Fatalf("Java URL generation failed for supported platform: %v", err)
		}

		if url == "" || extractDir == "" {
			t.Error("Java URL or extract dir is empty")
		}

		// Verify URL structure is valid
		if !strings.HasPrefix(url, "https://") {
			t.Error("Java URL should be HTTPS")
		}

		// Verify platform-specific characteristics
		currentPlatform := platform.GetPlatform()
		currentArch := platform.GetArch()

		switch currentPlatform {
		case "windows":
			if !strings.Contains(url, ".zip") {
				t.Error("Windows should use zip archives")
			}
		case "linux", "darwin":
			if !strings.Contains(url, ".tar.gz") {
				t.Error("Linux/macOS should use tar.gz archives")
			}
		}

		// Verify architecture handling
		if currentArch == "amd64" && !strings.Contains(url, "x64") {
			t.Error("amd64 should map to x64 in URL")
		}
		if currentArch == "arm64" && !strings.Contains(url, "aarch64") {
			t.Error("arm64 should map to aarch64 in URL")
		}
	}

	// Test Node.js command generation
	nodeCmd := platform.GetNodeInstallCommand()
	if len(nodeCmd) == 0 {
		t.Error("Node install command should not be empty")
	}

	// Platform-specific command validation
	currentPlatform := platform.GetPlatform()
	switch currentPlatform {
	case "linux":
		if !containsString(nodeCmd, "apt-get") {
			t.Error("Linux should suggest apt-get for Node.js")
		}
	case "darwin":
		if !containsString(nodeCmd, "brew") {
			t.Error("macOS should suggest brew for Node.js")
		}
	case "windows":
		if !containsString(nodeCmd, "nodejs.org") {
			t.Error("Windows should suggest manual installation")
		}
	}
}

func TestErrorPropagation_CrossComponent(t *testing.T) {
	// Test error propagation across multiple components
	manager := NewLSPInstallManager()

	// Create installer that fails validation
	failingInstaller := newMockLanguageInstaller("failing")
	failingInstaller.setValidateError(errors.New("security validation failed"))
	manager.RegisterInstaller("failing", failingInstaller)

	// Create installer that fails installation
	installFailInstaller := newMockLanguageInstaller("install-fail")
	installFailInstaller.setInstallError(errors.New("download failed"))
	manager.RegisterInstaller("install-fail", installFailInstaller)

	// Create working installer
	workingInstaller := newMockLanguageInstaller("working")
	manager.RegisterInstaller("working", workingInstaller)

	ctx, cancel := common.CreateContext(5 * time.Second)
	defer cancel()

	// Test individual failures
	err := manager.InstallLanguage(ctx, "failing", InstallOptions{})
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Expected validation error, got: %v", err)
	}

	err = manager.InstallLanguage(ctx, "install-fail", InstallOptions{})
	if err == nil || !strings.Contains(err.Error(), "download failed") {
		t.Errorf("Expected download error, got: %v", err)
	}

	// Test that working installer still works
	err = manager.InstallLanguage(ctx, "working", InstallOptions{})
	if err != nil {
		t.Errorf("Working installer should succeed: %v", err)
	}

	// Test InstallAll error aggregation
	err = manager.InstallAll(ctx, InstallOptions{})
	if err == nil {
		t.Error("InstallAll should fail with some failing installers")
	}

	// Verify error message mentions failures
	if !strings.Contains(err.Error(), "some installations failed") {
		t.Errorf("Expected error aggregation message, got: %v", err)
	}

	// Verify status reflects mixed results
	status := manager.GetStatus()
	if !status["working"].Installed {
		t.Error("Working installer should be marked as installed")
	}
	if status["failing"].Installed {
		t.Error("Failing installer should not be marked as installed")
	}
	if status["install-fail"].Installed {
		t.Error("Install-fail installer should not be marked as installed")
	}
}

func TestInstallation_CleanupOnFailure(t *testing.T) {
	// This test verifies that failures don't leave the system in inconsistent state
	manager := NewLSPInstallManager()

	installer := newMockLanguageInstaller("cleanup-test")
	// Set it to fail after "installation"
	installer.setValidateError(errors.New("validation failed after install"))

	manager.RegisterInstaller("cleanup-test", installer)

	ctx, cancel := common.CreateContext(5 * time.Second)
	defer cancel()

	// Install should fail at validation stage
	err := manager.InstallLanguage(ctx, "cleanup-test", InstallOptions{})
	if err == nil {
		t.Error("Expected installation to fail at validation")
	}

	// Verify state is consistent
	status := manager.GetStatus()
	if status["cleanup-test"].Installed {
		t.Error("Failed installation should not be marked as installed")
	}

	// Verify we can retry installation
	installer.setValidateError(nil) // Remove validation error
	err = manager.InstallLanguage(ctx, "cleanup-test", InstallOptions{})
	if err != nil {
		t.Errorf("Retry installation should succeed: %v", err)
	}

	status = manager.GetStatus()
	if !status["cleanup-test"].Installed {
		t.Error("Retry installation should be marked as installed")
	}
}

func TestInstallOptions_Behavior(t *testing.T) {
	tests := []struct {
		name          string
		preInstalled  bool
		options       InstallOptions
		expectInstall bool
		expectError   bool
	}{
		{
			name:          "Normal install on clean system",
			preInstalled:  false,
			options:       InstallOptions{},
			expectInstall: true,
			expectError:   false,
		},
		{
			name:          "Skip install if already installed",
			preInstalled:  true,
			options:       InstallOptions{},
			expectInstall: false,
			expectError:   false,
		},
		{
			name:          "Force reinstall when already installed",
			preInstalled:  true,
			options:       InstallOptions{Force: true},
			expectInstall: true,
			expectError:   false,
		},
		{
			name:          "Skip validation",
			preInstalled:  false,
			options:       InstallOptions{SkipValidation: true},
			expectInstall: true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLSPInstallManager()
			installer := newMockLanguageInstaller("test")

			if tt.preInstalled {
				installer.setInstalled(true)
			}

			manager.RegisterInstaller("test", installer)

			ctx, cancel := common.CreateContext(5 * time.Second)
			defer cancel()

			err := manager.InstallLanguage(ctx, "test", tt.options)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			actualInstallCalls := installer.getInstallCallCount()
			if tt.expectInstall && actualInstallCalls == 0 {
				t.Error("Expected install call but none was made")
			}
			if !tt.expectInstall && actualInstallCalls > 0 {
				t.Error("Unexpected install call was made")
			}

			actualValidateCalls := installer.getValidateCallCount()
			if tt.options.SkipValidation && actualValidateCalls > 0 {
				t.Error("Validation was called despite skip flag")
			}
		})
	}
}

// Helper function to check if slice contains string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
