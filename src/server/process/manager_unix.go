//go:build !windows
// +build !windows

package process

import (
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
)

// StopProcess terminates an LSP server process gracefully on Unix-like systems
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

	// Send shutdown and exit notifications with timeout
	if sender != nil {
		pm.sendShutdown(sender)
	}

	// Mark as inactive after shutdown sequence
	info.Active = false

	// Wait for graceful shutdown with proper timeout
	// Only wait if the process hasn't already exited (avoid duplicate Wait() calls)
	if info.Cmd != nil && info.Cmd.Process != nil && !info.ProcessExited {
		// Create channel to track process completion
		done := make(chan error, 1)
		go func() {
			// Check if process already exited
			if info.ProcessExited {
				done <- nil
				return
			}
			done <- info.Cmd.Wait()
		}()

		// Wait up to ProcessShutdownTimeout for graceful exit
		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(constants.ProcessShutdownTimeout):
			// Timeout - force kill only after waiting
			// Debug level for intentional stops, warn for unexpected
			common.LSPLogger.Debug("LSP server %s did not exit within %v, force killing", info.Language, constants.ProcessShutdownTimeout)
			if err := info.Cmd.Process.Kill(); err != nil {
				// Ignore errors if process already exited
				if !strings.Contains(err.Error(), "process already finished") &&
					!strings.Contains(err.Error(), "no child processes") &&
					!strings.Contains(err.Error(), "no such process") {
					common.LSPLogger.Debug("Failed to kill LSP server %s: %v", info.Language, err)
				}
			}
			// Wait for process to actually die
			<-done
		}
	}

	// Close pipes
	pm.CleanupProcess(info)

	return nil
}

// sendShutdown sends shutdown sequence to LSP server through the ShutdownSender
func (pm *LSPProcessManager) sendShutdown(sender ShutdownSender) {
	// Send shutdown request with its own timeout
	shutdownCtx, shutdownCancel := common.CreateContext(2 * time.Second)
	defer shutdownCancel()

	sender.SendShutdownRequest(shutdownCtx)

	// Send exit notification with its own timeout
	exitCtx, exitCancel := common.CreateContext(1 * time.Second)
	defer exitCancel()

	sender.SendExitNotification(exitCtx)
}
