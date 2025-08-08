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

// FilePathToURI converts a file system path to a properly escaped file:// URI
func FilePathToURI(path string) string {
	// Clean the path first
	path = filepath.Clean(path)

	// Convert to forward slashes for URI
	path = filepath.ToSlash(path)

	// On Windows, absolute paths need a leading slash to form file:///C:/...
	if runtime.GOOS == "windows" && filepath.IsAbs(path) && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Use url.URL to ensure proper escaping of special characters
	u := url.URL{Scheme: "file", Path: path}
	return u.String()
}
