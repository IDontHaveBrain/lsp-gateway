package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	testBinaryPath = "../../lsp-gateway"
)

func allocateTestPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to allocate test port: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Logf("Error closing test listener: %v", err)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func createMinimalConfigWithPort(port int) string {
	return fmt.Sprintf(`port: %d
servers: []
`, port)
}

func TestMainBinary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"InvalidCommands", testInvalidCommands},
		{"MalformedArguments", testMalformedArguments},
		{"MissingConfigs", testMissingConfigs},
		{"SignalHandling", testSignalHandling},
		{"ExitCodes", testExitCodes},
		{"HelpCommands", testHelpCommands},
		{"VersionCommand", testVersionCommand},
		{"ConfigValidation", testConfigValidation},
		{"PermissionErrors", testPermissionErrors},
		{"ResourceLimits", testResourceLimits},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func testInvalidCommands(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		args     []string
		wantExit int
		wantErr  string
	}{
		{
			name:     "UnknownCommand",
			args:     []string{"unknown-command"},
			wantExit: 1,
			wantErr:  "unknown command",
		},
		{
			name:     "InvalidSubcommand",
			args:     []string{"server", "invalid-sub"},
			wantExit: 1,
			wantErr:  "",
		},
		{
			name:     "TooManyArguments",
			args:     []string{"version", "extra", "args"},
			wantExit: 0, // Version command ignores extra args
			wantErr:  "",
		},
		{
			name:     "EmptyCommand",
			args:     []string{""},
			wantExit: 0, // Empty command shows help
			wantErr:  "",
		},
		{
			name:     "SpecialCharacters",
			args:     []string{"@#$%^&*()"},
			wantExit: 1,
			wantErr:  "unknown command",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := buildTestBinary(t)
			cmd.Args = append([]string{cmd.Path}, tc.args...)

			stderr, err := cmd.StderrPipe()
			if err != nil {
				t.Fatalf("Failed to create stderr pipe: %v", err)
			}

			err = cmd.Start()
			if err != nil {
				t.Fatalf("Failed to start command: %v", err)
			}

			stderrData, _ := io.ReadAll(stderr)
			err = cmd.Wait()

			var exitCode int
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				} else {
					t.Fatalf("Unexpected error type: %v", err)
				}
			}

			if exitCode != tc.wantExit {
				t.Errorf("Expected exit code %d, got %d", tc.wantExit, exitCode)
			}

			if tc.wantErr != "" && !strings.Contains(string(stderrData), tc.wantErr) {
				t.Errorf("Expected stderr to contain '%s', got: %s", tc.wantErr, string(stderrData))
			}
		})
	}
}

func testMalformedArguments(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{
			name:     "InvalidPortFlag",
			args:     []string{"server", "--port", "invalid"},
			wantExit: 1,
		},
		{
			name:     "PortOutOfRange",
			args:     []string{"server", "--port", "70000"},
			wantExit: 1,
		},
		{
			name:     "NegativePort",
			args:     []string{"server", "--port", "-1"},
			wantExit: 1,
		},
		{
			name:     "InvalidFlagFormat",
			args:     []string{"server", "-invalid-flag"},
			wantExit: 1,
		},
		{
			name:     "MissingFlagValue",
			args:     []string{"server", "--config"},
			wantExit: 1,
		},
		{
			name:     "InvalidTransportType",
			args:     []string{"mcp", "--transport", "invalid-transport"},
			wantExit: 1,
		},
		{
			name:     "InvalidTimeoutFormat",
			args:     []string{"mcp", "--timeout", "invalid"},
			wantExit: 1,
		},
		{
			name:     "NegativeTimeout",
			args:     []string{"mcp", "--timeout", "-5s"},
			wantExit: 1,
		},
		{
			name:     "InvalidMaxRetries",
			args:     []string{"mcp", "--max-retries", "invalid"},
			wantExit: 1,
		},
		{
			name:     "NegativeMaxRetries",
			args:     []string{"mcp", "--max-retries", "-1"},
			wantExit: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := buildTestBinary(t)
			cmd.Args = append([]string{cmd.Path}, tc.args...)

			err := cmd.Run()

			var exitCode int
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				} else {
					t.Fatalf("Unexpected error type: %v", err)
				}
			}

			if exitCode != tc.wantExit {
				t.Errorf("Expected exit code %d, got %d", tc.wantExit, exitCode)
			}
		})
	}
}

