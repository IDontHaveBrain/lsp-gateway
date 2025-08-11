package base

import (
	"fmt"
	"strings"
	"sync"
	"time"

	internalErrors "lsp-gateway/src/internal/errors"
)

// ErrorType represents different categories of errors that can occur
type ErrorType string

const (
	ErrorTypeTimeout     ErrorType = "timeout"
	ErrorTypeConnection  ErrorType = "connection"
	ErrorTypeProtocol    ErrorType = "protocol"
	ErrorTypeUnsupported ErrorType = "unsupported"
	ErrorTypeGeneral     ErrorType = "general"
)

// LanguageError represents an error with language-specific context
type LanguageError struct {
	Language  string
	Error     error
	ErrorType ErrorType
	Timestamp time.Time
}

// ErrorCollector provides thread-safe collection and reporting of language-specific errors
type ErrorCollector struct {
	mu     sync.RWMutex
	errors []LanguageError
}

// NewErrorCollector creates a new ErrorCollector instance
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		errors: make([]LanguageError, 0),
	}
}

// Add adds an error with language context and automatic type detection
func (ec *ErrorCollector) Add(language string, err error) {
	if err == nil {
		return
	}

	errorType := ec.detectErrorType(err)
	ec.AddTyped(language, err, errorType)
}

// AddTyped adds an error with explicit type classification
func (ec *ErrorCollector) AddTyped(language string, err error, errorType ErrorType) {
	if err == nil {
		return
	}

	ec.mu.Lock()
	defer ec.mu.Unlock()

	languageError := LanguageError{
		Language:  language,
		Error:     err,
		ErrorType: errorType,
		Timestamp: time.Now(),
	}

	ec.errors = append(ec.errors, languageError)
}

// GetErrors returns formatted error messages for logging/reporting
func (ec *ErrorCollector) GetErrors() []string {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	if len(ec.errors) == 0 {
		return nil
	}

	errorMessages := make([]string, len(ec.errors))
	for i, langErr := range ec.errors {
		errorMessages[i] = fmt.Sprintf("%s: %v", langErr.Language, langErr.Error)
	}

	return errorMessages
}

// GetErrorsByType returns errors filtered by type
func (ec *ErrorCollector) GetErrorsByType(errorType ErrorType) []LanguageError {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	var filtered []LanguageError
	for _, langErr := range ec.errors {
		if langErr.ErrorType == errorType {
			filtered = append(filtered, langErr)
		}
	}

	return filtered
}

// HasErrors returns true if any errors have been collected
func (ec *ErrorCollector) HasErrors() bool {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	return len(ec.errors) > 0
}

// GetSuccessCount returns count of successful operations (for partial success)
func (ec *ErrorCollector) GetSuccessCount(totalOperations int) int {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	return totalOperations - len(ec.errors)
}

// GetErrorCount returns the total number of errors collected
func (ec *ErrorCollector) GetErrorCount() int {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	return len(ec.errors)
}

// GetLanguagesWithErrors returns a list of languages that have errors
func (ec *ErrorCollector) GetLanguagesWithErrors() []string {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	languageSet := make(map[string]bool)
	for _, langErr := range ec.errors {
		languageSet[langErr.Language] = true
	}

	languages := make([]string, 0, len(languageSet))
	for lang := range languageSet {
		languages = append(languages, lang)
	}

	return languages
}

// GetErrorSummary returns a formatted summary of all errors by type
func (ec *ErrorCollector) GetErrorSummary() string {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	if len(ec.errors) == 0 {
		return "No errors"
	}

	errorsByType := make(map[ErrorType][]string)
	for _, langErr := range ec.errors {
		errorsByType[langErr.ErrorType] = append(
			errorsByType[langErr.ErrorType],
			fmt.Sprintf("%s: %v", langErr.Language, langErr.Error),
		)
	}

	var summaryParts []string
	for errorType, errorList := range errorsByType {
		summaryParts = append(summaryParts, fmt.Sprintf("%s (%d): %s",
			errorType, len(errorList), strings.Join(errorList, "; ")))
	}

	return strings.Join(summaryParts, " | ")
}

// Clear removes all collected errors
func (ec *ErrorCollector) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.errors = ec.errors[:0]
}

// detectErrorType automatically determines error type based on error content
// Uses centralized error classification from internal/errors package
func (ec *ErrorCollector) detectErrorType(err error) ErrorType {
	if err == nil {
		return ErrorTypeGeneral
	}

	// Use centralized error classification functions
	if internalErrors.IsTimeoutError(err) {
		return ErrorTypeTimeout
	}

	if internalErrors.IsConnectionError(err) || internalErrors.IsProcessError(err) {
		return ErrorTypeConnection
	}

	// Check for protocol errors (JSON-RPC, parsing, etc.)
	if internalErrors.IsProtocolError(err) {
		return ErrorTypeProtocol
	}

	if internalErrors.IsMethodNotSupportedError(err) {
		return ErrorTypeUnsupported
	}

	return ErrorTypeGeneral
}
