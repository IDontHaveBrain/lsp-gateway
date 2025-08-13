package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"lsp-gateway/src/internal/types"
)

func TestMethodNotSupportedError(t *testing.T) {
	tests := []struct {
		name       string
		server     string
		method     string
		suggestion string
		expected   string
	}{
		{
			name:       "Basic error",
			server:     "gopls",
			method:     "textDocument/formatting",
			suggestion: "Try using a different LSP server.",
			expected:   "LSP server 'gopls' does not support 'textDocument/formatting'. Try using a different LSP server.",
		},
		{
			name:       "Empty suggestion",
			server:     "pyright",
			method:     types.MethodWorkspaceSymbol,
			suggestion: "",
			expected:   "LSP server 'pyright' does not support 'workspace/symbol'. ",
		},
		{
			name:       "Long method name",
			server:     "typescript-language-server",
			method:     "textDocument/semanticTokens/full/delta",
			suggestion: "This feature requires a newer version.",
			expected:   "LSP server 'typescript-language-server' does not support 'textDocument/semanticTokens/full/delta'. This feature requires a newer version.",
		},
		{
			name:       "Special characters in server name",
			server:     "java-lsp/server",
			method:     types.MethodTextDocumentHover,
			suggestion: "Check server configuration.",
			expected:   "LSP server 'java-lsp/server' does not support 'textDocument/hover'. Check server configuration.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewMethodNotSupportedError(tt.server, tt.method, tt.suggestion)
			assert.Error(t, err)
			assert.Equal(t, tt.expected, err.Error())

			// Type assertion check
			methodErr, ok := err.(*MethodNotSupportedError)
			assert.True(t, ok)
			assert.Equal(t, tt.server, methodErr.Server)
			assert.Equal(t, tt.method, methodErr.Method)
			assert.Equal(t, tt.suggestion, methodErr.Suggestion)
		})
	}
}

func TestMethodNotSupportedError_Fields(t *testing.T) {
	server := "test-server"
	method := "test/method"
	suggestion := "Test suggestion"

	err := NewMethodNotSupportedError(server, method, suggestion)

	// Type assertion
	methodErr, ok := err.(*MethodNotSupportedError)
	assert.True(t, ok, "Should be able to cast to MethodNotSupportedError")

	// Field verification
	assert.Equal(t, server, methodErr.Server)
	assert.Equal(t, method, methodErr.Method)
	assert.Equal(t, suggestion, methodErr.Suggestion)
}

func TestMethodNotSupportedError_Interface(t *testing.T) {
	// Verify that MethodNotSupportedError implements the error interface
	var err error
	err = &MethodNotSupportedError{
		Server:     "test",
		Method:     "test/method",
		Suggestion: "test suggestion",
	}

	assert.NotNil(t, err)
	assert.Equal(t, "LSP server 'test' does not support 'test/method'. test suggestion", err.Error())
}

func TestMethodNotSupportedError_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		server     string
		method     string
		suggestion string
		expected   string
	}{
		{
			name:       "Empty server name",
			server:     "",
			method:     types.MethodTextDocumentHover,
			suggestion: "Server not specified",
			expected:   "LSP server '' does not support 'textDocument/hover'. Server not specified",
		},
		{
			name:       "Empty method name",
			server:     "gopls",
			method:     "",
			suggestion: "Method not specified",
			expected:   "LSP server 'gopls' does not support ''. Method not specified",
		},
		{
			name:       "All empty fields",
			server:     "",
			method:     "",
			suggestion: "",
			expected:   "LSP server '' does not support ''. ",
		},
		{
			name:       "Very long suggestion",
			server:     "server",
			method:     "method",
			suggestion: "This is a very long suggestion that contains detailed information about why the method is not supported and what alternatives might be available for the user to consider when encountering this error.",
			expected:   "LSP server 'server' does not support 'method'. This is a very long suggestion that contains detailed information about why the method is not supported and what alternatives might be available for the user to consider when encountering this error.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewMethodNotSupportedError(tt.server, tt.method, tt.suggestion)
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestMethodNotSupportedError_Comparison(t *testing.T) {
	err1 := NewMethodNotSupportedError("server1", "method1", "suggestion1")
	err2 := NewMethodNotSupportedError("server1", "method1", "suggestion1")
	err3 := NewMethodNotSupportedError("server2", "method1", "suggestion1")

	// Same values should produce same error message
	assert.Equal(t, err1.Error(), err2.Error())

	// Different values should produce different error message
	assert.NotEqual(t, err1.Error(), err3.Error())
}
