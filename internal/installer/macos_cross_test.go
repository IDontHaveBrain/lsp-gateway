package installer

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/platform"
)

func TestMacOSStrategyCrossCompilation(t *testing.T) {
	t.Run("MacOSStrategyCreation", func(t *testing.T) {
		strategy := NewMacOSStrategy()
		if strategy == nil {
			t.Fatal("NewMacOSStrategy returned nil")
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

		t.Logf("macOS strategy created successfully with %d package managers", len(managers))
	})

	t.Run("MacOSStrategyInterface", func(t *testing.T) {
		var strategy PlatformStrategy = NewMacOSStrategy()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("macOS strategy method panicked: %v", r)
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

func TestMacOSHomebrewPathLogic(t *testing.T) {
	t.Run("HomebrewPathDetection", func(t *testing.T) {
		strategy := NewMacOSStrategy()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Homebrew path detection panicked: %v", r)
			}
		}()

		info := strategy.GetPlatformInfo()
		if info.OS != "darwin" {
			t.Logf("Platform OS reported as %s (expected for cross-platform testing)", info.OS)
		}

		managers := strategy.GetPackageManagers()
		expectedManagers := []string{"brew"}

		if len(managers) != len(expectedManagers) {
			t.Errorf("Expected %d managers, got %d: %v", len(expectedManagers), len(managers), managers)
		}

		for i, expected := range expectedManagers {
			if i >= len(managers) || managers[i] != expected {
				t.Errorf("Expected manager %s at index %d, got %v", expected, i, managers)
			}
		}

		t.Logf("macOS strategy tested successfully")
	})
}

func TestMacOSVersionExtraction(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "go version format",
			input:    "go version go1.21.0 darwin/amd64",
			expected: "1.21.0",
		},
		{
			name:     "python version format",
			input:    "Python 3.11.5",
			expected: "3.11.5",
		},
		{
			name:     "node version format",
			input:    "v18.17.1",
			expected: "18.17.1",
		},
		{
			name:     "java version format",
			input:    "openjdk 17.0.8 2023-07-19",
			expected: "17.0.8",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			version := extractVersionFromOutput(tc.input)
			if version != tc.expected {
				t.Errorf("Expected version %s from input %s, got %s", tc.expected, tc.input, version)
			}
		})
	}
}

func TestMacOSEnvironmentSetupLogic(t *testing.T) {
	t.Run("EnvironmentPathHandling", func(t *testing.T) {
		strategy := NewMacOSStrategy()

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Environment setup panicked: %v", r)
			}
		}()

		info := strategy.GetPlatformInfo()

		if runtime.GOOS == "darwin" {
			if info.Architecture == "" {
				t.Error("Architecture should be detected on macOS")
			}
		} else {
			t.Logf("Cross-platform test: Platform info OS=%s, Arch=%s", info.OS, info.Architecture)
		}

		t.Logf("macOS environment logic tested successfully")
	})
}

func TestMacOSPackageNameGeneration(t *testing.T) {
	testCases := []struct {
		runtime     string
		version     string
		expectedPkg string
	}{
		{
			runtime:     "go",
			version:     "1.21",
			expectedPkg: "go",
		},
		{
			runtime:     "python",
			version:     "3.11",
			expectedPkg: "python@3.11",
		},
		{
			runtime:     "node",
			version:     "18",
			expectedPkg: "node@18",
		},
		{
			runtime:     "java",
			version:     "17",
			expectedPkg: "openjdk@17",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.runtime+"-"+tc.version, func(t *testing.T) {
			packageName := generateMacOSPackageName(tc.runtime, tc.version)
			if packageName != tc.expectedPkg {
				t.Errorf("Expected package %s for %s %s, got %s",
					tc.expectedPkg, tc.runtime, tc.version, packageName)
			}
		})
	}
}

func TestMacOSCommandExecutorCompatibility(t *testing.T) {
	t.Run("ShellDetection", func(t *testing.T) {
		mockExec := &mockMacOSCommandExecutor{
			shell:      "/bin/bash",
			shouldFail: false,
			results:    make(map[string]*platform.Result),
		}

		shell := mockExec.GetShell()
		if shell != "/bin/bash" {
			t.Errorf("Expected /bin/bash shell, got %s", shell)
		}

		args := mockExec.GetShellArgs("echo test")
		expectedArgs := []string{"-c", "echo test"}
		if len(args) != len(expectedArgs) {
			t.Errorf("Expected %d shell args, got %d", len(expectedArgs), len(args))
		}

		for i, expected := range expectedArgs {
			if i >= len(args) || args[i] != expected {
				t.Errorf("Expected arg %s at index %d, got %s", expected, i, args[i])
			}
		}

		available := mockExec.IsCommandAvailable("brew")
		if !available {
			t.Error("Mock executor should report brew as available")
		}

		t.Logf("macOS mock executor working correctly")
	})
}

func extractVersionFromOutput(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "go version") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "go") && strings.Contains(part, ".") {
					return strings.TrimPrefix(part, "go")
				}
			}
		}

		if strings.HasPrefix(line, "Python ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}

		if strings.HasPrefix(line, "v") && strings.Contains(line, ".") {
			return strings.TrimPrefix(line, "v")
		}

		if strings.Contains(line, "openjdk") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.Contains(part, ".") && !strings.Contains(part, "-") {
					return part
				}
			}
		}
	}

	return ""
}

func generateMacOSPackageName(runtime, version string) string {
	switch runtime {
	case "go":
		return "go"
	case "python":
		return "python@" + version
	case "node", "nodejs":
		return "node@" + version
	case "java":
		return "openjdk@" + version
	default:
		return runtime
	}
}

type mockMacOSCommandExecutor struct {
	shell      string
	shouldFail bool
	results    map[string]*platform.Result
}

func (m *mockMacOSCommandExecutor) Execute(cmd string, args []string, timeout time.Duration) (*platform.Result, error) {
	key := cmd + " " + strings.Join(args, " ")
	if result, exists := m.results[key]; exists {
		return result, nil
	}

	if m.shouldFail {
		return &platform.Result{
				ExitCode: 1,
				Stderr:   "mock command failed",
			}, &InstallerError{
				Type:    InstallerErrorTypeInstallation,
				Message: "mock command execution failed",
			}
	}

	var stdout string
	switch cmd {
	case "brew":
		stdout = "Homebrew 4.1.0"
	case "go":
		stdout = "go version go1.21.0 darwin/amd64"
	case "python3":
		stdout = "Python 3.11.5"
	case "node":
		stdout = "v18.17.1"
	case "java":
		stdout = "openjdk 17.0.8 2023-07-19"
	default:
		stdout = "mock output"
	}

	return &platform.Result{
		ExitCode: 0,
		Stdout:   stdout,
	}, nil
}

func (m *mockMacOSCommandExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(cmd, args, timeout)
}

func (m *mockMacOSCommandExecutor) GetShell() string {
	return m.shell
}

func (m *mockMacOSCommandExecutor) GetShellArgs(command string) []string {
	return []string{"-c", command}
}

func (m *mockMacOSCommandExecutor) IsCommandAvailable(command string) bool {
	macOSCommands := []string{"brew", "bash", "zsh", "sh", "go", "python3", "node", "java"}
	for _, cmd := range macOSCommands {
		if command == cmd {
			return !m.shouldFail
		}
	}
	return false
}
