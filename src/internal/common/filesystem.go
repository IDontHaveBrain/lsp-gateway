package common

import (
	"encoding/json"
	"fmt"
	"os"
)

// SafeReadFile wraps os.ReadFile with consistent error handling
func SafeReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return data, nil
}

// FileExists checks if a file exists using os.Stat, returns false if any error occurs
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReadJSONFile reads and unmarshals a JSON file in one operation
func ReadJSONFile(path string, v interface{}) error {
	data, err := SafeReadFile(path)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON from %s: %w", path, err)
	}

	return nil
}
