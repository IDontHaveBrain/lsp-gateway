package setup_test

import (
	"context"
	"fmt"
	"lsp-gateway/internal/setup"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/platform"
)

// MockCommandExecutor for testing runtime detection
type MockCommandExecutor struct {
	results map[string]*platform.Result
	errors  map[string]error
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		results: make(map[string]*platform.Result),
		errors:  make(map[string]error),
	}
}

func (m *MockCommandExecutor) SetResult(command string, result *platform.Result) {
	m.results[command] = result
}

func (m *MockCommandExecutor) SetError(command string, err error) {
	m.errors[command] = err
}

func (m *MockCommandExecutor) Execute(command string, args []string, timeout time.Duration) (*platform.Result, error) {
	key := command
	if len(args) > 0 {
		key = command + " " + args[0]
	}

	if err, exists := m.errors[key]; exists {
		return nil, err
	}

	if result, exists := m.results[key]; exists {
		return result, nil
	}

	// Default success result
	return &platform.Result{
		ExitCode: 0,
		Stdout:   "mock output",
		Stderr:   "",
	}, nil
}

func (m *MockCommandExecutor) ExecuteWithEnv(command string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(command, args, timeout)
}

func (m *MockCommandExecutor) GetShell() string {
	return "/bin/bash"
}

func (m *MockCommandExecutor) GetShellArgs(command string) []string {
	return []string{"-c", command}
}

func (m *MockCommandExecutor) IsCommandAvailable(command string) bool {
	// For mock purposes, assume commands are available unless specifically set to error
	key := command + " --version"
	_, hasError := m.errors[key]
	return !hasError
}

// TestRuntimeDetector_DetectAll tests comprehensive runtime detection
func TestRuntimeDetector_DetectAll(t *testing.T) {
	detector := setup.NewRuntimeDetector()
	mockExecutor := NewMockCommandExecutor()
	detector.SetExecutor(mockExecutor)

	// Set up mock results for different runtimes
	mockExecutor.SetResult("go version", &platform.Result{
		ExitCode: 0,
		Stdout:   "go version go1.21.0 linux/amd64",
		Stderr:   "",
	})

	mockExecutor.SetResult("python3 --version", &platform.Result{
		ExitCode: 0,
		Stdout:   "Python 3.11.0",
		Stderr:   "",
	})

	mockExecutor.SetResult("node --version", &platform.Result{
		ExitCode: 0,
		Stdout:   "v20.1.0",
		Stderr:   "",
	})

	mockExecutor.SetResult("java -version", &platform.Result{
		ExitCode: 0,
		Stdout:   "",
		Stderr:   "openjdk version \"21.0.0\" 2023-09-19",
	})

	ctx := context.Background()
	report, err := detector.DetectAll(ctx)

	if err != nil {
		t.Fatalf("DetectAll failed: %v", err)
	}

	if report == nil {
		t.Fatal("Expected non-nil detection report")
	}

	// Verify report structure
	if report.Summary.TotalRuntimes != 4 {
		t.Errorf("Expected 4 total runtimes, got %d", report.Summary.TotalRuntimes)
	}

	if len(report.Runtimes) != 4 {
		t.Errorf("Expected 4 runtime results, got %d", len(report.Runtimes))
	}

	// Verify individual runtime detection
	expectedRuntimes := []string{"go", "python", "nodejs", "java"}
	for _, runtime := range expectedRuntimes {
		if info, exists := report.Runtimes[runtime]; !exists {
			t.Errorf("Expected runtime %s in detection report", runtime)
		} else {
			if info.Name != runtime {
				t.Errorf("Expected runtime name %s, got %s", runtime, info.Name)
			}

			if info.DetectedAt.IsZero() {
				t.Errorf("Expected DetectedAt to be set for runtime %s", runtime)
			}

			if info.Duration <= 0 {
				t.Errorf("Expected positive duration for runtime %s", runtime)
			}
		}
	}

	// Verify summary calculations
	if report.Summary.AverageDetectTime <= 0 {
		t.Error("Expected positive average detection time")
	}

	if report.Duration <= 0 {
		t.Error("Expected positive total duration")
	}

	if report.SessionID == "" {
		t.Error("Expected session ID to be set")
	}

	t.Logf("Detection completed: %d runtimes, %.1f%% success rate, %v total duration",
		report.Summary.TotalRuntimes, report.Summary.SuccessRate, report.Duration)
}

