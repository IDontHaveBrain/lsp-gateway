package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ValidateCommand(command string, args []string) error {
	allowedCommands := map[string]bool{
		"gopls":                      true,
		"pyls":                       true,
		"pyright-langserver":         true,
		"pyright":                    true,
		"typescript-language-server": true,
		"tsserver":                   true,
		"java":                       true,
		"node":                       true,
		"python":                     true,
		"python3":                    true,
		"pylsp":                      true,
		"jdtls":                      true,
		"jdtls.py":                   true,
		// Installation tools
		"go":      true,
		"npm":     true,
		"npx":     true,
		"pip":     true,
		"pip3":    true,
		"curl":    true,
		"wget":    true,
		"tar":     true,
		"unzip":   true,
		"apt-get": true,
		"brew":    true,
		"echo":    true,
	}

	baseName := filepath.Base(command)

	if !allowedCommands[baseName] {
		for allowedCmd := range allowedCommands {
			if strings.HasSuffix(baseName, allowedCmd) {
				goto cmdAllowed
			}
		}
		return fmt.Errorf("command not in whitelist: %s", baseName)
	}

cmdAllowed:
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

func ValidateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal not allowed: %s", path)
	}

	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal detected after cleaning: %s", cleanPath)
	}

	return nil
}