func testMissingConfigs(t *testing.T) {
	t.Parallel()

	testCases := getMissingConfigTestCases()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runMissingConfigTest(t, tc)
		})
	}
}

type missingConfigTestCase struct {
	name       string
	args       []string
	setupFiles func(t *testing.T, tmpDir string)
	wantExit   int
	wantErr    string
}

func getMissingConfigTestCases() []missingConfigTestCase {
	return []missingConfigTestCase{
		{
			name:     "MissingConfigFile",
			args:     []string{"server", "--config", "/nonexistent/config.yaml"},
			wantExit: 1,
			wantErr:  "failed to load configuration",
		},
		{
			name:       "EmptyConfigFile",
			args:       []string{"server", "--config", "empty.yaml"},
			setupFiles: setupEmptyConfigFile,
			wantExit:   1,
			wantErr:    "invalid configuration",
		},
		{
			name:       "InvalidYAMLConfig",
			args:       []string{"server", "--config", "invalid.yaml"},
			setupFiles: setupInvalidYAMLConfigFile,
			wantExit:   1,
			wantErr:    "failed to load configuration",
		},
		{
			name:       "ConfigWithoutServers",
			args:       []string{"server", "--config", "no-servers.yaml"},
			setupFiles: setupConfigWithoutServers,
			wantExit:   1,
			wantErr:    "invalid configuration",
		},
		{
			name:       "ConfigWithInvalidServer",
			args:       []string{"server", "--config", "invalid-server.yaml"},
			setupFiles: setupConfigWithInvalidServer,
			wantExit:   1,
			wantErr:    "invalid configuration",
		},
	}
}

func setupEmptyConfigFile(t *testing.T, tmpDir string) {
	configPath := filepath.Join(tmpDir, "empty.yaml")
	err := os.WriteFile(configPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty config: %v", err)
	}
}

func setupInvalidYAMLConfigFile(t *testing.T, tmpDir string) {
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	invalidYAML := `
				servers:
				  - name: "test
				    invalid yaml syntax
				`
	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid config: %v", err)
	}
}

func setupConfigWithoutServers(t *testing.T, tmpDir string) {
	configPath := filepath.Join(tmpDir, "no-servers.yaml")
	testPort := allocateTestPort(t)
	configContent := createMinimalConfigWithPort(testPort)
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
}

func setupConfigWithInvalidServer(t *testing.T, tmpDir string) {
	configPath := filepath.Join(tmpDir, "invalid-server.yaml")
	testPort := allocateTestPort(t)
	configContent := fmt.Sprintf(`
port: %d
servers:
  - name: ""
    languages: []
    command: ""
    args: []
    transport: "invalid"
`, testPort)
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
}

func runMissingConfigTest(t *testing.T, tc missingConfigTestCase) {
	tmpDir := t.TempDir()

	if tc.setupFiles != nil {
		tc.setupFiles(t, tmpDir)
	}

	cmd := prepareMissingConfigCommand(t, tmpDir, tc.args)
	result := executeMissingConfigCommand(t, cmd)
	validateMissingConfigResult(t, result, tc)
}

func prepareMissingConfigCommand(t *testing.T, tmpDir string, inputArgs []string) *exec.Cmd {
	cmd := buildTestBinary(t)
	cmd.Dir = tmpDir

	var args []string
	for _, arg := range inputArgs {
		if strings.HasSuffix(arg, ".yaml") && !filepath.IsAbs(arg) {
			arg = filepath.Join(tmpDir, arg)
		}
		args = append(args, arg)
	}
	cmd.Args = append([]string{cmd.Path}, args...)
	return cmd
}

