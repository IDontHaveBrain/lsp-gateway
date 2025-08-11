package unit

import (
	"testing"

	"lsp-gateway/src/utils/filepattern"
)

func TestFilePatternComprehensive(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
		desc    string
	}{
		// Basic glob patterns
		{
			name:    "Simple star glob",
			path:    "main.go",
			pattern: "*.go",
			want:    true,
			desc:    "Should match any .go file",
		},
		{
			name:    "No extension match",
			path:    "main.js",
			pattern: "*.go",
			want:    false,
			desc:    "Should not match different extension",
		},

		// Directory-specific patterns
		{
			name:    "Exact directory match",
			path:    "src/server/gateway.go",
			pattern: "src/server/*.go",
			want:    true,
			desc:    "Should match files in specific directory",
		},
		{
			name:    "Subdirectory exclusion",
			path:    "src/server/cache/manager.go",
			pattern: "src/server/*.go",
			want:    false,
			desc:    "Should not match files in subdirectories",
		},
		{
			name:    "Parent directory no match",
			path:    "src/gateway.go",
			pattern: "src/server/*.go",
			want:    false,
			desc:    "Should not match files in parent directory",
		},

		// Recursive patterns with **
		{
			name:    "Recursive from root",
			path:    "deeply/nested/path/file.go",
			pattern: "**/*.go",
			want:    true,
			desc:    "Should match .go files at any depth",
		},
		{
			name:    "Recursive in subdirectory",
			path:    "src/server/cache/deep/manager.go",
			pattern: "src/server/**/*.go",
			want:    true,
			desc:    "Should match files in subdirectories recursively",
		},
		{
			name:    "Recursive middle pattern",
			path:    "src/a/b/c/file.go",
			pattern: "src/**/c/*.go",
			want:    true,
			desc:    "Should match with ** in the middle",
		},
		{
			name:    "Recursive prefix pattern",
			path:    "src/server/file.go",
			pattern: "src/**/file.go",
			want:    true,
			desc:    "Should match specific file at any depth",
		},

		// Windows path compatibility
		{
			name:    "Windows backslash path",
			path:    `src\server\gateway.go`,
			pattern: "src/server/*.go",
			want:    true,
			desc:    "Should normalize Windows paths",
		},
		{
			name:    "Mixed slashes",
			path:    `src\server/gateway.go`,
			pattern: "src/server/*.go",
			want:    true,
			desc:    "Should handle mixed path separators",
		},
		{
			name:    "Windows pattern",
			path:    "src/server/gateway.go",
			pattern: `src\server\*.go`,
			want:    true,
			desc:    "Should normalize Windows patterns",
		},

		// File URI handling
		{
			name:    "File URI basic",
			path:    "file:///home/user/project/main.go",
			pattern: "*.go",
			want:    true,
			desc:    "Should handle file:// URIs",
		},
		{
			name:    "File URI with path",
			path:    "file:///home/user/project/src/server/gateway.go",
			pattern: "src/server/*.go",
			want:    true,
			desc:    "Should match path in file URI",
		},
		{
			name:    "File URI Windows",
			path:    "file:///C:/Users/project/src/main.go",
			pattern: "src/*.go",
			want:    true,
			desc:    "Should handle Windows file URIs",
		},

		// Edge cases
		{
			name:    "Empty pattern",
			path:    "any/file.go",
			pattern: "",
			want:    true,
			desc:    "Empty pattern should match all",
		},
		{
			name:    "Star pattern",
			path:    "any/file.go",
			pattern: "*",
			want:    true,
			desc:    "Single * should match all",
		},
		{
			name:    "Star star slash star",
			path:    "any/file.go",
			pattern: "**/*",
			want:    true,
			desc:    "**/* should match all",
		},
		{
			name:    "Directory prefix with slash",
			path:    "src/server/file.go",
			pattern: "src/",
			want:    true,
			desc:    "Directory prefix should match",
		},
		{
			name:    "Wrong directory prefix",
			path:    "tests/file.go",
			pattern: "src/",
			want:    false,
			desc:    "Wrong directory prefix should not match",
		},

		// Complex patterns
		{
			name:    "Multiple wildcards",
			path:    "src/server/test_file_v2.go",
			pattern: "src/*/test_*.go",
			want:    true,
			desc:    "Should handle multiple wildcards",
		},
		{
			name:    "Character class",
			path:    "file1.go",
			pattern: "file[0-9].go",
			want:    true,
			desc:    "Should support character classes",
		},
		{
			name:    "Character class no match",
			path:    "fileA.go",
			pattern: "file[0-9].go",
			want:    false,
			desc:    "Character class should not match outside range",
		},
		{
			name:    "Question mark wildcard",
			path:    "test.go",
			pattern: "tes?.go",
			want:    true,
			desc:    "? should match single character",
		},

		// Absolute vs relative paths
		{
			name:    "Absolute path with relative pattern",
			path:    "/home/user/project/src/main.go",
			pattern: "src/*.go",
			want:    true,
			desc:    "Should match absolute paths with relative patterns",
		},
		{
			name:    "Both absolute",
			path:    "/home/user/project/src/main.go",
			pattern: "/home/user/project/src/*.go",
			want:    true,
			desc:    "Should match when both are absolute",
		},

		// Special characters in filenames
		{
			name:    "Filename with dash",
			path:    "src/test-file.go",
			pattern: "src/*.go",
			want:    true,
			desc:    "Should match files with dashes",
		},
		{
			name:    "Filename with underscore",
			path:    "src/test_file.go",
			pattern: "src/*.go",
			want:    true,
			desc:    "Should match files with underscores",
		},
		{
			name:    "Filename with dot",
			path:    "src/test.file.go",
			pattern: "src/*.go",
			want:    true,
			desc:    "Should match files with dots",
		},

		// Nested directory patterns
		{
			name:    "Specific nested path",
			path:    "vendor/github.com/user/repo/file.go",
			pattern: "vendor/**/*.go",
			want:    true,
			desc:    "Should match deeply nested vendor files",
		},
		{
			name:    "Exclude vendor simple",
			path:    "vendor/file.go",
			pattern: "src/**/*.go",
			want:    false,
			desc:    "Should not match vendor when looking in src",
		},

		// Multiple directory levels
		{
			name:    "Two levels exact",
			path:    "a/b/file.go",
			pattern: "a/b/*.go",
			want:    true,
			desc:    "Should match two-level directory exactly",
		},
		{
			name:    "Three levels exact",
			path:    "a/b/c/file.go",
			pattern: "a/b/c/*.go",
			want:    true,
			desc:    "Should match three-level directory exactly",
		},
		{
			name:    "Mismatch level depth",
			path:    "a/b/c/d/file.go",
			pattern: "a/b/*.go",
			want:    false,
			desc:    "Should not match deeper than specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filepattern.Match(tt.path, tt.pattern)
			if got != tt.want {
				t.Errorf("%s: Match(%q, %q) = %v, want %v",
					tt.desc, tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

// TestFilePatternBenchmark provides benchmark cases for performance testing
func TestFilePatternPerformance(t *testing.T) {
	// Test with large number of patterns to ensure performance
	patterns := []string{
		"*.go",
		"src/**/*.go",
		"src/server/*.go",
		"vendor/**/*.go",
		"**/*_test.go",
	}

	paths := []string{
		"main.go",
		"src/server/gateway.go",
		"src/server/cache/manager.go",
		"vendor/external/lib.go",
		"tests/integration_test.go",
		"deeply/nested/path/to/file.go",
	}

	// Run all combinations
	for _, pattern := range patterns {
		for _, path := range paths {
			_ = filepattern.Match(path, pattern)
		}
	}

	// If we get here without hanging, performance is acceptable
	t.Log("Performance test completed successfully")
}

// TestFilePatternDocumentation ensures our documentation examples work
func TestFilePatternDocumentationExamples(t *testing.T) {
	examples := []struct {
		desc    string
		path    string
		pattern string
		want    bool
	}{
		{
			desc:    "Example: All Go files",
			path:    "main.go",
			pattern: "*.go",
			want:    true,
		},
		{
			desc:    "Example: Go files in src/server",
			path:    "src/server/gateway.go",
			pattern: "src/server/*.go",
			want:    true,
		},
		{
			desc:    "Example: Python files in src and subdirs",
			path:    "src/lib/utils.py",
			pattern: "src/**/*.py",
			want:    true,
		},
		{
			desc:    "Example: All files",
			path:    "any/path/to/file.txt",
			pattern: "**/*",
			want:    true,
		},
	}

	for _, ex := range examples {
		t.Run(ex.desc, func(t *testing.T) {
			got := filepattern.Match(ex.path, ex.pattern)
			if got != ex.want {
				t.Errorf("Documentation example failed: %s\nMatch(%q, %q) = %v, want %v",
					ex.desc, ex.path, ex.pattern, got, ex.want)
			}
		})
	}
}