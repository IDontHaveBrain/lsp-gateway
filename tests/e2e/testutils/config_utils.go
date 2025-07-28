package testutils

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TempConfigOptions provides options for creating temporary configuration files
type TempConfigOptions struct {
	// Content is the raw configuration content
	Content string
	// Template allows using predefined templates
	Template string
	// Variables for template substitution
	Variables map[string]string
	// FilePrefix for the temporary file
	FilePrefix string
	// FileSuffix for the temporary file (e.g., ".yaml", ".json")
	FileSuffix string
	// Directory to create the temp file in (defaults to system temp)
	Directory string
}

// CreateTempConfig creates a temporary configuration file with the specified content
// Returns the path to the created file and cleanup function
func CreateTempConfig(content string) (string, func(), error) {
	return CreateTempConfigWithOptions(TempConfigOptions{
		Content:    content,
		FilePrefix: "test_config_",
		FileSuffix: ".yaml",
	})
}

// CreateTempConfigWithOptions creates a temporary configuration file with advanced options
func CreateTempConfigWithOptions(opts TempConfigOptions) (string, func(), error) {
	// Set defaults
	if opts.FilePrefix == "" {
		opts.FilePrefix = "test_config_"
	}
	if opts.FileSuffix == "" {
		opts.FileSuffix = ".yaml"
	}
	if opts.Directory == "" {
		opts.Directory = os.TempDir()
	}

	// Process content based on template or direct content
	content := opts.Content
	if opts.Template != "" {
		var err error
		content, err = processTemplate(opts.Template, opts.Variables)
		if err != nil {
			return "", nil, fmt.Errorf("failed to process template: %w", err)
		}
	} else if opts.Variables != nil {
		content = substituteVariables(content, opts.Variables)
	}

	// Create temporary file
	tempFile, err := ioutil.TempFile(opts.Directory, opts.FilePrefix+"*"+opts.FileSuffix)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Write content to file
	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", nil, fmt.Errorf("failed to write config content: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return "", nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Create cleanup function
	cleanup := func() {
		os.Remove(tempFile.Name())
	}

	return tempFile.Name(), cleanup, nil
}

// CreateTempConfigInDir creates a temporary config file in a specific directory
func CreateTempConfigInDir(content, dir string) (string, func(), error) {
	return CreateTempConfigWithOptions(TempConfigOptions{
		Content:    content,
		Directory:  dir,
		FilePrefix: "test_config_",
		FileSuffix: ".yaml",
	})
}

// CreateTempJSONConfig creates a temporary JSON configuration file
func CreateTempJSONConfig(content string) (string, func(), error) {
	return CreateTempConfigWithOptions(TempConfigOptions{
		Content:    content,
		FilePrefix: "test_config_",
		FileSuffix: ".json",
	})
}

// processTemplate processes predefined configuration templates
func processTemplate(templateName string, variables map[string]string) (string, error) {
	// Get project root to locate template files
	projectRoot, err := GetProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to get project root: %w", err)
	}

	// Common template locations
	templatePaths := []string{
		filepath.Join(projectRoot, "config-templates", templateName+".yaml"),
		filepath.Join(projectRoot, "tests", "fixtures", templateName+".yaml"),
		filepath.Join(projectRoot, "tests", "e2e", "fixtures", templateName+".yaml"),
	}

	var templateContent string
	for _, path := range templatePaths {
		if content, err := ioutil.ReadFile(path); err == nil {
			templateContent = string(content)
			break
		}
	}

	if templateContent == "" {
		return "", fmt.Errorf("template not found: %s", templateName)
	}

	// Substitute variables
	return substituteVariables(templateContent, variables), nil
}

// substituteVariables replaces variables in the format ${VAR_NAME} or {{VAR_NAME}}
func substituteVariables(content string, variables map[string]string) string {
	if variables == nil {
		return content
	}

	result := content
	for key, value := range variables {
		// Support both ${VAR} and {{VAR}} formats
		result = strings.ReplaceAll(result, "${"+key+"}", value)
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	return result
}

// GetDefaultTestConfig returns a basic configuration for testing
func GetDefaultTestConfig() string {
	return `
servers:
  - name: "test-server"
    language: "go"
    command: ["gopls"]
    transport: "stdio"
    
performance:
  cache:
    enabled: true
    memory_limit: "100MB"
    
logging:
  level: "debug"
  
mcp:
  enabled: true
  tools:
    - "textDocument/definition"
    - "textDocument/references"
    - "textDocument/hover"
`
}

// GetMinimalTestConfig returns a minimal configuration for basic testing
func GetMinimalTestConfig() string {
	return `
servers:
  - name: "minimal-server"
    language: "go"
    command: ["gopls"]
    transport: "stdio"
`
}

// DetectNodePath dynamically detects the global Node.js modules path
// Returns the path to global node_modules directory for NODE_PATH environment variable
func DetectNodePath() string {
	// First, try to use npm root -g to get the global node_modules path
	if cmd := exec.Command("npm", "root", "-g"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			path := strings.TrimSpace(string(output))
			if path != "" && isValidNodePath(path) {
				return path
			}
		}
	}

	// Fallback to common global node_modules paths
	commonPaths := []string{
		"/usr/local/lib/node_modules",
		"/usr/lib/node_modules", 
		"/opt/homebrew/lib/node_modules",  // macOS Homebrew on Apple Silicon
		"/home/linuxbrew/.linuxbrew/lib/node_modules", // Linux Homebrew
		filepath.Join(os.Getenv("HOME"), ".npm-global", "lib", "node_modules"), // npm config prefix
	}

	// Add paths based on NODE_PREFIX if set
	if nodePrefix := os.Getenv("NODE_PREFIX"); nodePrefix != "" {
		commonPaths = append([]string{filepath.Join(nodePrefix, "lib", "node_modules")}, commonPaths...)
	}

	// Add paths based on npm prefix config
	if cmd := exec.Command("npm", "config", "get", "prefix"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			prefix := strings.TrimSpace(string(output))
			if prefix != "" && prefix != "undefined" {
				commonPaths = append([]string{filepath.Join(prefix, "lib", "node_modules")}, commonPaths...)
			}
		}
	}

	// Return the first valid path found
	for _, path := range commonPaths {
		if isValidNodePath(path) {
			return path
		}
	}

	// Final fallback - return the most common default path
	return "/usr/local/lib/node_modules"
}

// isValidNodePath checks if a path exists and is a valid node_modules directory
func isValidNodePath(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}