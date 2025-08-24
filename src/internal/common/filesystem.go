package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafeReadFile wraps os.ReadFile with consistent error handling
func SafeReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return data, nil
}

func ResolveUnder(base, p string) (string, error) {
	if base == "" {
		wd, _ := os.Getwd()
		base = wd
	}
	ab, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("invalid base: %w", err)
	}
	var tp string
	if filepath.IsAbs(p) {
		tp = filepath.Clean(p)
	} else {
		tp = filepath.Clean(filepath.Join(ab, p))
	}
	rel, err := filepath.Rel(ab, tp)
	if err != nil {
		return "", fmt.Errorf("path resolution failed: %w", err)
	}
	if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes base directory")
	}
	return tp, nil
}

func SafeReadFileUnder(base, p string) ([]byte, error) {
	tp, err := ResolveUnder(base, p)
	if err != nil {
		return nil, err
	}
	return SafeReadFile(tp)
}

func CreateFileUnder(base, p string, perm os.FileMode) (*os.File, error) {
	tp, err := ResolveUnder(base, p)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(tp), 0o750); err != nil && filepath.Dir(tp) != "." {
		return nil, err
	}
	f, err := os.OpenFile(tp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return nil, err
	}
	return f, nil
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
