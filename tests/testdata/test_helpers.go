package testdata

import (
	"context"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/setup"
	"lsp-gateway/internal/types"
	"time"
)

type TestContext struct {
	Context  context.Context
	Cancel   context.CancelFunc
	Timeout  time.Duration
	Metadata map[string]interface{}
}

func NewTestContext(timeout time.Duration) *TestContext {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return &TestContext{
		Context:  ctx,
		Cancel:   cancel,
		Timeout:  timeout,
		Metadata: make(map[string]interface{}),
	}
}

func (tc *TestContext) Cleanup() {
	if tc.Cancel != nil {
		tc.Cancel()
	}
}

func CreateMockRuntimeInfo(name, version, path string, installed, compatible bool) *setup.RuntimeInfo {
	return &setup.RuntimeInfo{
		Name:      name,
		Installed: installed,
		Version:   version,
		ParsedVersion: &setup.Version{
			Major:    1,
			Minor:    0,
			Patch:    0,
			Original: version,
		},
		Compatible:   compatible,
		MinVersion:   "1.0.0",
		Path:         path,
		WorkingDir:   "/tmp",
		DetectionCmd: name + " --version",
		Issues:       []string{},
		Warnings:     []string{},
		Metadata:     map[string]interface{}{"test": true},
		DetectedAt:   time.Now(),
		Duration:     time.Millisecond * 50,
	}
}

func CreateMockDetectionReport(platform platform.Platform, arch platform.Architecture) *setup.DetectionReport {
	return &setup.DetectionReport{
		Timestamp:    time.Now(),
		Platform:     platform,
		Architecture: arch,
		Runtimes: map[string]*setup.RuntimeInfo{
			"go":     CreateMockRuntimeInfo("go", "1.24.0", "/usr/local/go/bin/go", true, true),
			"python": CreateMockRuntimeInfo("python", "3.11.0", "/usr/bin/python3", true, true),
			"nodejs": CreateMockRuntimeInfo("nodejs", "20.0.0", "/usr/bin/node", true, true),
			"java":   CreateMockRuntimeInfo("java", "17.0.0", "/usr/bin/java", true, true),
		},
		Summary: setup.DetectionSummary{
			TotalRuntimes:      4,
			InstalledRuntimes:  4,
			CompatibleRuntimes: 4,
			IssuesFound:        0,
			WarningsFound:      0,
			SuccessRate:        100.0,
			AverageDetectTime:  time.Millisecond * 50,
		},
		Duration:  time.Millisecond * 200,
		Issues:    []string{},
		Warnings:  []string{},
		SessionID: "test-session-" + time.Now().Format("20060102-150405"),
		Metadata:  map[string]interface{}{"test": true},
	}
}

func CreateMockInstallResult(runtime, version, path string, success bool) *types.InstallResult {
	var errors []string
	if !success {
		errors = append(errors, "Installation failed")
	}

	return &types.InstallResult{
		Success:  success,
		Runtime:  runtime,
		Version:  version,
		Path:     path,
		Method:   "test_install",
		Duration: time.Second * 2,
		Errors:   errors,
		Warnings: []string{},
		Messages: []string{"Test installation of " + runtime},
		Details:  map[string]interface{}{"test": true},
	}
}

func CreateMockVerificationResult(runtime, version, path string, installed, compatible bool) *types.VerificationResult {
	var issues []types.Issue
	if !installed {
		issues = append(issues, types.Issue{
			Severity:    types.IssueSeverityHigh,
			Category:    types.IssueCategoryInstallation,
			Title:       runtime + " not installed",
			Description: "Runtime " + runtime + " is not installed on the system",
			Solution:    "Install " + runtime + " using the appropriate package manager",
			Details:     map[string]interface{}{"runtime": runtime},
		})
	}

	if !compatible {
		issues = append(issues, types.Issue{
			Severity:    types.IssueSeverityMedium,
			Category:    types.IssueCategoryVersion,
			Title:       runtime + " version incompatible",
			Description: "Runtime " + runtime + " version is not compatible with requirements",
			Solution:    "Upgrade " + runtime + " to a compatible version",
			Details:     map[string]interface{}{"runtime": runtime, "version": version},
		})
	}

	return &types.VerificationResult{
		Installed:       installed,
		Compatible:      compatible,
		Version:         version,
		Path:            path,
		Runtime:         runtime,
		Issues:          issues,
		Details:         map[string]interface{}{"test": true},
		Metadata:        map[string]interface{}{"verification": "test"},
		EnvironmentVars: map[string]string{},
		Recommendations: []string{},
		WorkingDir:      "/tmp",
		AdditionalPaths: []string{},
		VerifiedAt:      time.Now(),
		Duration:        time.Millisecond * 100,
	}
}

