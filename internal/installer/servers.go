package installer

import (
	"fmt"
	"runtime"
	"time"

	"lsp-gateway/internal/types"
)

type ServerInstaller = types.ServerInstaller

type ServerInstallOptions = types.ServerInstallOptions

type DependencyValidationResult = types.DependencyValidationResult

type ServerPlatformStrategy = types.ServerPlatformStrategy

type ServerDefinition = types.ServerDefinition

type ServerInstallMethod = types.InstallMethod

type DefaultServerInstaller struct {
	registry         *ServerRegistry
	runtimeInstaller types.RuntimeInstaller
	strategies       map[string]types.ServerPlatformStrategy
}

func NewServerInstaller(runtimeInstaller types.RuntimeInstaller) *DefaultServerInstaller {
	// Add defensive nil check for runtime installer
	if runtimeInstaller == nil {
		return nil
	}

	installer := &DefaultServerInstaller{
		registry:         NewServerRegistry(),
		runtimeInstaller: runtimeInstaller,
		strategies:       make(map[string]types.ServerPlatformStrategy),
	}

	// Initialize server platform strategies
	installer.initializeStrategies()

	return installer
}

// initializeStrategies sets up the server platform strategies
func (s *DefaultServerInstaller) initializeStrategies() {
	// Use universal strategy for all platforms - it handles cross-platform differences internally
	universalStrategy := NewUniversalServerStrategy()

	s.strategies["windows"] = universalStrategy
	s.strategies["linux"] = universalStrategy
	s.strategies["darwin"] = universalStrategy
}

func (s *DefaultServerInstaller) Install(server string, options types.ServerInstallOptions) (*types.InstallResult, error) {
	startTime := time.Now()

	_, err := s.registry.GetServer(server)
	if err != nil {
		return nil, NewInstallerError(InstallerErrorTypeNotFound, server,
			fmt.Sprintf("unknown server: %s", server), err)
	}

	if !options.SkipDependencyCheck {
		if depResult, err := s.ValidateDependencies(server); err != nil || !depResult.CanInstall {
			return &types.InstallResult{
				Success:  false,
				Version:  "",
				Path:     "",
				Method:   "dependency_check",
				Duration: time.Since(startTime),
				Warnings: []string{},
				Errors:   []string{fmt.Sprintf("dependency validation failed: %v", err)},
				Details:  map[string]interface{}{"server": server},
			}, nil
		}
	}

	platform := options.Platform
	if platform == "" {
		platform = s.detectCurrentPlatform()
	}

	strategy := s.GetPlatformStrategy(platform)
	if strategy == nil {
		return &types.InstallResult{
			Success:  false,
			Version:  "",
			Path:     "",
			Method:   "platform_strategy",
			Duration: time.Since(startTime),
			Warnings: []string{},
			Errors:   []string{fmt.Sprintf("no strategy available for platform: %s", platform)},
			Details:  map[string]interface{}{"server": server, "platform": platform},
		}, nil
	}

	installResult, installErr := strategy.InstallServer(server, options)
	if installErr != nil {
		return &types.InstallResult{
			Success:  false,
			Version:  "",
			Path:     "",
			Method:   fmt.Sprintf("%s_platform_strategy", platform),
			Duration: time.Since(startTime),
			Warnings: []string{},
			Errors:   []string{installErr.Error()},
			Details:  map[string]interface{}{"server": server, "platform": platform},
		}, nil
	}

	if installResult != nil {
		installResult.Duration = time.Since(startTime)
		installResult.Details["platform"] = platform
		return installResult, nil
	}

	result := &types.InstallResult{
		Success:  true,
		Version:  options.Version,
		Path:     "",
		Method:   fmt.Sprintf("%s_platform_strategy", platform),
		Duration: time.Since(startTime),
		Warnings: []string{},
		Errors:   []string{},
		Details:  map[string]interface{}{"server": server, "platform": platform},
	}

	if !options.SkipVerify {
		if verifyResult, err := s.Verify(server); err != nil || !verifyResult.Installed {
			result.Success = false
			result.Warnings = append(result.Warnings, "installation completed but verification failed")
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("verification error: %v", err))
			}
		} else {
			result.Path = verifyResult.Path
			result.Version = verifyResult.Version
		}
	}

	return result, nil
}

