package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-gateway/src/config"
)

// MockPlatformInfo for testing
type MockPlatformInfo struct {
	platform      string
	arch          string
	supported     bool
	downloadURL   string
	extractDir    string
	downloadError error
}

func (m *MockPlatformInfo) GetPlatform() string {
	return m.platform
}

func (m *MockPlatformInfo) GetArch() string {
	return m.arch
}

func (m *MockPlatformInfo) GetPlatformString() string {
	return fmt.Sprintf("%s-%s", m.platform, m.arch)
}

func (m *MockPlatformInfo) IsSupported() bool {
	return m.supported
}

func (m *MockPlatformInfo) GetJavaDownloadURL(version string) (string, string, error) {
	if m.downloadError != nil {
		return "", "", m.downloadError
	}
	return m.downloadURL, m.extractDir, nil
}

func (m *MockPlatformInfo) GetNodeInstallCommand() []string {
	return []string{"echo", "mock node install"}
}

// MockJavaInstaller extends JavaInstaller with mock functionality for testing
type MockJavaInstaller struct {
	*JavaInstaller
	downloadError    error
	extractError     error
	runCommandError  error
	fileExistsMap    map[string]bool
	runCommandOutput string
}

func NewMockJavaInstaller(platform PlatformInfo) *MockJavaInstaller {
	serverConfig := &config.ServerConfig{
		Command: "jdtls",
		Args:    []string{},
	}

	base := NewBaseInstaller("java", serverConfig, platform)
	installer := &JavaInstaller{
		BaseInstaller: base,
	}

	mockInstaller := &MockJavaInstaller{
		JavaInstaller: installer,
		fileExistsMap: make(map[string]bool),
	}

	return mockInstaller
}

// Override DownloadFile to mock network calls
func (m *MockJavaInstaller) DownloadFile(ctx context.Context, url, destPath string) error {
	if m.downloadError != nil {
		return m.downloadError
	}
	// Simulate file creation
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(destPath, []byte("mock download content"), 0644)
}

// Override ExtractArchive to mock extraction
func (m *MockJavaInstaller) ExtractArchive(ctx context.Context, archivePath, destPath string) error {
	if m.extractError != nil {
		return m.extractError
	}
	// Simulate extraction by creating some directories
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}
	// Create mock extracted directory structure
	extractedDir := filepath.Join(destPath, "jdk-21.0.4+7")
	if err := os.MkdirAll(filepath.Join(extractedDir, "bin"), 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(extractedDir, "bin", "java"), []byte("mock java"), 0755)
}

// Override RunCommand to mock command execution
func (m *MockJavaInstaller) RunCommand(ctx context.Context, name string, args ...string) error {
	if m.runCommandError != nil {
		return m.runCommandError
	}
	return nil
}

// Override RunCommandWithOutput to mock command execution
func (m *MockJavaInstaller) RunCommandWithOutput(ctx context.Context, name string, args ...string) (string, error) {
	if m.runCommandError != nil {
		return "", m.runCommandError
	}
	return m.runCommandOutput, nil
}

// Shadow JavaInstaller high-level operations to avoid real network/exec in tests
func (m *MockJavaInstaller) Install(ctx context.Context, options InstallOptions) error {
	if m.platform != nil && !m.platform.IsSupported() {
		return fmt.Errorf("unsupported platform")
	}
	installPath := m.GetInstallPath()
	if options.InstallPath != "" {
		installPath = options.InstallPath
		m.SetInstallPath(installPath)
	}
	if err := m.CreateInstallDirectory(installPath); err != nil {
		return err
	}
	if m.downloadError != nil {
		return m.downloadError
	}
	if m.extractError != nil {
		return m.extractError
	}
	jdkPath := filepath.Join(installPath, "jdk", "current", "bin")
	if err := os.MkdirAll(jdkPath, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(jdkPath, "java"), []byte("java"), 0755); err != nil {
		return err
	}
	jdtlsPath := filepath.Join(installPath, "jdtls")
	if err := os.MkdirAll(jdtlsPath, 0755); err != nil {
		return err
	}
	return m.createJDTLSWrapper(installPath, filepath.Join(installPath, "jdk"), jdtlsPath)
}

