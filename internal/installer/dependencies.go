package installer

import (
	"context"
	"fmt"
	"lsp-gateway/internal/types"
	"strings"
	"time"
)

type DependencyValidator interface {
	ValidateServerDependencies(ctx context.Context, serverName string) (*DetailedDependencyValidationResult, error)

	ValidateAllDependencies(ctx context.Context) (*AllDependenciesResult, error)

	ValidateRuntimeForServers(ctx context.Context, runtime string) (*RuntimeServerValidationResult, error)

	GetServerDependencies(serverName string) (*ServerDependencyInfo, error)

	GetInstallationSuggestions(ctx context.Context, result *DetailedDependencyValidationResult) (*InstallationSuggestions, error)
}

type DetailedDependencyValidationResult struct {
	ServerName       string                       // Name of the server being validated
	CanInstall       bool                         // Whether the server can be installed with current dependencies
	ValidationTime   time.Time                    // When validation was performed
	Duration         time.Duration                // Time taken for validation
	RequiredRuntime  *RuntimeRequirement          // Primary runtime requirement
	OptionalRuntimes []*RuntimeRequirement        // Optional runtime dependencies
	MissingRequired  []*MissingDependency         // Required dependencies that are missing
	MissingOptional  []*MissingDependency         // Optional dependencies that are missing
	VersionIssues    []*VersionCompatibilityIssue // Version compatibility problems
	Warnings         []string                     // Non-critical warnings
	Recommendations  []string                     // Installation/configuration recommendations
	Metadata         map[string]interface{}       // Additional validation metadata
}

type AllDependenciesResult struct {
	ValidationTime     time.Time                                      // When validation was performed
	Duration           time.Duration                                  // Total validation time
	ServerResults      map[string]*DetailedDependencyValidationResult // Results for each server
	GlobalIssues       []string                                       // Issues affecting multiple servers
	InstallableServers []string                                       // Servers that can be installed immediately
	MissingRuntimes    []string                                       // Runtimes that need to be installed
	Summary            *DependencyValidationSummary                   // Overall summary
	Recommendations    []string                                       // Global recommendations
}

type RuntimeServerValidationResult struct {
	Runtime              string                  // Runtime being validated
	RuntimeAvailable     bool                    // Whether runtime is available
	RuntimeVersion       string                  // Detected runtime version
	SupportedServers     []*ServerDependencyInfo // Servers that can use this runtime
	InstallableServers   []string                // Servers ready for installation
	ServersNeedingUpdate []string                // Servers requiring runtime update
	ValidationTime       time.Time               // When validation was performed
	Duration             time.Duration           // Validation duration
}

type ServerDependencyInfo struct {
	ServerName          string                // Server name
	DisplayName         string                // Human-readable server name
	RequiredRuntime     *RuntimeRequirement   // Primary runtime requirement
	OptionalRuntimes    []*RuntimeRequirement // Optional dependencies
	AdditionalPackages  []*PackageRequirement // Additional package requirements (e.g., pip packages)
	EnvironmentVars     map[string]string     // Required environment variables
	PreInstallCommands  []string              // Commands to run before installation
	PostInstallCommands []string              // Commands to run after installation
	VerificationSteps   []*VerificationStep   // Steps to verify successful installation
	ConflictsWith       []string              // Servers/packages that conflict with this one
	InstallationNotes   []string              // Additional installation notes
}

type RuntimeRequirement struct {
	Name               string   // Runtime name (go, python, nodejs, java)
	MinVersion         string   // Minimum required version
	RecommendedVersion string   // Recommended version
	Required           bool     // Whether this is a mandatory dependency
	Purpose            string   // Why this runtime is needed
	InstallationCmd    []string // Suggested installation command
}

type PackageRequirement struct {
	Name            string   // Package name
	Runtime         string   // Runtime this package belongs to (python, nodejs, etc.)
	MinVersion      string   // Minimum version required
	InstallCommands []string // Commands to install this package
	Required        bool     // Whether this package is mandatory
	Purpose         string   // Why this package is needed
}