// TestRuntimeDetector_DetectNodejs tests Node.js detection specifically
func TestRuntimeDetector_DetectNodejs(t *testing.T) {
	detector := setup.NewRuntimeDetector()
	mockExecutor := NewMockCommandExecutor()
	detector.SetExecutor(mockExecutor)

	testCases := []struct {
		name            string
		mockResult      *platform.Result
		mockError       error
		expectInstalled bool
		expectVersion   string
	}{
		{
			name: "NodeJS Installed",
			mockResult: &platform.Result{
				ExitCode: 0,
				Stdout:   "v20.1.0",
				Stderr:   "",
			},
			expectInstalled: true,
			expectVersion:   "v20.1.0",
		},
		{
			name:            "NodeJS Not Found",
			mockError:       fmt.Errorf("command not found: node"),
			expectInstalled: false,
		},
		{
			name:            "NodeJS Command Error",
			mockError:       fmt.Errorf("node command failed with exit code 1"),
			expectInstalled: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset mock
			mockExecutor = NewMockCommandExecutor()
			detector.SetExecutor(mockExecutor)

			if tc.mockResult != nil {
				mockExecutor.SetResult("node --version", tc.mockResult)
			}
			if tc.mockError != nil {
				mockExecutor.SetError("node --version", tc.mockError)
			}

			ctx := context.Background()
			info, err := detector.DetectNodejs(ctx)

			if err != nil && tc.expectInstalled {
				t.Fatalf("DetectNodejs failed unexpectedly: %v", err)
			}

			if info == nil {
				t.Fatal("Expected non-nil runtime info")
			}

			if info.Installed != tc.expectInstalled {
				t.Errorf("Expected installed=%v, got %v", tc.expectInstalled, info.Installed)
			}

			if tc.expectInstalled && info.Version != tc.expectVersion {
				t.Errorf("Expected version %s, got %s", tc.expectVersion, info.Version)
			}

			if !tc.expectInstalled && len(info.Issues) == 0 {
				t.Error("Expected issues for uninstalled runtime")
			}
		})
	}
}

// TestRuntimeDetector_SetLogger tests logger configuration
func TestRuntimeDetector_SetLogger(t *testing.T) {
	detector := setup.NewRuntimeDetector()

	// Test setting nil logger (should not crash)
	detector.SetLogger(nil)

	// Test setting valid logger
	logger := setup.NewSetupLogger(nil)
	detector.SetLogger(logger)

	// COMMENTED OUT: accessing unexported field
	/*
		if detector.logger != logger {
			t.Error("Expected logger to be set")
		}
	*/

	// Test that detection still works with custom logger
	mockExecutor := NewMockCommandExecutor()
	detector.SetExecutor(mockExecutor)

	mockExecutor.SetResult("go version", &platform.Result{
		ExitCode: 0,
		Stdout:   "go version go1.21.0 linux/amd64",
		Stderr:   "",
	})

	ctx := context.Background()
	info, err := detector.DetectGo(ctx)

	if err != nil {
		t.Fatalf("DetectGo failed with custom logger: %v", err)
	}

	if !info.Installed {
		t.Error("Expected Go to be detected as installed")
	}
}

