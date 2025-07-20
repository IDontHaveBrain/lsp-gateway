package diagnostics

import (
	"context"
	"fmt"
	"lsp-gateway/internal/types"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type StatusReporter interface {
	GetRuntimeStatus() (*RuntimeStatus, error)

	GetServerStatus() (*ServerStatus, error)

	GetSystemStatus() (*SystemStatus, error)

	GetConfigStatus() (*ConfigStatus, error)

	GetStatus(component string) (*ComponentStatus, error)
}

type RuntimeStatus struct {
	Timestamp time.Time                     `json:"timestamp"`
	Runtimes  map[string]*RuntimeStatusInfo `json:"runtimes"`
	Summary   RuntimeStatusSummary          `json:"summary"`
}

type RuntimeStatusInfo struct {
	Name        string            `json:"name"`
	Status      ComponentStatus   `json:"status"`
	Installed   bool              `json:"installed"`
	Version     string            `json:"version"`
	Path        string            `json:"path"`
	Compatible  bool              `json:"compatible"`
	Working     bool              `json:"working"`
	LastChecked time.Time         `json:"last_checked"`
	Issues      []string          `json:"issues"`
	Metadata    map[string]string `json:"metadata"`
}

type RuntimeStatusSummary struct {
	Total      int `json:"total"`
	Installed  int `json:"installed"`
	Compatible int `json:"compatible"`
	Working    int `json:"working"`
	Issues     int `json:"issues"`
}

type ServerStatus struct {
	Timestamp time.Time                    `json:"timestamp"`
	Servers   map[string]*ServerStatusInfo `json:"servers"`
	Summary   ServerStatusSummary          `json:"summary"`
}

type ServerStatusInfo struct {
	Name            string            `json:"name"`
	Status          ComponentStatus   `json:"status"`
	Installed       bool              `json:"installed"`
	Version         string            `json:"version"`
	Path            string            `json:"path"`
	Runtime         string            `json:"runtime"`
	RuntimeCheck    bool              `json:"runtime_check"`
	ExecutableCheck bool              `json:"executable_check"`
	Communication   bool              `json:"communication"`
	LastChecked     time.Time         `json:"last_checked"`
	Issues          []string          `json:"issues"`
	Metadata        map[string]string `json:"metadata"`
}

type ServerStatusSummary struct {
	Total        int `json:"total"`
	Installed    int `json:"installed"`
	Working      int `json:"working"`
	RuntimeReady int `json:"runtime_ready"`
	Issues       int `json:"issues"`
}

type SystemStatus struct {
	Timestamp time.Time           `json:"timestamp"`
	Overall   ComponentStatus     `json:"overall"`
	Platform  *PlatformStatus     `json:"platform"`
	Runtimes  *RuntimeStatus      `json:"runtimes"`
	Servers   *ServerStatus       `json:"servers"`
	Config    *ConfigStatus       `json:"config"`
	Network   *NetworkStatus      `json:"network"`
	Summary   SystemStatusSummary `json:"summary"`
}

type PlatformStatus struct {
	OS              string                 `json:"os"`
	Architecture    string                 `json:"architecture"`
	Distribution    string                 `json:"distribution"`
	Version         string                 `json:"version"`
	PackageManagers []PackageManagerStatus `json:"package_managers"`
	Status          ComponentStatus        `json:"status"`
	Issues          []string               `json:"issues"`
}

type PackageManagerStatus struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Working   bool   `json:"working"`
}

type ConfigStatus struct {
	Status       ComponentStatus `json:"status"`
	FileExists   bool            `json:"file_exists"`
	FilePath     string          `json:"file_path"`
	Syntax       bool            `json:"syntax"`
	Schema       bool            `json:"schema"`
	ServerCount  int             `json:"server_count"`
	Issues       []string        `json:"issues"`
	LastModified time.Time       `json:"last_modified"`
}

type NetworkStatus struct {
	Status           ComponentStatus `json:"status"`
	InternetAccess   bool            `json:"internet_access"`
	RepositoryAccess map[string]bool `json:"repository_access"` // repository URL -> accessible
	ProxyDetected    bool            `json:"proxy_detected"`
	ProxyConfig      string          `json:"proxy_config"`
	Issues           []string        `json:"issues"`
}

