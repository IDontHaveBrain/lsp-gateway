package diagnostics

import (
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/setup"
	"lsp-gateway/internal/types"
	"os"
	"path/filepath"
	"time"
)

type DiagnosticChecker interface {
	CheckSystemHealth() (*HealthReport, error)

	CheckRuntimeHealth() (*RuntimeReport, error)

	CheckServerHealth() (*ServerReport, error)

	CheckConfigHealth() (*ConfigReport, error)

	RunDiagnostics() (*DiagnosticReport, error)
}

type HealthReport struct {
	Timestamp       time.Time
	Overall         HealthStatus
	Platform        *PlatformHealth
	Runtimes        *RuntimeHealth
	Servers         *ServerHealth
	Config          *ConfigHealth
	Network         *NetworkHealth
	Issues          []DiagnosticIssue
	Recommendations []Recommendation
}

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusCritical  HealthStatus = "critical"
)

type PlatformHealth struct {
	Status          HealthStatus
	OS              string
	Architecture    string
	Distribution    string
	PackageManagers []string
	Issues          []string
}

type RuntimeHealth struct {
	Status   HealthStatus
	Runtimes map[string]*RuntimeHealthInfo
	Issues   []string
}

type RuntimeHealthInfo struct {
	Status     HealthStatus
	Installed  bool
	Version    string
	Compatible bool
	Path       string
	Working    bool
	Issues     []string
}

type ServerHealth struct {
	Status  HealthStatus
	Servers map[string]*ServerHealthInfo
	Issues  []string
}

type ServerHealthInfo struct {
	Status          HealthStatus
	Installed       bool
	Version         string
	RuntimeCheck    bool
	ExecutableCheck bool
	Communication   bool
	Issues          []string
}

type ConfigHealth struct {
	Status       HealthStatus
	FileExists   bool
	Syntax       bool
	Schema       bool
	ServerConfig bool
	Issues       []string
}

type NetworkHealth struct {
	Status           HealthStatus
	InternetAccess   bool
	RepositoryAccess bool
	ProxyDetected    bool
	Issues           []string
}

type DiagnosticIssue struct {
	installer.Issue
	Component string `json:"component"` // Additional field for component tracking
}

type IssueSeverity = installer.IssueSeverity
type IssueCategory = installer.IssueCategory

const (
	IssueSeverityInfo     = installer.IssueSeverityInfo
	IssueSeverityWarning  = installer.IssueSeverityMedium // Map warning to medium
	IssueSeverityError    = installer.IssueSeverityHigh   // Map error to high
	IssueSeverityCritical = installer.IssueSeverityCritical
)

const (
	IssueCategoryPlatform   IssueCategory = "platform"
	IssueCategoryRuntime    IssueCategory = "runtime"
	IssueCategoryServer     IssueCategory = "server"
	IssueCategoryConfig     IssueCategory = "config"
	IssueCategoryNetwork    IssueCategory = "network"
	IssueCategoryPermission IssueCategory = "permission"
	IssueCategoryDependency IssueCategory = "dependency"
)

type Recommendation struct {
	Priority    RecommendationPriority
	Category    IssueCategory
	Title       string
	Description string
	Commands    []string
	AutoFix     bool
}

type RecommendationPriority string

const (
	RecommendationPriorityLow      RecommendationPriority = "low"
	RecommendationPriorityMedium   RecommendationPriority = "medium"
	RecommendationPriorityHigh     RecommendationPriority = "high"
	RecommendationPriorityCritical RecommendationPriority = "critical"
)

type RuntimeReport struct {
	Timestamp       time.Time
	Overall         HealthStatus
	Runtimes        map[string]*RuntimeHealthInfo
	Issues          []DiagnosticIssue
	Recommendations []Recommendation
}

type ServerReport struct {
	Timestamp       time.Time
	Overall         HealthStatus
	Servers         map[string]*ServerHealthInfo
	Issues          []DiagnosticIssue
	Recommendations []Recommendation
}

type ConfigReport struct {
	Timestamp       time.Time
	Overall         HealthStatus
	Config          *ConfigHealth
	Issues          []DiagnosticIssue
	Recommendations []Recommendation
}

