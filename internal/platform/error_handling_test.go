package platform

import (
	"errors"
	"testing"
)

// Test PlatformError creation and methods
func TestPlatformError(t *testing.T) {
	baseErr := errors.New("base error")
	platformErr := NewPlatformError(ErrCodePackageManagerFailed, "test operation failed", "test problem", "test suggestion")
	platformErr = platformErr.WithCause(baseErr)

	// Test Error() method
	errorMsg := platformErr.Error()
	if errorMsg == "" {
		t.Error("Error() should return non-empty string")
	}
	if !contains(errorMsg, "test operation failed") {
		t.Errorf("Error message should contain message, got: %s", errorMsg)
	}

	// Test Unwrap() method
	unwrapped := platformErr.Unwrap()
	if unwrapped != baseErr {
		t.Errorf("Expected unwrapped error to be baseErr, got: %v", unwrapped)
	}

	// Test Is() method - comparing same type
	otherPlatformErr := NewPlatformError(ErrCodePackageManagerFailed, "other message", "other problem", "other suggestion")
	if !platformErr.Is(otherPlatformErr) {
		t.Error("Is() should return true for same error type")
	}

	// Create a different type of platform error using a constructor that sets different Type
	differentPlatformErr := NewUnsupportedPlatformError("custom-os")
	if platformErr.Is(differentPlatformErr) {
		t.Error("Is() should return false for different error type")
	}

	// Test GetUserGuidance() method
	guidance := platformErr.GetUserGuidance()
	if guidance.Problem != "test problem" {
		t.Errorf("Expected problem 'test problem', got: %s", guidance.Problem)
	}
	if guidance.Suggestion != "test suggestion" {
		t.Errorf("Expected suggestion 'test suggestion', got: %s", guidance.Suggestion)
	}

	// Test WithSystemInfo() method
	systemInfo := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}
	enrichedErr := platformErr.WithSystemInfo(systemInfo)
	if enrichedErr == nil {
		t.Error("WithSystemInfo() should return non-nil error")
	}

	// Test WithCause() method
	newCause := errors.New("new cause")
	errWithCause := platformErr.WithCause(newCause)
	if errWithCause == nil {
		t.Error("WithCause() should return non-nil error")
	}
	if errWithCause.Unwrap() != newCause {
		t.Error("WithCause() should set the new cause")
	}
}

// Test PlatformError with nil cause
func TestPlatformErrorNilCause(t *testing.T) {
	platformErr := NewPlatformError(ErrCodeInsufficientPermissions, "test without cause", "test problem", "test suggestion")

	// Test Error() method
	errorMsg := platformErr.Error()
	if !contains(errorMsg, "test without cause") {
		t.Errorf("Error message should contain message, got: %s", errorMsg)
	}

	// Test Unwrap() method with nil cause
	unwrapped := platformErr.Unwrap()
	if unwrapped != nil {
		t.Errorf("Expected unwrapped error to be nil, got: %v", unwrapped)
	}

	// Test Is() method with different types
	otherErr := NewUnsupportedPlatformError("custom-os")
	if platformErr.Is(otherErr) {
		t.Error("Is() should return false for different error types")
	}
}

// Test ExecutionError creation and methods
func TestExecutionError(t *testing.T) {
	baseErr := errors.New("command failed")
	execErr := NewExecutionError("ls", 1, "permission denied", baseErr)

	// Test Error() method
	errorMsg := execErr.Error()
	if errorMsg == "" {
		t.Error("Error() should return non-empty string")
	}

	// Should contain command and exit code information
	if !contains(errorMsg, "ls") {
		t.Error("Error message should contain command name")
	}
	if !contains(errorMsg, "exit code 1") {
		t.Error("Error message should contain exit code")
	}

	// Test Unwrap() method
	unwrapped := execErr.Unwrap()
	if unwrapped != baseErr {
		t.Errorf("Expected unwrapped error to be baseErr, got: %v", unwrapped)
	}
}

// Test ExecutionError with zero exit code
func TestExecutionErrorZeroExitCode(t *testing.T) {
	execErr := NewExecutionError("echo", 0, "test output", nil)

	errorMsg := execErr.Error()
	if !contains(errorMsg, "echo") {
		t.Error("Error message should contain command name")
	}
	if !contains(errorMsg, "exit code 0") {
		t.Error("Error message should contain exit code even when 0")
	}
}

