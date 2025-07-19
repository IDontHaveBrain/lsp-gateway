package mcp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestNewResponseFormatter(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "creates new formatter instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewResponseFormatter()

			if formatter == nil {
				t.Fatal("NewResponseFormatter() returned nil")
			}

			// Verify it's the correct type by checking it's not nil and has expected methods
			// Since ResponseFormatter has no exported fields, we just verify it's functional
		})
	}
}

func TestFormatDefinitionResponse(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name          string
		rawResponse   json.RawMessage
		requestArgs   map[string]interface{}
		expectMethod  string
		expectSuccess bool
		checkResults  func(results interface{}) bool
		checkSummary  func(summary string) bool
	}{
		{
			name: "single definition location",
			rawResponse: json.RawMessage(`{
				"uri": "file:///home/user/test.go",
				"range": {
					"start": {"line": 10, "character": 5},
					"end": {"line": 10, "character": 15}
				}
			}`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/main.go",
				"line":      5,
				"character": 10,
			},
			expectMethod:  "textDocument/definition",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				count, ok := resultsMap["definitionCount"].(int)
				if !ok || count != 1 {
					return false
				}
				_, hasDefinitions := resultsMap["definitions"]
				_, hasPrimary := resultsMap["primaryDefinition"]
				return hasDefinitions && hasPrimary
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "Definition found in test.go at line 11")
			},
		},
		{
			name: "multiple definition locations",
			rawResponse: json.RawMessage(`[
				{
					"uri": "file:///home/user/test1.go",
					"range": {"start": {"line": 5, "character": 0}, "end": {"line": 5, "character": 10}}
				},
				{
					"uri": "file:///home/user/test2.go", 
					"range": {"start": {"line": 15, "character": 5}, "end": {"line": 15, "character": 20}}
				}
			]`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/main.go",
				"line":      10,
				"character": 5,
			},
			expectMethod:  "textDocument/definition",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				count, ok := resultsMap["definitionCount"].(int)
				return ok && count == 2
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "Found 2 possible definitions")
			},
		},
		{
			name:        "no definitions found",
			rawResponse: json.RawMessage(`null`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/main.go",
				"line":      5,
				"character": 10,
			},
			expectMethod:  "textDocument/definition",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				count, ok := resultsMap["definitionCount"].(int)
				return ok && count == 0
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "No definition found")
			},
		},
		{
			name:        "invalid JSON response",
			rawResponse: json.RawMessage(`{invalid json`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/main.go",
				"line":      5,
				"character": 10,
			},
			expectMethod:  "textDocument/definition",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				count, ok := resultsMap["definitionCount"].(int)
				return ok && count == 0
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "No definition found")
			},
		},
		{
			name:        "empty array response",
			rawResponse: json.RawMessage(`[]`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/main.go",
				"line":      5,
				"character": 10,
			},
			expectMethod:  "textDocument/definition",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				count, ok := resultsMap["definitionCount"].(int)
				return ok && count == 0
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "No definition found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatDefinitionResponse(tt.rawResponse, tt.requestArgs)

			if result.Method != tt.expectMethod {
				t.Errorf("Expected method %s, got %s", tt.expectMethod, result.Method)
			}

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectSuccess, result.Success)
			}

			if tt.checkResults != nil && !tt.checkResults(result.Results) {
				t.Errorf("Results check failed for test %s. Results: %+v", tt.name, result.Results)
			}

			if tt.checkSummary != nil && !tt.checkSummary(result.Summary) {
				t.Errorf("Summary check failed for test %s. Summary: %s", tt.name, result.Summary)
			}

			if result.Context == nil {
				t.Error("Context should not be nil")
			}

			if result.RawResponse == nil {
				t.Error("RawResponse should not be nil")
			}
		})
	}
}

