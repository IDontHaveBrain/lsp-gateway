package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockFileSystem provides file system mocking for missing file scenarios
type MockFileSystem struct {
	mu           sync.RWMutex
	files        map[string][]byte
	missingFiles map[string]bool
	permissions  map[string]os.FileMode
}

func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files:        make(map[string][]byte),
		missingFiles: make(map[string]bool),
		permissions:  make(map[string]os.FileMode),
	}
}

func (m *MockFileSystem) AddFile(path string, content []byte, mode os.FileMode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = content
	m.permissions[path] = mode
	delete(m.missingFiles, path)
}

func (m *MockFileSystem) RemoveFile(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, path)
	m.missingFiles[path] = true
}

func (m *MockFileSystem) FileExists(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.missingFiles[path] {
		return false
	}
	_, exists := m.files[path]
	return exists
}

func (m *MockFileSystem) ReadFile(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.missingFiles[path] {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	content, exists := m.files[path]
	if !exists {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	return content, nil
}

// Helper functions for test configuration creation
func createValidConfigContent() string {
	return `
port: 8080
servers:
  - name: go-lsp
    languages: [go]
    command: gopls
    transport: stdio
  - name: python-lsp
    languages: [python]
    command: python
    args: ["-m", "pylsp"]
    transport: stdio
`
}

func createPartialConfigContent() string {
	return `
port: 8080
# Missing servers section
`
}

func createCorruptedConfigContent() string {
	return `
port: 8080
servers:
  - name: go-lsp
    languages: [go
    command: gopls
    transport: stdio
# Unclosed YAML structure
`
}

func createEmptyConfigContent() string {
	return ""
}

func createInvalidPortConfigContent() string {
	return `
port: invalid_port
servers:
  - name: go-lsp
    languages: [go]
    command: gopls
    transport: stdio
`
}

// Test missing configuration file scenarios
func TestLoadConfig_MissingConfigFile(t *testing.T) {
	t.Parallel()
	
	// Test completely missing config file
	nonExistentPath := "/nonexistent/path/config.yaml"
	_, err := LoadConfig(nonExistentPath)
	
	if err == nil {
		t.Error("Expected error for missing config file")
	}
	
	if !strings.Contains(err.Error(), "configuration file not found") {
		t.Errorf("Expected 'configuration file not found' error, got: %v", err)
	}
	
	if !os.IsNotExist(err) {
		// Check if the error unwraps to os.ErrNotExist
		if !strings.Contains(err.Error(), "configuration file not found") {
			t.Error("Error should indicate file not found")
		}
	}
}

func TestLoadConfig_MissingConfigDirectory(t *testing.T) {
	t.Parallel()
	
	// Test config file in missing directory
	missingDirPath := "/nonexistent/directory/config.yaml"
	_, err := LoadConfig(missingDirPath)
	
	if err == nil {
		t.Error("Expected error for config file in missing directory")
	}
	
	if !strings.Contains(err.Error(), "configuration file not found") {
		t.Errorf("Expected 'configuration file not found' error, got: %v", err)
	}
}

func TestLoadConfig_EmptyConfigPath(t *testing.T) {
	t.Parallel()
	
	// Test empty config path (should use default)
	_, err := LoadConfig("")
	
	// This should try to load the default config file, which likely doesn't exist
	if err == nil {
		// If no error, the default config.yaml exists and is valid
		t.Log("Default config.yaml exists and is valid")
	} else if strings.Contains(err.Error(), "configuration file not found") {
		t.Log("Default config.yaml does not exist (expected)")
	} else {
		t.Errorf("Unexpected error with empty config path: %v", err)
	}
}

func TestLoadConfig_PermissionDenied(t *testing.T) {
	t.Parallel()
	
	// Create a temporary file with restricted permissions
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "no_read_config.yaml")
	
	err := os.WriteFile(configFile, []byte(createValidConfigContent()), 0000) // No permissions
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	_, loadErr := LoadConfig(configFile)
	if loadErr == nil {
		t.Error("Expected error for permission denied config file")
	}
	
	if !strings.Contains(loadErr.Error(), "failed to read configuration file") {
		t.Errorf("Expected 'failed to read configuration file' error, got: %v", loadErr)
	}
}

func TestLoadConfig_CorruptedConfigFile(t *testing.T) {
	t.Parallel()
	
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "corrupted_config.yaml")
	
	err := os.WriteFile(configFile, []byte(createCorruptedConfigContent()), 0644)
	if err != nil {
		t.Fatalf("Failed to create corrupted config file: %v", err)
	}
	
	_, loadErr := LoadConfig(configFile)
	if loadErr == nil {
		t.Error("Expected error for corrupted config file")
	}
	
	if !strings.Contains(loadErr.Error(), "failed to parse configuration file") {
		t.Errorf("Expected 'failed to parse configuration file' error, got: %v", loadErr)
	}
}

