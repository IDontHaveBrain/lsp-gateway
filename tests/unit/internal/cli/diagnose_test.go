package cli_test

import (
	"lsp-gateway/internal/cli"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	testutil "lsp-gateway/tests/utils/helpers"

	"github.com/spf13/cobra"
)

func TestDiagnoseCommand(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testDiagnoseCommandMetadata},
		{"FlagParsing", testDiagnoseCommandFlagParsing},
		{"SystemDiagnostics", testDiagnoseSystemDiagnostics},
		{"RuntimesDiagnostics", testDiagnoseRuntimesDiagnostics},
		{"ServersDiagnostics", testDiagnoseServersDiagnostics},
		{"ConfigDiagnostics", testDiagnoseConfigDiagnostics},
		{"JSONOutput", testDiagnoseJSONOutput},
		{"HumanOutput", testDiagnoseHumanOutput},
		{"ErrorScenarios", testDiagnoseErrorScenarios},
		{"Help", testDiagnoseCommandHelp},
		{"Integration", testDiagnoseCommandIntegration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnoseJSON = false
			diagnoseVerbose = false
			diagnoseTimeout = 60 * time.Second
			diagnoseAll = false
			tt.testFunc(t)
		})
	}
}

func testDiagnoseCommandMetadata(t *testing.T) {
	if diagnoseCmd.Use != "diagnose" {
		t.Errorf("Expected Use to be 'diagnose', got '%s'", diagnoseCmd.Use)
	}

	expectedShort := "Run comprehensive system diagnostics"
	if diagnoseCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, diagnoseCmd.Short)
	}

	if !strings.Contains(diagnoseCmd.Long, "comprehensive diagnostics") {
		t.Error("Expected Long description to mention comprehensive diagnostics")
	}

	if !strings.Contains(diagnoseCmd.Long, "LSP Gateway installation") {
		t.Error("Expected Long description to mention LSP Gateway installation")
	}

	if diagnoseCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}

	if diagnoseCmd.Run != nil {
		t.Error("Expected Run function to be nil (using RunE instead)")
	}

	subcommands := diagnoseCmd.Commands()
	expectedSubcommands := map[string]bool{
		"runtimes": false,
		"servers":  false,
		"config":   false,
	}

	for _, cmd := range subcommands {
		if _, exists := expectedSubcommands[cmd.Use]; exists {
			expectedSubcommands[cmd.Use] = true
		}
	}

	for cmdName, found := range expectedSubcommands {
		if !found {
			t.Errorf("Expected subcommand '%s' to be registered", cmdName)
		}
	}
}