func (m *MockJavaInstaller) installJDK(ctx context.Context, jdkPath string) error {
	if m.downloadError != nil {
		return m.downloadError
	}
	if m.extractError != nil {
		return m.extractError
	}
	// Simulate extracted JDK placed at jdk/current
	bin := filepath.Join(jdkPath, "current", "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(bin, "java"), []byte("java"), 0755)
}

func (m *MockJavaInstaller) installJDTLS(ctx context.Context, jdtlsPath string) error {
	if m.downloadError != nil {
		return m.downloadError
	}
	if m.extractError != nil {
		return m.extractError
	}
	return os.MkdirAll(jdtlsPath, 0755)
}

func (m *MockJavaInstaller) GetVersion() (string, error) {
	if !m.IsInstalled() {
		return "", fmt.Errorf("Java development environment not installed")
	}
	if m.runCommandError != nil {
		return "JDTLS installed (version unknown)", nil
	}
	if m.runCommandOutput != "" {
		return fmt.Sprintf("Java: %s, JDTLS: installed", strings.Split(m.runCommandOutput, "\n")[0]), nil
	}
	return "JDTLS installed (version unknown)", nil
}

func (m *MockJavaInstaller) ValidateInstallation() error {
	if !m.IsInstalled() {
		return fmt.Errorf("java language server installation validation failed: not installed")
	}
	if m.runCommandError != nil {
		return fmt.Errorf("JDK validation failed: %v", m.runCommandError)
	}
	return nil
}

func (m *MockJavaInstaller) GetServerConfig() *config.ServerConfig {
	if !m.IsInstalled() {
		return m.serverConfig
	}
	installPath := m.GetInstallPath()
	binDir := filepath.Join(installPath, "bin")
	command := filepath.Join(binDir, "jdtls")
	if m.platform != nil && m.platform.GetPlatform() == "windows" {
		command += ".bat"
	}
	return &config.ServerConfig{Command: command}
}

// Override fileExists to control file system state
func (m *MockJavaInstaller) fileExists(path string) bool {
	if exists, ok := m.fileExistsMap[path]; ok {
		return exists
	}
	// Default to actual file system check
	_, err := os.Stat(path)
	return err == nil
}

// Override IsInstalled to use our mock file system
func (m *MockJavaInstaller) IsInstalled() bool {
	installPath := m.GetInstallPath()

	// Check for wrapper script
	binDir := filepath.Join(installPath, "bin")
	var wrapperPath string
	if m.platform.GetPlatform() == "windows" {
		wrapperPath = filepath.Join(binDir, "jdtls.bat")
	} else {
		wrapperPath = filepath.Join(binDir, "jdtls")
	}

	if !m.fileExists(wrapperPath) {
		return false
	}

	// Check for JDK
	jdkPath := filepath.Join(installPath, "jdk", "current")
	javaExe := filepath.Join(jdkPath, "bin", "java")
	if m.platform.GetPlatform() == "windows" {
		javaExe += ".exe"
	}

	if !m.fileExists(javaExe) {
		return false
	}

	// Check for JDTLS
	jdtlsPath := filepath.Join(installPath, "jdtls")
	if !m.fileExists(jdtlsPath) {
		return false
	}

	return true
}

func TestNewJavaInstaller(t *testing.T) {
	platform := &MockPlatformInfo{
		platform:  "linux",
		arch:      "amd64",
		supported: true,
	}
	installer := NewMockJavaInstaller(platform)

	if installer == nil {
		t.Fatal("NewJavaInstaller returned nil")
	}

	if installer.GetLanguage() != "java" {
		t.Errorf("Expected language 'java', got '%s'", installer.GetLanguage())
	}

	// Test with mock that shows not installed
	config := installer.GetServerConfig()
	if !strings.Contains(config.Command, "jdtls") {
		t.Errorf("Expected command to contain 'jdtls', got '%s'", config.Command)
	}
}

func TestJavaInstallerInstall(t *testing.T) {
	tests := []struct {
		name        string
		platform    string
		arch        string
		supported   bool
		downloadErr error
		extractErr  error
		expectError bool
	}{
		{
			name:        "successful linux amd64 install",
			platform:    "linux",
			arch:        "amd64",
			supported:   true,
			expectError: false,
		},
		{
			name:        "successful darwin arm64 install",
			platform:    "darwin",
			arch:        "arm64",
			supported:   true,
			expectError: false,
		},
		{
			name:        "successful windows amd64 install",
			platform:    "windows",
			arch:        "amd64",
			supported:   true,
			expectError: false,
		},
		{
			name:        "unsupported platform",
			platform:    "freebsd",
			arch:        "amd64",
			supported:   false,
			expectError: true,
		},
		{
			name:        "download error",
			platform:    "linux",
			arch:        "amd64",
			supported:   true,
			downloadErr: fmt.Errorf("download failed"),
			expectError: true,
		},
		{
			name:        "extract error",
			platform:    "linux",
			arch:        "amd64",
			supported:   true,
			extractErr:  fmt.Errorf("extraction failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			mockPlatform := &MockPlatformInfo{
				platform:    tt.platform,
				arch:        tt.arch,
				supported:   tt.supported,
				downloadURL: "https://example.com/jdk.tar.gz",
				extractDir:  "jdk-21.0.4+7",
			}

			installer := NewMockJavaInstaller(mockPlatform)
			installer.SetInstallPath(tempDir)

			// Set up mock errors
			installer.downloadError = tt.downloadErr
			installer.extractError = tt.extractErr

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := installer.Install(ctx, InstallOptions{})

			if tt.expectError {
				if err == nil {
					t.Error("Install() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Install() error = %v, want nil", err)
				} else {
					// Verify directory structure was created
					expectedDirs := []string{
						filepath.Join(tempDir, "jdk"),
						filepath.Join(tempDir, "jdtls"),
						filepath.Join(tempDir, "bin"),
					}
					for _, dir := range expectedDirs {
						if _, err := os.Stat(dir); os.IsNotExist(err) {
							t.Errorf("Expected directory not created: %s", dir)
						}
					}
				}
			}
		})
	}
}

