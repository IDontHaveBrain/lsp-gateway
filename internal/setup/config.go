package setup

import (
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"time"
)

type ServerVerifier interface {
	VerifyServer(serverName string) (*ServerVerificationResult, error)
}

type ServerRegistry interface {
	GetServersByRuntime(runtime string) []*ServerDefinition
	GetServer(serverName string) (*ServerDefinition, error)
}

type ServerDefinition struct {
	Name          string                 `json:"name"`
	DisplayName   string                 `json:"display_name"`
	Languages     []string               `json:"languages"`
	DefaultConfig map[string]interface{} `json:"default_config,omitempty"`
}

type ServerVerificationResult struct {
	Installed  bool   `json:"installed"`
	Compatible bool   `json:"compatible"`
	Functional bool   `json:"functional"`
	Path       string `json:"path,omitempty"`
}

type ConfigGenerator interface {
	GenerateFromDetected(ctx context.Context) (*ConfigGenerationResult, error)

	GenerateForRuntime(ctx context.Context, runtime string) (*ConfigGenerationResult, error)

	GenerateDefault() (*ConfigGenerationResult, error)

	UpdateConfig(existing *config.GatewayConfig, updates ConfigUpdates) (*ConfigUpdateResult, error)

	ValidateConfig(config *config.GatewayConfig) (*ConfigValidationResult, error)

	ValidateGenerated(config *config.GatewayConfig) (*ConfigValidationResult, error)

	SetLogger(logger *SetupLogger)
}

type ConfigGenerationResult struct {
	Config           *config.GatewayConfig  `json:"config"`
	DetectionReport  *DetectionReport       `json:"detection_report,omitempty"`
	ServersGenerated int                    `json:"servers_generated"`
	ServersSkipped   int                    `json:"servers_skipped"`
	AutoDetected     bool                   `json:"auto_detected"`
	GeneratedAt      time.Time              `json:"generated_at"`
	Duration         time.Duration          `json:"duration"`
	Messages         []string               `json:"messages"`
	Warnings         []string               `json:"warnings"`
	Issues           []string               `json:"issues"`
	Metadata         map[string]interface{} `json:"metadata"`
}

type ConfigValidationResult struct {
	Valid            bool                   `json:"valid"`
	Issues           []string               `json:"issues"`
	Warnings         []string               `json:"warnings"`
	ServersValidated int                    `json:"servers_validated"`
	ServerIssues     map[string][]string    `json:"server_issues"`
	ValidatedAt      time.Time              `json:"validated_at"`
	Duration         time.Duration          `json:"duration"`
	Metadata         map[string]interface{} `json:"metadata"`
}

