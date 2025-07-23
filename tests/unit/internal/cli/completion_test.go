package cli_test

import (
	"lsp-gateway/internal/cli"
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
			if cli.GetCompletionCmd().Args != nil {
				argsErr := cli.GetCompletionCmd().Args(cli.GetCompletionCmd(), tt.args)
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
			rootCmd := &cobra.Command{Use: "lsp-gateway"}

			// Create a copy of the actual completion command to avoid affecting other tests
			cmd := &cobra.Command{
				Use:       cli.GetCompletionCmd().Use,
				ValidArgs: cli.GetCompletionCmd().ValidArgs,
				Args:      cli.GetCompletionCmd().Args,
				RunE:      cli.GetCompletionCmd().RunE,
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
			cmd := &cobra.Command{Use: "completion"}

			runCompletionFn := cli.GetRunCompletion()
			err := runCompletionFn(cmd, []string{tt.shell})

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
	// Test that ValidArgs were set during command initialization
	// ValidArgs are now set in the command definitions in their respective files
	
	expectedRuntimeArgs := []string{"go", "python", "nodejs", "java", "all"}
	installRuntimeCmd := cli.GetInstallRuntimeCmd()
	if !stringSlicesEqual(installRuntimeCmd.ValidArgs, expectedRuntimeArgs) {
		t.Errorf("installRuntimeCmd.ValidArgs = %v, want %v", installRuntimeCmd.ValidArgs, expectedRuntimeArgs)
	}

	verifyRuntimeCmd := cli.GetVerifyRuntimeCmd()
	if !stringSlicesEqual(verifyRuntimeCmd.ValidArgs, expectedRuntimeArgs) {
		t.Errorf("verifyRuntimeCmd.ValidArgs = %v, want %v", verifyRuntimeCmd.ValidArgs, expectedRuntimeArgs)
	}

	expectedServerArgs := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}
	installServerCmd := cli.GetInstallServerCmd()
	if !stringSlicesEqual(installServerCmd.ValidArgs, expectedServerArgs) {
		t.Errorf("installServerCmd.ValidArgs = %v, want %v", installServerCmd.ValidArgs, expectedServerArgs)
	}
}

func TestCompletionCommandMetadata(t *testing.T) {
	if cli.GetCompletionCmd().Use != "completion" {
		t.Errorf("cli.GetCompletionCmd().Use = %s, want completion", cli.GetCompletionCmd().Use)
	}

	if cli.GetCompletionCmd().Short == "" {
		t.Error("cli.GetCompletionCmd().Short should not be empty")
	}

	if cli.GetCompletionCmd().Long == "" {
		t.Error("cli.GetCompletionCmd().Long should not be empty")
	}

	expectedValidArgs := []string{"bash", "zsh", "fish", "powershell"}
	if !stringSlicesEqual(cli.GetCompletionCmd().ValidArgs, expectedValidArgs) {
		t.Errorf("cli.GetCompletionCmd().ValidArgs = %v, want %v", cli.GetCompletionCmd().ValidArgs, expectedValidArgs)
	}

	// Test that the command has proper argument validation
	if cli.GetCompletionCmd().Args == nil {
		t.Error("cli.GetCompletionCmd().Args should not be nil")
	}
}

func TestCompletionIntegration(t *testing.T) {
	// Test completion command is properly added to root
	found := false
	for _, cmd := range cli.GetRootCmd().Commands() {
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
	cmd := &cobra.Command{Use: "completion"}

	for i := 0; i < b.N; i++ {
		// Capture output using pipe
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		runCompletionFn := cli.GetRunCompletion()
		runCompletionFn(cmd, []string{"bash"})

		w.Close()
		_, _ = io.ReadAll(r)
		os.Stdout = oldStdout
	}
}
