package installer

import (
	"fmt"
	rt "runtime"
	"time"

	"lsp-gateway/internal/types"
)

type RuntimeInstaller = types.RuntimeInstaller

type InstallOptions = types.InstallOptions

type InstallResult = types.InstallResult

type VerificationResult = types.VerificationResult

type Issue = types.Issue

type IssueSeverity = types.IssueSeverity

const (
	IssueSeverityCritical = types.IssueSeverityCritical
	IssueSeverityHigh     = types.IssueSeverityHigh
	IssueSeverityMedium   = types.IssueSeverityMedium
	IssueSeverityLow      = types.IssueSeverityLow
	IssueSeverityInfo     = types.IssueSeverityInfo
)

type IssueCategory = types.IssueCategory

const (
	IssueCategoryInstallation  = types.IssueCategoryInstallation
	IssueCategoryVersion       = types.IssueCategoryVersion
	IssueCategoryPath          = types.IssueCategoryPath
	IssueCategoryEnvironment   = types.IssueCategoryEnvironment
	IssueCategoryPermissions   = types.IssueCategoryPermissions
	IssueCategoryDependencies  = types.IssueCategoryDependencies
	IssueCategoryConfiguration = types.IssueCategoryConfiguration
	IssueCategoryCorruption    = types.IssueCategoryCorruption
)

type RuntimeDefinition = types.RuntimeDefinition

type InstallMethod = types.InstallMethod

type DefaultRuntimeInstaller struct {
	strategies map[string]types.RuntimePlatformStrategy
	registry   *RuntimeRegistry
}

func NewRuntimeInstaller() *DefaultRuntimeInstaller {
	installer := &DefaultRuntimeInstaller{
		strategies: make(map[string]types.RuntimePlatformStrategy),
		registry:   NewRuntimeRegistry(),
	}

	// Add defensive nil checks for strategy creation
	if windowsStrategy := NewWindowsRuntimeStrategy(); windowsStrategy != nil {
		installer.strategies["windows"] = windowsStrategy
	}

	if linuxStrategy, err := NewLinuxRuntimeStrategy(); err == nil && linuxStrategy != nil {
		installer.strategies["linux"] = linuxStrategy
	}

	if macosStrategy := NewMacOSRuntimeStrategy(); macosStrategy != nil {
		installer.strategies["darwin"] = macosStrategy
	}

	return installer
}

