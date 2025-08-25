// Package security provides command validation and safety checks.
package security

import (
	"fmt"
	"lsp-gateway/src/internal/registry"
	"os"
	"path/filepath"
	"strings"
)

func ValidateCommand(command string, args []string) error {
	// Convert registry commands slice to map for efficient lookup
	allowedCommands := make(map[string]bool)
	for _, cmd := range registry.GetAllowedCommands() {
		allowedCommands[cmd] = true
	}

	// In test environments, allow a small set of benign utilities used by tests
	if os.Getenv("ALLOW_TEST_COMMANDS") == "1" || os.Getenv("GO_TEST") == "true" {
		for _, extra := range []string{"sleep", "sh", "bash"} {
			allowedCommands[extra] = true
			// also allow .exe no-op mapping via nameNoExt logic below
		}
	}

	baseName := filepath.Base(command)
	// Also allow extensionless comparison (e.g., gopls vs gopls.exe)
	nameNoExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	// Allow Windows cmd wrapper only for executing whitelisted scripts
	skipBaseCheck := false
	lowerBase := strings.ToLower(baseName)
	if lowerBase == "cmd.exe" || lowerBase == "cmd" {
		if len(args) < 2 {
			return fmt.Errorf("invalid cmd wrapper usage: missing /c and target script")
		}
		first := strings.ToLower(args[0])
		if first != "/c" {
			return fmt.Errorf("invalid cmd wrapper usage: first arg must be /c")
		}
		script := filepath.Base(args[1])
		scriptNoExt := strings.TrimSuffix(script, filepath.Ext(script))
		if !allowedCommands[script] && !allowedCommands[scriptNoExt] {
			return fmt.Errorf("command not in whitelist: %s", script)
		}
		skipBaseCheck = true
	}

	if !skipBaseCheck && !allowedCommands[baseName] && !allowedCommands[nameNoExt] {
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
