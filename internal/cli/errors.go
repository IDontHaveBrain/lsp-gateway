package cli

import (
	"fmt"
	"os"
	"strings"
)

type ErrorType int

const (
	ErrorTypeConfig ErrorType = iota
	ErrorTypeInstallation
	ErrorTypeNetwork
	ErrorTypePermission
	ErrorTypeRuntime
	ErrorTypeServer
	ErrorTypeValidation
	ErrorTypeGeneral
)

type CLIError struct {
	Type        ErrorType
	Message     string
	Cause       error
	Suggestions []string
	RelatedCmds []string
}

func (e *CLIError) Error() string {
	if e == nil {
		return "❌ Error: unknown error (nil CLIError)"
	}

	var parts []string

	switch e.Type {
	case ErrorTypeConfig:
		parts = append(parts, "⚙️  Configuration Error:")
	case ErrorTypeInstallation:
		parts = append(parts, "📦 Installation Error:")
	case ErrorTypeNetwork:
		parts = append(parts, "🌐 Network Error:")
	case ErrorTypePermission:
		parts = append(parts, "🔒 Permission Error:")
	case ErrorTypeRuntime:
		parts = append(parts, "⚡ Runtime Error:")
	case ErrorTypeServer:
		parts = append(parts, "🖥️  Server Error:")
	case ErrorTypeValidation:
		parts = append(parts, "✅ Validation Error:")
	default:
		parts = append(parts, "❌ Error:")
	}

	message := e.Message
	if message == "" {
		message = "unknown error"
	}
	parts = append(parts, message)

	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("\n   Cause: %v", e.Cause))
	}

	if len(e.Suggestions) > 0 {
		parts = append(parts, "\n\n💡 Try these solutions:")
		for i, suggestion := range e.Suggestions {
			parts = append(parts, fmt.Sprintf("   %d. %s", i+1, suggestion))
		}
	}

	if len(e.RelatedCmds) > 0 {
		parts = append(parts, "\n\n🔗 Related commands:")
		for _, cmd := range e.RelatedCmds {
			parts = append(parts, fmt.Sprintf("   lspg %s", cmd))
		}
	}

	return strings.Join(parts, " ")
}

func NewConfigError(message string, cause error) *CLIError {
	return &CLIError{
		Type:    ErrorTypeConfig,
		Message: message,
		Cause:   cause,
		Suggestions: []string{
			"Check if config file exists with: ls -la config.yaml",
			"Generate a new config with: lspg config generate",
			"Validate existing config with: lspg config validate",
			"Run complete setup: lspg setup all",
		},
		RelatedCmds: []string{
			"config generate",
			"config validate",
			"setup all",
			"diagnose",
		},
	}
}

func NewConfigNotFoundError(configPath string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeConfig,
		Message: fmt.Sprintf("Configuration file not found: %s", configPath),
		Suggestions: []string{
			fmt.Sprintf("Create config file: lspg config generate --output %s", configPath),
			"Run complete setup: lspg setup all",
			"Run diagnostics to check system: lspg diagnose",
			"Check current directory: pwd && ls -la *.yaml",
		},
		RelatedCmds: []string{
			"config generate",
			"setup all",
			"diagnose",
		},
	}
}

func NewPortInUseError(port int) *CLIError {
	return &CLIError{
		Type:    ErrorTypeNetwork,
		Message: fmt.Sprintf("Port %d is already in use", port),
		Suggestions: []string{
			fmt.Sprintf("Use a different port: lspg server --port %d", port+1),
			fmt.Sprintf("Check what's using the port: lsof -i :%d", port),
			fmt.Sprintf("Kill process using port: sudo kill $(lsof -t -i:%d)", port),
			"Wait a moment and try again (port may be in TIME_WAIT state)",
		},
		RelatedCmds: []string{
			"server --port <port>",
			"config generate",
		},
	}
}

