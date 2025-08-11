package errors

import (
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
)

// TextDocumentIdentifier represents an LSP text document identifier
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// Position represents an LSP position with line and character
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// ValidateURI validates and extracts URI from parameters
func ValidateURI(params map[string]interface{}) (string, error) {
	if params == nil {
		return "", NewValidationError("params", "no parameters provided")
	}

	// Try direct URI field first
	if uri, exists := params["uri"]; exists {
		if uriStr, ok := uri.(string); ok && uriStr != "" {
			return validateURIFormat(uriStr)
		}
		return "", NewValidationError("uri", "URI must be a non-empty string")
	}

	// Try textDocument.uri pattern
	if textDoc, exists := params["textDocument"]; exists {
		if textDocMap, ok := textDoc.(map[string]interface{}); ok {
			if uri, exists := textDocMap["uri"]; exists {
				if uriStr, ok := uri.(string); ok && uriStr != "" {
					return validateURIFormat(uriStr)
				}
				return "", NewValidationError("textDocument.uri", "URI must be a non-empty string")
			}
		}
		return "", NewValidationError("textDocument", "textDocument must contain uri field")
	}

	return "", NewValidationError("uri", "no URI found in parameters")
}

// ValidatePosition validates and extracts position from parameters
func ValidatePosition(params map[string]interface{}) (line, character int, err error) {
	if params == nil {
		return 0, 0, NewValidationError("params", "no parameters provided")
	}

	position, exists := params["position"]
	if !exists {
		return 0, 0, NewValidationError("position", "position parameter is required")
	}

	positionMap, ok := position.(map[string]interface{})
	if !ok {
		return 0, 0, NewValidationError("position", "position must be an object")
	}

	// Validate line
	lineVal, exists := positionMap["line"]
	if !exists {
		return 0, 0, NewValidationError("position.line", "line is required")
	}

	line, err = validateInteger(lineVal, "position.line", 0, -1)
	if err != nil {
		return 0, 0, err
	}

	// Validate character
	charVal, exists := positionMap["character"]
	if !exists {
		return 0, 0, NewValidationError("position.character", "character is required")
	}

	character, err = validateInteger(charVal, "position.character", 0, -1)
	if err != nil {
		return 0, 0, err
	}

	return line, character, nil
}

// ValidateTextDocument validates and extracts text document from parameters
func ValidateTextDocument(params map[string]interface{}) (*TextDocumentIdentifier, error) {
	if params == nil {
		return nil, NewValidationError("params", "no parameters provided")
	}

	textDoc, exists := params["textDocument"]
	if !exists {
		return nil, NewValidationError("textDocument", "textDocument parameter is required")
	}

	textDocMap, ok := textDoc.(map[string]interface{})
	if !ok {
		return nil, NewValidationError("textDocument", "textDocument must be an object")
	}

	uri, exists := textDocMap["uri"]
	if !exists {
		return nil, NewValidationError("textDocument.uri", "textDocument.uri is required")
	}

	uriStr, ok := uri.(string)
	if !ok || uriStr == "" {
		return nil, NewValidationError("textDocument.uri", "textDocument.uri must be a non-empty string")
	}

	validatedURI, err := validateURIFormat(uriStr)
	if err != nil {
		return nil, err
	}

	return &TextDocumentIdentifier{URI: validatedURI}, nil
}

// ValidateRequired checks that all required parameters are present
func ValidateRequired(params map[string]interface{}, required ...string) error {
	if params == nil {
		return NewValidationError("params", "no parameters provided")
	}

	var missing []string
	for _, param := range required {
		if _, exists := params[param]; !exists {
			missing = append(missing, param)
		}
	}

	if len(missing) > 0 {
		return NewValidationError("required_parameters",
			fmt.Sprintf("missing required parameters: %s", strings.Join(missing, ", ")))
	}

	return nil
}

// ValidateOptionalString validates an optional string parameter
func ValidateOptionalString(params map[string]interface{}, paramName string) (string, error) {
	if params == nil {
		return "", nil
	}

	val, exists := params[paramName]
	if !exists {
		return "", nil
	}

	if val == nil {
		return "", nil
	}

	strVal, ok := val.(string)
	if !ok {
		return "", NewValidationError(paramName, "must be a string")
	}

	return strVal, nil
}

// ValidateOptionalInt validates an optional integer parameter
func ValidateOptionalInt(params map[string]interface{}, paramName string, minVal int, maxVal int) (int, bool, error) {
	if params == nil {
		return 0, false, nil
	}

	val, exists := params[paramName]
	if !exists {
		return 0, false, nil
	}

	if val == nil {
		return 0, false, nil
	}

	intVal, err := validateInteger(val, paramName, minVal, maxVal)
	if err != nil {
		return 0, false, err
	}

	return intVal, true, nil
}

// ValidateOptionalBool validates an optional boolean parameter
func ValidateOptionalBool(params map[string]interface{}, paramName string) (bool, bool, error) {
	if params == nil {
		return false, false, nil
	}

	val, exists := params[paramName]
	if !exists {
		return false, false, nil
	}

	if val == nil {
		return false, false, nil
	}

	boolVal, ok := val.(bool)
	if !ok {
		return false, false, NewValidationError(paramName, "must be a boolean")
	}

	return boolVal, true, nil
}

