package installer

import (
	"errors"
	"testing"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
)

// Mock CommandExecutor for testing
type mockRuntimeCommandExecutor struct {
	commands map[string]*platform.Result
	failNext bool
}

func newMockRuntimeCommandExecutor() *mockRuntimeCommandExecutor {
	return &mockRuntimeCommandExecutor{
		commands: make(map[string]*platform.Result),
	}
}

func (m *mockRuntimeCommandExecutor) Execute(command string, args []string, timeout time.Duration) (*platform.Result, error) {
	if m.failNext {
		m.failNext = false
		return &platform.Result{ExitCode: 1, Stderr: "mock error"}, errors.New("mock error")
	}

	key := command
	if len(args) > 0 {
		key = command + " " + args[0]
	}
	if result, exists := m.commands[key]; exists {
		return result, nil
	}

	return &platform.Result{ExitCode: 0, Stdout: "mock success"}, nil
}

func (m *mockRuntimeCommandExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(cmd, args, timeout)
}

func (m *mockRuntimeCommandExecutor) GetShell() string {
	return "bash"
}

func (m *mockRuntimeCommandExecutor) GetShellArgs(command string) []string {
	return []string{"-c", command}
}

func (m *mockRuntimeCommandExecutor) IsCommandAvailable(command string) bool {
	return true // For simplicity in testing
}

func (m *mockRuntimeCommandExecutor) SetCommand(command string, result *platform.Result) {
	m.commands[command] = result
}

func (m *mockRuntimeCommandExecutor) SetFailNext() {
	m.failNext = true
}

// Test WindowsRuntimeStrategy.InstallRuntime with different runtime types
func TestWindowsRuntimeStrategy_InstallRuntime(t *testing.T) {
	tests := []struct {
		name          string
		runtime       string
		version       string
		expectedError string
	}{
		{
			name:    "Install Go",
			runtime: "go",
			version: "1.21.0",
		},
		{
			name:    "Install Python",
			runtime: "python",
			version: "3.11.0",
		},
		{
			name:    "Install Node.js",
			runtime: "nodejs",
			version: "18.0.0",
		},
		{
			name:    "Install Java",
			runtime: "java",
			version: "17.0.0",
		},
		{
			name:          "Unsupported runtime",
			runtime:       "unsupported",
			version:       "1.0.0",
			expectedError: "unsupported runtime: unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &WindowsRuntimeStrategy{
				strategy: NewWindowsStrategy(),
			}

			options := types.InstallOptions{
				Version: tt.version,
			}

			result, err := strategy.InstallRuntime(tt.runtime, options)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
				}
				return
			}

			// For supported runtimes, result should be returned (success depends on actual system)
			if result == nil {
				t.Error("Expected result, got nil")
				return
			}

			// Verify the result structure
			if result.Success && result.Version != tt.version {
				t.Errorf("Expected version '%s', got '%s'", tt.version, result.Version)
			}

			// Verify that errors array is initialized
			if result.Errors == nil {
				t.Error("Expected Errors slice to be initialized")
			}
		})
	}
}

func TestWindowsRuntimeStrategy_VerifyRuntime(t *testing.T) {
	strategy := &WindowsRuntimeStrategy{
		strategy: NewWindowsStrategy(),
	}

	tests := []struct {
		name    string
		runtime string
	}{
		{"Verify Go", "go"},
		{"Verify Python", "python"},
		{"Verify Node.js", "nodejs"},
		{"Verify Java", "java"},
		{"Verify Unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := strategy.VerifyRuntime(tt.runtime)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("Expected result, got nil")
				return
			}

			// Windows strategy returns a basic result structure
			if result.Installed {
				t.Error("Expected installed=false for basic verification")
			}

			if result.Compatible {
				t.Error("Expected compatible=false for basic verification")
			}

			if result.Issues == nil {
				t.Error("Expected Issues slice to be initialized")
			}

			if result.Details == nil {
				t.Error("Expected Details map to be initialized")
			}
		})
	}
}

