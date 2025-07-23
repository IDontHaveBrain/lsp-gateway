package installer_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/platform"
)

type mockPackageManager struct {
	name             string
	available        bool
	shouldFail       bool
	shouldFailVerify bool
	installCalls     []string
	verifyCalls      []string
	platforms        []string
	adminReq         bool
}

func newMockPackageManager(name string, available bool) *mockPackageManager {
	return &mockPackageManager{
		name:      name,
		available: available,
		platforms: []string{"windows"},
		adminReq:  false,
	}
}

func (m *mockPackageManager) IsAvailable() bool {
	return m.available
}

func (m *mockPackageManager) Install(component string) error {
	m.installCalls = append(m.installCalls, component)
	if m.shouldFail {
		if strings.Contains(component, "permission") {
			return errors.New("access denied - permission error")
		}
		if strings.Contains(component, "notfound") {
			return errors.New("package not found")
		}
		return errors.New("installation failed")
	}
	return nil
}

func (m *mockPackageManager) Verify(component string) (*platform.VerificationResult, error) {
	m.verifyCalls = append(m.verifyCalls, component)
	if m.shouldFailVerify {
		return &platform.VerificationResult{
			Installed: false,
			Issues:    []string{"verification failed"},
		}, nil
	}
	return &platform.VerificationResult{
		Installed: true,
		Version:   "test-version",
		Path:      "/test/path",
	}, nil
}

func (m *mockPackageManager) GetName() string {
	return m.name
}

func (m *mockPackageManager) RequiresAdmin() bool {
	return m.adminReq
}

func (m *mockPackageManager) GetPlatforms() []string {
	return m.platforms
}

type mockCommandExecutor struct {
	shouldFail bool
	results    map[string]*platform.Result
}

func newMockCommandExecutor() *mockCommandExecutor {
	return &mockCommandExecutor{
		results: make(map[string]*platform.Result),
	}
}

func (m *mockCommandExecutor) Execute(cmd string, args []string, timeout time.Duration) (*platform.Result, error) {
	key := fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	if result, exists := m.results[key]; exists {
		return result, nil
	}

	if m.shouldFail {
		return &platform.Result{
			ExitCode: 1,
			Stderr:   "command failed",
		}, errors.New("command execution failed")
	}

	return &platform.Result{
		ExitCode: 0,
		Stdout:   "mock output",
	}, nil
}

func (m *mockCommandExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(cmd, args, timeout)
}

func (m *mockCommandExecutor) GetShell() string {
	return "cmd"
}

func (m *mockCommandExecutor) GetShellArgs(command string) []string {
	return []string{"/C", command}
}

func (m *mockCommandExecutor) IsCommandAvailable(command string) bool {
	return !m.shouldFail
}

func (m *mockCommandExecutor) SetResult(cmd string, args []string, result *platform.Result) {
	key := fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	m.results[key] = result
}

func TestNewWindowsStrategy(t *testing.T) {
	strategy := NewWindowsStrategy()

	if strategy == nil {
		t.Fatal("NewWindowsStrategy returned nil")
	}

	if len(strategy.packageManagers) != 2 {
		t.Errorf("Expected 2 package managers, got %d", len(strategy.packageManagers))
	}

	if strategy.executor == nil {
		t.Error("Executor should not be nil")
	}

	if strategy.retryAttempts != 3 {
		t.Errorf("Expected 3 retry attempts, got %d", strategy.retryAttempts)
	}

	if strategy.retryDelay != 5*time.Second {
		t.Errorf("Expected 5 second retry delay, got %v", strategy.retryDelay)
	}
}

func TestWindowsStrategy_InstallGo_Success(t *testing.T) {
	strategy := createTestWindowsStrategy()
	mockWinget := strategy.packageManagers[0].(*mockPackageManager)

	err := strategy.InstallGo("1.21")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(mockWinget.installCalls) != 1 {
		t.Errorf("Expected 1 install call, got %d", len(mockWinget.installCalls))
	}

	if mockWinget.installCalls[0] != "GoLang.Go" {
		t.Errorf("Expected install call for 'GoLang.Go', got '%s'", mockWinget.installCalls[0])
	}

	if len(mockWinget.verifyCalls) != 1 {
		t.Errorf("Expected 1 verify call, got %d", len(mockWinget.verifyCalls))
	}
}

