package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigFilePermissionErrors tests various file permission error scenarios
func TestConfigFilePermissionErrors(t *testing.T) {
	t.Parallel()

	t.Run("file exists but not readable", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping permission test when running as root")
		}

		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")

		// Create file and remove read permissions
		err := os.WriteFile(configFile, []byte("port: 8080\nservers: []"), 0200)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		_, err = LoadConfig(configFile)
		if err == nil {
			t.Error("Expected error when loading unreadable config file")
		}

		if !strings.Contains(err.Error(), "permission denied") &&
			!strings.Contains(err.Error(), "access is denied") {
			t.Errorf("Expected permission error, got: %v", err)
		}
	})

	t.Run("directory instead of file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config.yaml")

		err := os.Mkdir(configDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		_, err = LoadConfig(configDir)
		if err == nil {
			t.Error("Expected error when loading directory as config file")
		}
	})

	t.Run("invalid file path characters", func(t *testing.T) {
		invalidPath := "/\x00invalid\x00path/config.yaml"
		_, err := LoadConfig(invalidPath)
		if err == nil {
			t.Error("Expected error for invalid file path")
		}
	})
}

// TestConfigCorruptionScenarios tests various data corruption scenarios
func TestConfigCorruptionScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		configContent string
		expectedError string
	}{
		{
			name:          "truncated YAML",
			configContent: "port: 8080\nservers:\n  - name: go",
			expectedError: "unmarshal",
		},
		{
			name:          "mixed indentation",
			configContent: "port: 8080\n\tservers:\n  - name: go-lsp\n\t  command: gopls",
			expectedError: "yaml",
		},
		{
			name:          "binary data in YAML",
			configContent: "port: 8080\x00\x01\x02servers: []",
			expectedError: "unmarshal",
		},
		{
			name:          "extremely long values",
			configContent: "port: 8080\nservers:\n  - name: " + strings.Repeat("a", 10000) + "\n    command: gopls",
			expectedError: "",
		},
		{
			name:          "unicode control characters",
			configContent: "port: 8080\nservers:\n  - name: \"go\u0000lsp\"\n    command: gopls",
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "config.yaml")

			err := os.WriteFile(configFile, []byte(tt.configContent), 0600)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			config, err := LoadConfig(configFile)

			if tt.expectedError == "" {
				// Test should succeed but validate the config
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if config != nil {
					validateErr := ValidateConfig(config)
					if validateErr != nil {
						t.Logf("Config validation failed as expected: %v", validateErr)
					}
				}
			} else {
				// Test should fail
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(strings.ToLower(err.Error()), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
				}
			}
		})
	}
}

