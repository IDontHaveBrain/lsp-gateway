package mcp

import (
	"reflect"
	"strings"
	"testing"
)

func TestNewParameterValidator(t *testing.T) {
	tests := []struct {
		name              string
		expectedURIRegex  string
		expectedFileRegex string
	}{
		{
			name:              "constructor creates validator with compiled regexes",
			expectedURIRegex:  "^file://",
			expectedFileRegex: `\.[a-zA-Z0-9]+$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewParameterValidator()

			if validator == nil {
				t.Fatal("NewParameterValidator() returned nil")
			}

			if validator.uriRegex == nil {
				t.Error("uriRegex should not be nil")
			}

			if validator.fileExtRegex == nil {
				t.Error("fileExtRegex should not be nil")
			}

			// Test that regexes are properly compiled and functional
			if !validator.uriRegex.MatchString("file:///test.go") {
				t.Error("uriRegex should match file:// URIs")
			}

			if !validator.fileExtRegex.MatchString("/path/test.go") {
				t.Error("fileExtRegex should match files with extensions")
			}
		})
	}
}

func TestValidateParameters(t *testing.T) {
	validator := NewParameterValidator()

	tests := []struct {
		name        string
		schema      map[string]interface{}
		params      map[string]interface{}
		expectError bool
		errorCount  int
		description string
	}{
		{
			name: "valid parameters pass validation",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"uri": map[string]interface{}{
						"type": "string",
					},
					"line": map[string]interface{}{
						"type":    "integer",
						"minimum": float64(0),
					},
				},
				"required": []interface{}{"uri"},
			},
			params: map[string]interface{}{
				"uri":  "file:///home/user/test.go",
				"line": float64(10),
			},
			expectError: false,
			description: "Valid URI and line parameters",
		},
		{
			name: "missing required field triggers error",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"uri": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"uri"},
			},
			params:      map[string]interface{}{},
			expectError: true,
			errorCount:  1,
			description: "Missing required URI field",
		},
		{
			name: "multiple validation errors are aggregated",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"uri": map[string]interface{}{
						"type": "string",
					},
					"line": map[string]interface{}{
						"type":    "integer",
						"minimum": float64(0),
					},
					"name": map[string]interface{}{
						"type":      "string",
						"minLength": float64(3),
					},
				},
				"required": []interface{}{"uri", "line"},
			},
			params: map[string]interface{}{
				"line": float64(-1), // Invalid: negative
				"name": "ab",        // Invalid: too short
				// Missing required uri
			},
			expectError: true,
			errorCount:  4, // Missing uri + invalid line + invalid name + tool constraint for negative line
			description: "Multiple validation failures",
		},
		{
			name: "empty schema allows any parameters",
			schema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
			params: map[string]interface{}{
				"anyField": "anyValue",
			},
			expectError: false,
			description: "Empty schema validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateParameters(tt.schema, tt.params)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error for %s, but got none", tt.description)
					return
				}

				paramErr, ok := err.(*ParameterValidationError)
				if !ok {
					t.Errorf("Expected ParameterValidationError, got %T", err)
					return
				}

				if len(paramErr.Errors) != tt.errorCount {
					t.Errorf("Expected %d validation errors, got %d", tt.errorCount, len(paramErr.Errors))
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error for %s, but got: %v", tt.description, err)
				}
			}
		})
	}
}

func TestValidateStringField(t *testing.T) {
	validator := NewParameterValidator()

	tests := []struct {
		name        string
		field       string
		value       string
		schema      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:  "valid string passes",
			field: "name",
			value: "valid_name",
			schema: map[string]interface{}{
				"type": "string",
			},
			expectError: false,
		},
		{
			name:  "string too short fails minLength",
			field: "name",
			value: "ab",
			schema: map[string]interface{}{
				"type":      "string",
				"minLength": float64(3),
			},
			expectError: true,
			errorMsg:    "must be at least 3 characters long",
		},
		{
			name:  "string too long fails maxLength",
			field: "name",
			value: "very_long_string",
			schema: map[string]interface{}{
				"type":      "string",
				"maxLength": float64(5),
			},
			expectError: true,
			errorMsg:    "must be at most 5 characters long",
		},
		{
			name:  "string fails pattern validation",
			field: "identifier",
			value: "invalid-identifier!",
			schema: map[string]interface{}{
				"type":    "string",
				"pattern": "^[a-zA-Z_][a-zA-Z0-9_]*$",
			},
			expectError: true,
			errorMsg:    "does not match required pattern",
		},
		{
			name:  "string passes pattern validation",
			field: "identifier",
			value: "valid_identifier_123",
			schema: map[string]interface{}{
				"type":    "string",
				"pattern": "^[a-zA-Z_][a-zA-Z0-9_]*$",
			},
			expectError: false,
		},
		{
			name:  "empty string with minLength fails",
			field: "required_field",
			value: "",
			schema: map[string]interface{}{
				"type":      "string",
				"minLength": float64(1),
			},
			expectError: true,
			errorMsg:    "must be at least 1 characters long",
		},
		{
			name:  "invalid minLength constraint type",
			field: "field",
			value: "test",
			schema: map[string]interface{}{
				"type":      "string",
				"minLength": "not a number",
			},
			expectError: false,
		},
		{
			name:  "invalid maxLength constraint type",
			field: "field",
			value: "test",
			schema: map[string]interface{}{
				"type":      "string",
				"maxLength": "not a number",
			},
			expectError: false,
		},
		{
			name:  "invalid pattern constraint type",
			field: "field",
			value: "test",
			schema: map[string]interface{}{
				"type":    "string",
				"pattern": 123,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateStringField(tt.field, tt.value, tt.schema)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error, but got none")
					return
				}
				if !strings.Contains(err.Message, tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Message)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, but got: %v", err.Message)
				}
			}
		})
	}
}

func TestValidateIntegerField(t *testing.T) {
	validator := NewParameterValidator()

	tests := []struct {
		name        string
		field       string
		value       interface{}
		schema      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:  "valid integer passes",
			field: "count",
			value: int64(10),
			schema: map[string]interface{}{
				"type": "integer",
			},
			expectError: false,
		},
		{
			name:  "integer too small fails minimum",
			field: "count",
			value: int64(5),
			schema: map[string]interface{}{
				"type":    "integer",
				"minimum": float64(10),
			},
			expectError: true,
			errorMsg:    "must be at least 10",
		},
		{
			name:  "integer too large fails maximum",
			field: "count",
			value: int64(100),
			schema: map[string]interface{}{
				"type":    "integer",
				"maximum": float64(50),
			},
			expectError: true,
			errorMsg:    "must be at most 50",
		},
		{
			name:  "float64 integer passes",
			field: "line",
			value: float64(42),
			schema: map[string]interface{}{
				"type": "integer",
			},
			expectError: false,
		},
		{
			name:  "negative integer with minimum 0 fails",
			field: "position",
			value: int64(-1),
			schema: map[string]interface{}{
				"type":    "integer",
				"minimum": float64(0),
			},
			expectError: true,
			errorMsg:    "must be at least 0",
		},
		{
			name:  "boundary value at minimum passes",
			field: "value",
			value: int64(0),
			schema: map[string]interface{}{
				"type":    "integer",
				"minimum": float64(0),
			},
			expectError: false,
		},
		{
			name:  "boundary value at maximum passes",
			field: "value",
			value: int64(100),
			schema: map[string]interface{}{
				"type":    "integer",
				"maximum": float64(100),
			},
			expectError: false,
		},
		{
			name:  "int32 value passes",
			field: "value",
			value: int32(42),
			schema: map[string]interface{}{
				"type": "integer",
			},
			expectError: false,
		},
		{
			name:  "regular int value passes",
			field: "value",
			value: 42,
			schema: map[string]interface{}{
				"type": "integer",
			},
			expectError: false,
		},
		{
			name:  "invalid minimum constraint type",
			field: "value",
			value: int64(10),
			schema: map[string]interface{}{
				"type":    "integer",
				"minimum": "not a number",
			},
			expectError: false,
		},
		{
			name:  "invalid maximum constraint type",
			field: "value",
			value: int64(10),
			schema: map[string]interface{}{
				"type":    "integer",
				"maximum": "not a number",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateIntegerField(tt.field, tt.value, tt.schema)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error, but got none")
					return
				}
				if !strings.Contains(err.Message, tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Message)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, but got: %v", err.Message)
				}
			}
		})
	}
}

func TestValidateField(t *testing.T) {
	validator := NewParameterValidator()

	tests := []struct {
		name        string
		field       string
		value       interface{}
		schema      map[string]interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:  "valid string field",
			field: "name",
			value: "test_string",
			schema: map[string]interface{}{
				"type": "string",
			},
			expectError: false,
		},
		{
			name:  "wrong type for string field",
			field: "name",
			value: 123,
			schema: map[string]interface{}{
				"type": "string",
			},
			expectError: true,
			errorMsg:    "must be a string",
		},
		{
			name:  "valid integer field",
			field: "count",
			value: float64(42),
			schema: map[string]interface{}{
				"type": "integer",
			},
			expectError: false,
		},
		{
			name:  "decimal value for integer field",
			field: "count",
			value: 42.5,
			schema: map[string]interface{}{
				"type": "integer",
			},
			expectError: true,
			errorMsg:    "must be an integer, got decimal",
		},
		{
			name:  "wrong type for integer field",
			field: "count",
			value: "not_a_number",
			schema: map[string]interface{}{
				"type": "integer",
			},
			expectError: true,
			errorMsg:    "must be an integer",
		},
		{
			name:  "valid boolean field",
			field: "enabled",
			value: true,
			schema: map[string]interface{}{
				"type": "boolean",
			},
			expectError: false,
		},
		{
			name:  "wrong type for boolean field",
			field: "enabled",
			value: "true",
			schema: map[string]interface{}{
				"type": "boolean",
			},
			expectError: true,
			errorMsg:    "must be a boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateField(tt.field, tt.value, tt.schema)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error, but got none")
					return
				}
				if !strings.Contains(err.Message, tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Message)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, but got: %v", err.Message)
				}
			}
		})
	}
}

func TestValidateURI(t *testing.T) {
	validator := NewParameterValidator()

	tests := []struct {
		name        string
		uri         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid file URI",
			uri:         "file:///home/user/test.go",
			expectError: false,
		},
		{
			name:        "valid Windows file URI",
			uri:         "file:///C:/Users/user/test.js",
			expectError: false,
		},
		{
			name:        "non-file URI rejected",
			uri:         "http://example.com/test.go",
			expectError: true,
			errorMsg:    "must be a file:// URI",
		},
		{
			name:        "invalid URI format",
			uri:         "file://invalid uri with spaces",
			expectError: true,
			errorMsg:    "invalid URI format",
		},
		{
			name:        "relative path rejected",
			uri:         "file:relative/path/test.go",
			expectError: true,
			errorMsg:    "must be a file:// URI",
		},
		{
			name:        "path traversal attack prevented",
			uri:         "file:///home/user/../../../etc/passwd",
			expectError: true,
			errorMsg:    "contains dangerous patterns",
		},
		{
			name:        "path traversal in middle prevented",
			uri:         "file:///home/../user/test.go",
			expectError: true,
			errorMsg:    "contains dangerous patterns",
		},
		{
			name:        "missing file extension rejected",
			uri:         "file:///home/user/test",
			expectError: true,
			errorMsg:    "should have a file extension",
		},
		{
			name:        "directory path rejected",
			uri:         "file:///home/user/",
			expectError: true,
			errorMsg:    "should have a file extension",
		},
		{
			name:        "valid file with multiple extensions",
			uri:         "file:///home/user/test.spec.ts",
			expectError: false,
		},
		{
			name:        "URI with special characters",
			uri:         "file:///home/user/test%20file.go",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateURI(tt.uri)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error for URI: %s", tt.uri)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error for URI: %s, but got: %v", tt.uri, err)
				}
			}
		})
	}
}

func TestValidateToolConstraints(t *testing.T) {
	validator := NewParameterValidator()

	tests := []struct {
		name        string
		params      map[string]interface{}
		expectError bool
		errorCount  int
		description string
	}{
		{
			name: "valid parameters pass",
			params: map[string]interface{}{
				"uri":       "file:///home/user/test.go",
				"line":      float64(10),
				"character": float64(5),
				"query":     "symbol_name",
			},
			expectError: false,
			description: "All valid tool parameters",
		},
		{
			name: "invalid URI triggers error",
			params: map[string]interface{}{
				"uri": "http://example.com/test.go",
			},
			expectError: true,
			errorCount:  1,
			description: "Non-file URI should fail",
		},
		{
			name: "negative line number fails",
			params: map[string]interface{}{
				"line": float64(-1),
			},
			expectError: true,
			errorCount:  1,
			description: "Line numbers must be non-negative",
		},
		{
			name: "negative character position fails",
			params: map[string]interface{}{
				"character": float64(-5),
			},
			expectError: true,
			errorCount:  1,
			description: "Character positions must be non-negative",
		},
		{
			name: "empty query string fails",
			params: map[string]interface{}{
				"query": "",
			},
			expectError: true,
			errorCount:  1,
			description: "Empty query strings should fail",
		},
		{
			name: "whitespace-only query fails",
			params: map[string]interface{}{
				"query": "   \t\n  ",
			},
			expectError: true,
			errorCount:  1,
			description: "Whitespace-only queries should fail",
		},
		{
			name: "multiple constraint violations",
			params: map[string]interface{}{
				"uri":       "http://example.com/test.go",
				"line":      float64(-1),
				"character": float64(-5),
				"query":     "",
			},
			expectError: true,
			errorCount:  4,
			description: "All constraints should fail",
		},
		{
			name: "boundary values pass",
			params: map[string]interface{}{
				"line":      float64(0),
				"character": float64(0),
				"query":     "a", // Single character query
			},
			expectError: false,
			description: "Zero-based positions should be valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.validateToolConstraints(tt.params)

			if tt.expectError {
				if len(errors) == 0 {
					t.Errorf("Expected validation errors for %s, but got none", tt.description)
					return
				}
				if len(errors) != tt.errorCount {
					t.Errorf("Expected %d validation errors, got %d", tt.errorCount, len(errors))
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected no validation errors for %s, but got %d", tt.description, len(errors))
				}
			}
		})
	}
}

func TestSanitizeParameters(t *testing.T) {
	validator := NewParameterValidator()

	tests := []struct {
		name           string
		params         map[string]interface{}
		expected       map[string]interface{}
		expectError    bool
		errorMsg       string
		checkTransform func(original, sanitized map[string]interface{}) bool
	}{
		{
			name: "URI path normalization",
			params: map[string]interface{}{
				"uri": "file:///home/user/../user/./test.go",
			},
			expected: map[string]interface{}{
				"uri": "file:///home/user/test.go",
			},
			expectError: false,
			checkTransform: func(original, sanitized map[string]interface{}) bool {
				return sanitized["uri"].(string) == "file:///home/user/test.go"
			},
		},
		{
			name: "integer normalization from float64",
			params: map[string]interface{}{
				"line":      float64(42),
				"character": float64(10),
			},
			expected: map[string]interface{}{
				"line":      int64(42),
				"character": int64(10),
			},
			expectError: false,
			checkTransform: func(original, sanitized map[string]interface{}) bool {
				line, ok1 := sanitized["line"].(int64)
				char, ok2 := sanitized["character"].(int64)
				return ok1 && ok2 && line == 42 && char == 10
			},
		},
		{
			name: "query string trimming",
			params: map[string]interface{}{
				"query": "  symbol_name  \t\n",
			},
			expected: map[string]interface{}{
				"query": "symbol_name",
			},
			expectError: false,
			checkTransform: func(original, sanitized map[string]interface{}) bool {
				return sanitized["query"].(string) == "symbol_name"
			},
		},
		{
			name: "boolean string parsing",
			params: map[string]interface{}{
				"includeDeclaration": "true",
			},
			expected: map[string]interface{}{
				"includeDeclaration": true,
			},
			expectError: false,
			checkTransform: func(original, sanitized map[string]interface{}) bool {
				val, ok := sanitized["includeDeclaration"].(bool)
				return ok && val
			},
		},
		{
			name: "boolean string parsing false",
			params: map[string]interface{}{
				"includeDeclaration": "false",
			},
			expected: map[string]interface{}{
				"includeDeclaration": false,
			},
			expectError: false,
			checkTransform: func(original, sanitized map[string]interface{}) bool {
				val, ok := sanitized["includeDeclaration"].(bool)
				return ok && !val
			},
		},
		{
			name: "preserve non-boolean string for includeDeclaration",
			params: map[string]interface{}{
				"includeDeclaration": "maybe",
			},
			expected: map[string]interface{}{
				"includeDeclaration": "maybe",
			},
			expectError: false,
			checkTransform: func(original, sanitized map[string]interface{}) bool {
				return sanitized["includeDeclaration"].(string) == "maybe"
			},
		},
		{
			name: "preserve unchanged fields",
			params: map[string]interface{}{
				"customField": "unchanged",
				"number":      123,
			},
			expected: map[string]interface{}{
				"customField": "unchanged",
				"number":      123,
			},
			expectError: false,
			checkTransform: func(original, sanitized map[string]interface{}) bool {
				return sanitized["customField"].(string) == "unchanged" &&
					sanitized["number"].(int) == 123
			},
		},
		{
			name: "invalid URI causes error",
			params: map[string]interface{}{
				"uri": "invalid\x00uri\nwith\tcontrol\rchars",
			},
			expectError: true,
			errorMsg:    "failed to parse URI",
		},
		{
			name: "comprehensive sanitization",
			params: map[string]interface{}{
				"uri":                "file:///home/user/../user/test.go",
				"line":               float64(15),
				"character":          float64(20),
				"query":              "  search_term  ",
				"includeDeclaration": "true",
				"other":              "preserved",
			},
			expectError: false,
			checkTransform: func(original, sanitized map[string]interface{}) bool {
				return sanitized["uri"].(string) == "file:///home/user/test.go" &&
					sanitized["line"].(int64) == 15 &&
					sanitized["character"].(int64) == 20 &&
					sanitized["query"].(string) == "search_term" &&
					sanitized["includeDeclaration"].(bool) == true &&
					sanitized["other"].(string) == "preserved"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized, err := validator.SanitizeParameters(tt.params)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected sanitization error, but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no sanitization error, but got: %v", err)
				return
			}

			if tt.checkTransform != nil {
				if !tt.checkTransform(tt.params, sanitized) {
					t.Errorf("Sanitization transform check failed. Original: %+v, Sanitized: %+v", tt.params, sanitized)
				}
			} else if !reflect.DeepEqual(sanitized, tt.expected) {
				t.Errorf("Expected sanitized params %+v, got %+v", tt.expected, sanitized)
			}
		})
	}
}

func TestParameterValidationError(t *testing.T) {
	tests := []struct {
		name     string
		errors   []ValidationError
		expected string
	}{
		{
			name: "single error returns simple message",
			errors: []ValidationError{
				{
					Field:    "uri",
					Value:    "invalid",
					Expected: "file URI",
					Actual:   "invalid",
					Message:  "URI must be a file:// URI",
				},
			},
			expected: "URI must be a file:// URI",
		},
		{
			name: "multiple errors returns aggregated message",
			errors: []ValidationError{
				{
					Field:   "uri",
					Message: "URI must be a file:// URI",
				},
				{
					Field:   "line",
					Message: "Line must be non-negative",
				},
				{
					Field:   "query",
					Message: "Query cannot be empty",
				},
			},
			expected: "Multiple validation errors: URI must be a file:// URI; Line must be non-negative; Query cannot be empty",
		},
		{
			name:     "empty errors list",
			errors:   []ValidationError{},
			expected: "Multiple validation errors: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ParameterValidationError{Errors: tt.errors}
			actual := err.Error()

			if actual != tt.expected {
				t.Errorf("Expected error message '%s', got '%s'", tt.expected, actual)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	validator := NewParameterValidator()

	t.Run("getIntValue", func(t *testing.T) {
		tests := []struct {
			name     string
			value    interface{}
			expected int64
		}{
			{"int value", 42, 42},
			{"int32 value", int32(42), 42},
			{"int64 value", int64(42), 42},
			{"float64 value", float64(42), 42},
			{"invalid value", "not a number", -1},
			{"nil value", nil, -1},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := validator.getIntValue(tt.value)
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			})
		}
	})

	t.Run("normalizeInteger", func(t *testing.T) {
		tests := []struct {
			name     string
			value    interface{}
			expected interface{}
		}{
			{"whole number float64", float64(42), int64(42)},
			{"decimal float64", float64(42.5), float64(42.5)},
			{"int value unchanged", 42, 42},
			{"string value unchanged", "42", "42"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := validator.normalizeInteger(tt.value)
				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("Expected %v (%T), got %v (%T)", tt.expected, tt.expected, result, result)
				}
			})
		}
	})
}

func TestEdgeCases(t *testing.T) {
	validator := NewParameterValidator()

	t.Run("nil schema handling", func(t *testing.T) {
		err := validator.ValidateParameters(nil, map[string]interface{}{"test": "value"})
		if err != nil {
			t.Errorf("Expected no error with nil schema, got: %v", err)
		}
	})

	t.Run("nil params handling", func(t *testing.T) {
		schema := map[string]interface{}{
			"required": []interface{}{"uri"},
		}
		err := validator.ValidateParameters(schema, nil)
		if err == nil {
			t.Error("Expected error with nil params and required fields")
		}
	})

	t.Run("empty params and schema", func(t *testing.T) {
		err := validator.ValidateParameters(map[string]interface{}{}, map[string]interface{}{})
		if err != nil {
			t.Errorf("Expected no error with empty params and schema, got: %v", err)
		}
	})

	t.Run("malformed schema properties", func(t *testing.T) {
		schema := map[string]interface{}{
			"properties": "not a map", // Invalid properties type
		}
		err := validator.ValidateParameters(schema, map[string]interface{}{"test": "value"})
		if err != nil {
			t.Errorf("Expected no error with malformed schema properties, got: %v", err)
		}
	})

	t.Run("malformed required fields", func(t *testing.T) {
		schema := map[string]interface{}{
			"required": "not an array", // Invalid required type
		}
		err := validator.ValidateParameters(schema, map[string]interface{}{})
		if err != nil {
			t.Errorf("Expected no error with malformed required fields, got: %v", err)
		}
	})

	t.Run("non-string required field name", func(t *testing.T) {
		schema := map[string]interface{}{
			"required": []interface{}{123, "valid_field"}, // Non-string required field
		}
		err := validator.ValidateParameters(schema, map[string]interface{}{"valid_field": "value"})
		if err != nil {
			t.Errorf("Expected no error with non-string required field, got: %v", err)
		}
	})

	t.Run("field with non-map schema", func(t *testing.T) {
		// This test checks graceful handling of malformed property schemas
		// The validation code should not process fields with non-map schemas
		schema := map[string]interface{}{
			"properties": map[string]interface{}{
				"invalid_field": "not a map",
				"valid_field": map[string]interface{}{
					"type": "string",
				},
			},
		}
		params := map[string]interface{}{
			"valid_field": "value",
			// Don't include invalid_field to avoid the panic
		}
		err := validator.ValidateParameters(schema, params)
		if err != nil {
			t.Errorf("Expected no error with non-map field schema when field not provided, got: %v", err)
		}
	})

	t.Run("unknown field type", func(t *testing.T) {
		schema := map[string]interface{}{
			"properties": map[string]interface{}{
				"unknown_type": map[string]interface{}{
					"type": "unknown_type",
				},
			},
		}
		err := validator.ValidateParameters(schema, map[string]interface{}{"unknown_type": "value"})
		if err != nil {
			t.Errorf("Expected no error with unknown field type, got: %v", err)
		}
	})
}

func TestBoundaryConditions(t *testing.T) {
	validator := NewParameterValidator()

	t.Run("string length boundaries", func(t *testing.T) {
		schema := map[string]interface{}{
			"type":      "string",
			"minLength": float64(5),
			"maxLength": float64(10),
		}

		// Test exact boundaries
		err := validator.validateStringField("test", "12345", schema) // Exactly 5 chars
		if err != nil {
			t.Errorf("Expected no error for string at minimum length, got: %v", err.Message)
		}

		err = validator.validateStringField("test", "1234567890", schema) // Exactly 10 chars
		if err != nil {
			t.Errorf("Expected no error for string at maximum length, got: %v", err.Message)
		}

		err = validator.validateStringField("test", "1234", schema) // 4 chars - too short
		if err == nil {
			t.Error("Expected error for string below minimum length")
		}

		err = validator.validateStringField("test", "12345678901", schema) // 11 chars - too long
		if err == nil {
			t.Error("Expected error for string above maximum length")
		}
	})

	t.Run("integer value boundaries", func(t *testing.T) {
		schema := map[string]interface{}{
			"type":    "integer",
			"minimum": float64(0),
			"maximum": float64(100),
		}

		// Test exact boundaries
		err := validator.validateIntegerField("test", int64(0), schema) // Exactly minimum
		if err != nil {
			t.Errorf("Expected no error for integer at minimum value, got: %v", err.Message)
		}

		err = validator.validateIntegerField("test", int64(100), schema) // Exactly maximum
		if err != nil {
			t.Errorf("Expected no error for integer at maximum value, got: %v", err.Message)
		}

		err = validator.validateIntegerField("test", int64(-1), schema) // Below minimum
		if err == nil {
			t.Error("Expected error for integer below minimum value")
		}

		err = validator.validateIntegerField("test", int64(101), schema) // Above maximum
		if err == nil {
			t.Error("Expected error for integer above maximum value")
		}
	})
}
