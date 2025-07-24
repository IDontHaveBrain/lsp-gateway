package mocks

import (
	"lsp-gateway/internal/types"
	"time"
)

type MockRuntimeInstaller struct {
	InstallFunc                 func(runtime string, options types.InstallOptions) (*types.InstallResult, error)
	VerifyFunc                  func(runtime string) (*types.VerificationResult, error)
	GetSupportedRuntimesFunc    func() []string
	GetRuntimeInfoFunc          func(runtime string) (*types.RuntimeDefinition, error)
	ValidateVersionFunc         func(runtime, minVersion string) (*types.VersionValidationResult, error)
	GetPlatformStrategyFunc     func(platform string) types.RuntimePlatformStrategy

	InstallCalls                []InstallCall
	VerifyCalls                 []string
	GetSupportedRuntimesCalls   int
	GetRuntimeInfoCalls         []string
	ValidateVersionCalls        []ValidateVersionCall
	GetPlatformStrategyCalls    []string
}

type InstallCall struct {
	Runtime string
	Options types.InstallOptions
}

type ValidateVersionCall struct {
	Runtime   string
	MinVersion string
}

func NewMockRuntimeInstaller() *MockRuntimeInstaller {
	return &MockRuntimeInstaller{
		InstallCalls:                make([]InstallCall, 0),
		VerifyCalls:                 make([]string, 0),
		GetRuntimeInfoCalls:         make([]string, 0),
		ValidateVersionCalls:        make([]ValidateVersionCall, 0),
		GetPlatformStrategyCalls:    make([]string, 0),
	}
}

func (m *MockRuntimeInstaller) Install(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
	m.InstallCalls = append(m.InstallCalls, InstallCall{Runtime: runtime, Options: options})
	if m.InstallFunc != nil {
		return m.InstallFunc(runtime, options)
	}

	return &types.InstallResult{
		Success:  true,
		Runtime:  runtime,
		Version:  "1.24.0",
		Path:     "/usr/local/bin/" + runtime,
		Method:   "package_manager",
		Duration: time.Second * 5,
		Errors:   []string{},
		Warnings: []string{},
		Messages: []string{"Successfully installed " + runtime},
		Details:  map[string]interface{}{"installation_method": "mock"},
	}, nil
}

func (m *MockRuntimeInstaller) Verify(runtime string) (*types.VerificationResult, error) {
	m.VerifyCalls = append(m.VerifyCalls, runtime)
	if m.VerifyFunc != nil {
		return m.VerifyFunc(runtime)
	}

	return &types.VerificationResult{
		Installed:       true,
		Compatible:      true,
		Version:         "1.24.0",
		Path:            "/usr/local/bin/" + runtime,
		Runtime:         runtime,
		Issues:          []types.Issue{},
		Details:         map[string]interface{}{"verified": true},
		Metadata:        map[string]interface{}{"mock_verified": true},
		EnvironmentVars: map[string]string{},
		Recommendations: []string{},
		WorkingDir:      "/tmp",
		AdditionalPaths: []string{},
		VerifiedAt:      time.Now(),
		Duration:        time.Millisecond * 100,
	}, nil
}

func (m *MockRuntimeInstaller) GetSupportedRuntimes() []string {
	m.GetSupportedRuntimesCalls++
	if m.GetSupportedRuntimesFunc != nil {
		return m.GetSupportedRuntimesFunc()
	}
	return []string{"go", "python", "nodejs", "java"}
}

func (m *MockRuntimeInstaller) GetRuntimeInfo(runtime string) (*types.RuntimeDefinition, error) {
	m.GetRuntimeInfoCalls = append(m.GetRuntimeInfoCalls, runtime)
	if m.GetRuntimeInfoFunc != nil {
		return m.GetRuntimeInfoFunc(runtime)
	}

	return &types.RuntimeDefinition{
		Name:               runtime,
		DisplayName:        runtime + " Runtime",
		MinVersion:         "1.0.0",
		RecommendedVersion: "1.24.0",
		InstallMethods: map[string]types.InstallMethod{
			"package_manager": {
				Name:         "Package Manager",
				Platform:     "linux",
				Method:       "apt",
				Commands:     []string{"apt", "install", "-y", runtime},
				Description:  "Install via system package manager",
				Requirements: []string{"sudo"},
			},
		},
		VerificationCmd: []string{runtime, "--version"},
		VersionCommand:  []string{runtime, "--version"},
		EnvVars:         map[string]string{},
		Dependencies:    []string{},
		PostInstall:     []string{},
	}, nil
}