type ComponentStatus string

const (
	ComponentStatusHealthy      ComponentStatus = "healthy"
	ComponentStatusDegraded     ComponentStatus = "degraded"
	ComponentStatusUnhealthy    ComponentStatus = "unhealthy"
	ComponentStatusUnknown      ComponentStatus = "unknown"
	ComponentStatusNotInstalled ComponentStatus = "not_installed"
)

type SystemStatusSummary struct {
	HealthyComponents   []string `json:"healthy_components"`
	DegradedComponents  []string `json:"degraded_components"`
	UnhealthyComponents []string `json:"unhealthy_components"`
	TotalIssues         int      `json:"total_issues"`
	CriticalIssues      int      `json:"critical_issues"`
}

type DefaultStatusReporter struct {
	runtimeDetector RuntimeDetector
	serverInstaller ServerInstaller
	packageMgrs     []PackageManager
	configLoader    ConfigLoader
}

type RuntimeDetector interface {
	DetectAll(ctx context.Context) (*DetectionReport, error)
	DetectGo(ctx context.Context) (*RuntimeInfo, error)
	DetectPython(ctx context.Context) (*RuntimeInfo, error)
	DetectNodejs(ctx context.Context) (*RuntimeInfo, error)
	DetectJava(ctx context.Context) (*RuntimeInfo, error)
}

type ServerInstaller interface {
	Verify(server string) (*VerificationResult, error)
	GetSupportedServers() []string
	GetServerInfo(server string) (*ServerDefinition, error)
}

type PackageManager interface {
	IsAvailable() bool
	GetName() string
	GetVersion() (string, error)
	TestInstall() error
}

type ConfigLoader interface {
	LoadConfig(configPath string) (*GatewayConfig, error)
	ValidateConfig(config *GatewayConfig) error
}

type DetectionReport struct {
	Timestamp    time.Time
	Platform     interface{}
	Architecture interface{}
	Runtimes     map[string]*RuntimeInfo
	Summary      DetectionSummary
	Duration     time.Duration
	Issues       []string
	Warnings     []string
	SessionID    string
	Metadata     map[string]interface{}
}

type DetectionSummary struct {
	TotalRuntimes      int
	InstalledRuntimes  int
	CompatibleRuntimes int
	IssuesFound        int
	WarningsFound      int
	SuccessRate        float64
	AverageDetectTime  time.Duration
}

type RuntimeInfo struct {
	Name          string
	Installed     bool
	Version       string
	ParsedVersion interface{}
	Compatible    bool
	MinVersion    string
	Path          string
	WorkingDir    string
	DetectionCmd  string
	Issues        []string
	Warnings      []string
	Metadata      map[string]interface{}
	DetectedAt    time.Time
	Duration      time.Duration
}

type VerificationResult struct {
	Installed       bool
	Compatible      bool
	Version         string
	Path            string
	Issues          []types.Issue
	Recommendations []string
	VerifiedAt      time.Time
	Duration        time.Duration
	Metadata        map[string]interface{}
	EnvironmentVars map[string]string
	WorkingDir      string
	AdditionalPaths []string
}

type ServerDefinition struct {
	Name        string
	DisplayName string
	Runtime     string
	Languages   []string
	Extensions  []string
	ConfigName  string
}

type GatewayConfig struct {
	Port    int
	Servers []ServerConfig
}

type ServerConfig struct {
	Name      string
	Languages []string
	Command   string
	Args      []string
	Transport string
}

func NewStatusReporter() *DefaultStatusReporter {
	return &DefaultStatusReporter{}
}

func NewStatusReporterWithDeps(
	runtimeDetector RuntimeDetector,
	serverInstaller ServerInstaller,
	packageMgrs []PackageManager,
	configLoader ConfigLoader,
) *DefaultStatusReporter {
	return &DefaultStatusReporter{
		runtimeDetector: runtimeDetector,
		serverInstaller: serverInstaller,
		packageMgrs:     packageMgrs,
		configLoader:    configLoader,
	}
}

