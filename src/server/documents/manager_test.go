package documents

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Mock LSP client for testing
type mockLSPClient struct {
	mu            sync.Mutex
	notifications []notificationCall
	active        bool
	supports      map[string]bool
}

type notificationCall struct {
	method string
	params interface{}
}

func newMockLSPClient() *mockLSPClient {
	return &mockLSPClient{
		notifications: make([]notificationCall, 0),
		active:        true,
		supports: map[string]bool{
			"textDocument/didOpen":   true,
			"textDocument/didClose":  true,
			"textDocument/didChange": true,
		},
	}
}

func (m *mockLSPClient) Start(ctx context.Context) error {
	m.active = true
	return nil
}

func (m *mockLSPClient) Stop() error {
	m.active = false
	return nil
}

func (m *mockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return json.RawMessage(`{"result": {}}`), nil
}

func (m *mockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, notificationCall{
		method: method,
		params: params,
	})
	return nil
}

func (m *mockLSPClient) IsActive() bool {
	return m.active
}

func (m *mockLSPClient) Supports(method string) bool {
	return m.supports[method]
}

func TestNewLSPDocumentManager(t *testing.T) {
	manager := NewLSPDocumentManager()
	if manager == nil {
		t.Fatal("NewLSPDocumentManager returned nil")
	}
}

func TestDetectLanguage(t *testing.T) {
	manager := NewLSPDocumentManager()

	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{
			name:     "Go file",
			uri:      "file:///test/main.go",
			expected: "go",
		},
		{
			name:     "Python file",
			uri:      "file:///test/script.py",
			expected: "python",
		},
		{
			name:     "JavaScript file",
			uri:      "file:///test/app.js",
			expected: "javascript",
		},
		{
			name:     "TypeScript file",
			uri:      "file:///test/component.ts",
			expected: "typescript",
		},
		{
			name:     "Java file",
			uri:      "file:///test/Main.java",
			expected: "java",
		},
		{
			name:     "JSX file",
			uri:      "file:///test/component.jsx",
			expected: "javascript",
		},
		{
			name:     "TSX file",
			uri:      "file:///test/component.tsx",
			expected: "typescript",
		},
		{
			name:     "Unknown extension",
			uri:      "file:///test/data.txt",
			expected: "",
		},
		{
			name:     "No extension",
			uri:      "file:///test/README",
			expected: "",
		},
		{
			name:     "Mixed case extension",
			uri:      "file:///test/Main.GO",
			expected: "go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			language := manager.DetectLanguage(tt.uri)
			if language != tt.expected {
				t.Errorf("DetectLanguage(%s) = %v, want %v", tt.uri, language, tt.expected)
			}
		})
	}
}

func TestExtractURI(t *testing.T) {
	manager := NewLSPDocumentManager()

	tests := []struct {
		name        string
		params      interface{}
		expectedURI string
		expectError bool
	}{
		{
			name: "textDocument with URI",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test/main.go",
				},
			},
			expectedURI: "file:///test/main.go",
			expectError: false,
		},
		{
			name: "uri field directly",
			params: map[string]interface{}{
				"uri": "file:///test/script.py",
			},
			expectedURI: "file:///test/script.py",
			expectError: false,
		},
		{
			name: "nested textDocument",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri":     "file:///test/app.js",
					"version": 1,
				},
				"position": map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			},
			expectedURI: "file:///test/app.js",
			expectError: false,
		},
		{
			name:        "no URI field",
			params:      map[string]interface{}{},
			expectedURI: "",
			expectError: true,
		},
		{
			name:        "nil params",
			params:      nil,
			expectedURI: "",
			expectError: true,
		},
		{
			name: "invalid textDocument type",
			params: map[string]interface{}{
				"textDocument": "not a map",
			},
			expectedURI: "",
			expectError: true,
		},
		{
			name: "URI not string",
			params: map[string]interface{}{
				"uri": 123,
			},
			expectedURI: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri, err := manager.ExtractURI(tt.params)

			if tt.expectError {
				if err == nil {
					t.Error("ExtractURI() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("ExtractURI() unexpected error: %v", err)
				}
				if uri != tt.expectedURI {
					t.Errorf("ExtractURI() = %v, want %v", uri, tt.expectedURI)
				}
			}
		})
	}
}