type ConfigUpdates struct {
	Port int `json:"port,omitempty"`

	AddServers []config.ServerConfig `json:"add_servers,omitempty"`

	RemoveServers []string `json:"remove_servers,omitempty"`

	UpdateServers []config.ServerConfig `json:"update_servers,omitempty"`

	ReplaceAllServers bool `json:"replace_all_servers,omitempty"`

	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ConfigUpdateResult struct {
	Config         *config.GatewayConfig  `json:"config"`
	UpdatesApplied ConfigUpdatesSummary   `json:"updates_applied"`
	UpdatedAt      time.Time              `json:"updated_at"`
	Duration       time.Duration          `json:"duration"`
	Messages       []string               `json:"messages"`
	Warnings       []string               `json:"warnings"`
	Issues         []string               `json:"issues"`
	Metadata       map[string]interface{} `json:"metadata"`
}

type ConfigUpdatesSummary struct {
	PortChanged        bool     `json:"port_changed"`
	ServersAdded       int      `json:"servers_added"`
	ServersRemoved     int      `json:"servers_removed"`
	ServersUpdated     int      `json:"servers_updated"`
	ServersReplaced    bool     `json:"servers_replaced"`
	AddedServerNames   []string `json:"added_server_names"`
	RemovedServerNames []string `json:"removed_server_names"`
	UpdatedServerNames []string `json:"updated_server_names"`
}

type ServerConfigTemplate struct {
	RuntimeName       string   `json:"runtime_name"`
	ServerName        string   `json:"server_name"`
	ConfigName        string   `json:"config_name"`
	Command           string   `json:"command"`
	Args              []string `json:"args"`
	Languages         []string `json:"languages"`
	Transport         string   `json:"transport"`
	RequiredRuntime   string   `json:"required_runtime"`
	MinRuntimeVersion string   `json:"min_runtime_version"`
}

type DefaultConfigGenerator struct {
	runtimeDetector RuntimeDetector
	serverVerifier  ServerVerifier
	serverRegistry  ServerRegistry
	logger          *SetupLogger
	templates       map[string]*ServerConfigTemplate
}

func NewConfigGenerator() *DefaultConfigGenerator {
	generator := &DefaultConfigGenerator{
		runtimeDetector: NewRuntimeDetector(),
		serverVerifier:  NewDefaultServerVerifier(),
		serverRegistry:  NewDefaultServerRegistry(),
		logger:          NewSetupLogger(nil),
		templates:       make(map[string]*ServerConfigTemplate),
	}

	generator.initializeTemplates()

	return generator
}

func NewConfigGeneratorWithDependencies(detector RuntimeDetector, verifier ServerVerifier, registry ServerRegistry) *DefaultConfigGenerator {
	generator := &DefaultConfigGenerator{
		runtimeDetector: detector,
		serverVerifier:  verifier,
		serverRegistry:  registry,
		logger:          NewSetupLogger(nil),
		templates:       make(map[string]*ServerConfigTemplate),
	}

	generator.initializeTemplates()
	return generator
}

func (g *DefaultConfigGenerator) SetLogger(logger *SetupLogger) {
	if logger != nil {
		g.logger = logger
	}
}

func (g *DefaultConfigGenerator) GenerateFromDetected(ctx context.Context) (*ConfigGenerationResult, error) {
	startTime := time.Now()

	g.logger.WithOperation("generate-config-from-detected").Info("Starting configuration generation from detected runtimes")

	result := &ConfigGenerationResult{
		Config:           nil,
		DetectionReport:  nil,
		ServersGenerated: 0,
		ServersSkipped:   0,
		AutoDetected:     true,
		GeneratedAt:      startTime,
		Messages:         []string{},
		Warnings:         []string{},
		Issues:           []string{},
		Metadata:         make(map[string]interface{}),
	}

	g.logger.UserInfo("Detecting installed runtimes...")
	detectionReport, err := g.runtimeDetector.DetectAll(ctx)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Runtime detection failed: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("runtime detection failed: %w", err)
	}

	result.DetectionReport = detectionReport
	result.Messages = append(result.Messages,
		fmt.Sprintf("Detected %d runtimes (%d installed, %d compatible)",
			detectionReport.Summary.TotalRuntimes,
			detectionReport.Summary.InstalledRuntimes,
			detectionReport.Summary.CompatibleRuntimes))

	gatewayConfig := &config.GatewayConfig{
		Port:    8080, // Default port
		Servers: []config.ServerConfig{},
	}

	g.logger.UserInfo("Generating server configurations...")

	for runtimeName, runtimeInfo := range detectionReport.Runtimes {
		if !runtimeInfo.Installed || !runtimeInfo.Compatible {
			g.logger.WithField("runtime", runtimeName).Debug("Skipping incompatible runtime")
			result.Messages = append(result.Messages,
				fmt.Sprintf("Skipped %s: not installed or incompatible", runtimeName))
			continue
		}

		servers := g.serverRegistry.GetServersByRuntime(runtimeName)
		for _, serverDef := range servers {
			serverConfig, err := g.generateServerConfig(ctx, runtimeInfo, serverDef)
			if err != nil {
				g.logger.WithError(err).WithFields(map[string]interface{}{
					"runtime": runtimeName,
					"server":  serverDef.Name,
				}).Warn("Failed to generate server configuration")

				result.Issues = append(result.Issues,
					fmt.Sprintf("Failed to generate config for %s: %v", serverDef.Name, err))
				result.ServersSkipped++
				continue
			}

			if serverConfig != nil {
				gatewayConfig.Servers = append(gatewayConfig.Servers, *serverConfig)
				result.ServersGenerated++
				result.Messages = append(result.Messages,
					fmt.Sprintf("Generated configuration for %s (%s)", serverDef.DisplayName, serverDef.Name))
			} else {
				result.ServersSkipped++
				result.Messages = append(result.Messages,
					fmt.Sprintf("Skipped %s: server not available", serverDef.DisplayName))
			}
		}
	}

	if len(gatewayConfig.Servers) == 0 {
		result.Warnings = append(result.Warnings, "No servers were auto-detected, adding default Go server")
		defaultConfig := g.createDefaultGoServer()
		gatewayConfig.Servers = append(gatewayConfig.Servers, defaultConfig)
		result.ServersGenerated = 1
	}

	if err := gatewayConfig.Validate(); err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Generated configuration is invalid: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("generated configuration validation failed: %w", err)
	}

	result.Config = gatewayConfig
	result.Duration = time.Since(startTime)

	result.Metadata["detection_summary"] = detectionReport.Summary
	result.Metadata["generation_method"] = "auto_detected"
	result.Metadata["total_runtimes_checked"] = detectionReport.Summary.TotalRuntimes
	result.Metadata["compatible_runtimes"] = detectionReport.Summary.CompatibleRuntimes

	g.logger.UserSuccess(fmt.Sprintf("Configuration generated successfully: %d servers configured", result.ServersGenerated))
	g.logger.WithFields(map[string]interface{}{
		"servers_generated": result.ServersGenerated,
		"servers_skipped":   result.ServersSkipped,
		"duration":          result.Duration,
	}).Info("Configuration generation completed")

	return result, nil
}

