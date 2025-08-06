package types

import (
	"testing"
)

func TestMatchFilePattern(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		pattern  string
		expected bool
	}{
		// Current directory patterns
		{"Current dir .", "/home/test/file.go", ".", true},
		{"Current dir ./", "/home/test/file.go", "./", true},

		// Simple glob patterns
		{"Extension glob", "/home/test/file.go", "*.go", true},
		{"Extension glob no match", "/home/test/file.py", "*.go", false},

		// Directory patterns
		{"Directory prefix", "/home/test/src/file.go", "src/", true},
		{"Directory prefix no match", "/home/test/lib/file.go", "src/", false},
		{"Directory no slash", "/home/test/src/file.go", "src", true},

		// Recursive patterns
		{"Recursive **/*.go", "/home/test/src/nested/file.go", "**/*.go", true},
		{"Recursive src/**/*.go", "/home/test/src/nested/deep/file.go", "src/**/*.go", true},
		{"Recursive src/**/*.go no match", "/home/test/lib/file.go", "src/**/*.go", false},

		// Complex patterns
		{"src/*.go direct", "/home/test/src/file.go", "src/*.go", true},
		{"src/*.go nested no match", "/home/test/src/nested/file.go", "src/*.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchFilePattern(tt.filePath, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchFilePattern(%q, %q) = %v, want %v",
					tt.filePath, tt.pattern, result, tt.expected)
			}
		})
	}
}