func CreateMockGatewayConfig(port int, servers []config.ServerConfig) *config.GatewayConfig {
	return &config.GatewayConfig{
		Port:    port,
		Servers: servers,
	}
}

func CreateMockServerConfig(name string, languages []string, command string, args []string) config.ServerConfig {
	return config.ServerConfig{
		Name:      name,
		Languages: languages,
		Command:   command,
		Args:      args,
		Transport: "stdio",
	}
}

func CreateDefaultTestServers() []config.ServerConfig {
	return []config.ServerConfig{
		CreateMockServerConfig("go-lsp", []string{"go"}, "gopls", []string{}),
		CreateMockServerConfig("python-lsp", []string{"python"}, "python", []string{"-m", "pylsp"}),
		CreateMockServerConfig("typescript-lsp", []string{"typescript", "javascript"}, "typescript-language-server", []string{"--stdio"}),
		CreateMockServerConfig("java-lsp", []string{"java"}, "java", []string{"-jar", "/usr/local/share/jdtls/jdtls.jar"}),
	}
}

func CreateMockConfigGenerationResult(config *config.GatewayConfig, serversGenerated int, autoDetected bool) *setup.ConfigGenerationResult {
	return &setup.ConfigGenerationResult{
		Config:           config,
		DetectionReport:  nil,
		ServersGenerated: serversGenerated,
		ServersSkipped:   0,
		AutoDetected:     autoDetected,
		GeneratedAt:      time.Now(),
		Duration:         time.Millisecond * 300,
		Messages:         []string{"Test configuration generated"},
		Warnings:         []string{},
		Issues:           []string{},
		Metadata:         map[string]interface{}{"test": true},
	}
}

func CreateMockConfigValidationResult(valid bool, issues []string, warnings []string) *setup.ConfigValidationResult {
	return &setup.ConfigValidationResult{
		Valid:            valid,
		Issues:           issues,
		Warnings:         warnings,
		ServersValidated: 4,
		ServerIssues:     make(map[string][]string),
		ValidatedAt:      time.Now(),
		Duration:         time.Millisecond * 50,
		Metadata:         map[string]interface{}{"test": true},
	}
}

func CreateMockRuntimeDefinition(name, displayName, minVersion string) *types.RuntimeDefinition {
	return &types.RuntimeDefinition{
		Name:               name,
		DisplayName:        displayName,
		MinVersion:         minVersion,
		RecommendedVersion: "latest",
		InstallMethods: map[string]types.InstallMethod{
			"package_manager": {
				Name:         "Package Manager",
				Platform:     "linux",
				Method:       "apt",
				Commands:     []string{"apt", "install", "-y", name},
				Description:  "Install via system package manager",
				Requirements: []string{"sudo"},
			},
		},
		VerificationCmd: []string{name, "--version"},
		VersionCommand:  []string{name, "--version"},
		EnvVars:         map[string]string{},
		Dependencies:    []string{},
		PostInstall:     []string{},
	}
}