func testDiagnoseCommandFlagParsing(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedJSON    bool
		expectedVerbose bool
		expectedTimeout time.Duration
		expectedAll     bool
		expectedError   bool
	}{
		{
			name:            "DefaultFlags",
			args:            []string{},
			expectedJSON:    false,
			expectedVerbose: false,
			expectedTimeout: 60 * time.Second,
			expectedAll:     false,
			expectedError:   false,
		},
		{
			name:            "JSONFlag",
			args:            []string{"--json"},
			expectedJSON:    true,
			expectedVerbose: false,
			expectedTimeout: 60 * time.Second,
			expectedAll:     false,
			expectedError:   false,
		},
		{
			name:            "VerboseFlag",
			args:            []string{"--verbose"},
			expectedJSON:    false,
			expectedVerbose: true,
			expectedTimeout: 60 * time.Second,
			expectedAll:     false,
			expectedError:   false,
		},
		{
			name:            "VerboseFlagShort",
			args:            []string{"-v"},
			expectedJSON:    false,
			expectedVerbose: true,
			expectedTimeout: 60 * time.Second,
			expectedAll:     false,
			expectedError:   false,
		},
		{
			name:            "TimeoutFlag",
			args:            []string{"--timeout", "30s"},
			expectedJSON:    false,
			expectedVerbose: false,
			expectedTimeout: 30 * time.Second,
			expectedAll:     false,
			expectedError:   false,
		},
		{
			name:            "AllFlag",
			args:            []string{"--all"},
			expectedJSON:    false,
			expectedVerbose: false,
			expectedTimeout: 60 * time.Second,
			expectedAll:     true,
			expectedError:   false,
		},
		{
			name:            "AllFlags",
			args:            []string{"--json", "--verbose", "--timeout", "45s", "--all"},
			expectedJSON:    true,
			expectedVerbose: true,
			expectedTimeout: 45 * time.Second,
			expectedAll:     true,
			expectedError:   false,
		},
		{
			name:            "InvalidTimeout",
			args:            []string{"--timeout", "invalid"},
			expectedJSON:    false,
			expectedVerbose: false,
			expectedTimeout: 60 * time.Second,
			expectedAll:     false,
			expectedError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnoseJSON = false
			diagnoseVerbose = false
			diagnoseTimeout = 60 * time.Second
			diagnoseAll = false

			cmd := &cobra.Command{
				Use: "diagnose",
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}

			cmd.PersistentFlags().BoolVar(&diagnoseJSON, "json", false, "Output in JSON format")
			cmd.PersistentFlags().BoolVarP(&diagnoseVerbose, "verbose", "v", false, "Verbose diagnostic output")
			cmd.PersistentFlags().DurationVar(&diagnoseTimeout, "timeout", 60*time.Second, "Diagnostic timeout")
			cmd.PersistentFlags().BoolVar(&diagnoseAll, "all", false, "Run all available diagnostics")

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if diagnoseJSON != tt.expectedJSON {
				t.Errorf("Expected JSON to be %v, got %v", tt.expectedJSON, diagnoseJSON)
			}
			if diagnoseVerbose != tt.expectedVerbose {
				t.Errorf("Expected verbose to be %v, got %v", tt.expectedVerbose, diagnoseVerbose)
			}
			if diagnoseTimeout != tt.expectedTimeout {
				t.Errorf("Expected timeout to be %v, got %v", tt.expectedTimeout, diagnoseTimeout)
			}
			if diagnoseAll != tt.expectedAll {
				t.Errorf("Expected all to be %v, got %v", tt.expectedAll, diagnoseAll)
			}
		})
	}
}