type DiagnosticReport struct {
	Timestamp     time.Time
	Overall       HealthStatus
	SystemHealth  *HealthReport
	RuntimeReport *RuntimeReport
	ServerReport  *ServerReport
	ConfigReport  *ConfigReport
	Summary       *DiagnosticSummary
}

type DiagnosticSummary struct {
	TotalIssues          int
	CriticalIssues       int
	ErrorIssues          int
	WarningIssues        int
	InfoIssues           int
	TotalRecommendations int
	AutoFixAvailable     int
	HealthyComponents    int
	UnhealthyComponents  int
}

type DefaultDiagnosticChecker struct {
	runtimeDetector  *setup.DefaultRuntimeDetector
	serverVerifier   *installer.DefaultServerVerifier
	platformDetector platform.CommandExecutor
	packageManagers  []platform.PackageManager
	configPath       string
}

func NewDiagnosticChecker() *DefaultDiagnosticChecker {
	runtimeDetector := setup.NewRuntimeDetector()

	runtimeInstaller := installer.NewRuntimeInstaller()
	serverVerifier := installer.NewServerVerifier(runtimeInstaller)

	platformExecutor := platform.NewCommandExecutor()
	packageManagers := platform.GetAvailablePackageManagers()

	return &DefaultDiagnosticChecker{
		runtimeDetector:  runtimeDetector,
		serverVerifier:   serverVerifier,
		platformDetector: platformExecutor,
		packageManagers:  packageManagers,
		configPath:       "config.yaml", // Default config path
	}
}

func NewDiagnosticCheckerWithConfig(configPath string) *DefaultDiagnosticChecker {
	checker := NewDiagnosticChecker()
	checker.configPath = configPath
	return checker
}

func (d *DefaultDiagnosticChecker) CheckSystemHealth() (*HealthReport, error) {
	startTime := time.Now()

	report := &HealthReport{
		Timestamp:       startTime,
		Platform:        &PlatformHealth{},
		Runtimes:        &RuntimeHealth{Runtimes: make(map[string]*RuntimeHealthInfo)},
		Servers:         &ServerHealth{Servers: make(map[string]*ServerHealthInfo)},
		Config:          &ConfigHealth{},
		Network:         &NetworkHealth{},
		Issues:          []DiagnosticIssue{},
		Recommendations: []Recommendation{},
	}

	d.checkPlatformHealth(report)

	d.checkSystemResources(report)

	d.checkPackageManagers(report)

	d.checkNetworkHealth(report)

	d.calculateOverallHealth(report)

	return report, nil
}

func (d *DefaultDiagnosticChecker) CheckRuntimeHealth() (*RuntimeReport, error) {
	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report := &RuntimeReport{
		Timestamp:       startTime,
		Runtimes:        make(map[string]*RuntimeHealthInfo),
		Issues:          []DiagnosticIssue{},
		Recommendations: []Recommendation{},
	}

	detectionReport, err := d.runtimeDetector.DetectAll(ctx)
	if err != nil {
		d.addRuntimeIssue(report, IssueSeverityCritical, IssueCategoryRuntime,
			"Runtime Detection Failed",
			fmt.Sprintf("Failed to detect runtimes: %v", err),
			"Check system permissions and ensure runtime commands are available in PATH")
		report.Overall = HealthStatusCritical
		return report, nil
	}

	for runtimeName, runtimeInfo := range detectionReport.Runtimes {
		healthInfo := d.convertRuntimeInfoToHealth(runtimeInfo)
		report.Runtimes[runtimeName] = healthInfo
	}

	d.generateRuntimeRecommendations(report)

	d.calculateRuntimeOverallHealth(report)

	return report, nil
}