type VerificationStep struct {
	Name           string   // Step name
	Command        []string // Command to run for verification
	ExpectedOutput string   // Expected output pattern
	FailureMessage string   // Message to show if verification fails
}

type MissingDependency struct {
	Type            string   // "runtime" or "package"
	Name            string   // Dependency name
	Required        bool     // Whether this is a mandatory dependency
	MinVersion      string   // Minimum version required
	CurrentVersion  string   // Currently installed version (if any)
	InstallCommands []string // Suggested installation commands
	Reason          string   // Why this dependency is needed
}

type VersionCompatibilityIssue struct {
	DependencyName   string              // Name of the dependency with version issue
	RequiredVersion  string              // Required version
	InstalledVersion string              // Currently installed version
	Severity         types.IssueSeverity // Severity of the issue
	Impact           string              // Impact on server functionality
	UpgradeCommands  []string            // Commands to upgrade to compatible version
}

type DependencyValidationSummary struct {
	TotalServers             int     // Total number of servers checked
	InstallableServers       int     // Servers ready for installation
	ServersWithMissing       int     // Servers with missing dependencies
	ServersWithVersionIssues int     // Servers with version compatibility issues
	MissingRuntimeCount      int     // Number of missing runtimes
	OverallReadiness         float64 // Percentage of servers ready for installation
}

type InstallationSuggestions struct {
	RuntimeInstallations   []*RuntimeInstallSuggestion // Runtime installation suggestions
	PackageInstallations   []*PackageInstallSuggestion // Package installation suggestions
	EnvironmentSetup       []string                    // Environment setup commands
	PlatformSpecificNotes  []string                    // Platform-specific installation notes
	OrderedInstallSequence []*InstallationStep         // Recommended installation order
	EstimatedTime          time.Duration               // Estimated time for full installation
	AlternativeOptions     []*AlternativeInstallOption // Alternative installation methods
}

type RuntimeInstallSuggestion struct {
	Runtime            string           // Runtime name
	RecommendedVersion string           // Recommended version to install
	InstallMethods     []*InstallMethod // Available installation methods
	Priority           int              // Installation priority (1 = highest)
	EstimatedTime      time.Duration    // Estimated installation time
	PostInstallSteps   []string         // Steps to run after installation
}

type PackageInstallSuggestion struct {
	Package       string        // Package name
	Runtime       string        // Runtime this package belongs to
	Commands      []string      // Installation commands
	Priority      int           // Installation priority
	EstimatedTime time.Duration // Estimated installation time
}

type InstallationStep struct {
	Order         int           // Step order
	Type          string        // "runtime", "package", "verification", "setup"
	Name          string        // Step name
	Commands      []string      // Commands to execute
	Description   string        // Step description
	Required      bool          // Whether this step is mandatory
	EstimatedTime time.Duration // Estimated time for this step
}

type AlternativeInstallOption struct {
	Method      string   // Installation method name
	Description string   // Method description
	Commands    []string // Commands for this method
	Pros        []string // Advantages of this method
	Cons        []string // Disadvantages of this method
	Difficulty  string   // "easy", "medium", "advanced"
}

type DefaultDependencyValidator struct {
	serverRegistry     *ServerRegistry
	runtimeDetector    types.RuntimeDetector
	runtimeInstaller   types.RuntimeInstaller
	dependencyRegistry *DependencyRegistry
	logger             types.SetupLogger
}

func NewDependencyValidator(
	serverRegistry *ServerRegistry,
	runtimeDetector types.RuntimeDetector,
	runtimeInstaller types.RuntimeInstaller,
) *DefaultDependencyValidator {
	return &DefaultDependencyValidator{
		serverRegistry:     serverRegistry,
		runtimeDetector:    runtimeDetector,
		runtimeInstaller:   runtimeInstaller,
		dependencyRegistry: NewDependencyRegistry(),
		logger:             nil, // Logger will be set externally
	}
}

