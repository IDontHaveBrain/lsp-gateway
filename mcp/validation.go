package mcp

import (
	"fmt"
	"net/url"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// ParameterValidator provides JSON schema validation for tool parameters
type ParameterValidator struct {
	uriRegex     *regexp.Regexp
	fileExtRegex *regexp.Regexp
}

// ParameterValidationError represents multiple validation errors
type ParameterValidationError struct {
	Errors []ValidationError `json:"errors"`
}

func (e *ParameterValidationError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Message
	}
	var messages []string
	for _, err := range e.Errors {
		messages = append(messages, err.Message)
	}
	return fmt.Sprintf("Multiple validation errors: %s", strings.Join(messages, "; "))
}

// NewParameterValidator creates a new parameter validator
func NewParameterValidator() *ParameterValidator {
	return &ParameterValidator{
		uriRegex:     regexp.MustCompile(`^file://`),
		fileExtRegex: regexp.MustCompile(`\.[a-zA-Z0-9]+$`),
	}
}

// ValidateParameters validates parameters against a JSON schema
func (v *ParameterValidator) ValidateParameters(schema map[string]interface{}, params map[string]interface{}) error {
	var errors []ValidationError

	// Get required fields
	required, _ := schema["required"].([]interface{})
	requiredFields := make(map[string]bool)
	for _, field := range required {
		if fieldName, ok := field.(string); ok {
			requiredFields[fieldName] = true
		}
	}

	// Get properties schema
	properties, _ := schema["properties"].(map[string]interface{})

	// Validate required fields
	for field := range requiredFields {
		if _, exists := params[field]; !exists {
			errors = append(errors, ValidationError{
				Field:    field,
				Value:    nil,
				Expected: "required field",
				Actual:   "missing",
				Message:  fmt.Sprintf("Required field '%s' is missing", field),
			})
		}
	}

	// Validate each parameter
	for field, value := range params {
		if propertySchema, exists := properties[field]; exists {
			if err := v.validateField(field, value, propertySchema.(map[string]interface{})); err != nil {
				errors = append(errors, *err)
			}
		}
	}

	// Validate tool-specific constraints
	if err := v.validateToolConstraints(params); err != nil {
		errors = append(errors, err...)
	}

	if len(errors) > 0 {
		return &ParameterValidationError{Errors: errors}
	}

	return nil
}

// validateField validates a single field against its schema
func (v *ParameterValidator) validateField(field string, value interface{}, schema map[string]interface{}) *ValidationError {
	expectedType, _ := schema["type"].(string)

	// Check type
	actualType := reflect.TypeOf(value).String()
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return &ValidationError{
				Field:    field,
				Value:    value,
				Expected: "string",
				Actual:   actualType,
				Message:  fmt.Sprintf("Field '%s' must be a string, got %s", field, actualType),
			}
		}

		// Validate string constraints
		if err := v.validateStringField(field, value.(string), schema); err != nil {
			return err
		}

	case "integer":
		switch v := value.(type) {
		case int, int32, int64:
			// Valid integer types
		case float64:
			// JSON numbers are float64, check if it's actually an integer
			if v != float64(int64(v)) {
				return &ValidationError{
					Field:    field,
					Value:    value,
					Expected: "integer",
					Actual:   "decimal",
					Message:  fmt.Sprintf("Field '%s' must be an integer, got decimal", field),
				}
			}
		default:
			return &ValidationError{
				Field:    field,
				Value:    value,
				Expected: "integer",
				Actual:   actualType,
				Message:  fmt.Sprintf("Field '%s' must be an integer, got %s", field, actualType),
			}
		}

		// Validate integer constraints
		if err := v.validateIntegerField(field, value, schema); err != nil {
			return err
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			return &ValidationError{
				Field:    field,
				Value:    value,
				Expected: "boolean",
				Actual:   actualType,
				Message:  fmt.Sprintf("Field '%s' must be a boolean, got %s", field, actualType),
			}
		}
	}

	return nil
}

