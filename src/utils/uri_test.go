package utils

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilePathToURI(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
		os       string
	}{
		{
			name:     "Unix absolute path",
			path:     "/home/user/file.go",
			expected: "file:///home/user/file.go",
			os:       "linux",
		},
		{
			name:     "Unix path with spaces",
			path:     "/home/user name/my file.go",
			expected: "file:///home/user%20name/my%20file.go",
			os:       "linux",
		},
		{
			name:     "Windows absolute path",
			path:     `C:\Users\username\file.go`,
			expected: "file:///C:/Users/username/file.go",
			os:       "windows",
		},
		{
			name:     "Windows path with spaces",
			path:     `C:\Program Files\app\file.go`,
			expected: "file:///C:/Program%20Files/app/file.go",
			os:       "windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS != tt.os {
				t.Skipf("Skipping %s test on %s", tt.os, runtime.GOOS)
			}
			result := FilePathToURI(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestURIToFilePath(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected string
		os       string
	}{
		{
			name:     "Unix file URI",
			uri:      "file:///home/user/file.go",
			expected: "/home/user/file.go",
			os:       "linux",
		},
		{
			name:     "Unix file URI with spaces",
			uri:      "file:///home/user%20name/my%20file.go",
			expected: "/home/user name/my file.go",
			os:       "linux",
		},
		{
			name:     "Windows file URI",
			uri:      "file:///C:/Users/username/file.go",
			expected: `C:\Users\username\file.go`,
			os:       "windows",
		},
		{
			name:     "Windows file URI with spaces",
			uri:      "file:///C:/Program%20Files/app/file.go",
			expected: `C:\Program Files\app\file.go`,
			os:       "windows",
		},
		{
			name:     "Not a file URI",
			uri:      "/home/user/file.go",
			expected: "/home/user/file.go",
			os:       "linux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS != tt.os {
				t.Skipf("Skipping %s test on %s", tt.os, runtime.GOOS)
			}
			result := URIToFilePath(tt.uri)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRoundTrip(t *testing.T) {
	// Test that converting a path to URI and back gives the original path
	paths := []string{
		"/home/user/file.go",
		"/home/user name/my file.go",
	}

	if runtime.GOOS == "windows" {
		paths = []string{
			`C:\Users\username\file.go`,
			`C:\Program Files\app\file.go`,
			`D:\Projects\my project\src\main.go`,
		}
	}

	for _, path := range paths {
		uri := FilePathToURI(path)
		result := URIToFilePath(uri)
		assert.Equal(t, path, result, "Round trip failed for path: %s", path)
	}
}

func TestURIToFilePathCached(t *testing.T) {
	// Clear cache before test
	ClearURICache()

	testURI := "file:///home/user/test.go"

	// First call should be a cache miss
	result1 := URIToFilePathCached(testURI)
	hits, misses, _, _ := GetURICacheStats()
	assert.Equal(t, int64(0), hits, "Should have 0 hits on first call")
	assert.Equal(t, int64(1), misses, "Should have 1 miss on first call")

	// Second call should be a cache hit
	result2 := URIToFilePathCached(testURI)
	hits, misses, _, _ = GetURICacheStats()
	assert.Equal(t, int64(1), hits, "Should have 1 hit on second call")
	assert.Equal(t, int64(1), misses, "Should still have 1 miss")

	// Results should be identical
	assert.Equal(t, result1, result2, "Cached result should match original")

	// Result should match non-cached version
	expected := URIToFilePath(testURI)
	assert.Equal(t, expected, result1, "Cached result should match non-cached result")
}

func TestFilePathToURICached(t *testing.T) {
	// Clear cache before test
	ClearURICache()

	testPath := "/home/user/test.go"
	if runtime.GOOS == "windows" {
		testPath = `C:\Users\user\test.go`
	}

	// First call should be a cache miss
	result1 := FilePathToURICached(testPath)
	_, _, hits, misses := GetURICacheStats()
	assert.Equal(t, int64(0), hits, "Should have 0 hits on first call")
	assert.Equal(t, int64(1), misses, "Should have 1 miss on first call")

	// Second call should be a cache hit
	result2 := FilePathToURICached(testPath)
	_, _, hits, misses = GetURICacheStats()
	assert.Equal(t, int64(1), hits, "Should have 1 hit on second call")
	assert.Equal(t, int64(1), misses, "Should still have 1 miss")

	// Results should be identical
	assert.Equal(t, result1, result2, "Cached result should match original")

	// Result should match non-cached version
	expected := FilePathToURI(testPath)
	assert.Equal(t, expected, result1, "Cached result should match non-cached result")
}

func TestCacheStatistics(t *testing.T) {
	// Clear cache before test
	ClearURICache()

	// Verify initial state
	uriHits, uriMisses, fileHits, fileMisses := GetURICacheStats()
	assert.Equal(t, int64(0), uriHits)
	assert.Equal(t, int64(0), uriMisses)
	assert.Equal(t, int64(0), fileHits)
	assert.Equal(t, int64(0), fileMisses)

	// Make some cached calls
	testURI := "file:///home/user/test.go"
	testPath := "/home/user/test.go"

	URIToFilePathCached(testURI)  // miss
	URIToFilePathCached(testURI)  // hit
	FilePathToURICached(testPath) // miss
	FilePathToURICached(testPath) // hit

	uriHits, uriMisses, fileHits, fileMisses = GetURICacheStats()
	assert.Equal(t, int64(1), uriHits)
	assert.Equal(t, int64(1), uriMisses)
	assert.Equal(t, int64(1), fileHits)
	assert.Equal(t, int64(1), fileMisses)
}

func TestCacheSize(t *testing.T) {
	// Clear cache before test
	ClearURICache()

	// Verify initial state
	uriSize, fileSize := GetURICacheSize()
	assert.Equal(t, 0, uriSize)
	assert.Equal(t, 0, fileSize)

	// Add some entries to cache
	URIToFilePathCached("file:///test1.go")
	URIToFilePathCached("file:///test2.go")
	FilePathToURICached("/test1.go")

	uriSize, fileSize = GetURICacheSize()
	assert.Equal(t, 2, uriSize, "Should have 2 URI entries")
	assert.Equal(t, 1, fileSize, "Should have 1 file path entry")
}

func TestClearURICache(t *testing.T) {
	// Add some entries to cache
	URIToFilePathCached("file:///test.go")
	FilePathToURICached("/test.go")

	// Verify cache has entries
	uriSize, fileSize := GetURICacheSize()
	assert.True(t, uriSize > 0 || fileSize > 0, "Cache should have entries")

	// Clear cache
	ClearURICache()

	// Verify cache is empty
	uriSize, fileSize = GetURICacheSize()
	assert.Equal(t, 0, uriSize, "URI cache should be empty")
	assert.Equal(t, 0, fileSize, "File path cache should be empty")

	// Verify stats are reset
	uriHits, uriMisses, fileHits, fileMisses := GetURICacheStats()
	assert.Equal(t, int64(0), uriHits)
	assert.Equal(t, int64(0), uriMisses)
	assert.Equal(t, int64(0), fileHits)
	assert.Equal(t, int64(0), fileMisses)
}

func TestCachedRoundTrip(t *testing.T) {
	// Clear cache before test
	ClearURICache()

	// Test that cached round trip conversions work correctly
	paths := []string{
		"/home/user/file.go",
		"/home/user name/my file.go",
	}

	if runtime.GOOS == "windows" {
		paths = []string{
			`C:\Users\username\file.go`,
			`C:\Program Files\app\file.go`,
			`D:\Projects\my project\src\main.go`,
		}
	}

	for _, path := range paths {
		uri := FilePathToURICached(path)
		result := URIToFilePathCached(uri)
		assert.Equal(t, path, result, "Cached round trip failed for path: %s", path)

		// Also verify against non-cached versions
		expectedURI := FilePathToURI(path)
		expectedPath := URIToFilePath(uri)
		assert.Equal(t, expectedURI, uri, "Cached URI should match non-cached")
		assert.Equal(t, expectedPath, result, "Cached path should match non-cached")
	}
}