func TestWindowsRuntimeStrategy_GetInstallCommand(t *testing.T) {
	strategy := &WindowsRuntimeStrategy{
		strategy: NewWindowsStrategy(),
	}

	tests := []struct {
		name    string
		runtime string
		version string
	}{
		{"Get Go command", "go", "1.21.0"},
		{"Get Python command", "python", "3.11.0"},
		{"Get Node.js command", "nodejs", "18.0.0"},
		{"Get Java command", "java", "17.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands, err := strategy.GetInstallCommand(tt.runtime, tt.version)

			if err == nil {
				t.Error("Expected error 'not implemented', got nil")
				return
			}

			if err.Error() != "not implemented" {
				t.Errorf("Expected error 'not implemented', got '%s'", err.Error())
			}

			if len(commands) != 0 {
				t.Errorf("Expected empty command slice, got %v", commands)
			}
		})
	}
}

func TestLinuxRuntimeStrategy_InstallRuntime(t *testing.T) {
	tests := []struct {
		name          string
		runtime       string
		version       string
		expectedError string
	}{
		{
			name:    "Install Go",
			runtime: "go",
			version: "1.21.0",
		},
		{
			name:    "Install Python",
			runtime: "python",
			version: "3.11.0",
		},
		{
			name:    "Install Node.js",
			runtime: "nodejs",
			version: "18.0.0",
		},
		{
			name:    "Install Java",
			runtime: "java",
			version: "17.0.0",
		},
		{
			name:          "Unsupported runtime",
			runtime:       "unsupported",
			version:       "1.0.0",
			expectedError: "unsupported runtime: unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Linux strategy - this might fail on non-Linux systems
			linuxStrategy, err := NewLinuxStrategy()
			if err != nil {
				t.Skipf("Cannot create Linux strategy on this system: %v", err)
				return
			}

			strategy := &LinuxRuntimeStrategy{
				strategy: linuxStrategy,
			}

			options := types.InstallOptions{
				Version: tt.version,
			}

			result, err := strategy.InstallRuntime(tt.runtime, options)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
				}
				return
			}

			// For supported runtimes, result should be returned
			if result == nil {
				t.Error("Expected result, got nil")
				return
			}

			// Verify the result structure
			if result.Success && result.Version != tt.version {
				t.Errorf("Expected version '%s', got '%s'", tt.version, result.Version)
			}

			// Verify that errors array is initialized
			if result.Errors == nil {
				t.Error("Expected Errors slice to be initialized")
			}
		})
	}
}

func TestLinuxRuntimeStrategy_VerifyRuntime(t *testing.T) {
	// Create a Linux strategy - this might fail on non-Linux systems
	linuxStrategy, err := NewLinuxStrategy()
	if err != nil {
		t.Skipf("Cannot create Linux strategy on this system: %v", err)
		return
	}

	strategy := &LinuxRuntimeStrategy{
		strategy: linuxStrategy,
	}

	tests := []struct {
		name    string
		runtime string
	}{
		{"Verify Go", "go"},
		{"Verify Python", "python"},
		{"Verify Node.js", "nodejs"},
		{"Verify Java", "java"},
		{"Verify Unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := strategy.VerifyRuntime(tt.runtime)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("Expected result, got nil")
				return
			}

			// Linux strategy returns a basic result structure
			if result.Installed {
				t.Error("Expected installed=false for basic verification")
			}

			if result.Compatible {
				t.Error("Expected compatible=false for basic verification")
			}

			if result.Issues == nil {
				t.Error("Expected Issues slice to be initialized")
			}

			if result.Details == nil {
				t.Error("Expected Details map to be initialized")
			}
		})
	}
}

