package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
)

// ProcessInfo holds information about a running LSP server process
type ProcessInfo struct {
	Cmd               *exec.Cmd
	Stdin             io.WriteCloser
	Stdout            io.ReadCloser
	Stderr            io.ReadCloser
	StopCh            chan struct{}
	Active            bool
	Language          string
	IntentionalStop   bool // Flag to indicate intentional shutdown
	ProcessExited     bool // Flag to indicate process has already exited
}

// ShutdownSender interface for sending LSP shutdown messages
type ShutdownSender interface {
	SendShutdownRequest(ctx context.Context) error
	SendExitNotification(ctx context.Context) error
}

// ProcessManager interface for LSP server process lifecycle management
type ProcessManager interface {
	StartProcess(config types.ClientConfig, language string) (*ProcessInfo, error)
	StopProcess(info *ProcessInfo, sender ShutdownSender) error
	MonitorProcess(info *ProcessInfo, onExit func(error))
	CleanupProcess(info *ProcessInfo)
}

// LSPProcessManager implements ProcessManager for LSP server processes
type LSPProcessManager struct{}

// NewLSPProcessManager creates a new LSP process manager
func NewLSPProcessManager() *LSPProcessManager {
	return &LSPProcessManager{}
}

// StartProcess initializes and starts an LSP server process
func (pm *LSPProcessManager) StartProcess(config types.ClientConfig, language string) (*ProcessInfo, error) {
	// Create command
	cmd := exec.Command(config.Command, config.Args...)

	// Use current working directory, but fallback to /tmp if needed
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	} else {
		cmd.Dir = "/tmp"
	}

	// Create process info
	info := &ProcessInfo{
		Cmd:      cmd,
		StopCh:   make(chan struct{}),
		Active:   false,
		Language: language,
	}

	// Setup pipes
	var err error
	info.Stdin, err = cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	info.Stdout, err = cmd.StdoutPipe()
	if err != nil {
		info.Stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	info.Stderr, err = cmd.StderrPipe()
	if err != nil {
		info.Stdin.Close()
		info.Stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start process
	if err := cmd.Start(); err != nil {
		pm.CleanupProcess(info)
		return nil, fmt.Errorf("failed to start LSP server: %w", err)
	}

	common.LSPLogger.Info("Started LSP server process for %s: PID %d", language, cmd.Process.Pid)
	return info, nil
}

// MonitorProcess watches an LSP server process and reports when it exits
func (pm *LSPProcessManager) MonitorProcess(info *ProcessInfo, onExit func(error)) {
	if info == nil || info.Cmd == nil || info.Cmd.Process == nil {
		common.LSPLogger.Error("monitorProcess called with nil process info or command")
		if onExit != nil {
			onExit(fmt.Errorf("invalid process info"))
		}
		return
	}

	// Wait for process to finish
	err := info.Cmd.Wait()
	
	// Mark process as exited
	info.ProcessExited = true

	// Check if client was active when process exited (helps identify crashes vs normal shutdown)
	wasActive := info.Active

	// Enhanced error reporting based on process state
	// Only log errors if this wasn't an intentional stop
	if !info.IntentionalStop {
		if err != nil {
			errStr := err.Error()
			// Filter out common shutdown errors on all platforms
			isCommonShutdownError := strings.Contains(errStr, "signal: killed") ||
				strings.Contains(errStr, "waitid: no child processes") ||
				strings.Contains(errStr, "process already finished") ||
				strings.Contains(errStr, "exit status 1") ||  // Common on Windows
				strings.Contains(errStr, "exit status 0xc000013a")  // Windows CTRL_C_EVENT
				
			if !isCommonShutdownError {
				if wasActive {
					common.LSPLogger.Error("LSP server %s crashed unexpectedly: %v", info.Language, err)
				} else {
					common.LSPLogger.Warn("LSP server %s failed to start: %v", info.Language, err)
				}
			}
		} else {
			if wasActive {
				common.LSPLogger.Info("LSP server %s exited normally", info.Language)
			} else {
				common.LSPLogger.Info("LSP server %s stopped during initialization", info.Language)
			}
		}
	}

	// Signal stop to other goroutines
	select {
	case <-info.StopCh:
		// Already stopped
	default:
		close(info.StopCh)
	}

	// Notify caller of process exit
	if onExit != nil {
		onExit(err)
	}
}

// CleanupProcess closes all pipes and resources
func (pm *LSPProcessManager) CleanupProcess(info *ProcessInfo) {
	if info == nil {
		return
	}

	if info.Stdin != nil {
		info.Stdin.Close()
		info.Stdin = nil
	}
	if info.Stdout != nil {
		info.Stdout.Close()
		info.Stdout = nil
	}
	if info.Stderr != nil {
		info.Stderr.Close()
		info.Stderr = nil
	}
}
