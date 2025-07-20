package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
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
			parts = append(parts, fmt.Sprintf("   lsp-gateway %s", cmd))
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
			"Generate a new config with: lsp-gateway config generate",
			"Validate existing config with: lsp-gateway config validate",
			"Run setup wizard: lsp-gateway setup wizard",
		},
		RelatedCmds: []string{
			"config generate",
			"config validate",
			"setup wizard",
			"diagnose",
		},
	}
}

func NewConfigNotFoundError(configPath string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeConfig,
		Message: fmt.Sprintf("Configuration file not found: %s", configPath),
		Suggestions: []string{
			fmt.Sprintf("Create config file: lsp-gateway config generate --output %s", configPath),
			"Run complete setup: lsp-gateway setup all",
			"Use interactive setup: lsp-gateway setup wizard",
			"Check current directory: pwd && ls -la *.yaml",
		},
		RelatedCmds: []string{
			"config generate",
			"setup all",
			"setup wizard",
		},
	}
}

func NewPortInUseError(port int) *CLIError {
	return &CLIError{
		Type:    ErrorTypeNetwork,
		Message: fmt.Sprintf("Port %d is already in use", port),
		Suggestions: []string{
			fmt.Sprintf("Use a different port: lsp-gateway server --port %d", port+1),
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
			fmt.Sprintf("Install runtime: lsp-gateway install runtime %s", runtime),
			"Check runtime status: lsp-gateway status runtimes",
			"Run system diagnostics: lsp-gateway diagnose",
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
			fmt.Sprintf("Install server: lsp-gateway install server %s", server),
			"Install all servers: lsp-gateway install servers",
			"Check server status: lsp-gateway status servers",
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
			"Run with elevated privileges if needed: sudo lsp-gateway ...",
			"Check directory permissions for parent folders",
		},
		RelatedCmds: []string{
			"diagnose",
			"status",
		},
	}
}

func NewInstallationError(component string, cause error) *CLIError {
	return &CLIError{
		Type:    ErrorTypeInstallation,
		Message: fmt.Sprintf("Failed to install %s", component),
		Cause:   cause,
		Suggestions: []string{
			"Check internet connectivity: ping google.com",
			"Retry installation: lsp-gateway install --force",
			"Run system diagnostics: lsp-gateway diagnose",
			"Check available disk space: df -h",
			"Update package managers (apt, brew, etc.)",
		},
		RelatedCmds: []string{
			"install --force",
			"diagnose",
			"status",
		},
	}
}

