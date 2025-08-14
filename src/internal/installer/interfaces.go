package installer

import (
	"context"
	"lsp-gateway/src/config"
)

// LanguageInstaller interface defines the contract for language-specific installers
type LanguageInstaller interface {
	// Install installs the language server with given options
	Install(ctx context.Context, options InstallOptions) error

	// IsInstalled checks if the language server is already installed and working
	IsInstalled() bool

	// GetVersion returns the currently installed version
	GetVersion() (string, error)

	// Uninstall removes the installed language server
	Uninstall() error

	// GetLanguage returns the language this installer handles
	GetLanguage() string

	// GetServerConfig returns the configuration for this language server
	GetServerConfig() *config.ServerConfig

	// ValidateInstallation performs post-install validation
	ValidateInstallation() error
}

// InstallManager manages all language installers
type InstallManager interface {
	// RegisterInstaller registers a language installer
	RegisterInstaller(language string, installer LanguageInstaller)

	// GetInstaller returns installer for a specific language
	GetInstaller(language string) (LanguageInstaller, error)

	// InstallLanguage installs a specific language server
	InstallLanguage(ctx context.Context, language string, options InstallOptions) error

	// InstallAll installs all supported language servers
	InstallAll(ctx context.Context, options InstallOptions) error

	// GetStatus returns installation status for all languages
	GetStatus() map[string]InstallStatus

	// GetSupportedLanguages returns list of supported languages
	GetSupportedLanguages() []string
}

// InstallOptions contains options for installation
type InstallOptions struct {
	// InstallPath specifies custom installation directory
	InstallPath string

	// Version specifies target version (empty for latest)
	Version string

	// Force reinstalls even if already installed
	Force bool

	// Offline prevents downloading (use local/cached only)
	Offline bool

	// SkipValidation skips post-install validation
	SkipValidation bool

	// UpdateConfig automatically updates configuration files
	UpdateConfig bool

	// Server selects variant when multiple LSP servers exist for a language
	// e.g., for python: "jedi" only
	Server string
}

// InstallStatus represents installation status for a language
type InstallStatus struct {
	// Language name
	Language string

	// Installed indicates if the server is installed
	Installed bool

	// Available indicates if installation is supported
	Available bool

	// Version of installed server (if installed)
	Version string

	// Error contains any error from checking status
	Error error

	// InstallPath contains the installation path
	InstallPath string

	// ConfigUpdated indicates if config was automatically updated
	ConfigUpdated bool
}

// Platform information interface
type PlatformInfo interface {
	// GetPlatform returns current platform (linux, darwin, windows)
	GetPlatform() string

	// GetArch returns current architecture (amd64, arm64)
	GetArch() string

	// GetPlatformString returns platform string for downloads
	GetPlatformString() string

	// IsSupported checks if current platform is supported
	IsSupported() bool

	// GetJavaDownloadURL returns the appropriate JDK download URL for the platform
	GetJavaDownloadURL(version string) (string, string, error)

	// GetNodeInstallCommand returns the command to install Node.js/npm if needed
	GetNodeInstallCommand() []string
}