func (d *DefaultDiagnosticChecker) CheckServerHealth() (*ServerReport, error) {
	startTime := time.Now()

	report := &ServerReport{
		Timestamp:       startTime,
		Servers:         make(map[string]*ServerHealthInfo),
		Issues:          []DiagnosticIssue{},
		Recommendations: []Recommendation{},
	}

	supportedServers := d.serverVerifier.GetSupportedServers()
	if len(supportedServers) == 0 {
		d.addServerIssue(report, IssueSeverityWarning, IssueCategoryServer,
			"No Supported Servers",
			"No language servers are registered for verification",
			"Register language servers in the server registry")
		report.Overall = HealthStatusDegraded
		return report, nil
	}

	verificationResults, err := d.serverVerifier.VerifyAllServers()
	if err != nil {
		d.addServerIssue(report, IssueSeverityCritical, IssueCategoryServer,
			"Server Verification Failed",
			fmt.Sprintf("Failed to verify language servers: %v", err),
			"Check server installations and system dependencies")
		report.Overall = HealthStatusCritical
		return report, nil
	}

	for serverName, verifyResult := range verificationResults {
		healthInfo := d.convertServerVerificationToHealth(verifyResult)
		report.Servers[serverName] = healthInfo

		for _, issue := range verifyResult.Issues {
			d.addServerIssue(report, d.mapIssueSeverity(issue.Severity),
				d.mapIssueCategory(issue.Category), issue.Title, issue.Description, issue.Solution)
		}

		for _, rec := range verifyResult.Recommendations {
			d.addServerRecommendation(report, RecommendationPriorityMedium, IssueCategoryServer,
				fmt.Sprintf("%s Recommendation", serverName), rec, []string{}, false)
		}
	}

	d.calculateServerOverallHealth(report)

	return report, nil
}

func (d *DefaultDiagnosticChecker) CheckConfigHealth() (*ConfigReport, error) {
	startTime := time.Now()

	report := &ConfigReport{
		Timestamp:       startTime,
		Config:          &ConfigHealth{},
		Issues:          []DiagnosticIssue{},
		Recommendations: []Recommendation{},
	}

	configPath := d.configPath
	if configPath == "" {
		configPath = "config.yaml"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		report.Config.FileExists = false
		d.addConfigIssue(report, IssueSeverityError, IssueCategoryConfig,
			"Configuration File Missing",
			fmt.Sprintf("Configuration file not found at: %s", configPath),
			"Create a configuration file using: lsp-gateway config generate")
		d.addConfigRecommendation(report, RecommendationPriorityHigh, IssueCategoryConfig,
			"Generate Configuration", "Create a default configuration file",
			[]string{"lsp-gateway config generate"}, true)
		report.Config.Status = HealthStatusUnhealthy
		report.Overall = HealthStatusUnhealthy
		return report, nil
	}

	report.Config.FileExists = true

	gatewayConfig, err := config.LoadConfig(configPath)
	if err != nil {
		report.Config.Syntax = false
		d.addConfigIssue(report, IssueSeverityError, IssueCategoryConfig,
			"Configuration Load Failed",
			fmt.Sprintf("Failed to load configuration: %v", err),
			"Check configuration file syntax and format")
		report.Config.Status = HealthStatusUnhealthy
		report.Overall = HealthStatusUnhealthy
		return report, nil
	}

	report.Config.Syntax = true

	if err := config.ValidateConfig(gatewayConfig); err != nil {
		report.Config.Schema = false
		d.addConfigIssue(report, IssueSeverityError, IssueCategoryConfig,
			"Configuration Validation Failed",
			fmt.Sprintf("Configuration validation failed: %v", err),
			"Fix configuration file according to schema requirements")
		report.Config.Status = HealthStatusUnhealthy
		report.Overall = HealthStatusUnhealthy
		return report, nil
	}

	report.Config.Schema = true

	if len(gatewayConfig.Servers) == 0 {
		report.Config.ServerConfig = false
		d.addConfigIssue(report, IssueSeverityWarning, IssueCategoryConfig,
			"No Servers Configured",
			"No language servers are configured in the configuration file",
			"Add language server configurations to enable LSP functionality")
		d.addConfigRecommendation(report, RecommendationPriorityMedium, IssueCategoryConfig,
			"Add Server Configuration", "Configure at least one language server",
			[]string{"lsp-gateway install servers"}, true)
		report.Config.Status = HealthStatusDegraded
		report.Overall = HealthStatusDegraded
	} else {
		report.Config.ServerConfig = true

		serverIssues := d.validateServerConfigurations(gatewayConfig.Servers)
		if len(serverIssues) > 0 {
			for _, issue := range serverIssues {
				d.addConfigIssue(report, IssueSeverityWarning, IssueCategoryConfig,
					"Server Configuration Issue", issue,
					"Review and fix server configuration")
			}
			if report.Config.Status != HealthStatusUnhealthy {
				report.Config.Status = HealthStatusDegraded
				report.Overall = HealthStatusDegraded
			}
		}
	}

	if report.Config.Status == "" {
		report.Config.Status = HealthStatusHealthy
		report.Overall = HealthStatusHealthy
	}

	return report, nil
}

