package cli

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/transport"
)

type ValidationError struct {
	Field       string
	Value       interface{}
	Message     string
	Suggestions []string
}

func (v *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for %s: %s", v.Field, v.Message)
}

func ValidatePort(port int, fieldName string) *ValidationError {
	if port < 1 || port > 65535 {
		return &ValidationError{
			Field:   fieldName,
			Value:   port,
			Message: fmt.Sprintf("port %d is out of valid range", port),
			Suggestions: []string{
				"Use a port between 1 and 65535",
				"Common ports: 8080, 3000, 9090",
				"Check available ports with: netstat -tuln",
			},
		}
	}

	if port < 1024 {
		return &ValidationError{
			Field:   fieldName,
			Value:   port,
			Message: fmt.Sprintf("port %d is privileged (requires sudo)", port),
			Suggestions: []string{
				"Use a non-privileged port (>= 1024): 8080, 3000, 9090",
				"Run with sudo if privileged port is required",
				"Most applications work fine with ports 8080+",
			},
		}
	}

	return nil
}

func ValidatePortAvailability(port int, fieldName string) *ValidationError {
	if err := ValidatePort(port, fieldName); err != nil {
		return err
	}

	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			return &ValidationError{
				Field:   fieldName,
				Value:   port,
				Message: fmt.Sprintf("port %d is already in use", port),
				Suggestions: []string{
					fmt.Sprintf("Try a different port: %d, %d, %d", port+1, port+10, port+100),
					fmt.Sprintf("Check what's using the port: lsof -i :%d", port),
					fmt.Sprintf("Kill process using port: sudo kill $(lsof -t -i:%d)", port),
					"Wait a moment as port may be in TIME_WAIT state",
				},
			}
		}
		return &ValidationError{
			Field:   fieldName,
			Value:   port,
			Message: fmt.Sprintf("cannot bind to port %d: %v", port, err),
			Suggestions: []string{
				"Check if you have permission to bind to this port",
				"Try a different port number",
				"Ensure no firewall is blocking the port",
			},
		}
	}

	if err := listener.Close(); err != nil {
		// Ignore listener close errors during validation
		_ = err
	}
	return nil
}

func ValidateFilePath(path, fieldName, operation string) *ValidationError {
	if path == "" {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: "file path cannot be empty",
			Suggestions: []string{
				"Provide a valid file path",
				"Use absolute paths for clarity: /path/to/file",
				"Use relative paths from current directory: ./file",
			},
		}
	}

	if !filepath.IsAbs(path) && !strings.HasPrefix(path, "./") && !strings.HasPrefix(path, "../") {
		if !strings.Contains(path, "/") && !strings.Contains(path, "\\") {
		} else {
			return &ValidationError{
				Field:   fieldName,
				Value:   path,
				Message: "path format is ambiguous",
				Suggestions: []string{
					"Use absolute paths: /full/path/to/file",
					"Use explicit relative paths: ./relative/path",
					"Use simple filename for current directory: filename.ext",
				},
			}
		}
	}

	switch operation {
	case "read":
		return validateFileReadable(path, fieldName)
	case "write":
		return validateFileWritable(path, fieldName)
	case "create":
		return validateFileCreatable(path, fieldName)
	default:
		return validateFileExists(path, fieldName)
	}
}

func validateFileExists(path, fieldName string) *ValidationError {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("file does not exist: %s", path),
			Suggestions: []string{
				"Check if the file path is correct",
				"Use absolute path to avoid confusion",
				"Create the file if it should exist",
				fmt.Sprintf(SUGGESTION_LIST_DIRECTORY, filepath.Dir(path)),
			},
		}
	}
	return nil
}

func validateFileReadable(path, fieldName string) *ValidationError {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("file does not exist: %s", path),
			Suggestions: []string{
				"Check if the file path is correct",
				"Create the file if it should exist",
				fmt.Sprintf(SUGGESTION_LIST_DIRECTORY, filepath.Dir(path)),
			},
		}
	}

	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("cannot access file: %s (%v)", path, err),
			Suggestions: []string{
				fmt.Sprintf("Check file permissions: ls -la %s", path),
				"Ensure you have read access to the file",
				"Check if parent directories are accessible",
			},
		}
	}

	if info.IsDir() {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("path is a directory, not a file: %s", path),
			Suggestions: []string{
				"Specify a file path, not a directory",
				"Add filename to the directory path",
				fmt.Sprintf(SUGGESTION_LIST_DIRECTORY, path),
			},
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("cannot read file: %s (%v)", path, err),
			Suggestions: []string{
				fmt.Sprintf("Check file permissions: ls -la %s", path),
				"Ensure you have read access to the file",
				fmt.Sprintf(SUGGESTION_FIX_PERMISSIONS, path),
			},
		}
	}
	if err := file.Close(); err != nil {
		// Ignore file close errors during validation
		_ = err
	}

	return nil
}

