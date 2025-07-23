package config_test

import (
	"crypto/rand"
	"fmt"
	configpkg "lsp-gateway/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestYAMLSyntaxCorruption tests various YAML syntax corruption scenarios
func TestYAMLSyntaxCorruption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		configContent string
		expectError   bool
		errorContains string
	}{
		{
			name: "invalid indentation mixed spaces and tabs",
			configContent: `port: 8080
	servers:
  - name: go-lsp
	  command: gopls`,
			expectError:   true,
			errorContains: "yaml",
		},
		{
			name: "unclosed array bracket",
			configContent: `port: 8080
servers: [
  name: go-lsp
  command: gopls`,
			expectError:   true,
			errorContains: "yaml",
		},
		{
			name: "unclosed map bracket",
			configContent: `port: 8080
servers:
  - {name: go-lsp
    command: gopls`,
			expectError:   true,
			errorContains: "yaml",
		},
		{
			name: "invalid character in key",
			configContent: `port: 8080
servers@invalid: []`,
			expectError:   false, // This is actually valid YAML
			errorContains: "",
		},
		{
			name: "unescaped special characters",
			configContent: `port: 8080
servers:
  - name: go-lsp
    command: "gopls \"--mode=stdio\"`,
			expectError:   true,
			errorContains: "yaml",
		},
		{
			name: "invalid anchor reference",
			configContent: `port: 8080
servers:
  - &invalid-anchor name: go-lsp
    command: *nonexistent-anchor`,
			expectError:   true,
			errorContains: "yaml",
		},
		{
			name: "deeply nested broken structure",
			configContent: `port: 8080
servers:
  - name: go-lsp
    config:
      nested:
        deeply:
          broken: {
            syntax: error`,
			expectError:   true,
			errorContains: "yaml",
		},
		{
			name: "invalid multiline string",
			configContent: `port: 8080
servers:
  - name: go-lsp
    command: |
      gopls
    unfinished-multiline: |
      this multiline string
      is not properly`,
			expectError:   false, // This might actually be valid YAML
			errorContains: "",
		},
		{
			name: "circular anchor reference",
			configContent: `port: 8080
servers:
  - &circular name: go-lsp
    ref: *circular`,
			expectError:   false, // YAML parsers handle circular references gracefully
			errorContains: "",
		},
		{
			name: "invalid flow sequence",
			configContent: `port: 8080
servers: [name: go-lsp, command: gopls, args: [--stdio,]]`,
			expectError:   false, // This is actually valid YAML syntax
			errorContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "corrupt_config.yaml")

			err := os.WriteFile(configFile, []byte(tt.configContent), 0600)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			config, err := configpkg.LoadConfig(configFile)

			if tt.expectError {
				if err == nil {
					t.Error("Expected YAML syntax error but got none")
				} else if tt.errorContains != "" && !strings.Contains(strings.ToLower(err.Error()), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
				if config != nil {
					t.Error("Expected nil config on YAML error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestConfigurationSchemaViolations tests schema-level corruption
func TestConfigurationSchemaViolations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		configContent string
		expectError   bool
		errorType     string
	}{
		{
			name: "port as string instead of int",
			configContent: `port: "not-a-number"
servers: []`,
			expectError: true,
			errorType:   "unmarshal",
		},
		{
			name: "servers as string instead of array",
			configContent: `port: 8080
servers: "not-an-array"`,
			expectError: true,
			errorType:   "unmarshal",
		},
		{
			name: "server without required fields",
			configContent: `port: 8080
servers:
  - invalid: server`,
			expectError: false, // Might load but fail validation
			errorType:   "",
		},
		{
			name: "nested invalid structure",
			configContent: `port: 8080
servers:
  - name: go-lsp
    command: gopls
    args: "should-be-array"`,
			expectError: true,
			errorType:   "unmarshal",
		},
		{
			name: "mixed data types in array",
			configContent: `port: 8080
servers:
  - name: go-lsp
  - "invalid-server-entry"
  - 123`,
			expectError: true,
			errorType:   "unmarshal",
		},
		{
			name: "invalid boolean values",
			configContent: `port: 8080
enabled: "maybe"
servers: []`,
			expectError: false, // Unknown fields might be ignored
			errorType:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "schema_violation.yaml")

			err := os.WriteFile(configFile, []byte(tt.configContent), 0600)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			config, err := configpkg.LoadConfig(configFile)

			if tt.expectError {
				if err == nil {
					t.Error("Expected schema violation error but got none")
				} else if tt.errorType != "" && !strings.Contains(strings.ToLower(err.Error()), tt.errorType) {
					t.Errorf("Expected error type '%s', got: %v", tt.errorType, err)
				}
			} else if err != nil {
				t.Logf("Schema violation test produced error (may be expected): %v", err)
			}

			// If config loaded, try validation
			if config != nil {
				validationErr := configpkg.ValidateConfig(config)
				t.Logf("Validation result for %s: %v", tt.name, validationErr)
			}
		})
	}
}

// TestBinaryDataCorruption tests binary corruption in configuration files
func TestBinaryDataCorruption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		corruptor   func([]byte) []byte
		expectError bool
	}{
		{
			name: "null bytes insertion",
			corruptor: func(data []byte) []byte {
				result := make([]byte, 0, len(data)+10)
				for i, b := range data {
					result = append(result, b)
					if i%20 == 0 {
						result = append(result, 0x00)
					}
				}
				return result
			},
			expectError: true,
		},
		{
			name: "random binary injection",
			corruptor: func(data []byte) []byte {
				result := make([]byte, len(data))
				copy(result, data)
				// Inject random bytes
				for i := 0; i < len(result); i += 50 {
					if i < len(result) {
						randomBytes := make([]byte, 1)
						_, _ = rand.Read(randomBytes)
						result[i] = randomBytes[0]
					}
				}
				return result
			},
			expectError: true,
		},
		{
			name: "utf8 corruption",
			corruptor: func(data []byte) []byte {
				result := make([]byte, 0, len(data)+20)
				for _, b := range data {
					result = append(result, b)
					// Insert invalid UTF-8 sequences
					if b == ':' {
						result = append(result, 0xFF, 0xFE)
					}
				}
				return result
			},
			expectError: true,
		},
		{
			name: "control character injection",
			corruptor: func(data []byte) []byte {
				result := make([]byte, 0, len(data)+10)
				for _, b := range data {
					result = append(result, b)
					if b == ' ' {
						result = append(result, 0x01, 0x02, 0x03) // Control chars
					}
				}
				return result
			},
			expectError: false, // Control chars might be ignored
		},
		{
			name: "bom insertion",
			corruptor: func(data []byte) []byte {
				// Insert UTF-8 BOM at various positions
				result := []byte{0xEF, 0xBB, 0xBF}
				result = append(result, data...)
				// Add more BOMs in the middle
				mid := len(result) / 2
				bom := []byte{0xEF, 0xBB, 0xBF}
				result = append(result[:mid], append(bom, result[mid:]...)...)
				return result
			},
			expectError: false, // BOMs might be handled gracefully
		},
	}

	baseConfig := configpkg.TEST_BASIC_CONFIG

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "binary_corrupt.yaml")

			corruptedData := tt.corruptor([]byte(baseConfig))
			err := os.WriteFile(configFile, corruptedData, 0600)
			if err != nil {
				t.Fatalf("Failed to write corrupted config: %v", err)
			}

			config, err := configpkg.LoadConfig(configFile)

			if tt.expectError {
				if err == nil {
					t.Error("Expected binary corruption error but got none")
				}
				if config != nil {
					t.Error("Expected nil config on binary corruption")
				}
			} else if err != nil {
				t.Logf("Binary corruption test produced error (may be expected): %v", err)
			}
		})
	}
}

