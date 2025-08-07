package installer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-gateway/src/config"
)

func TestNewBaseInstaller(t *testing.T) {
	serverConfig := &config.ServerConfig{
		Command: "gopls",
		Args:    []string{"serve"},
	}

	platform := NewLSPPlatformInfo()

	base := NewBaseInstaller("go", serverConfig, platform)
	if base == nil {
		t.Fatal("NewBaseInstaller returned nil")
	}

	if base.GetLanguage() != "go" {
		t.Errorf("Expected language 'go', got '%s'", base.GetLanguage())
	}

	if base.GetServerConfig() != serverConfig {
		t.Error("Server config not set correctly")
	}
}

func TestBaseInstallerIsInstalled(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectFound bool
	}{
		{
			name:        "existing command",
			command:     "echo",
			expectFound: true,
		},
		{
			name:        "non-existing command",
			command:     "nonexistentcommand12345",
			expectFound: false,
		},
		{
			name:        "go command",
			command:     "go",
			expectFound: true, // Assume go is installed for tests
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverConfig := &config.ServerConfig{
				Command: tt.command,
				Args:    []string{"--version"},
			}

			platform := NewLSPPlatformInfo()
			base := NewBaseInstaller(tt.name, serverConfig, platform)

			installed := base.IsInstalled()
			if installed != tt.expectFound {
				t.Errorf("IsInstalled() = %v, want %v", installed, tt.expectFound)
			}
		})
	}
}

func TestBaseInstallerGetVersion(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:        "echo version",
			command:     "echo",
			expectError: false,
		},
		{
			name:        "go version",
			command:     "go",
			expectError: false,
		},
		{
			name:        "non-existing command",
			command:     "nonexistentcommand12345",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverConfig := &config.ServerConfig{
				Command: tt.command,
				Args:    []string{"--version"},
			}

			platform := NewLSPPlatformInfo()
			base := NewBaseInstaller(tt.name, serverConfig, platform)

			version, err := base.GetVersion()
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if version == "" {
					t.Error("Expected non-empty version")
				}
			}
		})
	}
}

func TestBaseInstallerValidateInstallation(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectValid bool
	}{
		{
			name:        "valid command",
			command:     "echo",
			expectValid: true,
		},
		{
			name:        "invalid command",
			command:     "nonexistentcommand12345",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverConfig := &config.ServerConfig{
				Command: tt.command,
				Args:    []string{"test"},
			}

			platform := NewLSPPlatformInfo()
			base := NewBaseInstaller(tt.name, serverConfig, platform)

			err := base.ValidateInstallation()
			if tt.expectValid {
				if err != nil {
					t.Errorf("ValidateInstallation() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Error("ValidateInstallation() expected error but got nil")
				}
			}
		})
	}
}

func TestBaseInstallerRunCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        []string
		expectError bool
	}{
		{
			name:        "valid echo command",
			command:     "echo",
			args:        []string{"hello"},
			expectError: false,
		},
		{
			name:        "invalid command",
			command:     "nonexistentcommand12345",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "command with exit code 1",
			command:     "sh",
			args:        []string{"-c", "exit 1"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverConfig := &config.ServerConfig{
				Command: "test",
				Args:    []string{},
			}

			platform := NewLSPPlatformInfo()
			base := NewBaseInstaller(tt.name, serverConfig, platform)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := base.RunCommand(ctx, tt.command, tt.args...)
			if tt.expectError {
				if err == nil {
					t.Error("RunCommand() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("RunCommand() error = %v, want nil", err)
				}
			}
		})
	}
}

