package platform

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// TestInvalidExecutableFiles tests detection of invalid executable files
func TestInvalidExecutableFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		createFile    func(string) error
		expectFailure bool
		errorType     string
	}{
		{
			name: "valid shell script",
			createFile: func(path string) error {
				content := []byte("#!/bin/sh\necho 'hello world'\nexit 0\n")
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: false,
			errorType:     "",
		},
		{
			name: "script without shebang",
			createFile: func(path string) error {
				content := []byte("echo 'hello world'\nexit 0\n")
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: false, // Might still execute depending on shell
			errorType:     "",
		},
		{
			name: "binary with invalid magic number",
			createFile: func(path string) error {
				content := []byte{0x00, 0x00, 0x00, 0x00} // Invalid magic
				content = append(content, make([]byte, 96)...)
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: true,
			errorType:     "format",
		},
		{
			name: "corrupted ELF header",
			createFile: func(path string) error {
				content := []byte{0x7F, 0x45, 0x4C, 0x46} // ELF magic
				// Add corrupted ELF header fields
				corruptedHeader := make([]byte, 60)
				for i := range corruptedHeader {
					corruptedHeader[i] = 0xFF // Invalid values
				}
				content = append(content, corruptedHeader...)
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: true,
			errorType:     "format",
		},
		{
			name: "file with mixed binary and text",
			createFile: func(path string) error {
				content := []byte("#!/bin/sh\necho 'test'\n")
				// Inject binary data
				binaryData := make([]byte, 20)
				_, _ = rand.Read(binaryData)
				content = append(content, binaryData...)
				content = append(content, []byte("\nexit 0\n")...)
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: false, // Shell scripts can contain binary data
			errorType:     "",
		},
		{
			name: "executable with null bytes in critical sections",
			createFile: func(path string) error {
				content := []byte("#!/bin/sh\necho\x00\x01'corrupted'\nexit 0\n")
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: false, // Shell might handle this
			errorType:     "",
		},
		{
			name: "zero-length executable",
			createFile: func(path string) error {
				return os.WriteFile(path, []byte{}, 0755)
			},
			expectFailure: true,
			errorType:     "format",
		},
		{
			name: "executable with invalid UTF-8 sequences",
			createFile: func(path string) error {
				content := []byte("#!/bin/sh\necho '")
				content = append(content, 0xFF, 0xFE, 0xFD) // Invalid UTF-8
				content = append(content, []byte("'\nexit 0\n")...)
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: false, // Shell scripts don't require valid UTF-8
			errorType:     "",
		},
		{
			name: "extremely large executable",
			createFile: func(path string) error {
				content := []byte("#!/bin/sh\n")
				// Add large amount of data to test memory handling
				largeData := make([]byte, 1024*1024) // 1MB
				for i := range largeData {
					largeData[i] = byte('A' + (i % 26))
				}
				content = append(content, largeData...)
				content = append(content, []byte("\nexit 0\n")...)
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: false, // Large files should be handled
			errorType:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.name+"_test")
			err := tt.createFile(testFile)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test file validation
			info, err := os.Stat(testFile)
			if err != nil {
				t.Fatalf("Failed to stat test file: %v", err)
			}

			// Check if file is executable
			if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
				t.Error("Test file should be executable")
				return
			}

			// Test execution
			executor := NewCommandExecutor()
			result, err := executor.Execute(testFile, []string{}, 2*time.Second)

			if tt.expectFailure {
				if err == nil {
					t.Error("Expected executable validation to fail")
				} else if tt.errorType != "" && !strings.Contains(strings.ToLower(err.Error()), tt.errorType) {
					t.Errorf("Expected error type '%s', got: %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Logf("Execution error (may be expected in test env): %v", err)
				}
				if result != nil {
					t.Logf("Execution result: exit=%d, duration=%v", result.ExitCode, result.Duration)
				}
			}
		})
	}
}

// TestVersionInformationCorruption tests detection of corrupted version information
func TestVersionInformationCorruption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		versionOutput   string
		expectedVersion string
		expectError     bool
	}{
		{
			name:            "valid version",
			versionOutput:   "v1.2.3",
			expectedVersion: "1.2.3",
			expectError:     false,
		},
		{
			name:            "version with null bytes",
			versionOutput:   "v1.2\x00.3",
			expectedVersion: "",
			expectError:     true,
		},
		{
			name:            "version with control characters",
			versionOutput:   "v1.2.3\x01\x02\x03",
			expectedVersion: "1.2.3",
			expectError:     false,
		},
		{
			name:            "version with invalid UTF-8",
			versionOutput:   "v1.2.3\xFF\xFE",
			expectedVersion: "",
			expectError:     true,
		},
		{
			name:            "extremely long version string",
			versionOutput:   "v" + strings.Repeat("1.2.3.", 1000) + "final",
			expectedVersion: "",
			expectError:     true,
		},
		{
			name:            "version with mixed encodings",
			versionOutput:   "v1.2.3-α\x80\x81β",
			expectedVersion: "",
			expectError:     true,
		},
		{
			name:            "version with line breaks",
			versionOutput:   "v1.2.3\ncommit abc123\nbuild 456",
			expectedVersion: "1.2.3",
			expectError:     false,
		},
		{
			name:            "empty version output",
			versionOutput:   "",
			expectedVersion: "",
			expectError:     true,
		},
		{
			name:            "version with binary data",
			versionOutput:   string([]byte{0x76, 0x31, 0x2E, 0x32, 0x2E, 0x33, 0xFF, 0x00}),
			expectedVersion: "",
			expectError:     true,
		},
		{
			name:            "version with unicode normalization issues",
			versionOutput:   "v1.2.3-café", // Contains Unicode
			expectedVersion: "1.2.3",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := parseVersionFromOutput(tt.versionOutput)

			if tt.expectError {
				if err == nil {
					t.Error("Expected version parsing error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected version parsing error: %v", err)
				}
				if version != tt.expectedVersion {
					t.Errorf("Expected version '%s', got '%s'", tt.expectedVersion, version)
				}
			}
		})
	}
}

// TestEnvironmentFileCorruption tests corruption in environment configuration files
func TestEnvironmentFileCorruption(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		fileContent   string
		expectError   bool
		errorContains string
	}{
		{
			name: "valid environment file",
			fileContent: `PATH=/usr/bin:/bin
HOME=/home/user
TERM=xterm`,
			expectError: false,
		},
		{
			name: "environment file with null bytes",
			fileContent: `PATH=/usr/bin\x00:/bin
HOME=/home/user`,
			expectError:   true,
			errorContains: "null",
		},
		{
			name:          "environment file with invalid UTF-8",
			fileContent:   "PATH=/usr/bin:/bin\nHOME=\xFF\xFE/invalid",
			expectError:   true,
			errorContains: "utf",
		},
		{
			name:        "environment file with extremely long values",
			fileContent: fmt.Sprintf("PATH=%s\nHOME=/home/user", strings.Repeat("/very/long/path:", 10000)),
			expectError: false, // Long paths should be handled
		},
		{
			name: "environment file with malformed entries",
			fileContent: `PATH=/usr/bin:/bin
=invalid_entry_without_key
HOME=/home/user
INVALID_LINE_WITHOUT_EQUALS`,
			expectError: false, // Should ignore malformed entries
		},
		{
			name:          "environment file with binary corruption",
			fileContent:   string([]byte{0xFF, 0xFE, 'P', 'A', 'T', 'H', '=', '/', 'b', 'i', 'n'}),
			expectError:   true,
			errorContains: "utf",
		},
		{
			name:        "environment file with control characters",
			fileContent: "PATH=/usr/bin:/bin\x01\x02\x03\nHOME=/home/user",
			expectError: false, // Control chars might be stripped
		},
		{
			name:        "empty environment file",
			fileContent: "",
			expectError: false,
		},
		{
			name: "environment file with duplicate keys",
			fileContent: `PATH=/usr/bin
PATH=/bin
HOME=/home/user`,
			expectError: false, // Last value should win
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envFile := filepath.Join(tmpDir, tt.name+".env")
			err := os.WriteFile(envFile, []byte(tt.fileContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write environment file: %v", err)
			}

			envVars, err := parseEnvironmentFile(envFile)

			if tt.expectError {
				if err == nil {
					t.Error("Expected environment file parsing error")
				} else if tt.errorContains != "" && !strings.Contains(strings.ToLower(err.Error()), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected environment parsing error: %v", err)
				}
				t.Logf("Parsed %d environment variables", len(envVars))
			}
		})
	}
}

// TestBinaryFileCorruption tests detection of binary file corruption
func TestBinaryFileCorruption(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		createFile      func(string) error
		expectCorrupted bool
		corruptionType  string
	}{
		{
			name: "valid binary file",
			createFile: func(path string) error {
				// Create a simple but valid binary pattern
				content := make([]byte, 100)
				content[0] = 0x7F // Start of common binary format
				content[1] = 0x45
				content[2] = 0x4C
				content[3] = 0x46
				return os.WriteFile(path, content, 0644)
			},
			expectCorrupted: false,
		},
		{
			name: "binary with random corruption",
			createFile: func(path string) error {
				content := make([]byte, 1000)
				// Fill with random data
				_, _ = rand.Read(content)
				return os.WriteFile(path, content, 0644)
			},
			expectCorrupted: false, // Random data is not necessarily corrupted
		},
		{
			name: "binary with partial corruption",
			createFile: func(path string) error {
				content := make([]byte, 1000)
				// Start with valid pattern
				content[0] = 0x7F
				content[1] = 0x45
				content[2] = 0x4C
				content[3] = 0x46
				// Corrupt middle section
				for i := 500; i < 600; i++ {
					content[i] = 0xFF
				}
				return os.WriteFile(path, content, 0644)
			},
			expectCorrupted: false, // Partial corruption might be valid data
		},
		{
			name: "truncated binary",
			createFile: func(path string) error {
				content := []byte{0x7F, 0x45} // Truncated header
				return os.WriteFile(path, content, 0644)
			},
			expectCorrupted: true,
			corruptionType:  "truncated",
		},
		{
			name: "binary with repeating patterns",
			createFile: func(path string) error {
				content := make([]byte, 1000)
				for i := range content {
					content[i] = 0xAA // Repeating pattern might indicate corruption
				}
				return os.WriteFile(path, content, 0644)
			},
			expectCorrupted: false, // Repeating patterns can be valid
		},
		{
			name: "zero-filled binary",
			createFile: func(path string) error {
				content := make([]byte, 1000) // All zeros
				return os.WriteFile(path, content, 0644)
			},
			expectCorrupted: true,
			corruptionType:  "empty",
		},
		{
			name: "binary with embedded text corruption",
			createFile: func(path string) error {
				content := make([]byte, 100)
				content[0] = 0x7F
				content[1] = 0x45
				content[2] = 0x4C
				content[3] = 0x46
				// Embed corrupted text in binary
				copy(content[50:], []byte("corrupted\x00\x01text"))
				return os.WriteFile(path, content, 0644)
			},
			expectCorrupted: false, // Binary can contain various data
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.name+".bin")
			err := tt.createFile(testFile)
			if err != nil {
				t.Fatalf("Failed to create test binary: %v", err)
			}

			corrupted, corruptionType, err := detectBinaryCorruption(testFile)
			if err != nil {
				t.Errorf("Corruption detection failed: %v", err)
				return
			}

			if corrupted != tt.expectCorrupted {
				t.Errorf("Expected corrupted=%v, got corrupted=%v", tt.expectCorrupted, corrupted)
			}

			if tt.expectCorrupted && tt.corruptionType != "" {
				if !strings.Contains(strings.ToLower(corruptionType), tt.corruptionType) {
					t.Errorf("Expected corruption type '%s', got '%s'", tt.corruptionType, corruptionType)
				}
			}

			t.Logf("Binary corruption detection: corrupted=%v, type='%s'", corrupted, corruptionType)
		})
	}
}

