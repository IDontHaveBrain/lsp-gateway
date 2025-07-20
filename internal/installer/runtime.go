package installer

import (
	"fmt"
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
	if windowsStrategy := NewWindowsStrategy(); windowsStrategy != nil {
		installer.strategies["windows"] = &WindowsRuntimeStrategy{strategy: windowsStrategy}
	}
	
	if linuxStrategy, err := NewLinuxStrategy(); err == nil && linuxStrategy != nil {
		installer.strategies["linux"] = &LinuxRuntimeStrategy{strategy: linuxStrategy}
	}
	
	if macosStrategy := NewMacOSStrategy(); macosStrategy != nil {
		installer.strategies["darwin"] = &MacOSRuntimeStrategy{strategy: macosStrategy}
	}

	return installer
}

func (r *DefaultRuntimeInstaller) Install(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
	return &types.InstallResult{
		Success:  false,
		Version:  "",
		Path:     "",
		Method:   "",
		Duration: 0,
		Errors:   []string{"not implemented"},
		Warnings: []string{},
		Details:  map[string]interface{}{"runtime": runtime},
	}, nil
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
	case "python":
		r.verifyPython(result, runtimeDef)
	case "nodejs":
		r.verifyNodejs(result, runtimeDef)
	case "java":
		r.verifyJava(result, runtimeDef)
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
	return []string{"go", "python", "nodejs", "java"}
}

func (r *DefaultRuntimeInstaller) GetRuntimeInfo(runtime string) (*types.RuntimeDefinition, error) {
	return r.registry.GetRuntime(runtime)
}

func (r *DefaultRuntimeInstaller) ValidateVersion(runtime, minVersion string) (*types.VersionValidationResult, error) {
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
		MinVersion:         "3.8.0",
		RecommendedVersion: "3.11.0",
		InstallMethods:     map[string]types.InstallMethod{},
		VerificationCmd:    []string{"python3", "--version"},
		EnvVars:            map[string]string{},
	}

	r.runtimes["nodejs"] = &types.RuntimeDefinition{
		Name:               "nodejs",
		DisplayName:        "Node.js JavaScript Runtime",
		MinVersion:         "18.0.0",
		RecommendedVersion: "20.0.0",
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

type WindowsRuntimeStrategy struct {
	strategy *WindowsStrategy
}

func (w *WindowsRuntimeStrategy) InstallRuntime(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
	var err error
	switch runtime {
	case "go":
		err = w.strategy.InstallGo(options.Version)
	case "python":
		err = w.strategy.InstallPython(options.Version)
	case "nodejs":
		err = w.strategy.InstallNodejs(options.Version)
	case "java":
		err = w.strategy.InstallJava(options.Version)
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	if err != nil {
		return &types.InstallResult{
			Success: false,
			Errors:  []string{err.Error()},
		}, nil
	}

	return &types.InstallResult{
		Success: true,
		Version: options.Version,
	}, nil
}

func (w *WindowsRuntimeStrategy) VerifyRuntime(runtime string) (*types.VerificationResult, error) {
	return &types.VerificationResult{
		Installed:  false,
		Compatible: false,
		Version:    "",
		Path:       "",
		Issues:     []types.Issue{},
		Details:    map[string]interface{}{},
	}, nil
}

func (w *WindowsRuntimeStrategy) GetInstallCommand(runtime, version string) ([]string, error) {
	return []string{}, fmt.Errorf("not implemented")
}

type LinuxRuntimeStrategy struct {
	strategy *LinuxStrategy
}

func (l *LinuxRuntimeStrategy) InstallRuntime(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
	var err error
	switch runtime {
	case "go":
		err = l.strategy.InstallGo(options.Version)
	case "python":
		err = l.strategy.InstallPython(options.Version)
	case "nodejs":
		err = l.strategy.InstallNodejs(options.Version)
	case "java":
		err = l.strategy.InstallJava(options.Version)
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	if err != nil {
		return &types.InstallResult{
			Success: false,
			Errors:  []string{err.Error()},
		}, nil
	}

	return &types.InstallResult{
		Success: true,
		Version: options.Version,
	}, nil
}

func (l *LinuxRuntimeStrategy) VerifyRuntime(runtime string) (*types.VerificationResult, error) {
	return &types.VerificationResult{
		Installed:  false,
		Compatible: false,
		Version:    "",
		Path:       "",
		Issues:     []types.Issue{},
		Details:    map[string]interface{}{},
	}, nil
}

func (l *LinuxRuntimeStrategy) GetInstallCommand(runtime, version string) ([]string, error) {
	return []string{}, fmt.Errorf("not implemented")
}

type MacOSRuntimeStrategy struct {
	strategy *MacOSStrategy
}

func (m *MacOSRuntimeStrategy) InstallRuntime(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
	var err error
	switch runtime {
	case "go":
		err = m.strategy.InstallGo(options.Version)
	case "python":
		err = m.strategy.InstallPython(options.Version)
	case "nodejs":
		err = m.strategy.InstallNodejs(options.Version)
	case "java":
		err = m.strategy.InstallJava(options.Version)
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	if err != nil {
		return &types.InstallResult{
			Success: false,
			Errors:  []string{err.Error()},
		}, nil
	}

	return &types.InstallResult{
		Success: true,
		Version: options.Version,
	}, nil
}

func (m *MacOSRuntimeStrategy) VerifyRuntime(runtime string) (*types.VerificationResult, error) {
	return &types.VerificationResult{
		Installed:  false,
		Compatible: false,
		Version:    "",
		Path:       "",
		Issues:     []types.Issue{},
		Details:    map[string]interface{}{},
	}, nil
}

func (m *MacOSRuntimeStrategy) GetInstallCommand(runtime, version string) ([]string, error) {
	return []string{}, fmt.Errorf("not implemented")
}
