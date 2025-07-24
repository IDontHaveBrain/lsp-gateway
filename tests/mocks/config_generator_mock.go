package mocks

import (
	"context"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/setup"
	"time"
)

type MockConfigGenerator struct {
	GenerateFromDetectedFunc func(ctx context.Context) (*setup.ConfigGenerationResult, error)
	GenerateForRuntimeFunc   func(ctx context.Context, runtime string) (*setup.ConfigGenerationResult, error)
	GenerateDefaultFunc      func() (*setup.ConfigGenerationResult, error)
	UpdateConfigFunc         func(existing *config.GatewayConfig, updates setup.ConfigUpdates) (*setup.ConfigUpdateResult, error)
	ValidateConfigFunc       func(config *config.GatewayConfig) (*setup.ConfigValidationResult, error)
	ValidateGeneratedFunc    func(config *config.GatewayConfig) (*setup.ConfigValidationResult, error)
	SetLoggerFunc            func(logger *setup.SetupLogger)

	GenerateFromDetectedCalls []context.Context
	GenerateForRuntimeCalls   []GenerateForRuntimeCall
	GenerateDefaultCalls      int
	UpdateConfigCalls         []UpdateConfigCall
	ValidateConfigCalls       []*config.GatewayConfig
	ValidateGeneratedCalls    []*config.GatewayConfig
	SetLoggerCalls            []*setup.SetupLogger
}

type GenerateForRuntimeCall struct {
	Ctx     context.Context
	Runtime string
}

type UpdateConfigCall struct {
	Existing *config.GatewayConfig
	Updates  setup.ConfigUpdates
}

func NewMockConfigGenerator() *MockConfigGenerator {
	return &MockConfigGenerator{
		GenerateFromDetectedCalls: make([]context.Context, 0),
		GenerateForRuntimeCalls:   make([]GenerateForRuntimeCall, 0),
		UpdateConfigCalls:         make([]UpdateConfigCall, 0),
		ValidateConfigCalls:       make([]*config.GatewayConfig, 0),
		ValidateGeneratedCalls:    make([]*config.GatewayConfig, 0),
		SetLoggerCalls:            make([]*setup.SetupLogger, 0),
	}
}

func (m *MockConfigGenerator) GenerateFromDetected(ctx context.Context) (*setup.ConfigGenerationResult, error) {
	m.GenerateFromDetectedCalls = append(m.GenerateFromDetectedCalls, ctx)
	if m.GenerateFromDetectedFunc != nil {
		return m.GenerateFromDetectedFunc(ctx)
	}

	return &setup.ConfigGenerationResult{
		Config: &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{
					Name:      "go-lsp",
					Languages: []string{"go"},
					Command:   "gopls",
					Args:      []string{},
					Transport: "stdio",
				},
				{
					Name:      "python-lsp",
					Languages: []string{"python"},
					Command:   "python",
					Args:      []string{"-m", "pylsp"},
					Transport: "stdio",
				},
				{
					Name:      "typescript-lsp",
					Languages: []string{"typescript", "javascript"},
					Command:   "typescript-language-server",
					Args:      []string{"--stdio"},
					Transport: "stdio",
				},
				{
					Name:      "java-lsp",
					Languages: []string{"java"},
					Command:   "java",
					Args:      []string{"-jar", "/usr/local/share/jdtls/jdtls.jar"},
					Transport: "stdio",
				},
			},
		},
		DetectionReport:  nil, // Can be set via mock function if needed
		ServersGenerated: 4,
		ServersSkipped:   0,
		AutoDetected:     true,
		GeneratedAt:      time.Now(),
		Duration:         time.Millisecond * 500,
		Messages:         []string{"Generated configuration from detected runtimes"},
		Warnings:         []string{},
		Issues:           []string{},
		Metadata:         map[string]interface{}{"auto_detected": true, "mock": true},
	}, nil
}

