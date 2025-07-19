package cli

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testRootCommandMetadata},
		{"Help", testRootCommandHelp},
		{"Subcommands", testRootCommandSubcommands},
		{"SilenceFlags", testRootCommandSilenceFlags},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func testRootCommandMetadata(t *testing.T) {
	if rootCmd.Use != "lsp-gateway" {
		t.Errorf("Expected Use to be 'lsp-gateway', got '%s'", rootCmd.Use)
	}

	expectedShort := "LSP Gateway - A JSON-RPC gateway for Language Server Protocol servers"
	if rootCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, rootCmd.Short)
	}

	// Verify Long description contains key features
	expectedKeywords := []string{
		"unified JSON-RPC interface",
		"Multi-language LSP server management",
		"JSON-RPC interface over HTTP",
		"Automatic server routing",
		"YAML configuration",
	}

	for _, keyword := range expectedKeywords {
		if !strings.Contains(rootCmd.Long, keyword) {
			t.Errorf("Expected Long description to contain '%s'", keyword)
		}
	}
}

func testRootCommandHelp(t *testing.T) {
	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a new command instance to avoid affecting global state
	cmd := &cobra.Command{
		Use:   rootCmd.Use,
		Short: rootCmd.Short,
		Long:  rootCmd.Long,
	}

	// Add help flag
	cmd.SetArgs([]string{"--help"})

	// Execute and capture
	err := cmd.Execute()
	if cerr := w.Close(); cerr != nil {
		t.Logf("cleanup error closing writer: %v", cerr)
	}
	os.Stdout = oldStdout

	// Read output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Errorf("Help command should not return error, got: %v", err)
	}

	// Verify help output contains expected elements
	// When --help is used, it shows either the Long description or full help
	expectedElements := []string{
		"unified JSON-RPC interface", // From Long description
		"Language Server Protocol",   // From Long description
		"Multi-language LSP server",  // From Long description
	}

	for _, element := range expectedElements {
		if !strings.Contains(output, element) {
			t.Errorf("Expected help output to contain '%s', got:\n%s", element, output)
		}
	}

	// Verify it's not empty
	if len(strings.TrimSpace(output)) == 0 {
		t.Error("Help output should not be empty")
	}
}

func testRootCommandSubcommands(t *testing.T) {
	// Check that root command has subcommands
	subcommands := rootCmd.Commands()
	if len(subcommands) == 0 {
		t.Error("Expected root command to have subcommands")
	}

	// Verify expected subcommands exist
	expectedCommands := map[string]bool{
		"version": false,
		"server":  false,
	}

	for _, cmd := range subcommands {
		if _, exists := expectedCommands[cmd.Name()]; exists {
			expectedCommands[cmd.Name()] = true
		}
	}

	for cmdName, found := range expectedCommands {
		if !found {
			t.Errorf("Expected subcommand '%s' not found", cmdName)
		}
	}
}

func testRootCommandSilenceFlags(t *testing.T) {
	if !rootCmd.SilenceUsage {
		t.Error("Expected SilenceUsage to be true")
	}

	if !rootCmd.SilenceErrors {
		t.Error("Expected SilenceErrors to be true")
	}
}

func TestExecute(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"WithValidCommand", testExecuteWithValidCommand},
		{"ErrorHandling", testExecuteErrorHandling},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func testExecuteWithValidCommand(t *testing.T) {
	// Save original command
	originalCmd := rootCmd

	// Create a test command that doesn't exit
	testCmd := &cobra.Command{
		Use:   "lsp-gateway",
		Short: "test command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	// Replace rootCmd temporarily
	rootCmd = testCmd

	// Test should not panic or exit
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Execute should not panic, got: %v", r)
		}
		// Restore original command
		rootCmd = originalCmd
	}()

	// Since Execute calls os.Exit on error, we can't directly test it
	// Instead, test that rootCmd.Execute() works
	err := testCmd.Execute()
	if err != nil {
		t.Errorf("Expected no error from valid command execution, got: %v", err)
	}
}

func testExecuteErrorHandling(t *testing.T) {
	// Save original stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Save original command
	originalCmd := rootCmd

	// Create a test command that returns an error
	testCmd := &cobra.Command{
		Use:   "lsp-gateway",
		Short: "test command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("test error") // This will cause an error
		},
	}

	// Test that error is handled (we can't test os.Exit directly)
	err := testCmd.Execute()

	// Restore stderr
	if cerr := w.Close(); cerr != nil {
		t.Logf("cleanup error closing writer: %v", cerr)
	}
	os.Stderr = oldStderr

	// Read captured stderr
	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	stderrOutput := string(buf[:n])

	// Restore original command
	rootCmd = originalCmd

	// Verify error handling
	if err == nil {
		t.Error("Expected error from command execution")
	}

	// Note: We can't test the actual Execute() function that calls os.Exit
	// but we can verify the error handling pattern works
	t.Logf("Stderr output captured: %s", stderrOutput)
}

func TestRootCommandIntegration(t *testing.T) {
	// Test that the command can be initialized without panics
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Root command initialization should not panic, got: %v", r)
		}
	}()

	// Verify command is properly initialized
	if rootCmd == nil {
		t.Error("rootCmd should not be nil")
	}

	// Test command validation
	if rootCmd.Args != nil {
		err := rootCmd.Args(rootCmd, []string{})
		if err != nil {
			t.Errorf("Root command should validate empty args, got error: %v", err)
		}
	}
}

// TestRootCommandCompleteness verifies the command structure is complete
func TestRootCommandCompleteness(t *testing.T) {
	// Verify required fields are set
	if rootCmd.Use == "" {
		t.Error("Use field should not be empty")
	}

	if rootCmd.Short == "" {
		t.Error("Short field should not be empty")
	}

	if rootCmd.Long == "" {
		t.Error("Long field should not be empty")
	}

	// Verify command tree structure
	if !rootCmd.HasSubCommands() {
		t.Error("Root command should have subcommands")
	}

	// Test that help is available (help is automatically added by Cobra)
	// We can verify this by checking the command has proper structure
	if rootCmd.Use == "" {
		t.Error("Command should have a Use field for help to work")
	}
}

// Benchmark the command creation and execution
func BenchmarkRootCommandCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cmd := &cobra.Command{
			Use:   rootCmd.Use,
			Short: rootCmd.Short,
			Long:  rootCmd.Long,
		}
		_ = cmd
	}
}