func (d *DefaultDiagnosticChecker) RunDiagnostics() (*DiagnosticReport, error) {
	startTime := time.Now()

	systemHealth, err := d.CheckSystemHealth()
	if err != nil {
		return nil, fmt.Errorf("system health check failed: %w", err)
	}

	runtimeReport, err := d.CheckRuntimeHealth()
	if err != nil {
		return nil, fmt.Errorf("runtime health check failed: %w", err)
	}

	serverReport, err := d.CheckServerHealth()
	if err != nil {
		return nil, fmt.Errorf("server health check failed: %w", err)
	}

	configReport, err := d.CheckConfigHealth()
	if err != nil {
		return nil, fmt.Errorf("config health check failed: %w", err)
	}

	report := &DiagnosticReport{
		Timestamp:     startTime,
		SystemHealth:  systemHealth,
		RuntimeReport: runtimeReport,
		ServerReport:  serverReport,
		ConfigReport:  configReport,
	}

	summary := d.calculateDiagnosticSummary(systemHealth, runtimeReport, serverReport, configReport)
	report.Summary = summary

	report.Overall = d.determineOverallDiagnosticHealth(systemHealth, runtimeReport, serverReport, configReport)

	return report, nil
}

func (d *DefaultDiagnosticChecker) checkPlatformHealth(report *HealthReport) {
	currentPlatform := platform.GetCurrentPlatform()
	currentArch := platform.GetCurrentArchitecture()

	report.Platform.OS = currentPlatform.String()
	report.Platform.Architecture = currentArch.String()

	if platform.IsLinux() {
		if linuxInfo, err := platform.DetectLinuxDistribution(); err == nil {
			report.Platform.Distribution = string(linuxInfo.Distribution)
			report.Platform.PackageManagers = platform.GetPreferredPackageManagers(linuxInfo.Distribution)
		} else {
			report.Platform.Issues = append(report.Platform.Issues,
				fmt.Sprintf("Could not detect Linux distribution: %v", err))
		}
	}

	supportedPlatforms := []string{"linux", "darwin", "windows"}
	supported := false
	for _, p := range supportedPlatforms {
		if p == currentPlatform.String() {
			supported = true
			break
		}
	}

	if !supported {
		report.Platform.Status = HealthStatusUnhealthy
		report.Platform.Issues = append(report.Platform.Issues,
			fmt.Sprintf("Platform %s/%s is not officially supported", currentPlatform, currentArch))
		d.addIssue(report, IssueSeverityCritical, IssueCategoryPlatform,
			"Unsupported Platform",
			fmt.Sprintf("Platform %s/%s is not officially supported", currentPlatform, currentArch),
			"Use a supported platform: Linux, macOS, or Windows")
	} else {
		report.Platform.Status = HealthStatusHealthy
	}
}

func (d *DefaultDiagnosticChecker) checkSystemResources(report *HealthReport) {
	cwd, err := os.Getwd()
	if err != nil {
		d.addIssue(report, IssueSeverityError, IssueCategoryPermission,
			"Working Directory Access",
			fmt.Sprintf("Cannot access current working directory: %v", err),
			"Check directory permissions and access rights")
		return
	}

	testFile := filepath.Join(cwd, ".lsp-gateway-test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		d.addIssue(report, IssueSeverityError, IssueCategoryPermission,
			"Write Permission",
			fmt.Sprintf("Cannot write to current directory %s: %v", cwd, err),
			"Ensure write permissions for the current directory")
	} else {
		if err := os.Remove(testFile); err != nil {
		}
	}

	if homeDir, err := platform.GetHomeDirectory(); err != nil {
		d.addIssue(report, IssueSeverityWarning, IssueCategoryPlatform,
			"Home Directory Access",
			fmt.Sprintf("Cannot determine home directory: %v", err),
			"Check HOME environment variable and directory permissions")
	} else {
		if _, err := os.Stat(homeDir); err != nil {
			d.addIssue(report, IssueSeverityWarning, IssueCategoryPlatform,
				"Home Directory Missing",
				fmt.Sprintf("Home directory does not exist: %s", homeDir),
				"Check home directory configuration")
		}
	}

	tempDir := platform.GetTempDirectory()
	if _, err := os.Stat(tempDir); err != nil {
		d.addIssue(report, IssueSeverityWarning, IssueCategoryPlatform,
			"Temp Directory Access",
			fmt.Sprintf("Cannot access temp directory %s: %v", tempDir, err),
			"Check temp directory configuration and permissions")
	}
}