// validateStringField validates string-specific constraints
func (v *ParameterValidator) validateStringField(field, value string, schema map[string]interface{}) *ValidationError {
	// Check minimum length
	if minLength, exists := schema["minLength"]; exists {
		if min, ok := minLength.(float64); ok && len(value) < int(min) {
			return &ValidationError{
				Field:    field,
				Value:    value,
				Expected: fmt.Sprintf("min length %d", int(min)),
				Actual:   fmt.Sprintf("length %d", len(value)),
				Message:  fmt.Sprintf("Field '%s' must be at least %d characters long", field, int(min)),
			}
		}
	}

	// Check maximum length
	if maxLength, exists := schema["maxLength"]; exists {
		if max, ok := maxLength.(float64); ok && len(value) > int(max) {
			return &ValidationError{
				Field:    field,
				Value:    value,
				Expected: fmt.Sprintf("max length %d", int(max)),
				Actual:   fmt.Sprintf("length %d", len(value)),
				Message:  fmt.Sprintf("Field '%s' must be at most %d characters long", field, int(max)),
			}
		}
	}

	// Check pattern
	if pattern, exists := schema["pattern"]; exists {
		if patternStr, ok := pattern.(string); ok {
			if matched, _ := regexp.MatchString(patternStr, value); !matched {
				return &ValidationError{
					Field:    field,
					Value:    value,
					Expected: fmt.Sprintf("pattern %s", patternStr),
					Actual:   value,
					Message:  fmt.Sprintf("Field '%s' does not match required pattern", field),
				}
			}
		}
	}

	return nil
}

// validateIntegerField validates integer-specific constraints
func (v *ParameterValidator) validateIntegerField(field string, value interface{}, schema map[string]interface{}) *ValidationError {
	var intValue int64

	switch v := value.(type) {
	case int:
		intValue = int64(v)
	case int32:
		intValue = int64(v)
	case int64:
		intValue = v
	case float64:
		intValue = int64(v)
	default:
		return &ValidationError{
			Field:    field,
			Value:    value,
			Expected: "integer",
			Actual:   reflect.TypeOf(value).String(),
			Message:  fmt.Sprintf("Field '%s' is not a valid integer", field),
		}
	}

	// Check minimum value
	if minimum, exists := schema["minimum"]; exists {
		if min, ok := minimum.(float64); ok && intValue < int64(min) {
			return &ValidationError{
				Field:    field,
				Value:    value,
				Expected: fmt.Sprintf("minimum %d", int64(min)),
				Actual:   fmt.Sprintf("%d", intValue),
				Message:  fmt.Sprintf("Field '%s' must be at least %d", field, int64(min)),
			}
		}
	}

	// Check maximum value
	if maximum, exists := schema["maximum"]; exists {
		if max, ok := maximum.(float64); ok && intValue > int64(max) {
			return &ValidationError{
				Field:    field,
				Value:    value,
				Expected: fmt.Sprintf("maximum %d", int64(max)),
				Actual:   fmt.Sprintf("%d", intValue),
				Message:  fmt.Sprintf("Field '%s' must be at most %d", field, int64(max)),
			}
		}
	}

	return nil
}

