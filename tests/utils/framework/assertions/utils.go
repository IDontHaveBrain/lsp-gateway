package assertions

import (
	"lsp-gateway/tests/utils/framework/cases"
	lspconfig "lsp-gateway/tests/utils/framework/config"
)

// Utility functions and convenience methods for assertion helpers

// Quick assertion builders for common scenarios

// QuickDefinitionTest creates a simple definition test expectation
func QuickDefinitionTest(hasResult bool, uri string, line, char int) *ComprehensiveExpectedResult {
	builder := NewExpectedResult().WithDefinition(hasResult)

	if hasResult && uri != "" {
		builder = builder.WithDefinitionLocation(uri, line, char, line, char+10)
	}

	return builder.Build()
}

// QuickReferencesTest creates a simple references test expectation
func QuickReferencesTest(hasResult bool, minCount, maxCount int) *ComprehensiveExpectedResult {
	builder := NewExpectedResult().WithReferences(hasResult)

	if hasResult {
		builder = builder.WithReferencesCount(minCount, maxCount)
	}

	return builder.Build()
}

// QuickHoverTest creates a simple hover test expectation
func QuickHoverTest(hasResult bool, contentPatterns ...string) *ComprehensiveExpectedResult {
	builder := NewExpectedResult().WithHover(hasResult)

	if hasResult && len(contentPatterns) > 0 {
		builder = builder.WithHoverContent(true, contentPatterns...)
	}

	return builder.Build()
}

// QuickErrorTest creates a simple error test expectation
func QuickErrorTest(errorCode int, messagePatterns ...string) *ComprehensiveExpectedResult {
	builder := NewExpectedResult().
		Success(false).
		WithError(true).
		WithErrorCode(errorCode)

	if len(messagePatterns) > 0 {
		builder = builder.WithErrorMessageContains(messagePatterns...)
	}

	return builder.Build()
}

// Configuration utilities

// DefaultValidationConfig returns a default validation configuration
func DefaultValidationConfig() *lspconfig.ValidationConfig {
	return &lspconfig.ValidationConfig{
		StrictMode:        false,
		ValidateTypes:     true,
		ValidatePositions: true,
		ValidateURIs:      true,
	}
}

// Test case utilities

// CreateTestCase creates a test case with common defaults
func CreateTestCase(id, name, method string) *cases.TestCase {
	return &cases.TestCase{
		ID:                id,
		Name:              name,
		Method:            method,
		Status:            cases.TestStatusPending,
		ValidationResults: make([]*cases.ValidationResult, 0),
	}
}
