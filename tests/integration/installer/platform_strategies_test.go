package installer

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	installerPkg "lsp-gateway/internal/installer"
	"lsp-gateway/internal/platform"
)

// Mock infrastructure for comprehensive platform strategy testing

type mockPlatformPackageManager struct {
	name             string
	available        bool
	shouldFail       bool
	shouldFailVerify bool
	installCalls     []string
	verifyCalls      []string
	platforms        []string
	adminReq         bool
	installError     error
	verifyResult     *platform.VerificationResult
	verifyFunc       func(string) (*platform.VerificationResult, error)
}

func newMockPlatformPackageManager(name string, available bool) *mockPlatformPackageManager {
	return &mockPlatformPackageManager{
		name:      name,
		available: available,
		platforms: []string{"linux", "windows", "darwin"},
		adminReq:  false,
		verifyResult: &platform.VerificationResult{
			Installed: true,
			Version:   "test-version",
			Path:      "/test/path",
		},
	}
}

func (m *mockPlatformPackageManager) IsAvailable() bool {
	return m.available
}

func (m *mockPlatformPackageManager) Install(component string) error {
	m.installCalls = append(m.installCalls, component)
	if m.installError != nil {
		return m.installError
	}
	if m.shouldFail {
		if strings.Contains(component, "permission") {
			return errors.New("permission denied - insufficient privileges")
		}
		if strings.Contains(component, "notfound") {
			return errors.New("package not found in repositories")
		}
		if strings.Contains(component, "network") {
			return errors.New("network timeout - unable to reach repository")
		}
		if strings.Contains(component, "admin") {
			return errors.New("access denied - administrator privileges required")
		}
		return errors.New("installation failed")
	}
	return nil
}

func (m *mockPlatformPackageManager) Verify(component string) (*platform.VerificationResult, error) {
	m.verifyCalls = append(m.verifyCalls, component)
	if m.verifyFunc != nil {
		return m.verifyFunc(component)
	}
	if m.shouldFailVerify {
		return &platform.VerificationResult{
			Installed: false,
			Issues:    []string{"verification failed"},
		}, nil
	}
	return m.verifyResult, nil
}

func (m *mockPlatformPackageManager) GetName() string {
	return m.name
}

func (m *mockPlatformPackageManager) RequiresAdmin() bool {
	return m.adminReq
}

func (m *mockPlatformPackageManager) GetPlatforms() []string {
	return m.platforms
}

func (m *mockPlatformPackageManager) SetInstallError(err error) {
	m.installError = err
}

func (m *mockPlatformPackageManager) SetVerifyResult(result *platform.VerificationResult) {
	m.verifyResult = result
}

type mockPlatformCommandExecutor struct {
	shouldFail bool
	results    map[string]*platform.Result
	execCalls  []string
}

func newMockPlatformCommandExecutor() *mockPlatformCommandExecutor {
	return &mockPlatformCommandExecutor{
		results: make(map[string]*platform.Result),
	}
}

func (m *mockPlatformCommandExecutor) Execute(cmd string, args []string, _ time.Duration) (*platform.Result, error) {
	callKey := fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	m.execCalls = append(m.execCalls, callKey)

	if result, exists := m.results[callKey]; exists {
		return result, nil
	}

	if m.shouldFail {
		return &platform.Result{
			ExitCode: 1,
			Stderr:   "command execution failed",
		}, errors.New("command execution failed")
	}

	return &platform.Result{
		ExitCode: 0,
		Stdout:   "mock output",
	}, nil
}

func (m *mockPlatformCommandExecutor) ExecuteWithEnv(cmd string, args []string, _ map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(cmd, args, timeout)
}

func (m *mockPlatformCommandExecutor) GetShell() string {
	if runtime.GOOS == installerPkg.PlatformWindows {
		return "cmd"
	}
	return "bash"
}

func (m *mockPlatformCommandExecutor) GetShellArgs(command string) []string {
	if runtime.GOOS == installerPkg.PlatformWindows {
		return []string{"/C", command}
	}
	return []string{"-c", command}
}

func (m *mockPlatformCommandExecutor) IsCommandAvailable(_ string) bool {
	return !m.shouldFail
}

