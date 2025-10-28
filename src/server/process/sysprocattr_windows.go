//go:build windows
// +build windows

package process

import "os/exec"

// setProcessAttributes configures process-level settings for Windows systems.
// Windows lacks a direct equivalent to Unix process groups, so this is a no-op.
func setProcessAttributes(cmd *exec.Cmd) {
	// Intentionally empty: no additional attributes required on Windows.
	_ = cmd
}
