package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	LSPMethodHover = "textDocument/hover"
)

type Tool struct {
	Name         string                  `json:"name"`
	Title        *string                 `json:"title,omitempty"`
	Description  string                  `json:"description"`
	InputSchema  map[string]interface{}  `json:"inputSchema"`
	OutputSchema *map[string]interface{} `json:"outputSchema,omitempty"`
}

type MCPErrorCode int

const (
	MCPErrorParseError     MCPErrorCode = -32700
	MCPErrorInvalidRequest MCPErrorCode = -32600
	MCPErrorMethodNotFound MCPErrorCode = -32601
	MCPErrorInvalidParams  MCPErrorCode = -32602
	MCPErrorInternalError  MCPErrorCode = -32603

	MCPErrorServerError        MCPErrorCode = -32000
	MCPErrorInvalidURI         MCPErrorCode = -32001
	MCPErrorUnsupportedFeature MCPErrorCode = -32002
	MCPErrorTimeout            MCPErrorCode = -32003
	MCPErrorConnectionFailed   MCPErrorCode = -32004
	MCPErrorValidationFailed   MCPErrorCode = -32005
	MCPErrorLSPServerError     MCPErrorCode = -32006
	MCPErrorResourceNotFound   MCPErrorCode = -32007
	MCPErrorTooManyRequests    MCPErrorCode = -32008
)

type LSPErrorCode int

const (
	LSPErrorUnknownErrorCode      LSPErrorCode = -32001
	LSPErrorRequestFailed         LSPErrorCode = -32803
	LSPErrorServerNotInitialized  LSPErrorCode = -32002
	LSPErrorRequestCancelled      LSPErrorCode = -32800
	LSPErrorContentModified       LSPErrorCode = -32801
	LSPErrorServerCancelled       LSPErrorCode = -32802
	LSPErrorInvalidFilePath       LSPErrorCode = -32900
	LSPErrorInvalidPosition       LSPErrorCode = -32901
	LSPErrorTextDocumentNotFound  LSPErrorCode = -32902
	LSPErrorSymbolNotFound        LSPErrorCode = -32903
	LSPErrorWorkspaceNotAvailable LSPErrorCode = -32904
)