func (v *DefaultDependencyValidator) SetLogger(logger types.SetupLogger) {
	if logger != nil {
		v.logger = logger
	}
}

func (v *DefaultDependencyValidator) ValidateServerDependencies(ctx context.Context, serverName string) (*DetailedDependencyValidationResult, error) {
	startTime := time.Now()
	v.logValidationStart(serverName)

	if err := v.validateServerExists(serverName); err != nil {
		return nil, err
	}

	depInfo, err := v.getServerDependencyInfo(serverName)
	if err != nil {
		return nil, err
	}

	result := v.initializeDependencyValidationResult(serverName, startTime, depInfo)

	v.validateRequiredRuntimes(ctx, depInfo, result)
	v.validateOptionalRuntimes(ctx, depInfo, result)
	v.validateAdditionalPackages(ctx, depInfo, result)

	v.assessInstallability(result)
	v.generateValidationRecommendations(result, depInfo)
	v.finalizeDependencyValidation(result, serverName, startTime)

	return result, nil
}

func (v *DefaultDependencyValidator) logValidationStart(serverName string) {
	if v.logger != nil {
		v.logger.WithField("server", serverName).Info("Starting dependency validation for server")
	}
}

func (v *DefaultDependencyValidator) validateServerExists(serverName string) error {
	_, err := v.serverRegistry.GetServer(serverName)
	if err != nil {
		return NewInstallerError(InstallerErrorTypeNotFound, serverName,
			fmt.Sprintf("server not found: %s", serverName), err)
	}
	return nil
}

func (v *DefaultDependencyValidator) getServerDependencyInfo(serverName string) (*ServerDependencyInfo, error) {
	depInfo, err := v.GetServerDependencies(serverName)
	if err != nil {
		return nil, NewInstallerError(InstallerErrorTypeNotFound, serverName,
			fmt.Sprintf("dependency information not found for server: %s", serverName), err)
	}
	return depInfo, nil
}

func (v *DefaultDependencyValidator) initializeDependencyValidationResult(serverName string, startTime time.Time, depInfo *ServerDependencyInfo) *DetailedDependencyValidationResult {
	return &DetailedDependencyValidationResult{
		ServerName:       serverName,
		CanInstall:       true,
		ValidationTime:   startTime,
		RequiredRuntime:  depInfo.RequiredRuntime,
		OptionalRuntimes: depInfo.OptionalRuntimes,
		MissingRequired:  []*MissingDependency{},
		MissingOptional:  []*MissingDependency{},
		VersionIssues:    []*VersionCompatibilityIssue{},
		Warnings:         []string{},
		Recommendations:  []string{},
		Metadata:         make(map[string]interface{}),
	}
}

func (v *DefaultDependencyValidator) validateRequiredRuntimes(ctx context.Context, depInfo *ServerDependencyInfo, result *DetailedDependencyValidationResult) {
	if depInfo.RequiredRuntime == nil {
		return
	}

	if err := v.validateRuntimeRequirement(ctx, depInfo.RequiredRuntime, result); err != nil {
		v.logRuntimeValidationError(err, "Failed to validate required runtime")
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Runtime validation failed: %v", err))
	}
}

func (v *DefaultDependencyValidator) validateOptionalRuntimes(ctx context.Context, depInfo *ServerDependencyInfo, result *DetailedDependencyValidationResult) {
	for _, optionalRuntime := range depInfo.OptionalRuntimes {
		if err := v.validateRuntimeRequirement(ctx, optionalRuntime, result); err != nil {
			v.logOptionalRuntimeError(err, optionalRuntime.Name)
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Optional runtime %s validation failed: %v", optionalRuntime.Name, err))
		}
	}
}