// Test PackageManagerError creation and methods
func TestPackageManagerError(t *testing.T) {
	baseErr := errors.New("package not found")
	pkgErr := NewPackageManagerError("apt", "install", "nonexistent-package", "package not found", baseErr)

	// Test Error() method
	errorMsg := pkgErr.Error()
	if errorMsg == "" {
		t.Error("Error() should return non-empty string")
	}

	// Should contain package manager and operation information
	if !contains(errorMsg, "apt") {
		t.Error("Error message should contain package manager name")
	}
	if !contains(errorMsg, "install") {
		t.Error("Error message should contain operation")
	}
	if !contains(errorMsg, "nonexistent-package") {
		t.Error("Error message should contain package name")
	}

	// Test Unwrap() method
	unwrapped := pkgErr.Unwrap()
	if unwrapped != baseErr {
		t.Errorf("Expected unwrapped error to be baseErr, got: %v", unwrapped)
	}
}

// Test UnsupportedPlatformError creation and methods
func TestUnsupportedPlatformError(t *testing.T) {
	unsupportedErr := NewUnsupportedPlatformError("custom-os")

	// Test Error() method
	errorMsg := unsupportedErr.Error()
	if errorMsg == "" {
		t.Error("Error() should return non-empty string")
	}

	// Should contain platform information
	if !contains(errorMsg, "custom-os") {
		t.Error("Error message should contain platform name")
	}
	if !contains(errorMsg, "not supported") {
		t.Error("Error message should indicate platform is not supported")
	}
}

// Test InsufficientPermissionsError creation and methods
func TestInsufficientPermissionsError(t *testing.T) {
	permErr := NewInsufficientPermissionsError("install package", "/usr/local/bin")

	// Test Error() method
	errorMsg := permErr.Error()
	if errorMsg == "" {
		t.Error("Error() should return non-empty string")
	}

	// Should contain operation and path
	if !contains(errorMsg, "install package") {
		t.Error("Error message should contain operation")
	}
	if !contains(errorMsg, "/usr/local/bin") {
		t.Error("Error message should contain path")
	}

	// Test user guidance
	guidance := permErr.GetUserGuidance()
	if !contains(guidance.Problem, "install package") {
		t.Error("User guidance should contain operation")
	}
	if len(guidance.RecoverySteps) == 0 {
		t.Error("User guidance should contain recovery steps")
	}
}

// Test InsufficientPermissionsError with empty path
func TestInsufficientPermissionsErrorEmptyPath(t *testing.T) {
	permErr := NewInsufficientPermissionsError("restricted operation", "")

	errorMsg := permErr.Error()
	if !contains(errorMsg, "restricted operation") {
		t.Error("Error message should contain operation even with empty path")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test error chaining and composition
func TestErrorChaining(t *testing.T) {
	// Create a chain of errors
	rootErr := errors.New("root cause")
	execErr := NewExecutionError("failed-command", 1, "stderr output", rootErr)
	platformErr := NewPlatformError(ErrCodePackageManagerFailed, "command execution failed", "problem", "suggestion")
	platformErr = platformErr.WithCause(execErr)

	// Test error unwrapping chain
	if platformErr.Unwrap() != execErr {
		t.Error("Platform error should unwrap to execution error")
	}

	if execErr.Unwrap() != rootErr {
		t.Error("Execution error should unwrap to root error")
	}

	// Test error messages contain relevant information
	platformMsg := platformErr.Error()
	if !contains(platformMsg, "command execution failed") {
		t.Error("Platform error should contain its message")
	}

	execMsg := execErr.Error()
	if !contains(execMsg, "failed-command") {
		t.Error("Execution error should contain command name")
	}
}

// Test error creation with edge cases
func TestErrorEdgeCases(t *testing.T) {
	// Test with empty strings
	emptyPlatformErr := NewPlatformError(ErrCodeUnsupportedPlatform, "", "", "")
	if emptyPlatformErr.Error() == "" {
		t.Error("Even empty platform error should have some message")
	}

	// Test with very long strings
	longMessage := make([]byte, 1000)
	for i := range longMessage {
		longMessage[i] = 'a'
	}
	longErr := NewPlatformError(ErrCodePackageManagerFailed, string(longMessage), "problem", "suggestion")
	if len(longErr.Error()) == 0 {
		t.Error("Long message error should not be empty")
	}

	// Test execution error with empty command
	emptyExecErr := NewExecutionError("", 0, "", nil)
	if emptyExecErr.Error() == "" {
		t.Error("Even empty execution error should have some message")
	}

	// Test package manager error with empty values
	emptyPkgErr := NewPackageManagerError("", "", "", "", nil)
	if emptyPkgErr.Error() == "" {
		t.Error("Even empty package manager error should have some message")
	}
}