// TestTextFileEncodingCorruption tests text file encoding issues
func TestTextFileEncodingCorruption(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		content       []byte
		expectValid   bool
		encodingIssue string
	}{
		{
			name:        "valid UTF-8 text",
			content:     []byte("Hello, 世界! 🌍"),
			expectValid: true,
		},
		{
			name:          "invalid UTF-8 sequences",
			content:       []byte("Hello\xFF\xFE world"),
			expectValid:   false,
			encodingIssue: "utf8",
		},
		{
			name:          "mixed encoding corruption",
			content:       []byte("Hello\x80\x81 world"),
			expectValid:   false,
			encodingIssue: "utf8",
		},
		{
			name:        "UTF-8 with BOM",
			content:     append([]byte{0xEF, 0xBB, 0xBF}, []byte("Hello world")...),
			expectValid: true,
		},
		{
			name:        "text with null bytes",
			content:     []byte("Hello\x00world"),
			expectValid: true, // Null bytes are valid in binary context
		},
		{
			name:        "text with control characters",
			content:     []byte("Hello\x01\x02\x03world"),
			expectValid: true, // Control chars can be valid
		},
		{
			name:          "truncated UTF-8 multibyte sequence",
			content:       []byte("Hello \xE4\xB8"), // Incomplete Chinese character
			expectValid:   false,
			encodingIssue: "utf8",
		},
		{
			name:        "Latin-1 encoding",
			content:     []byte("Hello\xE9 world"), // é in Latin-1
			expectValid: false,                     // Invalid in UTF-8 context
		},
		{
			name:        "Windows line endings with corruption",
			content:     []byte("Line1\r\nLine2\xFF\r\nLine3"),
			expectValid: false,
		},
		{
			name:        "empty file",
			content:     []byte{},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.name+".txt")
			err := os.WriteFile(testFile, tt.content, 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			isValid, encodingIssue, err := validateTextFileEncoding(testFile)
			if err != nil {
				t.Errorf("Text validation failed: %v", err)
				return
			}

			if isValid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectValid, isValid)
			}

			if !tt.expectValid && tt.encodingIssue != "" {
				if !strings.Contains(strings.ToLower(encodingIssue), tt.encodingIssue) {
					t.Errorf("Expected encoding issue '%s', got '%s'", tt.encodingIssue, encodingIssue)
				}
			}

			t.Logf("Text encoding validation: valid=%v, issue='%s'", isValid, encodingIssue)
		})
	}
}

