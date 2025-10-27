package platform

import "runtime"

const (
	OSLinux   = "linux"
	OSDarwin  = "darwin"
	OSWindows = "windows"
)

// Current returns the current GOOS value.
func Current() string {
	return runtime.GOOS
}

// IsWindows reports whether the current operating system is Windows.
func IsWindows() bool {
	return runtime.GOOS == OSWindows
}

// IsLinux reports whether the current operating system is Linux.
func IsLinux() bool {
	return runtime.GOOS == OSLinux
}

// IsDarwin reports whether the current operating system is macOS.
func IsDarwin() bool {
	return runtime.GOOS == OSDarwin
}
