package testutils

import (
	"fmt"
	"os"
)

// CreateTempConfig creates a temporary YAML configuration file for testing
func CreateTempConfig(content string) (string, func(), error) {
	tempFile, err := os.CreateTemp("", "test_config_*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp config file: %w", err)
	}

	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", nil, fmt.Errorf("failed to write config content: %w", err)
	}

	tempFile.Close()

	cleanup := func() {
		os.Remove(tempFile.Name())
	}

	return tempFile.Name(), cleanup, nil
}

// GetTestConfig returns a basic test configuration for the simplified architecture
func GetTestConfig() string {
	return `servers:
  go:
    command: "gopls"
    args: []
    working_dir: ""
    initialization_options: {}
`
}

// GetMultiLanguageTestConfig returns test configuration for all 4 supported languages
func GetMultiLanguageTestConfig() string {
	return `servers:
  go:
    command: "gopls"
    args: []
    working_dir: ""
    initialization_options: {}
  python:
    command: "pylsp"
    args: []
    working_dir: ""
    initialization_options: {}
  javascript:
    command: "typescript-language-server"
    args: ["--stdio"]
    working_dir: ""
    initialization_options: {}
  java:
    command: "jdtls"
    args: []
    working_dir: ""
    initialization_options: {}
`
}
