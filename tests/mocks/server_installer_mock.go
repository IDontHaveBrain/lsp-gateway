package mocks

import (
	"lsp-gateway/internal/types"
	"time"
)

type MockServerInstaller struct {
	InstallFunc              func(server string, options types.ServerInstallOptions) (*types.InstallResult, error)
	VerifyFunc               func(server string) (*types.VerificationResult, error)
	GetSupportedServersFunc  func() []string
	GetServerInfoFunc        func(server string) (*types.ServerDefinition, error)
	ValidateDependenciesFunc func(server string) (*types.DependencyValidationResult, error)
	GetPlatformStrategyFunc  func(platform string) types.ServerPlatformStrategy

	InstallCalls              []ServerInstallCall
	VerifyCalls               []string
	GetSupportedServersCalls  int
	GetServerInfoCalls        []string
	ValidateDependenciesCalls []string
	GetPlatformStrategyCalls  []string
}

type ServerInstallCall struct {
	Server  string
	Options types.ServerInstallOptions
}

func NewMockServerInstaller() *MockServerInstaller {
	return &MockServerInstaller{
		InstallCalls:              make([]ServerInstallCall, 0),
		VerifyCalls:               make([]string, 0),
		GetServerInfoCalls:        make([]string, 0),
		ValidateDependenciesCalls: make([]string, 0),
		GetPlatformStrategyCalls:  make([]string, 0),
	}
}

func (m *MockServerInstaller) Install(server string, options types.ServerInstallOptions) (*types.InstallResult, error) {
	m.InstallCalls = append(m.InstallCalls, ServerInstallCall{Server: server, Options: options})
	if m.InstallFunc != nil {
		return m.InstallFunc(server, options)
	}

	return &types.InstallResult{
		Success:  true,
		Runtime:  server,
		Version:  "1.0.0",
		Path:     "/usr/local/bin/" + server,
		Method:   "npm_install",
		Duration: time.Second * 10,
		Errors:   []string{},
		Warnings: []string{},
		Messages: []string{"Successfully installed " + server + " language server"},
		Details:  map[string]interface{}{"server_type": "lsp", "installation_method": "mock"},
	}, nil
}

func (m *MockServerInstaller) Verify(server string) (*types.VerificationResult, error) {
	m.VerifyCalls = append(m.VerifyCalls, server)
	if m.VerifyFunc != nil {
		return m.VerifyFunc(server)
	}

	return &types.VerificationResult{
		Installed:       true,
		Compatible:      true,
		Version:         "1.0.0",
		Path:            "/usr/local/bin/" + server,
		Runtime:         server,
		Issues:          []types.Issue{},
		Details:         map[string]interface{}{"server_verified": true},
		Metadata:        map[string]interface{}{"server_type": "lsp"},
		EnvironmentVars: map[string]string{},
		Recommendations: []string{},
		WorkingDir:      "/tmp",
		AdditionalPaths: []string{},
		VerifiedAt:      time.Now(),
		Duration:        time.Millisecond * 200,
	}, nil
}

func (m *MockServerInstaller) GetSupportedServers() []string {
	m.GetSupportedServersCalls++
	if m.GetSupportedServersFunc != nil {
		return m.GetSupportedServersFunc()
	}
	return []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}
}