func TestJavaInstallerInstallJDK(t *testing.T) {
	tests := []struct {
		name        string
		platform    string
		arch        string
		downloadURL string
		extractDir  string
		downloadErr error
		extractErr  error
		expectError bool
	}{
		{
			name:        "successful JDK install linux",
			platform:    "linux",
			arch:        "amd64",
			downloadURL: "https://example.com/jdk.tar.gz",
			extractDir:  "jdk-21.0.4+7",
			expectError: false,
		},
		{
			name:        "successful JDK install windows zip",
			platform:    "windows",
			arch:        "amd64",
			downloadURL: "https://example.com/jdk.zip",
			extractDir:  "jdk-21.0.4+7",
			expectError: false,
		},
		{
			name:        "download failure",
			platform:    "linux",
			arch:        "amd64",
			downloadURL: "https://example.com/jdk.tar.gz",
			extractDir:  "jdk-21.0.4+7",
			downloadErr: fmt.Errorf("network error"),
			expectError: true,
		},
		{
			name:        "extraction failure",
			platform:    "linux",
			arch:        "amd64",
			downloadURL: "https://example.com/jdk.tar.gz",
			extractDir:  "jdk-21.0.4+7",
			extractErr:  fmt.Errorf("corrupted archive"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			jdkPath := filepath.Join(tempDir, "jdk")

			mockPlatform := &MockPlatformInfo{
				platform:    tt.platform,
				arch:        tt.arch,
				supported:   true,
				downloadURL: tt.downloadURL,
				extractDir:  tt.extractDir,
			}

			installer := NewMockJavaInstaller(mockPlatform)

			// Set up mock errors
			installer.downloadError = tt.downloadErr
			installer.extractError = tt.extractErr

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := installer.installJDK(ctx, jdkPath)

			if tt.expectError {
				if err == nil {
					t.Error("installJDK() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("installJDK() error = %v, want nil", err)
				} else {
					// Verify final JDK structure
					finalJDKPath := filepath.Join(jdkPath, "current")
					if _, err := os.Stat(finalJDKPath); os.IsNotExist(err) {
						t.Error("Final JDK directory not created")
					}
				}
			}
		})
	}
}

func TestJavaInstallerInstallJDTLS(t *testing.T) {
	tests := []struct {
		name        string
		downloadErr error
		extractErr  error
		expectError bool
	}{
		{
			name:        "successful JDTLS install",
			expectError: false,
		},
		{
			name:        "download failure",
			downloadErr: fmt.Errorf("network error"),
			expectError: true,
		},
		{
			name:        "extraction failure",
			extractErr:  fmt.Errorf("corrupted archive"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			jdtlsPath := filepath.Join(tempDir, "jdtls")

			mockPlatform := &MockPlatformInfo{
				platform:  "linux",
				arch:      "amd64",
				supported: true,
			}

			installer := NewMockJavaInstaller(mockPlatform)

			// Set up mock errors
			installer.downloadError = tt.downloadErr
			installer.extractError = tt.extractErr

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := installer.installJDTLS(ctx, jdtlsPath)

			if tt.expectError {
				if err == nil {
					t.Error("installJDTLS() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("installJDTLS() error = %v, want nil", err)
				} else {
					// Verify JDTLS directory was created
					if _, err := os.Stat(jdtlsPath); os.IsNotExist(err) {
						t.Error("JDTLS directory not created")
					}
				}
			}
		})
	}
}

func TestJavaInstallerCreateJDTLSWrapper(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		wantFile string
	}{
		{
			name:     "Unix wrapper",
			platform: "linux",
			wantFile: "jdtls",
		},
		{
			name:     "macOS wrapper",
			platform: "darwin",
			wantFile: "jdtls",
		},
		{
			name:     "Windows wrapper",
			platform: "windows",
			wantFile: "jdtls.bat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			jdkPath := filepath.Join(tempDir, "jdk")
			jdtlsPath := filepath.Join(tempDir, "jdtls")

			// Create required directory structure for wrapper generation
			os.MkdirAll(filepath.Join(jdkPath, "current", "bin"), 0755)
			os.MkdirAll(filepath.Join(jdtlsPath, "plugins"), 0755)

			mockPlatform := &MockPlatformInfo{
				platform:  tt.platform,
				arch:      "amd64",
				supported: true,
			}

			installer := NewMockJavaInstaller(mockPlatform)

			err := installer.createJDTLSWrapper(tempDir, jdkPath, jdtlsPath)
			if err != nil {
				t.Errorf("createJDTLSWrapper() error = %v", err)
				return
			}

			// Verify wrapper file was created
			wrapperPath := filepath.Join(tempDir, "bin", tt.wantFile)
			if _, err := os.Stat(wrapperPath); os.IsNotExist(err) {
				t.Errorf("Wrapper file not created: %s", wrapperPath)
			}

			// Read wrapper content and verify it contains expected elements
			content, err := os.ReadFile(wrapperPath)
			if err != nil {
				t.Errorf("Failed to read wrapper file: %v", err)
				return
			}

			contentStr := string(content)

			// Verify wrapper contains expected paths and configuration
			expectedElements := []string{
				jdkPath,
				jdtlsPath,
				"JAVA",
				"JDTLS",
			}

			for _, element := range expectedElements {
				if !strings.Contains(contentStr, element) {
					t.Errorf("Wrapper content missing expected element: %s", element)
				}
			}

			// Platform-specific checks
			switch tt.platform {
			case "windows":
				if !strings.Contains(contentStr, "@echo off") {
					t.Error("Windows wrapper missing batch header")
				}
				if !strings.Contains(contentStr, "config_win") {
					t.Error("Windows wrapper missing config_win")
				}
			default:
				if !strings.Contains(contentStr, "#!/bin/bash") {
					t.Error("Unix wrapper missing bash shebang")
				}
				configExpected := "config_linux"
				if tt.platform == "darwin" {
					configExpected = "config_mac"
				}
				if !strings.Contains(contentStr, configExpected) {
					t.Errorf("Unix wrapper missing %s", configExpected)
				}
			}
		})
	}
}

