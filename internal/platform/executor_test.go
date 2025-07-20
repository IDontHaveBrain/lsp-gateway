package platform

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewCommandExecutor(t *testing.T) {
	executor := NewCommandExecutor()
	if executor == nil {
		t.Fatal("NewCommandExecutor returned nil")
	}

	if runtime.GOOS == string(PlatformWindows) {
		if _, ok := executor.(*windowsExecutor); !ok {
			t.Errorf("Expected windowsExecutor on Windows, got %T", executor)
		}
	} else {
		if _, ok := executor.(*unixExecutor); !ok {
			t.Errorf("Expected unixExecutor on Unix-like systems, got %T", executor)
		}
	}
}

func TestExecuteBasicCommand(t *testing.T) {
	executor := NewCommandExecutor()

	var cmd string
	var args []string

	if runtime.GOOS == string(PlatformWindows) {
		cmd = "cmd"
		args = []string{"/C", "echo", "test"}
	} else {
		cmd = "echo"
		args = []string{"test"}
	}

	result, err := executor.Execute(cmd, args, 5*time.Second)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	expectedOutput := "test"
	if !strings.Contains(strings.TrimSpace(result.Stdout), expectedOutput) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedOutput, result.Stdout)
	}

	if result.Duration <= 0 {
		t.Errorf("Expected positive duration, got %v", result.Duration)
	}
}

func TestExecuteWithEnvironment(t *testing.T) {
	executor := NewCommandExecutor()

	env := map[string]string{
		"TEST_VAR": "test_value",
	}

	var cmd string
	var args []string

	if runtime.GOOS == string(PlatformWindows) {
		cmd = "cmd"
		args = []string{"/C", "echo", "%TEST_VAR%"}
	} else {
		cmd = "sh"
		args = []string{"-c", "echo $TEST_VAR"}
	}

	result, err := executor.ExecuteWithEnv(cmd, args, env, 5*time.Second)
	if err != nil {
		t.Fatalf("ExecuteWithEnv failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	expectedOutput := "test_value"
	if !strings.Contains(strings.TrimSpace(result.Stdout), expectedOutput) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedOutput, result.Stdout)
	}
}

func TestGetShell(t *testing.T) {
	executor := NewCommandExecutor()
	shell := executor.GetShell()

	if shell == "" {
		t.Error("GetShell returned empty string")
	}

	if !IsCommandAvailable(shell) {
		t.Errorf("Shell '%s' is not available in PATH", shell)
	}
}

func TestGetShellArgs(t *testing.T) {
	executor := NewCommandExecutor()
	command := "echo test"
	args := executor.GetShellArgs(command)

	if len(args) == 0 {
		t.Error("GetShellArgs returned empty args")
	}

	if runtime.GOOS == string(PlatformWindows) {
		hasValidFlag := false
		for _, arg := range args {
			if arg == "/C" || arg == "-Command" {
				hasValidFlag = true
				break
			}
		}
		if !hasValidFlag {
			t.Errorf("Expected Windows shell args to contain /C or -Command, got %v", args)
		}
	} else {
		hasC := false
		for _, arg := range args {
			if arg == "-c" {
				hasC = true
				break
			}
		}
		if !hasC {
			t.Errorf("Expected Unix shell args to contain -c, got %v", args)
		}
	}
}

func TestIsCommandAvailable(t *testing.T) {
	var knownCommand string
	if runtime.GOOS == string(PlatformWindows) {
		knownCommand = "cmd"
	} else {
		knownCommand = "sh"
	}

	if !IsCommandAvailable(knownCommand) {
		t.Errorf("Expected '%s' to be available", knownCommand)
	}

	if IsCommandAvailable("nonexistent_command_xyz123") {
		t.Error("Expected nonexistent command to not be available")
	}
}

func TestExecutorIsCommandAvailable(t *testing.T) {
	executor := NewCommandExecutor()

	var knownCommand string
	if runtime.GOOS == string(PlatformWindows) {
		knownCommand = "cmd"
	} else {
		knownCommand = "sh"
	}

	if !executor.IsCommandAvailable(knownCommand) {
		t.Errorf("Expected '%s' to be available", knownCommand)
	}

	if executor.IsCommandAvailable("nonexistent_command_xyz123") {
		t.Error("Expected nonexistent command to not be available")
	}
}

func TestExecuteShellCommand(t *testing.T) {
	executor := NewCommandExecutor()
	command := "echo shell_test"

	result, err := ExecuteShellCommand(executor, command, 5*time.Second)
	if err != nil {
		t.Fatalf("ExecuteShellCommand failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	expectedOutput := "shell_test"
	if !strings.Contains(strings.TrimSpace(result.Stdout), expectedOutput) {
		t.Errorf("Expected output to contain '%s', got '%s'", expectedOutput, result.Stdout)
	}
}

func TestExecuteTimeout(t *testing.T) {
	executor := NewCommandExecutor()

	var cmd string
	var args []string

	if runtime.GOOS == string(PlatformWindows) {
		cmd = "cmd"
		args = []string{"/C", "ping", "127.0.0.1", "-n", "10"}
	} else {
		cmd = "sleep"
		args = []string{"5"}
	}

	start := time.Now()
	result, err := executor.Execute(cmd, args, 100*time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error, but got none")
	}

	if result == nil {
		t.Fatal("Result should not be nil even on timeout")
	}

	if result.ExitCode != -1 {
		t.Errorf("Expected exit code -1 for timeout, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stderr, "timed out") && !strings.Contains(err.Error(), "timed out") && !strings.Contains(err.Error(), "killed") {
		t.Errorf("Expected stderr or error to contain 'timed out' or 'killed', got stderr: '%s', error: '%v'", result.Stderr, err)
	}

	if duration > 2*time.Second {
		t.Errorf("Command took too long to timeout: %v", duration)
	}
}

func TestConcurrentExecution(t *testing.T) {
	executor := NewCommandExecutor()

	var cmd string
	var args []string

	if runtime.GOOS == string(PlatformWindows) {
		cmd = "cmd"
		args = []string{"/C", "echo", "concurrent"}
	} else {
		cmd = "echo"
		args = []string{"concurrent"}
	}

	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()

			result, err := executor.Execute(cmd, args, 5*time.Second)
			if err != nil {
				t.Errorf("Concurrent execution %d failed: %v", id, err)
				return
			}

			if result.ExitCode != 0 {
				t.Errorf("Concurrent execution %d: expected exit code 0, got %d", id, result.ExitCode)
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}