// TestChecksumValidation tests integrity verification using checksums
func TestChecksumValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		originalData   []byte
		modifyData     func([]byte) []byte
		expectModified bool
	}{
		{
			name:           "no modification",
			originalData:   []byte("Hello, World!"),
			modifyData:     func(data []byte) []byte { return data },
			expectModified: false,
		},
		{
			name:         "single byte change",
			originalData: []byte("Hello, World!"),
			modifyData: func(data []byte) []byte {
				result := make([]byte, len(data))
				copy(result, data)
				result[0] = 'h' // Change 'H' to 'h'
				return result
			},
			expectModified: true,
		},
		{
			name:         "append data",
			originalData: []byte("Hello, World!"),
			modifyData: func(data []byte) []byte {
				return append(data, []byte(" Extra data")...)
			},
			expectModified: true,
		},
		{
			name:         "truncate data",
			originalData: []byte("Hello, World!"),
			modifyData: func(data []byte) []byte {
				return data[:5] // Keep only "Hello"
			},
			expectModified: true,
		},
		{
			name:         "inject null bytes",
			originalData: []byte("Hello, World!"),
			modifyData: func(data []byte) []byte {
				result := make([]byte, len(data)+2)
				copy(result, data[:6])
				result[6] = 0x00
				result[7] = 0x00
				copy(result[8:], data[6:])
				return result
			},
			expectModified: true,
		},
		{
			name:         "reorder bytes",
			originalData: []byte("Hello"),
			modifyData: func(data []byte) []byte {
				result := make([]byte, len(data))
				copy(result, data)
				// Swap first and last bytes
				result[0], result[len(result)-1] = result[len(result)-1], result[0]
				return result
			},
			expectModified: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalFile := filepath.Join(tmpDir, tt.name+"_original")
			modifiedFile := filepath.Join(tmpDir, tt.name+"_modified")

			// Write original file
			err := os.WriteFile(originalFile, tt.originalData, 0644)
			if err != nil {
				t.Fatalf("Failed to write original file: %v", err)
			}

			// Calculate original checksum
			originalChecksum, err := calculateFileChecksum(originalFile)
			if err != nil {
				t.Fatalf("Failed to calculate original checksum: %v", err)
			}

			// Create modified file
			modifiedData := tt.modifyData(tt.originalData)
			err = os.WriteFile(modifiedFile, modifiedData, 0644)
			if err != nil {
				t.Fatalf("Failed to write modified file: %v", err)
			}

			// Calculate modified checksum
			modifiedChecksum, err := calculateFileChecksum(modifiedFile)
			if err != nil {
				t.Fatalf("Failed to calculate modified checksum: %v", err)
			}

			isModified := originalChecksum != modifiedChecksum

			if isModified != tt.expectModified {
				t.Errorf("Expected modified=%v, got modified=%v", tt.expectModified, isModified)
			}

			t.Logf("Checksum validation: original=%s, modified=%s, changed=%v",
				originalChecksum[:8], modifiedChecksum[:8], isModified)
		})
	}
}

