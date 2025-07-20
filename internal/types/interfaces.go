package types

type RuntimeInstaller interface {
	Install(runtime string, options InstallOptions) (*InstallResult, error)

	Verify(runtime string) (*VerificationResult, error)

	GetSupportedRuntimes() []string

	GetRuntimeInfo(runtime string) (*RuntimeDefinition, error)

	ValidateVersion(runtime, minVersion string) (*VersionValidationResult, error)

	GetPlatformStrategy(platform string) RuntimePlatformStrategy
}

type ServerInstaller interface {
	Install(server string, options ServerInstallOptions) (*InstallResult, error)

	Verify(server string) (*VerificationResult, error)

	GetSupportedServers() []string

	GetServerInfo(server string) (*ServerDefinition, error)

	ValidateDependencies(server string) (*DependencyValidationResult, error)

	GetPlatformStrategy(platform string) ServerPlatformStrategy
}

type RuntimePlatformStrategy interface {
	InstallRuntime(runtime string, options InstallOptions) (*InstallResult, error)
	VerifyRuntime(runtime string) (*VerificationResult, error)
	GetInstallCommand(runtime, version string) ([]string, error)
}

type ServerPlatformStrategy interface {
	InstallServer(server string, options ServerInstallOptions) (*InstallResult, error)
	VerifyServer(server string) (*VerificationResult, error)
	GetInstallCommand(server, version string) ([]string, error)
}