func NewRuntimeNotFoundError(runtime string) *CLIError {
	supportedRuntimes := []string{"go", "python", "nodejs", "java"}
	return &CLIError{
		Type:    ErrorTypeRuntime,
		Message: fmt.Sprintf("Runtime '%s' not found or not supported", runtime),
		Suggestions: []string{
			fmt.Sprintf("Supported runtimes: %s", strings.Join(supportedRuntimes, ", ")),
			fmt.Sprintf("Install runtime: lspg install runtime %s", runtime),
			"Check runtime status: lspg status runtimes",
			"Run system diagnostics: lspg diagnose",
		},
		RelatedCmds: []string{
			"install runtime <name>",
			"status runtimes",
			"diagnose",
			"verify runtime <name>",
		},
	}
}

func NewServerNotFoundError(server string) *CLIError {
	supportedServers := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}
	return &CLIError{
		Type:    ErrorTypeServer,
		Message: fmt.Sprintf("Language server '%s' not found or not supported", server),
		Suggestions: []string{
			fmt.Sprintf("Supported servers: %s", strings.Join(supportedServers, ", ")),
			fmt.Sprintf("Install server: lspg install server %s", server),
			"Install all servers: lspg install servers",
			"Check server status: lspg status servers",
		},
		RelatedCmds: []string{
			"install server <name>",
			"install servers",
			"status servers",
			"verify runtime <name>",
		},
	}
}

func NewPermissionError(path string, operation string) *CLIError {
	return &CLIError{
		Type:    ErrorTypePermission,
		Message: fmt.Sprintf("Permission denied: cannot %s %s", operation, path),
		Suggestions: []string{
			fmt.Sprintf("Check file permissions: ls -la %s", path),
			fmt.Sprintf(SUGGESTION_FIX_PERMISSIONS, path),
			"Run with elevated privileges if needed: sudo lspg ...",
			"Check directory permissions for parent folders",
		},
		RelatedCmds: []string{
			"diagnose",
			"status",
		},
	}
}

func NewValidationError(item string, issues []string) *CLIError {
	suggestions := []string{
		"Fix the validation issues listed above",
		"Regenerate configuration: lspg config generate",
		"Run complete setup to fix configuration: lspg setup all",
	}

	if len(issues) > 0 {
		suggestions = append([]string{fmt.Sprintf("Issues found: %s", strings.Join(issues, ", "))}, suggestions...)
	}

	return &CLIError{
		Type:        ErrorTypeValidation,
		Message:     fmt.Sprintf("Validation failed for %s", item),
		Suggestions: suggestions,
		RelatedCmds: []string{
			"config validate",
			"config generate",
			"setup all",
			"diagnose",
		},
	}
}

func HandleConfigError(err error, configPath string) error {
	if err == nil {
		return nil
	}

	if os.IsNotExist(err) {
		return NewConfigNotFoundError(configPath)
	}

	if os.IsPermission(err) {
		return NewPermissionError(configPath, "read")
	}

	return NewConfigError("Failed to load configuration", err)
}

func HandleServerStartError(err error, port int) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	if strings.Contains(errStr, "bind") && strings.Contains(errStr, "address already in use") {
		return NewPortInUseError(port)
	}

	if strings.Contains(errStr, "permission denied") && port < 1024 {
		return &CLIError{
			Type:    ErrorTypePermission,
			Message: fmt.Sprintf("Permission denied: cannot bind to privileged port %d", port),
			Cause:   err,
			Suggestions: []string{
				fmt.Sprintf("Use a non-privileged port: lspg server --port %d", 8080),
				"Run with elevated privileges: sudo lspg server",
				"Use a port above 1024 (recommended)",
			},
			RelatedCmds: []string{
				"server --port <port>",
			},
		}
	}

	return &CLIError{
		Type:    ErrorTypeServer,
		Message: "Failed to start server",
		Cause:   err,
		Suggestions: []string{
			"Check configuration: lspg config validate",
			"Run diagnostics: lspg diagnose",
			"Verify runtime installations: lspg status runtimes",
			"Try a different port: lspg server --port 8081",
		},
		RelatedCmds: []string{
			"config validate",
			"diagnose",
			"status",
		},
	}
}