func TestFormatReferencesResponse(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name          string
		rawResponse   json.RawMessage
		requestArgs   map[string]interface{}
		expectMethod  string
		expectSuccess bool
		checkResults  func(results interface{}) bool
		checkSummary  func(summary string) bool
	}{
		{
			name: "multiple references in same file",
			rawResponse: json.RawMessage(`[
				{
					"uri": "file:///home/user/test.go",
					"range": {"start": {"line": 5, "character": 0}, "end": {"line": 5, "character": 10}}
				},
				{
					"uri": "file:///home/user/test.go",
					"range": {"start": {"line": 15, "character": 5}, "end": {"line": 15, "character": 15}}
				}
			]`),
			requestArgs: map[string]interface{}{
				"uri":                "file:///home/user/main.go",
				"line":               10,
				"character":          5,
				"includeDeclaration": true,
			},
			expectMethod:  "textDocument/references",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				refCount, ok := resultsMap["referenceCount"].(int)
				if !ok || refCount != 2 {
					return false
				}
				fileCount, ok := resultsMap["fileCount"].(int)
				if !ok || fileCount != 1 {
					return false
				}
				_, hasRefs := resultsMap["references"]
				_, hasRefsByFile := resultsMap["referencesByFile"]
				return hasRefs && hasRefsByFile
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "Found 2 references in 1 file")
			},
		},
		{
			name: "references across multiple files",
			rawResponse: json.RawMessage(`[
				{
					"uri": "file:///home/user/test1.go",
					"range": {"start": {"line": 5, "character": 0}, "end": {"line": 5, "character": 10}}
				},
				{
					"uri": "file:///home/user/test2.go",
					"range": {"start": {"line": 10, "character": 5}, "end": {"line": 10, "character": 15}}
				},
				{
					"uri": "file:///home/user/test3.go",
					"range": {"start": {"line": 15, "character": 0}, "end": {"line": 15, "character": 10}}
				}
			]`),
			requestArgs: map[string]interface{}{
				"uri":                "file:///home/user/main.go",
				"line":               5,
				"character":          10,
				"includeDeclaration": false,
			},
			expectMethod:  "textDocument/references",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				refCount, ok := resultsMap["referenceCount"].(int)
				if !ok || refCount != 3 {
					return false
				}
				fileCount, ok := resultsMap["fileCount"].(int)
				return ok && fileCount == 3
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "Found 3 references across 3 files")
			},
		},
		{
			name:        "no references found",
			rawResponse: json.RawMessage(`[]`),
			requestArgs: map[string]interface{}{
				"uri":                "file:///home/user/main.go",
				"line":               5,
				"character":          10,
				"includeDeclaration": false,
			},
			expectMethod:  "textDocument/references",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				refCount, ok := resultsMap["referenceCount"].(int)
				return ok && refCount == 0
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "No references found")
			},
		},
		{
			name:        "invalid JSON response",
			rawResponse: json.RawMessage(`{invalid json`),
			requestArgs: map[string]interface{}{
				"uri":                "file:///home/user/main.go",
				"line":               5,
				"character":          10,
				"includeDeclaration": false,
			},
			expectMethod:  "textDocument/references",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				refCount, ok := resultsMap["referenceCount"].(int)
				return ok && refCount == 0
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "No references found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatReferencesResponse(tt.rawResponse, tt.requestArgs)

			if result.Method != tt.expectMethod {
				t.Errorf("Expected method %s, got %s", tt.expectMethod, result.Method)
			}

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectSuccess, result.Success)
			}

			if tt.checkResults != nil && !tt.checkResults(result.Results) {
				t.Errorf("Results check failed for test %s. Results: %+v", tt.name, result.Results)
			}

			if tt.checkSummary != nil && !tt.checkSummary(result.Summary) {
				t.Errorf("Summary check failed for test %s. Summary: %s", tt.name, result.Summary)
			}
		})
	}
}

func TestFormatHoverResponse(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name          string
		rawResponse   json.RawMessage
		requestArgs   map[string]interface{}
		expectMethod  string
		expectSuccess bool
		checkResults  func(results interface{}) bool
		checkSummary  func(summary string) bool
	}{
		{
			name: "string hover content",
			rawResponse: json.RawMessage(`{
				"contents": "This is a hover message",
				"range": {
					"start": {"line": 5, "character": 0},
					"end": {"line": 5, "character": 10}
				}
			}`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/test.go",
				"line":      5,
				"character": 10,
			},
			expectMethod:  "textDocument/hover",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				hasHover, ok := resultsMap["hasHoverInfo"].(bool)
				if !ok || !hasHover {
					return false
				}
				content, ok := resultsMap["content"].(string)
				if !ok || content != "This is a hover message" {
					return false
				}
				contentType, ok := resultsMap["contentType"].(string)
				if !ok || contentType != "text" {
					return false
				}
				_, hasRange := resultsMap["range"]
				return hasRange
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "Hover info: This is a hover message")
			},
		},
		{
			name: "markdown hover content",
			rawResponse: json.RawMessage(`{
				"contents": "` + "```go\\nfunc TestFunction() {}\\n```" + `",
				"range": {
					"start": {"line": 10, "character": 5},
					"end": {"line": 10, "character": 15}
				}
			}`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/test.go",
				"line":      10,
				"character": 5,
			},
			expectMethod:  "textDocument/hover",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				contentType, ok := resultsMap["contentType"].(string)
				return ok && contentType == "markdown"
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "Hover info: ```go")
			},
		},
		{
			name: "array hover content",
			rawResponse: json.RawMessage(`{
				"contents": [
					"First line",
					{"value": "Second line with value"},
					"Third line"
				]
			}`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/test.go",
				"line":      5,
				"character": 10,
			},
			expectMethod:  "textDocument/hover",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				content, ok := resultsMap["content"].(string)
				return ok && strings.Contains(content, "First line") &&
					strings.Contains(content, "Second line with value") &&
					strings.Contains(content, "Third line")
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "Hover info: First line")
			},
		},
		{
			name: "object hover content with value",
			rawResponse: json.RawMessage(`{
				"contents": {
					"value": "Object hover content"
				}
			}`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/test.go",
				"line":      5,
				"character": 10,
			},
			expectMethod:  "textDocument/hover",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				content, ok := resultsMap["content"].(string)
				return ok && content == "Object hover content"
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "Hover info: Object hover content")
			},
		},
		{
			name:        "no hover content",
			rawResponse: json.RawMessage(`{}`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/test.go",
				"line":      5,
				"character": 10,
			},
			expectMethod:  "textDocument/hover",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				hasHover, ok := resultsMap["hasHoverInfo"].(bool)
				return ok && !hasHover
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "No hover information available")
			},
		},
		{
			name: "long hover content truncation in summary",
			rawResponse: json.RawMessage(`{
				"contents": "This is a very long hover message that should be truncated in the summary because it exceeds the maximum length limit"
			}`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/test.go",
				"line":      5,
				"character": 10,
			},
			expectMethod:  "textDocument/hover",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				content, ok := resultsMap["content"].(string)
				return ok && len(content) > 100 // Full content preserved in results
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "...") // Should be truncated in summary
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatHoverResponse(tt.rawResponse, tt.requestArgs)

			if result.Method != tt.expectMethod {
				t.Errorf("Expected method %s, got %s", tt.expectMethod, result.Method)
			}

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectSuccess, result.Success)
			}

			if tt.checkResults != nil && !tt.checkResults(result.Results) {
				t.Errorf("Results check failed for test %s. Results: %+v", tt.name, result.Results)
			}

			if tt.checkSummary != nil && !tt.checkSummary(result.Summary) {
				t.Errorf("Summary check failed for test %s. Summary: %s", tt.name, result.Summary)
			}
		})
	}
}

