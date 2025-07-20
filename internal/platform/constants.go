package platform

const (
	OS_RELEASE_FILE              = "/etc/os-release"
	SHELL_POWERSHELL             = "powershell"
	SHELL_BASH                   = "bash"
	SHELL_ZSH                    = "zsh"
	SHELL_PWSH                   = "pwsh"
	ENV_VAR_FORMAT               = "%s=%s"
	ENV_VAR_HOME                 = "HOME"
	COMMAND_TIMEOUT_MESSAGE      = "command timed out after %v"
	UNSUPPORTED_PLATFORM_MESSAGE = "Platform '%s' is not supported by this application"
	PACKAGE_MANAGER_DNF          = "dnf"
	PACKAGE_MANAGER_BREW         = "brew"
	PACKAGE_MANAGER_WINGET       = "winget"
	EXECUTABLE_EXTENSION         = ".exe"

	// Platform identifiers
	PLATFORM_DARWIN = "darwin"

	// Error messages
	ERROR_COMMAND_TIMEOUT     = "command timed out: %w"
	ERROR_NO_PACKAGE_MANAGERS = "no package managers available on platform %s"

	// Status values
	STATUS_UNKNOWN = "unknown"
)