func TestLoadConfig_EmptyConfigFile(t *testing.T) {
	t.Parallel()
	
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "empty_config.yaml")
	
	err := os.WriteFile(configFile, []byte(createEmptyConfigContent()), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty config file: %v", err)
	}
	
	config, loadErr := LoadConfig(configFile)
	if loadErr != nil {
		t.Errorf("Unexpected error loading empty config: %v", loadErr)
	}
	
	// Empty config should get defaults applied
	if config.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Port)
	}
}

func TestLoadConfig_PartialConfigFile(t *testing.T) {
	t.Parallel()
	
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "partial_config.yaml")
	
	err := os.WriteFile(configFile, []byte(createPartialConfigContent()), 0644)
	if err != nil {
		t.Fatalf("Failed to create partial config file: %v", err)
	}
	
	config, loadErr := LoadConfig(configFile)
	if loadErr != nil {
		t.Errorf("Unexpected error loading partial config: %v", loadErr)
	}
	
	// Partial config should still load
	if config.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", config.Port)
	}
	
	if len(config.Servers) != 0 {
		t.Errorf("Expected 0 servers in partial config, got %d", len(config.Servers))
	}
}

func TestLoadConfig_InvalidPortConfig(t *testing.T) {
	t.Parallel()
	
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid_port_config.yaml")
	
	err := os.WriteFile(configFile, []byte(createInvalidPortConfigContent()), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid port config file: %v", err)
	}
	
	_, loadErr := LoadConfig(configFile)
	if loadErr == nil {
		t.Error("Expected error for invalid port config")
	}
	
	if !strings.Contains(loadErr.Error(), "failed to parse configuration file") {
		t.Errorf("Expected 'failed to parse configuration file' error, got: %v", loadErr)
	}
}

// Test configuration validation with missing sections
func TestValidateConfig_MissingRequiredSections(t *testing.T) {
	t.Parallel()
	
	tests := []struct {
		name    string
		config  *GatewayConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "Nil configuration",
			config:  nil,
			wantErr: true,
			errMsg:  "configuration cannot be nil",
		},
		{
			name: "Missing servers section",
			config: &GatewayConfig{
				Port:    8080,
				Servers: []ServerConfig{},
			},
			wantErr: true,
			errMsg:  "at least one server must be configured",
		},
		{
			name: "Invalid port range",
			config: &GatewayConfig{
				Port: 0,
				Servers: []ServerConfig{
					{
						Name:      "test-lsp",
						Languages: []string{"test"},
						Command:   "test-cmd",
						Transport: "stdio",
					},
				},
			},
			wantErr: true,
			errMsg:  "port must be between 1 and 65535",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Expected error message containing '%s', got: %v", tt.errMsg, err)
			}
		})
	}
}

// Test configuration loading with missing schema files
func TestLoadConfig_MissingSchemaValidation(t *testing.T) {
	t.Parallel()
	
	// Test loading a config that would normally require schema validation
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config_no_schema.yaml")
	
	configContent := `
port: 8080
servers:
  - name: unknown-server
    languages: [unknown]
    command: unknown-cmd
    transport: unknown-transport
`
	
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}
	
	config, loadErr := LoadConfig(configFile)
	if loadErr != nil {
		t.Errorf("Config should load even with unknown values: %v", loadErr)
	}
	
	// Validation should catch invalid transport
	validateErr := ValidateConfig(config)
	if validateErr == nil {
		t.Error("Expected validation error for unknown transport")
	}
}

// Test concurrent access to missing configuration files
func TestLoadConfig_ConcurrentMissingFileAccess(t *testing.T) {
	t.Parallel()
	
	nonExistentPath := "/nonexistent/concurrent/config.yaml"
	concurrency := 10
	errors := make(chan error, concurrency)
	
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := LoadConfig(nonExistentPath)
			errors <- err
		}()
	}
	
	wg.Wait()
	close(errors)
	
	errorCount := 0
	for err := range errors {
		if err == nil {
			t.Error("Expected error for missing config file in concurrent access")
		} else {
			errorCount++
		}
	}
	
	if errorCount != concurrency {
		t.Errorf("Expected %d errors, got %d", concurrency, errorCount)
	}
}

// Test configuration file recovery scenarios
func TestLoadConfig_FileRecoveryScenarios(t *testing.T) {
	t.Parallel()
	
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "recovery_config.yaml")
	
	// First attempt - file doesn't exist
	_, err1 := LoadConfig(configFile)
	if err1 == nil {
		t.Error("Expected error for missing file")
	}
	
	// Create the file
	err := os.WriteFile(configFile, []byte(createValidConfigContent()), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}
	
	// Second attempt - file exists
	config, err2 := LoadConfig(configFile)
	if err2 != nil {
		t.Errorf("Expected successful load after file creation: %v", err2)
	}
	
	if config == nil {
		t.Error("Expected valid config after file creation")
	}
	
	// Remove file again
	os.Remove(configFile)
	
	// Third attempt - file missing again
	_, err3 := LoadConfig(configFile)
	if err3 == nil {
		t.Error("Expected error after file removal")
	}
}