func TestBaseInstallerRunCommandWithOutput(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		args           []string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "echo with output",
			command:        "echo",
			args:           []string{"hello world"},
			expectedOutput: "hello world",
			expectError:    false,
		},
		{
			name:        "invalid command",
			command:     "nonexistentcommand12345",
			args:        []string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverConfig := &config.ServerConfig{
				Command: "test",
				Args:    []string{},
			}

			platform := NewLSPPlatformInfo()
			base := NewBaseInstaller(tt.name, serverConfig, platform)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			output, err := base.RunCommandWithOutput(ctx, tt.command, tt.args...)
			if tt.expectError {
				if err == nil {
					t.Error("RunCommandWithOutput() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("RunCommandWithOutput() error = %v, want nil", err)
				}
				if !strings.Contains(output, tt.expectedOutput) {
					t.Errorf("RunCommandWithOutput() output = %v, want to contain %v", output, tt.expectedOutput)
				}
			}
		})
	}
}

func TestBaseInstallerInstallWithPackageManager(t *testing.T) {
	tests := []struct {
		name        string
		pkgManager  string
		packageName string
		version     string
		expectError bool
	}{
		{
			name:        "go package manager",
			pkgManager:  "go",
			packageName: "test-package",
			version:     "1.0.0",
			expectError: false, // Will fail with "go install" but that's expected in test
		},
		{
			name:        "unsupported package manager",
			pkgManager:  "unsupported-pm",
			packageName: "test-package",
			version:     "1.0.0",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverConfig := &config.ServerConfig{
				Command: "test",
				Args:    []string{},
			}

			platform := NewLSPPlatformInfo()
			base := NewBaseInstaller(tt.name, serverConfig, platform)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := base.InstallWithPackageManager(ctx, tt.pkgManager, tt.packageName, tt.version)
			if tt.expectError {
				if err == nil {
					t.Error("InstallWithPackageManager() expected error but got nil")
				}
			} else {
				// For supported package managers, we expect an error during actual install
				// (since test-package doesn't exist), but not an "unsupported" error
				if err != nil && strings.Contains(err.Error(), "unsupported package manager") {
					t.Errorf("InstallWithPackageManager() got unsupported error for %s: %v", tt.pkgManager, err)
				}
			}
		})
	}
}

func TestBaseInstallerCreateInstallDirectory(t *testing.T) {
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "test-install")

	serverConfig := &config.ServerConfig{
		Command: "test",
		Args:    []string{},
	}

	platform := NewLSPPlatformInfo()
	base := NewBaseInstaller("test", serverConfig, platform)

	err := base.CreateInstallDirectory(testDir)
	if err != nil {
		t.Errorf("CreateInstallDirectory() error = %v", err)
	}

	// Check if directory was created
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}
}

func TestBaseInstallerGetInstallPath(t *testing.T) {
	serverConfig := &config.ServerConfig{
		Command: "test",
		Args:    []string{},
	}

	platform := NewLSPPlatformInfo()
	base := NewBaseInstaller("test", serverConfig, platform)

	// Test default path
	path := base.GetInstallPath()
	if path == "" {
		t.Error("GetInstallPath() returned empty path")
	}

	// Test custom path
	customPath := "/custom/install/path"
	base.SetInstallPath(customPath)
	path = base.GetInstallPath()
	if path != customPath {
		t.Errorf("GetInstallPath() = %v, want %v", path, customPath)
	}
}

func TestBaseInstallerDownloadFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	tempDir := t.TempDir()
	destPath := filepath.Join(tempDir, "test-download.txt")

	serverConfig := &config.ServerConfig{
		Command: "test",
		Args:    []string{},
	}

	platform := NewLSPPlatformInfo()
	base := NewBaseInstaller("test", serverConfig, platform)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a simple HTTP URL that should be available
	// Note: This test may fail if there's no internet connection
	url := "https://httpbin.org/robots.txt"
	err := base.DownloadFile(ctx, url, destPath)
	if err != nil {
		t.Logf("DownloadFile() error = %v (may be expected if no internet)", err)
		return // Don't fail the test for network issues
	}

	// Check if file was created
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Downloaded file was not created")
	}
}

