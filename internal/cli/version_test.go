package cli

import (
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

	expectedShort := "Show version information"
	if versionCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, versionCmd.Short)
	}

	expectedLong := "Show version information for LSP Gateway."
	if versionCmd.Long != expectedLong {
		t.Errorf("Expected Long to be '%s', got '%s'", expectedLong, versionCmd.Long)
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

	if !strings.Contains(output, "LSP Gateway MVP") {
		t.Errorf("Expected output to contain 'LSP Gateway MVP', got: %s", output)
	}
}

func testVersionCommandOutputFormat(t *testing.T) {
	output := captureStdout(t, func() {
		err := versionCmd.RunE(versionCmd, []string{})
		if err != nil {
			t.Fatalf("Version command execution failed: %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines of output, got %d: %v", len(lines), lines)
	}

	expectedPatterns := []struct {
		line    int
		pattern string
		desc    string
	}{
		{0, `^LSP Gateway MVP$`, "First line should be 'LSP Gateway MVP'"},
		{1, `^Go Version: go\d+\.\d+(\.\d+)?.*$`, "Second line should be Go version"},
		{2, `^OS/Arch: \w+/\w+$`, "Third line should be OS/Arch"},
	}

	for _, ep := range expectedPatterns {
		if ep.line >= len(lines) {
			t.Errorf("Missing line %d for %s", ep.line, ep.desc)
			continue
		}

		matched, err := regexp.MatchString(ep.pattern, lines[ep.line])
		if err != nil {
			t.Errorf("Regex error for %s: %v", ep.desc, err)
			continue
		}

		if !matched {
			t.Errorf("%s: expected pattern '%s', got '%s'", ep.desc, ep.pattern, lines[ep.line])
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

	expectedOSArch := fmt.Sprintf("OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	if !strings.Contains(output, expectedOSArch) {
		t.Errorf("Expected output to contain '%s', got output: %s", expectedOSArch, output)
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
		"Show version information",
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

	if !strings.Contains(output, "LSP Gateway MVP") {
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

	if !strings.Contains(output, "LSP Gateway MVP") {
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

	if !strings.Contains(output, "LSP Gateway MVP") {
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