func validateFileWritable(path, fieldName string) *ValidationError {
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return &ValidationError{
				Field:   fieldName,
				Value:   path,
				Message: fmt.Sprintf("path is a directory, not a file: %s", path),
				Suggestions: []string{
					"Specify a file path, not a directory",
					"Add filename to the directory path",
				},
			}
		}

		file, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return &ValidationError{
				Field:   fieldName,
				Value:   path,
				Message: fmt.Sprintf("cannot write to file: %s (%v)", path, err),
				Suggestions: []string{
					fmt.Sprintf("Check file permissions: ls -la %s", path),
					fmt.Sprintf(SUGGESTION_FIX_PERMISSIONS, path),
					"Ensure you have write access to the file",
				},
			}
		}
		if err := file.Close(); err != nil {
			// Ignore file close errors during validation
			_ = err
		}
		return nil
	}

	return validateFileCreatable(path, fieldName)
}

func validateFileCreatable(path, fieldName string) *ValidationError {
	dir := filepath.Dir(path)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("parent directory does not exist: %s", dir),
			Suggestions: []string{
				fmt.Sprintf("Create parent directory: mkdir -p %s", dir),
				"Check if the path is correct",
				"Use an existing directory",
			},
		}
	}

	tempFile := filepath.Join(dir, ".lsp-gateway-test-write")
	file, err := os.Create(tempFile)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("cannot create file in directory: %s (%v)", dir, err),
			Suggestions: []string{
				fmt.Sprintf("Check directory permissions: ls -la %s", dir),
				fmt.Sprintf("Fix permissions: chmod 755 %s", dir),
				"Ensure you have write access to the directory",
			},
		}
	}
	if err := file.Close(); err != nil {
		// Ignore file close errors during validation
		_ = err
	}
	if err := os.Remove(tempFile); err != nil {
		// Ignore temp file removal errors during validation
		_ = err
	}

	return nil
}

func ValidateTransport(transportType, fieldName string) *ValidationError {
	if transportType == "" {
		return &ValidationError{
			Field:   fieldName,
			Value:   transportType,
			Message: "transport type cannot be empty",
			Suggestions: []string{
				"Use 'stdio' for standard input/output transport",
				"Use 'tcp' for TCP transport",
				"Use 'http' for HTTP transport",
			},
		}
	}

	validTransports := map[string]bool{
		transport.TransportStdio: true,
		transport.TransportTCP:   true,
		transport.TransportHTTP:  true,
	}

	if !validTransports[transportType] {
		validList := []string{transport.TransportStdio, transport.TransportTCP, transport.TransportHTTP}
		return &ValidationError{
			Field:   fieldName,
			Value:   transportType,
			Message: fmt.Sprintf("invalid transport type: %s", transportType),
			Suggestions: []string{
				fmt.Sprintf("Valid transport types: %s", strings.Join(validList, ", ")),
				"Use 'stdio' for most language servers",
				"Use 'tcp' for network-based servers",
			},
		}
	}

	return nil
}

