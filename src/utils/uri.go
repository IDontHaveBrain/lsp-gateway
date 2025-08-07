package utils

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// URIToFilePath converts a file:// URI to a file system path
func URIToFilePath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return uri
	}

	// Remove the file:// prefix
	path := strings.TrimPrefix(uri, "file://")
	
	// Decode URL-encoded characters
	decoded, err := url.PathUnescape(path)
	if err == nil {
		path = decoded
	}

	// On Windows, file URIs look like file:///C:/path/to/file
	// After removing file://, we have /C:/path/to/file
	// We need to remove the leading slash for Windows absolute paths
	if runtime.GOOS == "windows" && len(path) > 2 {
		// Check if this looks like a Windows absolute path (e.g., /C:/)
		if path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}
		// Convert forward slashes back to backslashes for Windows
		path = filepath.FromSlash(path)
	}

	return path
}

// FilePathToURI converts a file system path to a file:// URI
func FilePathToURI(path string) string {
	// Clean the path first
	path = filepath.Clean(path)
	
	// Convert to forward slashes for URI
	path = filepath.ToSlash(path)
	
	// On Windows, we need to handle absolute paths specially
	if runtime.GOOS == "windows" && filepath.IsAbs(path) {
		// Windows absolute paths like C:/Users/... need to become file:///C:/Users/...
		return "file:///" + path
	}
	
	// Unix absolute paths like /home/user/... need to become file:///home/user/...
	if strings.HasPrefix(path, "/") {
		return "file://" + path
	}
	
	// Relative paths (shouldn't happen in practice but handle gracefully)
	return "file://" + path
}