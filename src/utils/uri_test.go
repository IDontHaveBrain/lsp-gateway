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
			expected: "file:///home/user name/my file.go",
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
			expected: "file:///C:/Program Files/app/file.go",
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
			uri:      "file:///home/user name/my file.go",
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
			uri:      "file:///C:/Program Files/app/file.go",
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