func (v *DefaultDependencyValidator) validateAdditionalPackages(ctx context.Context, depInfo *ServerDependencyInfo, result *DetailedDependencyValidationResult) {
	for _, pkg := range depInfo.AdditionalPackages {
		if err := v.validatePackageRequirement(ctx, pkg, result); err != nil {
			v.logPackageValidationError(err, pkg.Name)
			if pkg.Required {
				result.CanInstall = false
			}
		}
	}
}

func (v *DefaultDependencyValidator) logRuntimeValidationError(err error, message string) {
	if v.logger != nil {
		v.logger.WithError(err).Warn(message)
	}
}

func (v *DefaultDependencyValidator) logOptionalRuntimeError(err error, runtimeName string) {
	if v.logger != nil {
		v.logger.WithError(err).Debug("Optional runtime validation failed")
	}
}

func (v *DefaultDependencyValidator) logPackageValidationError(err error, packageName string) {
	if v.logger != nil {
		v.logger.WithError(err).WithField("package", packageName).Warn("Package validation failed")
	}
}

func (v *DefaultDependencyValidator) assessInstallability(result *DetailedDependencyValidationResult) {
	if len(result.MissingRequired) == 0 && len(result.VersionIssues) == 0 {
		return
	}

	hasBlockingIssues := v.hasBlockingVersionIssues(result.VersionIssues)
	if len(result.MissingRequired) > 0 || hasBlockingIssues {
		result.CanInstall = false
	}
}

func (v *DefaultDependencyValidator) hasBlockingVersionIssues(versionIssues []*VersionCompatibilityIssue) bool {
	for _, issue := range versionIssues {
		if issue.Severity == types.IssueSeverityCritical || issue.Severity == types.IssueSeverityHigh {
			return true
		}
	}
	return false
}

func (v *DefaultDependencyValidator) finalizeDependencyValidation(result *DetailedDependencyValidationResult, serverName string, startTime time.Time) {
	result.Duration = time.Since(startTime)

	if v.logger != nil {
		v.logger.WithFields(map[string]interface{}{
			"server":           serverName,
			"can_install":      result.CanInstall,
			"missing_required": len(result.MissingRequired),
			"missing_optional": len(result.MissingOptional),
			"version_issues":   len(result.VersionIssues),
			"duration":         result.Duration,
		}).Info("Dependency validation completed")
	}
}

func (v *DefaultDependencyValidator) ValidateAllDependencies(ctx context.Context) (*AllDependenciesResult, error) {
	startTime := time.Now()

	if v.logger != nil {
		v.logger.Info("Starting dependency validation for all servers")
	}

	result := &AllDependenciesResult{
		ValidationTime:     startTime,
		ServerResults:      make(map[string]*DetailedDependencyValidationResult),
		GlobalIssues:       []string{},
		InstallableServers: []string{},
		MissingRuntimes:    []string{},
		Recommendations:    []string{},
	}

	servers := v.serverRegistry.ListServers()

	for _, serverName := range servers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		serverResult, err := v.ValidateServerDependencies(ctx, serverName)
		if err != nil {
			if v.logger != nil {
				v.logger.WithError(err).WithField("server", serverName).Warn("Server validation failed")
			}
			result.GlobalIssues = append(result.GlobalIssues,
				fmt.Sprintf("Failed to validate %s: %v", serverName, err))
			continue
		}

		result.ServerResults[serverName] = serverResult

		if serverResult.CanInstall {
			result.InstallableServers = append(result.InstallableServers, serverName)
		}

		for _, missing := range serverResult.MissingRequired {
			if missing.Type == RuntimeKeyword {
				if !containsString(result.MissingRuntimes, missing.Name) {
					result.MissingRuntimes = append(result.MissingRuntimes, missing.Name)
				}
			}
		}
	}

	result.Summary = v.generateValidationSummary(result)

	result.Recommendations = v.generateGlobalRecommendations(result)

	result.Duration = time.Since(startTime)

	if v.logger != nil {
		v.logger.WithFields(map[string]interface{}{
			"total_servers":       len(servers),
			"installable_servers": len(result.InstallableServers),
			"missing_runtimes":    len(result.MissingRuntimes),
			"global_issues":       len(result.GlobalIssues),
			"duration":            result.Duration,
		}).Info("All dependencies validation completed")
	}

	return result, nil
}