func (s *DefaultStatusReporter) GetRuntimeStatus() (*RuntimeStatus, error) {
	now := time.Now()
	runtimeStatus := &RuntimeStatus{
		Timestamp: now,
		Runtimes:  make(map[string]*RuntimeStatusInfo),
		Summary:   RuntimeStatusSummary{},
	}

	if s.runtimeDetector == nil {
		runtimeStatus.Summary = RuntimeStatusSummary{
			Total:      0,
			Installed:  0,
			Compatible: 0,
			Working:    0,
			Issues:     1,
		}
		return runtimeStatus, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	report, err := s.runtimeDetector.DetectAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("runtime detection failed: %w", err)
	}

	totalIssues := 0
	for name, runtime := range report.Runtimes {
		status := s.convertRuntimeInfoToStatus(runtime)
		runtimeStatus.Runtimes[name] = status
		totalIssues += len(status.Issues)
	}

	runtimeStatus.Summary = RuntimeStatusSummary{
		Total:      report.Summary.TotalRuntimes,
		Installed:  report.Summary.InstalledRuntimes,
		Compatible: report.Summary.CompatibleRuntimes,
		Working:    report.Summary.InstalledRuntimes, // Assume installed = working for now
		Issues:     totalIssues,
	}

	return runtimeStatus, nil
}

func (s *DefaultStatusReporter) GetServerStatus() (*ServerStatus, error) {
	now := time.Now()
	serverStatus := &ServerStatus{
		Timestamp: now,
		Servers:   make(map[string]*ServerStatusInfo),
		Summary:   ServerStatusSummary{},
	}

	if s.serverInstaller == nil {
		serverStatus.Summary = ServerStatusSummary{
			Total:        0,
			Installed:    0,
			Working:      0,
			RuntimeReady: 0,
			Issues:       1,
		}
		return serverStatus, nil
	}

	supportedServers := s.serverInstaller.GetSupportedServers()
	totalIssues := 0
	installedCount := 0
	workingCount := 0
	runtimeReadyCount := 0

	for _, serverName := range supportedServers {
		statusInfo := &ServerStatusInfo{
			Name:            serverName,
			Status:          ComponentStatusUnknown,
			Installed:       false,
			Version:         "",
			Path:            "",
			Runtime:         "",
			RuntimeCheck:    false,
			ExecutableCheck: false,
			Communication:   false,
			LastChecked:     now,
			Issues:          []string{},
			Metadata:        make(map[string]string),
		}

		if serverDef, err := s.serverInstaller.GetServerInfo(serverName); err == nil {
			statusInfo.Runtime = serverDef.Runtime
		}

		if verifyResult, err := s.serverInstaller.Verify(serverName); err == nil {
			statusInfo.Installed = verifyResult.Installed
			statusInfo.Version = verifyResult.Version
			statusInfo.Path = verifyResult.Path
			statusInfo.ExecutableCheck = verifyResult.Installed

			for _, issue := range verifyResult.Issues {
				statusInfo.Issues = append(statusInfo.Issues, issue.Description)
			}

			if verifyResult.Installed && verifyResult.Compatible && len(verifyResult.Issues) == 0 {
				statusInfo.Status = ComponentStatusHealthy
				statusInfo.Communication = true
				workingCount++
			} else if verifyResult.Installed {
				statusInfo.Status = ComponentStatusDegraded
			} else {
				statusInfo.Status = ComponentStatusNotInstalled
			}

			if verifyResult.Installed {
				installedCount++
			}

			if statusInfo.Runtime != "" && verifyResult.Installed {
				statusInfo.RuntimeCheck = true
				runtimeReadyCount++
			}

			totalIssues += len(verifyResult.Issues)
		} else {
			statusInfo.Issues = append(statusInfo.Issues, fmt.Sprintf("Verification failed: %v", err))
			statusInfo.Status = ComponentStatusUnknown
			totalIssues++
		}

		serverStatus.Servers[serverName] = statusInfo
	}

	serverStatus.Summary = ServerStatusSummary{
		Total:        len(supportedServers),
		Installed:    installedCount,
		Working:      workingCount,
		RuntimeReady: runtimeReadyCount,
		Issues:       totalIssues,
	}

	return serverStatus, nil
}