func TestBaseInstallerExtractArchive(t *testing.T) {
	// Create a simple test archive
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "test.tar.gz")
	extractPath := filepath.Join(tempDir, "extracted")

	// Create a simple tar.gz archive for testing
	// This is a minimal test - in practice, you'd create a real archive
	serverConfig := &config.ServerConfig{
		Command: "test",
		Args:    []string{},
	}

	platform := NewLSPPlatformInfo()
	base := NewBaseInstaller("test", serverConfig, platform)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with non-existent archive (should fail)
	err := base.ExtractArchive(ctx, archivePath, extractPath)
	if err == nil {
		t.Error("ExtractArchive() expected error for non-existent archive")
	}

	// Test with unsupported format
	unsupportedPath := filepath.Join(tempDir, "test.rar")
	err = base.ExtractArchive(ctx, unsupportedPath, extractPath)
	if err == nil {
		t.Error("ExtractArchive() expected error for unsupported format")
	}
}

func TestInstallOptions(t *testing.T) {
	tests := []struct {
		name    string
		options InstallOptions
	}{
		{
			name: "default options",
			options: InstallOptions{
				Force:   false,
				Version: "",
			},
		},
		{
			name: "with force",
			options: InstallOptions{
				Force:   true,
				Version: "",
			},
		},
		{
			name: "with version",
			options: InstallOptions{
				Force:   false,
				Version: "1.2.3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just test that the InstallOptions struct can be created and used
			if tt.options.Version == "1.2.3" && !tt.options.Force {
				t.Log("InstallOptions with specific version created successfully")
			}
			if tt.options.Force && tt.options.Version == "" {
				t.Log("InstallOptions with force flag created successfully")
			}
		})
	}
}

func TestPlatformInfo(t *testing.T) {
	platform := NewLSPPlatformInfo()

	// Test GetPlatform
	p := platform.GetPlatform()
	if p == "" {
		t.Error("GetPlatform() returned empty string")
	}

	expectedPlatforms := []string{"linux", "darwin", "windows"}
	found := false
	for _, expected := range expectedPlatforms {
		if p == expected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GetPlatform() = %v, expected one of %v", p, expectedPlatforms)
	}

	// Test GetArch
	arch := platform.GetArch()
	if arch == "" {
		t.Error("GetArch() returned empty string")
	}

	// Test GetPlatformString
	platformStr := platform.GetPlatformString()
	if platformStr == "" {
		t.Error("GetPlatformString() returned empty string")
	}

	// Test IsSupported
	supported := platform.IsSupported()
	if !supported {
		t.Error("IsSupported() returned false for current platform")
	}
}

func TestValidationMethods(t *testing.T) {
	serverConfig := &config.ServerConfig{
		Command: "echo",
		Args:    []string{"test"},
	}

	platform := NewLSPPlatformInfo()
	base := NewBaseInstaller("test", serverConfig, platform)

	// Test IsInstalledByCommand
	installed := base.IsInstalledByCommand("echo")
	if !installed {
		t.Error("IsInstalledByCommand('echo') should return true")
	}

	// Test GetVersionByCommand
	version, err := base.GetVersionByCommand("echo", "test")
	if err != nil {
		t.Errorf("GetVersionByCommand() error = %v", err)
	}
	if version == "" {
		t.Error("GetVersionByCommand() returned empty version")
	}

	// Test ValidateByCommand
	err = base.ValidateByCommand("echo")
	if err != nil {
		t.Errorf("ValidateByCommand() error = %v", err)
	}
}

func TestPackageManagerHelpers(t *testing.T) {
	serverConfig := &config.ServerConfig{
		Command: "test",
		Args:    []string{},
	}

	platform := NewLSPPlatformInfo()
	base := NewBaseInstaller("test", serverConfig, platform)

	// Test ValidateWithPackageManager for Go (if available)
	if runtime.GOOS != "windows" { // Skip on Windows as go command may not be available
		err := base.ValidateWithPackageManager("echo", "go")
		if err != nil {
			t.Logf("ValidateWithPackageManager() error = %v (expected if Go not installed)", err)
		}
	}
}