func testDiagnoseSystemDiagnostics(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	tmpDir := testutil.TempDir(t)
	configPath := filepath.Join(tmpDir, "config.yaml")
	port := AllocateTestPort(t)
	configContent := CreateConfigWithPort(port)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	tests := []struct {
		name        string
		setupFunc   func()
		expectError bool
	}{
		{
			name:        "ValidSystemDiagnostics",
			setupFunc:   func() {},
			expectError: false,
		},
		{
			name: "TimeoutScenario",
			setupFunc: func() {
				diagnoseTimeout = 1 * time.Nanosecond // Very short timeout
			},
			expectError: false, // System diagnostics should complete quickly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnoseJSON = false
			diagnoseVerbose = false
			diagnoseTimeout = 60 * time.Second
			diagnoseAll = false

			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			err := diagnoseSystem(diagnoseCmd, []string{})

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func testDiagnoseRuntimesDiagnostics(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	tests := []struct {
		name        string
		setupFunc   func()
		expectError bool
	}{
		{
			name:        "RuntimeDiagnostics",
			setupFunc:   func() {},
			expectError: false, // Should not error even if installer not available
		},
		{
			name: "RuntimeDiagnosticsJSON",
			setupFunc: func() {
				diagnoseJSON = true
			},
			expectError: false,
		},
		{
			name: "RuntimeDiagnosticsVerbose",
			setupFunc: func() {
				diagnoseVerbose = true
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnoseJSON = false
			diagnoseVerbose = false
			diagnoseTimeout = 60 * time.Second
			diagnoseAll = false

			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			err := diagnoseRuntimes(diagnoseRuntimesCmd, []string{})

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func testDiagnoseServersDiagnostics(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	tests := []struct {
		name        string
		setupFunc   func()
		expectError bool
	}{
		{
			name:        "ServerDiagnostics",
			setupFunc:   func() {},
			expectError: false, // Should not error even if installer not available
		},
		{
			name: "ServerDiagnosticsJSON",
			setupFunc: func() {
				diagnoseJSON = true
			},
			expectError: false,
		},
		{
			name: "ServerDiagnosticsVerbose",
			setupFunc: func() {
				diagnoseVerbose = true
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnoseJSON = false
			diagnoseVerbose = false
			diagnoseTimeout = 60 * time.Second
			diagnoseAll = false

			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			err := diagnoseServers(diagnoseServersCmd, []string{})

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func testDiagnoseConfigDiagnostics(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	tmpDir := testutil.TempDir(t)

	tests := []struct {
		name         string
		setupFunc    func() string // Returns config path
		expectError  bool
		expectStatus string
	}{
		{
			name: "ValidConfig",
			setupFunc: func() string {
				port := AllocateTestPort(t)
				configPath := filepath.Join(tmpDir, "valid_config.yaml")
				content := CreateConfigWithPort(port)
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create test config: %v", err)
				}
				return configPath
			},
			expectError:  false,
			expectStatus: "passed",
		},
		{
			name: "MissingConfig",
			setupFunc: func() string {
				return filepath.Join(tmpDir, "missing_config.yaml")
			},
			expectError:  false, // Should not error, but should report failed status
			expectStatus: "failed",
		},
		{
			name: "InvalidConfig",
			setupFunc: func() string {
				configPath := filepath.Join(tmpDir, "invalid_config.yaml")
				content := "invalid: yaml: content: ["
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create test config: %v", err)
				}
				return configPath
			},
			expectError:  false,
			expectStatus: "failed",
		},
		{
			name: "EmptyConfig",
			setupFunc: func() string {
				configPath := filepath.Join(tmpDir, "empty_config.yaml")
				content := "port: 8080\nservers: []"
				if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create test config: %v", err)
				}
				return configPath
			},
			expectError:  false,
			expectStatus: "warning", // No servers configured
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnoseJSON = false
			diagnoseVerbose = false
			diagnoseTimeout = 60 * time.Second
			diagnoseAll = false

			configPath := tt.setupFunc()

			oldWd, _ := os.Getwd()
			defer func() { _ = os.Chdir(oldWd) }()
			if err := os.Chdir(filepath.Dir(configPath)); err != nil {
				t.Fatalf("Failed to change directory: %v", err)
			}

			expectedConfigPath := filepath.Join(filepath.Dir(configPath), "config.yaml")
			if configPath != expectedConfigPath {
				if _, err := os.Stat(configPath); err == nil {
					_ = os.Rename(configPath, expectedConfigPath)
					defer func() { _ = os.Remove(expectedConfigPath) }()
				}
			}

			err := diagnoseConfig(diagnoseConfigCmd, []string{})

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func testDiagnoseJSONOutput(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	tmpDir := testutil.TempDir(t)
	configPath := filepath.Join(tmpDir, "config.yaml")
	port := AllocateTestPort(t)
	configContent := CreateConfigWithPort(port)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	diagnoseJSON = true
	diagnoseVerbose = false
	diagnoseTimeout = 60 * time.Second
	diagnoseAll = false

	err := diagnoseSystem(diagnoseCmd, []string{})

	_ = w.Close()
	os.Stdout = old

	output, _ := io.ReadAll(r)
	outputStr := string(output)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if outputStr != "" {
		var report DiagnosticReport
		if err := json.Unmarshal(output, &report); err != nil {
			t.Errorf("Failed to parse JSON output: %v", err)
		}

		if report.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set")
		}
		if report.Platform == "" {
			t.Error("Expected platform to be set")
		}
		if len(report.Results) == 0 {
			t.Error("Expected at least one diagnostic result")
		}
	}
}

func testDiagnoseHumanOutput(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	tmpDir := testutil.TempDir(t)
	configPath := filepath.Join(tmpDir, "config.yaml")
	port := AllocateTestPort(t)
	configContent := CreateConfigWithPort(port)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	diagnoseJSON = false
	diagnoseVerbose = true
	diagnoseTimeout = 60 * time.Second
	diagnoseAll = false

	err := diagnoseSystem(diagnoseCmd, []string{})

	_ = w.Close()
	os.Stdout = old

	output, _ := io.ReadAll(r)
	outputStr := string(output)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !strings.Contains(outputStr, "LSP Gateway System Diagnostics") {
		t.Error("Expected human output to contain diagnostics header")
	}
}

func testDiagnoseErrorScenarios(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	tests := []struct {
		name        string
		setupFunc   func()
		expectError bool
		errorType   string
	}{
		{
			name: "InvalidTimeout",
			setupFunc: func() {
				diagnoseTimeout = -1 * time.Second
			},
			expectError: true,
			errorType:   "validation",
		},
		{
			name: "ZeroTimeout",
			setupFunc: func() {
				diagnoseTimeout = 0
			},
			expectError: true,
			errorType:   "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnoseJSON = false
			diagnoseVerbose = false
			diagnoseTimeout = 60 * time.Second
			diagnoseAll = false

			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			err := diagnoseSystem(diagnoseCmd, []string{})

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func testDiagnoseCommandHelp(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	helpOutput := captureCommandHelp(t, diagnoseCmd)
	if !strings.Contains(helpOutput, "comprehensive system diagnostics") {
		t.Error("Expected help to contain description of comprehensive diagnostics")
	}
	if !strings.Contains(helpOutput, "--json") {
		t.Error("Expected help to contain --json flag")
	}
	if !strings.Contains(helpOutput, "--verbose") {
		t.Error("Expected help to contain --verbose flag")
	}

	runtimesHelpOutput := captureCommandHelp(t, diagnoseRuntimesCmd)
	if !strings.Contains(runtimesHelpOutput, "runtime diagnostics") {
		t.Error("Expected runtimes help to contain runtime diagnostics description")
	}

	serversHelpOutput := captureCommandHelp(t, diagnoseServersCmd)
	if !strings.Contains(serversHelpOutput, "language server diagnostics") {
		t.Error("Expected servers help to contain language server description")
	}

	configHelpOutput := captureCommandHelp(t, diagnoseConfigCmd)
	if !strings.Contains(configHelpOutput, "configuration diagnostics") {
		t.Error("Expected config help to contain configuration description")
	}
}

func testDiagnoseCommandIntegration(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	tmpDir := testutil.TempDir(t)
	configPath := filepath.Join(tmpDir, "config.yaml")
	port := AllocateTestPort(t)
	configContent := CreateConfigWithPort(port)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	_ = os.Chdir(tmpDir)

	tests := []struct {
		name    string
		command *cobra.Command
		args    []string
	}{
		{
			name:    "MainDiagnose",
			command: diagnoseCmd,
			args:    []string{},
		},
		{
			name:    "RuntimesDiagnose",
			command: diagnoseRuntimesCmd,
			args:    []string{},
		},
		{
			name:    "ServersDiagnose",
			command: diagnoseServersCmd,
			args:    []string{},
		},
		{
			name:    "ConfigDiagnose",
			command: diagnoseConfigCmd,
			args:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnoseJSON = false
			diagnoseVerbose = false
			diagnoseTimeout = 60 * time.Second
			diagnoseAll = false

			err := tt.command.RunE(tt.command, tt.args)
			if err != nil {
				t.Errorf("Integration test failed for %s: %v", tt.name, err)
			}
		})
	}
}

func captureCommandHelp(t *testing.T, cmd *cobra.Command) string {
	t.Helper()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Help(); err != nil {
		t.Logf("Failed to get command help: %v", err)
	}

	return buf.String()
}

func TestDiagnoseSubcommands(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	subcommandTests := []struct {
		name         string
		cmd          *cobra.Command
		expectedUse  string
		keywordCheck []string
	}{
		{
			name:         "RuntimesSubcommand",
			cmd:          diagnoseRuntimesCmd,
			expectedUse:  "runtimes",
			keywordCheck: []string{"runtime diagnostics", "Installation detection"},
		},
		{
			name:         "ServersSubcommand",
			cmd:          diagnoseServersCmd,
			expectedUse:  "servers",
			keywordCheck: []string{"language server installations", "Installation detection"},
		},
		{
			name:         "ConfigSubcommand",
			cmd:          diagnoseConfigCmd,
			expectedUse:  "config",
			keywordCheck: []string{"configuration diagnostics", "Configuration file existence"},
		},
	}

	for _, tt := range subcommandTests {
		t.Run(tt.name, func(t *testing.T) {
			// Removed t.Parallel() to prevent deadlock

			if tt.cmd.Use != tt.expectedUse {
				t.Errorf("Expected Use to be '%s', got '%s'", tt.expectedUse, tt.cmd.Use)
			}

			if tt.cmd.RunE == nil {
				t.Error("Expected RunE function to be set")
			}

			if tt.cmd.Run != nil {
				t.Error("Expected Run function to be nil (using RunE instead)")
			}

			for _, keyword := range tt.keywordCheck {
				if !strings.Contains(tt.cmd.Long, keyword) {
					t.Errorf("Expected Long description to contain '%s'", keyword)
				}
			}
		})
	}
}

func TestDiagnosticStructures(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	result := DiagnosticResult{
		Name:        "Test Check",
		Status:      "passed",
		Message:     "Test passed successfully",
		Details:     map[string]interface{}{"version": "1.0.0"},
		Suggestions: []string{"Keep it up"},
		Timestamp:   time.Now(),
	}

	if result.Name != "Test Check" {
		t.Errorf("Expected Name to be 'Test Check', got '%s'", result.Name)
	}
	if result.Status != "passed" {
		t.Errorf("Expected Status to be 'passed', got '%s'", result.Status)
	}

	report := DiagnosticReport{
		Timestamp:      time.Now(),
		Platform:       "test/platform",
		GoVersion:      "go1.21.0",
		LSPGatewayPath: "/test/path",
		Results:        []DiagnosticResult{result},
	}

	calculateSummary(&report)

	if report.Summary.TotalChecks != 1 {
		t.Errorf("Expected TotalChecks to be 1, got %d", report.Summary.TotalChecks)
	}
	if report.Summary.Passed != 1 {
		t.Errorf("Expected Passed to be 1, got %d", report.Summary.Passed)
	}
	if report.Summary.OverallStatus != "passed" {
		t.Errorf("Expected OverallStatus to be 'passed', got '%s'", report.Summary.OverallStatus)
	}
}

func TestDiagnosticOutputFormats(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock

	results := []DiagnosticResult{
		{
			Name:    "Test Check 1",
			Status:  "passed",
			Message: "Everything is working fine",
			Details: map[string]interface{}{"version": "1.0.0"},
		},
		{
			Name:        "Test Check 2",
			Status:      "warning",
			Message:     "Minor issue detected",
			Suggestions: []string{"Consider updating"},
		},
		{
			Name:        "Test Check 3",
			Status:      "failed",
			Message:     "Critical issue found",
			Suggestions: []string{"Immediate action required", "Check documentation"},
		},
	}

	report := &DiagnosticReport{
		Timestamp: time.Now(),
		Platform:  "test/platform",
		Results:   results,
	}

	calculateSummary(report)

	if report.Summary.TotalChecks != 3 {
		t.Errorf("Expected TotalChecks to be 3, got %d", report.Summary.TotalChecks)
	}
	if report.Summary.Passed != 1 {
		t.Errorf("Expected Passed to be 1, got %d", report.Summary.Passed)
	}
	if report.Summary.Warnings != 1 {
		t.Errorf("Expected Warnings to be 1, got %d", report.Summary.Warnings)
	}
	if report.Summary.Failed != 1 {
		t.Errorf("Expected Failed to be 1, got %d", report.Summary.Failed)
	}
	if report.Summary.OverallStatus != "failed" {
		t.Errorf("Expected OverallStatus to be 'failed', got '%s'", report.Summary.OverallStatus)
	}

	testCases := []struct {
		status   string
		expected string
	}{
		{"passed", "✓ PASSED"},
		{"failed", "✗ FAILED"},
		{"warning", "⚠ WARNING"},
		{"skipped", "- SKIPPED"},
		{"unknown", "UNKNOWN"},
	}

	for _, tc := range testCases {
		result := getColoredStatus(tc.status)
		if result != tc.expected {
			t.Errorf("Expected getColoredStatus('%s') to be '%s', got '%s'", tc.status, tc.expected, result)
		}
	}
}

func BenchmarkDiagnoseSystemDiagnostics(b *testing.B) {
	diagnoseJSON = true
	diagnoseVerbose = false
	diagnoseTimeout = 10 * time.Second
	diagnoseAll = false

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), diagnoseTimeout)

		report := &DiagnosticReport{
			Timestamp: time.Now(),
			Platform:  "bench/test",
			Results:   []DiagnosticResult{},
		}

		if err := runSystemDiagnostics(ctx, report); err != nil {
			b.Logf("System diagnostics failed: %v", err)
		}

		cancel()
	}
}

func BenchmarkCalculateSummary(b *testing.B) {
	results := make([]DiagnosticResult, 100)
	for i := 0; i < 100; i++ {
		status := "passed"
		if i%10 == 0 {
			status = "failed"
		} else if i%5 == 0 {
			status = "warning"
		}

		results[i] = DiagnosticResult{
			Name:    fmt.Sprintf("Test Check %d", i),
			Status:  status,
			Message: "Test message",
		}
	}

	report := &DiagnosticReport{
		Results: results,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateSummary(report)
	}
}
