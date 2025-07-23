package platform_test

import (
	"context"
	"fmt"
	"lsp-gateway/internal/platform"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Test command execution error scenarios
func TestCommandExecutionErrorsSimple(t *testing.T) {
	executor := platform.NewCommandExecutor()

	// Test non-existent command
	t.Run("Non-existent command", func(t *testing.T) {
		_, err := executor.Execute("non-existent-command-xyz", []string{}, 5*time.Second)
		if err == nil {
			t.Error("Expected error for non-existent command")
		}
	})

	// Test command timeout
	t.Run("Command timeout", func(t *testing.T) {
		var cmd string
		var args []string
		if platform.IsWindows() {
			cmd = "ping"
			args = []string{"127.0.0.1", "-n", "10"}
		} else {
			cmd = "sleep"
			args = []string{"10"}
		}

		start := time.Now()
		_, err := executor.Execute(cmd, args, 100*time.Millisecond)
		duration := time.Since(start)

		if err == nil {
			t.Error("Expected timeout error")
		}
		if duration > 2*time.Second {
			t.Error("Command should have timed out quickly")
		}
	})

	// Test command with non-zero exit code
	t.Run("Non-zero exit code", func(t *testing.T) {
		var cmd string
		var args []string
		if platform.IsWindows() {
			cmd = "cmd"
			args = []string{"/c", "exit 1"}
		} else {
			cmd = "sh"
			args = []string{"-c", "exit 1"}
		}

		result, err := executor.Execute(cmd, args, 5*time.Second)
		// Either error is returned or exit code is captured in result
		if err == nil {
			if result.ExitCode != 1 {
				t.Errorf("Expected exit code 1, got: %d", result.ExitCode)
			}
		} else {
			// If error is returned, check that we can extract exit code
			exitCode := platform.GetExitCodeFromError(err)
			if exitCode != 1 {
				t.Errorf("Expected exit code 1 from error, got: %d", exitCode)
			}
		}
	})
}

// Test utility functions
func TestUtilityFunctionsSimple(t *testing.T) {
	// Test isCommandAvailable function - skipped as it tests unexported functions
	t.Run("isCommandAvailable", func(t *testing.T) {
		t.Skip("isCommandAvailable is an unexported function that cannot be tested from external packages")
	})

	// Test IsCommandAvailable global function
	t.Run("IsCommandAvailable global", func(t *testing.T) {
		if !platform.IsCommandAvailable("echo") {
			t.Error("echo should be available via global function")
		}

		if platform.IsCommandAvailable("fake-command-12345") {
			t.Error("Fake command should not be available via global function")
		}
	})

	// Test GetExitCodeFromError function
	t.Run("GetExitCodeFromError", func(t *testing.T) {
		// Test with exec.ExitError
		cmd := exec.Command("sh", "-c", "exit 42")
		err := cmd.Run()
		if err != nil {
			exitCode := platform.GetExitCodeFromError(err)
			if exitCode != 42 {
				t.Errorf("Expected exit code 42, got: %d", exitCode)
			}
		}

		// Test with non-exit error
		normalErr := fmt.Errorf("normal error")
		exitCode := platform.GetExitCodeFromError(normalErr)
		if exitCode != -1 {
			t.Errorf("Expected exit code -1 for non-exit error, got: %d", exitCode)
		}

		// Test with nil error
		exitCode = platform.GetExitCodeFromError(nil)
		if exitCode != 0 {
			t.Errorf("Expected exit code 0 for nil error, got: %d", exitCode)
		}
	})

	// Test GetCommandPath function
	t.Run("GetCommandPath", func(t *testing.T) {
		// Test with echo command
		path, err := platform.GetCommandPath("echo")
		if err != nil && path == "" {
			t.Error("echo should have a path or no error")
		}

		// Test with non-existent command
		_, err = platform.GetCommandPath("non-existent-command-xyz")
		if err == nil {
			t.Error("Non-existent command should return error")
		}
	})

	// Test ExecuteShellCommandWithContext function
	t.Run("ExecuteShellCommandWithContext", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		executor := platform.NewCommandExecutor()

		result, err := platform.ExecuteShellCommandWithContext(executor, ctx, "echo context_test")
		if err != nil {
			t.Errorf("ExecuteShellCommandWithContext failed: %v", err)
		}
		if !strings.Contains(result.Stdout, "context_test") {
			t.Errorf("Expected 'context_test' in output, got: %s", result.Stdout)
		}
	})

	// Test ExecuteShellCommandWithContext timeout
	t.Run("ExecuteShellCommandWithContext timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		executor := platform.NewCommandExecutor()

		var command string
		if platform.IsWindows() {
			command = "ping 127.0.0.1 -n 10"
		} else {
			command = "sleep 5"
		}

		start := time.Now()
		_, err := platform.ExecuteShellCommandWithContext(executor, ctx, command)
		duration := time.Since(start)

		if err == nil {
			t.Error("Expected context timeout error")
		}
		if duration > 2*time.Second {
			t.Error("Context timeout should happen quickly")
		}
	})
}

