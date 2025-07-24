package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"lsp-gateway/internal/cli"
	"lsp-gateway/internal/types"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestStatusCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "status all",
			args:        []string{},
			expectError: false,
		},
		{
			name:        "status with json flag",
			args:        []string{"--json"},
			expectError: false,
		},
		{
			name:        "status with verbose flag",
			args:        []string{"--verbose"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create fresh command
			cmd := &cobra.Command{
				Use:  "status",
				RunE: cli.StatusAll,
			}
			cmd.Flags().Bool("json", false, "JSON output")
			cmd.Flags().Bool("verbose", false, "Verbose output")
			cmd.Flags().Duration("timeout", 30*time.Second, "Timeout")

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			if tt.expectError && err == nil {
				t.Errorf("Expected error for args %v, but got none", tt.args)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for args %v: %v", tt.args, err)
			}

			// Check output was generated
			if !tt.expectError && len(output) == 0 {
				t.Errorf("Expected output for args %v, but got none", tt.args)
			}
		})
	}
}

func TestStatusRuntimesCommand(t *testing.T) {
	tests := []struct {
		name        string
		jsonFlag    bool
		verboseFlag bool
	}{
		{
			name:        "runtimes default",
			jsonFlag:    false,
			verboseFlag: false,
		},
		{
			name:        "runtimes json",
			jsonFlag:    true,
			verboseFlag: false,
		},
		{
			name:        "runtimes verbose",
			jsonFlag:    false,
			verboseFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set flags
			*cli.StatusJSON = tt.jsonFlag
			*cli.StatusVerbose = tt.verboseFlag

			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cmd := &cobra.Command{Use: "runtimes"}
			err := cli.StatusRuntimes(cmd, []string{})

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			// Reset flags
			*cli.StatusJSON = false
			*cli.StatusVerbose = false

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check output format
			if tt.jsonFlag {
				var jsonOutput map[string]interface{}
				if err := json.Unmarshal(output, &jsonOutput); err != nil {
					t.Errorf("Expected valid JSON output, but got error: %v", err)
				}
			} else {
				outputStr := string(output)
				if !strings.Contains(outputStr, "Runtime") {
					t.Errorf("Expected human output to contain 'Runtime', but got: %s", outputStr)
				}
			}
		})
	}
}

func TestStatusRuntimeCommand(t *testing.T) {
	tests := []struct {
		name        string
		runtime     string
		expectError bool
	}{
		{
			name:        "go runtime",
			runtime:     "go",
			expectError: false,
		},
		{
			name:        "python runtime",
			runtime:     "python",
			expectError: false,
		},
		{
			name:        "nodejs runtime",
			runtime:     "nodejs",
			expectError: false,
		},
		{
			name:        "node alias",
			runtime:     "node",
			expectError: false,
		},
		{
			name:        "java runtime",
			runtime:     "java",
			expectError: false,
		},
		{
			name:        "invalid runtime",
			runtime:     "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cmd := &cobra.Command{Use: "runtime"}
			err := cli.StatusRuntime(cmd, []string{tt.runtime})

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			if tt.expectError && err == nil {
				t.Errorf("Expected error for runtime %s, but got none", tt.runtime)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for runtime %s: %v", tt.runtime, err)
			}

			// Check output was generated for valid runtimes
			if !tt.expectError && len(output) == 0 {
				t.Errorf("Expected output for runtime %s, but got none", tt.runtime)
			}
		})
	}
}

func TestStatusServersCommand(t *testing.T) {
	tests := []struct {
		name     string
		jsonFlag bool
	}{
		{
			name:     "servers default",
			jsonFlag: false,
		},
		{
			name:     "servers json",
			jsonFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set flag
			*cli.StatusJSON = tt.jsonFlag

			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cmd := &cobra.Command{Use: "servers"}
			err := cli.StatusServers(cmd, []string{})

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			// Reset flag
			*cli.StatusJSON = false

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check output was generated
			if len(output) == 0 {
				t.Errorf("Expected output, but got none")
			}

			// Check output format
			if tt.jsonFlag {
				var jsonOutput map[string]interface{}
				if err := json.Unmarshal(output, &jsonOutput); err != nil {
					t.Errorf("Expected valid JSON output, but got error: %v", err)
				}
			}
		})
	}
}

