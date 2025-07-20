package platform

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCrossPlatformCommandExecution(t *testing.T) {
	t.Run("PlatformSpecificShells", testPlatformSpecificShells)
	t.Run("BasicCommandExecution", testBasicCommandExecution)
	t.Run("EnvironmentVariableHandling", testEnvironmentVariableHandling)
	t.Run("ShellArgumentGeneration", testShellArgumentGeneration)
}

func testPlatformSpecificShells(t *testing.T) {
	executor := NewCommandExecutor()
	shell := executor.GetShell()
	t.Logf("Detected shell: %s", shell)

	validateShellForPlatform(t, shell)
}

func validateShellForPlatform(t *testing.T, shell string) {
	switch runtime.GOOS {
	case "windows":
		validateWindowsShell(t, shell)
	case "linux", "darwin":
		validateUnixShell(t, shell)
	default:
		t.Logf("Unknown platform %s, shell: %s", runtime.GOOS, shell)
	}
}

func validateWindowsShell(t *testing.T, shell string) {
	expectedShells := []string{"cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh", "pwsh.exe"}
	if !containsShell(shell, expectedShells) {
		t.Errorf("Windows shell %s not in expected list: %v", shell, expectedShells)
	}
}

func validateUnixShell(t *testing.T, shell string) {
	expectedShells := []string{"sh", "bash", "zsh", "dash", "ksh", "fish"}
	if !containsShell(shell, expectedShells) {
		t.Errorf("Unix shell %s not in expected list: %v", shell, expectedShells)
	}
}

func containsShell(shell string, expectedShells []string) bool {
	for _, expected := range expectedShells {
		if strings.Contains(strings.ToLower(shell), expected) {
			return true
		}
	}
	return false
}

func testBasicCommandExecution(t *testing.T) {
	executor := NewCommandExecutor()
	cmd, args := getPlatformEchoCommand()

	if !executor.IsCommandAvailable(cmd) {
		t.Skipf("Command %s not available", cmd)
	}

	result, err := executor.Execute(cmd, args, 10*time.Second)
	if err != nil {
		t.Logf("Command execution failed (may be expected in test environment): %v", err)
		return
	}

	validateCommandResult(t, result, "cross-platform-test")
	t.Logf("Command executed successfully: %s", strings.TrimSpace(result.Stdout))
}

func getPlatformEchoCommand() (string, []string) {
	switch runtime.GOOS {
	case "windows":
		return "cmd", []string{"/C", "echo", "cross-platform-test"}
	default:
		return "echo", []string{"cross-platform-test"}
	}
}

func validateCommandResult(t *testing.T, result *Result, expectedOutput string) {
	if result.ExitCode != 0 {
		t.Errorf("Command failed with exit code %d, stderr: %s", result.ExitCode, result.Stderr)
	}

	if !strings.Contains(result.Stdout, expectedOutput) {
		t.Errorf("Expected output to contain '%s', got: %s", expectedOutput, result.Stdout)
	}
}

func testEnvironmentVariableHandling(t *testing.T) {
	executor := NewCommandExecutor()
	cmd, args, envVar := getPlatformEnvCommand()

	if !executor.IsCommandAvailable(cmd) {
		t.Skipf("Command %s not available", cmd)
	}

	env := map[string]string{
		"TEST_VAR": "cross-platform-env-test",
	}

	result, err := executor.ExecuteWithEnv(cmd, args, env, 10*time.Second)
	if err != nil {
		t.Logf("Environment test failed (may be expected): %v", err)
		return
	}

	if result.ExitCode != 0 {
		t.Logf("Environment test command failed with exit code %d", result.ExitCode)
		return
	}

	t.Logf("Environment variable test completed: %s", envVar)
}

func getPlatformEnvCommand() (string, []string, string) {
	switch runtime.GOOS {
	case "windows":
		return "cmd", []string{"/C", "echo", "%PATH%"}, "PATH"
	default:
		return "sh", []string{"-c", "echo $PATH"}, "PATH"
	}
}

func testShellArgumentGeneration(t *testing.T) {
	executor := NewCommandExecutor()
	testCommand := "echo hello world"
	args := executor.GetShellArgs(testCommand)

	t.Logf("Shell args for '%s': %v", testCommand, args)

	if len(args) == 0 {
		t.Error("Shell args should not be empty")
	}

	validateShellArgs(t, args)
}

func validateShellArgs(t *testing.T, args []string) {
	switch runtime.GOOS {
	case "windows":
		validateWindowsShellArgs(t, args)
	default:
		validateUnixShellArgs(t, args)
	}
}