// ValidateArray validates that a parameter is an array and returns its length
func ValidateArray(params map[string]interface{}, paramName string, required bool) ([]interface{}, error) {
	if params == nil {
		if required {
			return nil, NewValidationError("params", "no parameters provided")
		}
		return nil, nil
	}

	val, exists := params[paramName]
	if !exists {
		if required {
			return nil, NewValidationError(paramName, "parameter is required")
		}
		return nil, nil
	}

	if val == nil {
		if required {
			return nil, NewValidationError(paramName, "parameter cannot be null")
		}
		return nil, nil
	}

	arrayVal, ok := val.([]interface{})
	if !ok {
		return nil, NewValidationError(paramName, "must be an array")
	}

	return arrayVal, nil
}

// ValidateWorkspaceSymbolParams validates workspace/symbol request parameters
func ValidateWorkspaceSymbolParams(params map[string]interface{}) (string, error) {
	if params == nil {
		return "", NewValidationError("params", "no parameters provided")
	}

	query, exists := params["query"]
	if !exists {
		return "", NewValidationError("query", "query parameter is required")
	}

	queryStr, ok := query.(string)
	if !ok {
		return "", NewValidationError("query", "query must be a string")
	}

	// Query can be empty string for workspace symbol requests
	return queryStr, nil
}

// ValidateCompletionParams validates textDocument/completion request parameters
func ValidateCompletionParams(params map[string]interface{}) (*TextDocumentIdentifier, int, int, error) {
	textDoc, err := ValidateTextDocument(params)
	if err != nil {
		return nil, 0, 0, err
	}

	line, character, err := ValidatePosition(params)
	if err != nil {
		return nil, 0, 0, err
	}

	return textDoc, line, character, nil
}

// Helper functions

// validateURIFormat validates URI format and converts file paths to file:// URIs
func validateURIFormat(uriStr string) (string, error) {
	if uriStr == "" {
		return "", NewValidationError("uri", "URI cannot be empty")
	}

	// If it looks like a file path, convert to file:// URI
	if !strings.Contains(uriStr, "://") {
		// Clean the path and convert to file:// URI
		cleanPath := filepath.Clean(uriStr)
		if !filepath.IsAbs(cleanPath) {
			return "", NewValidationError("uri", "relative file paths are not supported")
		}
		uriStr = "file://" + cleanPath
	}

	// Validate URI format
	parsedURL, err := url.Parse(uriStr)
	if err != nil {
		return "", NewValidationError("uri", fmt.Sprintf("invalid URI format: %v", err))
	}

	// Ensure scheme is present
	if parsedURL.Scheme == "" {
		return "", NewValidationError("uri", "URI must have a scheme (e.g., file://)")
	}

	return uriStr, nil
}

// validateInteger validates integer values with optional min/max bounds
func validateInteger(val interface{}, paramName string, minVal int, maxVal int) (int, error) {
	var intVal int

	switch v := val.(type) {
	case int:
		intVal = v
	case int32:
		intVal = int(v)
	case int64:
		intVal = int(v)
	case float64:
		// JSON numbers come as float64
		if v != float64(int(v)) {
			return 0, NewValidationError(paramName, "must be an integer")
		}
		intVal = int(v)
	default:
		return 0, NewValidationError(paramName, fmt.Sprintf("must be an integer, got %T", val))
	}

	// Check bounds
	if minVal >= 0 && intVal < minVal {
		return 0, NewValidationError(paramName, fmt.Sprintf("must be >= %d", minVal))
	}
	if maxVal >= 0 && intVal > maxVal {
		return 0, NewValidationError(paramName, fmt.Sprintf("must be <= %d", maxVal))
	}

	return intVal, nil
}

// ValidateParamMap validates that params is not nil and converts it to a map[string]interface{}
// This is a compatibility function that can be used during migration
func ValidateParamMap(params interface{}) (map[string]interface{}, error) {
	if params == nil {
		return nil, NewValidationError("params", "no parameters provided")
	}

	paramMap, ok := params.(map[string]interface{})
	if !ok {
		return nil, NewValidationError("params",
			fmt.Sprintf("params must be an object, got %s", reflect.TypeOf(params)))
	}

	return paramMap, nil
}

// ValidateStringParam validates a required string parameter
func ValidateStringParam(params map[string]interface{}, paramName string) (string, error) {
	if params == nil {
		return "", NewValidationError("params", "no parameters provided")
	}

	val, exists := params[paramName]
	if !exists {
		return "", NewValidationError(paramName, "parameter is required")
	}

	if val == nil {
		return "", NewValidationError(paramName, "parameter cannot be null")
	}

	strVal, ok := val.(string)
	if !ok {
		return "", NewValidationError(paramName, "must be a string")
	}

	if strVal == "" {
		return "", NewValidationError(paramName, "cannot be empty")
	}

	return strVal, nil
}
