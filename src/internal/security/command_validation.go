// Package security provides command validation and safety checks.
package security

import (
	"fmt"
	"lsp-gateway/src/internal/registry"
	"path/filepath"
	"strings"
)

func ValidateCommand(command string, args []string) error {
	// Convert registry commands slice to map for efficient lookup
	allowedCommands := make(map[string]bool)
	for _, cmd := range registry.GetAllowedCommands() {
		allowedCommands[cmd] = true
	}

	baseName := filepath.Base(command)
	// Also allow extensionless comparison (e.g., gopls vs gopls.exe)
	nameNoExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	if !allowedCommands[baseName] && !allowedCommands[nameNoExt] {
		return fmt.Errorf("command not in whitelist: %s", baseName)
	}

	for _, arg := range args {
		if strings.Contains(arg, "..") {
			return fmt.Errorf("path traversal detected in argument: %s", arg)
		}

		if strings.Contains(arg, "|") || strings.Contains(arg, "&") ||
			strings.Contains(arg, ";") || strings.Contains(arg, "`") ||
			strings.Contains(arg, "$") || strings.Contains(arg, "$(") {
			return fmt.Errorf("shell injection detected in argument: %s", arg)
		}
	}

	return nil
}