func (m *MockRuntimeInstaller) ValidateVersion(runtime, minVersion string) (*types.VersionValidationResult, error) {
	m.ValidateVersionCalls = append(m.ValidateVersionCalls, ValidateVersionCall{Runtime: runtime, MinVersion: minVersion})
	if m.ValidateVersionFunc != nil {
		return m.ValidateVersionFunc(runtime, minVersion)
	}

	return &types.VersionValidationResult{
		Valid:            true,
		RequiredVersion:  minVersion,
		InstalledVersion: "1.24.0",
		Issues:           []types.Issue{},
	}, nil
}

func (m *MockRuntimeInstaller) GetPlatformStrategy(platform string) types.RuntimePlatformStrategy {
	m.GetPlatformStrategyCalls = append(m.GetPlatformStrategyCalls, platform)
	if m.GetPlatformStrategyFunc != nil {
		return m.GetPlatformStrategyFunc(platform)
	}
	return NewMockRuntimePlatformStrategy()
}

func (m *MockRuntimeInstaller) Reset() {
	m.InstallCalls = make([]InstallCall, 0)
	m.VerifyCalls = make([]string, 0)
	m.GetSupportedRuntimesCalls = 0
	m.GetRuntimeInfoCalls = make([]string, 0)
	m.ValidateVersionCalls = make([]ValidateVersionCall, 0)
	m.GetPlatformStrategyCalls = make([]string, 0)

	m.InstallFunc = nil
	m.VerifyFunc = nil
	m.GetSupportedRuntimesFunc = nil
	m.GetRuntimeInfoFunc = nil
	m.ValidateVersionFunc = nil
	m.GetPlatformStrategyFunc = nil
}

type MockRuntimePlatformStrategy struct {
	InstallRuntimeFunc      func(runtime string, options types.InstallOptions) (*types.InstallResult, error)
	VerifyRuntimeFunc       func(runtime string) (*types.VerificationResult, error)
	GetInstallCommandFunc   func(runtime, version string) ([]string, error)

	InstallRuntimeCalls     []InstallCall
	VerifyRuntimeCalls      []string
	GetInstallCommandCalls  []GetInstallCommandCall
}

type GetInstallCommandCall struct {
	Runtime string
	Version string
}

func NewMockRuntimePlatformStrategy() *MockRuntimePlatformStrategy {
	return &MockRuntimePlatformStrategy{
		InstallRuntimeCalls:    make([]InstallCall, 0),
		VerifyRuntimeCalls:     make([]string, 0),
		GetInstallCommandCalls: make([]GetInstallCommandCall, 0),
	}
}

func (m *MockRuntimePlatformStrategy) InstallRuntime(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
	m.InstallRuntimeCalls = append(m.InstallRuntimeCalls, InstallCall{Runtime: runtime, Options: options})
	if m.InstallRuntimeFunc != nil {
		return m.InstallRuntimeFunc(runtime, options)
	}

	return &types.InstallResult{
		Success:  true,
		Runtime:  runtime,
		Version:  options.Version,
		Path:     "/usr/local/bin/" + runtime,
		Method:   "platform_strategy",
		Duration: time.Second * 3,
		Errors:   []string{},
		Warnings: []string{},
		Messages: []string{"Installed via platform strategy"},
		Details:  map[string]interface{}{"strategy": "mock"},
	}, nil
}

func (m *MockRuntimePlatformStrategy) VerifyRuntime(runtime string) (*types.VerificationResult, error) {
	m.VerifyRuntimeCalls = append(m.VerifyRuntimeCalls, runtime)
	if m.VerifyRuntimeFunc != nil {
		return m.VerifyRuntimeFunc(runtime)
	}

	return &types.VerificationResult{
		Installed:       true,
		Compatible:      true,
		Version:         "1.24.0",
		Path:            "/usr/local/bin/" + runtime,
		Runtime:         runtime,
		Issues:          []types.Issue{},
		Details:         map[string]interface{}{"strategy_verified": true},
		Metadata:        map[string]interface{}{"platform_strategy": true},
		EnvironmentVars: map[string]string{},
		Recommendations: []string{},
		WorkingDir:      "/tmp",
		AdditionalPaths: []string{},
		VerifiedAt:      time.Now(),
		Duration:        time.Millisecond * 50,
	}, nil
}

func (m *MockRuntimePlatformStrategy) GetInstallCommand(runtime, version string) ([]string, error) {
	m.GetInstallCommandCalls = append(m.GetInstallCommandCalls, GetInstallCommandCall{Runtime: runtime, Version: version})
	if m.GetInstallCommandFunc != nil {
		return m.GetInstallCommandFunc(runtime, version)
	}
	return []string{"install", runtime, version}, nil
}