func (s *DefaultServerInstaller) Verify(server string) (*types.VerificationResult, error) {
	// Add defensive nil check for runtime installer
	if s.runtimeInstaller == nil {
		return nil, NewInstallerError(InstallerErrorTypeNotFound, server,
			"runtime installer not available", nil)
	}

	verifier := NewServerVerifier(s.runtimeInstaller)
	if verifier == nil {
		return nil, NewInstallerError(InstallerErrorTypeNotFound, server,
			"server verifier could not be created", nil)
	}

	serverResult, err := verifier.VerifyServer(server)
	if err != nil {
		return nil, err
	}

	result := &types.VerificationResult{
		Installed:       serverResult.Installed,
		Compatible:      serverResult.Compatible,
		Version:         serverResult.Version,
		Path:            serverResult.Path,
		Issues:          serverResult.Issues,
		Details:         make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
		Metadata:        make(map[string]interface{}),
	}

	result.Details["server"] = server
	result.Details["verified_at"] = serverResult.VerifiedAt
	result.Details["duration"] = serverResult.Duration
	result.Details["functional"] = serverResult.Functional
	result.Details["runtime_required"] = serverResult.RuntimeRequired

	if serverResult.RuntimeStatus != nil {
		result.Details["runtime_status"] = serverResult.RuntimeStatus
	}

	if serverResult.HealthCheck != nil {
		result.Details["health_check"] = map[string]interface{}{
			"responsive":   serverResult.HealthCheck.Responsive,
			"startup_time": serverResult.HealthCheck.StartupTime,
			"test_method":  serverResult.HealthCheck.TestMethod,
			"tested_at":    serverResult.HealthCheck.TestedAt,
		}
	}

	for k, v := range serverResult.Metadata {
		result.Details[k] = v
	}

	return result, nil
}

func (s *DefaultServerInstaller) ValidateDependencies(server string) (*types.DependencyValidationResult, error) {
	// Add defensive nil check for runtime installer
	if s.runtimeInstaller == nil {
		return nil, NewInstallerError(InstallerErrorTypeNotFound, server,
			"runtime installer not available", nil)
	}

	serverDef, err := s.registry.GetServer(server)
	if err != nil {
		return nil, NewInstallerError(InstallerErrorTypeNotFound, server,
			fmt.Sprintf("unknown server: %s", server), err)
	}

	result := &types.DependencyValidationResult{
		CanInstall:      false,
		MissingRuntimes: []string{},
		VersionIssues:   []types.VersionIssue{},
		Recommendations: []string{},
	}

	runtimeResult, err := s.runtimeInstaller.Verify(serverDef.Runtime)
	if err != nil {
		result.MissingRuntimes = append(result.MissingRuntimes, serverDef.Runtime)
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Install %s runtime before installing %s server", serverDef.Runtime, server))
		return result, nil
	}

	if !runtimeResult.Installed {
		result.MissingRuntimes = append(result.MissingRuntimes, serverDef.Runtime)
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Install %s runtime before installing %s server", serverDef.Runtime, server))
		return result, nil
	}

	if !runtimeResult.Compatible {
		result.VersionIssues = append(result.VersionIssues, types.VersionIssue{
			Component:        serverDef.Runtime,
			RequiredVersion:  serverDef.MinVersion,
			InstalledVersion: runtimeResult.Version,
			Severity:         types.IssueSeverityHigh,
		})
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Upgrade %s runtime to meet minimum version requirements", serverDef.Runtime))
		return result, nil
	}

	result.CanInstall = true

	return result, nil
}

func (s *DefaultServerInstaller) GetSupportedServers() []string {
	return s.registry.ListServers()
}

func (s *DefaultServerInstaller) GetServerInfo(server string) (*types.ServerDefinition, error) {
	return s.registry.GetServer(server)
}

func (s *DefaultServerInstaller) GetPlatformStrategy(platform string) types.ServerPlatformStrategy {
	if strategy, exists := s.strategies[platform]; exists {
		return strategy
	}

	// Safe fallback - check linux strategy exists before returning
	if linuxStrategy, exists := s.strategies["linux"]; exists {
		return linuxStrategy
	}

	// Return nil if no strategies available - caller must handle this
	return nil
}

func (s *DefaultServerInstaller) detectCurrentPlatform() string {
	return runtime.GOOS
}

type ServerRegistry struct {
	servers map[string]*types.ServerDefinition
}

func NewServerRegistry() *ServerRegistry {
	registry := &ServerRegistry{
		servers: make(map[string]*types.ServerDefinition),
	}

	registry.registerDefaults()

	return registry
}