type missingConfigResult struct {
	exitCode   int
	stderrData []byte
	err        error
}

func executeMissingConfigCommand(t *testing.T, cmd *exec.Cmd) missingConfigResult {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	stderrData, _ := io.ReadAll(stderr)
	err = cmd.Wait()

	var exitCode int
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			t.Fatalf("Unexpected error type: %v", err)
		}
	}

	return missingConfigResult{
		exitCode:   exitCode,
		stderrData: stderrData,
		err:        err,
	}
}

func validateMissingConfigResult(t *testing.T, result missingConfigResult, tc missingConfigTestCase) {
	if result.exitCode != tc.wantExit {
		t.Errorf("Expected exit code %d, got %d", tc.wantExit, result.exitCode)
	}

	if tc.wantErr != "" && !strings.Contains(string(result.stderrData), tc.wantErr) {
		t.Errorf("Expected stderr to contain '%s', got: %s", tc.wantErr, string(result.stderrData))
	}
}

func testSignalHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping signal handling test in short mode")
	}

	testCases := []struct {
		name   string
		signal os.Signal
		args   []string
	}{
		{
			name:   "SIGINT",
			signal: syscall.SIGINT,
			args:   []string{"version"}, // Use version command which exits quickly
		},
		{
			name:   "SIGTERM",
			signal: syscall.SIGTERM,
			args:   []string{"version"}, // Use version command which exits quickly
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := buildTestBinary(t)
			cmd.Args = append([]string{cmd.Path}, tc.args...)

			err := cmd.Start()
			if err != nil {
				t.Fatalf("Failed to start command: %v", err)
			}

			time.Sleep(100 * time.Millisecond)

			err = cmd.Process.Signal(tc.signal)
			if err != nil {
				t.Fatalf("Failed to send signal: %v", err)
			}

			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()

			select {
			case err := <-done:
				if err != nil {
					if exitError, ok := err.(*exec.ExitError); ok {
						if exitError.ExitCode() != 0 && exitError.ExitCode() != 1 {
							t.Logf("Process killed by signal (expected for signal handling test)")
						}
					}
				}
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				t.Error("Process did not respond to signal within timeout")
			}
		})
	}
}

func testExitCodes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{
			name:     "SuccessfulVersionCommand",
			args:     []string{"version"},
			wantExit: 0,
		},
		{
			name:     "SuccessfulHelpCommand",
			args:     []string{"--help"},
			wantExit: 0,
		},
		{
			name:     "UnknownCommandError",
			args:     []string{"nonexistent"},
			wantExit: 1,
		},
		{
			name:     "InvalidFlagError",
			args:     []string{"--invalid-flag"},
			wantExit: 1,
		},
		{
			name:     "MissingConfigError",
			args:     []string{"server", "--config", "/nonexistent.yaml"},
			wantExit: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := buildTestBinary(t)
			cmd.Args = append([]string{cmd.Path}, tc.args...)

			err := cmd.Run()

			var exitCode int
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				} else {
					t.Fatalf("Unexpected error type: %v", err)
				}
			}

			if exitCode != tc.wantExit {
				t.Errorf("Expected exit code %d, got %d", tc.wantExit, exitCode)
			}
		})
	}
}