type StructuredError struct {
	Code        MCPErrorCode           `json:"code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Retryable   bool                   `json:"retryable"`
}

func (e *StructuredError) Error() string {
	return e.Message
}

type ValidationError struct {
	Field    string      `json:"field"`
	Value    interface{} `json:"value"`
	Expected string      `json:"expected"`
	Actual   string      `json:"actual"`
	Message  string      `json:"message"`
}

// ParameterValidator provides methods for common parameter validation patterns
type ParameterValidator struct{}

// NewParameterValidator creates a new parameter validator
func NewParameterValidator() *ParameterValidator {
	return &ParameterValidator{}
}

// getStringParam safely extracts a string parameter from the arguments map
func getStringParam(args map[string]interface{}, key string, required bool) (string, error) {
	value, exists := args[key]
	if !exists {
		if required {
			return "", &StructuredError{
				Code:    MCPErrorInvalidParams,
				Message: fmt.Sprintf("Required parameter '%s' is missing", key),
				Details: fmt.Sprintf("Parameter '%s' must be provided as a string", key),
				Context: map[string]interface{}{
					"parameter": key,
					"required":  true,
				},
				Suggestions: []string{
					fmt.Sprintf("Provide the '%s' parameter", key),
					"Check the tool schema for required parameters",
				},
				Retryable: true,
			}
		}
		return "", nil
	}

	if value == nil {
		if required {
			return "", &StructuredError{
				Code:    MCPErrorInvalidParams,
				Message: fmt.Sprintf("Parameter '%s' cannot be null", key),
				Details: "String parameters must have non-null values",
				Context: map[string]interface{}{
					"parameter": key,
					"value":     nil,
				},
				Suggestions: []string{
					"Provide a valid string value",
					"Remove the parameter if it's optional",
				},
				Retryable: true,
			}
		}
		return "", nil
	}

	strValue, ok := value.(string)
	if !ok {
		return "", &StructuredError{
			Code:    MCPErrorInvalidParams,
			Message: fmt.Sprintf("Parameter '%s' must be a string", key),
			Details: fmt.Sprintf("Expected string, got %T", value),
			Context: map[string]interface{}{
				"parameter": key,
				"value":     value,
				"expected":  "string",
				"actual":    fmt.Sprintf("%T", value),
			},
			Suggestions: []string{
				"Provide a string value",
				"Check parameter type in the request",
			},
			Retryable: true,
		}
	}

	return strValue, nil
}

// getIntParam safely extracts an integer parameter from the arguments map
func getIntParam(args map[string]interface{}, key string, required bool, defaultValue int) (int, error) {
	value, exists := args[key]
	if !exists {
		if required {
			return 0, &StructuredError{
				Code:    MCPErrorInvalidParams,
				Message: fmt.Sprintf("Required parameter '%s' is missing", key),
				Details: fmt.Sprintf("Parameter '%s' must be provided as an integer", key),
				Context: map[string]interface{}{
					"parameter": key,
					"required":  true,
				},
				Suggestions: []string{
					fmt.Sprintf("Provide the '%s' parameter", key),
					"Check the tool schema for required parameters",
				},
				Retryable: true,
			}
		}
		return defaultValue, nil
	}

	if value == nil {
		if required {
			return 0, &StructuredError{
				Code:    MCPErrorInvalidParams,
				Message: fmt.Sprintf("Parameter '%s' cannot be null", key),
				Details: "Integer parameters must have non-null values",
				Context: map[string]interface{}{
					"parameter": key,
					"value":     nil,
				},
				Suggestions: []string{
					"Provide a valid integer value",
					"Remove the parameter if it's optional",
				},
				Retryable: true,
			}
		}
		return defaultValue, nil
	}

	// Handle different numeric types that JSON might produce
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		// JSON numbers are parsed as float64, convert to int if it's a whole number
		if v == float64(int(v)) {
			return int(v), nil
		}
		return 0, &StructuredError{
			Code:    MCPErrorInvalidParams,
			Message: fmt.Sprintf("Parameter '%s' must be an integer", key),
			Details: fmt.Sprintf("Decimal value %v cannot be converted to integer", v),
			Context: map[string]interface{}{
				"parameter": key,
				"value":     value,
				"expected":  "integer",
				"actual":    "decimal",
			},
			Suggestions: []string{
				"Provide a whole number",
				"Remove decimal places from the value",
			},
			Retryable: true,
		}
	case string:
		// Try to parse string as integer
		intVal, err := strconv.Atoi(v)
		if err != nil {
			return 0, &StructuredError{
				Code:    MCPErrorInvalidParams,
				Message: fmt.Sprintf("Parameter '%s' string value cannot be converted to integer", key),
				Details: fmt.Sprintf("String '%s' is not a valid integer", v),
				Context: map[string]interface{}{
					"parameter": key,
					"value":     value,
					"expected":  "integer",
					"actual":    "string",
				},
				Suggestions: []string{
					"Provide a numeric value",
					"Check the parameter format",
				},
				Retryable: true,
			}
		}
		return intVal, nil
	default:
		return 0, &StructuredError{
			Code:    MCPErrorInvalidParams,
			Message: fmt.Sprintf("Parameter '%s' must be an integer", key),
			Details: fmt.Sprintf("Expected integer, got %T", value),
			Context: map[string]interface{}{
				"parameter": key,
				"value":     value,
				"expected":  "integer",
				"actual":    fmt.Sprintf("%T", value),
			},
			Suggestions: []string{
				"Provide an integer value",
				"Check parameter type in the request",
			},
			Retryable: true,
		}
	}
}

// getBoolParam safely extracts a boolean parameter from the arguments map
func getBoolParam(args map[string]interface{}, key string, defaultValue bool) (bool, error) {
	value, exists := args[key]
	if !exists {
		return defaultValue, nil
	}

	if value == nil {
		return defaultValue, nil
	}

	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		// Parse common string representations of booleans
		lower := strings.ToLower(strings.TrimSpace(v))
		switch lower {
		case "true", "yes", "1", "on":
			return true, nil
		case "false", "no", "0", "off":
			return false, nil
		default:
			return false, &StructuredError{
				Code:    MCPErrorInvalidParams,
				Message: fmt.Sprintf("Parameter '%s' string value cannot be converted to boolean", key),
				Details: fmt.Sprintf("String '%s' is not a valid boolean", v),
				Context: map[string]interface{}{
					"parameter": key,
					"value":     value,
					"expected":  "boolean",
					"actual":    "string",
				},
				Suggestions: []string{
					"Use 'true' or 'false'",
					"Use boolean type instead of string",
				},
				Retryable: true,
			}
		}
	case int, int64:
		// Handle integer representations (0 = false, non-zero = true)
		intVal := 0
		if iv, ok := v.(int); ok {
			intVal = iv
		} else if iv64, ok := v.(int64); ok {
			intVal = int(iv64)
		}
		return intVal != 0, nil
	case float64:
		// Handle float representations (0.0 = false, non-zero = true)
		return v != 0.0, nil
	default:
		return false, &StructuredError{
			Code:    MCPErrorInvalidParams,
			Message: fmt.Sprintf("Parameter '%s' must be a boolean", key),
			Details: fmt.Sprintf("Expected boolean, got %T", value),
			Context: map[string]interface{}{
				"parameter": key,
				"value":     value,
				"expected":  "boolean",
				"actual":    fmt.Sprintf("%T", value),
			},
			Suggestions: []string{
				"Provide a boolean value (true/false)",
				"Check parameter type in the request",
			},
			Retryable: true,
		}
	}
}

// validateLSPPosition validates that line and character values are non-negative
func validateLSPPosition(line, character int) error {
	if line < 0 {
		return &StructuredError{
			Code:    MCPErrorInvalidParams,
			Message: "Line number must be non-negative",
			Details: fmt.Sprintf("Line number %d is invalid. LSP positions are 0-based.", line),
			Context: map[string]interface{}{
				"line":      line,
				"character": character,
				"min_line":  0,
			},
			Suggestions: []string{
				"Use line numbers starting from 0",
				"Check that line number is not negative",
			},
			Retryable: true,
		}
	}

	if character < 0 {
		return &StructuredError{
			Code:    MCPErrorInvalidParams,
			Message: "Character position must be non-negative",
			Details: fmt.Sprintf("Character position %d is invalid. LSP positions are 0-based.", character),
			Context: map[string]interface{}{
				"line":          line,
				"character":     character,
				"min_character": 0,
			},
			Suggestions: []string{
				"Use character positions starting from 0",
				"Check that character position is not negative",
			},
			Retryable: true,
		}
	}

	return nil
}

// validateURI performs basic URI format validation
func validateURI(uri string) error {
	if strings.TrimSpace(uri) == "" {
		return &StructuredError{
			Code:    MCPErrorInvalidURI,
			Message: "URI cannot be empty",
			Details: "A valid file URI must be provided",
			Context: map[string]interface{}{
				"uri": uri,
			},
			Suggestions: []string{
				"Provide a valid file URI (e.g., file:///path/to/file.go)",
				"Check that the URI is not empty or whitespace",
			},
			Retryable: true,
		}
	}

	parsedURI, err := url.Parse(uri)
	if err != nil {
		return &StructuredError{
			Code:    MCPErrorInvalidURI,
			Message: "Invalid URI format",
			Details: fmt.Sprintf("URI parsing failed: %v", err),
			Context: map[string]interface{}{
				"uri":   uri,
				"error": err.Error(),
			},
			Suggestions: []string{
				"Check URI syntax and encoding",
				"Ensure proper URI format (e.g., file:///path/to/file.go)",
			},
			Retryable: true,
		}
	}

	// Additional validation for file URIs (most common case)
	if parsedURI.Scheme != "" && parsedURI.Scheme != "file" {
		return &StructuredError{
			Code:    MCPErrorInvalidURI,
			Message: "Unsupported URI scheme",
			Details: fmt.Sprintf("Scheme '%s' is not supported. Only 'file' URIs are currently supported.", parsedURI.Scheme),
			Context: map[string]interface{}{
				"uri":    uri,
				"scheme": parsedURI.Scheme,
			},
			Suggestions: []string{
				"Use file:// URI scheme",
				"Convert to absolute file path with file:// prefix",
			},
			Retryable: true,
		}
	}

	return nil
}

// ValidateRequiredParams validates that all required parameters are present and valid
func (v *ParameterValidator) ValidateRequiredParams(args map[string]interface{}, required []string) error {
	for _, param := range required {
		if _, exists := args[param]; !exists {
			return &StructuredError{
				Code:    MCPErrorInvalidParams,
				Message: fmt.Sprintf("Required parameter '%s' is missing", param),
				Details: "All required parameters must be provided",
				Context: map[string]interface{}{
					"missing_parameter":   param,
					"required_parameters": required,
				},
				Suggestions: []string{
					fmt.Sprintf("Add the '%s' parameter to your request", param),
					"Check the tool schema for all required parameters",
				},
				Retryable: true,
			}
		}
	}
	return nil
}

// ValidatePositionParams validates URI, line, and character parameters commonly used in LSP requests
func (v *ParameterValidator) ValidatePositionParams(args map[string]interface{}) (string, int, int, error) {
	uri, err := getStringParam(args, "uri", true)
	if err != nil {
		return "", 0, 0, err
	}

	if err := validateURI(uri); err != nil {
		return "", 0, 0, err
	}

	line, err := getIntParam(args, "line", true, 0)
	if err != nil {
		return "", 0, 0, err
	}

	character, err := getIntParam(args, "character", true, 0)
	if err != nil {
		return "", 0, 0, err
	}

	if err := validateLSPPosition(line, character); err != nil {
		return "", 0, 0, err
	}

	return uri, line, character, nil
}

// createValidationErrorResult creates a ToolResult with validation error details
func createValidationErrorResult(validationErrors []ValidationError) *ToolResult {
	var messages []string
	var suggestions []string
	context := make(map[string]interface{})
	
	for _, ve := range validationErrors {
		messages = append(messages, ve.Message)
		context[ve.Field] = map[string]interface{}{
			"value":    ve.Value,
			"expected": ve.Expected,
			"actual":   ve.Actual,
		}
	}

	// Generate comprehensive suggestions based on validation errors
	suggestions = append(suggestions, "Check all parameter values and types")
	suggestions = append(suggestions, "Refer to the tool schema for correct parameter formats")
	if len(validationErrors) > 1 {
		suggestions = append(suggestions, "Fix all validation errors before retrying")
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Validation failed: %s", strings.Join(messages, "; ")),
		}},
		IsError: true,
		Error: &StructuredError{
			Code:        MCPErrorValidationFailed,
			Message:     "Multiple validation errors occurred",
			Details:     fmt.Sprintf("Found %d validation error(s)", len(validationErrors)),
			Context:     context,
			Suggestions: suggestions,
			Retryable:   true,
		},
	}
}

// createParameterMissingError creates a ToolResult for missing parameter errors
func createParameterMissingError(paramName, toolName string) *ToolResult {
	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Required parameter '%s' is missing for tool '%s'", paramName, toolName),
		}},
		IsError: true,
		Error: &StructuredError{
			Code:    MCPErrorInvalidParams,
			Message: fmt.Sprintf("Required parameter '%s' is missing for tool '%s'", paramName, toolName),
			Details: fmt.Sprintf("The '%s' tool requires the '%s' parameter to function properly", toolName, paramName),
			Context: map[string]interface{}{
				"parameter": paramName,
				"tool":      toolName,
				"required":  true,
			},
			Suggestions: []string{
				fmt.Sprintf("Add the '%s' parameter to your request", paramName),
				fmt.Sprintf("Check the '%s' tool schema for all required parameters", toolName),
				"Ensure all required parameters are included in the request body",
			},
			Retryable: true,
		},
	}
}

// createParameterTypeError creates a ToolResult for parameter type errors
func createParameterTypeError(paramName, expected, actual, toolName string) *ToolResult {
	var suggestions []string
	
	// Add specific suggestions based on the parameter type
	switch expected {
	case "string":
		suggestions = []string{
			"Provide a string value (wrap in quotes if needed)",
			"Check that the value is not a number or boolean",
		}
	case "integer":
		suggestions = []string{
			"Provide a numeric value (whole number)",
			"Remove quotes if the value is wrapped in strings",
			"Ensure the value is not a decimal number",
		}
	case "boolean":
		suggestions = []string{
			"Use true or false (without quotes)",
			"Use 1 or 0 for numeric boolean representation",
		}
	case "file:// URI":
		suggestions = []string{
			"Use file:// URI format (e.g., file:///path/to/file.go)",
			"Convert relative paths to absolute file URIs",
			"Ensure proper URI encoding for special characters",
		}
	default:
		suggestions = []string{
			fmt.Sprintf("Provide a value of type '%s'", expected),
			"Check the parameter type in your request",
		}
	}
	
	suggestions = append(suggestions, fmt.Sprintf("Refer to the '%s' tool documentation for parameter specifications", toolName))

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Parameter '%s' type error for tool '%s': expected %s, got %s", paramName, toolName, expected, actual),
		}},
		IsError: true,
		Error: &StructuredError{
			Code:    MCPErrorInvalidParams,
			Message: fmt.Sprintf("Parameter '%s' has invalid type for tool '%s'", paramName, toolName),
			Details: fmt.Sprintf("Expected %s but received %s", expected, actual),
			Context: map[string]interface{}{
				"parameter": paramName,
				"tool":      toolName,
				"expected":  expected,
				"actual":    actual,
			},
			Suggestions: suggestions,
			Retryable:   true,
		},
	}
}

// createLSPRequestError creates a ToolResult for LSP request errors with enhanced context
func createLSPRequestError(method, toolName string, err error, requestParams map[string]interface{}) *ToolResult {
	var suggestions []string
	var errorCode MCPErrorCode = MCPErrorLSPServerError
	
	errorMsg := err.Error()
	
	// Analyze error type and provide specific suggestions
	if strings.Contains(strings.ToLower(errorMsg), "connection") {
		errorCode = MCPErrorConnectionFailed
		suggestions = []string{
			"Check if the language server is running",
			"Verify the language server configuration",
			"Restart the language server if connection issues persist",
		}
	} else if strings.Contains(strings.ToLower(errorMsg), "timeout") {
		errorCode = MCPErrorTimeout
		suggestions = []string{
			"Try the request again - the server may be busy",
			"Check if the file is very large and may take time to process",
			"Verify the language server is responding to other requests",
		}
	} else if strings.Contains(strings.ToLower(errorMsg), "not found") || strings.Contains(strings.ToLower(errorMsg), "invalid") {
		errorCode = MCPErrorResourceNotFound
		suggestions = []string{
			"Verify the file path exists and is accessible",
			"Check that the file is within the workspace",
			"Ensure the file has been saved and is not a temporary file",
		}
	} else {
		suggestions = []string{
			"Check the language server logs for more details",
			"Verify the request parameters are correct",
			"Try the request again - this may be a temporary issue",
		}
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("LSP request failed for tool '%s': %s", toolName, errorMsg),
		}},
		IsError: true,
		Error: &StructuredError{
			Code:    errorCode,
			Message: fmt.Sprintf("Language server request failed for '%s' tool", toolName),
			Details: fmt.Sprintf("LSP method '%s' failed: %s", method, errorMsg),
			Context: map[string]interface{}{
				"tool":           toolName,
				"lsp_method":     method,
				"request_params": requestParams,
				"error":          errorMsg,
			},
			Suggestions: suggestions,
			Retryable:   errorCode != MCPErrorResourceNotFound,
		},
	}
}

// createURIValidationError creates detailed URI validation errors with format examples
func createURIValidationError(uri, toolName string, parseError error) *ToolResult {
	var suggestions []string
	var details string
	
	if strings.TrimSpace(uri) == "" {
		details = "URI cannot be empty or whitespace"
		suggestions = []string{
			"Provide a valid file URI (e.g., file:///home/user/project/main.go)",
			"Use absolute file paths with file:// prefix",
			"Ensure the URI is not empty or contains only whitespace",
		}
	} else if parseError != nil {
		details = fmt.Sprintf("URI parsing failed: %s", parseError.Error())
		suggestions = []string{
			"Check URI syntax and special character encoding",
			"Ensure proper URI format: file:///absolute/path/to/file",
			"Use forward slashes (/) even on Windows systems",
			"Encode special characters in file paths (spaces, unicode, etc.)",
		}
	} else {
		// Check for common URI issues
		if !strings.HasPrefix(uri, "file://") {
			details = "URI must use file:// scheme for local files"
			suggestions = []string{
				fmt.Sprintf("Add file:// prefix: file://%s", uri),
				"Use file:// scheme for local file access",
				"Convert relative paths to absolute file URIs",
			}
		} else {
			details = "URI format validation failed"
			suggestions = []string{
				"Use proper file URI format: file:///absolute/path/to/file",
				"Check that the file path is absolute (starts with /)",
				"Verify file path syntax and encoding",
			}
		}
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Invalid URI format for tool '%s': %s", toolName, details),
		}},
		IsError: true,
		Error: &StructuredError{
			Code:    MCPErrorInvalidURI,
			Message: fmt.Sprintf("Invalid URI format for tool '%s'", toolName),
			Details: details,
			Context: map[string]interface{}{
				"tool":       toolName,
				"uri":        uri,
				"valid_examples": []string{
					"file:///home/user/project/main.go",
					"file:///C:/Users/user/project/main.js",
					"file:///path/to/file.py",
				},
			},
			Suggestions: suggestions,
			Retryable:   true,
		},
	}
}

// createPositionValidationError creates detailed position validation errors
func createPositionValidationError(line, character int, toolName string) *ToolResult {
	var details string
	var suggestions []string
	
	if line < 0 && character < 0 {
		details = fmt.Sprintf("Both line (%d) and character (%d) positions are invalid. LSP positions are 0-based and must be non-negative.", line, character)
		suggestions = []string{
			"Use line and character positions starting from 0",
			"Check that both position values are not negative",
			"Verify the cursor position in your editor (convert to 0-based)",
		}
	} else if line < 0 {
		details = fmt.Sprintf("Line number %d is invalid. LSP line positions are 0-based and must be non-negative.", line)
		suggestions = []string{
			"Use line numbers starting from 0 (first line is 0)",
			"Convert 1-based line numbers to 0-based (subtract 1)",
			"Check that line number is not negative",
		}
	} else if character < 0 {
		details = fmt.Sprintf("Character position %d is invalid. LSP character positions are 0-based and must be non-negative.", character)
		suggestions = []string{
			"Use character positions starting from 0 (first character is 0)",
			"Convert 1-based column numbers to 0-based (subtract 1)",
			"Check that character position is not negative",
		}
	} else {
		details = "Position validation failed for unknown reason"
		suggestions = []string{
			"Ensure both line and character are non-negative integers",
			"Use 0-based positioning (first line and character are 0)",
		}
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Invalid position for tool '%s': %s", toolName, details),
		}},
		IsError: true,
		Error: &StructuredError{
			Code:    MCPErrorInvalidParams,
			Message: fmt.Sprintf("Invalid position parameters for tool '%s'", toolName),
			Details: details,
			Context: map[string]interface{}{
				"tool":      toolName,
				"line":      line,
				"character": character,
				"min_line":  0,
				"min_char":  0,
				"position_format": "0-based indexing",
			},
			Suggestions: suggestions,
			Retryable:   true,
		},
	}
}

type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolResult struct {
	Content []ContentBlock    `json:"content"`
	IsError bool              `json:"isError,omitempty"`
	Error   *StructuredError  `json:"error,omitempty"`
	Meta    *ResponseMetadata `json:"meta,omitempty"`
}

type ResponseMetadata struct {
	Timestamp   string                 `json:"timestamp"`
	Duration    string                 `json:"duration,omitempty"`
	LSPMethod   string                 `json:"lspMethod"`
	CacheHit    bool                   `json:"cacheHit,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	RequestInfo map[string]interface{} `json:"requestInfo,omitempty"`
}