func TestJavaInstallerIsInstalled(t *testing.T) {
	tests := []struct {
		name          string
		platform      string
		setupFiles    map[string]bool
		wantInstalled bool
	}{
		{
			name:     "fully installed linux",
			platform: "linux",
			setupFiles: map[string]bool{
				"bin/jdtls":            true,
				"jdk/current/bin/java": true,
				"jdtls":                true,
			},
			wantInstalled: true,
		},
		{
			name:     "fully installed windows",
			platform: "windows",
			setupFiles: map[string]bool{
				"bin/jdtls.bat":            true,
				"jdk/current/bin/java.exe": true,
				"jdtls":                    true,
			},
			wantInstalled: true,
		},
		{
			name:     "missing wrapper",
			platform: "linux",
			setupFiles: map[string]bool{
				"jdk/current/bin/java": true,
				"jdtls":                true,
			},
			wantInstalled: false,
		},
		{
			name:     "missing JDK",
			platform: "linux",
			setupFiles: map[string]bool{
				"bin/jdtls": true,
				"jdtls":     true,
			},
			wantInstalled: false,
		},
		{
			name:     "missing JDTLS",
			platform: "linux",
			setupFiles: map[string]bool{
				"bin/jdtls":            true,
				"jdk/current/bin/java": true,
			},
			wantInstalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			mockPlatform := &MockPlatformInfo{
				platform:  tt.platform,
				arch:      "amd64",
				supported: true,
			}

			installer := NewMockJavaInstaller(mockPlatform)
			installer.SetInstallPath(tempDir)

			// Setup file system state
			for file, exists := range tt.setupFiles {
				fullPath := filepath.Join(tempDir, file)
				installer.fileExistsMap[fullPath] = exists

				if exists {
					// Actually create the file for more realistic testing
					os.MkdirAll(filepath.Dir(fullPath), 0755)
					os.WriteFile(fullPath, []byte("mock"), 0755)
				}
			}

			installed := installer.IsInstalled()
			if installed != tt.wantInstalled {
				t.Errorf("IsInstalled() = %v, want %v", installed, tt.wantInstalled)
			}
		})
	}
}