// TestConfigValidationEdgeCases tests edge cases in configuration validation
func TestConfigValidationEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  *GatewayConfig
		wantErr bool
	}{
		{
			name: "extremely high port number",
			config: &GatewayConfig{
				Port: 99999,
				Servers: []ServerConfig{
					{Name: "test", Languages: []string{"go"}, Command: "gopls", Transport: "stdio"},
				},
			},
			wantErr: true,
		},
		{
			name: "reserved port number",
			config: &GatewayConfig{
				Port: 22, // SSH port
				Servers: []ServerConfig{
					{Name: "test", Languages: []string{"go"}, Command: "gopls", Transport: "stdio"},
				},
			},
			wantErr: false, // Should be allowed but warned about
		},
		{
			name: "empty language in server",
			config: &GatewayConfig{
				Port: 8080,
				Servers: []ServerConfig{
					{Name: "test", Languages: []string{""}, Command: "gopls", Transport: "stdio"},
				},
			},
			wantErr: true,
		},
		{
			name: "whitespace-only language",
			config: &GatewayConfig{
				Port: 8080,
				Servers: []ServerConfig{
					{Name: "test", Languages: []string{"   "}, Command: "gopls", Transport: "stdio"},
				},
			},
			wantErr: true,
		},
		{
			name: "command with null bytes",
			config: &GatewayConfig{
				Port: 8080,
				Servers: []ServerConfig{
					{Name: "test", Languages: []string{"go"}, Command: "gop\x00ls", Transport: "stdio"},
				},
			},
			wantErr: true,
		},
		{
			name: "args with empty strings",
			config: &GatewayConfig{
				Port: 8080,
				Servers: []ServerConfig{
					{Name: "test", Languages: []string{"go"}, Command: "gopls", Args: []string{""}, Transport: "stdio"},
				},
			},
			wantErr: false, // Empty args might be valid
		},
		{
			name: "very long server name",
			config: &GatewayConfig{
				Port: 8080,
				Servers: []ServerConfig{
					{Name: strings.Repeat("a", 1000), Languages: []string{"go"}, Command: "gopls", Transport: "stdio"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfigMemoryExhaustionHandling tests handling of memory exhaustion scenarios
func TestConfigMemoryExhaustionHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory exhaustion test in short mode")
	}

	t.Parallel()

	t.Run("very large config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "large_config.yaml")

		// Create a very large config file
		var content strings.Builder
		content.WriteString("port: 8080\nservers:\n")

		// Add many servers to test memory handling
		for i := 0; i < 1000; i++ {
			content.WriteString("  - name: server")
			content.WriteString(strings.Repeat("a", 100)) // Long names
			content.WriteString("\n    languages: [go]\n    command: gopls\n    transport: stdio\n")
		}

		err := os.WriteFile(configFile, []byte(content.String()), 0600)
		if err != nil {
			t.Fatalf("Failed to write large config file: %v", err)
		}

		// This should either succeed or fail gracefully
		config, err := LoadConfig(configFile)
		if err != nil {
			t.Logf("Large config loading failed as expected: %v", err)
		} else {
			// If it loads, validation might fail due to duplicate names
			validateErr := ValidateConfig(config)
			t.Logf("Large config validation result: %v", validateErr)
		}
	})
}

// TestConfigConcurrentAccess tests concurrent access to configuration loading
func TestConfigConcurrentAccess(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "concurrent_config.yaml")

	configContent := `
port: 8080
servers:
  - name: go-lsp
    languages: [go]
    command: gopls
    transport: stdio
`

	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test concurrent loading
	const numGoroutines = 50
	errors := make(chan error, numGoroutines)
	configs := make(chan *GatewayConfig, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			config, err := LoadConfig(configFile)
			if err != nil {
				errors <- err
				return
			}
			configs <- config
		}()
	}

	// Collect results
	errorCount := 0
	configCount := 0
	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-errors:
			errorCount++
			t.Logf("Concurrent load error: %v", err)
		case config := <-configs:
			configCount++
			if config == nil {
				t.Error("Got nil config from concurrent load")
			}
		}
	}

	if configCount == 0 {
		t.Error("No configs loaded successfully")
	}

	t.Logf("Concurrent access results: %d successes, %d errors", configCount, errorCount)
}

// TestConfigRecoveryFromErrors tests recovery scenarios
func TestConfigRecoveryFromErrors(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "recovery_config.yaml")

	// Test recovery from temporary permission issues
	t.Run("temporary permission issue", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping permission test when running as root")
		}

		// Create file without read permissions
		err := os.WriteFile(configFile, []byte("port: 8080\nservers: []"), 0200)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		// First attempt should fail
		_, err = LoadConfig(configFile)
		if err == nil {
			t.Error("Expected error for unreadable file")
		}

		// Fix permissions
		err = os.Chmod(configFile, 0600)
		if err != nil {
			t.Fatalf("Failed to fix permissions: %v", err)
		}

		// Second attempt should succeed
		config, err := LoadConfig(configFile)
		if err != nil {
			t.Errorf("Expected recovery after fixing permissions: %v", err)
		}
		if config == nil {
			t.Error("Expected config after recovery")
		}
	})
}