func (r *DefaultRuntimeInstaller) Install(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
	startTime := time.Now()

	// Validate runtime against registry
	runtimeDef, err := r.registry.GetRuntime(runtime)
	if err != nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  runtime,
			Version:  options.Version,
			Path:     "",
			Method:   "",
			Duration: time.Since(startTime),
			Errors:   []string{fmt.Sprintf("unknown runtime: %s", runtime)},
			Warnings: []string{},
			Messages: []string{},
			Details:  map[string]interface{}{"runtime": runtime, "error": err.Error()},
		}, NewInstallerError(InstallerErrorTypeNotFound, runtime, fmt.Sprintf("unknown runtime: %s", runtime), err)
	}

	// Determine platform from options, fallback to runtime.GOOS if not specified
	platform := options.Platform
	if platform == "" {
		platform = rt.GOOS
	}

	// Get platform strategy
	strategy := r.GetPlatformStrategy(platform)
	if strategy == nil {
		return &types.InstallResult{
			Success:  false,
			Runtime:  runtime,
			Version:  options.Version,
			Path:     "",
			Method:   "",
			Duration: time.Since(startTime),
			Errors:   []string{fmt.Sprintf("no platform strategy available for %s", platform)},
			Warnings: []string{},
			Messages: []string{},
			Details:  map[string]interface{}{"runtime": runtime, "platform": platform},
		}, NewInstallerError(InstallerErrorTypeUnsupported, runtime, fmt.Sprintf("platform %s not supported", platform), nil)
	}

	// Prepare installation options - use recommended version if none specified
	installOptions := options
	if installOptions.Version == "" {
		installOptions.Version = runtimeDef.RecommendedVersion
	}

	// Set default timeout if not specified
	if installOptions.Timeout == 0 {
		installOptions.Timeout = 5 * time.Minute
	}

	// If not forced, check if runtime is already installed and compatible
	if !options.Force {
		if verifyResult, verifyErr := strategy.VerifyRuntime(runtime); verifyErr == nil && verifyResult.Installed && verifyResult.Compatible {
			// Runtime already installed and working
			return &types.InstallResult{
				Success:  true,
				Runtime:  runtime,
				Version:  verifyResult.Version,
				Path:     verifyResult.Path,
				Method:   "already_installed",
				Duration: time.Since(startTime),
				Errors:   []string{},
				Warnings: []string{},
				Messages: []string{"Runtime already installed and verified"},
				Details: map[string]interface{}{
					"runtime":      runtime,
					"platform":     platform,
					"verified":     true,
					"skip_reason":  "already_installed",
					"installed_at": startTime,
					"verification": verifyResult,
				},
			}, nil
		}
	}

	// Perform the installation using platform strategy
	result, err := strategy.InstallRuntime(runtime, installOptions)
	if err != nil {
		// Installation failed
		duration := time.Since(startTime)

		// Enhance error details if result exists
		if result != nil {
			result.Duration = duration
			result.Runtime = runtime
			if len(result.Details) == 0 {
				result.Details = make(map[string]interface{})
			}
			result.Details["runtime"] = runtime
			result.Details["platform"] = platform
			result.Details["attempted_version"] = installOptions.Version
			result.Details["error"] = err.Error()
			result.Details["installed_at"] = startTime

			return result, NewInstallerError(InstallerErrorTypeInstallation, runtime, fmt.Sprintf("installation failed: %v", err), err)
		}

		// Create error result if strategy didn't return one
		return &types.InstallResult{
			Success:  false,
			Runtime:  runtime,
			Version:  installOptions.Version,
			Path:     "",
			Method:   "",
			Duration: duration,
			Errors:   []string{err.Error()},
			Warnings: []string{},
			Messages: []string{},
			Details: map[string]interface{}{
				"runtime":           runtime,
				"platform":          platform,
				"attempted_version": installOptions.Version,
				"error":             err.Error(),
				"installed_at":      startTime,
			},
		}, NewInstallerError(InstallerErrorTypeInstallation, runtime, fmt.Sprintf("installation failed: %v", err), err)
	}

	// Installation completed - enhance result with additional metadata
	if result != nil {
		result.Duration = time.Since(startTime)
		result.Runtime = runtime

		// Initialize Details map if nil
		if result.Details == nil {
			result.Details = make(map[string]interface{})
		}

		// Add comprehensive metadata
		result.Details["runtime"] = runtime
		result.Details["platform"] = platform
		result.Details["installed_at"] = startTime
		result.Details["runtime_definition"] = runtimeDef

		// Post-installation verification for successful installs
		if result.Success {
			if verifyResult, verifyErr := strategy.VerifyRuntime(runtime); verifyErr == nil {
				result.Details["post_install_verification"] = verifyResult

				// Update result with verified information if missing
				if result.Version == "" && verifyResult.Version != "" {
					result.Version = verifyResult.Version
				}
				if result.Path == "" && verifyResult.Path != "" {
					result.Path = verifyResult.Path
				}

				// Add warning if verification failed
				if !verifyResult.Installed || !verifyResult.Compatible {
					result.Warnings = append(result.Warnings, "Installation completed but post-install verification failed")
					if len(verifyResult.Issues) > 0 {
						for _, issue := range verifyResult.Issues {
							result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", issue.Title, issue.Description))
						}
					}
				}
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Post-install verification failed: %v", verifyErr))
				result.Details["verification_error"] = verifyErr.Error()
			}
		}

		return result, nil
	}

	// Should not reach here, but handle unexpected nil result
	return &types.InstallResult{
		Success:  false,
		Runtime:  runtime,
		Version:  installOptions.Version,
		Path:     "",
		Method:   "",
		Duration: time.Since(startTime),
		Errors:   []string{"installation returned nil result"},
		Warnings: []string{},
		Messages: []string{},
		Details: map[string]interface{}{
			"runtime":      runtime,
			"platform":     platform,
			"installed_at": startTime,
		},
	}, NewInstallerError(InstallerErrorTypeUnknown, runtime, "installation returned nil result", nil)
}