func NewGatewayStartupError(cause error) *CLIError {
	return &CLIError{
		Type:    ErrorTypeServer,
		Message: "Failed to start LSP Gateway",
		Cause:   cause,
		Suggestions: []string{
			"Check configuration: lspg config validate",
			"Verify runtime installations: lspg status runtimes",
			"Run system diagnostics: lspg diagnose",
			"Check for conflicting processes on the port",
		},
		RelatedCmds: []string{
			"config validate",
			"status runtimes",
			"diagnose",
			"server --port <port>",
		},
	}
}

func NewInstallerNotAvailableError(component string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeInstallation,
		Message: fmt.Sprintf("Failed to create %s installer", component),
		Suggestions: []string{
			"Check system compatibility",
			"Verify platform support",
			"Run diagnostics: lspg diagnose",
			"Report issue if platform should be supported",
		},
		RelatedCmds: []string{
			"diagnose",
			"status",
			"version",
		},
	}
}

func NewFileOperationError(operation string, path string, cause error) *CLIError {
	errorType := ErrorTypePermission
	if os.IsNotExist(cause) {
		errorType = ErrorTypeConfig
	}

	return &CLIError{
		Type:    errorType,
		Message: fmt.Sprintf("Failed to %s file: %s", operation, path),
		Cause:   cause,
		Suggestions: []string{
			fmt.Sprintf("Check if file exists: ls -la %s", path),
			fmt.Sprintf("Check file permissions: ls -la %s", path),
			"Ensure parent directory exists",
			"Check available disk space: df -h",
		},
		RelatedCmds: []string{
			"diagnose",
			"status",
		},
	}
}

func NewUnsupportedRuntimeError(runtime string, supportedRuntimes []string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeRuntime,
		Message: fmt.Sprintf("Runtime '%s' is not supported", runtime),
		Suggestions: []string{
			fmt.Sprintf("Supported runtimes are: %s", strings.Join(supportedRuntimes, ", ")),
			"Use 'lspg status runtimes' to see available runtimes",
			"Install missing runtime with package manager (apt, brew, npm, etc.)",
			"Check spelling of runtime name",
		},
		RelatedCmds: []string{
			"status runtimes",
			"install runtime <name>",
			"diagnose",
		},
	}
}

func NewInstallerCreationError(component string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeInstallation,
		Message: fmt.Sprintf("Failed to initialize %s installer", component),
		Suggestions: []string{
			"Check if platform is supported: lspg diagnose",
			"Verify system dependencies are available",
			"Check available disk space: df -h",
			"Try running with elevated privileges if needed",
		},
		RelatedCmds: []string{
			"diagnose",
			"status",
		},
	}
}

func NewJSONMarshalError(context string, cause error) *CLIError {
	return &CLIError{
		Type:    ErrorTypeGeneral,
		Message: fmt.Sprintf("Failed to format %s as JSON output", context),
		Cause:   cause,
		Suggestions: []string{
			"Try using the default (non-JSON) output format",
			"Check if the data contains invalid characters",
			"Report this as a bug if it persists",
		},
		RelatedCmds: []string{
			"diagnose",
		},
	}
}

func NewInfoRetrievalError(component string, cause error) *CLIError {
	return &CLIError{
		Type:    ErrorTypeRuntime,
		Message: fmt.Sprintf("Failed to gather %s information", component),
		Cause:   cause,
		Suggestions: []string{
			fmt.Sprintf("Check if %s is properly installed", component),
			"Run system diagnostics: lspg diagnose",
			"Verify system dependencies are available",
			"Check system resources and permissions",
		},
		RelatedCmds: []string{
			"diagnose",
			"status",
			fmt.Sprintf("install %s", component),
		},
	}
}

func NewMCPServerError(message string, cause error) *CLIError {
	return &CLIError{
		Type:    ErrorTypeServer,
		Message: fmt.Sprintf("MCP server error: %s", message),
		Cause:   cause,
		Suggestions: []string{
			"Check LSP Gateway configuration: lspg config validate",
			"Verify language servers are available: lspg status servers",
			"Try restarting the MCP server",
			"Check network connectivity if using TCP transport",
		},
		RelatedCmds: []string{
			"config validate",
			"status servers",
			"diagnose",
		},
	}
}