func TestLinuxRuntimeStrategy_GetInstallCommand(t *testing.T) {
	// Create a Linux strategy - this might fail on non-Linux systems
	linuxStrategy, err := NewLinuxStrategy()
	if err != nil {
		t.Skipf("Cannot create Linux strategy on this system: %v", err)
		return
	}

	strategy := &LinuxRuntimeStrategy{
		strategy: linuxStrategy,
	}

	tests := []struct {
		name    string
		runtime string
		version string
	}{
		{"Get Go command", "go", "1.21.0"},
		{"Get Python command", "python", "3.11.0"},
		{"Get Node.js command", "nodejs", "18.0.0"},
		{"Get Java command", "java", "17.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands, err := strategy.GetInstallCommand(tt.runtime, tt.version)

			if err == nil {
				t.Error("Expected error 'not implemented', got nil")
				return
			}

			if err.Error() != "not implemented" {
				t.Errorf("Expected error 'not implemented', got '%s'", err.Error())
			}

			if len(commands) != 0 {
				t.Errorf("Expected empty command slice, got %v", commands)
			}
		})
	}
}

func TestMacOSRuntimeStrategy_InstallRuntime(t *testing.T) {
	tests := []struct {
		name          string
		runtime       string
		version       string
		expectedError string
	}{
		{
			name:    "Install Go",
			runtime: "go",
			version: "1.21.0",
		},
		{
			name:    "Install Python",
			runtime: "python",
			version: "3.11.0",
		},
		{
			name:    "Install Node.js",
			runtime: "nodejs",
			version: "18.0.0",
		},
		{
			name:    "Install Java",
			runtime: "java",
			version: "17.0.0",
		},
		{
			name:          "Unsupported runtime",
			runtime:       "unsupported",
			version:       "1.0.0",
			expectedError: "unsupported runtime: unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &MacOSRuntimeStrategy{
				strategy: NewMacOSStrategy(),
			}

			options := types.InstallOptions{
				Version: tt.version,
			}

			result, err := strategy.InstallRuntime(tt.runtime, options)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.expectedError)
				} else if err.Error() != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, err.Error())
				}
				return
			}

			// For supported runtimes, result should be returned
			if result == nil {
				t.Error("Expected result, got nil")
				return
			}

			// Verify the result structure
			if result.Success && result.Version != tt.version {
				t.Errorf("Expected version '%s', got '%s'", tt.version, result.Version)
			}

			// Verify that errors array is initialized
			if result.Errors == nil {
				t.Error("Expected Errors slice to be initialized")
			}
		})
	}
}

func TestMacOSRuntimeStrategy_VerifyRuntime(t *testing.T) {
	strategy := &MacOSRuntimeStrategy{
		strategy: NewMacOSStrategy(),
	}

	tests := []struct {
		name    string
		runtime string
	}{
		{"Verify Go", "go"},
		{"Verify Python", "python"},
		{"Verify Node.js", "nodejs"},
		{"Verify Java", "java"},
		{"Verify Unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := strategy.VerifyRuntime(tt.runtime)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("Expected result, got nil")
				return
			}

			// macOS strategy returns a basic result structure
			if result.Installed {
				t.Error("Expected installed=false for basic verification")
			}

			if result.Compatible {
				t.Error("Expected compatible=false for basic verification")
			}

			if result.Issues == nil {
				t.Error("Expected Issues slice to be initialized")
			}

			if result.Details == nil {
				t.Error("Expected Details map to be initialized")
			}
		})
	}
}

func TestMacOSRuntimeStrategy_GetInstallCommand(t *testing.T) {
	strategy := &MacOSRuntimeStrategy{
		strategy: NewMacOSStrategy(),
	}

	tests := []struct {
		name    string
		runtime string
		version string
	}{
		{"Get Go command", "go", "1.21.0"},
		{"Get Python command", "python", "3.11.0"},
		{"Get Node.js command", "nodejs", "18.0.0"},
		{"Get Java command", "java", "17.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commands, err := strategy.GetInstallCommand(tt.runtime, tt.version)

			if err == nil {
				t.Error("Expected error 'not implemented', got nil")
				return
			}

			if err.Error() != "not implemented" {
				t.Errorf("Expected error 'not implemented', got '%s'", err.Error())
			}

			if len(commands) != 0 {
				t.Errorf("Expected empty command slice, got %v", commands)
			}
		})
	}
}