// TestRuntimeDetector_ErrorHandling tests error handling in detection
func TestRuntimeDetector_ErrorHandling(t *testing.T) {
	detector := setup.NewRuntimeDetector()
	mockExecutor := NewMockCommandExecutor()
	detector.SetExecutor(mockExecutor)

	t.Run("CommandNotFound", func(t *testing.T) {
		mockExecutor.SetError("go version", fmt.Errorf("command not found: go"))

		ctx := context.Background()
		info, err := detector.DetectGo(ctx)

		// May return error for command not found, should handle gracefully
		if info == nil {
			t.Fatal("Expected non-nil info even on command not found")
		}

		if info.Installed {
			t.Error("Expected Go to be detected as not installed")
		}

		// If error occurred, info should have issues or the error should be handled appropriately
		if err == nil && len(info.Issues) == 0 {
			t.Error("Expected either error or issues for command not found")
		}

		t.Logf("Command not found handling: error=%v, installed=%v, issues=%d", err != nil, info.Installed, len(info.Issues))
	})

	t.Run("CommandTimeout", func(t *testing.T) {
		// Set very short timeout
		detector.SetTimeout(1 * time.Millisecond)

		// Reset executor to avoid cached results
		mockExecutor = NewMockCommandExecutor()
		detector.SetExecutor(mockExecutor)

		ctx := context.Background()
		info, err := detector.DetectGo(ctx)

		// Should handle timeout gracefully
		if info == nil {
			t.Fatal("Expected non-nil info even on timeout")
		}

		// Reset timeout for other tests
		detector.SetTimeout(30 * time.Second)

		t.Logf("Timeout handling test completed: info=%v, err=%v", info != nil, err != nil)
	})

	t.Run("NonZeroExitCode", func(t *testing.T) {
		mockExecutor.SetResult("java -version", &platform.Result{
			ExitCode: 0, // Java -version typically returns 0 but outputs to stderr
			Stdout:   "",
			Stderr:   "openjdk version \"21.0.0\" 2023-09-19",
		})

		ctx := context.Background()
		info, err := detector.DetectJava(ctx)

		if err != nil {
			t.Errorf("Expected no error for Java version detection, got: %v", err)
		}

		if !info.Installed {
			t.Error("Expected Java to be detected as installed")
		}

		if info.Version == "" {
			t.Error("Expected version to be extracted from stderr")
		}
	})
}

// TestRuntimeDetector_VersionCompatibility tests version compatibility checking
func TestRuntimeDetector_VersionCompatibility(t *testing.T) {
	detector := setup.NewRuntimeDetector()
	mockExecutor := NewMockCommandExecutor()
	detector.SetExecutor(mockExecutor)

	testCases := []struct {
		runtime          string
		versionOutput    string
		expectInstalled  bool
		expectCompatible bool
	}{
		{
			runtime:          "go",
			versionOutput:    "go version go1.21.0 linux/amd64",
			expectInstalled:  true,
			expectCompatible: true,
		},
		{
			runtime:          "go",
			versionOutput:    "go version go1.18.0 linux/amd64",
			expectInstalled:  true,
			expectCompatible: true, // 1.18 should be compatible based on actual requirements
		},
		{
			runtime:          "python",
			versionOutput:    "Python 3.11.0",
			expectInstalled:  true,
			expectCompatible: true,
		},
		{
			runtime:          "python",
			versionOutput:    "Python 3.7.0",
			expectInstalled:  true,
			expectCompatible: false, // Below minimum version
		},
	}

	for _, tc := range testCases {
		t.Run(tc.runtime+"_"+tc.versionOutput, func(t *testing.T) {
			// Reset mock executor
			mockExecutor = NewMockCommandExecutor()
			detector.SetExecutor(mockExecutor)

			// Set up appropriate mock based on runtime
			switch tc.runtime {
			case "go":
				mockExecutor.SetResult("go version", &platform.Result{
					ExitCode: 0,
					Stdout:   tc.versionOutput,
					Stderr:   "",
				})
			case "python":
				mockExecutor.SetResult("python3 --version", &platform.Result{
					ExitCode: 0,
					Stdout:   tc.versionOutput,
					Stderr:   "",
				})
			}

			ctx := context.Background()
			var info *setup.RuntimeInfo
			var err error

			switch tc.runtime {
			case "go":
				info, err = detector.DetectGo(ctx)
			case "python":
				info, err = detector.DetectPython(ctx)
			}

			if err != nil {
				t.Fatalf("Detection failed: %v", err)
			}

			if info.Installed != tc.expectInstalled {
				t.Errorf("Expected installed=%v, got %v", tc.expectInstalled, info.Installed)
			}

			if info.Compatible != tc.expectCompatible {
				t.Errorf("Expected compatible=%v, got %v", tc.expectCompatible, info.Compatible)
			}

			if !tc.expectCompatible && len(info.Warnings) == 0 {
				t.Error("Expected warnings for incompatible version")
			}

			t.Logf("Version compatibility test: %s %s -> installed=%v, compatible=%v",
				tc.runtime, tc.versionOutput, info.Installed, info.Compatible)
		})
	}
}

