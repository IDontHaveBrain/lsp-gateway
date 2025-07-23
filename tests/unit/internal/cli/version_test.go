package cli_test

import (
	"lsp-gateway/internal/cli"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestVersionCommand(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testVersionCommandMetadata},
		{"Execution", testVersionCommandExecution},
		{"OutputFormat", testVersionCommandOutputFormat},
		{"RuntimeInfo", testVersionCommandRuntimeInfo},
		{"Help", testVersionCommandHelp},
		{"Integration", testVersionCommandIntegration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func testVersionCommandMetadata(t *testing.T) {
	if versionCmd.Use != CmdVersion {
		t.Errorf("Expected Use to be 'version', got '%s'", versionCmd.Use)
	}

	expectedShort := "Show version and build information"
	if versionCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, versionCmd.Short)
	}

	// Updated to check for key phrases rather than exact match due to comprehensive long description
	if !strings.Contains(versionCmd.Long, "Display comprehensive version and build information") {
		t.Errorf("Expected Long to contain comprehensive version information description, got '%s'", versionCmd.Long)
	}

	if versionCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}

	if versionCmd.Run != nil {
		t.Error("Expected Run function to be nil (using RunE instead)")
	}
}

func testVersionCommandExecution(t *testing.T) {
	output := captureStdout(t, func() {
		err := versionCmd.RunE(versionCmd, []string{})
		if err != nil {
			t.Errorf("Expected no error from version command execution, got: %v", err)
		}
	})

	if output == "" {
		t.Error("Expected version command to produce output")
	}

	if !strings.Contains(output, "LSP Gateway Version Information") {
		t.Errorf("Expected output to contain 'LSP Gateway Version Information', got: %s", output)
	}
}

func testVersionCommandOutputFormat(t *testing.T) {
	output := captureStdout(t, func() {
		err := versionCmd.RunE(versionCmd, []string{})
		if err != nil {
			t.Fatalf("Version command execution failed: %v", err)
		}
	})

	// Check for expected content patterns in the rich output format
	requiredPatterns := []struct {
		pattern string
		desc    string
	}{
		{`LSP Gateway Version Information`, "Should contain main header"},
		{`Version:\s+\w+`, "Should contain version information"},
		{`Runtime Information:`, "Should contain runtime section"},
		{`Go Version:\s+go\d+\.\d+`, "Should contain Go version"},
		{`Platform:\s+\w+`, "Should contain platform information"},
		{`Architecture:\s+\w+`, "Should contain architecture information"},
	}

	for _, ep := range requiredPatterns {
		matched, err := regexp.MatchString(ep.pattern, output)
		if err != nil {
			t.Errorf("Regex error for %s: %v", ep.desc, err)
			continue
		}

		if !matched {
			t.Errorf("%s: pattern '%s' not found in output:\n%s", ep.desc, ep.pattern, output)
		}
	}
}

func testVersionCommandRuntimeInfo(t *testing.T) {
	output := captureStdout(t, func() {
		err := versionCmd.RunE(versionCmd, []string{})
		if err != nil {
			t.Fatalf("Version command execution failed: %v", err)
		}
	})

	runtimeVersion := runtime.Version()
	if !strings.Contains(output, "Go Version:") {
		t.Error("Expected output to contain 'Go Version:'")
	}
	if !strings.Contains(output, runtimeVersion) {
		t.Errorf("Expected output to contain Go version '%s', got output: %s", runtimeVersion, output)
	}

	// Check for platform and architecture separately as they are on different lines
	expectedPlatform := fmt.Sprintf("Platform:     %s", runtime.GOOS)
	expectedArch := fmt.Sprintf("Architecture: %s", runtime.GOARCH)
	if !strings.Contains(output, expectedPlatform) {
		t.Errorf("Expected output to contain '%s', got output: %s", expectedPlatform, output)
	}
	if !strings.Contains(output, expectedArch) {
		t.Errorf("Expected output to contain '%s', got output: %s", expectedArch, output)
	}
}

func testVersionCommandHelp(t *testing.T) {
	output := captureStdout(t, func() {
		testVersionCmd := &cobra.Command{
			Use:   versionCmd.Use,
			Short: versionCmd.Short,
			Long:  versionCmd.Long,
			RunE:  versionCmd.RunE,
		}
		testVersionCmd.SetArgs([]string{"--help"})
		err := testVersionCmd.Execute()
		if err != nil {
			t.Errorf("Help command should not return error, got: %v", err)
		}
	})

	expectedElements := []string{
		"Display comprehensive version and build information",
		"version",
		"Usage:",
		"Flags:",
	}

	for _, element := range expectedElements {
		if !strings.Contains(output, element) {
			t.Errorf("Expected help output to contain '%s', got:\n%s", element, output)
		}
	}
}