func (d *DefaultDiagnosticChecker) checkPackageManagers(report *HealthReport) {
	if len(d.packageManagers) == 0 {
		d.addIssue(report, IssueSeverityWarning, IssueCategoryDependency,
			"No Package Managers",
			"No package managers detected on the system",
			"Install a package manager for automatic dependency installation")
		d.addRecommendation(report, RecommendationPriorityMedium, IssueCategoryDependency,
			"Install Package Manager",
			"Install a package manager for your platform",
			[]string{"Linux: apt/yum/dnf", "macOS: brew", "Windows: winget/choco"}, false)
		return
	}

	var managers []string
	for _, mgr := range d.packageManagers {
		managers = append(managers, mgr.GetName())
	}
	report.Platform.PackageManagers = managers

	bestMgr := platform.GetBestPackageManager()
	if bestMgr != nil && bestMgr.RequiresAdmin() {
		d.addIssue(report, IssueSeverityInfo, IssueCategoryPermission,
			"Admin Privileges Required",
			fmt.Sprintf("Package manager %s requires administrative privileges", bestMgr.GetName()),
			"Be prepared to provide admin credentials when installing packages")
	}
}

func (d *DefaultDiagnosticChecker) checkNetworkHealth(report *HealthReport) {
	report.Network.Status = HealthStatusHealthy
	report.Network.InternetAccess = true   // Assume true for now
	report.Network.RepositoryAccess = true // Assume true for now
	report.Network.ProxyDetected = false

	proxyVars := []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"}
	for _, proxyVar := range proxyVars {
		if proxy := os.Getenv(proxyVar); proxy != "" {
			report.Network.ProxyDetected = true
			report.Network.Issues = append(report.Network.Issues,
				fmt.Sprintf("Proxy detected: %s=%s", proxyVar, proxy))
			d.addIssue(report, IssueSeverityInfo, IssueCategoryNetwork,
				"Proxy Configuration Detected",
				fmt.Sprintf("Proxy configuration found: %s", proxy),
				"Ensure proxy configuration is correct for network access")
			break
		}
	}
}

func (d *DefaultDiagnosticChecker) calculateOverallHealth(report *HealthReport) {
	criticalIssues := 0
	errorIssues := 0
	warningIssues := 0

	for _, issue := range report.Issues {
		switch issue.Severity {
		case IssueSeverityCritical:
			criticalIssues++
		case IssueSeverityError:
			errorIssues++
		case IssueSeverityWarning:
			warningIssues++
		}
	}

	if criticalIssues > 0 {
		report.Overall = HealthStatusCritical
	} else if errorIssues > 0 {
		report.Overall = HealthStatusUnhealthy
	} else if warningIssues > 0 {
		report.Overall = HealthStatusDegraded
	} else {
		report.Overall = HealthStatusHealthy
	}
}

func (d *DefaultDiagnosticChecker) convertRuntimeInfoToHealth(runtimeInfo *setup.RuntimeInfo) *RuntimeHealthInfo {
	status := HealthStatusHealthy
	if !runtimeInfo.Installed {
		status = HealthStatusUnhealthy
	} else if !runtimeInfo.Compatible {
		status = HealthStatusDegraded
	}

	return &RuntimeHealthInfo{
		Status:     status,
		Installed:  runtimeInfo.Installed,
		Version:    runtimeInfo.Version,
		Compatible: runtimeInfo.Compatible,
		Path:       runtimeInfo.Path,
		Working:    runtimeInfo.Installed && len(runtimeInfo.Issues) == 0,
		Issues:     runtimeInfo.Issues,
	}
}