func NewValidationError(item string, issues []string) *CLIError {
	suggestions := []string{
		"Fix the validation issues listed above",
		"Regenerate configuration: lsp-gateway config generate",
		"Use setup wizard for guided configuration: lsp-gateway setup wizard",
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
			"setup wizard",
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
				fmt.Sprintf("Use a non-privileged port: lsp-gateway server --port %d", 8080),
				"Run with elevated privileges: sudo lsp-gateway server",
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
			"Check configuration: lsp-gateway config validate",
			"Run diagnostics: lsp-gateway diagnose",
			"Verify runtime installations: lsp-gateway status runtimes",
			"Try a different port: lsp-gateway server --port 8081",
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
			"Check configuration: lsp-gateway config validate",
			"Verify runtime installations: lsp-gateway status runtimes",
			"Run system diagnostics: lsp-gateway diagnose",
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

func NewRuntimeValidationError(runtime string, supportedRuntimes []string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeValidation,
		Message: fmt.Sprintf("Unsupported runtime: %s", runtime),
		Suggestions: []string{
			fmt.Sprintf("Supported runtimes: %s", strings.Join(supportedRuntimes, ", ")),
			"Check spelling and try again",
			"List available runtimes: lsp-gateway status runtimes",
			"Install a supported runtime: lsp-gateway install runtime <name>",
		},
		RelatedCmds: []string{
			"status runtimes",
			"install runtime <name>",
			"diagnose runtimes",
		},
	}
}

func NewServerValidationError(server string, supportedServers []string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeValidation,
		Message: fmt.Sprintf("Unsupported language server: %s", server),
		Suggestions: []string{
			fmt.Sprintf("Supported servers: %s", strings.Join(supportedServers, ", ")),
			"Check spelling and try again",
			"List available servers: lsp-gateway status servers",
			"Install a supported server: lsp-gateway install server <name>",
		},
		RelatedCmds: []string{
			"status servers",
			"install server <name>",
			"diagnose servers",
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
			"Run diagnostics: lsp-gateway diagnose",
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

func NewTimeoutError(operation string, timeout time.Duration) *CLIError {
	return &CLIError{
		Type:    ErrorTypeGeneral,
		Message: fmt.Sprintf("Operation timed out after %v: %s", timeout, operation),
		Suggestions: []string{
			"Increase timeout with --timeout flag",
			"Check network connectivity",
			"Run diagnostics: lsp-gateway diagnose",
			"Try operation again later",
		},
		RelatedCmds: []string{
			"diagnose",
			"status",
		},
	}
}

func FormatCLIError(err error) error {
	if cliErr, ok := err.(*CLIError); ok {
		return cliErr
	}

	return &CLIError{
		Type:    ErrorTypeGeneral,
		Message: err.Error(),
		Suggestions: []string{
			"Run diagnostics for more information: lsp-gateway diagnose",
			"Check system status: lsp-gateway status",
			"Consult documentation or report issue if problem persists",
		},
		RelatedCmds: []string{
			"diagnose",
			"status",
			"version",
		},
	}
}

func NewConfigParseError(configPath string, cause error) *CLIError {
	return &CLIError{
		Type:    ErrorTypeConfig,
		Message: fmt.Sprintf("Configuration file contains syntax errors: %s", configPath),
		Cause:   cause,
		Suggestions: []string{
			"Check YAML syntax in your config file",
			"Generate a new config file: lsp-gateway config generate --overwrite",
			"Validate the config structure online at yamllint.com",
			"Compare with working config: lsp-gateway config show --json > backup.json",
		},
		RelatedCmds: []string{
			"config generate",
			"config validate",
			"config show",
		},
	}
}

func NewConfigReadError(configPath string, cause error) *CLIError {
	return &CLIError{
		Type:    ErrorTypeConfig,
		Message: fmt.Sprintf("Cannot read configuration file: %s", configPath),
		Cause:   cause,
		Suggestions: []string{
			fmt.Sprintf("Check file permissions: ls -la %s", configPath),
			fmt.Sprintf("Fix file permissions: chmod 644 %s", configPath),
			"Ensure the file exists and is readable",
			"Generate a new config file: lsp-gateway config generate",
		},
		RelatedCmds: []string{
			"config generate",
			"diagnose",
		},
	}
}

func NewUnsupportedRuntimeError(runtime string, supportedRuntimes []string) *CLIError {
	return &CLIError{
		Type:    ErrorTypeRuntime,
		Message: fmt.Sprintf("Runtime '%s' is not supported", runtime),
		Suggestions: []string{
			fmt.Sprintf("Supported runtimes are: %s", strings.Join(supportedRuntimes, ", ")),
			"Use 'lsp-gateway status runtimes' to see available runtimes",
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
			"Check if platform is supported: lsp-gateway diagnose",
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
			"Run system diagnostics: lsp-gateway diagnose",
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
			"Check LSP Gateway configuration: lsp-gateway config validate",
			"Verify language servers are available: lsp-gateway status servers",
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

func NewMCPProtocolError(issue string, cause error) *CLIError {
	return &CLIError{
		Type:    ErrorTypeValidation,
		Message: fmt.Sprintf("MCP protocol error: %s", issue),
		Cause:   cause,
		Suggestions: []string{
			"Check MCP client compatibility",
			"Verify message format and encoding",
			"Try using stdio transport instead of TCP",
			"Report this as a bug if using supported MCP client",
		},
		RelatedCmds: []string{
			"mcp --transport stdio",
			"diagnose",
		},
	}
}

func NewMCPTransportError(transport string, cause error) *CLIError {
	return &CLIError{
		Type:    ErrorTypeNetwork,
		Message: fmt.Sprintf("MCP %s transport error", transport),
		Cause:   cause,
		Suggestions: []string{
			"Try using stdio transport: lsp-gateway mcp --transport stdio",
			"Check if port is available: lsof -i :9090",
			"Verify network permissions and firewall settings",
			"Use a different port: lsp-gateway mcp --transport tcp --port 9091",
		},
		RelatedCmds: []string{
			"mcp --transport stdio",
			"mcp --transport tcp --port <port>",
			"diagnose",
		},
	}
}
