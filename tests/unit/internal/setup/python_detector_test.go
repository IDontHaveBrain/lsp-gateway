package setup_test

import (
	"context"
	"lsp-gateway/internal/setup"
	"testing"
)

func TestPythonDetector_NewPythonDetector(t *testing.T) {
	detector := setup.NewPythonDetector()

	if detector == nil {
		t.Fatal("Expected non-nil detector")
	}

	if detector.executor == nil {
		t.Error("Expected non-nil executor")
	}

	if detector.versionChecker == nil {
		t.Error("Expected non-nil version checker")
	}

	if detector.logger == nil {
		t.Error("Expected non-nil logger")
	}
}

func TestPythonDetector_DetectPython(t *testing.T) {
	detector := setup.NewPythonDetector()

	info, err := detector.DetectPython()

	if err != nil {
		t.Logf("Python detection returned error: %v", err)
	}

	if info == nil {
		t.Fatal("Expected non-nil runtime info")
	}

	if info.Name != "python" {
		t.Errorf("Expected name 'python', got %s", info.Name)
	}

	if info.Issues == nil {
		t.Error("Expected non-nil issues slice")
	}

	if info.Warnings == nil {
		info.Warnings = []string{} // Initialize if nil for testing
	}

	t.Logf("Python detection results:")
	t.Logf("  Installed: %v", info.Installed)
	t.Logf("  Version: %s", info.Version)
	t.Logf("  Compatible: %v", info.Compatible)
	t.Logf("  Path: %s", info.Path)
	t.Logf("  Issues count: %d", len(info.Issues))
	t.Logf("  Warnings count: %d", len(info.Warnings))

	if len(info.Issues) > 0 {
		t.Logf("  Issues: %v", info.Issues)
	}

	if len(info.Warnings) > 0 {
		t.Logf("  Warnings: %v", info.Warnings)
	}
}

func TestDefaultRuntimeDetector_DetectPython(t *testing.T) {
	detector := setup.NewRuntimeDetector()
	ctx := context.Background()

	info, err := detector.DetectPython(ctx)

	if err != nil {
		t.Logf("Runtime detector Python detection returned error: %v", err)
	}

	if info == nil {
		t.Fatal("Expected non-nil runtime info from runtime detector")
	}

	if info.Name != "python" {
		t.Errorf("Expected name 'python', got %s", info.Name)
	}

	if info.DetectedAt.IsZero() {
		t.Error("Expected non-zero detection timestamp")
	}

	if info.Duration == 0 {
		t.Error("Expected non-zero detection duration")
	}

	t.Logf("Runtime detector Python detection results:")
	t.Logf("  Installed: %v", info.Installed)
	t.Logf("  Version: %s", info.Version)
	t.Logf("  Compatible: %v", info.Compatible)
	t.Logf("  Path: %s", info.Path)
	t.Logf("  Duration: %v", info.Duration)
	t.Logf("  Issues count: %d", len(info.Issues))
}

func TestPythonDetector_ParseVersionString(t *testing.T) {
	detector := setup.NewPythonDetector()

	testCases := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"Python 3.11.2", "3.11.2", false},
		{"Python 3.8.10", "3.8.10", false},
		{"Python 2.7.18", "2.7.18", false},
		{"3.9.7", "3.9.7", false},
		{"invalid version", "", true},
		{"", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := detector.parseVersionString(tc.input)

			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tc.input, err)
				}

				if result != tc.expected {
					t.Errorf("Expected '%s', got '%s' for input '%s'", tc.expected, result, tc.input)
				}
			}
		})
	}
}

func TestPythonDetector_ParsePipVersion(t *testing.T) {
	detector := setup.NewPythonDetector()

	testCases := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"pip 23.0.1 from /usr/lib/python3/site-packages/pip (python 3.11)", "23.0.1", false},
		{"pip 21.3.1", "21.3.1", false},
		{"pip 20.0", "20.0", false},
		{"invalid pip output", "", true},
		{"", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := detector.parsePipVersion(tc.input)

			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tc.input, err)
				}

				if result != tc.expected {
					t.Errorf("Expected '%s', got '%s' for input '%s'", tc.expected, result, tc.input)
				}
			}
		})
	}
}

func TestPythonDetector_GetPythonCommands(t *testing.T) {
	detector := setup.NewPythonDetector()

	commands := detector.getPythonCommands()

	if len(commands) == 0 {
		t.Error("Expected at least one Python command")
	}

	hasCommonCommand := false
	for _, cmd := range commands {
		if cmd == "python" || cmd == "python3" || cmd == "py" {
			hasCommonCommand = true
			break
		}
	}

	if !hasCommonCommand {
		t.Error("Expected at least one common Python command (python, python3, py)")
	}

	t.Logf("Python commands to try: %v", commands)
}