func (g *DefaultConfigGenerator) GenerateForRuntime(ctx context.Context, runtime string) (*ConfigGenerationResult, error) {
	startTime := time.Now()

	g.logger.WithOperation("generate-config-runtime").WithField("runtime", runtime).Info("Generating configuration for specific runtime")

	result := &ConfigGenerationResult{
		Config:           nil,
		ServersGenerated: 0,
		ServersSkipped:   0,
		AutoDetected:     false,
		GeneratedAt:      startTime,
		Messages:         []string{},
		Warnings:         []string{},
		Issues:           []string{},
		Metadata:         make(map[string]interface{}),
	}

	var runtimeInfo *RuntimeInfo
	var err error

	switch runtime {
	case "go":
		runtimeInfo, err = g.runtimeDetector.DetectGo(ctx)
	case "python":
		runtimeInfo, err = g.runtimeDetector.DetectPython(ctx)
	case "nodejs":
		runtimeInfo, err = g.runtimeDetector.DetectNodejs(ctx)
	case "java":
		runtimeInfo, err = g.runtimeDetector.DetectJava(ctx)
	default:
		result.Issues = append(result.Issues, fmt.Sprintf("Unsupported runtime: %s", runtime))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Failed to detect %s runtime: %v", runtime, err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("runtime detection failed: %w", err)
	}

	gatewayConfig := &config.GatewayConfig{
		Port:    8080,
		Servers: []config.ServerConfig{},
	}

	if runtimeInfo.Installed && runtimeInfo.Compatible {
		servers := g.serverRegistry.GetServersByRuntime(runtime)
		for _, serverDef := range servers {
			serverConfig, err := g.generateServerConfig(ctx, runtimeInfo, serverDef)
			if err != nil {
				result.Issues = append(result.Issues,
					fmt.Sprintf("Failed to generate config for %s: %v", serverDef.Name, err))
				result.ServersSkipped++
				continue
			}

			if serverConfig != nil {
				gatewayConfig.Servers = append(gatewayConfig.Servers, *serverConfig)
				result.ServersGenerated++
				result.Messages = append(result.Messages,
					fmt.Sprintf("Generated configuration for %s", serverDef.DisplayName))
			} else {
				result.ServersSkipped++
			}
		}
	} else {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%s runtime is not installed or compatible", runtime))
	}

	if len(gatewayConfig.Servers) > 0 {
		if err := gatewayConfig.Validate(); err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Generated configuration is invalid: %v", err))
			result.Duration = time.Since(startTime)
			return result, fmt.Errorf("configuration validation failed: %w", err)
		}
	} else {
		result.Warnings = append(result.Warnings, fmt.Sprintf("No servers configured for %s runtime", runtime))
	}

	result.Config = gatewayConfig
	result.Duration = time.Since(startTime)
	result.Metadata["target_runtime"] = runtime
	result.Metadata["runtime_installed"] = runtimeInfo.Installed
	result.Metadata["runtime_compatible"] = runtimeInfo.Compatible
	result.Metadata["runtime_version"] = runtimeInfo.Version

	return result, nil
}

func (g *DefaultConfigGenerator) GenerateDefault() (*ConfigGenerationResult, error) {
	startTime := time.Now()

	result := &ConfigGenerationResult{
		Config:           config.DefaultConfig(),
		ServersGenerated: 1, // Default config has one Go server
		ServersSkipped:   0,
		AutoDetected:     false,
		GeneratedAt:      startTime,
		Duration:         time.Since(startTime),
		Messages:         []string{"Generated default configuration with Go language server"},
		Warnings:         []string{},
		Issues:           []string{},
		Metadata: map[string]interface{}{
			"generation_method": "default",
			"default_server":    "go-lsp",
		},
	}

	return result, nil
}

func (g *DefaultConfigGenerator) UpdateConfig(existing *config.GatewayConfig, updates ConfigUpdates) (*ConfigUpdateResult, error) {
	startTime := time.Now()
	g.logger.WithOperation("update-config").Info("Starting configuration update")

	result := g.initializeUpdateResult(startTime)

	if existing == nil {
		return g.handleNilUpdateConfig(result, startTime)
	}

	updatedConfig := g.cloneExistingConfig(existing)
	g.applyPortUpdates(existing, updates, updatedConfig, result)
	g.applyServerUpdates(updates, updatedConfig, result)

	if err := g.validateUpdatedConfig(updatedConfig, result, startTime); err != nil {
		return result, err
	}

	g.finalizeUpdateResult(existing, updatedConfig, updates, result, startTime)
	g.logUpdateCompletion(result)

	return result, nil
}