func TestEnsureOpen(t *testing.T) {
	manager := NewLSPDocumentManager()
	client := newMockLSPClient()

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	testContent := "package main\n\nfunc main() {}\n"

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fileURI := fmt.Sprintf("file://%s", testFile)

	tests := []struct {
		name        string
		uri         string
		params      interface{}
		expectError bool
	}{
		{
			name: "valid Go file",
			uri:  fileURI,
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
			},
			expectError: false,
		},
		{
			name: "non-existent file",
			uri:  "file:///nonexistent/file.go",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///nonexistent/file.go",
				},
			},
			expectError: false, // Should not error, just log warning
		},
		{
			name: "invalid URI",
			uri:  "not-a-uri",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "not-a-uri",
				},
			},
			expectError: false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset client notifications
			client.notifications = make([]notificationCall, 0)

			err := manager.EnsureOpen(client, tt.uri, tt.params)

			if tt.expectError {
				if err == nil {
					t.Error("EnsureOpen() expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("EnsureOpen() unexpected error: %v", err)
				}

				// Check that didOpen notification was sent
				found := false
				for _, call := range client.notifications {
					if call.method == "textDocument/didOpen" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected textDocument/didOpen notification to be sent")
				}
			}
		})
	}
}

func TestEnsureOpenWithFileContent(t *testing.T) {
	manager := NewLSPDocumentManager()
	client := newMockLSPClient()

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.py")
	testContent := "#!/usr/bin/env python3\nprint('Hello, World!')\n"

	err := os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fileURI := fmt.Sprintf("file://%s", testFile)

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
	}

	err = manager.EnsureOpen(client, fileURI, params)
	if err != nil {
		t.Errorf("EnsureOpen() unexpected error: %v", err)
	}

	// Check that didOpen notification was sent with correct content
	found := false
	for _, call := range client.notifications {
		if call.method == "textDocument/didOpen" {
			if params, ok := call.params.(map[string]interface{}); ok {
				if textDoc, ok := params["textDocument"].(map[string]interface{}); ok {
					if text, ok := textDoc["text"].(string); ok {
						if text == testContent {
							found = true
							break
						}
					}
				}
			}
		}
	}
	if !found {
		t.Error("Expected textDocument/didOpen notification with correct file content")
	}
}

func TestLanguageDetectionInEnsureOpen(t *testing.T) {
	manager := NewLSPDocumentManager()
	client := newMockLSPClient()

	tempDir := t.TempDir()

	tests := []struct {
		name           string
		filename       string
		expectedLangId string
	}{
		{
			name:           "TypeScript file",
			filename:       "app.ts",
			expectedLangId: "typescript",
		},
		{
			name:           "JavaScript file",
			filename:       "script.js",
			expectedLangId: "javascript",
		},
		{
			name:           "Python file",
			filename:       "main.py",
			expectedLangId: "python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, tt.filename)
			err := os.WriteFile(testFile, []byte("// test content"), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			fileURI := fmt.Sprintf("file://%s", testFile)
			params := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
			}

			// Reset client notifications
			client.notifications = make([]notificationCall, 0)

			err = manager.EnsureOpen(client, fileURI, params)
			if err != nil {
				t.Errorf("EnsureOpen() unexpected error: %v", err)
			}

			// Check language ID in notification
			found := false
			for _, call := range client.notifications {
				if call.method == "textDocument/didOpen" {
					if params, ok := call.params.(map[string]interface{}); ok {
						if textDoc, ok := params["textDocument"].(map[string]interface{}); ok {
							if langId, ok := textDoc["languageId"].(string); ok {
								if langId == tt.expectedLangId {
									found = true
									break
								}
							}
						}
					}
				}
			}
			if !found {
				t.Errorf("Expected textDocument/didOpen with languageId '%s'", tt.expectedLangId)
			}
		})
	}
}

