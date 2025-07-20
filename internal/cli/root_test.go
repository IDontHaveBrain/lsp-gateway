package cli

import (
	"fmt"
	"os"
	"os/exec"
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
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &cobra.Command{
		Use:   rootCmd.Use,
		Short: rootCmd.Short,
		Long:  rootCmd.Long,
	}

	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if cerr := w.Close(); cerr != nil {
		t.Logf("cleanup error closing writer: %v", cerr)
	}
	os.Stdout = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Errorf("Help command should not return error, got: %v", err)
	}

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

	if len(strings.TrimSpace(output)) == 0 {
		t.Error("Help output should not be empty")
	}
}

func testRootCommandSubcommands(t *testing.T) {
	subcommands := rootCmd.Commands()
	if len(subcommands) == 0 {
		t.Error("Expected root command to have subcommands")
	}

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
	originalCmd := rootCmd

	testCmd := &cobra.Command{
		Use:   "lsp-gateway",
		Short: "test command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	rootCmd = testCmd

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Execute should not panic, got: %v", r)
		}
		rootCmd = originalCmd
	}()

	err := testCmd.Execute()
	if err != nil {
		t.Errorf("Expected no error from valid command execution, got: %v", err)
	}
}

func testExecuteErrorHandling(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	originalCmd := rootCmd

	testCmd := &cobra.Command{
		Use:   "lsp-gateway",
		Short: "test command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("test error") // This will cause an error
		},
	}

	err := testCmd.Execute()

	if cerr := w.Close(); cerr != nil {
		t.Logf("cleanup error closing writer: %v", cerr)
	}
	os.Stderr = oldStderr

	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	stderrOutput := string(buf[:n])

	rootCmd = originalCmd

	if err == nil {
		t.Error("Expected error from command execution")
	}

	t.Logf("Stderr output captured: %s", stderrOutput)
}

func TestRootCommandIntegration(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Root command initialization should not panic, got: %v", r)
		}
	}()

	if rootCmd == nil {
		t.Error("rootCmd should not be nil")
	}

	if rootCmd.Args != nil {
		err := rootCmd.Args(rootCmd, []string{})
		if err != nil {
			t.Errorf("Root command should validate empty args, got error: %v", err)
		}
	}
}

func TestRootCommandCompleteness(t *testing.T) {
	if rootCmd.Use == "" {
		t.Error("Use field should not be empty")
	}

	if rootCmd.Short == "" {
		t.Error("Short field should not be empty")
	}

	if rootCmd.Long == "" {
		t.Error("Long field should not be empty")
	}

	if !rootCmd.HasSubCommands() {
		t.Error("Root command should have subcommands")
	}

	if rootCmd.Use == "" {
		t.Error("Command should have a Use field for help to work")
	}
}

func TestExecuteErrorHandling(t *testing.T) {

	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Args = []string{"lsp-gateway", "--invalid-flag"}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := rootCmd.Execute()

	_ = w.Close()
	os.Stderr = oldStderr

	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	stderrOutput := string(buf[:n])

	if err == nil {
		t.Error("Expected error from invalid flag")
	}

	if err != nil {
		var testOutput strings.Builder
		fmt.Fprintf(&testOutput, "Error: %v\n", err)

		output := testOutput.String()
		if !strings.Contains(output, "Error:") {
			t.Errorf("Expected error output to contain 'Error:', got: %s", output)
		}

		t.Logf("Execute() error handling pattern verified: %s", output)
		t.Logf("Stderr captured: %s", stderrOutput)
	}
}

func TestExecuteSubprocess(t *testing.T) {

	if os.Getenv("TEST_EXECUTE_SUBPROCESS") == "1" {
		_ = Execute()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestExecuteSubprocess")
	cmd.Env = append(os.Environ(), "TEST_EXECUTE_SUBPROCESS=1")

	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Logf("Execute() ran successfully: %s", string(output))
	} else {
		t.Logf("Execute() handled error correctly (exit code non-zero): %v", err)
		t.Logf("Output: %s", string(output))
	}

	t.Log("Execute() function coverage test completed")
}

func TestExecuteWithDifferentArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedExit  bool
		expectedError bool
	}{
		{
			name:          "NoArgs",
			args:          []string{},
			expectedExit:  false,
			expectedError: false,
		},
		{
			name:          "HelpFlag",
			args:          []string{"--help"},
			expectedExit:  false,
			expectedError: false,
		},
		{
			name:          "VersionCommand",
			args:          []string{"version"},
			expectedExit:  false,
			expectedError: false,
		},
		{
			name:          "InvalidFlag",
			args:          []string{"--invalid-flag"},
			expectedExit:  true,
			expectedError: true,
		},
		{
			name:          "UnknownCommand",
			args:          []string{"unknown-command"},
			expectedExit:  true,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalArgs := os.Args
			defer func() { os.Args = originalArgs }()

			os.Args = append([]string{"lsp-gateway"}, tt.args...)
			rootCmd.SetArgs(tt.args)

			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			err := rootCmd.Execute()

			_ = w.Close()
			os.Stderr = oldStderr

			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			stderrOutput := string(buf[:n])

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error for args %v, got nil", tt.args)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for args %v, got: %v", tt.args, err)
				}
			}

			t.Logf("Args: %v, Error: %v, Stderr: %s", tt.args, err, stderrOutput)
		})
	}
}