func (m *MockServerInstaller) GetServerInfo(server string) (*types.ServerDefinition, error) {
	m.GetServerInfoCalls = append(m.GetServerInfoCalls, server)
	if m.GetServerInfoFunc != nil {
		return m.GetServerInfoFunc(server)
	}

	serverInfo := map[string]*types.ServerDefinition{
		"gopls": {
			Name:              "gopls",
			DisplayName:       "Go Language Server",
			Runtime:           "go",
			MinVersion:        "1.0.0",
			MinRuntimeVersion: "1.20.0",
			InstallCmd:        []string{"go", "install", "golang.org/x/tools/gopls@latest"},
			VerifyCmd:         []string{"gopls", "version"},
			ConfigKey:         "gopls",
			Description:       "Official Go language server",
			Homepage:          "https://golang.org/x/tools/gopls",
			Languages:         []string{"go"},
			Extensions:        []string{".go"},
			InstallMethods: []types.InstallMethod{
				{
					Name:         "Go Install",
					Platform:     "linux",
					Method:       "go_install",
					Commands:     []string{"go", "install", "golang.org/x/tools/gopls@latest"},
					Description:  "Install via go install",
					Requirements: []string{"go"},
				},
			},
			VersionCommand: []string{"gopls", "version"},
		},
		"pylsp": {
			Name:              "pylsp",
			DisplayName:       "Python LSP Server",
			Runtime:           "python",
			MinVersion:        "1.0.0",
			MinRuntimeVersion: "3.9.0",
			InstallCmd:        []string{"pip", "install", "python-lsp-server"},
			VerifyCmd:         []string{"pylsp", "--help"},
			ConfigKey:         "pylsp",
			Description:       "Python Language Server Protocol implementation",
			Homepage:          "https://github.com/python-lsp/python-lsp-server",
			Languages:         []string{"python"},
			Extensions:        []string{".py"},
			InstallMethods: []types.InstallMethod{
				{
					Name:         "Pip Install",
					Platform:     "linux",
					Method:       "pip",
					Commands:     []string{"pip", "install", "python-lsp-server"},
					Description:  "Install via pip",
					Requirements: []string{"python", "pip"},
				},
			},
			VersionCommand: []string{"pylsp", "--version"},
		},
		"typescript-language-server": {
			Name:              "typescript-language-server",
			DisplayName:       "TypeScript Language Server",
			Runtime:           "nodejs",
			MinVersion:        "1.0.0",
			MinRuntimeVersion: "22.0.0",
			InstallCmd:        []string{"npm", "install", "-g", "typescript-language-server", "typescript"},
			VerifyCmd:         []string{"typescript-language-server", "--version"},
			ConfigKey:         "tsserver",
			Description:       "TypeScript & JavaScript language server",
			Homepage:          "https://github.com/typescript-language-server/typescript-language-server",
			Languages:         []string{"typescript", "javascript"},
			Extensions:        []string{".ts", ".tsx", ".js", ".jsx"},
			InstallMethods: []types.InstallMethod{
				{
					Name:         "NPM Install",
					Platform:     "linux",
					Method:       "npm",
					Commands:     []string{"npm", "install", "-g", "typescript-language-server", "typescript"},
					Description:  "Install via npm",
					Requirements: []string{"nodejs", "npm"},
				},
			},
			VersionCommand: []string{"typescript-language-server", "--version"},
		},
		"jdtls": {
			Name:              "jdtls",
			DisplayName:       "Eclipse JDT Language Server",
			Runtime:           "java",
			MinVersion:        "1.0.0",
			MinRuntimeVersion: "11.0.0",
			InstallCmd:        []string{"curl", "-o", "jdt-language-server.tar.gz", "https://download.eclipse.org/jdtls/snapshots/jdt-language-server-latest.tar.gz"},
			VerifyCmd:         []string{"java", "-jar", "jdtls.jar", "--version"},
			ConfigKey:         "jdtls",
			Description:       "Eclipse JDT Language Server for Java",
			Homepage:          "https://github.com/eclipse/eclipse.jdt.ls",
			Languages:         []string{"java"},
			Extensions:        []string{".java"},
			InstallMethods: []types.InstallMethod{
				{
					Name:         "Download Archive",
					Platform:     "linux",
					Method:       "download",
					Commands:     []string{"curl", "-o", "jdt-language-server.tar.gz", "https://download.eclipse.org/jdtls/snapshots/jdt-language-server-latest.tar.gz"},
					Description:  "Download and extract JDTLS",
					Requirements: []string{"java", "curl"},
				},
			},
			VersionCommand: []string{"java", "-jar", "jdtls.jar", "--version"},
		},
	}

	if info, exists := serverInfo[server]; exists {
		return info, nil
	}

	return &types.ServerDefinition{
		Name:              server,
		DisplayName:       server + " Language Server",
		Runtime:           "unknown",
		MinVersion:        "1.0.0",
		MinRuntimeVersion: "1.0.0",
		InstallCmd:        []string{"echo", "mock install " + server},
		VerifyCmd:         []string{"echo", "mock verify " + server},
		ConfigKey:         server,
		Description:       "Mock " + server + " language server",
		Homepage:          "https://example.com/" + server,
		Languages:         []string{server},
		Extensions:        []string{"." + server},
		InstallMethods:    []types.InstallMethod{},
		VersionCommand:    []string{"echo", "1.0.0"},
	}, nil
}