// TestPartialFileCorruption tests incomplete and truncated files
func TestPartialFileCorruption(t *testing.T) {
	t.Parallel()

	baseConfig := `port: 8080
debug: false
timeout: 30s
servers:
  - name: go-lsp
    languages: [go, golang]
    command: gopls
    args: ["--mode=stdio"]
    transport: stdio
    timeout: 10s
  - name: python-lsp
    languages: [python, py]
    command: python
    args: ["-m", "pylsp"]
    transport: stdio
logging:
  level: info
  format: json`

	tests := []struct {
		name        string
		truncateAt  int
		expectError bool
	}{
		{
			name:        "truncate at beginning",
			truncateAt:  10,
			expectError: false, // May result in valid but incomplete YAML
		},
		{
			name:        "truncate in middle of key",
			truncateAt:  50,
			expectError: false, // May result in valid but incomplete YAML
		},
		{
			name:        "truncate in array",
			truncateAt:  150,
			expectError: true,
		},
		{
			name:        "truncate at end of valid section",
			truncateAt:  200,
			expectError: false, // Might be valid partial YAML
		},
		{
			name:        "single character",
			truncateAt:  1,
			expectError: true,
		},
		{
			name:        "empty file",
			truncateAt:  0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "truncated.yaml")

			var truncatedData []byte
			if tt.truncateAt > 0 && tt.truncateAt < len(baseConfig) {
				truncatedData = []byte(baseConfig[:tt.truncateAt])
			} else if tt.truncateAt >= len(baseConfig) {
				truncatedData = []byte(baseConfig)
			} else {
				truncatedData = []byte{}
			}

			err := os.WriteFile(configFile, truncatedData, 0600)
			if err != nil {
				t.Fatalf("Failed to write truncated config: %v", err)
			}

			config, err := configpkg.LoadConfig(configFile)

			if tt.expectError {
				if err == nil {
					t.Error("Expected truncation error but got none")
				}
			} else if err != nil {
				t.Logf("Truncation test produced error (may be expected): %v", err)
			}

			if config != nil {
				// Even if loading succeeded, validation might fail
				validationErr := configpkg.ValidateConfig(config)
				t.Logf("Validation result for truncated config: %v", validationErr)
			}
		})
	}
}