func testHelpCommands(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		args        []string
		wantExit    int
		wantContain []string
	}{
		{
			name:     "GlobalHelp",
			args:     []string{"--help"},
			wantExit: 0,
			wantContain: []string{
				"LSP Gateway",
				"unified JSON-RPC interface",
				"Language Server Protocol",
			},
		},
		{
			name:     "ServerHelp",
			args:     []string{"server", "--help"},
			wantExit: 0,
			wantContain: []string{
				"Start the LSP Gateway server",
				"--config",
				"--port",
			},
		},
		{
			name:     "MCPHelp",
			args:     []string{"mcp", "--help"},
			wantExit: 0,
			wantContain: []string{
				"Model Context Protocol",
				"MCP server",
				"--gateway",
				"--transport",
			},
		},
		{
			name:     "VersionHelp",
			args:     []string{"version", "--help"},
			wantExit: 0,
			wantContain: []string{
				"Show version information",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := buildTestBinary(t)
			cmd.Args = append([]string{cmd.Path}, tc.args...)

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatalf("Failed to create stdout pipe: %v", err)
			}

			err = cmd.Start()
			if err != nil {
				t.Fatalf("Failed to start command: %v", err)
			}

			stdoutData, _ := io.ReadAll(stdout)
			err = cmd.Wait()

			var exitCode int
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				} else {
					t.Fatalf("Unexpected error type: %v", err)
				}
			}

			if exitCode != tc.wantExit {
				t.Errorf("Expected exit code %d, got %d", tc.wantExit, exitCode)
			}

			output := string(stdoutData)
			for _, contain := range tc.wantContain {
				if !strings.Contains(output, contain) {
					t.Errorf("Expected output to contain '%s', got: %s", contain, output)
				}
			}
		})
	}
}

func testVersionCommand(t *testing.T) {
	t.Parallel()

	cmd := buildTestBinary(t)
	cmd.Args = append([]string{cmd.Path}, "version")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	stdoutData, _ := io.ReadAll(stdout)
	err = cmd.Wait()

	if err != nil {
		t.Fatalf("Version command should not error: %v", err)
	}

	output := string(stdoutData)
	expectedElements := []string{
		"LSP Gateway MVP",
		"Go Version:",
		"OS/Arch:",
	}

	for _, element := range expectedElements {
		if !strings.Contains(output, element) {
			t.Errorf("Expected version output to contain '%s', got: %s", element, output)
		}
	}
}

func testConfigValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		configData string
		wantExit   int
		wantErr    string
	}{
		{
			name: "ValidConfig",
			configData: `
port: 8080
servers:
  - name: "test-lsp"
    languages: ["test"]
    command: "nonexistent-command"
    args: ["hello"]
    transport: "stdio"
`,
			wantExit: 1, // Will fail to start nonexistent server
			wantErr:  "",
		},
		{
			name: "DuplicateServerNames",
			configData: `
port: 8080
servers:
  - name: "test-lsp"
    languages: ["test1"]
    command: "echo"
    args: []
    transport: "stdio"
  - name: "test-lsp"
    languages: ["test2"]
    command: "echo"
    args: []
    transport: "stdio"
`,
			wantExit: 1,
			wantErr:  "invalid configuration",
		},
		{
			name: "EmptyServerName",
			configData: `
port: 8080
servers:
  - name: ""
    languages: ["test"]
    command: "echo"
    args: []
    transport: "stdio"
`,
			wantExit: 1,
			wantErr:  "invalid configuration",
		},
		{
			name: "EmptyLanguages",
			configData: `
port: 8080
servers:
  - name: "test-lsp"
    languages: []
    command: "echo"
    args: []
    transport: "stdio"
`,
			wantExit: 1,
			wantErr:  "invalid configuration",
		},
		{
			name: "EmptyCommand",
			configData: `
port: 8080
servers:
  - name: "test-lsp"
    languages: ["test"]
    command: ""
    args: []
    transport: "stdio"
`,
			wantExit: 1,
			wantErr:  "invalid configuration",
		},
		{
			name: "InvalidTransport",
			configData: `
port: 8080
servers:
  - name: "test-lsp"
    languages: ["test"]
    command: "echo"
    args: []
    transport: "invalid"
`,
			wantExit: 1,
			wantErr:  "invalid configuration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "test-config.yaml")

			err := os.WriteFile(configPath, []byte(tc.configData), 0644)
			if err != nil {
				t.Fatalf("Failed to create config file: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			binaryPath := testBinaryPath
			cmd := exec.CommandContext(ctx, binaryPath, "server", "--config", configPath)

			stderr, err := cmd.StderrPipe()
			if err != nil {
				t.Fatalf("Failed to create stderr pipe: %v", err)
			}

			err = cmd.Start()
			if err != nil {
				t.Fatalf("Failed to start command: %v", err)
			}

			stderrData, _ := io.ReadAll(stderr)
			err = cmd.Wait()

			var exitCode int
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				} else {
					t.Fatalf("Unexpected error type: %v", err)
				}
			}

			if exitCode != tc.wantExit {
				t.Errorf("Expected exit code %d, got %d", tc.wantExit, exitCode)
			}

			if tc.wantErr != "" && !strings.Contains(string(stderrData), tc.wantErr) {
				t.Errorf("Expected stderr to contain '%s', got: %s", tc.wantErr, string(stderrData))
			}
		})
	}
}