func testVersionCommandIntegration(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == CmdVersion {
			found = true
			break
		}
	}

	if !found {
		t.Error("version command should be added to root command")
	}

	output := captureStdout(t, func() {
		testRoot := &cobra.Command{
			Use:   rootCmd.Use,
			Short: rootCmd.Short,
		}
		testVersion := &cobra.Command{
			Use:   versionCmd.Use,
			Short: versionCmd.Short,
			RunE:  versionCmd.RunE,
		}
		testRoot.AddCommand(testVersion)

		testRoot.SetArgs([]string{"version"})
		err := testRoot.Execute()
		if err != nil {
			t.Errorf("Expected no error executing version through root, got: %v", err)
		}
	})

	if !strings.Contains(output, "LSP Gateway Version Information") {
		t.Error("Expected version output when executed through root command")
	}
}

func TestVersionCommandEdgeCases(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"WithArgs", testVersionCommandWithArgs},
		{"MultipleExecutions", testVersionCommandMultipleExecutions},
		{"OutputBuffering", testVersionCommandOutputBuffering},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func testVersionCommandWithArgs(t *testing.T) {
	output := captureStdout(t, func() {
		err := versionCmd.RunE(versionCmd, []string{"extra", "args"})
		if err != nil {
			t.Errorf("Version command should ignore extra args, got error: %v", err)
		}
	})

	if !strings.Contains(output, "LSP Gateway Version Information") {
		t.Error("Version command should work with extra arguments")
	}
}

func testVersionCommandMultipleExecutions(t *testing.T) {
	var outputs []string

	for i := 0; i < 3; i++ {
		output := captureStdout(t, func() {
			err := versionCmd.RunE(versionCmd, []string{})
			if err != nil {
				t.Errorf("Execution %d failed: %v", i, err)
			}
		})
		outputs = append(outputs, output)
	}

	for i := 1; i < len(outputs); i++ {
		if outputs[i] != outputs[0] {
			t.Errorf("Execution %d output differs from first execution:\nFirst: %s\nCurrent: %s",
				i, outputs[0], outputs[i])
		}
	}
}

func testVersionCommandOutputBuffering(t *testing.T) {
	var buf bytes.Buffer

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := versionCmd.RunE(versionCmd, []string{})
	if err != nil {
		t.Errorf("Version command execution failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Logf("cleanup error closing writer: %v", err)
	}
	os.Stdout = oldStdout

	if _, err := io.Copy(&buf, r); err != nil {
		t.Logf("error copying output: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "LSP Gateway Version Information") {
		t.Error("Expected buffered output to contain version information")
	}
}

func TestVersionCommandCompleteness(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock
	if versionCmd.Name() != CmdVersion {
		t.Errorf("Expected command name 'version', got '%s'", versionCmd.Name())
	}

	if versionCmd.HasPersistentFlags() {
		t.Error("Version command should not have persistent flags")
	}

	if !versionCmd.HasLocalFlags() {
		t.Error("Version command should have local flags (--json)")
	}

	if versionCmd.Flag("json") == nil {
		t.Error("Version command should have --json flag")
	}

	if versionCmd.HasSubCommands() {
		t.Error("Version command should not have subcommands")
	}

	if versionCmd.Flag("help") == nil {
		if versionCmd.Use == "" {
			t.Error("Command use should not be empty")
		}
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Logf("cleanup error closing writer: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("Failed to read captured output: %v", err)
	}

	return buf.String()
}

func BenchmarkVersionCommandExecution(b *testing.B) {
	for i := 0; i < b.N; i++ {
		captureStdout(&testing.T{}, func() {
			if err := versionCmd.RunE(versionCmd, []string{}); err != nil {
				b.Logf("version command error: %v", err)
			}
		})
	}
}

func BenchmarkVersionCommandOutputParsing(b *testing.B) {
	output := captureStdout(&testing.T{}, func() {
		if err := versionCmd.RunE(versionCmd, []string{}); err != nil {
			b.Logf("version command error: %v", err)
		}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		for _, line := range lines {
			_ = strings.Contains(line, ":")
		}
	}
}