func (g *DefaultConfigGenerator) initializeUpdateResult(startTime time.Time) *ConfigUpdateResult {
	return &ConfigUpdateResult{
		Config:         nil,
		UpdatesApplied: ConfigUpdatesSummary{},
		UpdatedAt:      startTime,
		Messages:       []string{},
		Warnings:       []string{},
		Issues:         []string{},
		Metadata:       make(map[string]interface{}),
	}
}

func (g *DefaultConfigGenerator) handleNilUpdateConfig(result *ConfigUpdateResult, startTime time.Time) (*ConfigUpdateResult, error) {
	result.Issues = append(result.Issues, "Existing configuration cannot be nil")
	result.Duration = time.Since(startTime)
	return result, fmt.Errorf("existing configuration cannot be nil")
}

func (g *DefaultConfigGenerator) cloneExistingConfig(existing *config.GatewayConfig) *config.GatewayConfig {
	updatedConfig := &config.GatewayConfig{
		Port:    existing.Port,
		Servers: make([]config.ServerConfig, len(existing.Servers)),
	}
	copy(updatedConfig.Servers, existing.Servers)
	return updatedConfig
}

func (g *DefaultConfigGenerator) applyPortUpdates(existing *config.GatewayConfig, updates ConfigUpdates, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	if updates.Port > 0 && updates.Port != existing.Port {
		updatedConfig.Port = updates.Port
		result.UpdatesApplied.PortChanged = true
		result.Messages = append(result.Messages, fmt.Sprintf("Updated port from %d to %d", existing.Port, updates.Port))
	}
}

func (g *DefaultConfigGenerator) applyServerUpdates(updates ConfigUpdates, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	if updates.ReplaceAllServers {
		g.replaceAllServers(updates, updatedConfig, result)
	} else {
		g.applyIncrementalServerUpdates(updates, updatedConfig, result)
	}
}

func (g *DefaultConfigGenerator) replaceAllServers(updates ConfigUpdates, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	updatedConfig.Servers = make([]config.ServerConfig, len(updates.AddServers))
	copy(updatedConfig.Servers, updates.AddServers)

	result.UpdatesApplied.ServersReplaced = true
	result.UpdatesApplied.ServersAdded = len(updates.AddServers)
	result.Messages = append(result.Messages, fmt.Sprintf("Replaced all servers with %d new servers", len(updates.AddServers)))

	for _, server := range updates.AddServers {
		result.UpdatesApplied.AddedServerNames = append(result.UpdatesApplied.AddedServerNames, server.Name)
	}
}

func (g *DefaultConfigGenerator) applyIncrementalServerUpdates(updates ConfigUpdates, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	g.removeServers(updates.RemoveServers, updatedConfig, result)
	g.updateServers(updates.UpdateServers, updatedConfig, result)
	g.addServers(updates.AddServers, updatedConfig, result)
}

func (g *DefaultConfigGenerator) removeServers(removeServers []string, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	for _, serverName := range removeServers {
		g.removeServer(serverName, updatedConfig, result)
	}
}

func (g *DefaultConfigGenerator) removeServer(serverName string, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	newServers := []config.ServerConfig{}
	removed := false

	for _, server := range updatedConfig.Servers {
		if server.Name != serverName {
			newServers = append(newServers, server)
		} else {
			removed = true
		}
	}

	if removed {
		updatedConfig.Servers = newServers
		result.UpdatesApplied.ServersRemoved++
		result.UpdatesApplied.RemovedServerNames = append(result.UpdatesApplied.RemovedServerNames, serverName)
		result.Messages = append(result.Messages, fmt.Sprintf("Removed server: %s", serverName))
	} else {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s not found for removal", serverName))
	}
}

func (g *DefaultConfigGenerator) updateServers(updateServers []config.ServerConfig, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	for _, updateServer := range updateServers {
		g.updateServer(updateServer, updatedConfig, result)
	}
}

func (g *DefaultConfigGenerator) updateServer(updateServer config.ServerConfig, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	updated := false
	for i, server := range updatedConfig.Servers {
		if server.Name == updateServer.Name {
			updatedConfig.Servers[i] = updateServer
			updated = true
			result.UpdatesApplied.ServersUpdated++
			result.UpdatesApplied.UpdatedServerNames = append(result.UpdatesApplied.UpdatedServerNames, updateServer.Name)
			result.Messages = append(result.Messages, fmt.Sprintf("Updated server: %s", updateServer.Name))
			break
		}
	}

	if !updated {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s not found for update", updateServer.Name))
	}
}

