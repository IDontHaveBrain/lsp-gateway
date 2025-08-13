package errors

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"lsp-gateway/src/internal/types"
)

func TestLSPError(t *testing.T) {
	err := NewLSPError(-32601, "Method not found", map[string]string{"method": "test"})

	if err.Code != -32601 {
		t.Errorf("Expected code -32601, got %d", err.Code)
	}

	if err.Message != "Method not found" {
		t.Errorf("Expected message 'Method not found', got %s", err.Message)
	}

	expectedError := "LSP error -32601: Method not found (data: map[method:test])"
	if err.Error() != expectedError {
		t.Errorf("Expected error string %s, got %s", expectedError, err.Error())
	}
}

func TestValidationError(t *testing.T) {
	err := NewValidationError("uri", "URI cannot be empty")

	if err.Parameter != "uri" {
		t.Errorf("Expected parameter 'uri', got %s", err.Parameter)
	}

	expectedError := "validation error for parameter 'uri': URI cannot be empty"
	if err.Error() != expectedError {
		t.Errorf("Expected error string %s, got %s", expectedError, err.Error())
	}
}

func TestConnectionError(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := NewConnectionError("go", cause)

	if err.Language != "go" {
		t.Errorf("Expected language 'go', got %s", err.Language)
	}

	if err.Cause != cause {
		t.Errorf("Expected cause to be preserved")
	}

	if !strings.Contains(err.Error(), "connection error for go") {
		t.Errorf("Expected error string to contain language, got %s", err.Error())
	}
}

func TestTimeoutError(t *testing.T) {
	timeout := 30 * time.Second
	err := NewTimeoutError("initialization", "python", timeout, nil)

	if err.Operation != "initialization" {
		t.Errorf("Expected operation 'initialization', got %s", err.Operation)
	}

	if err.Language != "python" {
		t.Errorf("Expected language 'python', got %s", err.Language)
	}

	if err.Timeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, err.Timeout)
	}
}

func TestMethodNotSupportedErrorUnified(t *testing.T) {
	err := NewMethodNotSupportedError("pylsp", types.MethodWorkspaceSymbol, "Consider using jedi-language-server")

	// Test that the error is correctly identified as a method not supported error
	if !IsMethodNotSupportedError(err) {
		t.Error("Expected IsMethodNotSupportedError to return true")
	}

	expectedError := "LSP server 'pylsp' does not support 'workspace/symbol'. Consider using jedi-language-server"
	if err.Error() != expectedError {
		t.Errorf("Expected error string %s, got %s", expectedError, err.Error())
	}

	// Test type assertion to access fields
	if methodErr, ok := err.(*MethodNotSupportedError); ok {
		if methodErr.Server != "pylsp" {
			t.Errorf("Expected server 'pylsp', got %s", methodErr.Server)
		}

		if methodErr.Method != types.MethodWorkspaceSymbol {
			t.Errorf("Expected method 'workspace/symbol', got %s", methodErr.Method)
		}

		if methodErr.Suggestion != "Consider using jedi-language-server" {
			t.Errorf("Expected suggestion, got %s", methodErr.Suggestion)
		}
	} else {
		t.Error("Expected error to be of type *MethodNotSupportedError")
	}
}

func TestProcessError(t *testing.T) {
	cause := fmt.Errorf("executable not found")
	err := NewProcessError("rust", "rust-analyzer", "start", cause)

	if err.Language != "rust" {
		t.Errorf("Expected language 'rust', got %s", err.Language)
	}

	if err.Command != "rust-analyzer" {
		t.Errorf("Expected command 'rust-analyzer', got %s", err.Command)
	}

	if err.Type != "start" {
		t.Errorf("Expected type 'start', got %s", err.Type)
	}

	if err.Cause != cause {
		t.Errorf("Expected cause to be preserved")
	}
}

func TestErrorClassification(t *testing.T) {
	// Test connection error detection
	connErr := NewConnectionError("go", fmt.Errorf("connection refused"))
	if !IsConnectionError(connErr) {
		t.Error("Expected IsConnectionError to return true for ConnectionError")
	}

	// Test validation error detection
	valErr := NewValidationError("position", "invalid line number")
	if !IsValidationError(valErr) {
		t.Error("Expected IsValidationError to return true for ValidationError")
	}

	// Test timeout error detection
	timeoutErr := NewTimeoutError("test", "go", time.Second, context.DeadlineExceeded)
	if !IsTimeoutError(timeoutErr) {
		t.Error("Expected IsTimeoutError to return true for TimeoutError")
	}

	// Test context.DeadlineExceeded detection
	if !IsTimeoutError(context.DeadlineExceeded) {
		t.Error("Expected IsTimeoutError to return true for context.DeadlineExceeded")
	}

	// Test cancellation error detection
	if !IsCancellationError(context.Canceled) {
		t.Error("Expected IsCancellationError to return true for context.Canceled")
	}

	// Test method not supported detection
	methodErr := NewMethodNotSupportedError("server", "method", "suggestion")
	if !IsMethodNotSupportedError(methodErr) {
		t.Error("Expected IsMethodNotSupportedError to return true for MethodNotSupportedError")
	}

	// Test process error detection
	procErr := NewProcessError("go", "gopls", "start", fmt.Errorf("not found"))
	if !IsProcessError(procErr) {
		t.Error("Expected IsProcessError to return true for ProcessError")
	}
}

func TestErrorWrapping(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	wrappedErr := WrapWithContext("test operation", originalErr)

	if !strings.Contains(wrappedErr.Error(), "test operation") {
		t.Error("Expected wrapped error to contain context")
	}

	if !strings.Contains(wrappedErr.Error(), "original error") {
		t.Error("Expected wrapped error to contain original message")
	}
}

func TestErrorWrappingSpecific(t *testing.T) {
	originalErr := fmt.Errorf("parameter is missing")
	valErr := WrapValidationError("uri", originalErr)

	if !IsValidationError(valErr) {
		t.Error("Expected wrapped error to be ValidationError")
	}

	connErr := WrapConnectionError("python", fmt.Errorf("connection failed"))
	if !IsConnectionError(connErr) {
		t.Error("Expected wrapped error to be ConnectionError")
	}
}

func TestNilErrorHandling(t *testing.T) {
	// Test that nil errors are handled properly
	if IsConnectionError(nil) {
		t.Error("Expected IsConnectionError to return false for nil")
	}

	if IsValidationError(nil) {
		t.Error("Expected IsValidationError to return false for nil")
	}

	if IsTimeoutError(nil) {
		t.Error("Expected IsTimeoutError to return false for nil")
	}

	if IsCancellationError(nil) {
		t.Error("Expected IsCancellationError to return false for nil")
	}

	if WrapWithContext("test", nil) != nil {
		t.Error("Expected WrapWithContext to return nil for nil error")
	}
}