func (s *DefaultStatusReporter) GetSystemStatus() (*SystemStatus, error) {
	now := time.Now()

	runtimeStatus, err := s.GetRuntimeStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime status: %w", err)
	}

	serverStatus, err := s.GetServerStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get server status: %w", err)
	}

	configStatus, err := s.GetConfigStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get config status: %w", err)
	}

	platformStatus := s.getPlatformStatus()

	networkStatus := s.getNetworkStatus()

	healthyComponents := []string{}
	degradedComponents := []string{}
	unhealthyComponents := []string{}
	totalIssues := 0
	criticalIssues := 0

	if platformStatus.Status.IsHealthy() {
		healthyComponents = append(healthyComponents, "platform")
	} else if platformStatus.Status.NeedsAttention() {
		degradedComponents = append(degradedComponents, "platform")
	} else {
		unhealthyComponents = append(unhealthyComponents, "platform")
	}
	totalIssues += len(platformStatus.Issues)

	if runtimeStatus.Summary.Issues == 0 && runtimeStatus.Summary.Installed > 0 {
		healthyComponents = append(healthyComponents, "runtimes")
	} else if runtimeStatus.Summary.Installed > 0 {
		degradedComponents = append(degradedComponents, "runtimes")
	} else {
		unhealthyComponents = append(unhealthyComponents, "runtimes")
	}
	totalIssues += runtimeStatus.Summary.Issues

	if serverStatus.Summary.Issues == 0 && serverStatus.Summary.Working > 0 {
		healthyComponents = append(healthyComponents, "servers")
	} else if serverStatus.Summary.Installed > 0 {
		degradedComponents = append(degradedComponents, "servers")
	} else {
		unhealthyComponents = append(unhealthyComponents, "servers")
	}
	totalIssues += serverStatus.Summary.Issues

	if configStatus.Status.IsHealthy() {
		healthyComponents = append(healthyComponents, "config")
	} else if configStatus.Status.NeedsAttention() {
		degradedComponents = append(degradedComponents, "config")
	} else {
		unhealthyComponents = append(unhealthyComponents, "config")
	}
	totalIssues += len(configStatus.Issues)

	if networkStatus.Status.IsHealthy() {
		healthyComponents = append(healthyComponents, "network")
	} else if networkStatus.Status.NeedsAttention() {
		degradedComponents = append(degradedComponents, "network")
	} else {
		unhealthyComponents = append(unhealthyComponents, "network")
	}
	totalIssues += len(networkStatus.Issues)

	var overallStatus ComponentStatus
	if len(unhealthyComponents) > 0 {
		overallStatus = ComponentStatusUnhealthy
		criticalIssues = len(unhealthyComponents)
	} else if len(degradedComponents) > 0 {
		overallStatus = ComponentStatusDegraded
	} else if len(healthyComponents) > 0 {
		overallStatus = ComponentStatusHealthy
	} else {
		overallStatus = ComponentStatusUnknown
	}

	return &SystemStatus{
		Timestamp: now,
		Overall:   overallStatus,
		Platform:  platformStatus,
		Runtimes:  runtimeStatus,
		Servers:   serverStatus,
		Config:    configStatus,
		Network:   networkStatus,
		Summary: SystemStatusSummary{
			HealthyComponents:   healthyComponents,
			DegradedComponents:  degradedComponents,
			UnhealthyComponents: unhealthyComponents,
			TotalIssues:         totalIssues,
			CriticalIssues:      criticalIssues,
		},
	}, nil
}