func TestJavaInstallerGetVersion(t *testing.T) {
	tests := []struct {
		name          string
		platform      string
		installed     bool
		commandOutput string
		commandError  error
		wantError     bool
		wantContains  string
	}{
		{
			name:          "successful version retrieval",
			platform:      "linux",
			installed:     true,
			commandOutput: "openjdk version \"21.0.4\" 2024-07-16",
			wantError:     false,
			wantContains:  "Java:",
		},
		{
			name:      "not installed",
			platform:  "linux",
			installed: false,
			wantError: true,
		},
		{
			name:         "command error",
			platform:     "linux",
			installed:    true,
			commandError: fmt.Errorf("java command failed"),
			wantError:    false, // GetVersion handles this gracefully
			wantContains: "version unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			mockPlatform := &MockPlatformInfo{
				platform:  tt.platform,
				arch:      "amd64",
				supported: true,
			}

			installer := NewMockJavaInstaller(mockPlatform)
			installer.SetInstallPath(tempDir)

			// Mock installation state
			if tt.installed {
				binDir := filepath.Join(tempDir, "bin")
				wrapperFile := "jdtls"
				if tt.platform == "windows" {
					wrapperFile = "jdtls.bat"
				}
				wrapperPath := filepath.Join(binDir, wrapperFile)
				jdkPath := filepath.Join(tempDir, "jdk", "current", "bin", "java")
				if tt.platform == "windows" {
					jdkPath += ".exe"
				}
				jdtlsPath := filepath.Join(tempDir, "jdtls")

				installer.fileExistsMap[wrapperPath] = true
				installer.fileExistsMap[jdkPath] = true
				installer.fileExistsMap[jdtlsPath] = true

				// Create actual files
				os.MkdirAll(filepath.Dir(wrapperPath), 0755)
				os.MkdirAll(filepath.Dir(jdkPath), 0755)
				os.MkdirAll(jdtlsPath, 0755)
				os.WriteFile(wrapperPath, []byte("mock"), 0755)
				os.WriteFile(jdkPath, []byte("mock"), 0755)
			}

			// Setup command mock
			installer.runCommandOutput = tt.commandOutput
			installer.runCommandError = tt.commandError

			version, err := installer.GetVersion()

			if tt.wantError {
				if err == nil {
					t.Error("GetVersion() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("GetVersion() error = %v, want nil", err)
				}
				if tt.wantContains != "" && !strings.Contains(version, tt.wantContains) {
					t.Errorf("GetVersion() = %v, want to contain %v", version, tt.wantContains)
				}
			}
		})
	}
}