func TestFormatDocumentSymbolsResponse(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name          string
		rawResponse   json.RawMessage
		requestArgs   map[string]interface{}
		expectMethod  string
		expectSuccess bool
		checkResults  func(results interface{}) bool
		checkSummary  func(summary string) bool
	}{
		{
			name: "hierarchical document symbols",
			rawResponse: json.RawMessage(`[
				{
					"name": "TestClass",
					"detail": "class detail",
					"kind": 5,
					"range": {"start": {"line": 0, "character": 0}, "end": {"line": 20, "character": 0}},
					"selectionRange": {"start": {"line": 0, "character": 6}, "end": {"line": 0, "character": 15}},
					"children": [
						{
							"name": "method1",
							"kind": 6,
							"range": {"start": {"line": 5, "character": 4}, "end": {"line": 10, "character": 4}},
							"selectionRange": {"start": {"line": 5, "character": 4}, "end": {"line": 5, "character": 11}},
							"children": []
						}
					]
				}
			]`),
			requestArgs: map[string]interface{}{
				"uri": "file:///home/user/test.go",
			},
			expectMethod:  "textDocument/documentSymbol",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				symbolCount, ok := resultsMap["symbolCount"].(int)
				if !ok || symbolCount != 1 {
					return false
				}
				hasHierarchy, ok := resultsMap["hasHierarchy"].(bool)
				if !ok || !hasHierarchy {
					return false
				}
				_, hasSymbols := resultsMap["symbols"]
				_, hasSymbolsByKind := resultsMap["symbolsByKind"]
				return hasSymbols && hasSymbolsByKind
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "Found 1 symbols in test.go")
			},
		},
		{
			name:        "invalid JSON triggers fallback parsing",
			rawResponse: json.RawMessage(`invalid json`),
			requestArgs: map[string]interface{}{
				"uri": "file:///home/user/test.go",
			},
			expectMethod:  "textDocument/documentSymbol",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				symbolCount, ok := resultsMap["symbolCount"].(int)
				if !ok || symbolCount != 0 {
					return false
				}
				hasHierarchy, ok := resultsMap["hasHierarchy"].(bool)
				return ok && !hasHierarchy
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "No symbols found in the document")
			},
		},
		{
			name:        "no symbols found",
			rawResponse: json.RawMessage(`[]`),
			requestArgs: map[string]interface{}{
				"uri": "file:///home/user/empty.go",
			},
			expectMethod:  "textDocument/documentSymbol",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				symbolCount, ok := resultsMap["symbolCount"].(int)
				return ok && symbolCount == 0
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "No symbols found in the document")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatDocumentSymbolsResponse(tt.rawResponse, tt.requestArgs)

			if result.Method != tt.expectMethod {
				t.Errorf("Expected method %s, got %s", tt.expectMethod, result.Method)
			}

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectSuccess, result.Success)
			}

			if tt.checkResults != nil && !tt.checkResults(result.Results) {
				t.Errorf("Results check failed for test %s. Results: %+v", tt.name, result.Results)
			}

			if tt.checkSummary != nil && !tt.checkSummary(result.Summary) {
				t.Errorf("Summary check failed for test %s. Summary: %s", tt.name, result.Summary)
			}
		})
	}
}

func TestFormatWorkspaceSymbolsResponse(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name          string
		rawResponse   json.RawMessage
		requestArgs   map[string]interface{}
		expectMethod  string
		expectSuccess bool
		checkResults  func(results interface{}) bool
		checkSummary  func(summary string) bool
	}{
		{
			name: "workspace symbols found",
			rawResponse: json.RawMessage(`[
				{
					"name": "TestFunction",
					"kind": 12,
					"location": {
						"uri": "file:///home/user/test1.go",
						"range": {"start": {"line": 5, "character": 0}, "end": {"line": 10, "character": 0}}
					},
					"containerName": "main"
				},
				{
					"name": "TestClass",
					"kind": 5,
					"location": {
						"uri": "file:///home/user/test2.go",
						"range": {"start": {"line": 0, "character": 0}, "end": {"line": 20, "character": 0}}
					}
				}
			]`),
			requestArgs: map[string]interface{}{
				"query": "Test",
			},
			expectMethod:  "workspace/symbol",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				symbolCount, ok := resultsMap["symbolCount"].(int)
				if !ok || symbolCount != 2 {
					return false
				}
				fileCount, ok := resultsMap["fileCount"].(int)
				if !ok || fileCount != 2 {
					return false
				}
				query, ok := resultsMap["query"].(string)
				if !ok || query != "Test" {
					return false
				}
				_, hasSymbols := resultsMap["symbols"]
				_, hasSymbolsByFile := resultsMap["symbolsByFile"]
				_, hasSymbolsByKind := resultsMap["symbolsByKind"]
				return hasSymbols && hasSymbolsByFile && hasSymbolsByKind
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "Found 2 symbols matching 'Test' across 2 files")
			},
		},
		{
			name:        "no workspace symbols found",
			rawResponse: json.RawMessage(`[]`),
			requestArgs: map[string]interface{}{
				"query": "NonexistentSymbol",
			},
			expectMethod:  "workspace/symbol",
			expectSuccess: true,
			checkResults: func(results interface{}) bool {
				resultsMap, ok := results.(map[string]interface{})
				if !ok {
					return false
				}
				symbolCount, ok := resultsMap["symbolCount"].(int)
				return ok && symbolCount == 0
			},
			checkSummary: func(summary string) bool {
				return strings.Contains(summary, "No symbols found matching query: NonexistentSymbol")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatWorkspaceSymbolsResponse(tt.rawResponse, tt.requestArgs)

			if result.Method != tt.expectMethod {
				t.Errorf("Expected method %s, got %s", tt.expectMethod, result.Method)
			}

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success %v, got %v", tt.expectSuccess, result.Success)
			}

			if tt.checkResults != nil && !tt.checkResults(result.Results) {
				t.Errorf("Results check failed for test %s. Results: %+v", tt.name, result.Results)
			}

			if tt.checkSummary != nil && !tt.checkSummary(result.Summary) {
				t.Errorf("Summary check failed for test %s. Summary: %s", tt.name, result.Summary)
			}
		})
	}
}

