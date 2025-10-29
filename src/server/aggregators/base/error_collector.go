package base

import (
	"fmt"
	"strings"
	"sync"
	"time"

	internalErrors "lsp-gateway/src/internal/errors"
)

const (
	noErrors string = "No errors"
)

// LanguageError represents an error with language-specific context
type LanguageError struct {
	Language       string
	Error          error
	Classification internalErrors.ErrorClassification
	Timestamp      time.Time
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

// Add adds an error with language context and automatic classification
func (ec *ErrorCollector) Add(language string, err error) {
	if err == nil {
		return
	}

	classification := internalErrors.ClassifyError(err)
	ec.AddTyped(language, err, classification)
}

// AddTyped adds an error with explicit classification
func (ec *ErrorCollector) AddTyped(language string, err error, classification internalErrors.ErrorClassification) {
	if err == nil {
		return
	}

	ec.mu.Lock()
	defer ec.mu.Unlock()

	languageError := LanguageError{
		Language:       language,
		Error:          err,
		Classification: classification,
		Timestamp:      time.Now(),
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

// GetErrorsByType returns errors filtered by classification
func (ec *ErrorCollector) GetErrorsByType(classification internalErrors.ErrorClassification) []LanguageError {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	var filtered []LanguageError
	for _, langErr := range ec.errors {
		if langErr.Classification == classification {
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

// GetErrorSummary returns a formatted summary of all errors by classification
func (ec *ErrorCollector) GetErrorSummary() string {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	if len(ec.errors) == 0 {
		return noErrors
	}

	errorsByClass := make(map[internalErrors.ErrorClassification][]string)
	for _, langErr := range ec.errors {
		errorsByClass[langErr.Classification] = append(
			errorsByClass[langErr.Classification],
			fmt.Sprintf("%s: %v", langErr.Language, langErr.Error),
		)
	}

	var summaryParts []string
	for classification, errorList := range errorsByClass {
		summaryParts = append(summaryParts, fmt.Sprintf("%s (%d): %s",
			classification, len(errorList), strings.Join(errorList, "; ")))
	}

	return strings.Join(summaryParts, " | ")
}

// Clear removes all collected errors
func (ec *ErrorCollector) Clear() {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.errors = ec.errors[:0]
}