func TestDocumentManagerInterface(t *testing.T) {
	// Test that LSPDocumentManager implements DocumentManager interface
	var _ DocumentManager = NewLSPDocumentManager()

	manager := NewLSPDocumentManager()

	// Test DetectLanguage method
	lang := manager.DetectLanguage("file:///test.go")
	if lang != "go" {
		t.Errorf("Expected 'go', got '%s'", lang)
	}

	// Test ExtractURI method
	params := map[string]interface{}{
		"uri": "file:///test/main.go",
	}
	uri, err := manager.ExtractURI(params)
	if err != nil {
		t.Errorf("ExtractURI() error = %v", err)
	}
	if uri != "file:///test/main.go" {
		t.Errorf("ExtractURI() = %v, want 'file:///test/main.go'", uri)
	}

	// Test EnsureOpen method with mock client
	client := newMockLSPClient()
	err = manager.EnsureOpen(client, "file:///test.go", params)
	if err != nil {
		t.Errorf("EnsureOpen() error = %v", err)
	}
}

func TestErrorHandling(t *testing.T) {
	manager := NewLSPDocumentManager()

	// Test ExtractURI with malformed params
	tests := []struct {
		name   string
		params interface{}
	}{
		{
			name:   "string instead of map",
			params: "not a map",
		},
		{
			name:   "slice instead of map",
			params: []string{"array", "data"},
		},
		{
			name:   "number instead of map",
			params: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.ExtractURI(tt.params)
			if err == nil {
				t.Error("ExtractURI() expected error for malformed params")
			}
		})
	}
}

// Mock LSP client that always fails on SendNotification
type failingMockLSPClient struct {
	active   bool
	supports map[string]bool
}

func (m *failingMockLSPClient) Start(ctx context.Context) error {
	m.active = true
	return nil
}

func (m *failingMockLSPClient) Stop() error {
	m.active = false
	return nil
}

func (m *failingMockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return json.RawMessage(`{"result": {}}`), nil
}

func (m *failingMockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	return fmt.Errorf("mock notification failure")
}

func (m *failingMockLSPClient) IsActive() bool {
	return m.active
}

func (m *failingMockLSPClient) Supports(method string) bool {
	return m.supports[method]
}

func TestClientNotificationFailure(t *testing.T) {
	manager := NewLSPDocumentManager()

	// Create a mock client that fails on SendNotification
	failingClient := &failingMockLSPClient{
		active: true,
		supports: map[string]bool{
			"textDocument/didOpen": true,
		},
	}

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")
	err := os.WriteFile(testFile, []byte("package main"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fileURI := fmt.Sprintf("file://%s", testFile)
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
	}

	err = manager.EnsureOpen(failingClient, fileURI, params)
	if err == nil {
		t.Error("EnsureOpen() expected error when client notification fails")
	}
}

// Mock reader/writer for testing I/O scenarios
type mockReadCloser struct {
	data []byte
	pos  int
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, io.EOF
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	return nil
}

func TestConcurrentDocumentOperations(t *testing.T) {
	manager := NewLSPDocumentManager()
	client := newMockLSPClient()

	tempDir := t.TempDir()

	// Create multiple test files
	files := make([]string, 10)
	for i := 0; i < 10; i++ {
		filename := fmt.Sprintf("test%d.go", i)
		testFile := filepath.Join(tempDir, filename)
		err := os.WriteFile(testFile, []byte(fmt.Sprintf("package test%d", i)), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
		files[i] = fmt.Sprintf("file://%s", testFile)
	}

	// Test concurrent EnsureOpen calls
	done := make(chan error, len(files))
	for _, fileURI := range files {
		go func(uri string) {
			params := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": uri,
				},
			}
			done <- manager.EnsureOpen(client, uri, params)
		}(fileURI)
	}

	// Wait for all operations to complete
	for i := 0; i < len(files); i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent EnsureOpen failed: %v", err)
		}
	}

	// Verify all notifications were sent
	client.mu.Lock()
	notificationCount := len(client.notifications)
	client.mu.Unlock()
	if notificationCount != len(files) {
		t.Errorf("Expected %d notifications, got %d", len(files), notificationCount)
	}
}