func (d *DefaultDiagnosticChecker) generateRuntimeRecommendations(report *RuntimeReport) {
	runtimeInstallCommands := map[string]string{
		"go":     "lsp-gateway install runtime go",
		"python": "lsp-gateway install runtime python",
		"nodejs": "lsp-gateway install runtime nodejs",
		"java":   "lsp-gateway install runtime java",
	}

	for runtimeName, healthInfo := range report.Runtimes {
		if !healthInfo.Installed {
			if installCmd, exists := runtimeInstallCommands[runtimeName]; exists {
				d.addRuntimeRecommendation(report, RecommendationPriorityHigh, IssueCategoryRuntime,
					fmt.Sprintf("Install %s Runtime", runtimeName),
					fmt.Sprintf("Install %s runtime for language server support", runtimeName),
					[]string{installCmd}, true)
			}
		} else if !healthInfo.Compatible {
			d.addRuntimeRecommendation(report, RecommendationPriorityMedium, IssueCategoryRuntime,
				fmt.Sprintf("Upgrade %s Runtime", runtimeName),
				fmt.Sprintf("Upgrade %s to a compatible version", runtimeName),
				[]string{fmt.Sprintf("Upgrade %s to the latest version", runtimeName)}, false)
		}
	}
}

func (d *DefaultDiagnosticChecker) calculateRuntimeOverallHealth(report *RuntimeReport) {
	if len(report.Runtimes) == 0 {
		report.Overall = HealthStatusCritical
		return
	}

	installedCount := 0
	compatibleCount := 0
	unhealthyCount := 0

	for _, healthInfo := range report.Runtimes {
		if healthInfo.Installed {
			installedCount++
		}
		if healthInfo.Compatible {
			compatibleCount++
		}
		if healthInfo.Status == HealthStatusUnhealthy || healthInfo.Status == HealthStatusCritical {
			unhealthyCount++
		}
	}

	if installedCount == 0 {
		report.Overall = HealthStatusCritical
	} else if unhealthyCount > 0 || compatibleCount < installedCount {
		report.Overall = HealthStatusDegraded
	} else {
		report.Overall = HealthStatusHealthy
	}
}

func (d *DefaultDiagnosticChecker) convertServerVerificationToHealth(verifyResult *installer.ServerVerificationResult) *ServerHealthInfo {
	status := HealthStatusHealthy
	if !verifyResult.Installed {
		status = HealthStatusUnhealthy
	} else if !verifyResult.Compatible || !verifyResult.Functional {
		status = HealthStatusDegraded
	}

	return &ServerHealthInfo{
		Status:          status,
		Installed:       verifyResult.Installed,
		Version:         verifyResult.Version,
		RuntimeCheck:    verifyResult.RuntimeStatus != nil && verifyResult.RuntimeStatus.Compatible,
		ExecutableCheck: verifyResult.Installed,
		Communication:   verifyResult.Functional,
		Issues:          d.convertInstallerIssuesToStrings(verifyResult.Issues),
	}
}

func (d *DefaultDiagnosticChecker) calculateServerOverallHealth(report *ServerReport) {
	if len(report.Servers) == 0 {
		report.Overall = HealthStatusDegraded
		return
	}

	installedCount := 0
	functionalCount := 0
	unhealthyCount := 0

	for _, healthInfo := range report.Servers {
		if healthInfo.Installed {
			installedCount++
		}
		if healthInfo.Communication {
			functionalCount++
		}
		if healthInfo.Status == HealthStatusUnhealthy || healthInfo.Status == HealthStatusCritical {
			unhealthyCount++
		}
	}

	if installedCount == 0 {
		report.Overall = HealthStatusDegraded
	} else if unhealthyCount > 0 {
		report.Overall = HealthStatusDegraded
	} else if functionalCount == installedCount {
		report.Overall = HealthStatusHealthy
	} else {
		report.Overall = HealthStatusDegraded
	}
}