func (m *MockConfigGenerator) GenerateForRuntime(ctx context.Context, runtime string) (*setup.ConfigGenerationResult, error) {
	m.GenerateForRuntimeCalls = append(m.GenerateForRuntimeCalls, GenerateForRuntimeCall{Ctx: ctx, Runtime: runtime})
	if m.GenerateForRuntimeFunc != nil {
		return m.GenerateForRuntimeFunc(ctx, runtime)
	}

	serverConfigs := map[string]config.ServerConfig{
		"go": {
			Name:      "go-lsp",
			Languages: []string{"go"},
			Command:   "gopls",
			Args:      []string{},
			Transport: "stdio",
		},
		"python": {
			Name:      "python-lsp",
			Languages: []string{"python"},
			Command:   "python",
			Args:      []string{"-m", "pylsp"},
			Transport: "stdio",
		},
		"nodejs": {
			Name:      "typescript-lsp",
			Languages: []string{"typescript", "javascript"},
			Command:   "typescript-language-server",
			Args:      []string{"--stdio"},
			Transport: "stdio",
		},
		"java": {
			Name:      "java-lsp",
			Languages: []string{"java"},
			Command:   "java",
			Args:      []string{"-jar", "/usr/local/share/jdtls/jdtls.jar"},
			Transport: "stdio",
		},
	}

	var servers []config.ServerConfig
	if serverConfig, exists := serverConfigs[runtime]; exists {
		servers = []config.ServerConfig{serverConfig}
	}

	return &setup.ConfigGenerationResult{
		Config: &config.GatewayConfig{
			Port:    8080,
			Servers: servers,
		},
		DetectionReport:  nil,
		ServersGenerated: len(servers),
		ServersSkipped:   0,
		AutoDetected:     false,
		GeneratedAt:      time.Now(),
		Duration:         time.Millisecond * 200,
		Messages:         []string{"Generated configuration for runtime: " + runtime},
		Warnings:         []string{},
		Issues:           []string{},
		Metadata:         map[string]interface{}{"runtime": runtime, "mock": true},
	}, nil
}

func (m *MockConfigGenerator) GenerateDefault() (*setup.ConfigGenerationResult, error) {
	m.GenerateDefaultCalls++
	if m.GenerateDefaultFunc != nil {
		return m.GenerateDefaultFunc()
	}

	return &setup.ConfigGenerationResult{
		Config: &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{
					Name:      "go-lsp",
					Languages: []string{"go"},
					Command:   "gopls",
					Args:      []string{},
					Transport: "stdio",
				},
			},
		},
		DetectionReport:  nil,
		ServersGenerated: 1,
		ServersSkipped:   0,
		AutoDetected:     false,
		GeneratedAt:      time.Now(),
		Duration:         time.Millisecond * 100,
		Messages:         []string{"Generated default configuration"},
		Warnings:         []string{},
		Issues:           []string{},
		Metadata:         map[string]interface{}{"default": true, "mock": true},
	}, nil
}

func (m *MockConfigGenerator) UpdateConfig(existing *config.GatewayConfig, updates setup.ConfigUpdates) (*setup.ConfigUpdateResult, error) {
	m.UpdateConfigCalls = append(m.UpdateConfigCalls, UpdateConfigCall{Existing: existing, Updates: updates})
	if m.UpdateConfigFunc != nil {
		return m.UpdateConfigFunc(existing, updates)
	}

	updatedConfig := &config.GatewayConfig{
		Port:    existing.Port,
		Servers: make([]config.ServerConfig, len(existing.Servers)),
	}
	copy(updatedConfig.Servers, existing.Servers)

	summary := setup.ConfigUpdatesSummary{
		PortChanged:        false,
		ServersAdded:       0,
		ServersRemoved:     0,
		ServersUpdated:     0,
		ServersReplaced:    false,
		AddedServerNames:   []string{},
		RemovedServerNames: []string{},
		UpdatedServerNames: []string{},
	}

	if updates.Port != 0 && updates.Port != existing.Port {
		updatedConfig.Port = updates.Port
		summary.PortChanged = true
	}

	if len(updates.AddServers) > 0 {
		updatedConfig.Servers = append(updatedConfig.Servers, updates.AddServers...)
		summary.ServersAdded = len(updates.AddServers)
		for _, server := range updates.AddServers {
			summary.AddedServerNames = append(summary.AddedServerNames, server.Name)
		}
	}

	if len(updates.RemoveServers) > 0 {
		var filteredServers []config.ServerConfig
		for _, server := range updatedConfig.Servers {
			shouldRemove := false
			for _, removeName := range updates.RemoveServers {
				if server.Name == removeName {
					shouldRemove = true
					summary.RemovedServerNames = append(summary.RemovedServerNames, server.Name)
					break
				}
			}
			if !shouldRemove {
				filteredServers = append(filteredServers, server)
			}
		}
		updatedConfig.Servers = filteredServers
		summary.ServersRemoved = len(updates.RemoveServers)
	}

	return &setup.ConfigUpdateResult{
		Config:         updatedConfig,
		UpdatesApplied: summary,
		UpdatedAt:      time.Now(),
		Duration:       time.Millisecond * 150,
		Messages:       []string{"Configuration updated successfully"},
		Warnings:       []string{},
		Issues:         []string{},
		Metadata:       map[string]interface{}{"mock_update": true},
	}, nil
}

