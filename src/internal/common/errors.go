package common

import (
	"lsp-gateway/src/internal/errors"
)

// Compatibility wrappers for existing error functions
// These functions now use the unified error system internally

// ValidateParamMap validates that params is not nil and converts it to a map[string]interface{}
func ValidateParamMap(params interface{}) (map[string]interface{}, error) {
	return errors.ValidateParamMap(params)
}

// WrapProcessingError wraps an error with operation context for better error messages
func WrapProcessingError(operation string, err error) error {
	return errors.WrapWithContext(operation, err)
}

// ParameterValidationError creates a formatted parameter validation error
func ParameterValidationError(msg string) error {
	return errors.NewValidationError("parameter", msg)
}

// NoParametersError returns a standardized "no parameters provided" error
func NoParametersError() error {
	return errors.NewValidationError("params", "no parameters provided")
}

// Additional helper functions for common error scenarios

// CreateConnectionError creates a connection error for a specific language
func CreateConnectionError(language string, cause error) error {
	return errors.NewConnectionError(language, cause)
}

// CreateTimeoutError creates a timeout error for an operation
func CreateTimeoutError(operation, language string, cause error) error {
	return errors.NewTimeoutError(operation, language, 0, cause)
}

// CreateValidationErrorForURI creates a validation error for URI-related issues
func CreateValidationErrorForURI(msg string) error {
	return errors.NewValidationError("uri", msg)
}

// CreateValidationErrorForPosition creates a validation error for position-related issues
func CreateValidationErrorForPosition(msg string) error {
	return errors.NewValidationError("position", msg)
}

// IsRetryableError checks if an error represents a condition that can be retried
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Connection and timeout errors are generally retryable
	return errors.IsConnectionError(err) || errors.IsTimeoutError(err)
}

// GetErrorCategory returns a category string for error classification
func GetErrorCategory(err error) string {
	if err == nil {
		return "none"
	}

	switch {
	case errors.IsConnectionError(err):
		return "connection"
	case errors.IsTimeoutError(err):
		return "timeout"
	case errors.IsValidationError(err):
		return "validation"
	case errors.IsMethodNotSupportedError(err):
		return "unsupported"
	case errors.IsCancellationError(err):
		return "cancellation"
	default:
		return "general"
	}
}
