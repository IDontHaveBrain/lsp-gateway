//go:build windows
// +build windows

package process

import (
	"fmt"
	"os"
	"time"

	"lsp-gateway/src/internal/common"
)

// StopProcess terminates an LSP server process on Windows
// Windows doesn't support graceful shutdown signals to other processes,
// so we send the LSP protocol shutdown/exit and then force kill
func (pm *LSPProcessManager) StopProcess(info *ProcessInfo, sender ShutdownSender) error {
	if info == nil {
		return nil
	}

	// Mark this as an intentional stop
	info.IntentionalStop = true

	// Close stop channel to signal goroutines to stop
	select {
	case <-info.StopCh:
		// Already closed
	default:
		close(info.StopCh)
	}

	// Send shutdown and exit notifications
	if sender != nil {
		// Send shutdown request with timeout
		shutdownCtx, shutdownCancel := common.CreateContext(2 * time.Second)
		sender.SendShutdownRequest(shutdownCtx)
		shutdownCancel()

		// Send exit notification with timeout
		exitCtx, exitCancel := common.CreateContext(1 * time.Second)
		sender.SendExitNotification(exitCtx)
		exitCancel()
	}

	// Mark as inactive after shutdown sequence
	info.Active = false

	// On Windows, we can't send signals to other processes, so we kill immediately
	// Only kill if the process hasn't already exited (avoid duplicate operations)
	if info.Cmd != nil && info.Cmd.Process != nil && !info.ProcessExited {
		// Kill the process immediately on Windows
		if err := info.Cmd.Process.Kill(); err != nil {
			// If kill fails, the process might already be dead
			common.LSPLogger.Debug("Process kill for %s returned: %v", info.Language, err)
		}

		// Wait for the process to actually exit
		done := make(chan error, 1)
		go func() {
			// Check if process already exited
			if info.ProcessExited {
				done <- nil
				return
			}
			done <- info.Cmd.Wait()
		}()

		// Give it a moment to die
		select {
		case <-done:
			// Process exited
		case <-time.After(2 * time.Second):
			// Process still not dead after kill, log but continue (debug level for tests)
			common.LSPLogger.Debug("LSP server %s process did not terminate after kill", info.Language)
		}
	}

	// Close pipes
	pm.CleanupProcess(info)

	return nil
}

// killProcess forcefully terminates a Windows process
func killProcess(pid int) error {
	// On Windows, Process.Kill() should work
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}
	return proc.Kill()
}