func (m *MockServerInstaller) ValidateDependencies(server string) (*types.DependencyValidationResult, error) {
	m.ValidateDependenciesCalls = append(m.ValidateDependenciesCalls, server)
	if m.ValidateDependenciesFunc != nil {
		return m.ValidateDependenciesFunc(server)
	}

	return &types.DependencyValidationResult{
		Server:            server,
		Valid:             true,
		RuntimeRequired:   "go",
		RuntimeInstalled:  true,
		RuntimeVersion:    "1.24.0",
		RuntimeCompatible: true,
		Issues:            []types.Issue{},
		CanInstall:        true,
		MissingRuntimes:   []string{},
		VersionIssues:     []types.VersionIssue{},
		Recommendations:   []string{},
		ValidatedAt:       time.Now(),
		Duration:          time.Millisecond * 100,
	}, nil
}

func (m *MockServerInstaller) GetPlatformStrategy(platform string) types.ServerPlatformStrategy {
	m.GetPlatformStrategyCalls = append(m.GetPlatformStrategyCalls, platform)
	if m.GetPlatformStrategyFunc != nil {
		return m.GetPlatformStrategyFunc(platform)
	}
	return NewMockServerPlatformStrategy()
}

func (m *MockServerInstaller) Reset() {
	m.InstallCalls = make([]ServerInstallCall, 0)
	m.VerifyCalls = make([]string, 0)
	m.GetSupportedServersCalls = 0
	m.GetServerInfoCalls = make([]string, 0)
	m.ValidateDependenciesCalls = make([]string, 0)
	m.GetPlatformStrategyCalls = make([]string, 0)

	m.InstallFunc = nil
	m.VerifyFunc = nil
	m.GetSupportedServersFunc = nil
	m.GetServerInfoFunc = nil
	m.ValidateDependenciesFunc = nil
	m.GetPlatformStrategyFunc = nil
}

type MockServerPlatformStrategy struct {
	InstallServerFunc     func(server string, options types.ServerInstallOptions) (*types.InstallResult, error)
	VerifyServerFunc      func(server string) (*types.VerificationResult, error)
	GetInstallCommandFunc func(server, version string) ([]string, error)

	InstallServerCalls     []ServerInstallCall
	VerifyServerCalls      []string
	GetInstallCommandCalls []GetServerInstallCommandCall
}

type GetServerInstallCommandCall struct {
	Server  string
	Version string
}

func NewMockServerPlatformStrategy() *MockServerPlatformStrategy {
	return &MockServerPlatformStrategy{
		InstallServerCalls:     make([]ServerInstallCall, 0),
		VerifyServerCalls:      make([]string, 0),
		GetInstallCommandCalls: make([]GetServerInstallCommandCall, 0),
	}
}

func (m *MockServerPlatformStrategy) InstallServer(server string, options types.ServerInstallOptions) (*types.InstallResult, error) {
	m.InstallServerCalls = append(m.InstallServerCalls, ServerInstallCall{Server: server, Options: options})
	if m.InstallServerFunc != nil {
		return m.InstallServerFunc(server, options)
	}

	return &types.InstallResult{
		Success:  true,
		Runtime:  server,
		Version:  options.Version,
		Path:     "/usr/local/bin/" + server,
		Method:   "platform_strategy",
		Duration: time.Second * 5,
		Errors:   []string{},
		Warnings: []string{},
		Messages: []string{"Installed " + server + " via platform strategy"},
		Details:  map[string]interface{}{"strategy": "mock_server"},
	}, nil
}

func (m *MockServerPlatformStrategy) VerifyServer(server string) (*types.VerificationResult, error) {
	m.VerifyServerCalls = append(m.VerifyServerCalls, server)
	if m.VerifyServerFunc != nil {
		return m.VerifyServerFunc(server)
	}

	return &types.VerificationResult{
		Installed:       true,
		Compatible:      true,
		Version:         "1.0.0",
		Path:            "/usr/local/bin/" + server,
		Runtime:         server,
		Issues:          []types.Issue{},
		Details:         map[string]interface{}{"server_strategy_verified": true},
		Metadata:        map[string]interface{}{"platform_strategy": true},
		EnvironmentVars: map[string]string{},
		Recommendations: []string{},
		WorkingDir:      "/tmp",
		AdditionalPaths: []string{},
		VerifiedAt:      time.Now(),
		Duration:        time.Millisecond * 75,
	}, nil
}

func (m *MockServerPlatformStrategy) GetInstallCommand(server, version string) ([]string, error) {
	m.GetInstallCommandCalls = append(m.GetInstallCommandCalls, GetServerInstallCommandCall{Server: server, Version: version})
	if m.GetInstallCommandFunc != nil {
		return m.GetInstallCommandFunc(server, version)
	}
	return []string{"install", server, version}, nil
}
