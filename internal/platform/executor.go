package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type Result struct {
	ExitCode int           `json:"exit_code"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	Duration time.Duration `json:"duration"`
}

type CommandExecutor interface {
	Execute(cmd string, args []string, timeout time.Duration) (*Result, error)

	ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*Result, error)

	GetShell() string

	GetShellArgs(command string) []string

	IsCommandAvailable(command string) bool
}

type windowsExecutor struct{}

type unixExecutor struct{}

func NewCommandExecutor() CommandExecutor {
	if runtime.GOOS == string(PlatformWindows) {
		return &windowsExecutor{}
	}
	return &unixExecutor{}
}

func (w *windowsExecutor) Execute(cmd string, args []string, timeout time.Duration) (*Result, error) {
	return w.ExecuteWithEnv(cmd, args, nil, timeout)
}

func (w *windowsExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()

	execCmd := exec.CommandContext(ctx, cmd, args...)

	if env != nil {
		execCmd.Env = os.Environ()
		for key, value := range env {
			execCmd.Env = append(execCmd.Env, fmt.Sprintf(ENV_VAR_FORMAT, key, value))
		}
	}

	stdout, err := execCmd.Output()
	duration := time.Since(start)

	result := &Result{
		Stdout:   string(stdout),
		Duration: duration,
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			result.Stderr = string(exitError.Stderr)
		} else if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Stderr = fmt.Sprintf(COMMAND_TIMEOUT_MESSAGE, timeout)
			return result, fmt.Errorf(ERROR_COMMAND_TIMEOUT, err)
		} else {
			result.ExitCode = -1
			result.Stderr = err.Error()
		}
		return result, err
	}

	result.ExitCode = 0
	return result, nil
}

func (w *windowsExecutor) GetShell() string {
	if isCommandAvailable(SHELL_PWSH) {
		return SHELL_PWSH
	}
	if isCommandAvailable(SHELL_POWERSHELL) {
		return SHELL_POWERSHELL
	}
	return "cmd"
}

func (w *windowsExecutor) GetShellArgs(command string) []string {
	shell := w.GetShell()
	switch shell {
	case SHELL_PWSH, SHELL_POWERSHELL:
		return []string{"-NoProfile", "-NonInteractive", "-Command", command}
	default: // cmd
		return []string{"/C", command}
	}
}

func (w *windowsExecutor) IsCommandAvailable(command string) bool {
	return isCommandAvailable(command)
}

func (u *unixExecutor) Execute(cmd string, args []string, timeout time.Duration) (*Result, error) {
	return u.ExecuteWithEnv(cmd, args, nil, timeout)
}

func (u *unixExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()

	execCmd := exec.CommandContext(ctx, cmd, args...)

	if env != nil {
		execCmd.Env = os.Environ()
		for key, value := range env {
			execCmd.Env = append(execCmd.Env, fmt.Sprintf(ENV_VAR_FORMAT, key, value))
		}
	}

	stdout, err := execCmd.Output()
	duration := time.Since(start)

	result := &Result{
		Stdout:   string(stdout),
		Duration: duration,
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
			result.Stderr = string(exitError.Stderr)
		} else if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Stderr = fmt.Sprintf(COMMAND_TIMEOUT_MESSAGE, timeout)
			return result, fmt.Errorf(ERROR_COMMAND_TIMEOUT, err)
		} else {
			result.ExitCode = -1
			result.Stderr = err.Error()
		}
		return result, err
	}

	result.ExitCode = 0
	return result, nil
}

func (u *unixExecutor) GetShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}

	if isCommandAvailable(SHELL_BASH) {
		return SHELL_BASH
	}

	return "sh"
}

func (u *unixExecutor) GetShellArgs(command string) []string {
	shell := u.GetShell()

	if strings.HasSuffix(shell, SHELL_BASH) || strings.HasSuffix(shell, "sh") ||
		strings.HasSuffix(shell, SHELL_ZSH) || strings.HasSuffix(shell, "dash") {
		return []string{"-c", command}
	}

	return []string{"-c", command}
}

func (u *unixExecutor) IsCommandAvailable(command string) bool {
	return isCommandAvailable(command)
}

func ExecuteShellCommand(executor CommandExecutor, command string, timeout time.Duration) (*Result, error) {
	shell := executor.GetShell()
	args := executor.GetShellArgs(command)
	return executor.Execute(shell, args, timeout)
}

func isCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

func IsCommandAvailable(command string) bool {
	return isCommandAvailable(command)
}

func GetExitCodeFromError(err error) int {
	if err == nil {
		return 0
	}

	if exitError, ok := err.(*exec.ExitError); ok {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
		return exitError.ExitCode()
	}

	return -1
}

func GetCommandPath(command string) (string, error) {
	return exec.LookPath(command)
}

func ExecuteShellCommandWithContext(executor CommandExecutor, ctx context.Context, command string) (*Result, error) {
	timeout := 30 * time.Second

	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	shell := executor.GetShell()
	args := executor.GetShellArgs(command)
	return executor.Execute(shell, args, timeout)
}