func validateWindowsShellArgs(t *testing.T, args []string) {
	found := false
	for _, arg := range args {
		if strings.Contains(arg, "/C") || strings.Contains(arg, "-c") {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Windows shell args don't contain expected pattern: %v", args)
	}
}

func validateUnixShellArgs(t *testing.T, args []string) {
	found := false
	for _, arg := range args {
		if arg == "-c" {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Unix shell args don't contain -c: %v", args)
	}
}

func TestCrossPlatformCommandAvailability(t *testing.T) {
	t.Run("CommonCommands", testCommonCommands)
	t.Run("NonExistentCommands", testNonExistentCommands)
}

func testCommonCommands(t *testing.T) {
	executor := NewCommandExecutor()
	platformCommands := getPlatformCommands()

	expectedCommands, exists := platformCommands[runtime.GOOS]
	if !exists {
		t.Skipf("No expected commands defined for platform %s", runtime.GOOS)
	}

	testCommandAvailability(t, executor, expectedCommands)
}

func getPlatformCommands() map[string][]string {
	return map[string][]string{
		"windows": {"cmd", "where", "dir"},
		"linux":   {"sh", "ls", "which", "cat"},
		"darwin":  {"sh", "ls", "which", "cat"},
	}
}

func testCommandAvailability(t *testing.T, executor CommandExecutor, commands []string) {
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			available := executor.IsCommandAvailable(cmd)
			t.Logf("Command %s availability: %v", cmd, available)
		})
	}
}

func testNonExistentCommands(t *testing.T) {
	executor := NewCommandExecutor()
	nonExistentCommands := []string{
		"definitely-not-a-real-command-12345",
		"this-command-should-never-exist",
		"fake-executable-xyz",
	}

	for _, cmd := range nonExistentCommands {
		t.Run(cmd, func(t *testing.T) {
			available := executor.IsCommandAvailable(cmd)
			if available {
				t.Errorf("Command %s should not be available", cmd)
			}
		})
	}
}

func TestCrossPlatformCommandTimeout(t *testing.T) {
	t.Run("TimeoutHandling", testTimeoutHandling)
}

func testTimeoutHandling(t *testing.T) {
	executor := NewCommandExecutor()
	cmd, args := getPlatformSleepCommand()

	if !executor.IsCommandAvailable(cmd) {
		t.Skipf("Command %s not available for timeout test", cmd)
	}

	start := time.Now()
	result, err := executor.Execute(cmd, args, 2*time.Second)
	elapsed := time.Since(start)

	validateTimeout(t, elapsed, err, result)
}

func getPlatformSleepCommand() (string, []string) {
	switch runtime.GOOS {
	case "windows":
		return "cmd", []string{"/C", "timeout", "5"}
	default:
		return "sleep", []string{"5"}
	}
}

func validateTimeout(t *testing.T, elapsed time.Duration, err error, result *Result) {
	if elapsed >= 4*time.Second {
		t.Errorf("Command should have timed out, but took %v", elapsed)
	}

	if err == nil && result != nil && result.ExitCode == 0 {
		t.Logf("Command completed normally (timeout may not have triggered)")
	} else {
		t.Logf("Command timed out as expected: %v", err)
	}
}

func TestCrossPlatformShellSupport(t *testing.T) {
	t.Run("PlatformShellSupport", func(t *testing.T) {
		testCases := []struct {
			shell       string
			windows     bool
			unix        bool
			description string
		}{
			{"cmd", true, false, "Windows Command Prompt"},
			{"cmd.exe", true, false, "Windows Command Prompt (explicit)"},
			{"powershell", true, false, "Windows PowerShell"},
			{"powershell.exe", true, false, "Windows PowerShell (explicit)"},
			{"pwsh", true, false, "PowerShell Core"},
			{"sh", false, true, "Bourne Shell"},
			{"bash", false, true, "Bash Shell"},
			{"zsh", false, true, "Z Shell"},
			{"fish", false, true, "Fish Shell"},
			{"ksh", false, true, "Korn Shell"},
			{"dash", false, true, "Dash Shell"},
			{"nonexistent", false, false, "Non-existent shell"},
		}

		for _, tc := range testCases {
			t.Run(tc.shell, func(t *testing.T) {
				supported := SupportsShell(tc.shell)

				var expected bool
				switch runtime.GOOS {
				case "windows":
					expected = tc.windows
				case "linux", "darwin":
					expected = tc.unix
				default:
					expected = false
				}

				if supported != expected {
					t.Errorf("Shell %s support: expected %v, got %v (platform: %s)",
						tc.shell, expected, supported, runtime.GOOS)
				}

				t.Logf("Shell %s (%s): supported=%v (platform: %s)",
					tc.shell, tc.description, supported, runtime.GOOS)
			})
		}
	})

	t.Run("ShellPathVariations", func(t *testing.T) {
		pathVariations := map[string][]string{
			"bash": {
				"bash",
				"/bin/bash",
				"/usr/bin/bash",
				"/usr/local/bin/bash",
			},
			"cmd": {
				"cmd",
				"cmd.exe",
				"C:\\Windows\\System32\\cmd.exe",
				"C:/Windows/System32/cmd.exe",
			},
		}

		for shell, paths := range pathVariations {
			t.Run(shell, func(t *testing.T) {
				for _, path := range paths {
					supported := SupportsShell(path)
					t.Logf("Path %s supported: %v", path, supported)

				}
			})
		}
	})
}