func (s *ServerRegistry) registerDefaults() {
	s.servers["gopls"] = &types.ServerDefinition{
		Name:              "gopls",
		DisplayName:       "Go Language Server",
		Runtime:           "go",
		MinVersion:        "1.19.0",
		MinRuntimeVersion: "1.19.0",
		InstallCmd:        []string{"go", "install", "golang.org/x/tools/gopls@latest"},
		VerifyCmd:         []string{"gopls", "version"},
		ConfigKey:         "go-lsp",
		Description:       "Official Go language server",
		Homepage:          "https://golang.org/x/tools/gopls",
		Languages:         []string{"go"},
		InstallMethods: []types.InstallMethod{
			{
				Name:          "go_install",
				Platform:      "all",
				Method:        "go_install",
				Commands:      []string{"go", "install", "golang.org/x/tools/gopls@latest"},
				Description:   "Install using go install command",
				Requirements:  []string{"go >= 1.19.0"},
				PreRequisites: []string{"go version"},
				Verification:  []string{"gopls", "version"},
				PostInstall:   []string{},
			},
		},
		VersionCommand: []string{"gopls", "version"},
	}

	s.servers["pylsp"] = &types.ServerDefinition{
		Name:        "pylsp",
		DisplayName: "Python Language Server",
		Runtime:     "python",
		MinVersion:  "3.9.0",
		InstallCmd:  []string{"pip", "install", "python-lsp-server"},
		VerifyCmd:   []string{"pylsp", "--version"},
		ConfigKey:   "python-lsp",
		Description: "Python LSP server implementation",
		Homepage:    "https://github.com/python-lsp/python-lsp-server",
		Languages:   []string{"python"},
		InstallMethods: []types.InstallMethod{
			{
				Name:          "pip_install",
				Platform:      "all",
				Method:        "pip_install",
				Commands:      []string{"pip", "install", "python-lsp-server"},
				Description:   "Install using pip package manager",
				Requirements:  []string{"python >= 3.9.0", "pip"},
				PreRequisites: []string{"python --version", "pip --version"},
				Verification:  []string{"pylsp", "--version"},
				PostInstall:   []string{},
			},
		},
		VersionCommand: []string{"pylsp", "--version"},
	}

	s.servers["typescript-language-server"] = &types.ServerDefinition{
		Name:        "typescript-language-server",
		DisplayName: "TypeScript Language Server",
		Runtime:     "nodejs",
		MinVersion:  "22.0.0",
		InstallCmd:  []string{"npm", "install", "-g", "typescript-language-server", "typescript"},
		VerifyCmd:   []string{"typescript-language-server", "--version"},
		ConfigKey:   "typescript-lsp",
		Description: "Language server for TypeScript and JavaScript",
		Homepage:    "https://github.com/typescript-language-server/typescript-language-server",
		Languages:   []string{"typescript", "javascript"},
		InstallMethods: []types.InstallMethod{
			{
				Name:          "npm_global",
				Platform:      "all",
				Method:        "npm_global",
				Commands:      []string{"npm", "install", "-g", "typescript-language-server", "typescript"},
				Description:   "Install globally using npm",
				Requirements:  []string{"nodejs >= 22.0.0", "npm"},
				PreRequisites: []string{"node --version", "npm --version"},
				Verification:  []string{"typescript-language-server", "--version"},
				PostInstall:   []string{},
			},
		},
		VersionCommand: []string{"typescript-language-server", "--version"},
	}

	jdtlsVerifyCmd := getJDTLSVerificationCommands()
	s.servers["jdtls"] = &types.ServerDefinition{
		Name:              "jdtls",
		DisplayName:       "Eclipse JDT Language Server",
		Runtime:           "java",
		MinVersion:        "1.48.0",
		MinRuntimeVersion: "21.0.0",
		InstallCmd:        []string{"# Automated download and installation from Eclipse releases"},
		VerifyCmd:         jdtlsVerifyCmd,
		ConfigKey:         "java-lsp",
		Description:       "Eclipse JDT Language Server for Java with automated installation",
		Homepage:          "https://github.com/eclipse/eclipse.jdt.ls",
		Languages:         []string{"java"},
		InstallMethods: []types.InstallMethod{
			{
				Name:          "automated_download",
				Platform:      "all",
				Method:        "automated_download",
				Commands:      []string{"# Automated download with checksum verification"},
				Description:   "Automated download and installation from Eclipse releases with SHA256 verification",
				Requirements:  []string{"java >= 21.0.0"},
				PreRequisites: []string{"java --version"},
				Verification:  jdtlsVerifyCmd,
				PostInstall:   []string{"echo", "JDTLS installed successfully. Use 'jdtls <workspace>' to start the language server."},
			},
		},
		VersionCommand: jdtlsVerifyCmd,
	}
}

func (s *ServerRegistry) GetServer(name string) (*types.ServerDefinition, error) {
	if server, exists := s.servers[name]; exists {
		return server, nil
	}
	return nil, &InstallerError{
		Type:    InstallerErrorTypeNotFound,
		Message: "server not found: " + name,
	}
}

func (s *ServerRegistry) ListServers() []string {
	names := make([]string, 0, len(s.servers))
	for name := range s.servers {
		names = append(names, name)
	}
	return names
}

func (s *ServerRegistry) GetServersByRuntime(runtime string) []*types.ServerDefinition {
	var servers []*types.ServerDefinition
	for _, server := range s.servers {
		if server.Runtime == runtime {
			servers = append(servers, server)
		}
	}
	return servers
}

func (s *ServerRegistry) GetServersByLanguage(language string) []*types.ServerDefinition {
	var servers []*types.ServerDefinition
	for _, server := range s.servers {
		for _, lang := range server.Languages {
			if lang == language {
				servers = append(servers, server)
				break
			}
		}
	}
	return servers
}
