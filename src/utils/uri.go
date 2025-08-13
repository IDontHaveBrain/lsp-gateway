package utils

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// Cache infrastructure for URI conversions
var (
	// Thread-safe cache using sync.Map for concurrent access
	uriToFilePathCache = &sync.Map{}
	filePathToURICache = &sync.Map{}

	// Atomic counters for cache statistics
	uriToFilePathHits   int64
	uriToFilePathMisses int64
	filePathToURIHits   int64
	filePathToURIMisses int64
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
		// Normalize drive letter to uppercase for consistency
		if len(path) >= 2 && path[1] == ':' {
			b := path[0]
			if 'a' <= b && b <= 'z' {
				path = strings.ToUpper(path[:1]) + path[1:]
			}
		}
		path = LongPath(path)
	}

	return path
}

// FilePathToURI converts a file system path to a properly escaped file:// URI
func FilePathToURI(path string) string {
	// Clean the path first
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = LongPath(path)
	}

	// Convert to forward slashes for URI
	path = filepath.ToSlash(path)

	// Normalize Windows drive letter to uppercase for consistency
	if runtime.GOOS == "windows" && len(path) >= 2 && path[1] == ':' {
		if 'a' <= path[0] && path[0] <= 'z' {
			path = strings.ToUpper(path[:1]) + path[1:]
		}
	}

	// On Windows, absolute paths need a leading slash to form file:///C:/...
	if runtime.GOOS == "windows" && filepath.IsAbs(path) && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Use url.URL to ensure proper escaping of special characters
	u := url.URL{Scheme: "file", Path: path}
	return u.String()
}

// URIToFilePathCached converts a file:// URI to a file system path with caching
func URIToFilePathCached(uri string) string {
	// Check cache first
	if cached, ok := uriToFilePathCache.Load(uri); ok {
		atomic.AddInt64(&uriToFilePathHits, 1)
		return cached.(string)
	}

	// Cache miss - compute and store result
	atomic.AddInt64(&uriToFilePathMisses, 1)
	result := URIToFilePath(uri)
	uriToFilePathCache.Store(uri, result)
	return result
}

// FilePathToURICached converts a file system path to a properly escaped file:// URI with caching
func FilePathToURICached(path string) string {
	// Check cache first
	if cached, ok := filePathToURICache.Load(path); ok {
		atomic.AddInt64(&filePathToURIHits, 1)
		return cached.(string)
	}

	// Cache miss - compute and store result
	atomic.AddInt64(&filePathToURIMisses, 1)
	result := FilePathToURI(path)
	filePathToURICache.Store(path, result)
	return result
}

// GetURICacheStats returns cache hit/miss statistics for both conversion functions
func GetURICacheStats() (uriToFileHits, uriToFileMisses, fileToURIHits, fileToURIMisses int64) {
	return atomic.LoadInt64(&uriToFilePathHits),
		atomic.LoadInt64(&uriToFilePathMisses),
		atomic.LoadInt64(&filePathToURIHits),
		atomic.LoadInt64(&filePathToURIMisses)
}

// ClearURICache clears all cached URI conversions and resets statistics
func ClearURICache() {
	uriToFilePathCache = &sync.Map{}
	filePathToURICache = &sync.Map{}
	atomic.StoreInt64(&uriToFilePathHits, 0)
	atomic.StoreInt64(&uriToFilePathMisses, 0)
	atomic.StoreInt64(&filePathToURIHits, 0)
	atomic.StoreInt64(&filePathToURIMisses, 0)
}

// GetURICacheSize returns approximate cache sizes (number of entries)
func GetURICacheSize() (uriToFileSize, fileToURISize int) {
	var uriCount, fileCount int

	uriToFilePathCache.Range(func(_, _ interface{}) bool {
		uriCount++
		return true
	})

	filePathToURICache.Range(func(_, _ interface{}) bool {
		fileCount++
		return true
	})

	return uriCount, fileCount
}