func ValidateURL(urlStr, fieldName string) *ValidationError {
	if urlStr == "" {
		return &ValidationError{
			Field:   fieldName,
			Value:   urlStr,
			Message: "URL cannot be empty",
			Suggestions: []string{
				"Provide a valid URL (e.g., http://localhost:8080)",
				"Use http:// or https:// prefix",
				"Check if the URL format is correct",
			},
		}
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   urlStr,
			Message: fmt.Sprintf("invalid URL format: %v", err),
			Suggestions: []string{
				"Use proper URL format: http://host:port/path",
				"Examples: http://localhost:8080, https://api.example.com",
				"Ensure protocol (http/https) is specified",
			},
		}
	}

	if parsedURL.Scheme == "" {
		return &ValidationError{
			Field:   fieldName,
			Value:   urlStr,
			Message: "URL must include scheme (http:// or https://)",
			Suggestions: []string{
				fmt.Sprintf("Add scheme: http://%s", urlStr),
				fmt.Sprintf("Or use HTTPS: https://%s", urlStr),
				"Valid schemes: http, https",
			},
		}
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &ValidationError{
			Field:   fieldName,
			Value:   urlStr,
			Message: fmt.Sprintf("unsupported URL scheme: %s", parsedURL.Scheme),
			Suggestions: []string{
				"Use 'http' or 'https' scheme",
				fmt.Sprintf("Try: http://%s", parsedURL.Host),
				fmt.Sprintf("Or: https://%s", parsedURL.Host),
			},
		}
	}

	if parsedURL.Host == "" {
		return &ValidationError{
			Field:   fieldName,
			Value:   urlStr,
			Message: "URL must include host",
			Suggestions: []string{
				"Include hostname: http://localhost:8080",
				"Use IP address: http://127.0.0.1:8080",
				"Specify domain: http://api.example.com",
			},
		}
	}

	return nil
}

func ValidateTimeout(timeout time.Duration, fieldName string) *ValidationError {
	if timeout < 0 {
		return &ValidationError{
			Field:   fieldName,
			Value:   timeout,
			Message: "timeout cannot be negative",
			Suggestions: []string{
				"Use positive duration: 30s, 1m, 5m",
				"Use 0 for no timeout (not recommended)",
				"Common timeouts: 10s, 30s, 60s",
			},
		}
	}

	if timeout > 0 && timeout < time.Second {
		return &ValidationError{
			Field:   fieldName,
			Value:   timeout,
			Message: "timeout is too short (less than 1 second)",
			Suggestions: []string{
				"Use at least 1 second: 1s, 5s, 10s",
				"Consider network latency in timeout values",
				"Recommended minimums: 5s for local, 30s for network",
			},
		}
	}

	if timeout > 10*time.Minute {
		return &ValidationError{
			Field:   fieldName,
			Value:   timeout,
			Message: "timeout is very long (over 10 minutes)",
			Suggestions: []string{
				"Consider shorter timeout: 30s, 1m, 5m",
				"Very long timeouts may cause resource issues",
				"Use reasonable timeouts based on expected response time",
			},
		}
	}

	return nil
}

func ValidateIntRange(value, min, max int, fieldName string) *ValidationError {
	if value < min || value > max {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Message: fmt.Sprintf("value %d is outside valid range [%d, %d]", value, min, max),
			Suggestions: []string{
				fmt.Sprintf("Use a value between %d and %d", min, max),
				fmt.Sprintf("Minimum value: %d", min),
				fmt.Sprintf("Maximum value: %d", max),
			},
		}
	}
	return nil
}

func ValidateRuntimeName(runtime, fieldName string) *ValidationError {
	if runtime == "" {
		return nil // Empty is acceptable for auto-detection
	}

	supportedRuntimes := []string{"go", "python", "nodejs", "java"}
	for _, supported := range supportedRuntimes {
		if runtime == supported {
			return nil
		}
	}

	return &ValidationError{
		Field:   fieldName,
		Value:   runtime,
		Message: fmt.Sprintf("unsupported runtime: %s", runtime),
		Suggestions: []string{
			fmt.Sprintf("Supported runtimes: %s", strings.Join(supportedRuntimes, ", ")),
			"Use empty string for auto-detection",
			"Check spelling of runtime name",
		},
	}
}

func ToValidationError(err *ValidationError) *CLIError {
	if err == nil {
		return nil
	}

	return &CLIError{
		Type:        ErrorTypeValidation,
		Message:     err.Error(),
		Suggestions: err.Suggestions,
		RelatedCmds: []string{
			"config validate",
			"diagnose",
			"help",
		},
	}
}

func ValidateMultiple(validators ...func() *ValidationError) *CLIError {
	for _, validator := range validators {
		if err := validator(); err != nil {
			return ToValidationError(err)
		}
	}
	return nil
}