// TestConcurrentFileValidation tests file validation under concurrent access
func TestConcurrentFileValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "concurrent_test.txt")

	// Create initial file
	initialContent := []byte("Hello, World!")
	err := os.WriteFile(testFile, initialContent, 0644)
	if err != nil {
		t.Fatalf("Failed to write initial file: %v", err)
	}

	const numGoroutines = 10
	results := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	// Start concurrent validation
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < 5; j++ {
				valid, _, err := validateTextFileEncoding(testFile)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d iteration %d: %w", id, j, err)
					return
				}
				results <- valid
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Modify file concurrently
	go func() {
		time.Sleep(2 * time.Millisecond)
		corruptedContent := []byte("Hello\xFF\xFE World")
		_ = os.WriteFile(testFile, corruptedContent, 0644)

		time.Sleep(2 * time.Millisecond)
		// Restore original content
		_ = os.WriteFile(testFile, initialContent, 0644)
	}()

	// Collect results
	validCount := 0
	invalidCount := 0
	errorCount := 0

	for i := 0; i < numGoroutines*5; i++ {
		select {
		case valid := <-results:
			if valid {
				validCount++
			} else {
				invalidCount++
			}
		case err := <-errors:
			errorCount++
			t.Logf("Expected concurrent validation error: %v", err)
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for concurrent validation")
			return
		}
	}

	t.Logf("Concurrent validation results: %d valid, %d invalid, %d errors",
		validCount, invalidCount, errorCount)
}