// validateToolConstraints validates tool-specific business logic constraints
func (v *ParameterValidator) validateToolConstraints(params map[string]interface{}) []ValidationError {
	var errors []ValidationError

	// Validate URI format
	if uri, exists := params["uri"]; exists {
		if uriStr, ok := uri.(string); ok {
			if err := v.validateURI(uriStr); err != nil {
				errors = append(errors, ValidationError{
					Field:    "uri",
					Value:    uriStr,
					Expected: "valid file URI",
					Actual:   uriStr,
					Message:  err.Error(),
				})
			}
		}
	}

	// Validate position constraints
	if line, exists := params["line"]; exists {
		if lineNum := v.getIntValue(line); lineNum < 0 {
			errors = append(errors, ValidationError{
				Field:    "line",
				Value:    line,
				Expected: "non-negative integer",
				Actual:   fmt.Sprintf("%v", line),
				Message:  "Line number must be non-negative (0-based)",
			})
		}
	}

	if character, exists := params["character"]; exists {
		if charNum := v.getIntValue(character); charNum < 0 {
			errors = append(errors, ValidationError{
				Field:    "character",
				Value:    character,
				Expected: "non-negative integer",
				Actual:   fmt.Sprintf("%v", character),
				Message:  "Character position must be non-negative (0-based)",
			})
		}
	}

	// Validate query constraints
	if query, exists := params["query"]; exists {
		if queryStr, ok := query.(string); ok {
			if len(strings.TrimSpace(queryStr)) == 0 {
				errors = append(errors, ValidationError{
					Field:    "query",
					Value:    queryStr,
					Expected: "non-empty string",
					Actual:   "empty string",
					Message:  "Search query cannot be empty",
				})
			}
		}
	}

	return errors
}

// validateURI validates file URI format and accessibility
func (v *ParameterValidator) validateURI(uri string) error {
	// Check if it's a file URI
	if !v.uriRegex.MatchString(uri) {
		return fmt.Errorf("URI must be a file:// URI, got: %s", uri)
	}

	// Parse the URI
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("invalid URI format: %v", err)
	}

	// Check if path is absolute
	if !filepath.IsAbs(parsedURI.Path) {
		return fmt.Errorf("URI path must be absolute: %s", parsedURI.Path)
	}

	// Check for dangerous path patterns
	if strings.Contains(parsedURI.Path, "..") {
		return fmt.Errorf("URI path contains dangerous patterns: %s", parsedURI.Path)
	}

	// Validate file extension exists (for most operations)
	if !v.fileExtRegex.MatchString(parsedURI.Path) {
		return fmt.Errorf("URI path should have a file extension: %s", parsedURI.Path)
	}

	return nil
}

// SanitizeParameters sanitizes and normalizes parameters
func (v *ParameterValidator) SanitizeParameters(params map[string]interface{}) (map[string]interface{}, error) {
	sanitized := make(map[string]interface{})

	for key, value := range params {
		switch key {
		case "uri":
			if uriStr, ok := value.(string); ok {
				// Normalize URI path
				parsedURI, err := url.Parse(uriStr)
				if err != nil {
					return nil, fmt.Errorf("failed to parse URI: %v", err)
				}

				// Clean the path
				cleanPath := filepath.Clean(parsedURI.Path)
				parsedURI.Path = cleanPath

				sanitized[key] = parsedURI.String()
			} else {
				sanitized[key] = value
			}

		case "line", "character":
			// Ensure integers are properly typed
			sanitized[key] = v.normalizeInteger(value)

		case "query":
			if queryStr, ok := value.(string); ok {
				// Trim whitespace and normalize
				sanitized[key] = strings.TrimSpace(queryStr)
			} else {
				sanitized[key] = value
			}

		case "includeDeclaration":
			// Ensure boolean is properly typed
			if boolVal, ok := value.(bool); ok {
				sanitized[key] = boolVal
			} else if strVal, ok := value.(string); ok {
				// Try to parse string as boolean
				if parsed, err := strconv.ParseBool(strVal); err == nil {
					sanitized[key] = parsed
				} else {
					sanitized[key] = value
				}
			} else {
				sanitized[key] = value
			}

		default:
			sanitized[key] = value
		}
	}

	return sanitized, nil
}

// Helper functions

func (v *ParameterValidator) getIntValue(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	default:
		return -1
	}
}

func (v *ParameterValidator) normalizeInteger(value interface{}) interface{} {
	switch v := value.(type) {
	case float64:
		// JSON numbers come as float64, convert to int if it's a whole number
		if v == float64(int64(v)) {
			return int64(v)
		}
		return v
	default:
		return value
	}
}
