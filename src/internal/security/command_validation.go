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
		"typescript-language-server": true,
		"tsserver":                   true,
		"java":                       true,
		"java.exe":                   true,
		"node":                       true,
		"node.exe":                   true,
		"python":                     true,
		"python.exe":                 true,
		"python3":                    true,
		"python3.exe":                true,
		"pylsp":                      true,
		"jdtls":                      true,
		"jdtls.bat":                  true,
		"jdtls.py":                   true,
		// Installation tools
		"go":        true,
		"go.exe":    true,
		"npm":       true,
		"npm.cmd":   true,
		"npx":       true,
		"npx.cmd":   true,
		"pip":       true,
		"pip.exe":   true,
		"pip3":      true,
		"pip3.exe":  true,
		"curl":      true,
		"curl.exe":  true,
		"wget":      true,
		"wget.exe":  true,
		"tar":       true,
		"tar.exe":   true,
		"unzip":     true,
		"unzip.exe": true,
		"apt-get":   true,
		"brew":      true,
		"echo":      true,
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