func (v *DefaultDependencyValidator) ValidateRuntimeForServers(ctx context.Context, runtime string) (*RuntimeServerValidationResult, error) {
	startTime := time.Now()

	if v.logger != nil {
		v.logger.WithField("runtime", runtime).Info("Starting runtime-server validation")
	}

	result := &RuntimeServerValidationResult{
		Runtime:              runtime,
		RuntimeAvailable:     false,
		SupportedServers:     []*ServerDependencyInfo{},
		InstallableServers:   []string{},
		ServersNeedingUpdate: []string{},
		ValidationTime:       startTime,
	}

	var runtimeInfo *types.RuntimeInfo
	var err error

	switch runtime {
	case RuntimeGo:
		runtimeInfo, err = v.runtimeDetector.DetectGo(ctx)
	case RuntimePython:
		runtimeInfo, err = v.runtimeDetector.DetectPython(ctx)
	case RuntimeNodeJS:
		runtimeInfo, err = v.runtimeDetector.DetectNodejs(ctx)
	case RuntimeJava:
		runtimeInfo, err = v.runtimeDetector.DetectJava(ctx)
	default:
		return nil, NewInstallerError(InstallerErrorTypeUnsupported, runtime,
			fmt.Sprintf("unsupported runtime: %s", runtime), nil)
	}

	if err != nil {
		if v.logger != nil {
			v.logger.WithError(err).WithField("runtime", runtime).Warn("Runtime detection failed")
		}
		result.RuntimeAvailable = false
	} else if runtimeInfo != nil {
		result.RuntimeAvailable = runtimeInfo.Installed
		result.RuntimeVersion = runtimeInfo.Version
	}

	allServers := v.serverRegistry.ListServers()
	for _, serverName := range allServers {
		depInfo, err := v.GetServerDependencies(serverName)
		if err != nil {
			continue
		}

		if depInfo.RequiredRuntime != nil && depInfo.RequiredRuntime.Name == runtime {
			result.SupportedServers = append(result.SupportedServers, depInfo)

			if result.RuntimeAvailable {
				if runtimeInfo != nil && runtimeInfo.Compatible {
					result.InstallableServers = append(result.InstallableServers, serverName)
				} else {
					result.ServersNeedingUpdate = append(result.ServersNeedingUpdate, serverName)
				}
			}
		}
	}

	result.Duration = time.Since(startTime)

	if v.logger != nil {
		v.logger.WithFields(map[string]interface{}{
			"runtime":                runtime,
			"runtime_available":      result.RuntimeAvailable,
			"supported_servers":      len(result.SupportedServers),
			"installable_servers":    len(result.InstallableServers),
			"servers_needing_update": len(result.ServersNeedingUpdate),
			"duration":               result.Duration,
		}).Info("Runtime-server validation completed")
	}

	return result, nil
}

func (v *DefaultDependencyValidator) GetServerDependencies(serverName string) (*ServerDependencyInfo, error) {
	return v.dependencyRegistry.GetServerDependencies(serverName)
}