func TestStatusServerCommand(t *testing.T) {
	tests := []struct {
		name        string
		server      string
		expectError bool
	}{
		{
			name:        "gopls server",
			server:      "gopls",
			expectError: false,
		},
		{
			name:        "pylsp server",
			server:      "pylsp",
			expectError: false,
		},
		{
			name:        "typescript-language-server",
			server:      "typescript-language-server",
			expectError: false,
		},
		{
			name:        "jdtls server",
			server:      "jdtls",
			expectError: false,
		},
		{
			name:        "invalid server",
			server:      "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cmd := &cobra.Command{Use: "server"}
			err := cli.StatusServer(cmd, []string{tt.server})

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			if tt.expectError && err == nil {
				t.Errorf("Expected error for server %s, but got none", tt.server)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for server %s: %v", tt.server, err)
			}

			// Check output was generated for valid servers
			if !tt.expectError && len(output) == 0 {
				t.Errorf("Expected output for server %s, but got none", tt.server)
			}
		})
	}
}

func TestStatusHelperFunctions(t *testing.T) {
	tests := []struct {
		name     string
		function func() string
		input    interface{}
		expected string
	}{
		{
			name:     "getStatusIcon true",
			function: func() string { return cli.GetStatusIcon(true) },
			expected: "✓ Installed",
		},
		{
			name:     "getStatusIcon false",
			function: func() string { return cli.GetStatusIcon(false) },
			expected: "✗ Not Installed",
		},
		{
			name:     "getCompatibleText true",
			function: func() string { return cli.GetCompatibleText(true) },
			expected: "Yes",
		},
		{
			name:     "getCompatibleText false",
			function: func() string { return cli.GetCompatibleText(false) },
			expected: "No",
		},
		{
			name:     "getWorkingText true",
			function: func() string { return cli.GetWorkingText(true) },
			expected: "Yes",
		},
		{
			name:     "getWorkingText false",
			function: func() string { return cli.GetWorkingText(false) },
			expected: "No",
		},
		{
			name:     "formatRuntimeName go",
			function: func() string { return cli.FormatRuntimeName("go") },
			expected: "Go:",
		},
		{
			name:     "formatRuntimeName python",
			function: func() string { return cli.FormatRuntimeName("python") },
			expected: "Python:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function()
			if result != tt.expected {
				t.Errorf("Expected %q, but got %q", tt.expected, result)
			}
		})
	}
}

func TestStatusTableFunctions(t *testing.T) {
	// Test table output functions
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cli.OutputServersTableHeader()

	w.Close()
	output, _ := io.ReadAll(r)
	os.Stdout = oldStdout

	outputStr := string(output)
	if !strings.Contains(outputStr, "Language Server Status") {
		t.Errorf("Expected table header to contain 'Language Server Status', but got: %s", outputStr)
	}
}

func TestStatusJSONFunctions(t *testing.T) {
	// Test JSON initialization and building functions
	status := cli.InitializeStatusData()
	if status["success"] != true {
		t.Errorf("Expected success=true, got %v", status["success"])
	}

	if _, ok := status["timestamp"]; !ok {
		t.Error("Expected timestamp field in status data")
	}
}