// TestBusinessRuleViolations tests logical corruption in configuration
func TestBusinessRuleViolations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		configContent         string
		expectLoadOK          bool
		expectValidationError bool
	}{
		{
			name: "invalid port range high",
			configContent: `port: 999999
servers: []`,
			expectLoadOK:          true,
			expectValidationError: true,
		},
		{
			name: "invalid port range low",
			configContent: `port: -1
servers: []`,
			expectLoadOK:          true,
			expectValidationError: true,
		},
		{
			name: "duplicate server names",
			configContent: `port: 8080
servers:
  - name: duplicate
    command: gopls
    languages: [go]
  - name: duplicate
    command: python
    languages: [python]`,
			expectLoadOK:          true,
			expectValidationError: true,
		},
		{
			name: "server without command",
			configContent: `port: 8080
servers:
  - name: invalid-server
    languages: [go]`,
			expectLoadOK:          true,
			expectValidationError: true,
		},
		{
			name: "server without languages",
			configContent: `port: 8080
servers:
  - name: invalid-server
    command: gopls`,
			expectLoadOK:          true,
			expectValidationError: true,
		},
		{
			name: "invalid transport type",
			configContent: `port: 8080
servers:
  - name: go-lsp
    command: gopls
    languages: [go]
    transport: invalid-transport`,
			expectLoadOK:          true,
			expectValidationError: true,
		},
		{
			name: "extremely long server name",
			configContent: fmt.Sprintf(`port: 8080
servers:
  - name: "%s"
    command: gopls
    languages: [go]`, strings.Repeat("a", 1000)),
			expectLoadOK:          true,
			expectValidationError: true,
		},
		{
			name: "command with invalid characters",
			configContent: `port: 8080
servers:
  - name: go-lsp
    command: "gop\x00ls"
    languages: [go]`,
			expectLoadOK:          true,
			expectValidationError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "business_rule_violation.yaml")

			err := os.WriteFile(configFile, []byte(tt.configContent), 0600)
			if err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			config, err := configpkg.LoadConfig(configFile)

			if tt.expectLoadOK {
				if err != nil {
					t.Errorf("Expected successful load but got error: %v", err)
				}
				if config == nil {
					t.Error("Expected config to be loaded")
				}
			} else {
				if err == nil {
					t.Error("Expected load error but got none")
				}
				return
			}

			// Test validation
			if config != nil {
				validationErr := configpkg.ValidateConfig(config)
				if tt.expectValidationError {
					if validationErr == nil {
						t.Error("Expected validation error but got none")
					}
				} else {
					if validationErr != nil {
						t.Errorf("Unexpected validation error: %v", validationErr)
					}
				}
			}
		})
	}
}