func TestRuntimeStrategies_EdgeCases(t *testing.T) {
	t.Run("WindowsRuntimeStrategy with nil strategy", func(t *testing.T) {
		strategy := &WindowsRuntimeStrategy{strategy: nil}

		// Test should handle nil strategy gracefully
		_, err := strategy.InstallRuntime("go", types.InstallOptions{})
		if err == nil {
			t.Error("Expected error with nil strategy")
		}
	})

	t.Run("LinuxRuntimeStrategy with nil strategy", func(t *testing.T) {
		strategy := &LinuxRuntimeStrategy{strategy: nil}

		// Test should handle nil strategy gracefully
		_, err := strategy.InstallRuntime("go", types.InstallOptions{})
		if err == nil {
			t.Error("Expected error with nil strategy")
		}
	})

	t.Run("MacOSRuntimeStrategy with nil strategy", func(t *testing.T) {
		strategy := &MacOSRuntimeStrategy{strategy: nil}

		// Test should handle nil strategy gracefully
		_, err := strategy.InstallRuntime("go", types.InstallOptions{})
		if err == nil {
			t.Error("Expected error with nil strategy")
		}
	})
}

func TestRuntimeStrategies_EmptyVersion(t *testing.T) {
	tests := []struct {
		name     string
		strategy types.RuntimePlatformStrategy
	}{
		{
			"Windows with empty version",
			&WindowsRuntimeStrategy{strategy: NewWindowsStrategy()},
		},
		{
			"macOS with empty version",
			&MacOSRuntimeStrategy{strategy: NewMacOSStrategy()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := types.InstallOptions{
				Version: "", // Empty version
			}

			_, err := tt.strategy.InstallRuntime("go", options)
			// Should handle empty version gracefully (success or specific error)
			if err != nil {
				// Verify it's a reasonable error message
				if err.Error() == "" {
					t.Error("Error message should not be empty")
				}
			}
		})
	}

	// Test Linux separately since it might not be available on all systems
	t.Run("Linux with empty version", func(t *testing.T) {
		linuxStrategy, err := NewLinuxStrategy()
		if err != nil {
			t.Skipf("Cannot create Linux strategy on this system: %v", err)
			return
		}

		strategy := &LinuxRuntimeStrategy{strategy: linuxStrategy}
		options := types.InstallOptions{
			Version: "", // Empty version
		}

		_, err = strategy.InstallRuntime("go", options)
		// Should handle empty version gracefully (success or specific error)
		if err != nil {
			// Verify it's a reasonable error message
			if err.Error() == "" {
				t.Error("Error message should not be empty")
			}
		}
	})
}

// Test strategy interface compliance
func TestRuntimeStrategyInterfaces(t *testing.T) {
	var _ types.RuntimePlatformStrategy = &WindowsRuntimeStrategy{}
	var _ types.RuntimePlatformStrategy = &LinuxRuntimeStrategy{}
	var _ types.RuntimePlatformStrategy = &MacOSRuntimeStrategy{}
}

// Benchmark tests to ensure performance
func BenchmarkWindowsRuntimeStrategy(b *testing.B) {
	strategy := &WindowsRuntimeStrategy{strategy: NewWindowsStrategy()}
	options := types.InstallOptions{Version: "1.21.0"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = strategy.InstallRuntime("go", options)
	}
}

func BenchmarkMacOSRuntimeStrategy(b *testing.B) {
	strategy := &MacOSRuntimeStrategy{strategy: NewMacOSStrategy()}
	options := types.InstallOptions{Version: "1.21.0"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = strategy.InstallRuntime("go", options)
	}
}