func TestWindowsStrategy_InstallPython_Success(t *testing.T) {
	strategy := createTestWindowsStrategy()
	mockWinget := strategy.packageManagers[0].(*mockPackageManager)

	err := strategy.InstallPython("3.11")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(mockWinget.installCalls) != 1 {
		t.Errorf("Expected 1 install call, got %d", len(mockWinget.installCalls))
	}

	if mockWinget.installCalls[0] != "Python.Python.3.11" {
		t.Errorf("Expected install call for 'Python.Python.3.11', got '%s'", mockWinget.installCalls[0])
	}
}

func TestWindowsStrategy_InstallNodejs_Success(t *testing.T) {
	strategy := createTestWindowsStrategy()
	mockWinget := strategy.packageManagers[0].(*mockPackageManager)

	err := strategy.InstallNodejs("18")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(mockWinget.installCalls) != 1 {
		t.Errorf("Expected 1 install call, got %d", len(mockWinget.installCalls))
	}

	if mockWinget.installCalls[0] != "OpenJS.NodeJS" {
		t.Errorf("Expected install call for 'OpenJS.NodeJS', got '%s'", mockWinget.installCalls[0])
	}
}

func TestWindowsStrategy_InstallJava_Success(t *testing.T) {
	strategy := createTestWindowsStrategy()
	mockWinget := strategy.packageManagers[0].(*mockPackageManager)

	err := strategy.InstallJava("17")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(mockWinget.installCalls) != 1 {
		t.Errorf("Expected 1 install call, got %d", len(mockWinget.installCalls))
	}

	if mockWinget.installCalls[0] != "Eclipse.Temurin.17.JDK" {
		t.Errorf("Expected install call for 'Eclipse.Temurin.17.JDK', got '%s'", mockWinget.installCalls[0])
	}
}

func TestWindowsStrategy_FallbackToChocolatey(t *testing.T) {
	strategy := createTestWindowsStrategy()

	mockWinget := strategy.packageManagers[0].(*mockPackageManager)
	mockChoco := strategy.packageManagers[1].(*mockPackageManager)
	mockWinget.available = false
	mockChoco.available = true

	err := strategy.InstallGo("1.21")

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if len(mockWinget.installCalls) != 0 {
		t.Errorf("Expected 0 winget install calls, got %d", len(mockWinget.installCalls))
	}

	if len(mockChoco.installCalls) != 1 {
		t.Errorf("Expected 1 chocolatey install call, got %d", len(mockChoco.installCalls))
	}

	if mockChoco.installCalls[0] != "golang" {
		t.Errorf("Expected chocolatey install call for 'golang', got '%s'", mockChoco.installCalls[0])
	}
}

func TestWindowsStrategy_AllManagersUnavailable(t *testing.T) {
	strategy := createTestWindowsStrategy()

	mockWinget := strategy.packageManagers[0].(*mockPackageManager)
	mockChoco := strategy.packageManagers[1].(*mockPackageManager)
	mockWinget.available = false
	mockChoco.available = false

	err := strategy.InstallGo("1.21")

	if err == nil {
		t.Error("Expected error when no package managers available")
	}

	installerErr, ok := err.(*InstallerError)
	if !ok {
		t.Errorf("Expected InstallerError, got %T", err)
	} else {
		if installerErr.Type != InstallerErrorTypeUnsupported {
			t.Errorf("Expected unsupported error type, got %s", installerErr.Type)
		}
	}
}

func TestWindowsStrategy_RetryLogic(t *testing.T) {
	strategy := createTestWindowsStrategy()
	strategy.retryAttempts = 2
	strategy.retryDelay = 1 * time.Millisecond // Fast test

	mockWinget := strategy.packageManagers[0].(*mockPackageManager)
	mockWinget.shouldFail = true

	mockChoco := strategy.packageManagers[1].(*mockPackageManager)
	mockChoco.available = false

	start := time.Now()
	err := strategy.InstallGo("1.21")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Expected error after retries")
	}

	if len(mockWinget.installCalls) != 2 {
		t.Errorf("Expected 2 install calls (with retry), got %d", len(mockWinget.installCalls))
	}

	if elapsed < 1*time.Millisecond {
		t.Errorf("Expected some delay due to retries, but took %v", elapsed)
	}
}

