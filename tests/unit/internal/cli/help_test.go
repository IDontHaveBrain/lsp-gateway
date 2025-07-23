package cli_test

import (
	"lsp-gateway/internal/cli"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHelpWorkflowsCommand(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectError    bool
		expectedOutput string
	}{
		{
			name:           "show all workflows",
			args:           []string{},
			expectError:    false,
			expectedOutput: "LSP Gateway Workflow Examples",
		},
		{
			name:           "first-time workflow",
			args:           []string{"first-time"},
			expectError:    false,
			expectedOutput: "First-Time Setup Workflow",
		},
		{
			name:           "development workflow",
			args:           []string{"development"},
			expectError:    false,
			expectedOutput: "Development Workflow",
		},
		{
			name:           "maintenance workflow",
			args:           []string{"maintenance"},
			expectError:    false,
			expectedOutput: "Maintenance Workflow",
		},
		{
			name:           "integration workflow",
			args:           []string{"integration"},
			expectError:    false,
			expectedOutput: "Integration Workflow",
		},
		{
			name:           "troubleshooting workflow",
			args:           []string{"troubleshooting"},
			expectError:    false,
			expectedOutput: "Troubleshooting Workflow",
		},
		{
			name:           "unknown workflow",
			args:           []string{"unknown"},
			expectError:    true,
			expectedOutput: "unknown workflow",
		},
		{
			name:        "too many args",
			args:        []string{"first-time", "extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create fresh command
			cmd := &cobra.Command{
				Use:  "workflows",
				Args: cobra.MaximumNArgs(1),
				RunE: runHelpWorkflows,
			}

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

			if tt.expectedOutput != "" && !strings.Contains(string(output), tt.expectedOutput) {
				t.Errorf("Expected output to contain %q, but got: %s", tt.expectedOutput, string(output))
			}
		})
	}
}

func TestRunHelpWorkflows(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{"no args", []string{}, false},
		{"first-time", []string{"first-time"}, false},
		{"development", []string{"development"}, false},
		{"maintenance", []string{"maintenance"}, false},
		{"integration", []string{"integration"}, false},
		{"troubleshooting", []string{"troubleshooting"}, false},
		{"invalid workflow", []string{"invalid"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cmd := &cobra.Command{Use: "workflows"}
			err := runHelpWorkflows(cmd, tt.args)

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			if tt.expectError && err == nil {
				t.Errorf("Expected error for args %v, but got none", tt.args)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for args %v: %v", tt.args, err)
			}

			// Check that some output was generated
			if !tt.expectError && len(output) == 0 {
				t.Errorf("Expected output for args %v, but got none", tt.args)
			}
		})
	}
}

func TestWorkflowFunctions(t *testing.T) {
	// Test each workflow function individually
	workflowTests := []struct {
		name     string
		function func()
		expected string
	}{
		{
			name:     "showAllWorkflows",
			function: showAllWorkflows,
			expected: "LSP Gateway Workflow Examples",
		},
		{
			name:     "showFirstTimeWorkflow",
			function: showFirstTimeWorkflow,
			expected: "First-Time Setup Workflow",
		},
		{
			name:     "showDevelopmentWorkflow",
			function: showDevelopmentWorkflow,
			expected: "Development Workflow",
		},
		{
			name:     "showMaintenanceWorkflow",
			function: showMaintenanceWorkflow,
			expected: "Maintenance Workflow",
		},
		{
			name:     "showIntegrationWorkflow",
			function: showIntegrationWorkflow,
			expected: "Integration Workflow",
		},
		{
			name:     "showTroubleshootingWorkflow",
			function: showTroubleshootingWorkflow,
			expected: "Troubleshooting Workflow",
		},
	}

	for _, tt := range workflowTests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			tt.function()

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			if !strings.Contains(string(output), tt.expected) {
				t.Errorf("Expected output to contain %q, but got: %s", tt.expected, string(output))
			}

			// Check that output contains useful information
			outputStr := string(output)
			if len(outputStr) < 100 {
				t.Errorf("Expected substantial output, but got only %d characters", len(outputStr))
			}
		})
	}
}

func TestHelpWorkflowsCommandMetadata(t *testing.T) {
	if helpWorkflowsCmd.Use != "workflows" {
		t.Errorf("helpWorkflowsCmd.Use = %s, want workflows", helpWorkflowsCmd.Use)
	}

	if helpWorkflowsCmd.Short == "" {
		t.Error("helpWorkflowsCmd.Short should not be empty")
	}

	if helpWorkflowsCmd.Long == "" {
		t.Error("helpWorkflowsCmd.Long should not be empty")
	}

	// Test that the command has proper argument validation
	if helpWorkflowsCmd.Args == nil {
		t.Error("helpWorkflowsCmd.Args should not be nil")
	}
}

func TestWorkflowContent(t *testing.T) {
	// Test that each workflow contains expected content
	contentTests := []struct {
		name            string
		function        func()
		expectedContent []string
	}{
		{
			name:     "first-time workflow",
			function: showFirstTimeWorkflow,
			expectedContent: []string{
				"lsp-gateway setup all",
				"lsp-gateway server",
				"STEP 1",
				"STEP 2",
				"VERIFICATION",
			},
		},
		{
			name:     "development workflow",
			function: showDevelopmentWorkflow,
			expectedContent: []string{
				"lsp-gateway status",
				"lsp-gateway server",
				"DAILY STARTUP",
				"CONFIGURATION MANAGEMENT",
				"RUNTIME MANAGEMENT",
			},
		},
		{
			name:     "maintenance workflow",
			function: showMaintenanceWorkflow,
			expectedContent: []string{
				"lsp-gateway diagnose",
				"REGULAR HEALTH CHECKS",
				"UPDATING COMPONENTS",
				"BACKUP AND RESTORE",
			},
		},
		{
			name:     "integration workflow",
			function: showIntegrationWorkflow,
			expectedContent: []string{
				"IDE/EDITOR INTEGRATION",
				"AI ASSISTANT INTEGRATION",
				"http://localhost:8080/jsonrpc",
				"MCP",
			},
		},
		{
			name:     "troubleshooting workflow",
			function: showTroubleshootingWorkflow,
			expectedContent: []string{
				"lsp-gateway diagnose",
				"STEP 1",
				"STEP 2",
				"COMMON ISSUES",
				"SYSTEMATIC DEBUGGING",
			},
		},
	}

	for _, tt := range contentTests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			tt.function()

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			outputStr := string(output)
			for _, expectedContent := range tt.expectedContent {
				if !strings.Contains(outputStr, expectedContent) {
					t.Errorf("Expected workflow %s to contain %q, but it didn't. Output: %s",
						tt.name, expectedContent, outputStr)
				}
			}
		})
	}
}

func TestHelpWorkflowsIntegration(t *testing.T) {
	// Test that workflows command is properly added to root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "workflows" {
			found = true
			break
		}
	}
	if !found {
		t.Error("workflows command not found in root command")
	}
}

// Benchmark workflow output generation
func BenchmarkAllWorkflows(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Capture output using pipe
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		showAllWorkflows()

		w.Close()
		_, _ = io.ReadAll(r)
		os.Stdout = oldStdout
	}
}

func BenchmarkFirstTimeWorkflow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Capture output using pipe
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		showFirstTimeWorkflow()

		w.Close()
		_, _ = io.ReadAll(r)
		os.Stdout = oldStdout
	}
}