func (m *MockConfigGenerator) ValidateConfig(config *config.GatewayConfig) (*setup.ConfigValidationResult, error) {
	m.ValidateConfigCalls = append(m.ValidateConfigCalls, config)
	if m.ValidateConfigFunc != nil {
		return m.ValidateConfigFunc(config)
	}

	var issues []string
	var warnings []string
	serverIssues := make(map[string][]string)

	if config.Port <= 0 || config.Port > 65535 {
		issues = append(issues, "Invalid port number")
	}

	if len(config.Servers) == 0 {
		warnings = append(warnings, "No servers configured")
	}

	for _, server := range config.Servers {
		var serverIssueList []string
		if server.Name == "" {
			serverIssueList = append(serverIssueList, "Server name is required")
		}
		if server.Command == "" {
			serverIssueList = append(serverIssueList, "Server command is required")
		}
		if len(server.Languages) == 0 {
			serverIssueList = append(serverIssueList, "At least one language must be specified")
		}
		if len(serverIssueList) > 0 {
			serverIssues[server.Name] = serverIssueList
			issues = append(issues, "Server "+server.Name+" has configuration issues")
		}
	}

	return &setup.ConfigValidationResult{
		Valid:            len(issues) == 0,
		Issues:           issues,
		Warnings:         warnings,
		ServersValidated: len(config.Servers),
		ServerIssues:     serverIssues,
		ValidatedAt:      time.Now(),
		Duration:         time.Millisecond * 50,
		Metadata:         map[string]interface{}{"mock_validation": true},
	}, nil
}

func (m *MockConfigGenerator) ValidateGenerated(config *config.GatewayConfig) (*setup.ConfigValidationResult, error) {
	m.ValidateGeneratedCalls = append(m.ValidateGeneratedCalls, config)
	if m.ValidateGeneratedFunc != nil {
		return m.ValidateGeneratedFunc(config)
	}

	result, err := m.ValidateConfig(config)
	if err != nil {
		return nil, err
	}

	result.Metadata["generated_validation"] = true
	return result, nil
}

func (m *MockConfigGenerator) SetLogger(logger *setup.SetupLogger) {
	m.SetLoggerCalls = append(m.SetLoggerCalls, logger)
	if m.SetLoggerFunc != nil {
		m.SetLoggerFunc(logger)
	}
}

func (m *MockConfigGenerator) Reset() {
	m.GenerateFromDetectedCalls = make([]context.Context, 0)
	m.GenerateForRuntimeCalls = make([]GenerateForRuntimeCall, 0)
	m.GenerateDefaultCalls = 0
	m.UpdateConfigCalls = make([]UpdateConfigCall, 0)
	m.ValidateConfigCalls = make([]*config.GatewayConfig, 0)
	m.ValidateGeneratedCalls = make([]*config.GatewayConfig, 0)
	m.SetLoggerCalls = make([]*setup.SetupLogger, 0)

	m.GenerateFromDetectedFunc = nil
	m.GenerateForRuntimeFunc = nil
	m.GenerateDefaultFunc = nil
	m.UpdateConfigFunc = nil
	m.ValidateConfigFunc = nil
	m.ValidateGeneratedFunc = nil
	m.SetLoggerFunc = nil
}