func (g *DefaultConfigGenerator) addServers(addServers []config.ServerConfig, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	for _, addServer := range addServers {
		g.addServer(addServer, updatedConfig, result)
	}
}

func (g *DefaultConfigGenerator) addServer(addServer config.ServerConfig, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	for _, server := range updatedConfig.Servers {
		if server.Name == addServer.Name {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s already exists, skipping add", addServer.Name))
			return
		}
	}

	updatedConfig.Servers = append(updatedConfig.Servers, addServer)
	result.UpdatesApplied.ServersAdded++
	result.UpdatesApplied.AddedServerNames = append(result.UpdatesApplied.AddedServerNames, addServer.Name)
	result.Messages = append(result.Messages, fmt.Sprintf("Added server: %s", addServer.Name))
}

func (g *DefaultConfigGenerator) validateUpdatedConfig(updatedConfig *config.GatewayConfig, result *ConfigUpdateResult, startTime time.Time) error {
	if err := updatedConfig.Validate(); err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Updated configuration is invalid: %v", err))
		result.Duration = time.Since(startTime)
		return fmt.Errorf("updated configuration validation failed: %w", err)
	}
	return nil
}

func (g *DefaultConfigGenerator) finalizeUpdateResult(existing, updatedConfig *config.GatewayConfig, updates ConfigUpdates, result *ConfigUpdateResult, startTime time.Time) {
	result.Config = updatedConfig
	result.Duration = time.Since(startTime)

	result.Metadata["original_servers"] = len(existing.Servers)
	result.Metadata["final_servers"] = len(updatedConfig.Servers)
	result.Metadata["original_port"] = existing.Port
	result.Metadata["final_port"] = updatedConfig.Port
	result.Metadata["update_method"] = "merge"
	if updates.ReplaceAllServers {
		result.Metadata["update_method"] = "replace_all"
	}
}

func (g *DefaultConfigGenerator) logUpdateCompletion(result *ConfigUpdateResult) {
	g.logger.UserSuccess(fmt.Sprintf("Configuration updated successfully: %d messages, %d warnings", len(result.Messages), len(result.Warnings)))
	g.logger.WithFields(map[string]interface{}{
		"servers_added":   result.UpdatesApplied.ServersAdded,
		"servers_removed": result.UpdatesApplied.ServersRemoved,
		"servers_updated": result.UpdatesApplied.ServersUpdated,
		"port_changed":    result.UpdatesApplied.PortChanged,
		"duration":        result.Duration,
	}).Info("Configuration update completed")
}

func (g *DefaultConfigGenerator) ValidateConfig(config *config.GatewayConfig) (*ConfigValidationResult, error) {
	startTime := time.Now()
	g.logger.WithOperation("validate-config").Info("Starting configuration validation")

	result := g.initializeValidationResult(startTime)

	if config == nil {
		return g.handleNilConfig(result, startTime)
	}

	g.validateBasicConfiguration(config, result)
	g.validatePortConfiguration(config, result)
	g.validateServerConfigurations(config, result)
	languageToServers := g.buildLanguageServerMapping(config)
	g.validateLanguageMapping(languageToServers, result)
	g.validateServerPresence(config, result)

	g.finalizeValidationResult(config, result, languageToServers, startTime)
	g.logValidationCompletion(result)

	return result, nil
}

func (g *DefaultConfigGenerator) initializeValidationResult(startTime time.Time) *ConfigValidationResult {
	return &ConfigValidationResult{
		Valid:            true,
		Issues:           []string{},
		Warnings:         []string{},
		ServersValidated: 0,
		ServerIssues:     make(map[string][]string),
		ValidatedAt:      startTime,
		Metadata:         make(map[string]interface{}),
	}
}

func (g *DefaultConfigGenerator) handleNilConfig(result *ConfigValidationResult, startTime time.Time) (*ConfigValidationResult, error) {
	result.Valid = false
	result.Issues = append(result.Issues, "Configuration cannot be nil")
	result.Duration = time.Since(startTime)
	return result, fmt.Errorf("configuration cannot be nil")
}

func (g *DefaultConfigGenerator) validateBasicConfiguration(config *config.GatewayConfig, result *ConfigValidationResult) {
	if err := config.Validate(); err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Basic validation failed: %v", err))
	}
}

func (g *DefaultConfigGenerator) validatePortConfiguration(config *config.GatewayConfig, result *ConfigValidationResult) {
	if config.Port < 1 || config.Port > 65535 {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Invalid port %d: must be between 1 and 65535", config.Port))
		return
	}

	g.checkCommonPortWarnings(config.Port, result)
}