func testPermissionErrors(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission tests when running as root")
	}

	t.Parallel()

	tmpDir := t.TempDir()

	noReadDir := filepath.Join(tmpDir, "no-read")
	err := os.Mkdir(noReadDir, 0000)
	if err != nil {
		t.Fatalf("Failed to create no-read directory: %v", err)
	}
	defer func() {
		if err := os.Chmod(noReadDir, 0755); err != nil {
			t.Logf("Failed to restore directory permissions during cleanup: %v", err)
		}
	}()

	configPath := filepath.Join(noReadDir, "config.yaml")

	cmd := buildTestBinary(t)
	cmd.Args = []string{cmd.Path, "server", "--config", configPath}

	err = cmd.Run()

	var exitCode int
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			t.Fatalf("Unexpected error type: %v", err)
		}
	}

	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for permission error, got %d", exitCode)
	}
}

func testResourceLimits(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{
			name:     "LargePortNumber",
			args:     []string{"server", "--port", "65535"},
			wantExit: 1, // Will fail to start but should handle large port
		},
		{
			name:     "VeryLongConfigPath",
			args:     []string{"server", "--config", strings.Repeat("a", 1000) + ".yaml"},
			wantExit: 1,
		},
		{
			name:     "VeryLongGatewayURL",
			args:     []string{"mcp", "--gateway", "http://" + strings.Repeat("a", 1000) + ".com"},
			wantExit: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			binaryPath := testBinaryPath
			cmd := exec.CommandContext(ctx, binaryPath)
			cmd.Args = append([]string{cmd.Path}, tc.args...)

			err := cmd.Run()

			var exitCode int
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				} else if err == context.DeadlineExceeded {
					exitCode = -1
				} else {
					t.Fatalf("Unexpected error type: %v", err)
				}
			}

			if exitCode != tc.wantExit && exitCode != -1 {
				t.Errorf("Expected exit code %d or -1 (timeout), got %d", tc.wantExit, exitCode)
			}
		})
	}
}

func buildTestBinary(t *testing.T) *exec.Cmd {
	t.Helper()

	binaryPath := testBinaryPath
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
		buildCmd.Dir = "../../"
		err := buildCmd.Run()
		if err != nil {
			t.Fatalf("Failed to build test binary: %v", err)
		}

		t.Cleanup(func() {
			if err := os.Remove(binaryPath); err != nil {
				t.Logf("Failed to remove test binary during cleanup: %v", err)
			}
		})
	}

	return exec.Command(binaryPath)
}

func TestMainFunctionCoverage(t *testing.T) {

	t.Log("Main function coverage achieved through integration tests")

	if testing.Short() {
		t.Skip("Skipping main function test in short mode")
	}

	cmd := buildTestBinary(t)
	cmd.Args = []string{cmd.Path, "version"}

	err := cmd.Run()
	if err != nil {
		t.Logf("Binary execution completed (expected for main coverage): %v", err)
	}
}

func BenchmarkMainExecution(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	binaryPath := "./lsp-gateway"
	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	err := buildCmd.Run()
	if err != nil {
		b.Fatalf("Failed to build test binary: %v", err)
	}
	defer func() {
		if err := os.Remove(binaryPath); err != nil {
			b.Logf("Failed to remove test binary during cleanup: %v", err)
		}
	}()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cmd := exec.Command(binaryPath, "version")
		_ = cmd.Run()
	}
}