func (v *DefaultDependencyValidator) GetInstallationSuggestions(ctx context.Context, result *DetailedDependencyValidationResult) (*InstallationSuggestions, error) {
	suggestions := &InstallationSuggestions{
		RuntimeInstallations:   []*RuntimeInstallSuggestion{},
		PackageInstallations:   []*PackageInstallSuggestion{},
		EnvironmentSetup:       []string{},
		PlatformSpecificNotes:  []string{},
		OrderedInstallSequence: []*InstallationStep{},
		AlternativeOptions:     []*AlternativeInstallOption{},
	}

	order := 1
	var totalEstimatedTime time.Duration

	for _, missing := range result.MissingRequired {
		if missing.Type == RuntimeKeyword {
			suggestion := &RuntimeInstallSuggestion{
				Runtime:            missing.Name,
				RecommendedVersion: missing.MinVersion,
				InstallMethods:     []*InstallMethod{},
				Priority:           order,
				EstimatedTime:      5 * time.Minute, // Default estimate
			}

			suggestions.RuntimeInstallations = append(suggestions.RuntimeInstallations, suggestion)

			step := &InstallationStep{
				Order:         order,
				Type:          "runtime",
				Name:          fmt.Sprintf("Install %s runtime", missing.Name),
				Commands:      missing.InstallCommands,
				Description:   fmt.Sprintf("Install %s runtime version %s or later", missing.Name, missing.MinVersion),
				Required:      true,
				EstimatedTime: suggestion.EstimatedTime,
			}
			suggestions.OrderedInstallSequence = append(suggestions.OrderedInstallSequence, step)
			totalEstimatedTime += suggestion.EstimatedTime
			order++
		}
	}

	for _, missing := range result.MissingRequired {
		if missing.Type == "package" {
			suggestion := &PackageInstallSuggestion{
				Package:       missing.Name,
				Runtime:       missing.Name, // This should be enhanced to track the actual runtime
				Commands:      missing.InstallCommands,
				Priority:      order,
				EstimatedTime: 2 * time.Minute, // Default estimate
			}

			suggestions.PackageInstallations = append(suggestions.PackageInstallations, suggestion)

			step := &InstallationStep{
				Order:         order,
				Type:          "package",
				Name:          fmt.Sprintf("Install %s package", missing.Name),
				Commands:      missing.InstallCommands,
				Description:   fmt.Sprintf("Install %s package: %s", missing.Name, missing.Reason),
				Required:      true,
				EstimatedTime: suggestion.EstimatedTime,
			}
			suggestions.OrderedInstallSequence = append(suggestions.OrderedInstallSequence, step)
			totalEstimatedTime += suggestion.EstimatedTime
			order++
		}
	}

	if len(suggestions.OrderedInstallSequence) > 0 {
		step := &InstallationStep{
			Order:         order,
			Type:          "verification",
			Name:          "Verify installation",
			Commands:      []string{fmt.Sprintf("lsp-gateway verify server %s", result.ServerName)},
			Description:   "Verify that the server can be installed successfully",
			Required:      true,
			EstimatedTime: 30 * time.Second,
		}
		suggestions.OrderedInstallSequence = append(suggestions.OrderedInstallSequence, step)
		totalEstimatedTime += step.EstimatedTime
	}

	suggestions.EstimatedTime = totalEstimatedTime

	suggestions.PlatformSpecificNotes = v.generatePlatformSpecificNotes(result)

	return suggestions, nil
}

func (v *DefaultDependencyValidator) validateRuntimeRequirement(ctx context.Context, req *RuntimeRequirement, result *DetailedDependencyValidationResult) error {
	var runtimeInfo *types.RuntimeInfo
	var err error

	switch req.Name {
	case RuntimeGo:
		runtimeInfo, err = v.runtimeDetector.DetectGo(ctx)
	case RuntimePython:
		runtimeInfo, err = v.runtimeDetector.DetectPython(ctx)
	case RuntimeNodeJS:
		runtimeInfo, err = v.runtimeDetector.DetectNodejs(ctx)
	case RuntimeJava:
		runtimeInfo, err = v.runtimeDetector.DetectJava(ctx)
	default:
		return fmt.Errorf("unsupported runtime: %s", req.Name)
	}

	if err != nil {
		return fmt.Errorf("failed to detect runtime %s: %w", req.Name, err)
	}

	if !runtimeInfo.Installed {
		missing := &MissingDependency{
			Type:            "runtime",
			Name:            req.Name,
			Required:        req.Required,
			MinVersion:      req.MinVersion,
			CurrentVersion:  "",
			InstallCommands: req.InstallationCmd,
			Reason:          req.Purpose,
		}

		if req.Required {
			result.MissingRequired = append(result.MissingRequired, missing)
			result.CanInstall = false
		} else {
			result.MissingOptional = append(result.MissingOptional, missing)
		}

		return nil
	}

	if req.MinVersion != "" && !runtimeInfo.Compatible {
		issue := &VersionCompatibilityIssue{
			DependencyName:   req.Name,
			RequiredVersion:  req.MinVersion,
			InstalledVersion: runtimeInfo.Version,
			Severity:         types.IssueSeverityHigh,
			Impact:           fmt.Sprintf("Server %s may not function correctly", result.ServerName),
			UpgradeCommands:  req.InstallationCmd,
		}

		if req.Required {
			issue.Severity = types.IssueSeverityCritical
			result.CanInstall = false
		}

		result.VersionIssues = append(result.VersionIssues, issue)
	}

	return nil
}