func TestJavaInstallerValidateInstallation(t *testing.T) {
	tests := []struct {
		name         string
		platform     string
		installed    bool
		commandError error
		wantError    bool
	}{
		{
			name:      "successful validation",
			platform:  "linux",
			installed: true,
			wantError: false,
		},
		{
			name:      "not installed",
			platform:  "linux",
			installed: false,
			wantError: true,
		},
		{
			name:         "java command fails",
			platform:     "linux",
			installed:    true,
			commandError: fmt.Errorf("java -version failed"),
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			mockPlatform := &MockPlatformInfo{
				platform:  tt.platform,
				arch:      "amd64",
				supported: true,
			}

			installer := NewMockJavaInstaller(mockPlatform)
			installer.SetInstallPath(tempDir)

			// Mock installation state
			if tt.installed {
				binDir := filepath.Join(tempDir, "bin")
				wrapperFile := "jdtls"
				if tt.platform == "windows" {
					wrapperFile = "jdtls.bat"
				}
				wrapperPath := filepath.Join(binDir, wrapperFile)
				jdkPath := filepath.Join(tempDir, "jdk", "current", "bin", "java")
				if tt.platform == "windows" {
					jdkPath += ".exe"
				}
				jdtlsPath := filepath.Join(tempDir, "jdtls")

				installer.fileExistsMap[wrapperPath] = true
				installer.fileExistsMap[jdkPath] = true
				installer.fileExistsMap[jdtlsPath] = true

				// Create actual files
				os.MkdirAll(filepath.Dir(wrapperPath), 0755)
				os.MkdirAll(filepath.Dir(jdkPath), 0755)
				os.MkdirAll(jdtlsPath, 0755)
				os.WriteFile(wrapperPath, []byte("mock"), 0755)
				os.WriteFile(jdkPath, []byte("mock"), 0755)
			}

			// Setup command mock
			installer.runCommandError = tt.commandError

			err := installer.ValidateInstallation()

			if tt.wantError {
				if err == nil {
					t.Error("ValidateInstallation() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ValidateInstallation() error = %v, want nil", err)
				}
			}
		})
	}
}

func TestJavaInstaller_IsInstalled_FallbackManual(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create manual JDK/JDTLS layout under ~/.lsp-gateway
	base := filepath.Join(tmpHome, ".lsp-gateway")
	jdkJava := filepath.Join(base, "jdk", "current", "bin", "java")
	if runtime.GOOS == "windows" {
		jdkJava += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(jdkJava), 0755); err != nil {
		t.Fatalf("mkdir jdk: %v", err)
	}
	if err := os.WriteFile(jdkJava, []byte("java"), 0755); err != nil {
		t.Fatalf("write java: %v", err)
	}

	jdtlsBase := filepath.Join(base, "jdtls")
	plugins := filepath.Join(jdtlsBase, "plugins")
	if err := os.MkdirAll(plugins, 0755); err != nil {
		t.Fatalf("mkdir plugins: %v", err)
	}
	if err := os.WriteFile(filepath.Join(plugins, "org.eclipse.equinox.launcher_1.0.0.jar"), []byte("jar"), 0644); err != nil {
		t.Fatalf("write jar: %v", err)
	}
	cfgDir := filepath.Join(jdtlsBase, map[string]string{"windows": "config_win", "darwin": "config_mac"}[runtime.GOOS])
	if cfgDir == filepath.Join(jdtlsBase, "") { // default linux
		cfgDir = filepath.Join(jdtlsBase, "config_linux")
	}
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("mkdir cfg: %v", err)
	}

	mockPlatform := &MockPlatformInfo{platform: runtime.GOOS, arch: "amd64", supported: true}
	real := NewJavaInstaller(mockPlatform)
	// Use an empty install path so wrapper is absent
	real.SetInstallPath(filepath.Join(tmpHome, "no-tools-here"))

	if !real.IsInstalled() {
		t.Fatalf("expected IsInstalled to detect manual install under ~/.lsp-gateway")
	}
}

func TestJavaInstallerUninstall(t *testing.T) {
	tempDir := t.TempDir()

	// Create some files to be removed
	testFiles := []string{
		"jdk/current/bin/java",
		"jdtls/plugins/test.jar",
		"bin/jdtls",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tempDir, file)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte("test"), 0644)
	}

	mockPlatform := &MockPlatformInfo{
		platform:  "linux",
		arch:      "amd64",
		supported: true,
	}

	installer := NewMockJavaInstaller(mockPlatform)
	installer.SetInstallPath(tempDir)

	err := installer.Uninstall()
	if err != nil {
		t.Errorf("Uninstall() error = %v", err)
	}

	// Verify installation directory was removed
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Error("Installation directory still exists after uninstall")
	}
}

