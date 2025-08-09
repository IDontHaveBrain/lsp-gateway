package common

import "fmt"

// ValidateParamMap validates that params is not nil and converts it to a map[string]interface{}
func ValidateParamMap(params interface{}) (map[string]interface{}, error) {
	if params == nil {
		return nil, fmt.Errorf("no parameters provided")
	}

	paramMap, ok := params.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("params is not a map")
	}

	return paramMap, nil
}

// WrapProcessingError wraps an error with operation context for better error messages
func WrapProcessingError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

// ParameterValidationError creates a formatted parameter validation error
func ParameterValidationError(msg string) error {
	return fmt.Errorf("parameter validation error: %s", msg)
}

// NoParametersError returns a standardized "no parameters provided" error
func NoParametersError() error {
	return fmt.Errorf("no parameters provided")
}
