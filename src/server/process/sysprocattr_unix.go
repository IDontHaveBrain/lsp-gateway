//go:build !windows
// +build !windows

package process

import (
	"os/exec"
	"syscall"
)

// setProcessAttributes configures process-level settings for Unix-like systems.
// We ensure the LSP server runs in its own process group so that when we stop
// the server we can reliably terminate child processes spawned by wrapper scripts.
func setProcessAttributes(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}