// Helper functions for validation logic

func parseVersionFromOutput(output string) (string, error) {
	if !utf8.ValidString(output) {
		return "", fmt.Errorf("invalid UTF-8 in version output")
	}

	if len(output) > 1000 {
		return "", fmt.Errorf("version output too long")
	}

	if strings.Contains(output, "\x00") {
		return "", fmt.Errorf("null bytes in version output")
	}

	// Extract version number (simple pattern matching)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "v") {
			version := strings.TrimPrefix(line, "v")
			// Extract just the version number part
			fields := strings.Fields(version)
			if len(fields) > 0 {
				return fields[0], nil
			}
		}
	}

	if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
		// Try to extract version from first line
		fields := strings.Fields(lines[0])
		for _, field := range fields {
			if strings.Contains(field, ".") {
				return field, nil
			}
		}
	}

	if strings.TrimSpace(output) == "" {
		return "", fmt.Errorf("empty version output")
	}

	return "", fmt.Errorf("could not parse version from output")
}

func parseEnvironmentFile(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if !utf8.Valid(data) {
		return nil, fmt.Errorf("environment file contains invalid UTF-8")
	}

	if bytes.Contains(data, []byte{0x00}) {
		return nil, fmt.Errorf("environment file contains null bytes")
	}

	envVars := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] != "" {
			envVars[parts[0]] = parts[1]
		}
	}

	return envVars, nil
}

func detectBinaryCorruption(filePath string) (bool, string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, "", err
	}

	if len(data) == 0 {
		return true, "empty", nil
	}

	if len(data) < 4 {
		return true, "truncated", nil
	}

	// Check for all zeros (potential corruption)
	allZero := true
	for _, b := range data {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return true, "empty", nil
	}

	// Additional corruption detection logic could be added here
	// For now, we'll be conservative and not flag valid patterns as corrupted

	return false, "", nil
}

func validateTextFileEncoding(filePath string) (bool, string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, "", err
	}

	if len(data) == 0 {
		return true, "", nil
	}

	if !utf8.Valid(data) {
		return false, "invalid UTF-8 encoding", nil
	}

	return true, "", nil
}

func calculateFileChecksum(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Simple checksum calculation (in real implementation, use crypto/sha256)
	checksum := 0
	for _, b := range data {
		checksum += int(b)
	}

	return fmt.Sprintf("%08x", checksum), nil
}

// Benchmark tests for content validation performance

func BenchmarkBinaryCorruptionDetection(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "benchmark.bin")

	// Create test binary
	content := make([]byte, 1024)
	_, _ = rand.Read(content)
	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := detectBinaryCorruption(testFile)
		if err != nil {
			b.Fatalf("Corruption detection failed: %v", err)
		}
	}
}

func BenchmarkTextEncodingValidation(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "benchmark.txt")

	content := []byte("Hello, World! 🌍 This is a test file with Unicode content.")
	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := validateTextFileEncoding(testFile)
		if err != nil {
			b.Fatalf("Encoding validation failed: %v", err)
		}
	}
}

func BenchmarkChecksumCalculation(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "benchmark_checksum.bin")

	content := make([]byte, 10240) // 10KB
	rand.Read(content)
	err := os.WriteFile(testFile, content, 0644)
	if err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := calculateFileChecksum(testFile)
		if err != nil {
			b.Fatalf("Checksum calculation failed: %v", err)
		}
	}
}