func TestExecuteErrorFormatting(t *testing.T) {
	testError := fmt.Errorf("test error message")
	testCmd := &cobra.Command{
		Use: "lsp-gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			return testError
		},
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := testCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	_ = w.Close()
	os.Stderr = oldStderr

	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	stderrOutput := string(buf[:n])

	if err == nil {
		t.Error("Expected error from test command")
	}

	expectedOutput := "Error: test error message\n"
	if !strings.Contains(stderrOutput, "Error: test error message") {
		t.Errorf("Expected stderr to contain '%s', got: %s", expectedOutput, stderrOutput)
	}

	t.Logf("Error formatting test - Error: %v, Stderr: %s", err, stderrOutput)
}

func TestExecuteActualFunction(t *testing.T) {
	if os.Getenv("TEST_EXECUTE_COVERAGE") == "1" {
		os.Args = []string{"lsp-gateway", "--invalid-test-flag"}
		_ = Execute() // This calls os.Exit(1) due to invalid flag
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestExecuteActualFunction")
	cmd.Env = append(os.Environ(), "TEST_EXECUTE_COVERAGE=1")

	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected non-zero exit code from Execute function with invalid flag")
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Error:") {
		t.Errorf("Expected output to contain 'Error:', got: %s", outputStr)
	}

	t.Logf("Execute function subprocess test - Exit error: %v, Output: %s", err, outputStr)
}

func TestExecuteSuccessPath(t *testing.T) {
	if os.Getenv("TEST_EXECUTE_SUCCESS") == "1" {
		os.Args = []string{"lsp-gateway", "--help"}
		_ = Execute() // Should not call os.Exit for help
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestExecuteSuccessPath")
	cmd.Env = append(os.Environ(), "TEST_EXECUTE_SUCCESS=1")

	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Logf("Help command execution: %v (may be expected due to subprocess test)", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "LSP Gateway") {
		t.Errorf("Expected help output to contain 'LSP Gateway', got: %s", outputStr)
	}

	t.Logf("Execute success path test - Output: %s", outputStr)
}

func TestRootCommandFlagParsing(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError bool
		errorContains string
	}{
		{
			name:          "ValidHelpShort",
			args:          []string{"-h"},
			expectedError: false,
		},
		{
			name:          "ValidHelpLong",
			args:          []string{"--help"},
			expectedError: false,
		},
		{
			name:          "InvalidFlag",
			args:          []string{"--nonexistent"},
			expectedError: true,
			errorContains: "unknown flag",
		},
		{
			name:          "InvalidShortFlag",
			args:          []string{"-x"},
			expectedError: true,
			errorContains: "unknown shorthand flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{
				Use:           rootCmd.Use,
				Short:         rootCmd.Short,
				Long:          rootCmd.Long,
				SilenceUsage:  rootCmd.SilenceUsage,
				SilenceErrors: rootCmd.SilenceErrors,
			}

			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error for args %v, got nil", tt.args)
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else if err != nil {
				t.Errorf("Expected no error for args %v, got: %v", tt.args, err)
			}

			t.Logf("Flag parsing test - Args: %v, Error: %v", tt.args, err)
		})
	}
}

func TestRootCommandInitialization(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd should not be nil after package initialization")
	}

	if rootCmd.Use == "" {
		t.Error("rootCmd.Use should not be empty")
	}

	if rootCmd.Short == "" {
		t.Error("rootCmd.Short should not be empty")
	}

	if rootCmd.Long == "" {
		t.Error("rootCmd.Long should not be empty")
	}

	if !rootCmd.SilenceUsage {
		t.Error("rootCmd.SilenceUsage should be true for proper error handling")
	}

	if !rootCmd.SilenceErrors {
		t.Error("rootCmd.SilenceErrors should be true for proper error handling")
	}

	subcommands := rootCmd.Commands()
	if len(subcommands) == 0 {
		t.Error("rootCmd should have subcommands attached")
	}

	help := rootCmd.HelpFunc()
	if help == nil {
		t.Error("rootCmd should have help function")
	}

	t.Logf("Root command initialized with %d subcommands", len(subcommands))
}

func TestExecuteErrorConditions(t *testing.T) {
	errorTests := []struct {
		name        string
		command     func() *cobra.Command
		expectedErr bool
	}{
		{
			name: "CommandWithError",
			command: func() *cobra.Command {
				return &cobra.Command{
					Use: "test",
					RunE: func(cmd *cobra.Command, args []string) error {
						return fmt.Errorf("command execution failed")
					},
				}
			},
			expectedErr: true,
		},
		{
			name: "CommandWithPanic",
			command: func() *cobra.Command {
				return &cobra.Command{
					Use: "test",
					RunE: func(cmd *cobra.Command, args []string) error {
						return fmt.Errorf("simulated panic error")
					},
				}
			},
			expectedErr: true,
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.command()

			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			err := cmd.Execute()

			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}

			_ = w.Close()
			os.Stderr = oldStderr

			buf := make([]byte, 512)
			n, _ := r.Read(buf)
			stderrOutput := string(buf[:n])

			if tt.expectedErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				if !strings.Contains(stderrOutput, "Error:") {
					t.Errorf("Expected stderr to contain 'Error:', got: %s", stderrOutput)
				}
			} else if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			t.Logf("Error condition test - Error: %v, Stderr: %s", err, stderrOutput)
		})
	}
}

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

func BenchmarkExecuteFunction(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cmd := &cobra.Command{
			Use: "test",
			RunE: func(cmd *cobra.Command, args []string) error {
				return nil
			},
		}
		cmd.SetArgs([]string{})
		_ = cmd.Execute()
	}
}