func (s *DefaultStatusReporter) GetConfigStatus() (*ConfigStatus, error) {
	configStatus := &ConfigStatus{
		Status:       ComponentStatusUnknown,
		FileExists:   false,
		FilePath:     "",
		Syntax:       false,
		Schema:       false,
		ServerCount:  0,
		Issues:       []string{},
		LastModified: time.Time{},
	}

	configPaths := []string{
		"config.yaml",
		"./config.yaml",
		"lsp-gateway.yaml",
		"./lsp-gateway.yaml",
	}

	var configPath string
	var configExists bool

	for _, path := range configPaths {
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			configPath = path
			configExists = true
			configStatus.FileExists = true
			configStatus.FilePath = path
			configStatus.LastModified = stat.ModTime()
			break
		}
	}

	if !configExists {
		configStatus.Status = ComponentStatusNotInstalled
		configStatus.Issues = append(configStatus.Issues, "No configuration file found")
		if cwd, err := os.Getwd(); err == nil {
			configStatus.FilePath = filepath.Join(cwd, "config.yaml")
		} else {
			configStatus.FilePath = "config.yaml"
		}
		return configStatus, nil
	}

	if s.configLoader != nil {
		if config, err := s.configLoader.LoadConfig(configPath); err != nil {
			configStatus.Status = ComponentStatusUnhealthy
			configStatus.Syntax = false
			configStatus.Issues = append(configStatus.Issues, fmt.Sprintf("Failed to load config: %v", err))
		} else {
			configStatus.Syntax = true
			configStatus.ServerCount = len(config.Servers)

			if err := s.configLoader.ValidateConfig(config); err != nil {
				configStatus.Status = ComponentStatusDegraded
				configStatus.Schema = false
				configStatus.Issues = append(configStatus.Issues, fmt.Sprintf("Config validation failed: %v", err))
			} else {
				configStatus.Status = ComponentStatusHealthy
				configStatus.Schema = true
			}
		}
	} else {
		configStatus.Status = ComponentStatusHealthy
		configStatus.Syntax = true
		configStatus.Schema = true
		if content, err := os.ReadFile(configPath); err == nil {
			serverCount := strings.Count(string(content), "- name:")
			if serverCount == 0 {
				serverCount = strings.Count(string(content), "name:")
			}
			configStatus.ServerCount = serverCount
		}
	}

	return configStatus, nil
}

func (s *DefaultStatusReporter) GetStatus(component string) (*ComponentStatus, error) {
	switch strings.ToLower(component) {
	case "runtime", "runtimes":
		runtimeStatus, err := s.GetRuntimeStatus()
		if err != nil {
			status := ComponentStatusUnknown
			return &status, err
		}
		if runtimeStatus.Summary.Issues > 0 {
			status := ComponentStatusDegraded
			return &status, nil
		}
		if runtimeStatus.Summary.Installed > 0 {
			status := ComponentStatusHealthy
			return &status, nil
		}
		status := ComponentStatusNotInstalled
		return &status, nil

	case "server", "servers":
		serverStatus, err := s.GetServerStatus()
		if err != nil {
			status := ComponentStatusUnknown
			return &status, err
		}
		if serverStatus.Summary.Issues > 0 {
			status := ComponentStatusDegraded
			return &status, nil
		}
		if serverStatus.Summary.Working > 0 {
			status := ComponentStatusHealthy
			return &status, nil
		}
		status := ComponentStatusNotInstalled
		return &status, nil

	case "config", "configuration":
		configStatus, err := s.GetConfigStatus()
		if err != nil {
			status := ComponentStatusUnknown
			return &status, err
		}
		return &configStatus.Status, nil

	case "platform":
		platformStatus := s.getPlatformStatus()
		return &platformStatus.Status, nil

	case "network":
		networkStatus := s.getNetworkStatus()
		return &networkStatus.Status, nil

	case "system", "overall":
		systemStatus, err := s.GetSystemStatus()
		if err != nil {
			status := ComponentStatusUnknown
			return &status, err
		}
		return &systemStatus.Overall, nil

	default:
		status := ComponentStatusUnknown
		return &status, fmt.Errorf("unknown component: %s", component)
	}
}

func (c ComponentStatus) String() string {
	return string(c)
}

func (c ComponentStatus) IsHealthy() bool {
	return c == ComponentStatusHealthy
}

func (c ComponentStatus) IsInstalled() bool {
	return c != ComponentStatusNotInstalled
}

func (c ComponentStatus) NeedsAttention() bool {
	return c == ComponentStatusDegraded || c == ComponentStatusUnhealthy
}