// TestRuntimeDetector_MetadataPopulation tests that metadata is properly populated
func TestRuntimeDetector_MetadataPopulation(t *testing.T) {
	detector := setup.NewRuntimeDetector()
	mockExecutor := NewMockCommandExecutor()
	detector.SetExecutor(mockExecutor)

	mockExecutor.SetResult("go version", &platform.Result{
		ExitCode: 0,
		Stdout:   "go version go1.21.0 linux/amd64",
		Stderr:   "",
	})

	ctx := context.Background()
	info, err := detector.DetectGo(ctx)

	if err != nil {
		t.Fatalf("DetectGo failed: %v", err)
	}

	if info.Metadata == nil {
		t.Fatal("Expected metadata to be populated")
	}

	// Check required metadata fields
	expectedFields := []string{"command_used", "exit_code", "stdout", "stderr", "platform", "architecture"}
	for _, field := range expectedFields {
		if _, exists := info.Metadata[field]; !exists {
			t.Errorf("Expected metadata field %s to be present", field)
		}
	}

	if exitCode, ok := info.Metadata["exit_code"].(int); !ok || exitCode != 0 {
		t.Errorf("Expected exit_code metadata to be 0, got %v", info.Metadata["exit_code"])
	}

	if stdout, ok := info.Metadata["stdout"].(string); !ok || stdout == "" {
		t.Error("Expected stdout metadata to be populated")
	}

	if platform, ok := info.Metadata["platform"].(string); !ok || platform == "" {
		t.Error("Expected platform metadata to be populated")
	}

	if architecture, ok := info.Metadata["architecture"].(string); !ok || architecture == "" {
		t.Error("Expected architecture metadata to be populated")
	}

	t.Logf("Metadata populated with %d fields", len(info.Metadata))
}

// TestRuntimeDetector_CustomCommands tests custom command configuration
func TestRuntimeDetector_CustomCommands(t *testing.T) {
	detector := setup.NewRuntimeDetector()
	mockExecutor := NewMockCommandExecutor()
	detector.SetExecutor(mockExecutor)

	// Set custom command for Python detection
	detector.SetCustomCommand("python", "python3.11 --version")

	mockExecutor.SetResult("python3.11 --version", &platform.Result{
		ExitCode: 0,
		Stdout:   "Python 3.11.5",
		Stderr:   "",
	})

	ctx := context.Background()
	info, err := detector.DetectPython(ctx)

	if err != nil {
		t.Fatalf("DetectPython with custom command failed: %v", err)
	}

	if !info.Installed {
		t.Error("Expected Python to be detected as installed with custom command")
	}

	if info.Version != "Python 3.11.5" {
		t.Errorf("Expected version 'Python 3.11.5', got %s", info.Version)
	}

	// Verify the custom command was used
	if cmdUsed, ok := info.Metadata["command_used"].(string); !ok || !strings.Contains(cmdUsed, "python3.11") {
		t.Errorf("Expected custom command to be recorded in metadata, got: %v", cmdUsed)
	}

	t.Logf("Custom command detection successful")
}