func CreateMockServerDefinition(name, displayName, runtime string, languages []string) *types.ServerDefinition {
	return &types.ServerDefinition{
		Name:              name,
		DisplayName:       displayName,
		Runtime:           runtime,
		MinVersion:        "1.0.0",
		MinRuntimeVersion: "1.0.0",
		InstallCmd:        []string{"install", name},
		VerifyCmd:         []string{name, "--version"},
		ConfigKey:         name,
		Description:       displayName + " language server",
		Homepage:          "https://example.com/" + name,
		Languages:         languages,
		Extensions:        []string{},
		InstallMethods: []types.InstallMethod{
			{
				Name:         "Default Install",
				Platform:     "linux",
				Method:       "default",
				Commands:     []string{"install", name},
				Description:  "Default installation method",
				Requirements: []string{runtime},
			},
		},
		VersionCommand: []string{name, "--version"},
	}
}

func CreateMockVersionValidationResult(valid bool, required, installed string) *types.VersionValidationResult {
	var issues []types.Issue
	if !valid {
		issues = append(issues, types.Issue{
			Severity:    types.IssueSeverityHigh,
			Category:    types.IssueCategoryVersion,
			Title:       "Version mismatch",
			Description: "Installed version does not meet requirements",
			Solution:    "Upgrade to version " + required + " or higher",
			Details:     map[string]interface{}{"required": required, "installed": installed},
		})
	}

	return &types.VersionValidationResult{
		Valid:            valid,
		RequiredVersion:  required,
		InstalledVersion: installed,
		Issues:           issues,
	}
}

func CreateMockDependencyValidationResult(server string, valid bool, runtimeRequired string) *types.DependencyValidationResult {
	var issues []types.Issue
	var missingRuntimes []string
	var versionIssues []types.VersionIssue

	if !valid {
		issues = append(issues, types.Issue{
			Severity:    types.IssueSeverityHigh,
			Category:    types.IssueCategoryDependencies,
			Title:       "Missing dependencies",
			Description: "Required runtime " + runtimeRequired + " is not available",
			Solution:    "Install runtime " + runtimeRequired,
			Details:     map[string]interface{}{"server": server, "runtime": runtimeRequired},
		})
		missingRuntimes = append(missingRuntimes, runtimeRequired)
	}

	return &types.DependencyValidationResult{
		Server:            server,
		Valid:             valid,
		RuntimeRequired:   runtimeRequired,
		RuntimeInstalled:  valid,
		RuntimeVersion:    "1.0.0",
		RuntimeCompatible: valid,
		Issues:            issues,
		CanInstall:        valid,
		MissingRuntimes:   missingRuntimes,
		VersionIssues:     versionIssues,
		Recommendations:   []string{},
		ValidatedAt:       time.Now(),
		Duration:          time.Millisecond * 100,
	}
}

func CreateMockInstallOptions(version string, force bool) types.InstallOptions {
	return types.InstallOptions{
		Version:        version,
		Force:          force,
		SkipVerify:     false,
		Timeout:        time.Minute * 5,
		PackageManager: "apt",
		Platform:       "linux",
	}
}

func CreateMockServerInstallOptions(version string, force bool) types.ServerInstallOptions {
	return types.ServerInstallOptions{
		Version:             version,
		Force:               force,
		SkipVerify:          false,
		SkipDependencyCheck: false,
		Timeout:             time.Minute * 10,
		Platform:            "linux",
		InstallMethod:       "default",
		WorkingDir:          "/tmp",
	}
}

type TestRunner struct {
	context  *TestContext
	failures []string
	warnings []string
}

func NewTestRunner(timeout time.Duration) *TestRunner {
	return &TestRunner{
		context:  NewTestContext(timeout),
		failures: make([]string, 0),
		warnings: make([]string, 0),
	}
}

func (tr *TestRunner) Context() context.Context {
	return tr.context.Context
}

func (tr *TestRunner) AddFailure(message string) {
	tr.failures = append(tr.failures, message)
}

func (tr *TestRunner) AddWarning(message string) {
	tr.warnings = append(tr.warnings, message)
}

func (tr *TestRunner) HasFailures() bool {
	return len(tr.failures) > 0
}

func (tr *TestRunner) GetFailures() []string {
	return tr.failures
}

func (tr *TestRunner) GetWarnings() []string {
	return tr.warnings
}

func (tr *TestRunner) Cleanup() {
	tr.context.Cleanup()
}