func TestExtractFileName(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{
			name:     "standard file URI",
			uri:      "file:///home/user/test.go",
			expected: "test.go",
		},
		{
			name:     "Windows file URI",
			uri:      "file:///C:/Users/user/test.js",
			expected: "test.js",
		},
		{
			name:     "URI with complex path",
			uri:      "file:///home/user/project/src/main/java/Test.java",
			expected: "Test.java",
		},
		{
			name:     "non-file URI",
			uri:      "/home/user/test.py",
			expected: "test.py",
		},
		{
			name:     "empty URI",
			uri:      "",
			expected: "unknown",
		},
		{
			name:     "URI with special characters",
			uri:      "file:///home/user/test%20file.go",
			expected: "test%20file.go",
		},
		{
			name:     "URI ending with slash",
			uri:      "file:///home/user/project/",
			expected: "project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.extractFileName(tt.uri)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFormatLocationDetails(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name     string
		location LSPLocation
		expected map[string]interface{}
	}{
		{
			name: "basic location",
			location: LSPLocation{
				URI: "file:///home/user/test.go",
				Range: LSPRange{
					Start: LSPPosition{Line: 5, Character: 10},
					End:   LSPPosition{Line: 5, Character: 20},
				},
			},
			expected: map[string]interface{}{
				"uri":            "file:///home/user/test.go",
				"fileName":       "test.go",
				"range":          LSPRange{Start: LSPPosition{Line: 5, Character: 10}, End: LSPPosition{Line: 5, Character: 20}},
				"line":           6, // 1-based
				"startCharacter": 10,
				"endCharacter":   20,
			},
		},
		{
			name: "zero-based position",
			location: LSPLocation{
				URI: "file:///home/user/main.js",
				Range: LSPRange{
					Start: LSPPosition{Line: 0, Character: 0},
					End:   LSPPosition{Line: 0, Character: 5},
				},
			},
			expected: map[string]interface{}{
				"uri":            "file:///home/user/main.js",
				"fileName":       "main.js",
				"range":          LSPRange{Start: LSPPosition{Line: 0, Character: 0}, End: LSPPosition{Line: 0, Character: 5}},
				"line":           1, // Should be 1-based for display
				"startCharacter": 0,
				"endCharacter":   5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatLocationDetails(tt.location)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestCountUniqueFiles(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name      string
		locations []LSPLocation
		expected  int
	}{
		{
			name: "single file",
			locations: []LSPLocation{
				{URI: "file:///home/user/test.go"},
				{URI: "file:///home/user/test.go"},
			},
			expected: 1,
		},
		{
			name: "multiple files",
			locations: []LSPLocation{
				{URI: "file:///home/user/test1.go"},
				{URI: "file:///home/user/test2.go"},
				{URI: "file:///home/user/test1.go"},
				{URI: "file:///home/user/test3.go"},
			},
			expected: 3,
		},
		{
			name:      "empty locations",
			locations: []LSPLocation{},
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.countUniqueFiles(tt.locations)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestExtractHoverText(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name     string
		contents interface{}
		expected string
	}{
		{
			name:     "string content",
			contents: "Simple hover text",
			expected: "Simple hover text",
		},
		{
			name: "array of strings",
			contents: []interface{}{
				"First line",
				"Second line",
				"Third line",
			},
			expected: "First line\nSecond line\nThird line",
		},
		{
			name: "array with objects containing value",
			contents: []interface{}{
				"String part",
				map[string]interface{}{"value": "Object value"},
				"Another string",
			},
			expected: "String part\nObject value\nAnother string",
		},
		{
			name: "object with value field",
			contents: map[string]interface{}{
				"value": "Object hover content",
			},
			expected: "Object hover content",
		},
		{
			name: "object without value field",
			contents: map[string]interface{}{
				"otherField": "Some content",
			},
			expected: "",
		},
		{
			name:     "nil content",
			contents: nil,
			expected: "",
		},
		{
			name:     "unsupported type",
			contents: 123,
			expected: "",
		},
		{
			name: "mixed array with non-string/object items",
			contents: []interface{}{
				"Valid string",
				123,
				map[string]interface{}{"value": "Valid object"},
				true,
			},
			expected: "Valid string\nValid object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.extractHoverText(tt.contents)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDetectContentType(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "markdown content",
			content:  "```go\nfunc main() {}\n```",
			expected: "markdown",
		},
		{
			name:     "code content with function",
			content:  "function testFunc() { return true; }",
			expected: "code",
		},
		{
			name:     "code content with class",
			content:  "class TestClass { constructor() {} }",
			expected: "code",
		},
		{
			name:     "code content with var",
			content:  "var testVariable = 42;",
			expected: "code",
		},
		{
			name:     "plain text",
			content:  "This is just plain text documentation",
			expected: "text",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "text",
		},
		{
			name:     "whitespace only",
			content:  "   \t\n   ",
			expected: "text",
		},
		{
			name:     "markdown with leading whitespace",
			content:  "   ```python\nprint('hello')\n```   ",
			expected: "markdown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.detectContentType(tt.content)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetSymbolKindName(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name     string
		kind     int
		expected string
	}{
		{name: "File", kind: 1, expected: "File"},
		{name: "Module", kind: 2, expected: "Module"},
		{name: "Namespace", kind: 3, expected: "Namespace"},
		{name: "Package", kind: 4, expected: "Package"},
		{name: "Class", kind: 5, expected: "Class"},
		{name: "Method", kind: 6, expected: "Method"},
		{name: "Property", kind: 7, expected: "Property"},
		{name: "Field", kind: 8, expected: "Field"},
		{name: "Constructor", kind: 9, expected: "Constructor"},
		{name: "Enum", kind: 10, expected: "Enum"},
		{name: "Interface", kind: 11, expected: "Interface"},
		{name: "Function", kind: 12, expected: "Function"},
		{name: "Variable", kind: 13, expected: "Variable"},
		{name: "Constant", kind: 14, expected: "Constant"},
		{name: "String", kind: 15, expected: "String"},
		{name: "Number", kind: 16, expected: "Number"},
		{name: "Boolean", kind: 17, expected: "Boolean"},
		{name: "Array", kind: 18, expected: "Array"},
		{name: "Object", kind: 19, expected: "Object"},
		{name: "Key", kind: 20, expected: "Key"},
		{name: "Null", kind: 21, expected: "Null"},
		{name: "EnumMember", kind: 22, expected: "EnumMember"},
		{name: "Struct", kind: 23, expected: "Struct"},
		{name: "Event", kind: 24, expected: "Event"},
		{name: "Operator", kind: 25, expected: "Operator"},
		{name: "TypeParameter", kind: 26, expected: "TypeParameter"},
		{name: "Unknown kind", kind: 999, expected: "Unknown(999)"},
		{name: "Zero kind", kind: 0, expected: "Unknown(0)"},
		{name: "Negative kind", kind: -1, expected: "Unknown(-1)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.getSymbolKindName(tt.kind)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGroupReferencesByFile(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name       string
		references []LSPLocation
		checkFunc  func(result map[string]interface{}) bool
	}{
		{
			name: "references in multiple files",
			references: []LSPLocation{
				{
					URI:   "file:///home/user/test1.go",
					Range: LSPRange{Start: LSPPosition{Line: 5, Character: 0}, End: LSPPosition{Line: 5, Character: 10}},
				},
				{
					URI:   "file:///home/user/test1.go",
					Range: LSPRange{Start: LSPPosition{Line: 10, Character: 5}, End: LSPPosition{Line: 10, Character: 15}},
				},
				{
					URI:   "file:///home/user/test2.go",
					Range: LSPRange{Start: LSPPosition{Line: 3, Character: 0}, End: LSPPosition{Line: 3, Character: 8}},
				},
			},
			checkFunc: func(result map[string]interface{}) bool {
				if len(result) != 2 {
					return false
				}
				test1, ok := result["test1.go"].(map[string]interface{})
				if !ok {
					return false
				}
				test1Refs, ok := test1["references"].([]map[string]interface{})
				if !ok || len(test1Refs) != 2 {
					return false
				}
				test1Count, ok := test1["referenceCount"].(int)
				if !ok || test1Count != 2 {
					return false
				}

				test2, ok := result["test2.go"].(map[string]interface{})
				if !ok {
					return false
				}
				test2Refs, ok := test2["references"].([]map[string]interface{})
				if !ok || len(test2Refs) != 1 {
					return false
				}
				test2Count, ok := test2["referenceCount"].(int)
				return ok && test2Count == 1
			},
		},
		{
			name:       "empty references",
			references: []LSPLocation{},
			checkFunc: func(result map[string]interface{}) bool {
				return len(result) == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.groupReferencesByFile(tt.references)
			if !tt.checkFunc(result) {
				t.Errorf("Check function failed for test %s. Result: %+v", tt.name, result)
			}
		})
	}
}

func TestUnicodeHandling(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name        string
		rawResponse json.RawMessage
		requestArgs map[string]interface{}
		checkFunc   func(result *FormattedLSPResponse) bool
	}{
		{
			name: "unicode in file names",
			rawResponse: json.RawMessage(`{
				"uri": "file:///home/user/тест.go",
				"range": {
					"start": {"line": 0, "character": 0},
					"end": {"line": 0, "character": 10}
				}
			}`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/main.go",
				"line":      0,
				"character": 0,
			},
			checkFunc: func(result *FormattedLSPResponse) bool {
				resultsMap, ok := result.Results.(map[string]interface{})
				if !ok {
					return false
				}
				definitions, ok := resultsMap["definitions"].([]map[string]interface{})
				if !ok || len(definitions) != 1 {
					return false
				}
				fileName, ok := definitions[0]["fileName"].(string)
				return ok && fileName == "тест.go"
			},
		},
		{
			name: "unicode in hover content",
			rawResponse: json.RawMessage(`{
				"contents": "Функция для тестирования 🚀"
			}`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///home/user/test.go",
				"line":      5,
				"character": 10,
			},
			checkFunc: func(result *FormattedLSPResponse) bool {
				resultsMap, ok := result.Results.(map[string]interface{})
				if !ok {
					return false
				}
				content, ok := resultsMap["content"].(string)
				return ok && strings.Contains(content, "Функция для тестирования 🚀")
			},
		},
		{
			name: "unicode in symbol names",
			rawResponse: json.RawMessage(`[
				{
					"name": "测试函数",
					"kind": 12,
					"location": {
						"uri": "file:///home/user/test.go",
						"range": {"start": {"line": 5, "character": 0}, "end": {"line": 10, "character": 0}}
					}
				}
			]`),
			requestArgs: map[string]interface{}{
				"query": "测试",
			},
			checkFunc: func(result *FormattedLSPResponse) bool {
				resultsMap, ok := result.Results.(map[string]interface{})
				if !ok {
					return false
				}
				symbols, ok := resultsMap["symbols"].([]map[string]interface{})
				if !ok || len(symbols) != 1 {
					return false
				}
				name, ok := symbols[0]["name"].(string)
				return ok && name == "测试函数"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result *FormattedLSPResponse
			if strings.Contains(tt.name, "hover") {
				result = formatter.FormatHoverResponse(tt.rawResponse, tt.requestArgs)
			} else if strings.Contains(tt.name, "symbol") {
				result = formatter.FormatWorkspaceSymbolsResponse(tt.rawResponse, tt.requestArgs)
			} else {
				result = formatter.FormatDefinitionResponse(tt.rawResponse, tt.requestArgs)
			}

			if !tt.checkFunc(result) {
				t.Errorf("Unicode handling check failed for test %s. Result: %+v", tt.name, result.Results)
			}
		})
	}
}

func TestLargeResponseHandling(t *testing.T) {
	formatter := NewResponseFormatter()

	t.Run("large number of definitions", func(t *testing.T) {
		// Create a large array of locations
		locations := make([]map[string]interface{}, 1000)
		for i := 0; i < 1000; i++ {
			locations[i] = map[string]interface{}{
				"uri": "file:///home/user/test.go",
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": i, "character": 0},
					"end":   map[string]interface{}{"line": i, "character": 10},
				},
			}
		}

		rawResponse, err := json.Marshal(locations)
		if err != nil {
			t.Fatalf("Failed to marshal large response: %v", err)
		}

		requestArgs := map[string]interface{}{
			"uri":       "file:///home/user/main.go",
			"line":      5,
			"character": 10,
		}

		result := formatter.FormatDefinitionResponse(rawResponse, requestArgs)

		resultsMap, ok := result.Results.(map[string]interface{})
		if !ok {
			t.Fatal("Results should be a map")
		}

		count, ok := resultsMap["definitionCount"].(int)
		if !ok || count != 1000 {
			t.Errorf("Expected 1000 definitions, got %d", count)
		}

		definitions, ok := resultsMap["definitions"].([]map[string]interface{})
		if !ok || len(definitions) != 1000 {
			t.Errorf("Expected 1000 formatted definitions, got %d", len(definitions))
		}
	})

	t.Run("large hover content", func(t *testing.T) {
		// Create very long hover content
		longContent := strings.Repeat("This is a very long hover message with lots of details. ", 100)
		hoverResponse := map[string]interface{}{
			"contents": longContent,
		}

		rawResponse, err := json.Marshal(hoverResponse)
		if err != nil {
			t.Fatalf("Failed to marshal large hover response: %v", err)
		}

		requestArgs := map[string]interface{}{
			"uri":       "file:///home/user/test.go",
			"line":      5,
			"character": 10,
		}

		result := formatter.FormatHoverResponse(rawResponse, requestArgs)

		resultsMap, ok := result.Results.(map[string]interface{})
		if !ok {
			t.Fatal("Results should be a map")
		}

		content, ok := resultsMap["content"].(string)
		if !ok {
			t.Fatal("Content should be a string")
		}

		if len(content) != len(longContent) {
			t.Errorf("Expected full content length %d, got %d", len(longContent), len(content))
		}

		// Summary should be truncated
		if len(result.Summary) > 200 {
			t.Errorf("Summary should be truncated, but got length %d", len(result.Summary))
		}
	})

	t.Run("many workspace symbols", func(t *testing.T) {
		// Create many symbols across many files
		symbols := make([]map[string]interface{}, 500)
		for i := 0; i < 500; i++ {
			symbols[i] = map[string]interface{}{
				"name": "Symbol" + string(rune('A'+i%26)),
				"kind": 12,
				"location": map[string]interface{}{
					"uri": "file:///home/user/file" + string(rune('0'+i%10)) + ".go",
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": i % 100, "character": 0},
						"end":   map[string]interface{}{"line": i % 100, "character": 10},
					},
				},
			}
		}

		rawResponse, err := json.Marshal(symbols)
		if err != nil {
			t.Fatalf("Failed to marshal large workspace symbols response: %v", err)
		}

		requestArgs := map[string]interface{}{
			"query": "Symbol",
		}

		result := formatter.FormatWorkspaceSymbolsResponse(rawResponse, requestArgs)

		resultsMap, ok := result.Results.(map[string]interface{})
		if !ok {
			t.Fatal("Results should be a map")
		}

		symbolCount, ok := resultsMap["symbolCount"].(int)
		if !ok || symbolCount != 500 {
			t.Errorf("Expected 500 symbols, got %d", symbolCount)
		}

		fileCount, ok := resultsMap["fileCount"].(int)
		if !ok || fileCount != 10 {
			t.Errorf("Expected 10 unique files, got %d", fileCount)
		}
	})
}

func TestMalformedJSONHandling(t *testing.T) {
	formatter := NewResponseFormatter()

	tests := []struct {
		name        string
		rawResponse json.RawMessage
		requestArgs map[string]interface{}
		formatFunc  func(json.RawMessage, map[string]interface{}) *FormattedLSPResponse
		expectEmpty bool
	}{
		{
			name:        "invalid JSON for definitions",
			rawResponse: json.RawMessage(`{invalid json syntax`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///test.go",
				"line":      5,
				"character": 10,
			},
			formatFunc:  formatter.FormatDefinitionResponse,
			expectEmpty: true,
		},
		{
			name:        "null JSON for references",
			rawResponse: json.RawMessage(`null`),
			requestArgs: map[string]interface{}{
				"uri":                "file:///test.go",
				"line":               5,
				"character":          10,
				"includeDeclaration": true,
			},
			formatFunc:  formatter.FormatReferencesResponse,
			expectEmpty: true,
		},
		{
			name:        "truncated JSON for hover",
			rawResponse: json.RawMessage(`{"contents": "test", "range"`),
			requestArgs: map[string]interface{}{
				"uri":       "file:///test.go",
				"line":      5,
				"character": 10,
			},
			formatFunc:  formatter.FormatHoverResponse,
			expectEmpty: true,
		},
		{
			name:        "empty string JSON",
			rawResponse: json.RawMessage(``),
			requestArgs: map[string]interface{}{
				"uri": "file:///test.go",
			},
			formatFunc:  formatter.FormatDocumentSymbolsResponse,
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.formatFunc(tt.rawResponse, tt.requestArgs)

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			if !result.Success {
				t.Error("Success should be true even for malformed JSON")
			}

			resultsMap, ok := result.Results.(map[string]interface{})
			if !ok {
				t.Fatal("Results should be a map")
			}

			// Check that counts are 0 for malformed JSON
			if tt.expectEmpty {
				for key, value := range resultsMap {
					if strings.Contains(key, "Count") {
						if count, ok := value.(int); ok && count != 0 {
							t.Errorf("Expected count field %s to be 0, got %d", key, count)
						}
					}
				}
			}
		})
	}
}

func TestEdgeCasesAndBoundaryConditions(t *testing.T) {
	formatter := NewResponseFormatter()

	t.Run("zero-length ranges", func(t *testing.T) {
		rawResponse := json.RawMessage(`{
			"uri": "file:///test.go",
			"range": {
				"start": {"line": 5, "character": 10},
				"end": {"line": 5, "character": 10}
			}
		}`)

		result := formatter.FormatDefinitionResponse(rawResponse, map[string]interface{}{
			"uri": "file:///main.go", "line": 0, "character": 0,
		})

		resultsMap := result.Results.(map[string]interface{})
		primaryDef := resultsMap["primaryDefinition"].(map[string]interface{})

		if primaryDef["startCharacter"] != primaryDef["endCharacter"] {
			t.Error("Zero-length range should have equal start and end characters")
		}
	})

	t.Run("negative line numbers", func(t *testing.T) {
		rawResponse := json.RawMessage(`{
			"uri": "file:///test.go",
			"range": {
				"start": {"line": -1, "character": 0},
				"end": {"line": -1, "character": 5}
			}
		}`)

		result := formatter.FormatDefinitionResponse(rawResponse, map[string]interface{}{
			"uri": "file:///main.go", "line": 0, "character": 0,
		})

		resultsMap := result.Results.(map[string]interface{})
		primaryDef := resultsMap["primaryDefinition"].(map[string]interface{})
		line := primaryDef["line"].(int)

		if line != 0 { // -1 + 1 (1-based conversion)
			t.Errorf("Expected line 0 for negative line number, got %d", line)
		}
	})

	t.Run("very large line numbers", func(t *testing.T) {
		rawResponse := json.RawMessage(`{
			"uri": "file:///test.go",
			"range": {
				"start": {"line": 999999, "character": 0},
				"end": {"line": 999999, "character": 5}
			}
		}`)

		result := formatter.FormatDefinitionResponse(rawResponse, map[string]interface{}{
			"uri": "file:///main.go", "line": 0, "character": 0,
		})

		resultsMap := result.Results.(map[string]interface{})
		primaryDef := resultsMap["primaryDefinition"].(map[string]interface{})
		line := primaryDef["line"].(int)

		if line != 1000000 { // 999999 + 1 (1-based conversion)
			t.Errorf("Expected line 1000000 for large line number, got %d", line)
		}
	})

	t.Run("empty symbol names", func(t *testing.T) {
		rawResponse := json.RawMessage(`[
			{
				"name": "",
				"kind": 12,
				"location": {
					"uri": "file:///test.go",
					"range": {"start": {"line": 5, "character": 0}, "end": {"line": 5, "character": 0}}
				}
			}
		]`)

		result := formatter.FormatWorkspaceSymbolsResponse(rawResponse, map[string]interface{}{
			"query": "",
		})

		resultsMap := result.Results.(map[string]interface{})
		symbols := resultsMap["symbols"].([]map[string]interface{})

		if len(symbols) != 1 || symbols[0]["name"].(string) != "" {
			t.Error("Empty symbol names should be preserved")
		}
	})

	t.Run("deeply nested symbol hierarchy", func(t *testing.T) {
		// Create deeply nested document symbols
		deepSymbol := map[string]interface{}{
			"name":           "Level3",
			"kind":           6,
			"range":          map[string]interface{}{"start": map[string]interface{}{"line": 15, "character": 8}, "end": map[string]interface{}{"line": 17, "character": 8}},
			"selectionRange": map[string]interface{}{"start": map[string]interface{}{"line": 15, "character": 8}, "end": map[string]interface{}{"line": 15, "character": 14}},
			"children":       []interface{}{},
		}

		level2Symbol := map[string]interface{}{
			"name":           "Level2",
			"kind":           5,
			"range":          map[string]interface{}{"start": map[string]interface{}{"line": 10, "character": 4}, "end": map[string]interface{}{"line": 20, "character": 4}},
			"selectionRange": map[string]interface{}{"start": map[string]interface{}{"line": 10, "character": 4}, "end": map[string]interface{}{"line": 10, "character": 10}},
			"children":       []interface{}{deepSymbol},
		}

		topSymbol := map[string]interface{}{
			"name":           "Level1",
			"kind":           2,
			"range":          map[string]interface{}{"start": map[string]interface{}{"line": 0, "character": 0}, "end": map[string]interface{}{"line": 25, "character": 0}},
			"selectionRange": map[string]interface{}{"start": map[string]interface{}{"line": 0, "character": 0}, "end": map[string]interface{}{"line": 0, "character": 6}},
			"children":       []interface{}{level2Symbol},
		}

		rawResponse, _ := json.Marshal([]interface{}{topSymbol})

		result := formatter.FormatDocumentSymbolsResponse(rawResponse, map[string]interface{}{
			"uri": "file:///test.go",
		})

		resultsMap := result.Results.(map[string]interface{})
		symbols := resultsMap["symbols"].([]map[string]interface{})

		if len(symbols) != 1 {
			t.Errorf("Expected 1 top-level symbol, got %d", len(symbols))
		}

		// Check that nested structure is preserved
		topSymbolResult := symbols[0]
		if topSymbolResult["childCount"].(int) != 1 {
			t.Error("Top symbol should have 1 child")
		}

		children := topSymbolResult["children"].([]map[string]interface{})
		if len(children) != 1 || children[0]["name"].(string) != "Level2" {
			t.Error("Nested structure should be preserved")
		}
	})
}

func TestContextBuilding(t *testing.T) {
	formatter := NewResponseFormatter()

	t.Run("definition context", func(t *testing.T) {
		requestArgs := map[string]interface{}{
			"uri":       "file:///test.go",
			"line":      10,
			"character": 5,
		}
		locations := []LSPLocation{
			{URI: "file:///target.go"},
			{URI: "file:///other.go"},
		}

		context := formatter.buildDefinitionContext(requestArgs, locations)

		if context["hasDefinitions"] != true {
			t.Error("Context should indicate definitions were found")
		}

		requestedPos := context["requestedPosition"].(map[string]interface{})
		if requestedPos["uri"] != "file:///test.go" {
			t.Error("Context should preserve requested position")
		}

		defFiles := context["definitionFiles"].([]string)
		if len(defFiles) != 2 {
			t.Errorf("Expected 2 definition files, got %d", len(defFiles))
		}
	})

	t.Run("references context", func(t *testing.T) {
		requestArgs := map[string]interface{}{
			"uri":                "file:///test.go",
			"line":               5,
			"character":          15,
			"includeDeclaration": true,
		}
		references := []LSPLocation{
			{URI: "file:///ref1.go"},
		}

		context := formatter.buildReferencesContext(requestArgs, references)

		if context["includeDeclaration"] != true {
			t.Error("Context should preserve includeDeclaration flag")
		}

		if context["hasReferences"] != true {
			t.Error("Context should indicate references were found")
		}
	})

	t.Run("hover context", func(t *testing.T) {
		requestArgs := map[string]interface{}{
			"uri":       "file:///test.go",
			"line":      8,
			"character": 12,
		}
		hover := LSPHover{
			Contents: "Hover content",
			Range:    &LSPRange{Start: LSPPosition{Line: 8, Character: 10}, End: LSPPosition{Line: 8, Character: 20}},
		}

		context := formatter.buildHoverContext(requestArgs, hover)

		if context["hasRange"] != true {
			t.Error("Context should indicate range is present")
		}

		requestedPos := context["requestedPosition"].(map[string]interface{})
		if requestedPos["line"] != 8 {
			t.Error("Context should preserve requested line")
		}
	})

	t.Run("document symbols context", func(t *testing.T) {
		requestArgs := map[string]interface{}{
			"uri": "file:///home/user/example.go",
		}

		context := formatter.buildDocumentSymbolsContext(requestArgs, 5)

		if context["hasSymbols"] != true {
			t.Error("Context should indicate symbols were found")
		}

		if context["fileName"] != "example.go" {
			t.Error("Context should extract filename correctly")
		}

		if context["documentUri"] != "file:///home/user/example.go" {
			t.Error("Context should preserve document URI")
		}
	})

	t.Run("workspace symbols context", func(t *testing.T) {
		requestArgs := map[string]interface{}{
			"query": "TestQuery",
		}
		symbols := []LSPSymbolInformation{
			{Name: "Symbol1", Location: LSPLocation{URI: "file:///test1.go"}},
			{Name: "Symbol2", Location: LSPLocation{URI: "file:///test2.go"}},
		}

		context := formatter.buildWorkspaceSymbolsContext(requestArgs, symbols)

		if context["query"] != "TestQuery" {
			t.Error("Context should preserve query")
		}

		if context["hasResults"] != true {
			t.Error("Context should indicate results were found")
		}

		resultFiles := context["resultFiles"].([]string)
		if len(resultFiles) != 2 {
			t.Errorf("Expected 2 result files, got %d", len(resultFiles))
		}
	})
}