// TestConcurrentCorruptionDetection tests corruption detection under concurrent access
func TestConcurrentCorruptionDetection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "concurrent_corrupt.yaml")

	// Create a base valid config
	validConfig := configpkg.TEST_BASIC_CONFIG

	err := os.WriteFile(configFile, []byte(validConfig), 0600)
	if err != nil {
		t.Fatalf("Failed to write base config: %v", err)
	}

	// Test concurrent reads while we corrupt the file
	const numReaders = 10
	errors := make(chan error, numReaders)
	done := make(chan bool, numReaders)

	// Start concurrent readers
	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < 5; j++ {
				_, err := configpkg.LoadConfig(configFile)
				if err != nil {
					errors <- fmt.Errorf("reader %d iteration %d: %w", id, j, err)
				}
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Corrupt the file while readers are accessing it
	go func() {
		time.Sleep(2 * time.Millisecond)
		corruptData := []byte("invalid: yaml: content\x00\x01")
		_ = os.WriteFile(configFile, corruptData, 0600)

		time.Sleep(2 * time.Millisecond)
		// Restore valid config
		_ = os.WriteFile(configFile, []byte(validConfig), 0600)
	}()

	// Wait for all readers to complete
	errorCount := 0
	for i := 0; i < numReaders; i++ {
		select {
		case err := <-errors:
			errorCount++
			t.Logf("Expected concurrent access error: %v", err)
		case <-done:
			// Reader completed successfully
		}
	}

	t.Logf("Concurrent corruption test completed: %d errors out of %d readers", errorCount, numReaders)
}

// TestCorruptionRecoveryScenarios tests recovery after corruption is fixed
func TestCorruptionRecoveryScenarios(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "recovery_test.yaml")

	validConfig := configpkg.TEST_BASIC_CONFIG

	// Test recovery from various corruption scenarios
	corruptionScenarios := []struct {
		name        string
		corruptData string
	}{
		{
			name:        "yaml syntax error",
			corruptData: "invalid: yaml: [unclosed",
		},
		{
			name:        "binary corruption",
			corruptData: "port: 8080\x00\x01servers: []",
		},
		{
			name:        "truncated file",
			corruptData: "port: 808",
		},
		{
			name:        "empty file",
			corruptData: "",
		},
	}

	for _, scenario := range corruptionScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Step 1: Create corrupt file
			err := os.WriteFile(configFile, []byte(scenario.corruptData), 0600)
			if err != nil {
				t.Fatalf("Failed to write corrupt config: %v", err)
			}

			// Step 2: Verify corruption is detected (either at load time or validation time)
			config, err := configpkg.LoadConfig(configFile)
			if err == nil && config != nil {
				// If loading succeeded, validation should fail for corrupted configs
				validationErr := configpkg.ValidateConfig(config)
				if validationErr == nil {
					t.Error("Expected corruption to be detected via loading or validation")
				}
			} else if err == nil {
				t.Error("Expected corruption to be detected")
			}

			// Step 3: Fix the file
			err = os.WriteFile(configFile, []byte(validConfig), 0600)
			if err != nil {
				t.Fatalf("Failed to write fixed config: %v", err)
			}

			// Step 4: Verify recovery
			config, err = configpkg.LoadConfig(configFile)
			if err != nil {
				t.Errorf("Failed to recover from %s corruption: %v", scenario.name, err)
			}

			if config == nil {
				t.Error("Expected config to be loaded after recovery")
			}

			// Step 5: Verify the recovered config is valid
			if config != nil {
				validationErr := configpkg.ValidateConfig(config)
				if validationErr != nil {
					t.Errorf("Recovered config failed validation: %v", validationErr)
				}
			}
		})
	}
}