func TestStatusDataBuilders(t *testing.T) {
	// Test runtime status data builder with successful verification
	installedCount := 0
	compatibleCount := 0
	
	// Mock successful verification result
	mockResult := &types.VerificationResult{
		Installed:  true,
		Compatible: true,
		Version:    "1.0.0",
		Path:       "/usr/bin/test",
	}

	data := cli.BuildRuntimeStatusData(mockResult, nil, &installedCount, &compatibleCount)

	if data["installed"] != true {
		t.Errorf("Expected installed=true, got %v", data["installed"])
	}
	if data["compatible"] != true {
		t.Errorf("Expected compatible=true, got %v", data["compatible"])
	}
	if data["version"] != "1.0.0" {
		t.Errorf("Expected version=1.0.0, got %v", data["version"])
	}
	if data["path"] != "/usr/bin/test" {
		t.Errorf("Expected path=/usr/bin/test, got %v", data["path"])
	}
	if installedCount != 1 {
		t.Errorf("Expected installedCount=1, got %d", installedCount)
	}
	if compatibleCount != 1 {
		t.Errorf("Expected compatibleCount=1, got %d", compatibleCount)
	}

	// Test with error
	installedCount2 := 0
	compatibleCount2 := 0
	testErr := fmt.Errorf("test error")
	
	errorData := cli.BuildRuntimeStatusData(nil, testErr, &installedCount2, &compatibleCount2)
	if errorData["installed"] != false {
		t.Errorf("Expected installed=false for error case, got %v", errorData["installed"])
	}
	if errorData["compatible"] != false {
		t.Errorf("Expected compatible=false for error case, got %v", errorData["compatible"])
	}
	if errorData["error"] != "test error" {
		t.Errorf("Expected error message, got %v", errorData["error"])
	}
	if installedCount2 != 0 {
		t.Errorf("Expected installedCount2=0 for error case, got %d", installedCount2)
	}
	if compatibleCount2 != 0 {
		t.Errorf("Expected compatibleCount2=0 for error case, got %d", compatibleCount2)
	}
}

func TestStatusCommandFlags(t *testing.T) {
	// Test that flags are properly defined
	cmd := cli.GetStatusCmd()

	jsonFlag := cmd.PersistentFlags().Lookup("json")
	if jsonFlag == nil {
		t.Error("json flag not found")
	}

	verboseFlag := cmd.PersistentFlags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("verbose flag not found")
	}

	timeoutFlag := cmd.PersistentFlags().Lookup("timeout")
	if timeoutFlag == nil {
		t.Error("timeout flag not found")
	}
}

func TestStatusSubcommands(t *testing.T) {
	// Test that subcommands are properly added
	expectedSubcommands := []string{"runtimes", "runtime", "servers", "server"}
	actualSubcommands := make([]string, 0)

	for _, cmd := range cli.GetStatusCmd().Commands() {
		actualSubcommands = append(actualSubcommands, cmd.Use)
	}

	for _, expected := range expectedSubcommands {
		found := false
		for _, actual := range actualSubcommands {
			if strings.Contains(actual, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected subcommand %s not found in %v", expected, actualSubcommands)
		}
	}
}

func TestStatusAll(t *testing.T) {
	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create command and call StatusAll
	cmd := &cobra.Command{Use: "status"}
	err := cli.StatusAll(cmd, []string{})

	w.Close()
	output, _ := io.ReadAll(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("StatusAll() error = %v", err)
	}

	// Should generate some output
	if len(output) == 0 {
		t.Error("Expected output from StatusAll(), but got none")
	}
}

// Benchmark status operations
func BenchmarkStatusAll(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Capture output using pipe
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Create command and call StatusAll
		cmd := &cobra.Command{Use: "status"}
		_ = cli.StatusAll(cmd, []string{})

		w.Close()
		_, _ = io.ReadAll(r)
		os.Stdout = oldStdout
	}
}

func BenchmarkStatusRuntimes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Capture output using pipe
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		*cli.StatusJSON = false
		// Create command and call StatusRuntimes
		cmd := &cobra.Command{Use: "runtimes"}
		_ = cli.StatusRuntimes(cmd, []string{})

		w.Close()
		_, _ = io.ReadAll(r)
		os.Stdout = oldStdout
	}
}