func ValidateProjectPath(path, fieldName string) *ValidationError {
	if path == "" {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: "project path cannot be empty",
			Suggestions: []string{
				"Provide a valid project directory path",
				"Use absolute paths for clarity: /path/to/project",
				"Use relative paths from current directory: ./project",
				"Use current directory: .",
			},
		}
	}

	// Normalize the path
	normalizedPath, err := NormalizeProjectPath(path)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("cannot normalize project path: %v", err),
			Suggestions: []string{
				"Check if the path format is correct",
				"Use absolute paths: /full/path/to/project",
				"Use explicit relative paths: ./relative/path",
			},
		}
	}

	// Check if directory exists
	info, err := os.Stat(normalizedPath)
	if os.IsNotExist(err) {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("project directory does not exist: %s", normalizedPath),
			Suggestions: []string{
				"Check if the project path is correct",
				"Create the project directory if needed",
				fmt.Sprintf(SUGGESTION_LIST_DIRECTORY, filepath.Dir(normalizedPath)),
				"Use an existing project directory",
			},
		}
	}

	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("cannot access project directory: %s (%v)", normalizedPath, err),
			Suggestions: []string{
				fmt.Sprintf(SUGGESTION_CHECK_PERMISSIONS, normalizedPath),
				"Ensure you have read access to the directory",
				"Check if parent directories are accessible",
			},
		}
	}

	// Check if it's actually a directory
	if !info.IsDir() {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("path is not a directory: %s", normalizedPath),
			Suggestions: []string{
				"Specify a directory path, not a file",
				"Remove filename from the path",
				fmt.Sprintf(SUGGESTION_LIST_DIRECTORY, filepath.Dir(normalizedPath)),
			},
		}
	}

	// Check if directory is readable
	if err := validateDirectoryReadable(normalizedPath, fieldName); err != nil {
		return err
	}

	return nil
}

func ValidateProjectStructure(path, fieldName string) *ValidationError {
	if err := ValidateProjectPath(path, fieldName); err != nil {
		return err
	}

	normalizedPath, err := NormalizeProjectPath(path)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("cannot normalize project path: %v", err),
			Suggestions: []string{
				"Check if the path format is correct",
				"Use absolute paths for clarity",
			},
		}
	}

	// Check for common project markers
	projectMarkers := []string{
		"go.mod",           // Go projects
		"package.json",     // Node.js/JavaScript projects
		"pyproject.toml",   // Python projects
		"setup.py",         // Python projects
		"requirements.txt", // Python projects
		"Cargo.toml",       // Rust projects
		"pom.xml",          // Java Maven projects
		"build.gradle",     // Java Gradle projects
		".git",             // Git repository
		"Makefile",         // Make-based projects
		"CMakeLists.txt",   // CMake projects
	}

	hasProjectStructure := false
	foundMarkers := []string{}

	for _, marker := range projectMarkers {
		markerPath := filepath.Join(normalizedPath, marker)
		if _, err := os.Stat(markerPath); err == nil {
			hasProjectStructure = true
			foundMarkers = append(foundMarkers, marker)
		}
	}

	if !hasProjectStructure {
		message := fmt.Sprintf("directory does not appear to be a project workspace: %s", normalizedPath)
		if len(foundMarkers) > 0 {
			message = fmt.Sprintf("directory contains some project markers (%v) but may not be a complete workspace: %s",
				foundMarkers, normalizedPath)
		}
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: message,
			Suggestions: []string{
				"Ensure the directory contains project files (go.mod, package.json, etc.)",
				"Initialize a project in this directory if needed",
				"Use a directory that contains a recognized project structure",
				fmt.Sprintf(SUGGESTION_LIST_DIRECTORY, normalizedPath),
			},
		}
	}

	return nil
}

func NormalizeProjectPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Handle special cases
	if path == "." {
		return filepath.Abs(".")
	}

	// Clean the path to remove redundant elements
	cleanPath := filepath.Clean(path)

	// Convert to absolute path if relative
	if !filepath.IsAbs(cleanPath) {
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return "", fmt.Errorf("cannot resolve absolute path: %w", err)
		}
		return absPath, nil
	}

	return cleanPath, nil
}

func validateDirectoryReadable(path, fieldName string) *ValidationError {
	// Try to read directory contents
	entries, err := os.ReadDir(path)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   path,
			Message: fmt.Sprintf("cannot read directory contents: %s (%v)", path, err),
			Suggestions: []string{
				fmt.Sprintf(SUGGESTION_CHECK_PERMISSIONS, path),
				"Ensure you have read access to the directory",
				fmt.Sprintf("Fix permissions: chmod 755 %s", path),
			},
		}
	}

	// Basic sanity check - directory should be accessible
	_ = entries // We just need to ensure we can read it

	return nil
}