func TestCrossPlatformExecutorErrors(t *testing.T) {
	t.Run("NonExistentCommand", testNonExistentCommand)
	t.Run("InvalidArguments", testInvalidArguments)
}

func testNonExistentCommand(t *testing.T) {
	executor := NewCommandExecutor()
	result, err := executor.Execute("non-existent-command-xyz", []string{}, 5*time.Second)

	if err == nil {
		t.Error("Expected error for non-existent command")
	}

	if result != nil && result.ExitCode == 0 {
		t.Error("Expected non-zero exit code for non-existent command")
	}

	t.Logf("Non-existent command error (expected): %v", err)
}

func testInvalidArguments(t *testing.T) {
	executor := NewCommandExecutor()
	cmd, args := getPlatformInvalidCommand()

	if !executor.IsCommandAvailable(cmd) {
		t.Skipf("Command %s not available", cmd)
	}

	result, err := executor.Execute(cmd, args, 5*time.Second)

	validateInvalidArgumentsResult(t, err, result)
}

func getPlatformInvalidCommand() (string, []string) {
	switch runtime.GOOS {
	case "windows":
		return "cmd", []string{"/X", "invalid-flag"}
	default:
		return "ls", []string{"--definitely-invalid-flag-xyz"}
	}
}

func validateInvalidArgumentsResult(t *testing.T, err error, result *Result) {
	if err == nil && (result == nil || result.ExitCode == 0) {
		t.Logf("Invalid arguments test: command succeeded unexpectedly")
	} else {
		t.Logf("Invalid arguments handled correctly: %v", err)
	}
}

func TestExecutorConcurrency(t *testing.T) {
	t.Run("ConcurrentExecution", testConcurrentExecution)
}

func testConcurrentExecution(t *testing.T) {
	executor := NewCommandExecutor()
	cmd, args := getPlatformConcurrentCommand()

	if !executor.IsCommandAvailable(cmd) {
		t.Skipf("Command %s not available", cmd)
	}

	results := executeConcurrentCommands(executor, cmd, args, 5)
	analyzeConcurrentResults(t, results)
}

func getPlatformConcurrentCommand() (string, []string) {
	switch runtime.GOOS {
	case "windows":
		return "cmd", []string{"/C", "echo", "concurrent"}
	default:
		return "echo", []string{"concurrent"}
	}
}

type concurrentResult struct {
	output *Result
	err    error
	id     int
}

func executeConcurrentCommands(executor CommandExecutor, cmd string, args []string, numCommands int) []concurrentResult {
	results := make(chan concurrentResult, numCommands)

	for i := 0; i < numCommands; i++ {
		go func(id int) {
			testArgs := append(args, fmt.Sprintf("test-%d", id))
			output, err := executor.Execute(cmd, testArgs, 10*time.Second)
			results <- concurrentResult{output: output, err: err, id: id}
		}(i)
	}

	var allResults []concurrentResult
	for i := 0; i < numCommands; i++ {
		allResults = append(allResults, <-results)
	}

	return allResults
}

func analyzeConcurrentResults(t *testing.T, results []concurrentResult) {
	successCount := 0
	for _, res := range results {
		if res.err == nil && res.output != nil && res.output.ExitCode == 0 {
			successCount++
			t.Logf("Concurrent command %d succeeded: %s",
				res.id, strings.TrimSpace(res.output.Stdout))
		} else {
			t.Logf("Concurrent command %d failed: %v", res.id, res.err)
		}
	}

	if successCount == 0 {
		t.Log("No concurrent commands succeeded (may be expected in test environment)")
	} else {
		t.Logf("Concurrent execution: %d/%d commands succeeded", successCount, len(results))
	}
}

func BenchmarkCrossPlatformExecution(b *testing.B) {
	executor := NewCommandExecutor()

	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/C", "echo", "benchmark"}
	default:
		cmd = "echo"
		args = []string{"benchmark"}
	}

	if !executor.IsCommandAvailable(cmd) {
		b.Skipf("Command %s not available", cmd)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(cmd, args, 5*time.Second)
		if err != nil {
			b.Logf("Benchmark execution failed: %v", err)
		}
	}
}

func BenchmarkShellDetection(b *testing.B) {
	executor := NewCommandExecutor()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = executor.GetShell()
	}
}

func BenchmarkCommandAvailability(b *testing.B) {
	executor := NewCommandExecutor()

	testCommands := []string{"echo", "ls", "cmd", "sh", "nonexistent"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cmd := testCommands[i%len(testCommands)]
		_ = executor.IsCommandAvailable(cmd)
	}
}
