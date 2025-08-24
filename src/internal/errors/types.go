// Package errors provides unified error types and codes.
package errors

import "fmt"

// This file provides backward compatibility wrappers for the legacy error types.
// New code should use the unified error system from unified_errors.go

// MethodNotSupportedError represents a user-friendly error for unsupported LSP methods
// Deprecated: Use the MethodNotSupportedError from unified_errors.go instead
type MethodNotSupportedError struct {
	Server     string
	Method     string
	Suggestion string
}

func (e *MethodNotSupportedError) Error() string {
	return fmt.Sprintf("LSP server '%s' does not support '%s'. %s",
		e.Server, e.Method, e.Suggestion)
}

// NewMethodNotSupportedError creates a new MethodNotSupportedError
// Deprecated: Use NewMethodNotSupportedError from unified_errors.go instead
func NewMethodNotSupportedError(server, method, suggestion string) error {
	return &MethodNotSupportedError{
		Server:     server,
		Method:     method,
		Suggestion: suggestion,
	}
}
