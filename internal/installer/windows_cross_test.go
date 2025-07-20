package installer

import (
	"testing"
	"time"

	"lsp-gateway/internal/platform"
)

func TestWindowsStrategyCrossCompilation(t *testing.T) {
	t.Run("WindowsStrategyCreation", func(t *testing.T) {
		strategy := NewWindowsStrategy()
		if strategy == nil {
			t.Fatal("NewWindowsStrategy returned nil")
		}

		info := strategy.GetPlatformInfo()
		if info == nil {
			t.Error("GetPlatformInfo returned nil")
		}

		managers := strategy.GetPackageManagers()
		if len(managers) == 0 {
			t.Error("GetPackageManagers returned empty slice")
		}

		available := strategy.IsPackageManagerAvailable("nonexistent")
		if available {
			t.Error("Non-existent package manager should not be available")
		}

		t.Logf("Windows strategy created successfully with %d package managers", len(managers))
	})

	t.Run("WindowsStrategyInterface", func(t *testing.T) {
		var strategy PlatformStrategy = NewWindowsStrategy()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Windows strategy method panicked: %v", r)
			}
		}()

		err := strategy.InstallGo("1.21")
		t.Logf("InstallGo result: %v", err)

		err = strategy.InstallPython("3.11")
		t.Logf("InstallPython result: %v", err)

		err = strategy.InstallNodejs("18")
		t.Logf("InstallNodejs result: %v", err)

		err = strategy.InstallJava("17")
		t.Logf("InstallJava result: %v", err)
	})
}

func TestWindowsPackageManagerMock(t *testing.T) {
	t.Run("MockPackageManagerInterface", func(t *testing.T) {
		mockWinget := &mockWindowsPackageManager{
			name:      "winget",
			available: true,
		}

		mockChoco := &mockWindowsPackageManager{
			name:      "chocolatey",
			available: true,
		}

		var pm1 platform.PackageManager = mockWinget
		var pm2 platform.PackageManager = mockChoco

		if !pm1.IsAvailable() {
			t.Error("Mock winget should be available")
		}

		if !pm2.IsAvailable() {
			t.Error("Mock chocolatey should be available")
		}

		if pm1.GetName() != "winget" {
			t.Errorf("Expected winget, got %s", pm1.GetName())
		}

		if pm2.GetName() != "chocolatey" {
			t.Errorf("Expected chocolatey, got %s", pm2.GetName())
		}
	})
}

func TestWindowsCommandExecutorMock(t *testing.T) {
	t.Run("MockCommandExecutorInterface", func(t *testing.T) {
		mockExec := &mockWindowsCommandExecutor{
			shouldFail: false,
			results:    make(map[string]*platform.Result),
		}

		var exec platform.CommandExecutor = mockExec

		result, err := exec.Execute("test", []string{"arg"}, 5*time.Second)
		if err != nil {
			t.Errorf("Mock executor should not fail: %v", err)
		}

		if result.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", result.ExitCode)
		}

		shell := exec.GetShell()
		if shell != "cmd" {
			t.Errorf("Expected cmd shell, got %s", shell)
		}

		available := exec.IsCommandAvailable("test")
		if !available {
			t.Error("Mock executor should report commands as available")
		}
	})
}

type mockWindowsPackageManager struct {
	name         string
	available    bool
	shouldFail   bool
	installCalls []string
	verifyCalls  []string
}

func (m *mockWindowsPackageManager) IsAvailable() bool {
	return m.available
}

func (m *mockWindowsPackageManager) Install(component string) error {
	m.installCalls = append(m.installCalls, component)
	if m.shouldFail {
		return &InstallerError{
			Type:      InstallerErrorTypeInstallation,
			Component: component,
			Message:   "mock installation failed",
		}
	}
	return nil
}

func (m *mockWindowsPackageManager) Verify(component string) (*platform.VerificationResult, error) {
	m.verifyCalls = append(m.verifyCalls, component)
	return &platform.VerificationResult{
		Installed: true,
		Version:   "mock-version",
		Path:      "/mock/path",
	}, nil
}

func (m *mockWindowsPackageManager) GetName() string {
	return m.name
}

func (m *mockWindowsPackageManager) RequiresAdmin() bool {
	return false
}

func (m *mockWindowsPackageManager) GetPlatforms() []string {
	return []string{"windows"}
}

type mockWindowsCommandExecutor struct {
	shouldFail bool
	results    map[string]*platform.Result
}

func (m *mockWindowsCommandExecutor) Execute(cmd string, args []string, timeout time.Duration) (*platform.Result, error) {
	if m.shouldFail {
		return &platform.Result{
				ExitCode: 1,
				Stderr:   "mock command failed",
			}, &InstallerError{
				Type:    InstallerErrorTypeInstallation,
				Message: "mock command execution failed",
			}
	}

	return &platform.Result{
		ExitCode: 0,
		Stdout:   "mock output",
	}, nil
}

func (m *mockWindowsCommandExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(cmd, args, timeout)
}

func (m *mockWindowsCommandExecutor) GetShell() string {
	return "cmd"
}

func (m *mockWindowsCommandExecutor) GetShellArgs(command string) []string {
	return []string{"/C", command}
}

func (m *mockWindowsCommandExecutor) IsCommandAvailable(command string) bool {
	return !m.shouldFail
}
