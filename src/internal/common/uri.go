package common

import (
	"runtime"
	"strings"
)

func URIToFilePath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return uri
	}

	// Remove the file:// prefix
	path := strings.TrimPrefix(uri, "file://")

	// On Windows, file URIs look like file:///C:/path/to/file
	// After removing file://, we have /C:/path/to/file
	// We need to remove the leading slash for Windows absolute paths
	if runtime.GOOS == "windows" && len(path) > 2 {
		// Check if this looks like a Windows absolute path (e.g., /C:/)
		if path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
	}

	return path
}