func (v *DefaultDependencyValidator) validatePackageRequirement(ctx context.Context, req *PackageRequirement, result *DetailedDependencyValidationResult) error {

	if v.logger != nil {
		v.logger.WithFields(map[string]interface{}{
			"package":  req.Name,
			"runtime":  req.Runtime,
			"required": req.Required,
		}).Debug("Package validation not fully implemented")
	}

	return nil
}

func (v *DefaultDependencyValidator) generateValidationRecommendations(result *DetailedDependencyValidationResult, depInfo *ServerDependencyInfo) {
	if !result.CanInstall {
		if len(result.MissingRequired) > 0 {
			result.Recommendations = append(result.Recommendations,
				"Install missing required dependencies before attempting server installation")
		}

		if len(result.VersionIssues) > 0 {
			result.Recommendations = append(result.Recommendations,
				"Update runtime versions to meet minimum requirements")
		}
	} else {
		if len(result.MissingOptional) > 0 {
			result.Recommendations = append(result.Recommendations,
				"Consider installing optional dependencies for enhanced functionality")
		}

		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Server %s is ready for installation", result.ServerName))
	}

	result.Recommendations = append(result.Recommendations, depInfo.InstallationNotes...)
}

func (v *DefaultDependencyValidator) generateValidationSummary(result *AllDependenciesResult) *DependencyValidationSummary {
	summary := &DependencyValidationSummary{
		TotalServers:        len(result.ServerResults),
		InstallableServers:  len(result.InstallableServers),
		MissingRuntimeCount: len(result.MissingRuntimes),
	}

	for _, serverResult := range result.ServerResults {
		if len(serverResult.MissingRequired) > 0 || len(serverResult.MissingOptional) > 0 {
			summary.ServersWithMissing++
		}
		if len(serverResult.VersionIssues) > 0 {
			summary.ServersWithVersionIssues++
		}
	}

	if summary.TotalServers > 0 {
		summary.OverallReadiness = float64(summary.InstallableServers) / float64(summary.TotalServers) * 100.0
	}

	return summary
}

func (v *DefaultDependencyValidator) generateGlobalRecommendations(result *AllDependenciesResult) []string {
	recommendations := []string{}

	if len(result.MissingRuntimes) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Install missing runtimes: %s", strings.Join(result.MissingRuntimes, ", ")))
	}

	if len(result.InstallableServers) == len(result.ServerResults) {
		recommendations = append(recommendations,
			"All servers are ready for installation!")
	} else if len(result.InstallableServers) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Ready to install: %s", strings.Join(result.InstallableServers, ", ")))
	}

	return recommendations
}

func (v *DefaultDependencyValidator) generatePlatformSpecificNotes(result *DetailedDependencyValidationResult) []string {
	notes := []string{}

	notes = append(notes, "Run 'npm run diagnostics' for comprehensive system analysis")
	notes = append(notes, "Use 'npm run runtime-install <runtime>' for automated runtime installation")

	return notes
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