func (s *DefaultStatusReporter) convertRuntimeInfoToStatus(runtime *RuntimeInfo) *RuntimeStatusInfo {
	var status ComponentStatus
	if !runtime.Installed {
		status = ComponentStatusNotInstalled
	} else if runtime.Compatible && len(runtime.Issues) == 0 {
		status = ComponentStatusHealthy
	} else if runtime.Installed {
		status = ComponentStatusDegraded
	} else {
		status = ComponentStatusUnhealthy
	}

	metadata := make(map[string]string)
	for key, value := range runtime.Metadata {
		if str, ok := value.(string); ok {
			metadata[key] = str
		} else {
			metadata[key] = fmt.Sprintf("%v", value)
		}
	}

	return &RuntimeStatusInfo{
		Name:        runtime.Name,
		Status:      status,
		Installed:   runtime.Installed,
		Version:     runtime.Version,
		Path:        runtime.Path,
		Compatible:  runtime.Compatible,
		Working:     runtime.Installed && runtime.Compatible,
		LastChecked: runtime.DetectedAt,
		Issues:      runtime.Issues,
		Metadata:    metadata,
	}
}

func (s *DefaultStatusReporter) getPlatformStatus() *PlatformStatus {
	platformStatus := &PlatformStatus{
		Status: ComponentStatusHealthy,
		Issues: []string{},
	}

	platformStatus.OS = runtime.GOOS
	platformStatus.Architecture = runtime.GOARCH

	switch runtime.GOOS {
	case "linux":
		if content, err := os.ReadFile("/etc/os-release"); err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "ID=") {
					platformStatus.Distribution = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
				}
				if strings.HasPrefix(line, "VERSION_ID=") {
					platformStatus.Version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
				}
			}
		}
	case "windows":
		platformStatus.Distribution = "windows"
	case "darwin":
		platformStatus.Distribution = "macos"
	}

	if s.packageMgrs != nil {
		for _, pm := range s.packageMgrs {
			pmStatus := PackageManagerStatus{
				Name:      pm.GetName(),
				Available: pm.IsAvailable(),
				Working:   false,
			}

			if pmStatus.Available {
				if version, err := pm.GetVersion(); err == nil {
					pmStatus.Version = version
				}

				if err := pm.TestInstall(); err == nil {
					pmStatus.Working = true
				}
			}

			platformStatus.PackageManagers = append(platformStatus.PackageManagers, pmStatus)
		}
	}

	return platformStatus
}

func (s *DefaultStatusReporter) getNetworkStatus() *NetworkStatus {
	networkStatus := &NetworkStatus{
		Status:           ComponentStatusHealthy,
		InternetAccess:   false,
		RepositoryAccess: make(map[string]bool),
		ProxyDetected:    false,
		ProxyConfig:      "",
		Issues:           []string{},
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	testURLs := []string{
		"https://golang.org",
		"https://pypi.org",
		"https://registry.npmjs.org",
		"https://repo1.maven.org",
	}

	successCount := 0
	for _, url := range testURLs {
		if resp, err := client.Get(url); err == nil {
			if closeErr := resp.Body.Close(); closeErr != nil {
			}
			if resp.StatusCode == 200 {
				networkStatus.RepositoryAccess[url] = true
				successCount++
			} else {
				networkStatus.RepositoryAccess[url] = false
			}
		} else {
			networkStatus.RepositoryAccess[url] = false
		}
	}

	if successCount > 0 {
		networkStatus.InternetAccess = true
	}

	if proxyEnv := os.Getenv("HTTP_PROXY"); proxyEnv != "" {
		networkStatus.ProxyDetected = true
		networkStatus.ProxyConfig = proxyEnv
	} else if proxyEnv := os.Getenv("http_proxy"); proxyEnv != "" {
		networkStatus.ProxyDetected = true
		networkStatus.ProxyConfig = proxyEnv
	}

	if !networkStatus.InternetAccess {
		networkStatus.Status = ComponentStatusUnhealthy
		networkStatus.Issues = append(networkStatus.Issues, "No internet connectivity detected")
	} else if successCount < len(testURLs)/2 {
		networkStatus.Status = ComponentStatusDegraded
		networkStatus.Issues = append(networkStatus.Issues, "Limited repository access")
	}

	return networkStatus
}