func (g *DefaultConfigGenerator) checkCommonPortWarnings(port int, result *ConfigValidationResult) {
	commonPorts := map[int]string{
		22:    "SSH",
		80:    "HTTP",
		443:   "HTTPS",
		3000:  "Common development server",
		5432:  "PostgreSQL",
		3306:  "MySQL",
		6379:  "Redis",
		27017: "MongoDB",
	}

	if service, exists := commonPorts[port]; exists {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Port %d is commonly used by %s", port, service))
	}
}

func (g *DefaultConfigGenerator) validateServerConfigurations(config *config.GatewayConfig, result *ConfigValidationResult) {
	for _, serverConfig := range config.Servers {
		result.ServersValidated++
		serverIssues := g.validateIndividualServer(serverConfig, result)

		if len(serverIssues) > 0 {
			result.ServerIssues[serverConfig.Name] = serverIssues
		}
	}
}

func (g *DefaultConfigGenerator) validateIndividualServer(serverConfig config.ServerConfig, result *ConfigValidationResult) []string {
	var serverIssues []string

	serverIssues = append(serverIssues, g.validateServerBasics(serverConfig, result)...)
	serverIssues = append(serverIssues, g.validateServerLanguages(serverConfig)...)
	g.validateServerInstallation(serverConfig, result)

	return serverIssues
}

func (g *DefaultConfigGenerator) validateServerBasics(serverConfig config.ServerConfig, result *ConfigValidationResult) []string {
	var issues []string

	if err := serverConfig.Validate(); err != nil {
		issues = append(issues, fmt.Sprintf("Server validation failed: %v", err))
		result.Valid = false
	}

	if len(serverConfig.Languages) == 0 {
		issues = append(issues, "No languages specified")
	}

	return issues
}

func (g *DefaultConfigGenerator) validateServerLanguages(serverConfig config.ServerConfig) []string {
	var issues []string
	langMap := make(map[string]bool)

	for _, lang := range serverConfig.Languages {
		if langMap[lang] {
			issues = append(issues, fmt.Sprintf("Duplicate language: %s", lang))
		}
		langMap[lang] = true
	}

	return issues
}

func (g *DefaultConfigGenerator) validateServerInstallation(serverConfig config.ServerConfig, result *ConfigValidationResult) {
	if g.serverVerifier == nil {
		return
	}

	verificationResult, err := g.serverVerifier.VerifyServer(serverConfig.Command)
	if err != nil {
		return
	}

	g.processServerVerificationResult(verificationResult, serverConfig, result)
}

func (g *DefaultConfigGenerator) processServerVerificationResult(verificationResult *ServerVerificationResult, serverConfig config.ServerConfig, result *ConfigValidationResult) {
	if !verificationResult.Installed {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s (%s) is not installed", serverConfig.Name, serverConfig.Command))
	}
	if !verificationResult.Compatible {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s may not be compatible", serverConfig.Name))
	}
	if !verificationResult.Functional {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s may not be functional", serverConfig.Name))
	}
}

func (g *DefaultConfigGenerator) buildLanguageServerMapping(config *config.GatewayConfig) map[string][]string {
	languageToServers := make(map[string][]string)
	for _, serverConfig := range config.Servers {
		for _, lang := range serverConfig.Languages {
			languageToServers[lang] = append(languageToServers[lang], serverConfig.Name)
		}
	}
	return languageToServers
}

func (g *DefaultConfigGenerator) validateLanguageMapping(languageToServers map[string][]string, result *ConfigValidationResult) {
	for lang, servers := range languageToServers {
		if len(servers) > 1 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Language %s is supported by multiple servers: %v (first one will be used)", lang, servers))
		}
	}
}

func (g *DefaultConfigGenerator) validateServerPresence(config *config.GatewayConfig, result *ConfigValidationResult) {
	if len(config.Servers) == 0 {
		result.Valid = false
		result.Issues = append(result.Issues, "No servers configured")
	}
}

func (g *DefaultConfigGenerator) finalizeValidationResult(config *config.GatewayConfig, result *ConfigValidationResult, languageToServers map[string][]string, startTime time.Time) {
	result.Duration = time.Since(startTime)

	result.Metadata["total_servers"] = len(config.Servers)
	result.Metadata["total_languages"] = len(languageToServers)
	result.Metadata["port"] = config.Port
	result.Metadata["validation_type"] = "comprehensive"
}

func (g *DefaultConfigGenerator) logValidationCompletion(result *ConfigValidationResult) {
	if result.Valid {
		g.logger.UserSuccess("Configuration validation passed")
	} else {
		g.logger.UserError(fmt.Sprintf("Configuration validation failed: %d issues found", len(result.Issues)))
	}

	g.logger.WithFields(map[string]interface{}{
		"valid":             result.Valid,
		"servers_validated": result.ServersValidated,
		"issues":            len(result.Issues),
		"warnings":          len(result.Warnings),
		"duration":          result.Duration,
	}).Info("Configuration validation completed")
}

