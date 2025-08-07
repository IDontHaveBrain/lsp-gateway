package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"lsp-gateway/src/server/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLSPFallback struct {
	response interface{}
	err      error
}

func (m *mockLSPFallback) ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	return m.response, m.err
}

func TestNewWorkspaceIndexer(t *testing.T) {
	fallback := &mockLSPFallback{}
	indexer := cache.NewWorkspaceIndexer(fallback)

	assert.NotNil(t, indexer)
}

func TestGetLanguageExtensions(t *testing.T) {
	indexer := cache.NewWorkspaceIndexer(nil)

	tests := []struct {
		name      string
		languages []string
		expected  []string
	}{
		{
			name:      "Go language",
			languages: []string{"go"},
			expected:  []string{".go"},
		},
		{
			name:      "Python language",
			languages: []string{"python"},
			expected:  []string{".py", ".pyi"},
		},
		{
			name:      "JavaScript language",
			languages: []string{"javascript"},
			expected:  []string{".js", ".jsx", ".mjs"},
		},
		{
			name:      "TypeScript language",
			languages: []string{"typescript"},
			expected:  []string{".ts", ".tsx", ".d.ts"},
		},
		{
			name:      "Java language",
			languages: []string{"java"},
			expected:  []string{".java"},
		},
		{
			name:      "Multiple languages",
			languages: []string{"go", "python"},
			expected:  []string{".go", ".py", ".pyi"},
		},
		{
			name:      "Unknown language",
			languages: []string{"unknown"},
			expected:  []string{".go", ".py", ".js", ".ts", ".java"}, // Default extensions
		},
		{
			name:      "Mixed known and unknown",
			languages: []string{"go", "unknown", "python"},
			expected:  []string{".go", ".py", ".pyi"},
		},
		{
			name:      "Empty languages",
			languages: []string{},
			expected:  []string{".go", ".py", ".pyi", ".js", ".jsx", ".mjs", ".ts", ".tsx", ".d.ts", ".java"}, // Returns all supported extensions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.GetLanguageExtensions(tt.languages)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestScanWorkspaceSourceFiles(t *testing.T) {
	// Create a temporary test directory structure
	tempDir, err := os.MkdirTemp("", "workspace_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := []string{
		"main.go",
		"test.py",
		"app.js",
		"src/module.go",
		"src/lib.py",
		"src/component.tsx",
		"vendor/external.go",  // Should be ignored
		".git/config",         // Should be ignored
		"node_modules/pkg.js", // Should be ignored
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tempDir, file)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	indexer := cache.NewWorkspaceIndexer(nil)

	tests := []struct {
		name        string
		extensions  []string
		maxFiles    int
		expected    []string
		expectedLen int // For cases where we only care about count
	}{
		{
			name:       "Go files only",
			extensions: []string{".go"},
			maxFiles:   10,
			expected: []string{
				filepath.Join(tempDir, "main.go"),
				filepath.Join(tempDir, "src/module.go"),
			},
		},
		{
			name:       "Python files only",
			extensions: []string{".py"},
			maxFiles:   10,
			expected: []string{
				filepath.Join(tempDir, "test.py"),
				filepath.Join(tempDir, "src/lib.py"),
			},
		},
		{
			name:       "Multiple extensions",
			extensions: []string{".go", ".py", ".js", ".tsx"},
			maxFiles:   10,
			expected: []string{
				filepath.Join(tempDir, "main.go"),
				filepath.Join(tempDir, "test.py"),
				filepath.Join(tempDir, "app.js"),
				filepath.Join(tempDir, "src/module.go"),
				filepath.Join(tempDir, "src/lib.py"),
				filepath.Join(tempDir, "src/component.tsx"),
			},
		},
		{
			name:        "Max files limit",
			extensions:  []string{".go", ".py", ".js", ".tsx"},
			maxFiles:    3,
			expectedLen: 3, // We expect exactly 3 files
		},
		{
			name:       "No matching files",
			extensions: []string{".java"},
			maxFiles:   10,
			expected:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.ScanWorkspaceSourceFiles(tempDir, tt.extensions, tt.maxFiles)

			if tt.expected != nil {
				assert.ElementsMatch(t, tt.expected, result)
			} else if tt.expectedLen > 0 {
				assert.Len(t, result, tt.expectedLen)
			}
		})
	}
}

func TestIndexWorkspaceFiles(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "workspace_index_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files
	goFile := filepath.Join(tempDir, "main.go")
	err = os.WriteFile(goFile, []byte("package main\n\nfunc main() {}"), 0644)
	require.NoError(t, err)

	pyFile := filepath.Join(tempDir, "test.py")
	err = os.WriteFile(pyFile, []byte("def test():\n    pass"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name      string
		languages []string
		maxFiles  int
		mockErr   error
		expectErr bool
	}{
		{
			name:      "Successful indexing",
			languages: []string{"go", "python"},
			maxFiles:  10,
			mockErr:   nil,
			expectErr: false,
		},
		{
			name:      "LSP fallback error",
			languages: []string{"go"},
			maxFiles:  10,
			mockErr:   assert.AnError,
			expectErr: false, // Errors are logged but not returned
		},
		{
			name:      "Empty languages",
			languages: []string{},
			maxFiles:  10,
			mockErr:   nil,
			expectErr: false,
		},
		{
			name:      "Zero max files",
			languages: []string{"go"},
			maxFiles:  0,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock response that simulates workspace symbols
			mockResponse := []map[string]interface{}{
				{
					"name": "testSymbol",
					"kind": 12, // Function
					"location": map[string]interface{}{
						"uri": "file://" + goFile,
					},
				},
			}

			fallback := &mockLSPFallback{
				response: mockResponse,
				err:      tt.mockErr,
			}

			indexer := cache.NewWorkspaceIndexer(fallback)
			ctx := context.Background()

			err := indexer.IndexWorkspaceFiles(ctx, tempDir, tt.languages, tt.maxFiles)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIndexWorkspaceFiles_ContextCancellation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "workspace_cancel_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test file
	err = os.WriteFile(filepath.Join(tempDir, "test.go"), []byte("package main"), 0644)
	require.NoError(t, err)

	fallback := &mockLSPFallback{
		response: []map[string]interface{}{},
		err:      nil,
	}

	indexer := cache.NewWorkspaceIndexer(fallback)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = indexer.IndexWorkspaceFiles(ctx, tempDir, []string{"go"}, 10)
	assert.NoError(t, err) // Should exit early without error
}
