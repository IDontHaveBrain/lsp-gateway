package errors

import (
	"fmt"
)

// MethodNotSupportedError represents a user-friendly error for unsupported LSP methods
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
func NewMethodNotSupportedError(server, method, suggestion string) error {
	return &MethodNotSupportedError{
		Server:     server,
		Method:     method,
		Suggestion: suggestion,
	}
}