type ContentBlock struct {
	Type        string                 `json:"type"`
	Text        string                 `json:"text,omitempty"`
	Data        interface{}            `json:"data,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// LSPClient interface defines the methods needed for LSP communication
type LSPClient interface {
	SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
}

type ToolHandler struct {
	Client LSPClient
	Tools  map[string]Tool
}

func NewToolHandler(client LSPClient) *ToolHandler {
	handler := &ToolHandler{
		Client: client,
		Tools:  make(map[string]Tool),
	}

	handler.RegisterDefaultTools()

	return handler
}

// NewToolHandlerWithDirectLSP creates a ToolHandler with DirectLSPManager as the LSP client
// DirectLSPManager implements the LSPClient interface, so it can be used directly
func NewToolHandlerWithDirectLSP(directLSPManager *DirectLSPManager) *ToolHandler {
	return NewToolHandler(directLSPManager)
}

func (h *ToolHandler) RegisterDefaultTools() {
	gotoDefTitle := "Go to Definition"
	gotoDefOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"uri": map[string]interface{}{"type": "string"},
									"range": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"start": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"line": map[string]interface{}{"type": "integer"},
													"character": map[string]interface{}{"type": "integer"},
												},
											},
											"end": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"line": map[string]interface{}{"type": "integer"},
													"character": map[string]interface{}{"type": "integer"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"lspMethod": map[string]interface{}{"type": "string"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["goto_definition"] = Tool{
		Name:         "goto_definition",
		Title:        &gotoDefTitle,
		Description:  "Navigate to the definition of a symbol at a specific position in a file",
		OutputSchema: gotoDefOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI (e.g., file:///path/to/file.go)",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number (0-based)",
				},
				"character": map[string]interface{}{
					"type":        "integer",
					"description": "Character position (0-based)",
				},
			},
			"required": []string{"uri", "line", "character"},
		},
	}

	findRefsTitle := "Find References"
	findRefsOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"uri": map[string]interface{}{"type": "string"},
									"range": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"start": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"line": map[string]interface{}{"type": "integer"},
													"character": map[string]interface{}{"type": "integer"},
												},
											},
											"end": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"line": map[string]interface{}{"type": "integer"},
													"character": map[string]interface{}{"type": "integer"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"lspMethod": map[string]interface{}{"type": "string"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["find_references"] = Tool{
		Name:         "find_references",
		Title:        &findRefsTitle,
		Description:  "Find all references to a symbol at a specific position in a file",
		OutputSchema: findRefsOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI (e.g., file:///path/to/file.go)",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number (0-based)",
				},
				"character": map[string]interface{}{
					"type":        "integer",
					"description": "Character position (0-based)",
				},
				"includeDeclaration": map[string]interface{}{
					"type":        "boolean",
					"description": "Include the declaration in the results",
					"default":     true,
				},
			},
			"required": []string{"uri", "line", "character"},
		},
	}

	hoverTitle := "Get Hover Information"
	hoverOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"contents": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"language": map[string]interface{}{"type": "string"},
											"value": map[string]interface{}{"type": "string"},
										},
									},
								},
								"range": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"start": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"line": map[string]interface{}{"type": "integer"},
												"character": map[string]interface{}{"type": "integer"},
											},
										},
										"end": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"line": map[string]interface{}{"type": "integer"},
												"character": map[string]interface{}{"type": "integer"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"lspMethod": map[string]interface{}{"type": "string"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["get_hover_info"] = Tool{
		Name:         "get_hover_info",
		Title:        &hoverTitle,
		Description:  "Get hover information for a symbol at a specific position in a file",
		OutputSchema: hoverOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI (e.g., file:///path/to/file.go)",
				},
				"line": map[string]interface{}{
					"type":        "integer",
					"description": "Line number (0-based)",
				},
				"character": map[string]interface{}{
					"type":        "integer",
					"description": "Character position (0-based)",
				},
			},
			"required": []string{"uri", "line", "character"},
		},
	}

	docSymbolsTitle := "Get Document Symbols"
	docSymbolsOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{"type": "string"},
									"kind": map[string]interface{}{"type": "integer"},
									"range": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"start": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"line": map[string]interface{}{"type": "integer"},
													"character": map[string]interface{}{"type": "integer"},
												},
											},
											"end": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"line": map[string]interface{}{"type": "integer"},
													"character": map[string]interface{}{"type": "integer"},
												},
											},
										},
									},
									"selectionRange": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"start": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"line": map[string]interface{}{"type": "integer"},
													"character": map[string]interface{}{"type": "integer"},
												},
											},
											"end": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"line": map[string]interface{}{"type": "integer"},
													"character": map[string]interface{}{"type": "integer"},
												},
											},
										},
									},
									"children": map[string]interface{}{
										"type": "array",
										"items": map[string]interface{}{"type": "object"},
									},
								},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"lspMethod": map[string]interface{}{"type": "string"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["get_document_symbols"] = Tool{
		Name:         "get_document_symbols",
		Title:        &docSymbolsTitle,
		Description:  "Get all symbols in a document",
		OutputSchema: docSymbolsOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"uri": map[string]interface{}{
					"type":        "string",
					"description": "File URI (e.g., file:///path/to/file.go)",
				},
			},
			"required": []string{"uri"},
		},
	}

	workspaceSymbolsTitle := "Search Workspace Symbols"
	workspaceSymbolsOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{"type": "string"},
									"kind": map[string]interface{}{"type": "integer"},
									"location": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"uri": map[string]interface{}{"type": "string"},
											"range": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"start": map[string]interface{}{
														"type": "object",
														"properties": map[string]interface{}{
															"line": map[string]interface{}{"type": "integer"},
															"character": map[string]interface{}{"type": "integer"},
														},
													},
													"end": map[string]interface{}{
														"type": "object",
														"properties": map[string]interface{}{
															"line": map[string]interface{}{"type": "integer"},
															"character": map[string]interface{}{"type": "integer"},
														},
													},
												},
											},
										},
									},
									"containerName": map[string]interface{}{"type": "string"},
								},
							},
						},
					},
				},
			},
			"isError": map[string]interface{}{"type": "boolean"},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"duration": map[string]interface{}{"type": "string"},
					"lspMethod": map[string]interface{}{"type": "string"},
					"cacheHit": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	}
	h.Tools["search_workspace_symbols"] = Tool{
		Name:         "search_workspace_symbols",
		Title:        &workspaceSymbolsTitle,
		Description:  "Search for symbols in the workspace",
		OutputSchema: workspaceSymbolsOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query for symbol names",
				},
			},
			"required": []string{"query"},
		},
	}
}

func (h *ToolHandler) ListTools() []Tool {
	tools := make([]Tool, 0, len(h.Tools))
	for _, tool := range h.Tools {
		tools = append(tools, tool)
	}
	return tools
}

func (h *ToolHandler) CallTool(ctx context.Context, call ToolCall) (*ToolResult, error) {
	tool, exists := h.Tools[call.Name]
	if !exists {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Unknown tool: %s", call.Name),
			}},
			IsError: true,
			Error: &StructuredError{
				Code:    MCPErrorMethodNotFound,
				Message: fmt.Sprintf("Tool '%s' is not available", call.Name),
				Details: "The requested tool is not registered or does not exist",
				Context: map[string]interface{}{
					"tool":           call.Name,
					"available_tools": h.getAvailableToolNames(),
				},
				Suggestions: []string{
					"Check the tool name for typos",
					"Use one of the available tools listed in the context",
					"Verify the tool is supported in this version",
				},
				Retryable: false,
			},
		}, nil
	}

	switch call.Name {
	case "goto_definition":
		return h.handleGotoDefinition(ctx, call.Arguments)
	case "find_references":
		return h.handleFindReferences(ctx, call.Arguments)
	case "get_hover_info":
		return h.handleGetHoverInfo(ctx, call.Arguments)
	case "get_document_symbols":
		return h.handleGetDocumentSymbols(ctx, call.Arguments)
	case "search_workspace_symbols":
		return h.handleSearchWorkspaceSymbols(ctx, call.Arguments)
	default:
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Tool '%s' exists but is not implemented", tool.Name),
			}},
			IsError: true,
			Error: &StructuredError{
				Code:    MCPErrorUnsupportedFeature,
				Message: fmt.Sprintf("Tool '%s' is not implemented", tool.Name),
				Details: "The tool is registered but its handler is not implemented",
				Context: map[string]interface{}{
					"tool": tool.Name,
				},
				Suggestions: []string{
					"Contact support to request implementation of this tool",
					"Use an alternative tool if available",
				},
				Retryable: false,
			},
		}, nil
	}
}

// getAvailableToolNames returns a list of available tool names
func (h *ToolHandler) getAvailableToolNames() []string {
	names := make([]string, 0, len(h.Tools))
	for name := range h.Tools {
		names = append(names, name)
	}
	return names
}

func (h *ToolHandler) handleGotoDefinition(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return h.handleGotoDefinitionWithContext(ctx, args, nil)
}

// Enhanced version with workspace context support
func (h *ToolHandler) handleGotoDefinitionWithContext(ctx context.Context, args map[string]interface{}, workspaceCtx *WorkspaceContext) (*ToolResult, error) {
	startTime := time.Now()

	// Validate required parameters
	validator := NewParameterValidator()
	uri, line, character, err := validator.ValidatePositionParams(args)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: "Parameter validation failed",
			}},
			IsError: true,
			Error:   err.(*StructuredError),
		}, nil
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}

	result, err := h.Client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err != nil {
		return createLSPRequestError("textDocument/definition", "goto_definition", err, params), nil
	}

	// Create enhanced result with project context
	toolResult := &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
			Data: h.parseDefinitionResult(result),
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			LSPMethod: "textDocument/definition",
		},
	}

	// Enhance with workspace context if available
	if workspaceCtx != nil && workspaceCtx.IsProjectAware() {
		toolResult = h.enhanceDefinitionWithProjectContext(toolResult, args, workspaceCtx)
	}

	return toolResult, nil
}

func (h *ToolHandler) handleFindReferences(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return h.handleFindReferencesWithContext(ctx, args, nil)
}

// Enhanced version with workspace context support
func (h *ToolHandler) handleFindReferencesWithContext(ctx context.Context, args map[string]interface{}, workspaceCtx *WorkspaceContext) (*ToolResult, error) {
	startTime := time.Now()

	// Validate required parameters
	validator := NewParameterValidator()
	uri, line, character, err := validator.ValidatePositionParams(args)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: "Parameter validation failed",
			}},
			IsError: true,
			Error:   err.(*StructuredError),
		}, nil
	}

	// Validate includeDeclaration parameter
	includeDeclaration, err := getBoolParam(args, "includeDeclaration", true)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: "Parameter validation failed",
			}},
			IsError: true,
			Error:   err.(*StructuredError),
		}, nil
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": includeDeclaration,
		},
	}

	result, err := h.Client.SendLSPRequest(ctx, "textDocument/references", params)
	if err != nil {
		return createLSPRequestError("textDocument/references", "find_references", err, params), nil
	}

	// Create enhanced result with project context
	toolResult := &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
			Data: h.parseReferencesResult(result),
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			LSPMethod: "textDocument/references",
		},
	}

	// Enhance with workspace context if available
	if workspaceCtx != nil && workspaceCtx.IsProjectAware() {
		toolResult = h.enhanceReferencesWithProjectContext(toolResult, args, workspaceCtx)
	}

	return toolResult, nil
}

func (h *ToolHandler) handleGetHoverInfo(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return h.handleGetHoverInfoWithContext(ctx, args, nil)
}

// Enhanced version with workspace context support
func (h *ToolHandler) handleGetHoverInfoWithContext(ctx context.Context, args map[string]interface{}, workspaceCtx *WorkspaceContext) (*ToolResult, error) {
	startTime := time.Now()

	// Validate required parameters
	validator := NewParameterValidator()
	uri, line, character, err := validator.ValidatePositionParams(args)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: "Parameter validation failed",
			}},
			IsError: true,
			Error:   err.(*StructuredError),
		}, nil
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
		"position": map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}

	result, err := h.Client.SendLSPRequest(ctx, LSPMethodHover, params)
	if err != nil {
		return createLSPRequestError(LSPMethodHover, "get_hover_info", err, params), nil
	}

	// Create enhanced result with project context
	toolResult := &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
			Data: h.parseHoverResult(result),
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			LSPMethod: LSPMethodHover,
		},
	}

	// Enhance with workspace context if available
	if workspaceCtx != nil && workspaceCtx.IsProjectAware() {
		toolResult = h.enhanceHoverWithProjectContext(toolResult, args, workspaceCtx)
	}

	return toolResult, nil
}

func (h *ToolHandler) handleGetDocumentSymbols(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return h.handleGetDocumentSymbolsWithContext(ctx, args, nil)
}

// Enhanced version with workspace context support
func (h *ToolHandler) handleGetDocumentSymbolsWithContext(ctx context.Context, args map[string]interface{}, workspaceCtx *WorkspaceContext) (*ToolResult, error) {
	startTime := time.Now()

	// Validate required parameters
	uri, err := getStringParam(args, "uri", true)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: "Parameter validation failed",
			}},
			IsError: true,
			Error:   err.(*StructuredError),
		}, nil
	}

	if err := validateURI(uri); err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: "Parameter validation failed",
			}},
			IsError: true,
			Error:   err.(*StructuredError),
		}, nil
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}

	result, err := h.Client.SendLSPRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return createLSPRequestError("textDocument/documentSymbol", "get_document_symbols", err, params), nil
	}

	// Create enhanced result with project context
	toolResult := &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
			Data: h.parseDocumentSymbolsResult(result),
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			LSPMethod: "textDocument/documentSymbol",
		},
	}

	// Enhance with workspace context if available
	if workspaceCtx != nil && workspaceCtx.IsProjectAware() {
		toolResult = h.enhanceDocumentSymbolsWithProjectContext(toolResult, args, workspaceCtx)
	}

	return toolResult, nil
}

func (h *ToolHandler) handleSearchWorkspaceSymbols(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return h.handleSearchWorkspaceSymbolsWithContext(ctx, args, nil)
}

// Enhanced version with workspace context support
func (h *ToolHandler) handleSearchWorkspaceSymbolsWithContext(ctx context.Context, args map[string]interface{}, workspaceCtx *WorkspaceContext) (*ToolResult, error) {
	startTime := time.Now()

	// Validate required parameters
	query, err := getStringParam(args, "query", true)
	if err != nil {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: "Parameter validation failed",
			}},
			IsError: true,
			Error:   err.(*StructuredError),
		}, nil
	}

	params := map[string]interface{}{
		"query": query,
	}

	result, err := h.Client.SendLSPRequest(ctx, "workspace/symbol", params)
	if err != nil {
		return createLSPRequestError("workspace/symbol", "search_workspace_symbols", err, params), nil
	}

	// Create enhanced result with project context
	toolResult := &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: string(result),
			Data: h.parseWorkspaceSymbolsResult(result),
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			Duration:  time.Since(startTime).String(),
			LSPMethod: "workspace/symbol",
		},
	}

	// Enhance with workspace context if available
	if workspaceCtx != nil && workspaceCtx.IsProjectAware() {
		toolResult = h.enhanceWorkspaceSymbolsWithProjectContext(toolResult, args, workspaceCtx)
	}

	return toolResult, nil
}
