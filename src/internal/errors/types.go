// Package errors provides unified error types and codes.
package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"
	"time"
)

const ConnectionTypeUnknown = "unknown"

// LSPError represents a standard LSP error with code and optional data
type LSPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *LSPError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("LSP error %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("LSP error %d: %s", e.Code, e.Message)
}

// ValidationError represents parameter validation errors
type ValidationError struct {
	Parameter string `json:"parameter"`
	Message   string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for parameter '%s': %s", e.Parameter, e.Message)
}

// ConnectionError represents network and LSP server connection errors
type ConnectionError struct {
	Language string `json:"language"`
	Cause    error  `json:"cause,omitempty"`
	Type     string `json:"type,omitempty"`
}

func (e *ConnectionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("connection error for %s (%s): %v", e.Language, e.Type, e.Cause)
	}
	return fmt.Sprintf("connection error for %s (%s)", e.Language, e.Type)
}

func (e *ConnectionError) Unwrap() error {
	return e.Cause
}

// TimeoutError represents operation timeout errors
type TimeoutError struct {
	Operation string        `json:"operation"`
	Language  string        `json:"language,omitempty"`
	Timeout   time.Duration `json:"timeout,omitempty"`
	Cause     error         `json:"cause,omitempty"`
}

func (e *TimeoutError) Error() string {
	if e.Language != "" {
		return fmt.Sprintf("timeout error for %s operation on %s (timeout: %v)", e.Operation, e.Language, e.Timeout)
	}
	return fmt.Sprintf("timeout error for %s operation (timeout: %v)", e.Operation, e.Timeout)
}

func (e *TimeoutError) Unwrap() error {
	return e.Cause
}

// MethodNotSupportedError represents a user-friendly error for unsupported LSP methods
type MethodNotSupportedError struct {
	Server     string
	Method     string
	Suggestion string
}

func (e *MethodNotSupportedError) Error() string {
	return fmt.Sprintf("LSP server '%s' does not support '%s'. %s", e.Server, e.Method, e.Suggestion)
}

// NewMethodNotSupportedError creates a new MethodNotSupportedError
func NewMethodNotSupportedError(server, method, suggestion string) error {
	return &MethodNotSupportedError{Server: server, Method: method, Suggestion: suggestion}
}

// ProcessError represents LSP server process errors
type ProcessError struct {
	Language string `json:"language"`
	Command  string `json:"command"`
	Cause    error  `json:"cause,omitempty"`
	Type     string `json:"type"` // "start", "stop", "communication"
}

func (e *ProcessError) Error() string {
	return fmt.Sprintf("process error for %s server (%s): %s - %v", e.Language, e.Type, e.Command, e.Cause)
}

func (e *ProcessError) Unwrap() error {
	return e.Cause
}

// Error constructors

// NewLSPError creates a new LSP error with specified code, message, and optional data
func NewLSPError(code int, message string, data interface{}) *LSPError {
	return &LSPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// NewValidationError creates a new validation error for the specified parameter
func NewValidationError(parameter, message string) *ValidationError {
	return &ValidationError{
		Parameter: parameter,
		Message:   message,
	}
}

// NewConnectionError creates a new connection error with language context
func NewConnectionError(language string, cause error) *ConnectionError {
	errType := ConnectionTypeUnknown
	if cause != nil {
		errType = classifyConnectionError(cause)
	}
	return &ConnectionError{
		Language: language,
		Cause:    cause,
		Type:     errType,
	}
}

// NewTimeoutError creates a new timeout error for the specified operation
func NewTimeoutError(operation, language string, timeout time.Duration, cause error) *TimeoutError {
	return &TimeoutError{
		Operation: operation,
		Language:  language,
		Timeout:   timeout,
		Cause:     cause,
	}
}

// NewProcessError creates a new process error for LSP server operations
func NewProcessError(language, command, errorType string, cause error) *ProcessError {
	return &ProcessError{
		Language: language,
		Command:  command,
		Type:     errorType,
		Cause:    cause,
	}
}

// Error classification functions

// ClassifyError classifies an error using type assertions and pattern matching
func ClassifyError(err error) ErrorClassification {
	if err == nil {
		return ClassUnknown
	}

	// Try type assertions first (most specific)
	var ce *ConnectionError
	if stderrors.As(err, &ce) {
		return ClassConnection
	}

	var ve *ValidationError
	if stderrors.As(err, &ve) {
		return ClassValidation
	}

	var te *TimeoutError
	if stderrors.As(err, &te) {
		return ClassTimeout
	}

	var me *MethodNotSupportedError
	if stderrors.As(err, &me) {
		return ClassMethodNotSupported
	}

	var pe *ProcessError
	if stderrors.As(err, &pe) {
		return ClassProcess
	}

	var le *LSPError
	if stderrors.As(err, &le) {
		return ClassProtocol
	}

	// Check for standard context errors
	if stderrors.Is(err, context.DeadlineExceeded) {
		return ClassTimeout
	}

	if stderrors.Is(err, context.Canceled) {
		return ClassCancellation
	}

	// Fallback to pattern matching
	return ClassifyByPattern(err)
}

// IsConnectionError checks if the error is a connection-related error
func IsConnectionError(err error) bool {
	return ClassifyError(err) == ClassConnection
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	return ClassifyError(err) == ClassValidation
}

// IsTimeoutError checks if the error is a timeout error
func IsTimeoutError(err error) bool {
	return ClassifyError(err) == ClassTimeout
}

// IsCancellationError checks if the error is a cancellation error
func IsCancellationError(err error) bool {
	return ClassifyError(err) == ClassCancellation
}

// IsMethodNotSupportedError checks if the error indicates an unsupported method
func IsMethodNotSupportedError(err error) bool {
	return ClassifyError(err) == ClassMethodNotSupported
}

// IsProcessError checks if the error is a process-related error
func IsProcessError(err error) bool {
	return ClassifyError(err) == ClassProcess
}

// IsProtocolError checks if the error is a protocol-related error (JSON-RPC, parsing, etc.)
func IsProtocolError(err error) bool {
	return ClassifyError(err) == ClassProtocol
}

// Error wrapping utilities

// WrapWithContext wraps an error with operation context
func WrapWithContext(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

// WrapValidationError wraps an error as a validation error
func WrapValidationError(parameter string, err error) error {
	if err == nil {
		return nil
	}
	return &ValidationError{
		Parameter: parameter,
		Message:   err.Error(),
	}
}

// WrapConnectionError wraps an error as a connection error for a specific language
func WrapConnectionError(language string, err error) error {
	if err == nil {
		return nil
	}
	return NewConnectionError(language, err)
}

// Helper functions for error classification

// classifyConnectionError determines the specific type of connection error
func classifyConnectionError(err error) string {
	if err == nil {
		return "unknown"
	}

	errMsg := strings.ToLower(err.Error())

	if strings.Contains(errMsg, "refused") || strings.Contains(errMsg, "connect") {
		return "refused"
	}
	if strings.Contains(errMsg, "timeout") {
		return "timeout"
	}
	if strings.Contains(errMsg, "unreachable") {
		return "unreachable"
	}
	if strings.Contains(errMsg, "broken pipe") {
		return "broken_pipe"
	}
	if strings.Contains(errMsg, "no such file") || strings.Contains(errMsg, "executable") {
		return "not_found"
	}

	return "network"
}
