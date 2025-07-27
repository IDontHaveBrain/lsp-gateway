package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// GetProjectRoot returns the absolute path to the project root directory
// It searches for common project markers like go.mod, package.json, etc.
func GetProjectRoot() (string, error) {
	// Start from the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Try to find project root by looking for common markers
	projectRoot, err := findProjectRootFromPath(wd)
	if err == nil {
		return projectRoot, nil
	}

	// Fallback: try to determine from caller's file location
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return "", fmt.Errorf("unable to determine caller location")
	}

	callerDir := filepath.Dir(filename)
	projectRoot, err = findProjectRootFromPath(callerDir)
	if err != nil {
		return "", fmt.Errorf("failed to find project root from caller location: %w", err)
	}

	return projectRoot, nil
}

// findProjectRootFromPath searches upward from the given path for project markers
func findProjectRootFromPath(startPath string) (string, error) {
	current := startPath
	
	for {
		// Check for common project root markers
		markers := []string{"go.mod", "package.json", ".git", "Makefile", "pyproject.toml", "pom.xml"}
		
		for _, marker := range markers {
			markerPath := filepath.Join(current, marker)
			if _, err := os.Stat(markerPath); err == nil {
				return current, nil
			}
		}

		// Move up one directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			break
		}
		current = parent
	}

	return "", fmt.Errorf("project root not found from path: %s", startPath)
}

// GetTestDataDir returns the path to the test data directory
func GetTestDataDir() (string, error) {
	root, err := GetProjectRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "tests", "testdata"), nil
}

// GetFixturesDir returns the path to the test fixtures directory
func GetFixturesDir() (string, error) {
	root, err := GetProjectRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "tests", "fixtures"), nil
}

// GetE2EFixturesDir returns the path to the E2E test fixtures directory
func GetE2EFixturesDir() (string, error) {
	root, err := GetProjectRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "tests", "e2e", "fixtures"), nil
}