func (d *DefaultDiagnosticChecker) validateServerConfigurations(servers []config.ServerConfig) []string {
	var issues []string

	for i, server := range servers {
		if server.Name == "" {
			issues = append(issues, fmt.Sprintf("Server %d: missing name", i))
		}
		if len(server.Languages) == 0 {
			issues = append(issues, fmt.Sprintf("Server %s: no languages configured", server.Name))
		}
		if server.Command == "" {
			issues = append(issues, fmt.Sprintf("Server %s: missing command", server.Name))
		}
		if server.Transport == "" {
			issues = append(issues, fmt.Sprintf("Server %s: missing transport", server.Name))
		}
	}

	return issues
}

func (d *DefaultDiagnosticChecker) calculateDiagnosticSummary(systemHealth *HealthReport, runtimeReport *RuntimeReport, serverReport *ServerReport, configReport *ConfigReport) *DiagnosticSummary {
	summary := &DiagnosticSummary{}

	allIssues := append(systemHealth.Issues, runtimeReport.Issues...)
	allIssues = append(allIssues, serverReport.Issues...)
	allIssues = append(allIssues, configReport.Issues...)

	summary.TotalIssues = len(allIssues)

	for _, issue := range allIssues {
		switch issue.Severity {
		case IssueSeverityCritical:
			summary.CriticalIssues++
		case IssueSeverityError:
			summary.ErrorIssues++
		case IssueSeverityWarning:
			summary.WarningIssues++
		case IssueSeverityInfo:
			summary.InfoIssues++
		}
	}

	allRecommendations := append(systemHealth.Recommendations, runtimeReport.Recommendations...)
	allRecommendations = append(allRecommendations, serverReport.Recommendations...)
	allRecommendations = append(allRecommendations, configReport.Recommendations...)

	summary.TotalRecommendations = len(allRecommendations)

	for _, rec := range allRecommendations {
		if rec.AutoFix {
			summary.AutoFixAvailable++
		}
	}

	components := []HealthStatus{
		systemHealth.Overall,
		runtimeReport.Overall,
		serverReport.Overall,
		configReport.Overall,
	}

	for _, status := range components {
		if status == HealthStatusHealthy {
			summary.HealthyComponents++
		} else {
			summary.UnhealthyComponents++
		}
	}

	return summary
}

func (d *DefaultDiagnosticChecker) determineOverallDiagnosticHealth(systemHealth *HealthReport, runtimeReport *RuntimeReport, serverReport *ServerReport, configReport *ConfigReport) HealthStatus {
	if systemHealth.Overall == HealthStatusCritical ||
		runtimeReport.Overall == HealthStatusCritical ||
		serverReport.Overall == HealthStatusCritical ||
		configReport.Overall == HealthStatusCritical {
		return HealthStatusCritical
	}

	if systemHealth.Overall == HealthStatusUnhealthy ||
		runtimeReport.Overall == HealthStatusUnhealthy ||
		serverReport.Overall == HealthStatusUnhealthy ||
		configReport.Overall == HealthStatusUnhealthy {
		return HealthStatusUnhealthy
	}

	if systemHealth.Overall == HealthStatusDegraded ||
		runtimeReport.Overall == HealthStatusDegraded ||
		serverReport.Overall == HealthStatusDegraded ||
		configReport.Overall == HealthStatusDegraded {
		return HealthStatusDegraded
	}

	return HealthStatusHealthy
}

func (d *DefaultDiagnosticChecker) addIssue(report *HealthReport, severity IssueSeverity, category IssueCategory, title, description, resolution string) {
	issue := DiagnosticIssue{
		Issue: installer.Issue{
			Severity:    severity,
			Category:    category,
			Title:       title,
			Description: description,
			Solution:    resolution,
			Details:     make(map[string]interface{}),
		},
		Component: "system",
	}
	report.Issues = append(report.Issues, issue)
}

func (d *DefaultDiagnosticChecker) addRecommendation(report *HealthReport, priority RecommendationPriority, category IssueCategory, title, description string, commands []string, autoFix bool) {
	recommendation := Recommendation{
		Priority:    priority,
		Category:    category,
		Title:       title,
		Description: description,
		Commands:    commands,
		AutoFix:     autoFix,
	}
	report.Recommendations = append(report.Recommendations, recommendation)
}

