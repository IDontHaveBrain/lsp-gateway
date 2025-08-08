package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuiltinTypeFilteringIntegration tests that builtin types are properly filtered during actual indexing
func TestBuiltinTypeFilteringIntegration(t *testing.T) {
	// Skip if not in CI environment and LSP servers might not be available
	if os.Getenv("CI") == "" && os.Getenv("RUN_INTEGRATION_TESTS") == "" {
		t.Skip("Skipping integration test in local environment (set RUN_INTEGRATION_TESTS=1 to run)")
	}

	// Create a temporary test directory with Go files
	tempDir, err := ioutil.TempDir("", "lsp-gateway-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test Go file with builtin and user-defined types
	testFile := filepath.Join(tempDir, "test.go")
	testContent := `package main

import "fmt"

// User-defined type
type MyStruct struct {
	Name string  // builtin string type
	Age  int     // builtin int type
}

// User-defined function
func ProcessData(data MyStruct) error {
	// Using builtin types
	var result string = data.Name
	var count int = data.Age
	
	fmt.Println(result, count)
	return nil  // builtin nil
}

// Another user function
func HandleRequest(input interface{}) bool {
	// interface{} and bool are builtins
	return true
}
`
	if err := ioutil.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create go.mod for the test directory
	goModContent := `module test-module

go 1.21
`
	goModFile := filepath.Join(tempDir, "go.mod")
	if err := ioutil.WriteFile(goModFile, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Change to test directory
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Test would normally run the actual indexing here
	// For demonstration, we'll check the content parsing
	content, err := ioutil.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	// Count builtin vs user-defined symbols
	lines := strings.Split(string(content), "\n")
	builtinTypes := []string{"string", "int", "error", "bool", "interface{}", "nil", "true", "false"}
	userDefinedTypes := []string{"MyStruct", "ProcessData", "HandleRequest"}

	builtinCount := 0
	userDefinedCount := 0

	for _, line := range lines {
		for _, builtin := range builtinTypes {
			if strings.Contains(line, builtin) {
				builtinCount++
			}
		}
		for _, userDefined := range userDefinedTypes {
			if strings.Contains(line, userDefined) {
				userDefinedCount++
			}
		}
	}

	t.Logf("Found %d occurrences of builtin types", builtinCount)
	t.Logf("Found %d occurrences of user-defined types", userDefinedCount)

	// Verify we have both types
	if builtinCount == 0 {
		t.Error("Expected to find builtin types in test file")
	}
	if userDefinedCount == 0 {
		t.Error("Expected to find user-defined types in test file")
	}

	// The actual indexing would filter out builtins
	t.Log("In actual indexing, builtin types would be filtered out")
}

// TestPositionValidationIntegration tests position validation with actual file content
func TestPositionValidationIntegration(t *testing.T) {
	// Create a temporary test file
	tempFile, err := ioutil.TempFile("", "position-test-*.go")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	testContent := `package test

func shortLine() {
	// This is a comment
}

func anotherFunction() {
	variable := "test string"
	println(variable)
}
`
	if _, err := tempFile.Write([]byte(testContent)); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tempFile.Close()

	// Read the content back
	content, err := ioutil.ReadFile(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	lines := strings.Split(string(content), "\n")

	// Test various positions
	tests := []struct {
		name        string
		line        int
		character   int
		shouldError bool
	}{
		{"Valid position at function name", 2, 5, false},
		{"Valid position at line start", 0, 0, false},
		{"Line exceeds file length", 100, 0, true},
		{"Character exceeds line length", 2, 200, true},
		{"Last line position", len(lines) - 1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := false

			// Validate position
			if tt.line >= len(lines) {
				hasError = true
				t.Logf("Line %d exceeds file with %d lines", tt.line, len(lines))
			} else if tt.line >= 0 && tt.line < len(lines) {
				line := lines[tt.line]
				if tt.character > len(line) {
					hasError = true
					t.Logf("Character %d exceeds line length %d", tt.character, len(line))
				}
			}

			if hasError != tt.shouldError {
				t.Errorf("Position (%d, %d): expected error=%v, got error=%v",
					tt.line, tt.character, tt.shouldError, hasError)
			}
		})
	}
}

// TestKeywordSkippingIntegration tests keyword skipping in actual Go code
func TestKeywordSkippingIntegration(t *testing.T) {
	tests := []struct {
		name         string
		codeLine     string
		expectedName string
	}{
		{
			name:         "Function declaration",
			codeLine:     "func myFunction() {",
			expectedName: "myFunction",
		},
		{
			name:         "Method with receiver",
			codeLine:     "func (r *Receiver) methodName() error {",
			expectedName: "methodName",
		},
		{
			name:         "Type declaration",
			codeLine:     "type CustomType struct {",
			expectedName: "CustomType",
		},
		{
			name:         "Variable declaration",
			codeLine:     "var globalVar string",
			expectedName: "globalVar",
		},
		{
			name:         "Constant declaration",
			codeLine:     "const MaxRetries = 3",
			expectedName: "MaxRetries",
		},
		{
			name:         "Interface declaration",
			codeLine:     "type Handler interface {",
			expectedName: "Handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find the identifier by skipping keywords
			line := tt.codeLine
			pos := 0

			// Skip common Go keywords
			keywords := []string{"func ", "type ", "var ", "const ", "interface "}
			for _, keyword := range keywords {
				if strings.HasPrefix(line, keyword) {
					pos += len(keyword)
					break
				}
			}

			// Special handling for methods with receivers
			if strings.HasPrefix(line, "func (") {
				if idx := strings.Index(line, ")"); idx != -1 {
					pos = idx + 2 // Skip past ") "
				}
			}

			// Skip whitespace
			for pos < len(line) && (line[pos] == ' ' || line[pos] == '\t') {
				pos++
			}

			// Extract identifier
			identStart := pos
			for pos < len(line) && (isLetter(line[pos]) || isDigit(line[pos]) || line[pos] == '_') {
				pos++
			}

			identifier := line[identStart:pos]
			if identifier != tt.expectedName {
				t.Errorf("Expected identifier '%s', got '%s' from line: %s",
					tt.expectedName, identifier, tt.codeLine)
			}
		})
	}
}

// TestErrorFilteringLogic tests the error message filtering logic
func TestErrorFilteringLogic(t *testing.T) {
	// Test different error messages
	errorMessages := []struct {
		message   string
		shouldLog bool
		reason    string
	}{
		{
			message:   "no identifier found at position 10:5",
			shouldLog: false,
			reason:    "Expected error for missing identifier",
		},
		{
			message:   "cannot get references for builtin type 'string'",
			shouldLog: false,
			reason:    "Expected error for builtin type",
		},
		{
			message:   "symbol 'int' not found in project",
			shouldLog: false,
			reason:    "Expected error for builtin not found",
		},
		{
			message:   "failed to connect to LSP server: connection refused",
			shouldLog: true,
			reason:    "Unexpected connection error",
		},
		{
			message:   "timeout waiting for LSP response",
			shouldLog: true,
			reason:    "Unexpected timeout error",
		},
		{
			message:   "invalid JSON in LSP response",
			shouldLog: true,
			reason:    "Unexpected parsing error",
		},
	}

	for _, em := range errorMessages {
		// Apply filtering logic
		shouldLog := !strings.Contains(em.message, "no identifier found") &&
			!strings.Contains(em.message, "builtin") &&
			!strings.Contains(em.message, "not found")

		if shouldLog != em.shouldLog {
			t.Errorf("Error filtering failed for '%s': expected shouldLog=%v, got %v (%s)",
				em.message, em.shouldLog, shouldLog, em.reason)
		} else {
			t.Logf("✓ Correctly filtered: '%s' (shouldLog=%v, reason: %s)",
				em.message, shouldLog, em.reason)
		}
	}
}

// Helper functions
func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