func (g *DefaultConfigGenerator) ValidateGenerated(config *config.GatewayConfig) (*ConfigValidationResult, error) {
	startTime := time.Now()

	result := &ConfigValidationResult{
		Valid:            true,
		Issues:           []string{},
		Warnings:         []string{},
		ServersValidated: 0,
		ServerIssues:     make(map[string][]string),
		ValidatedAt:      startTime,
		Metadata:         make(map[string]interface{}),
	}

	if err := config.Validate(); err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Configuration validation failed: %v", err))
	}

	for _, serverConfig := range config.Servers {
		result.ServersValidated++

		serverIssues := []string{}

		if serverDef, err := g.serverRegistry.GetServer(serverConfig.Command); err == nil {
			verificationResult, err := g.serverVerifier.VerifyServer(serverDef.Name)
			if err != nil {
				serverIssues = append(serverIssues, fmt.Sprintf("Verification failed: %v", err))
			} else {
				if !verificationResult.Installed {
					serverIssues = append(serverIssues, "Server not installed")
					result.Valid = false
				}
				if !verificationResult.Compatible {
					serverIssues = append(serverIssues, "Server version incompatible")
				}
				if !verificationResult.Functional {
					serverIssues = append(serverIssues, "Server not functional")
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("Server %s may not be functional", serverConfig.Name))
				}
			}
		} else {
			serverIssues = append(serverIssues, "Server definition not found")
		}

		if len(serverIssues) > 0 {
			result.ServerIssues[serverConfig.Name] = serverIssues
		}
	}

	result.Duration = time.Since(startTime)
	result.Metadata["validation_type"] = "generated_config"
	return result, nil
}

func (g *DefaultConfigGenerator) generateServerConfig(ctx context.Context, runtimeInfo *RuntimeInfo, serverDef *ServerDefinition) (*config.ServerConfig, error) {
	verificationResult, err := g.serverVerifier.VerifyServer(serverDef.Name)
	if err != nil {
		g.logger.WithError(err).WithField("server", serverDef.Name).Debug("Server verification failed")
		return nil, nil // Return nil to skip this server, not an error
	}

	if !verificationResult.Installed {
		g.logger.WithField("server", serverDef.Name).Debug("Server not installed")
		return nil, nil // Server not installed, skip it
	}

	serverConfig := &config.ServerConfig{
		Name:      serverDef.Name + "-lsp", // Generate config name from server name
		Languages: serverDef.Languages,
		Command:   serverDef.Name,
		Args:      []string{},
		Transport: "stdio", // Default transport
	}

	switch serverDef.Name {
	case SERVER_GOPLS:
		serverConfig.Args = []string{}

	case SERVER_PYLSP:
		serverConfig.Args = []string{}

	case "typescript-language-server":
		serverConfig.Args = []string{"--stdio"}

	case SERVER_JDTLS:
		if verificationResult.Path != "" {
			serverConfig.Command = verificationResult.Path
		}
		serverConfig.Args = []string{}
	}

	if serverDef.DefaultConfig != nil {
		if command, ok := serverDef.DefaultConfig["command"].(string); ok && command != "" {
			serverConfig.Command = command
		}
		if transport, ok := serverDef.DefaultConfig["transport"].(string); ok && transport != "" {
			serverConfig.Transport = transport
		}
		if args, ok := serverDef.DefaultConfig["args"].([]string); ok && len(args) > 0 {
			serverConfig.Args = args
		}
	}

	return serverConfig, nil
}

func (g *DefaultConfigGenerator) createDefaultGoServer() config.ServerConfig {
	return config.ServerConfig{
		Name:      "go-lsp",
		Languages: []string{"go"},
		Command:   SERVER_GOPLS,
		Args:      []string{},
		Transport: "stdio",
	}
}