func (d *DefaultDiagnosticChecker) addRuntimeIssue(report *RuntimeReport, severity IssueSeverity, category IssueCategory, title, description, resolution string) {
	issue := DiagnosticIssue{
		Issue: installer.Issue{
			Severity:    severity,
			Category:    category,
			Title:       title,
			Description: description,
			Solution:    resolution,
			Details:     make(map[string]interface{}),
		},
		Component: "runtime",
	}
	report.Issues = append(report.Issues, issue)
}

func (d *DefaultDiagnosticChecker) addRuntimeRecommendation(report *RuntimeReport, priority RecommendationPriority, category IssueCategory, title, description string, commands []string, autoFix bool) {
	recommendation := Recommendation{
		Priority:    priority,
		Category:    category,
		Title:       title,
		Description: description,
		Commands:    commands,
		AutoFix:     autoFix,
	}
	report.Recommendations = append(report.Recommendations, recommendation)
}

func (d *DefaultDiagnosticChecker) addServerIssue(report *ServerReport, severity IssueSeverity, category IssueCategory, title, description, resolution string) {
	issue := DiagnosticIssue{
		Issue: installer.Issue{
			Severity:    severity,
			Category:    category,
			Title:       title,
			Description: description,
			Solution:    resolution,
			Details:     make(map[string]interface{}),
		},
		Component: "server",
	}
	report.Issues = append(report.Issues, issue)
}

func (d *DefaultDiagnosticChecker) addServerRecommendation(report *ServerReport, priority RecommendationPriority, category IssueCategory, title, description string, commands []string, autoFix bool) {
	recommendation := Recommendation{
		Priority:    priority,
		Category:    category,
		Title:       title,
		Description: description,
		Commands:    commands,
		AutoFix:     autoFix,
	}
	report.Recommendations = append(report.Recommendations, recommendation)
}

func (d *DefaultDiagnosticChecker) addConfigIssue(report *ConfigReport, severity IssueSeverity, category IssueCategory, title, description, resolution string) {
	issue := DiagnosticIssue{
		Issue: installer.Issue{
			Severity:    severity,
			Category:    category,
			Title:       title,
			Description: description,
			Solution:    resolution,
			Details:     make(map[string]interface{}),
		},
		Component: "config",
	}
	report.Issues = append(report.Issues, issue)
}

func (d *DefaultDiagnosticChecker) addConfigRecommendation(report *ConfigReport, priority RecommendationPriority, category IssueCategory, title, description string, commands []string, autoFix bool) {
	recommendation := Recommendation{
		Priority:    priority,
		Category:    category,
		Title:       title,
		Description: description,
		Commands:    commands,
		AutoFix:     autoFix,
	}
	report.Recommendations = append(report.Recommendations, recommendation)
}

func (d *DefaultDiagnosticChecker) mapIssueSeverity(severity installer.IssueSeverity) IssueSeverity {
	switch severity {
	case installer.IssueSeverityLow:
		return IssueSeverityInfo
	case installer.IssueSeverityMedium:
		return IssueSeverityWarning
	case installer.IssueSeverityHigh:
		return IssueSeverityError
	case installer.IssueSeverityCritical:
		return IssueSeverityCritical
	default:
		return IssueSeverityInfo
	}
}

func (d *DefaultDiagnosticChecker) mapIssueCategory(category installer.IssueCategory) IssueCategory {
	switch category {
	case installer.IssueCategoryInstallation:
		return IssueCategoryServer
	case installer.IssueCategoryDependencies:
		return IssueCategoryDependency
	case installer.IssueCategoryPermissions:
		return IssueCategoryPermission
	case installer.IssueCategoryPath:
		return IssueCategoryPlatform
	case installer.IssueCategoryEnvironment:
		return IssueCategoryPlatform
	case installer.IssueCategoryConfiguration:
		return IssueCategoryConfig
	case types.IssueCategoryExecution:
		return IssueCategoryRuntime
	default:
		return IssueCategoryPlatform
	}
}

func (d *DefaultDiagnosticChecker) convertInstallerIssuesToStrings(issues []installer.Issue) []string {
	var result []string
	for _, issue := range issues {
		result = append(result, issue.Description)
	}
	return result
}