func TestWindowsStrategy_NonRetryableErrors(t *testing.T) {
	testCases := []struct {
		name        string
		packageName string
		expected    bool
	}{
		{"permission error", "permission", true},
		{"not found error", "notfound", true},
		{"generic error", "generic", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := createTestWindowsStrategy()
			strategy.retryAttempts = 3

			mockWinget := strategy.packageManagers[0].(*mockPackageManager)
			mockWinget.shouldFail = true

			mockChoco := strategy.packageManagers[1].(*mockPackageManager)
			mockChoco.available = false

			err := strategy.installComponent("test", "1.0", map[string]string{
				"winget": tc.packageName,
			})

			if err == nil {
				t.Error("Expected error")
			}

			callCount := len(mockWinget.installCalls)
			if tc.expected && callCount > 1 {
				t.Errorf("Non-retryable error should not retry, but got %d calls", callCount)
			}
			if !tc.expected && callCount != 3 {
				t.Errorf("Retryable error should retry %d times, but got %d calls", 3, callCount)
			}
		})
	}
}

func TestWindowsStrategy_VerificationFailure(t *testing.T) {
	strategy := createTestWindowsStrategy()

	mockWinget := strategy.packageManagers[0].(*mockPackageManager)
	mockWinget.shouldFailVerify = true

	mockChoco := strategy.packageManagers[1].(*mockPackageManager)
	mockChoco.available = false

	err := strategy.InstallGo("1.21")

	if err == nil {
		t.Error("Expected error when verification fails")
	}

	verifyErr, ok := err.(*VerificationError)
	if !ok {
		t.Errorf("Expected VerificationError, got %T", err)
	} else {
		if verifyErr.Component != "go" {
			t.Errorf("Expected component 'go', got '%s'", verifyErr.Component)
		}
	}
}

func TestWindowsStrategy_GetPlatformInfo(t *testing.T) {
	strategy := createTestWindowsStrategy()

	executor := strategy.executor.(*mockCommandExecutor)
	executor.SetResult("wmic", []string{"os", "get", "osarchitecture", "/value"}, &platform.Result{
		ExitCode: 0,
		Stdout:   "OSArchitecture=64-bit",
	})
	executor.SetResult("ver", []string{}, &platform.Result{
		ExitCode: 0,
		Stdout:   "Microsoft Windows [Version 10.0.19041.123]",
	})

	info := strategy.GetPlatformInfo()

	if info.OS != "windows" {
		t.Errorf("Expected OS 'windows', got '%s'", info.OS)
	}

	if info.Distribution != "windows" {
		t.Errorf("Expected Distribution 'windows', got '%s'", info.Distribution)
	}

	if info.Architecture != "amd64" {
		t.Errorf("Expected Architecture 'amd64', got '%s'", info.Architecture)
	}

	if !strings.Contains(info.Version, "Windows") {
		t.Errorf("Expected version to contain 'Windows', got '%s'", info.Version)
	}
}

func TestWindowsStrategy_GetPackageManagers(t *testing.T) {
	strategy := createTestWindowsStrategy()

	managers := strategy.GetPackageManagers()

	if len(managers) != 2 {
		t.Errorf("Expected 2 package managers, got %d", len(managers))
	}

	expectedManagers := []string{"winget", "chocolatey"}
	for i, expected := range expectedManagers {
		if managers[i] != expected {
			t.Errorf("Expected manager %s at index %d, got %s", expected, i, managers[i])
		}
	}
}

func TestWindowsStrategy_IsPackageManagerAvailable(t *testing.T) {
	strategy := createTestWindowsStrategy()

	if !strategy.IsPackageManagerAvailable("winget") {
		t.Error("Expected winget to be available")
	}

	if !strategy.IsPackageManagerAvailable("chocolatey") {
		t.Error("Expected chocolatey to be available")
	}

	if strategy.IsPackageManagerAvailable("nonexistent") {
		t.Error("Expected nonexistent manager to not be available")
	}
}

func TestWindowsStrategy_isNonRetryableError(t *testing.T) {
	strategy := NewWindowsStrategy()

	testCases := []struct {
		name     string
		error    error
		expected bool
	}{
		{"permission error", errors.New("access denied"), true},
		{"not found error", errors.New("package not found"), true},
		{"invalid argument", errors.New("invalid package name"), true},
		{"network error", errors.New("network timeout"), false},
		{"generic error", errors.New("something went wrong"), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := strategy.isNonRetryableError(tc.error)
			if result != tc.expected {
				t.Errorf("Expected %t for error '%s', got %t", tc.expected, tc.error.Error(), result)
			}
		})
	}
}

func createTestWindowsStrategy() *WindowsStrategy {
	mockWinget := newMockPackageManager("winget", true)
	mockChoco := newMockPackageManager("chocolatey", true)
	mockExecutor := newMockCommandExecutor()

	return &WindowsStrategy{
		packageManagers: []platform.PackageManager{mockWinget, mockChoco},
		executor:        mockExecutor,
		retryAttempts:   3,
		retryDelay:      5 * time.Second,
	}
}