func (r *DefaultRuntimeInstaller) Verify(runtime string) (*types.VerificationResult, error) {
	startTime := time.Now()

	runtimeDef, err := r.registry.GetRuntime(runtime)
	if err != nil {
		return nil, NewInstallerError(InstallerErrorTypeNotFound, runtime,
			fmt.Sprintf("unknown runtime: %s", runtime), err)
	}

	result := &types.VerificationResult{
		Installed:       false,
		Compatible:      false,
		Version:         "",
		Path:            "",
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
		Metadata:        make(map[string]interface{}),
	}
	result.Details["verified_at"] = startTime
	result.Details["runtime"] = runtime

	switch runtime {
	case "go":
		r.verifyGo(result, runtimeDef)
	case RuntimePython:
		r.verifyPython(result, runtimeDef)
	case RuntimeNodeJS:
		r.verifyNodejs(result, runtimeDef)
	case RuntimeJava:
		r.verifyJava(result, runtimeDef)
	case "java21":
		r.verifyJava21(result, runtimeDef)
	default:
		return nil, NewInstallerError(InstallerErrorTypeUnsupported, runtime,
			fmt.Sprintf("verification not supported for runtime: %s", runtime), nil)
	}

	result.Details["duration"] = time.Since(startTime)

	r.generateRecommendations(result)

	return result, nil
}

func (r *DefaultRuntimeInstaller) GetPlatformStrategy(platform string) types.RuntimePlatformStrategy {
	if strategy, exists := r.strategies[platform]; exists {
		return strategy
	}

	// Safe fallback - check linux strategy exists before returning
	if linuxStrategy, exists := r.strategies["linux"]; exists {
		return linuxStrategy
	}

	// Final fallback - try other available strategies
	for _, strategy := range r.strategies {
		if strategy != nil {
			return strategy
		}
	}

	// Return nil if no strategies available - caller must handle this
	return nil
}

func (r *DefaultRuntimeInstaller) GetSupportedRuntimes() []string {
	return []string{"go", "python", "nodejs", "java", "java21"}
}

func (r *DefaultRuntimeInstaller) GetRuntimeInfo(runtime string) (*types.RuntimeDefinition, error) {
	return r.registry.GetRuntime(runtime)
}

func (r *DefaultRuntimeInstaller) ValidateVersion(_ string, minVersion string) (*types.VersionValidationResult, error) {
	return &types.VersionValidationResult{
		Valid:            false,
		RequiredVersion:  minVersion,
		InstalledVersion: "",
		Issues:           []types.Issue{},
	}, fmt.Errorf("not implemented")
}

type RuntimeRegistry struct {
	runtimes map[string]*types.RuntimeDefinition
}

func NewRuntimeRegistry() *RuntimeRegistry {
	registry := &RuntimeRegistry{
		runtimes: make(map[string]*types.RuntimeDefinition),
	}

	registry.registerDefaults()

	return registry
}

func (r *RuntimeRegistry) registerDefaults() {
	r.runtimes["go"] = &types.RuntimeDefinition{
		Name:               "go",
		DisplayName:        "Go Programming Language",
		MinVersion:         "1.19.0",
		RecommendedVersion: "1.21.0",
		InstallMethods:     map[string]types.InstallMethod{},
		VerificationCmd:    []string{"go", "version"},
		EnvVars:            map[string]string{},
	}

	r.runtimes["python"] = &types.RuntimeDefinition{
		Name:               "python",
		DisplayName:        "Python Programming Language",
		MinVersion:         "3.9.0",
		RecommendedVersion: "3.11.0",
		InstallMethods:     map[string]types.InstallMethod{},
		VerificationCmd:    []string{"python3", "--version"},
		EnvVars:            map[string]string{},
	}

	r.runtimes["nodejs"] = &types.RuntimeDefinition{
		Name:               "nodejs",
		DisplayName:        "Node.js JavaScript Runtime",
		MinVersion:         "22.0.0",
		RecommendedVersion: "22.0.0",
		InstallMethods:     map[string]types.InstallMethod{},
		VerificationCmd:    []string{"node", "--version"},
		EnvVars:            map[string]string{},
	}

	r.runtimes["java"] = &types.RuntimeDefinition{
		Name:               "java",
		DisplayName:        "Java Development Kit",
		MinVersion:         "17.0.0",
		RecommendedVersion: "21.0.0",
		InstallMethods:     map[string]types.InstallMethod{},
		VerificationCmd:    []string{"java", "-version"},
		EnvVars:            map[string]string{},
	}

	r.runtimes["java21"] = &types.RuntimeDefinition{
		Name:               "java21",
		DisplayName:        "Java 21 Runtime",
		MinVersion:         "21.0.0",
		RecommendedVersion: "21.0.0",
		InstallMethods:     map[string]types.InstallMethod{},
		VerificationCmd:    []string{"java", "-version"},
		EnvVars:            map[string]string{},
	}
}

