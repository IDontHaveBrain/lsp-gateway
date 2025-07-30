package workspace

import (
	"testing"
)

func TestURIExtractor_ExtractFileURI(t *testing.T) {
	t.Parallel()
	extractor := NewURIExtractor(nil)

	tests := []struct {
		name           string
		method         string
		params         interface{}
		expectedURI    string
		expectedError  bool
	}{
		{
			name:   "textDocument/definition with file:// URI",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///path/to/file.go",
				},
				"position": map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			},
			expectedURI:   "/path/to/file.go",
			expectedError: false,
		},
		{
			name:   "textDocument/references with plain path",
			method: "textDocument/references",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "/path/to/file.py",
				},
				"position": map[string]interface{}{
					"line":      20,
					"character": 15,
				},
			},
			expectedURI:   "/path/to/file.py",
			expectedError: false,
		},
		{
			name:   "textDocument/hover",
			method: "textDocument/hover",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///path/to/file.js",
				},
				"position": map[string]interface{}{
					"line":      5,
					"character": 10,
				},
			},
			expectedURI:   "/path/to/file.js",
			expectedError: false,
		},
		{
			name:   "textDocument/documentSymbol",
			method: "textDocument/documentSymbol",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///path/to/file.java",
				},
			},
			expectedURI:   "/path/to/file.java",
			expectedError: false,
		},
		{
			name:   "textDocument/completion",
			method: "textDocument/completion",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "/path/to/file.ts",
				},
				"position": map[string]interface{}{
					"line":      15,
					"character": 8,
				},
			},
			expectedURI:   "/path/to/file.ts",
			expectedError: false,
		},
		{
			name:   "workspace/symbol returns empty URI",
			method: "workspace/symbol",
			params: map[string]interface{}{
				"query": "MyFunction",
			},
			expectedURI:   "",
			expectedError: false,
		},
		{
			name:          "unsupported method",
			method:        "textDocument/formatting",
			params:        map[string]interface{}{},
			expectedURI:   "",
			expectedError: true,
		},
		{
			name:          "invalid parameters type",
			method:        "textDocument/definition",
			params:        "invalid",
			expectedURI:   "",
			expectedError: true,
		},
		{
			name:   "missing textDocument",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"position": map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			},
			expectedURI:   "",
			expectedError: true,
		},
		{
			name:   "missing uri",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"version": 1,
				},
				"position": map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			},
			expectedURI:   "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uri, err := extractor.ExtractFileURI(tt.method, tt.params)
			
			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if uri != tt.expectedURI {
				t.Errorf("expected URI %q, got %q", tt.expectedURI, uri)
			}
		})
	}
}

func TestURIExtractor_normalizeURI(t *testing.T) {
	t.Parallel()
	extractor := NewURIExtractor(nil)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "file:// URI",
			input:    "file:///path/to/file.go",
			expected: "/path/to/file.go",
		},
		{
			name:     "plain path",
			input:    "/path/to/file.go",
			expected: "/path/to/file.go",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := extractor.NormalizeURI(tt.input)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}