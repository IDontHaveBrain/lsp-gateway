package constants

import (
	"runtime"
	"time"
)

// GetWorkspaceFolderSyncDelay returns an OS/language-specific delay for workspace folder sync
func GetWorkspaceFolderSyncDelay(language string) time.Duration {
	if runtime.GOOS == "windows" {
		if language == "java" {
			return 150 * time.Millisecond
		}
		return 700 * time.Millisecond
	}
	return 150 * time.Millisecond
}

// GetDocumentAnalysisDelay returns an OS/language-specific delay after didOpen
func GetDocumentAnalysisDelay(language string) time.Duration {
	if runtime.GOOS == "windows" {
		if language == "java" {
			return 200 * time.Millisecond
		}
		return 1200 * time.Millisecond
	}
	return 400 * time.Millisecond
}

// GetBackgroundIndexingDelay returns an initial delay before background indexing to allow servers to settle
func GetBackgroundIndexingDelay() time.Duration {
	if runtime.GOOS == "windows" {
		if isCI() || isWindowsCI() {
			return 20 * time.Second
		}
		return 12 * time.Second
	}
	return 3 * time.Second
}

// AdjustDurationForWindows multiplies a duration when running on Windows
func AdjustDurationForWindows(base time.Duration, multiplier float64) time.Duration {
	if runtime.GOOS == "windows" {
		return time.Duration(float64(base) * multiplier)
	}
	return base
}