func (r *RuntimeRegistry) GetRuntime(name string) (*types.RuntimeDefinition, error) {
	if runtime, exists := r.runtimes[name]; exists {
		return runtime, nil
	}
	return nil, &InstallerError{
		Type:    InstallerErrorTypeNotFound,
		Message: "runtime not found: " + name,
	}
}

func (r *RuntimeRegistry) RegisterRuntime(runtime *types.RuntimeDefinition) {
	r.runtimes[runtime.Name] = runtime
}

func (r *RuntimeRegistry) ListRuntimes() []string {
	names := make([]string, 0, len(r.runtimes))
	for name := range r.runtimes {
		names = append(names, name)
	}
	return names
}

func (r *DefaultRuntimeInstaller) verifyGo(result *types.VerificationResult, def *types.RuntimeDefinition) {
	r.verifyBasicInstallation(result, def)
	if !result.Installed {
		return
	}

	r.verifyGoEnvironment(result)

	r.verifyGoToolchain(result)

	r.verifyGoModules(result)
}

func (r *DefaultRuntimeInstaller) verifyPython(result *types.VerificationResult, def *types.RuntimeDefinition) {
	r.verifyBasicInstallation(result, def)
	if !result.Installed {
		return
	}

	r.verifyPythonEnvironment(result)

	r.verifyPythonPip(result)

	r.verifyPythonPackageInstallation(result)
}

func (r *DefaultRuntimeInstaller) verifyNodejs(result *types.VerificationResult, def *types.RuntimeDefinition) {
	r.verifyBasicInstallation(result, def)
	if !result.Installed {
		return
	}

	r.verifyNodejsEnvironment(result)

	r.verifyNodejsNpm(result)

	r.verifyNodejsPackageInstallation(result)
}

func (r *DefaultRuntimeInstaller) verifyJava(result *types.VerificationResult, def *types.RuntimeDefinition) {
	r.verifyBasicInstallation(result, def)
	if !result.Installed {
		return
	}

	r.verifyJavaEnvironment(result)

	r.verifyJavaCompiler(result)

	r.verifyJavaDevelopmentTools(result)
}

func (r *DefaultRuntimeInstaller) verifyJava21(result *types.VerificationResult, def *types.RuntimeDefinition) {
	// Use the comprehensive Java 21 verification utilities
	java21Result, err := VerifyJava21Installation()
	if err != nil {
		r.addIssue(result, types.IssueSeverityCritical, types.IssueCategoryInstallation,
			"Java 21 Verification Failed",
			fmt.Sprintf("Failed to verify Java 21 installation: %v", err),
			"Check Java 21 installation and try reinstalling",
			map[string]interface{}{"error": err.Error()})
		return
	}

	// Map Java 21 verification results to standard VerificationResult format
	result.Installed = java21Result.Installed
	result.Compatible = java21Result.Compatible
	result.Version = java21Result.Version
	result.Path = java21Result.Path
	result.Runtime = java21Result.Runtime
	result.VerifiedAt = java21Result.VerifiedAt
	result.Duration = java21Result.Duration

	// Copy environment variables
	for key, value := range java21Result.EnvironmentVars {
		result.EnvironmentVars[key] = value
	}

	// Copy metadata
	for key, value := range java21Result.Metadata {
		result.Metadata[key] = value
	}

	// Copy detailed information
	for key, value := range java21Result.Details {
		result.Details[key] = value
	}

	// Convert Java 21 issues to standard issues format
	for _, java21Issue := range java21Result.Issues {
		result.Issues = append(result.Issues, types.Issue{
			Severity:    java21Issue.Severity,
			Category:    java21Issue.Category,
			Title:       java21Issue.Title,
			Description: java21Issue.Description,
			Solution:    java21Issue.Solution,
			Details:     java21Issue.Details,
		})
	}

	// Add Java 21 specific recommendations
	if len(java21Result.Recommendations) > 0 {
		result.Details["java21_recommendations"] = java21Result.Recommendations
	}

	// Set runtime-specific details
	result.Details["java21_verification"] = true
	result.Details["java21_executable_path"] = GetJava21ExecutablePath()
	result.Details["java21_install_path"] = GetJava21InstallPath()
}