// TestChecksum ValidationScenarios tests integrity validation mechanisms
func TestChecksumValidationScenarios(t *testing.T) {
	t.Parallel()

	baseConfig := configpkg.TEST_BASIC_CONFIG

	tests := []struct {
		name               string
		modifier           func(string) string
		shouldDetectChange bool
	}{
		{
			name:               "no modification",
			modifier:           func(s string) string { return s },
			shouldDetectChange: false,
		},
		{
			name:               "whitespace addition",
			modifier:           func(s string) string { return s + "   " },
			shouldDetectChange: false, // YAML parsing ignores extra whitespace
		},
		{
			name:               "comment addition",
			modifier:           func(s string) string { return s + "\n# comment" },
			shouldDetectChange: false, // YAML parsing ignores comments
		},
		{
			name:               "port modification",
			modifier:           func(s string) string { return strings.Replace(s, "8080", "8081", 1) },
			shouldDetectChange: true,
		},
		{
			name:               "subtle character change",
			modifier:           func(s string) string { return strings.Replace(s, "gopls", "gop1s", 1) },
			shouldDetectChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "checksum_test.yaml")

			originalData := []byte(baseConfig)
			modifiedData := []byte(tt.modifier(baseConfig))

			// Write original
			err := os.WriteFile(configFile, originalData, 0600)
			if err != nil {
				t.Fatalf("Failed to write original config: %v", err)
			}

			// Load original to establish baseline
			originalConfig, err := configpkg.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load original config: %v", err)
			}

			// Write modified
			err = os.WriteFile(configFile, modifiedData, 0600)
			if err != nil {
				t.Fatalf("Failed to write modified config: %v", err)
			}

			// Load modified
			modifiedConfig, err := configpkg.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load modified config: %v", err)
			}

			// Compare configurations
			changed := !configsEqual(originalConfig, modifiedConfig)

			if tt.shouldDetectChange && !changed {
				t.Error("Expected change to be detected but configurations are equal")
			} else if !tt.shouldDetectChange && changed {
				t.Error("Expected no change but configurations differ")
			}
		})
	}
}

// Helper function to compare configurations
func configsEqual(c1, c2 *configpkg.GatewayConfig) bool {
	if c1 == nil || c2 == nil {
		return c1 == c2
	}

	if c1.Port != c2.Port {
		return false
	}

	if len(c1.Servers) != len(c2.Servers) {
		return false
	}

	for i := range c1.Servers {
		s1, s2 := c1.Servers[i], c2.Servers[i]
		if s1.Name != s2.Name || s1.Command != s2.Command || s1.Transport != s2.Transport {
			return false
		}

		if len(s1.Languages) != len(s2.Languages) {
			return false
		}

		for j := range s1.Languages {
			if s1.Languages[j] != s2.Languages[j] {
				return false
			}
		}

		if len(s1.Args) != len(s2.Args) {
			return false
		}

		for j := range s1.Args {
			if s1.Args[j] != s2.Args[j] {
				return false
			}
		}
	}

	return true
}

// Benchmark tests for corruption detection performance
func BenchmarkYAMLCorruptionDetection(b *testing.B) {
	tmpDir := b.TempDir()
	configFile := filepath.Join(tmpDir, "benchmark_corrupt.yaml")

	corruptConfig := `port: 8080
servers: [
  name: unclosed-array`

	err := os.WriteFile(configFile, []byte(corruptConfig), 0600)
	if err != nil {
		b.Fatalf("Failed to write corrupt config: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := configpkg.LoadConfig(configFile)
		_ = err // Ignore errors in benchmark
	}
}

func BenchmarkBinaryCorruptionDetection(b *testing.B) {
	tmpDir := b.TempDir()
	configFile := filepath.Join(tmpDir, "benchmark_binary.yaml")

	binaryCorrupt := []byte("port: 8080\x00\x01\x02servers: []")
	err := os.WriteFile(configFile, binaryCorrupt, 0600)
	if err != nil {
		b.Fatalf("Failed to write binary corrupt config: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := configpkg.LoadConfig(configFile)
		_ = err // Ignore errors in benchmark
	}
}