func (g *DefaultConfigGenerator) initializeTemplates() {
	g.templates[SERVER_GOPLS] = &ServerConfigTemplate{
		RuntimeName:       "go",
		ServerName:        SERVER_GOPLS,
		ConfigName:        "go-lsp",
		Command:           SERVER_GOPLS,
		Args:              []string{},
		Languages:         []string{"go"},
		Transport:         "stdio",
		RequiredRuntime:   "go",
		MinRuntimeVersion: "1.19.0",
	}

	g.templates[SERVER_PYLSP] = &ServerConfigTemplate{
		RuntimeName:       "python",
		ServerName:        SERVER_PYLSP,
		ConfigName:        "python-lsp",
		Command:           SERVER_PYLSP,
		Args:              []string{},
		Languages:         []string{"python"},
		Transport:         "stdio",
		RequiredRuntime:   "python",
		MinRuntimeVersion: "3.8.0",
	}

	g.templates["typescript-language-server"] = &ServerConfigTemplate{
		RuntimeName:       "nodejs",
		ServerName:        "typescript-language-server",
		ConfigName:        "typescript-lsp",
		Command:           "typescript-language-server",
		Args:              []string{"--stdio"},
		Languages:         []string{"typescript", "javascript"},
		Transport:         "stdio",
		RequiredRuntime:   "nodejs",
		MinRuntimeVersion: "18.0.0",
	}

	g.templates[SERVER_JDTLS] = &ServerConfigTemplate{
		RuntimeName:       "java",
		ServerName:        SERVER_JDTLS,
		ConfigName:        "java-lsp",
		Command:           SERVER_JDTLS,
		Args:              []string{},
		Languages:         []string{"java"},
		Transport:         "stdio",
		RequiredRuntime:   "java",
		MinRuntimeVersion: "17.0.0",
	}
}

func (g *DefaultConfigGenerator) GetSupportedRuntimes() []string {
	return []string{"go", "python", "nodejs", "java"}
}

func (g *DefaultConfigGenerator) GetSupportedServers() []string {
	servers := make([]string, 0, len(g.templates))
	for serverName := range g.templates {
		servers = append(servers, serverName)
	}
	return servers
}

func (g *DefaultConfigGenerator) GetTemplate(serverName string) (*ServerConfigTemplate, error) {
	if template, exists := g.templates[serverName]; exists {
		return template, nil
	}
	return nil, fmt.Errorf("template not found for server: %s", serverName)
}

type DefaultServerVerifier struct{}

func NewDefaultServerVerifier() *DefaultServerVerifier {
	return &DefaultServerVerifier{}
}

func (v *DefaultServerVerifier) VerifyServer(serverName string) (*ServerVerificationResult, error) {
	result := &ServerVerificationResult{
		Installed:  true, // Assume installed for basic implementation
		Compatible: true, // Assume compatible
		Functional: true, // Assume functional
		Path:       "",   // Path not determined in basic implementation
	}
	return result, nil
}

type DefaultServerRegistry struct {
	servers map[string]*ServerDefinition
}

func NewDefaultServerRegistry() *DefaultServerRegistry {
	registry := &DefaultServerRegistry{
		servers: make(map[string]*ServerDefinition),
	}

	registry.initializeServers()

	return registry
}

func (r *DefaultServerRegistry) initializeServers() {
	r.servers[SERVER_GOPLS] = &ServerDefinition{
		Name:        SERVER_GOPLS,
		DisplayName: "Go Language Server",
		Languages:   []string{"go"},
		DefaultConfig: map[string]interface{}{
			"command":   SERVER_GOPLS,
			"transport": "stdio",
			"args":      []string{},
		},
	}

	r.servers[SERVER_PYLSP] = &ServerDefinition{
		Name:        SERVER_PYLSP,
		DisplayName: "Python Language Server",
		Languages:   []string{"python"},
		DefaultConfig: map[string]interface{}{
			"command":   SERVER_PYLSP,
			"transport": "stdio",
			"args":      []string{},
		},
	}

	r.servers["typescript-language-server"] = &ServerDefinition{
		Name:        "typescript-language-server",
		DisplayName: "TypeScript Language Server",
		Languages:   []string{"typescript", "javascript"},
		DefaultConfig: map[string]interface{}{
			"command":   "typescript-language-server",
			"transport": "stdio",
			"args":      []string{"--stdio"},
		},
	}

	r.servers[SERVER_JDTLS] = &ServerDefinition{
		Name:        SERVER_JDTLS,
		DisplayName: "Java Language Server",
		Languages:   []string{"java"},
		DefaultConfig: map[string]interface{}{
			"command":   SERVER_JDTLS,
			"transport": "stdio",
			"args":      []string{},
		},
	}
}

func (r *DefaultServerRegistry) GetServersByRuntime(runtime string) []*ServerDefinition {
	var servers []*ServerDefinition

	runtimeToServers := map[string][]string{
		"go":     {SERVER_GOPLS},
		"python": {SERVER_PYLSP},
		"nodejs": {"typescript-language-server"},
		"java":   {SERVER_JDTLS},
	}

	serverNames, exists := runtimeToServers[runtime]
	if !exists {
		return servers
	}

	for _, serverName := range serverNames {
		if server, exists := r.servers[serverName]; exists {
			servers = append(servers, server)
		}
	}

	return servers
}

func (r *DefaultServerRegistry) GetServer(serverName string) (*ServerDefinition, error) {
	if server, exists := r.servers[serverName]; exists {
		return server, nil
	}
	return nil, fmt.Errorf("server not found: %s", serverName)
}
