//go:build !windows
// +build !windows

package process

import (
	"context"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
)

// StopProcess terminates an LSP server process gracefully on Unix-like systems
func (pm *LSPProcessManager) StopProcess(info *ProcessInfo, sender ShutdownSender) error {
	if info == nil {
		return nil
	}

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
	if info.Cmd != nil && info.Cmd.Process != nil {
		// Create channel to track process completion
		done := make(chan error, 1)
		go func() {
			done <- info.Cmd.Wait()
		}()

		// Wait up to ProcessShutdownTimeout for graceful exit
		select {
		case <-done:
			// Process exited gracefully
		case <-time.After(constants.ProcessShutdownTimeout):
			// Timeout - force kill only after waiting
			common.LSPLogger.Warn("LSP server %s did not exit gracefully within %v, force killing", info.Language, constants.ProcessShutdownTimeout)
			if err := info.Cmd.Process.Kill(); err != nil {
				common.LSPLogger.Error("Failed to kill LSP server %s: %v", info.Language, err)
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	sender.SendShutdownRequest(shutdownCtx)

	// Send exit notification with its own timeout
	exitCtx, exitCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer exitCancel()

	sender.SendExitNotification(exitCtx)
}
