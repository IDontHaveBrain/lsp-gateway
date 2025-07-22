package cli

import (
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "bash completion",
			args:        []string{"bash"},
			expectError: false,
		},
		{
			name:        "zsh completion",
			args:        []string{"zsh"},
			expectError: false,
		},
		{
			name:        "fish completion",
			args:        []string{"fish"},
			expectError: false,
		},
		{
			name:        "powershell completion",
			args:        []string{"powershell"},
			expectError: false,
		},
		{
			name:        "invalid shell",
			args:        []string{"invalid"},
			expectError: true,
		},
		{
			name:        "no args",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "too many args",
			args:        []string{"bash", "extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test argument validation directly first
			if completionCmd.Args != nil {
				argsErr := completionCmd.Args(completionCmd, tt.args)
				if tt.expectError && argsErr == nil {
					t.Errorf("Expected argument validation error for args %v, but got none", tt.args)
					return
				}
				if !tt.expectError && argsErr != nil {
					t.Errorf("Unexpected argument validation error for args %v: %v", tt.args, argsErr)
					return
				}
			}

			// If expecting error and got it from args validation, we're done
			if tt.expectError {
				return
			}

			// Capture output for successful cases
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Mock root command for completion generation
			rootCmd = &cobra.Command{Use: "lsp-gateway"}
			
			// Create a copy of the actual completion command to avoid affecting other tests
			cmd := &cobra.Command{
				Use:       completionCmd.Use,
				ValidArgs: completionCmd.ValidArgs,
				Args:      completionCmd.Args,
				RunE:      completionCmd.RunE,
			}
			rootCmd.AddCommand(cmd)

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			if err != nil {
				t.Errorf("Unexpected error for args %v: %v", tt.args, err)
			}

			// For valid shells, check that some completion content was generated
			if len(output) == 0 {
				t.Errorf("Expected completion output for shell %v, but got none", tt.args)
			}
		})
	}
}

func TestRunCompletion(t *testing.T) {
	tests := []struct {
		name        string
		shell       string
		expectError bool
	}{
		{"bash", "bash", false},
		{"zsh", "zsh", false},
		{"fish", "fish", false},
		{"powershell", "powershell", false},
		{"invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Mock root command
			rootCmd = &cobra.Command{Use: "lsp-gateway"}
			cmd := &cobra.Command{Use: "completion"}

			err := runCompletion(cmd, []string{tt.shell})

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			if tt.expectError && err == nil {
				t.Errorf("Expected error for shell %s, but got none", tt.shell)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for shell %s: %v", tt.shell, err)
			}

			// For valid shells, check output contains completion-related content
			if !tt.expectError && len(output) == 0 {
				t.Errorf("Expected completion output for shell %s, but got none", tt.shell)
			}
		})
	}
}

func TestSetupCompletions(t *testing.T) {
	// Mock commands for testing
	installRuntimeCmd = &cobra.Command{Use: "runtime"}
	verifyRuntimeCmd = &cobra.Command{Use: "runtime"}
	installServerCmd = &cobra.Command{Use: "server"}
	serverCmd = &cobra.Command{Use: "server"}
	mcpCmd = &cobra.Command{Use: "mcp"}
	configCmd = &cobra.Command{Use: "config"}

	// Add flags to test completion registration
	serverCmd.Flags().String("config", "", "config file")
	mcpCmd.Flags().String("config", "", "config file")
	configCmd.Flags().String("config", "", "config file")

	// Call setupCompletions
	setupCompletions()

	// Test that ValidArgs were set
	expectedRuntimeArgs := []string{"go", "python", "nodejs", "java", "all"}
	if !stringSlicesEqual(installRuntimeCmd.ValidArgs, expectedRuntimeArgs) {
		t.Errorf("installRuntimeCmd.ValidArgs = %v, want %v", installRuntimeCmd.ValidArgs, expectedRuntimeArgs)
	}

	if !stringSlicesEqual(verifyRuntimeCmd.ValidArgs, expectedRuntimeArgs) {
		t.Errorf("verifyRuntimeCmd.ValidArgs = %v, want %v", verifyRuntimeCmd.ValidArgs, expectedRuntimeArgs)
	}

	expectedServerArgs := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}
	if !stringSlicesEqual(installServerCmd.ValidArgs, expectedServerArgs) {
		t.Errorf("installServerCmd.ValidArgs = %v, want %v", installServerCmd.ValidArgs, expectedServerArgs)
	}
}

func TestCompletionCommandMetadata(t *testing.T) {
	if completionCmd.Use != "completion" {
		t.Errorf("completionCmd.Use = %s, want completion", completionCmd.Use)
	}

	if completionCmd.Short == "" {
		t.Error("completionCmd.Short should not be empty")
	}

	if completionCmd.Long == "" {
		t.Error("completionCmd.Long should not be empty")
	}

	expectedValidArgs := []string{"bash", "zsh", "fish", "powershell"}
	if !stringSlicesEqual(completionCmd.ValidArgs, expectedValidArgs) {
		t.Errorf("completionCmd.ValidArgs = %v, want %v", completionCmd.ValidArgs, expectedValidArgs)
	}

	// Test that the command has proper argument validation
	if completionCmd.Args == nil {
		t.Error("completionCmd.Args should not be nil")
	}
}

func TestCompletionIntegration(t *testing.T) {
	// Test completion command is properly added to root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "completion" {
			found = true
			break
		}
	}
	if !found {
		t.Error("completion command not found in root command")
	}
}

// Helper function to compare string slices
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Benchmark completion generation
func BenchmarkBashCompletion(b *testing.B) {
	rootCmd = &cobra.Command{Use: "lsp-gateway"}
	cmd := &cobra.Command{Use: "completion"}

	for i := 0; i < b.N; i++ {
		// Capture output using pipe
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		runCompletion(cmd, []string{"bash"})

		w.Close()
		_, _ = io.ReadAll(r)
		os.Stdout = oldStdout
	}
}