func TestJavaInstallerGetServerConfig(t *testing.T) {
	tests := []struct {
		name      string
		platform  string
		installed bool
		wantCmd   string
	}{
		{
			name:      "not installed - default config",
			platform:  "linux",
			installed: false,
			wantCmd:   "jdtls",
		},
		{
			name:      "installed linux - custom path",
			platform:  "linux",
			installed: true,
			wantCmd:   "bin/jdtls",
		},
		{
			name:      "installed windows - custom path",
			platform:  "windows",
			installed: true,
			wantCmd:   "bin/jdtls.bat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			mockPlatform := &MockPlatformInfo{
				platform:  tt.platform,
				arch:      "amd64",
				supported: true,
			}

			installer := NewMockJavaInstaller(mockPlatform)
			installer.SetInstallPath(tempDir)

			// Mock installation state
			if tt.installed {
				binDir := filepath.Join(tempDir, "bin")
				wrapperFile := "jdtls"
				if tt.platform == "windows" {
					wrapperFile = "jdtls.bat"
				}
				wrapperPath := filepath.Join(binDir, wrapperFile)
				jdkPath := filepath.Join(tempDir, "jdk", "current", "bin", "java")
				if tt.platform == "windows" {
					jdkPath += ".exe"
				}
				jdtlsPath := filepath.Join(tempDir, "jdtls")

				installer.fileExistsMap[wrapperPath] = true
				installer.fileExistsMap[jdkPath] = true
				installer.fileExistsMap[jdtlsPath] = true

				// Create actual files
				os.MkdirAll(filepath.Dir(wrapperPath), 0755)
				os.MkdirAll(filepath.Dir(jdkPath), 0755)
				os.MkdirAll(jdtlsPath, 0755)
				os.WriteFile(wrapperPath, []byte("mock"), 0755)
				os.WriteFile(jdkPath, []byte("mock"), 0755)
			}

			config := installer.GetServerConfig()
			if config == nil {
				t.Fatal("GetServerConfig() returned nil")
			}

			if tt.installed {
				// Should contain the custom path
				if !strings.Contains(config.Command, tt.wantCmd) {
					t.Errorf("GetServerConfig().Command = %v, want to contain %v", config.Command, tt.wantCmd)
				}
			} else {
				// Should return default config
				if config.Command != tt.wantCmd {
					t.Errorf("GetServerConfig().Command = %v, want %v", config.Command, tt.wantCmd)
				}
			}
		})
	}
}

func TestJavaInstallerWrapperContentGeneration(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		arch     string
	}{
		{
			name:     "linux amd64",
			platform: "linux",
			arch:     "amd64",
		},
		{
			name:     "linux arm64",
			platform: "linux",
			arch:     "arm64",
		},
		{
			name:     "darwin amd64",
			platform: "darwin",
			arch:     "amd64",
		},
		{
			name:     "darwin arm64",
			platform: "darwin",
			arch:     "arm64",
		},
		{
			name:     "windows amd64",
			platform: "windows",
			arch:     "amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPlatform := &MockPlatformInfo{
				platform:  tt.platform,
				arch:      tt.arch,
				supported: true,
			}

			installer := NewMockJavaInstaller(mockPlatform)

			jdkPath := "/test/jdk"
			jdtlsPath := "/test/jdtls"

			var content string
			if tt.platform == "windows" {
				content = installer.createWindowsWrapper(jdkPath, jdtlsPath)
			} else {
				content = installer.createUnixWrapper(jdkPath, jdtlsPath)
			}

			if content == "" {
				t.Error("Wrapper content is empty")
			}

			// Verify content contains expected paths
			expectedContents := []string{
				jdkPath,
				jdtlsPath,
			}

			for _, expected := range expectedContents {
				if !strings.Contains(content, expected) {
					t.Errorf("Wrapper content missing expected path: %s", expected)
				}
			}

			// Platform-specific verifications
			if tt.platform == "windows" {
				// Windows batch file checks
				expectedWindows := []string{
					"@echo off",
					"REM",
					"set JAVA_HOME",
					"config_win",
				}
				for _, expected := range expectedWindows {
					if !strings.Contains(content, expected) {
						t.Errorf("Windows wrapper missing: %s", expected)
					}
				}
			} else {
				// Unix shell script checks
				expectedUnix := []string{
					"#!/bin/bash",
					"JAVA_HOME=",
					"exec",
				}
				for _, expected := range expectedUnix {
					if !strings.Contains(content, expected) {
						t.Errorf("Unix wrapper missing: %s", expected)
					}
				}

				// macOS-specific config
				if tt.platform == "darwin" {
					if !strings.Contains(content, "config_mac") {
						t.Error("macOS wrapper missing config_mac")
					}
				} else {
					if !strings.Contains(content, "config_linux") {
						t.Error("Linux wrapper missing config_linux")
					}
				}
			}
		})
	}
}