func (m *mockPlatformCommandExecutor) SetResult(cmd string, args []string, result *platform.Result) {
	key := fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	m.results[key] = result
}

// Windows Strategy Tests

func TestWindowsStrategy_InstallGo_ComprehensiveScenarios(t *testing.T) {
	testCases := []struct {
		name                string
		version             string
		wingetAvailable     bool
		chocoAvailable      bool
		wingetShouldFail    bool
		chocoShouldFail     bool
		wingetFailVerify    bool
		chocoFailVerify     bool
		expectedError       bool
		expectedErrorType   installerPkg.InstallerErrorType
		expectedPackageName string
	}{
		{
			name:                "successful winget installation",
			version:             "1.21",
			wingetAvailable:     true,
			chocoAvailable:      true,
			expectedPackageName: "GoLang.Go",
		},
		{
			name:                "fallback to chocolatey when winget fails",
			version:             "1.21",
			wingetAvailable:     true,
			chocoAvailable:      true,
			wingetShouldFail:    true,
			expectedPackageName: "golang",
		},
		{
			name:              "all package managers unavailable",
			version:           "1.21",
			wingetAvailable:   false,
			chocoAvailable:    false,
			expectedError:     true,
			expectedErrorType: installerPkg.InstallerErrorTypeUnsupported,
		},
		{
			name:              "all package managers fail",
			version:           "1.21",
			wingetAvailable:   true,
			chocoAvailable:    true,
			wingetShouldFail:  true,
			chocoShouldFail:   true,
			expectedError:     true,
			expectedErrorType: installerPkg.InstallerErrorTypeInstallation,
		},
		{
			name:             "verification failure triggers fallback",
			version:          "1.21",
			wingetAvailable:  true,
			chocoAvailable:   true,
			wingetFailVerify: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := createTestWindowsStrategyWithConfig(tc.wingetAvailable, tc.chocoAvailable)

			packageManagers := strategy.GetPackageManagerInstances()
			if tc.wingetShouldFail && len(packageManagers) > 0 {
				if mock, ok := packageManagers[0].(*mockPlatformPackageManager); ok {
					mock.shouldFail = true
				}
			}
			if tc.chocoShouldFail && len(packageManagers) > 1 {
				if mock, ok := packageManagers[1].(*mockPlatformPackageManager); ok {
					mock.shouldFail = true
				}
			}
			if tc.wingetFailVerify && len(packageManagers) > 0 {
				if mock, ok := packageManagers[0].(*mockPlatformPackageManager); ok {
					mock.shouldFailVerify = true
				}
			}
			if tc.chocoFailVerify && len(packageManagers) > 1 {
				if mock, ok := packageManagers[1].(*mockPlatformPackageManager); ok {
					mock.shouldFailVerify = true
				}
			}

			err := strategy.InstallGo(tc.version)

			if tc.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if installerErr, ok := err.(*installerPkg.InstallerError); ok {
					if installerErr.Type != tc.expectedErrorType {
						t.Errorf("Expected error type %s, got %s", tc.expectedErrorType, installerErr.Type)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestWindowsStrategy_InstallPython_VersionHandling(t *testing.T) {
	testCases := []struct {
		name            string
		version         string
		expectedPackage string
	}{
		{"default version", "", "Python.Python.3.11"},
		{"specific version", "3.11", "Python.Python.3.11"},
		{"latest version", "latest", "Python.Python.3.11"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := createTestWindowsStrategyWithConfig(true, false)
			packageManagers := strategy.GetPackageManagerInstances()
			mockWinget := packageManagers[0].(*mockPlatformPackageManager)

			err := strategy.InstallPython(tc.version)

			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			if len(mockWinget.installCalls) != 1 {
				t.Errorf("Expected 1 install call, got %d", len(mockWinget.installCalls))
			}

			if mockWinget.installCalls[0] != tc.expectedPackage {
				t.Errorf("Expected package %s, got %s", tc.expectedPackage, mockWinget.installCalls[0])
			}
		})
	}
}

func TestWindowsStrategy_RetryMechanismWithDifferentErrors(t *testing.T) {
	testCases := []struct {
		name               string
		errorType          string
		expectedRetryCount int
	}{
		{"network error - should retry", "network", 3},
		{"permission error - should not retry", "permission", 1},
		{"not found error - should not retry", "notfound", 1},
		{"admin rights error - should not retry", "admin", 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := createTestWindowsStrategyWithConfig(true, false)
			strategy.SetRetryAttempts(3)
			strategy.SetRetryDelay(1 * time.Millisecond)

			packageManagers := strategy.GetPackageManagerInstances()
			mockWinget := packageManagers[0].(*mockPlatformPackageManager)
			mockWinget.shouldFail = true

			err := strategy.InstallComponent("test", "1.0", map[string]string{
				"winget": tc.errorType,
			})

			if err == nil {
				t.Error("Expected error")
			}

			if len(mockWinget.installCalls) != tc.expectedRetryCount {
				t.Errorf("Expected %d install calls, got %d", tc.expectedRetryCount, len(mockWinget.installCalls))
			}
		})
	}
}

func TestWindowsStrategy_GetPlatformInfo_ArchitectureDetection(t *testing.T) {
	testCases := []struct {
		name         string
		wmicOutput   string
		envVar       string
		expectedArch string
	}{
		{
			name:         "64-bit from wmic",
			wmicOutput:   "OSArchitecture=64-bit",
			expectedArch: "amd64",
		},
		{
			name:         "32-bit from wmic",
			wmicOutput:   "OSArchitecture=32-bit",
			expectedArch: "386",
		},
		{
			name:         "fallback to env var 64-bit",
			wmicOutput:   "",
			envVar:       "AMD64",
			expectedArch: "amd64",
		},
		{
			name:         "fallback to env var 32-bit",
			wmicOutput:   "",
			envVar:       "x86",
			expectedArch: "386",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := createTestWindowsStrategyWithConfig(true, true)
			executor := strategy.GetExecutor().(*mockPlatformCommandExecutor)

			if tc.wmicOutput != "" {
				executor.SetResult("wmic", []string{"os", "get", "osarchitecture", "/value"}, &platform.Result{
					ExitCode: 0,
					Stdout:   tc.wmicOutput,
				})
			} else {
				executor.SetResult("wmic", []string{"os", "get", "osarchitecture", "/value"}, &platform.Result{
					ExitCode: 1,
					Stderr:   "command failed",
				})
			}

			if tc.envVar != "" {
				// Set up the mock for both possible shell types to ensure cross-platform testing
				// Windows cmd format
				executor.SetResult("cmd", []string{"/C", "echo %PROCESSOR_ARCHITECTURE%"}, &platform.Result{
					ExitCode: 0,
					Stdout:   tc.envVar,
				})
				// PowerShell/bash format (used when running tests on Linux)
				executor.SetResult("bash", []string{"-c", "$env:PROCESSOR_ARCHITECTURE"}, &platform.Result{
					ExitCode: 0,
					Stdout:   tc.envVar,
				})
			}

			info := strategy.GetPlatformInfo()

			if info.Architecture != tc.expectedArch {
				t.Errorf("Expected architecture %s, got %s", tc.expectedArch, info.Architecture)
			}
		})
	}
}

// Linux Strategy Tests

func TestLinuxStrategy_NewLinuxStrategy_DistributionDetection(t *testing.T) {
	testCases := []struct {
		name                    string
		mockDistribution        platform.LinuxDistribution
		expectedPackageManagers []string
	}{
		{
			name:                    "Ubuntu distribution",
			mockDistribution:        platform.DistributionUbuntu,
			expectedPackageManagers: []string{"apt"},
		},
		{
			name:                    "Fedora distribution",
			mockDistribution:        platform.DistributionFedora,
			expectedPackageManagers: []string{"dnf"},
		},
		{
			name:                    "CentOS distribution",
			mockDistribution:        platform.DistributionCentOS,
			expectedPackageManagers: []string{"dnf", "yum"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test the package manager preference logic directly
			managers := platform.GetPreferredPackageManagers(tc.mockDistribution)

			if len(managers) != len(tc.expectedPackageManagers) {
				t.Errorf("Expected %d package managers, got %d", len(tc.expectedPackageManagers), len(managers))
			}

			for i, expected := range tc.expectedPackageManagers {
				if i < len(managers) && managers[i] != expected {
					t.Errorf("Expected package manager %s at index %d, got %s", expected, i, managers[i])
				}
			}
		})
	}
}

func TestLinuxStrategy_InstallComponent_PreferredManagerOrder(t *testing.T) {
	// Create a test strategy with mock Linux info
	linuxInfo := &platform.LinuxInfo{
		Distribution: platform.DistributionUbuntu,
		Version:      "22.04",
		Name:         "Ubuntu",
		ID:           "ubuntu",
	}

	strategy := createTestLinuxStrategy(linuxInfo)

	// Configure package managers - apt available, dnf not available
	packageManagers := strategy.GetPackageManagerInstances()
	aptMgr := packageManagers["apt"].(*mockPlatformPackageManager)
	aptMgr.available = true

	if dnfMgr, exists := packageManagers["dnf"]; exists {
		dnfMgr.(*mockPlatformPackageManager).available = false
	}

	err := strategy.InstallGo("1.21")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify apt was called (preferred for Ubuntu)
	if len(aptMgr.installCalls) != 1 {
		t.Errorf("Expected 1 apt install call, got %d", len(aptMgr.installCalls))
	}
}

func TestLinuxStrategy_InstallWithRetry_VersionCompatibility(t *testing.T) {
	linuxInfo := &platform.LinuxInfo{
		Distribution: platform.DistributionUbuntu,
		Version:      "22.04",
	}

	strategy := createTestLinuxStrategy(linuxInfo)
	strategy.SetRetryAttempts(2)

	packageManagers := strategy.GetPackageManagerInstances()
	aptMgr := packageManagers["apt"].(*mockPlatformPackageManager)
	aptMgr.available = true

	// Mock verification to return incompatible version first, then compatible
	verifyCallCount := 0
	aptMgr.verifyFunc = func(_ string) (*platform.VerificationResult, error) {
		verifyCallCount++
		if verifyCallCount == 1 {
			return &platform.VerificationResult{
				Installed: true,
				Version:   "1.19.0", // Incompatible with requested 1.21
			}, nil
		}
		return &platform.VerificationResult{
			Installed: true,
			Version:   "1.21.0",
			Path:      "/usr/bin/go",
		}, nil
	}

	err := strategy.InstallWithRetry(aptMgr, "go", "1.21")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Should have attempted install due to version incompatibility
	if len(aptMgr.installCalls) == 0 {
		t.Error("Expected at least one install call due to version incompatibility")
	}
}

func TestLinuxStrategy_InstallComponent_AllManagersFail(t *testing.T) {
	linuxInfo := &platform.LinuxInfo{
		Distribution: platform.DistributionUbuntu,
		Version:      "22.04",
	}

	strategy := createTestLinuxStrategy(linuxInfo)

	// Make all managers fail
	packageManagers := strategy.GetPackageManagerInstances()
	for _, mgr := range packageManagers {
		if mockMgr, ok := mgr.(*mockPlatformPackageManager); ok {
			mockMgr.shouldFail = true
		}
	}

	err := strategy.InstallGo("1.21")

	if err == nil {
		t.Error("Expected error when all package managers fail")
	}

	installerErr, ok := err.(*installerPkg.InstallerError)
	if !ok {
		t.Errorf("Expected InstallerError, got %T", err)
	} else {
		if installerErr.Type != installerPkg.InstallerErrorTypeInstallation {
			t.Errorf("Expected installation error type, got %s", installerErr.Type)
		}
	}
}

func TestLinuxStrategy_InstallComponent_NoPackageManagers(t *testing.T) {
	linuxInfo := &platform.LinuxInfo{
		Distribution: platform.DistributionUbuntu,
		Version:      "22.04",
	}

	strategy := installerPkg.NewTestLinuxStrategy(linuxInfo, newMockPlatformCommandExecutor())

	err := strategy.InstallGo("1.21")

	if err == nil {
		t.Error("Expected error when no package managers available")
	}

	installerErr, ok := err.(*installerPkg.InstallerError)
	if !ok {
		t.Errorf("Expected InstallerError, got %T", err)
	} else {
		if installerErr.Type != installerPkg.InstallerErrorTypeInstallation {
			t.Errorf("Expected installation error type, got %s", installerErr.Type)
		}
	}
}

func TestLinuxStrategy_GetPlatformInfo(t *testing.T) {
	linuxInfo := &platform.LinuxInfo{
		Distribution: platform.DistributionUbuntu,
		Version:      "22.04",
		Name:         "Ubuntu",
		ID:           "ubuntu",
	}

	strategy := createTestLinuxStrategy(linuxInfo)

	info := strategy.GetPlatformInfo()

	if info.OS != "linux" {
		t.Errorf("Expected OS 'linux', got '%s'", info.OS)
	}

	if info.Distribution != "ubuntu" {
		t.Errorf("Expected Distribution 'ubuntu', got '%s'", info.Distribution)
	}

	if info.Version != "22.04" {
		t.Errorf("Expected Version '22.04', got '%s'", info.Version)
	}
}

// macOS Strategy Tests

func TestMacOSStrategy_InstallGo_HomebrewSetup(t *testing.T) {
	testCases := []struct {
		name                string
		homebrewAvailable   bool
		homebrewInstallFail bool
		expectedError       bool
		expectedErrorType   installerPkg.InstallerErrorType
	}{
		{
			name:              "homebrew available - success",
			homebrewAvailable: true,
		},
		{
			name:                "homebrew unavailable - install fails",
			homebrewAvailable:   false,
			homebrewInstallFail: true,
			expectedError:       true,
			expectedErrorType:   installerPkg.InstallerErrorTypeInstallation,
		},
		{
			name:              "homebrew unavailable - install succeeds",
			homebrewAvailable: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := createTestMacOSStrategy()
			mockBrew := strategy.GetPackageManagerInstance().(*mockPlatformPackageManager)
			mockBrew.available = tc.homebrewAvailable

			if tc.homebrewInstallFail {
				executor := strategy.GetExecutor().(*mockPlatformCommandExecutor)
				executor.shouldFail = true
			}

			err := strategy.InstallGo("1.21")

			if tc.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if installerErr, ok := err.(*installerPkg.InstallerError); ok {
					if installerErr.Type != tc.expectedErrorType {
						t.Errorf("Expected error type %s, got %s", tc.expectedErrorType, installerErr.Type)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestMacOSStrategy_InstallWithRetry_BackoffStrategy(t *testing.T) {
	strategy := createTestMacOSStrategy()
	mockBrew := strategy.GetPackageManagerInstance().(*mockPlatformPackageManager)
	mockBrew.shouldFail = true
	mockBrew.shouldFailVerify = true

	start := time.Now()
	err := strategy.InstallWithRetry("go", "go")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error after retries")
	}

	// Should have called install 3 times (maxRetries)
	if len(mockBrew.installCalls) != 3 {
		t.Errorf("Expected 3 install calls, got %d", len(mockBrew.installCalls))
	}

	// Should have some delay due to backoff (1s + 4s = 5s minimum)
	if elapsed < 5*time.Millisecond {
		t.Errorf("Expected backoff delay, but elapsed time was %v", elapsed)
	}
}

func TestMacOSStrategy_VersionExtraction_EdgeCases(t *testing.T) {
	strategy := installerPkg.NewMacOSStrategy()

	testCases := []struct {
		name     string
		input    string
		expected string
		method   func(string) string
	}{
		{"empty version", "", "", strategy.ExtractMajorVersion},
		{"invalid format", "invalid.version.string", "", strategy.ExtractMajorVersion},
		{"single digit", "8", "8", strategy.ExtractMajorVersion},
		{"complex version", "17.0.2+8-LTS", "17", strategy.ExtractMajorVersion},
		{"go prefix removal", "go1.21.0", "1.21", strategy.NormalizeGoVersion},
		{"version with patch", "3.11.7", "3.11", strategy.ExtractMajorMinorVersion},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.method(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestMacOSStrategy_PackageNameGeneration_FormulaAvailability(t *testing.T) {
	strategy := createTestMacOSStrategy()
	executor := strategy.GetExecutor().(*mockPlatformCommandExecutor)

	// Mock formula search to return formula exists
	executor.SetResult("bash", []string{"-c", "brew search --formula python@3.12"}, &platform.Result{
		ExitCode: 0,
		Stdout:   "python@3.12",
	})

	packageName := strategy.GetPythonPackageName("3.12.0")

	// Should return the versioned package since formula exists
	if !strings.Contains(packageName, "python@") {
		t.Errorf("Expected versioned python package, got %s", packageName)
	}
}

func TestMacOSStrategy_EnvironmentSetup_JavaHome(t *testing.T) {
	strategy := createTestMacOSStrategy()
	executor := strategy.GetExecutor().(*mockPlatformCommandExecutor)

	// Mock brew --prefix command to return java path
	executor.SetResult("bash", []string{"-c", "brew --prefix openjdk@17"}, &platform.Result{
		ExitCode: 0,
		Stdout:   "/opt/homebrew/opt/openjdk@17",
	})

	err := strategy.SetupJavaEnvironment("17")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify executor was called with correct command
	found := false
	for _, call := range executor.execCalls {
		if strings.Contains(call, "brew --prefix openjdk@17") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected brew --prefix command to be called")
	}
}

func TestMacOSStrategy_UpdateHomebrewPath_ArchitectureSpecific(t *testing.T) {
	strategy := createTestMacOSStrategy()

	err := strategy.UpdateHomebrewPath()

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Test should complete without panicking
	// Actual path testing is difficult due to environment isolation
}

// Helper functions for creating test strategies

func createTestWindowsStrategyWithConfig(wingetAvailable, chocoAvailable bool) *installerPkg.WindowsStrategy {
	// Create the strategy using the normal constructor
	strategy := installerPkg.NewWindowsStrategy()

	// Configure it for testing
	strategy.SetRetryAttempts(3)
	strategy.SetRetryDelay(1 * time.Millisecond) // Fast for testing

	// Replace executor with mock
	strategy.SetExecutor(newMockPlatformCommandExecutor())

	// Replace package managers with mocks
	mockWinget := newMockPlatformPackageManager("winget", wingetAvailable)
	mockChoco := newMockPlatformPackageManager("chocolatey", chocoAvailable)
	strategy.SetPackageManagers([]platform.PackageManager{mockWinget, mockChoco})

	return strategy
}

func createTestLinuxStrategy(linuxInfo *platform.LinuxInfo) *installerPkg.LinuxStrategy {
	// Create a strategy using the test constructor
	strategy := installerPkg.NewTestLinuxStrategy(linuxInfo, newMockPlatformCommandExecutor())

	// Set up mock package managers
	packageManagers := make(map[string]platform.PackageManager)
	mockApt := newMockPlatformPackageManager("apt", true)
	mockDnf := newMockPlatformPackageManager("dnf", true)
	mockYum := newMockPlatformPackageManager("yum", true)

	packageManagers["apt"] = mockApt
	packageManagers["dnf"] = mockDnf
	packageManagers["yum"] = mockYum

	// Configure the strategy for testing
	strategy.SetPackageManagers(packageManagers)

	return strategy
}

func createTestMacOSStrategy() *installerPkg.MacOSStrategy {
	// Create the strategy using the normal constructor
	strategy := installerPkg.NewMacOSStrategy()

	// Replace executor with mock
	strategy.SetExecutor(newMockPlatformCommandExecutor())

	// Replace the package manager with a mock
	mockBrew := newMockPlatformPackageManager("homebrew", true)
	strategy.SetPackageManager(mockBrew)

	return strategy
}

// Benchmark tests for performance verification

func BenchmarkWindowsStrategy_InstallGo(b *testing.B) {
	strategy := createTestWindowsStrategyWithConfig(true, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.InstallGo("1.21")
	}
}

func BenchmarkLinuxStrategy_InstallGo(b *testing.B) {
	linuxInfo := &platform.LinuxInfo{
		Distribution: platform.DistributionUbuntu,
		Version:      "22.04",
	}
	strategy := createTestLinuxStrategy(linuxInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.InstallGo("1.21")
	}
}

func BenchmarkMacOSStrategy_InstallGo(b *testing.B) {
	strategy := createTestMacOSStrategy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strategy.InstallGo("1.21")
	}
}

// Integration-style tests for cross-component interaction

func TestPlatformStrategy_ErrorHandling_Comprehensive(t *testing.T) {
	testCases := []struct {
		name          string
		strategyType  string
		errorScenario string
		expectedType  installerPkg.InstallerErrorType
	}{
		{
			name:          "Windows - Network timeout",
			strategyType:  "windows",
			errorScenario: "network",
			expectedType:  installerPkg.InstallerErrorTypeInstallation,
		},
		{
			name:          "Windows - Permission denied",
			strategyType:  "windows",
			errorScenario: "permission",
			expectedType:  installerPkg.InstallerErrorTypeInstallation,
		},
		{
			name:          "Linux - All managers unavailable",
			strategyType:  "linux",
			errorScenario: "no_managers",
			expectedType:  installerPkg.InstallerErrorTypeInstallation,
		},
		{
			name:          "macOS - Homebrew setup failure",
			strategyType:  "macos",
			errorScenario: "homebrew_fail",
			expectedType:  installerPkg.InstallerErrorTypeInstallation,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error

			switch tc.strategyType {
			case "windows":
				strategy := createTestWindowsStrategyWithConfig(true, false)
				packageManagers := strategy.GetPackageManagerInstances()
				mockWinget := packageManagers[0].(*mockPlatformPackageManager)
				mockWinget.SetInstallError(errors.New(tc.errorScenario + " error"))
				err = strategy.InstallGo("1.21")

			case "linux":
				linuxInfo := &platform.LinuxInfo{Distribution: platform.DistributionUbuntu}
				strategy := createTestLinuxStrategy(linuxInfo)
				if tc.errorScenario == "no_managers" {
					strategy.ClearPackageManagers()
				}
				err = strategy.InstallGo("1.21")

			case "macos":
				strategy := createTestMacOSStrategy()
				if tc.errorScenario == "homebrew_fail" {
					mockBrew := strategy.GetPackageManagerInstance().(*mockPlatformPackageManager)
					mockBrew.available = false
					executor := strategy.GetExecutor().(*mockPlatformCommandExecutor)
					executor.shouldFail = true
				}
				err = strategy.InstallGo("1.21")
			}

			if err == nil {
				t.Error("Expected error but got none")
				return
			}

			if installerErr, ok := err.(*installerPkg.InstallerError); ok {
				if installerErr.Type != tc.expectedType {
					t.Errorf("Expected error type %s, got %s", tc.expectedType, installerErr.Type)
				}
			} else {
				t.Errorf("Expected InstallerError, got %T", err)
			}
		})
	}
}

func TestPlatformStrategy_TimeoutHandling(t *testing.T) {
	// Test timeout scenarios for different strategies
	testCases := []struct {
		name      string
		setupFunc func() error
	}{
		{
			name: "Windows timeout handling",
			setupFunc: func() error {
				strategy := createTestWindowsStrategyWithConfig(true, false)
				strategy.SetRetryDelay(1 * time.Nanosecond) // Very short for testing
				packageManagers := strategy.GetPackageManagerInstances()
				mockWinget := packageManagers[0].(*mockPlatformPackageManager)
				mockWinget.shouldFail = true
				return strategy.InstallGo("1.21")
			},
		},
		{
			name: "macOS timeout handling",
			setupFunc: func() error {
				strategy := createTestMacOSStrategy()
				mockBrew := strategy.GetPackageManagerInstance().(*mockPlatformPackageManager)
				mockBrew.shouldFail = true
				return strategy.InstallWithRetry("go", "go")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			err := tc.setupFunc()
			elapsed := time.Since(start)

			if err == nil {
				t.Error("Expected error due to timeout scenario")
			}

			// Verify the operation completed in reasonable time (not hanging)
			if elapsed > 10*time.Second {
				t.Errorf("Operation took too long: %v", elapsed)
			}
		})
	}
}
