package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateAndGetWorkingDir validates and normalizes a working directory path.
// If workingDir is empty, it returns the current working directory.
// If the current working directory cannot be determined, it falls back to /tmp.
func ValidateAndGetWorkingDir(workingDir string) (string, error) {
	// If a specific working directory is provided, validate and normalize it
	if workingDir != "" {
		// Expand ~ if present
		expandedPath, err := ExpandPath(workingDir)
		if err != nil {
			return "", fmt.Errorf("failed to expand working directory path '%s': %w", workingDir, err)
		}

		// Convert to absolute path
		absPath, err := filepath.Abs(expandedPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path for working directory '%s': %w", expandedPath, err)
		}

		// Check if directory exists
		if info, err := os.Stat(absPath); err != nil {
			return "", fmt.Errorf("working directory '%s' does not exist: %w", absPath, err)
		} else if !info.IsDir() {
			return "", fmt.Errorf("working directory '%s' is not a directory", absPath)
		}

		return absPath, nil
	}

	// Use current working directory as fallback
	if wd, err := os.Getwd(); err == nil {
		return wd, nil
	}

	// Final fallback to /tmp if current directory cannot be determined
	return "/tmp", nil
}

// GetLSPToolPath constructs the standard LSP tool installation path.
// Returns a path in the format: ~/.lsp-gateway/tools/{language}/bin/{tool}
func GetLSPToolPath(language, tool string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory cannot be determined
		return filepath.Join(".", ".lsp-gateway", "tools", language, "bin", tool)
	}

	return filepath.Join(homeDir, ".lsp-gateway", "tools", language, "bin", tool)
}

// GetLSPToolRoot returns the base installation directory for a language
// Format: ~/.lsp-gateway/tools/{language}
func GetLSPToolRoot(language string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".lsp-gateway", "tools", language)
	}
	return filepath.Join(homeDir, ".lsp-gateway", "tools", language)
}

// ExpandPath expands ~ to the user's home directory in file paths.
// This function was moved from src/config/config.go for centralized path handling.
func ExpandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path, fmt.Errorf("failed to get user home directory: %w", err)
	}

	if path == "~" {
		return homeDir, nil
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:]), nil
	}

	return path, nil
}