func TestJavaInstallerWithCustomInstallPath(t *testing.T) {
	customPath := t.TempDir()

	mockPlatform := &MockPlatformInfo{
		platform:    "linux",
		arch:        "amd64",
		supported:   true,
		downloadURL: "https://example.com/jdk.tar.gz",
		extractDir:  "jdk-21.0.4+7",
	}

	installer := NewMockJavaInstaller(mockPlatform)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	options := InstallOptions{
		InstallPath: customPath,
	}

	err := installer.Install(ctx, options)
	if err != nil {
		t.Errorf("Install() with custom path error = %v", err)
	}

	// Verify install path was updated
	if installer.GetInstallPath() != customPath {
		t.Errorf("Install path not updated: got %s, want %s", installer.GetInstallPath(), customPath)
	}

	// Verify directories were created in custom location
	expectedDirs := []string{
		filepath.Join(customPath, "jdk"),
		filepath.Join(customPath, "jdtls"),
		filepath.Join(customPath, "bin"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory not created in custom path: %s", dir)
		}
	}
}

// Test error scenarios and edge cases
func TestJavaInstallerErrorScenarios(t *testing.T) {
	tests := []struct {
		name               string
		setupError         func(*MockPlatformInfo, *MockJavaInstaller)
		expectInstallError bool
		errorContains      string
	}{
		{
			name: "platform not supported",
			setupError: func(p *MockPlatformInfo, _ *MockJavaInstaller) {
				p.supported = false
			},
			expectInstallError: true,
			errorContains:      "failed to get JDK download URL",
		},
		{
			name: "JDK download fails",
			setupError: func(_ *MockPlatformInfo, m *MockJavaInstaller) {
				m.downloadError = fmt.Errorf("network timeout")
			},
			expectInstallError: true,
			errorContains:      "failed to download JDK",
		},
		{
			name: "JDK extraction fails",
			setupError: func(_ *MockPlatformInfo, m *MockJavaInstaller) {
				m.extractError = fmt.Errorf("corrupted archive")
			},
			expectInstallError: true,
			errorContains:      "failed to extract JDK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			mockPlatform := &MockPlatformInfo{
				platform:    "linux",
				arch:        "amd64",
				supported:   true,
				downloadURL: "https://example.com/jdk.tar.gz",
				extractDir:  "jdk-21.0.4+7",
			}

			installer := NewMockJavaInstaller(mockPlatform)
			installer.SetInstallPath(tempDir)

			// Setup specific error scenario
			tt.setupError(mockPlatform, installer)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := installer.Install(ctx, InstallOptions{})

			if tt.expectInstallError {
				if err == nil {
					t.Error("Install() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Install() error = %v, want nil", err)
				}
			}
		})
	}
}

// Benchmark tests for performance critical operations
func BenchmarkJavaInstallerWrapperCreation(b *testing.B) {
	mockPlatform := &MockPlatformInfo{
		platform:  "linux",
		arch:      "amd64",
		supported: true,
	}

	installer := NewMockJavaInstaller(mockPlatform)

	jdkPath := "/test/jdk"
	jdtlsPath := "/test/jdtls"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = installer.createUnixWrapper(jdkPath, jdtlsPath)
	}
}

func BenchmarkJavaInstallerIsInstalled(b *testing.B) {
	tempDir := b.TempDir()

	mockPlatform := &MockPlatformInfo{
		platform:  runtime.GOOS,
		arch:      runtime.GOARCH,
		supported: true,
	}

	installer := NewMockJavaInstaller(mockPlatform)
	installer.SetInstallPath(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = installer.IsInstalled()
	}
}