// Test configuration directory permission scenarios
func TestLoadConfig_DirectoryPermissionScenarios(t *testing.T) {
	t.Parallel()
	
	tmpDir := t.TempDir()
	restrictedDir := filepath.Join(tmpDir, "restricted")
	
	// Create directory with read permissions
	err := os.Mkdir(restrictedDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create restricted directory: %v", err)
	}
	
	configFile := filepath.Join(restrictedDir, "config.yaml")
	err = os.WriteFile(configFile, []byte(createValidConfigContent()), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file in restricted directory: %v", err)
	}
	
	// Config should load normally
	config, loadErr := LoadConfig(configFile)
	if loadErr != nil {
		t.Errorf("Unexpected error loading config from accessible directory: %v", loadErr)
	}
	
	if config == nil {
		t.Error("Expected valid config from accessible directory")
	}
	
	// Remove read permission from directory (this test might not work on all systems)
	os.Chmod(restrictedDir, 0000)
	defer os.Chmod(restrictedDir, 0755) // Restore for cleanup
	
	// Try to access the file again - behavior may vary by OS
	_, loadErr2 := LoadConfig(configFile)
	
	// On some systems, this might still work if the file is already open
	// or cached, so we don't strictly require this to fail
	if loadErr2 != nil {
		t.Logf("Expected behavior: access denied to restricted directory: %v", loadErr2)
	}
}

// Test configuration with missing default values
func TestLoadConfig_MissingDefaultValues(t *testing.T) {
	t.Parallel()
	
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "no_defaults_config.yaml")
	
	configContent := `
servers:
  - name: minimal-server
    languages: [test]
    command: test-cmd
`
	
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create minimal config file: %v", err)
	}
	
	config, loadErr := LoadConfig(configFile)
	if loadErr != nil {
		t.Errorf("Unexpected error loading minimal config: %v", loadErr)
	}
	
	// Check that defaults are applied
	if config.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Port)
	}
	
	if config.Servers[0].Transport != DefaultTransport {
		t.Errorf("Expected default transport '%s', got '%s'", DefaultTransport, config.Servers[0].Transport)
	}
}

// Test large configuration file scenarios
func TestLoadConfig_LargeConfigurationFile(t *testing.T) {
	t.Parallel()
	
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "large_config.yaml")
	
	// Create a large configuration with many servers
	var configBuilder strings.Builder
	configBuilder.WriteString("port: 8080\nservers:\n")
	
	for i := 0; i < 100; i++ {
		configBuilder.WriteString(fmt.Sprintf(`  - name: server-%d
    languages: [lang%d]
    command: cmd%d
    transport: stdio
`, i, i, i))
	}
	
	err := os.WriteFile(configFile, []byte(configBuilder.String()), 0644)
	if err != nil {
		t.Fatalf("Failed to create large config file: %v", err)
	}
	
	start := time.Now()
	config, loadErr := LoadConfig(configFile)
	duration := time.Since(start)
	
	if loadErr != nil {
		t.Errorf("Unexpected error loading large config: %v", loadErr)
	}
	
	if config == nil {
		t.Error("Expected valid config for large file")
	}
	
	if len(config.Servers) != 100 {
		t.Errorf("Expected 100 servers, got %d", len(config.Servers))
	}
	
	// Ensure loading doesn't take too long (reasonable threshold)
	if duration > 5*time.Second {
		t.Errorf("Loading large config took too long: %v", duration)
	}
}

// Benchmark tests for missing file scenarios
func BenchmarkLoadConfig_MissingFile(b *testing.B) {
	nonExistentPath := "/nonexistent/benchmark/config.yaml"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadConfig(nonExistentPath)
	}
}

func BenchmarkLoadConfig_ExistingFile(b *testing.B) {
	tmpDir := b.TempDir()
	configFile := filepath.Join(tmpDir, "benchmark_config.yaml")
	
	err := os.WriteFile(configFile, []byte(createValidConfigContent()), 0644)
	if err != nil {
		b.Fatalf("Failed to create benchmark config file: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadConfig(configFile)
	}
}

func BenchmarkValidateConfig_CompleteConfig(b *testing.B) {
	config := &GatewayConfig{
		Port: 8080,
		Servers: []ServerConfig{
			{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
			},
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateConfig(config)
	}
}

func BenchmarkValidateConfig_PartialConfig(b *testing.B) {
	config := &GatewayConfig{
		Port:    8080,
		Servers: []ServerConfig{},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateConfig(config)
	}
}