// Test environment variable handling
func TestEnvironmentVariableHandlingSimple(t *testing.T) {
	executor := platform.NewCommandExecutor()

	// Test multiple environment variables
	t.Run("Multiple environment variables", func(t *testing.T) {
		env := map[string]string{
			"VAR1": "value1",
			"VAR2": "value2",
			"VAR3": "value with spaces",
		}

		var cmd string
		var args []string
		if platform.IsWindows() {
			cmd = "cmd"
			args = []string{"/c", "echo %VAR1%-%VAR2%-%VAR3%"}
		} else {
			cmd = "sh"
			args = []string{"-c", "echo $VAR1-$VAR2-$VAR3"}
		}

		result, err := executor.ExecuteWithEnv(cmd, args, env, 5*time.Second)
		if err != nil {
			t.Errorf("ExecuteWithEnv with multiple vars failed: %v", err)
		}

		output := strings.TrimSpace(result.Stdout)
		if !strings.Contains(output, "value1") {
			t.Errorf("Output should contain VAR1 value, got: %s", output)
		}
		if !strings.Contains(output, "value2") {
			t.Errorf("Output should contain VAR2 value, got: %s", output)
		}
		if !strings.Contains(output, "value with spaces") {
			t.Errorf("Output should contain VAR3 value, got: %s", output)
		}
	})

	// Test environment variable override
	t.Run("Environment variable override", func(t *testing.T) {
		// Set a known environment variable to a different value
		env := map[string]string{
			"PATH": "/custom/path",
		}

		var cmd string
		var args []string
		if platform.IsWindows() {
			cmd = "cmd"
			args = []string{"/c", "echo %PATH%"}
		} else {
			cmd = "sh"
			args = []string{"-c", "echo $PATH"}
		}

		result, err := executor.ExecuteWithEnv(cmd, args, env, 5*time.Second)
		if err != nil {
			t.Errorf("ExecuteWithEnv with override failed: %v", err)
		}

		if !strings.Contains(result.Stdout, "/custom/path") {
			t.Errorf("Expected custom PATH in output, got: %s", result.Stdout)
		}
	})

	// Test empty environment
	t.Run("Empty environment", func(t *testing.T) {
		env := map[string]string{}

		result, err := executor.ExecuteWithEnv("echo", []string{"empty_env"}, env, 5*time.Second)
		if err != nil {
			t.Errorf("ExecuteWithEnv with empty env failed: %v", err)
		}
		if !strings.Contains(result.Stdout, "empty_env") {
			t.Errorf("Expected 'empty_env' in output, got: %s", result.Stdout)
		}
	})
}

// Test concurrent command execution
func TestConcurrentCommandExecutionSimple(t *testing.T) {
	executor := platform.NewCommandExecutor()
	const numConcurrent = 5

	// Test concurrent execution safety
	t.Run("Concurrent execution safety", func(t *testing.T) {
		results := make(chan error, numConcurrent)

		for i := 0; i < numConcurrent; i++ {
			go func(id int) {
				_, err := executor.Execute("echo", []string{fmt.Sprintf("concurrent_%d", id)}, 5*time.Second)
				results <- err
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numConcurrent; i++ {
			err := <-results
			if err != nil {
				t.Errorf("Concurrent execution %d failed: %v", i, err)
			}
		}
	})

	// Test concurrent execution with different commands
	t.Run("Concurrent different commands", func(t *testing.T) {
		commands := []struct {
			cmd  string
			args []string
		}{
			{"echo", []string{"test1"}},
			{"echo", []string{"test2"}},
			{"echo", []string{"test3"}},
		}

		results := make(chan error, len(commands))

		for i, cmdArgs := range commands {
			go func(cmd string, args []string, id int) {
				_, err := executor.Execute(cmd, args, 5*time.Second)
				results <- err
			}(cmdArgs.cmd, cmdArgs.args, i)
		}

		// Wait for all commands to complete
		for i := 0; i < len(commands); i++ {
			err := <-results
			if err != nil {
				t.Errorf("Concurrent command %d failed: %v", i, err)
			}
		}
	})
}

// Test shell detection and arguments
func TestShellDetectionAndArgsSimple(t *testing.T) {
	executor := platform.NewCommandExecutor()

	// Test shell detection based on platform
	t.Run("Shell detection", func(t *testing.T) {
		shell := executor.GetShell()

		if platform.IsWindows() {
			expectedShells := []string{"cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh", "pwsh.exe"}
			found := false
			for _, expected := range expectedShells {
				if strings.Contains(strings.ToLower(shell), expected) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected Windows shell, got: %s", shell)
			}
		} else {
			expectedShells := []string{"sh", "bash", "zsh", "dash", "ksh", "fish"}
			found := false
			for _, expected := range expectedShells {
				if strings.Contains(strings.ToLower(shell), expected) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected Unix shell, got: %s", shell)
			}
		}
	})

	// Test shell argument generation
	t.Run("Shell argument generation", func(t *testing.T) {
		testCommand := "echo 'test command'"
		args := executor.GetShellArgs(testCommand)

		if len(args) == 0 {
			t.Error("Shell args should not be empty")
		}

		// Verify command is included in args
		argsStr := strings.Join(args, " ")
		if !strings.Contains(argsStr, "echo") {
			t.Errorf("Shell args should contain command, got: %v", args)
		}

		if platform.IsWindows() {
			// Windows should use /c or similar
			if !strings.Contains(argsStr, "/c") && !strings.Contains(argsStr, "-Command") {
				t.Errorf("Windows shell args should contain execution flag, got: %v", args)
			}
		} else {
			// Unix should use -c
			if !strings.Contains(argsStr, "-c") {
				t.Errorf("Unix shell args should contain -c flag, got: %v", args)
			}
		}
	})
}
