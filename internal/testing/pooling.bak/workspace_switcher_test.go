package pooling

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/transport"
)

// Mock implementations for testing

// MockLSPClient is a mock implementation of transport.LSPClient for testing
type MockLSPClient struct {
	requests      []MockRequest
	notifications []MockNotification
	responses     map[string]json.RawMessage
	errors        map[string]error
	active        bool
	mu            sync.RWMutex
}

type MockRequest struct {
	Method string
	Params interface{}
	Ctx    context.Context
}

type MockNotification struct {
	Method string
	Params interface{}
	Ctx    context.Context
}

func NewMockLSPClient() *MockLSPClient {
	return &MockLSPClient{
		requests:      make([]MockRequest, 0),
		notifications: make([]MockNotification, 0),
		responses:     make(map[string]json.RawMessage),
		errors:        make(map[string]error),
		active:        true,
	}
}

func (m *MockLSPClient) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = true
	return nil
}

func (m *MockLSPClient) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = false
	return nil
}

func (m *MockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.requests = append(m.requests, MockRequest{
		Method: method,
		Params: params,
		Ctx:    ctx,
	})
	
	if err, exists := m.errors[method]; exists {
		return nil, err
	}
	
	if response, exists := m.responses[method]; exists {
		return response, nil
	}
	
	// Default successful response
	return json.RawMessage(`{}`), nil
}

func (m *MockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.notifications = append(m.notifications, MockNotification{
		Method: method,
		Params: params,
		Ctx:    ctx,
	})
	
	if err, exists := m.errors[method]; exists {
		return err
	}
	
	return nil
}

func (m *MockLSPClient) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

func (m *MockLSPClient) GetRequests() []MockRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]MockRequest{}, m.requests...)
}

func (m *MockLSPClient) GetNotifications() []MockNotification {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]MockNotification{}, m.notifications...)
}

func (m *MockLSPClient) SetResponse(method string, response json.RawMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[method] = response
}

func (m *MockLSPClient) SetError(method string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[method] = err
}

func (m *MockLSPClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = make([]MockRequest, 0)
	m.notifications = make([]MockNotification, 0)
	m.responses = make(map[string]json.RawMessage)
	m.errors = make(map[string]error)
}

// MockLogger for testing
type MockLogger struct {
	logs []LogEntry
	mu   sync.RWMutex
}

type LogEntry struct {
	Level   string
	Message string
	Args    []interface{}
}

func NewMockLogger() *MockLogger {
	return &MockLogger{
		logs: make([]LogEntry, 0),
	}
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, LogEntry{Level: "DEBUG", Message: msg, Args: args})
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, LogEntry{Level: "INFO", Message: msg, Args: args})
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, LogEntry{Level: "WARN", Message: msg, Args: args})
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, LogEntry{Level: "ERROR", Message: msg, Args: args})
}

func (m *MockLogger) GetLogs() []LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]LogEntry{}, m.logs...)
}

func (m *MockLogger) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = make([]LogEntry, 0)
}

// Test helper functions

func createTempWorkspace(t *testing.T, projectType string) string {
	tempDir, err := os.MkdirTemp("", "workspace_test_")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	// Create project-specific files based on type
	switch projectType {
	case "java":
		createFile(t, filepath.Join(tempDir, "pom.xml"), `<?xml version="1.0"?><project></project>`)
		createDir(t, filepath.Join(tempDir, "src", "main", "java"))
	case "typescript":
		createFile(t, filepath.Join(tempDir, "tsconfig.json"), `{"compilerOptions": {}}`)
		createFile(t, filepath.Join(tempDir, "package.json"), `{"name": "test"}`)
	case "go":
		createFile(t, filepath.Join(tempDir, "go.mod"), `module test`)
		createFile(t, filepath.Join(tempDir, "main.go"), `package main`)
	case "python":
		createFile(t, filepath.Join(tempDir, "setup.py"), `from setuptools import setup`)
		createFile(t, filepath.Join(tempDir, "__init__.py"), ``)
	default:
		// Create a generic workspace
		createFile(t, filepath.Join(tempDir, "README.md"), `# Test Workspace`)
	}
	
	return tempDir
}

func createFile(t *testing.T, path, content string) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}
	
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create file %s: %v", path, err)
	}
}

func createDir(t *testing.T, path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", path, err)
	}
}

func cleanupWorkspace(t *testing.T, path string) {
	if err := os.RemoveAll(path); err != nil {
		t.Logf("Warning: Failed to cleanup workspace %s: %v", path, err)
	}
}

// Tests for WorkspaceSwitcher

func TestNewWorkspaceSwitcher(t *testing.T) {
	mockClient := NewMockLSPClient()
	mockLogger := NewMockLogger()
	
	switcher := NewWorkspaceSwitcher(mockClient, mockLogger)
	
	// Verify initialization
	if switcher.client != mockClient {
		t.Error("Expected client to be set correctly")
	}
	
	if switcher.logger != mockLogger {
		t.Error("Expected logger to be set correctly")
	}
	
	// Verify default strategies are registered
	expectedLanguages := []string{"java", "typescript", "go", "python"}
	for _, lang := range expectedLanguages {
		if !switcher.CanSwitchWorkspace(lang) {
			t.Errorf("Expected strategy to be registered for language: %s", lang)
		}
	}
}

func TestWorkspaceSwitcher_SwitchWorkspace_SameWorkspace(t *testing.T) {
	mockClient := NewMockLSPClient()
	mockLogger := NewMockLogger()
	switcher := NewWorkspaceSwitcher(mockClient, mockLogger)
	
	workspace := createTempWorkspace(t, "java")
	defer cleanupWorkspace(t, workspace)
	
	// Set initial workspace
	switcher.mu.Lock()
	switcher.currentWorkspace = workspace
	switcher.mu.Unlock()
	
	// Try to switch to the same workspace
	ctx := context.Background()
	err := switcher.SwitchWorkspace(ctx, workspace, "java")
	
	if err != nil {
		t.Errorf("Expected no error when switching to same workspace, got: %v", err)
	}
	
	// Verify no notifications were sent
	notifications := mockClient.GetNotifications()
	if len(notifications) != 0 {
		t.Errorf("Expected no notifications for same workspace switch, got: %d", len(notifications))
	}
}

func TestWorkspaceSwitcher_SwitchWorkspace_JavaSuccess(t *testing.T) {
	mockClient := NewMockLSPClient()
	mockLogger := NewMockLogger()
	switcher := NewWorkspaceSwitcher(mockClient, mockLogger)
	
	fromWorkspace := createTempWorkspace(t, "java")
	toWorkspace := createTempWorkspace(t, "java")
	defer cleanupWorkspace(t, fromWorkspace)
	defer cleanupWorkspace(t, toWorkspace)
	
	// Set initial workspace
	switcher.mu.Lock()
	switcher.currentWorkspace = fromWorkspace
	switcher.mu.Unlock()
	
	// Mock successful workspace/symbol response for validation
	mockClient.SetResponse("workspace/symbol", json.RawMessage(`[]`))
	
	ctx := context.Background()
	err := switcher.SwitchWorkspace(ctx, toWorkspace, "java")
	
	if err != nil {
		t.Errorf("Expected successful workspace switch, got error: %v", err)
	}
	
	// Verify current workspace was updated
	if switcher.GetCurrentWorkspace() != toWorkspace {
		t.Errorf("Expected current workspace to be %s, got %s", toWorkspace, switcher.GetCurrentWorkspace())
	}
	
	// Verify workspace/didChangeWorkspaceFolders notification was sent
	notifications := mockClient.GetNotifications()
	found := false
	for _, notification := range notifications {
		if notification.Method == "workspace/didChangeWorkspaceFolders" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected workspace/didChangeWorkspaceFolders notification to be sent")
	}
}

func TestWorkspaceSwitcher_SwitchWorkspace_InvalidLanguage(t *testing.T) {
	mockClient := NewMockLSPClient()
	mockLogger := NewMockLogger()
	switcher := NewWorkspaceSwitcher(mockClient, mockLogger)
	
	workspace := createTempWorkspace(t, "generic")
	defer cleanupWorkspace(t, workspace)
	
	ctx := context.Background()
	err := switcher.SwitchWorkspace(ctx, workspace, "invalidlanguage")
	
	if err == nil {
		t.Error("Expected error for invalid language")
	}
	
	if !fmt.Sprintf("%v", err).contains("no workspace switching strategy") {
		t.Errorf("Expected error message about missing strategy, got: %v", err)
	}
}

func TestWorkspaceSwitcher_SwitchWorkspace_ValidationFailure(t *testing.T) {
	mockClient := NewMockLSPClient()
	mockLogger := NewMockLogger()
	switcher := NewWorkspaceSwitcher(mockClient, mockLogger)
	
	// Try to switch to non-existent workspace
	nonExistentWorkspace := "/non/existent/path"
	
	ctx := context.Background()
	err := switcher.SwitchWorkspace(ctx, nonExistentWorkspace, "java")
	
	if err == nil {
		t.Error("Expected error for non-existent workspace")
	}
	
	if !fmt.Sprintf("%v", err).contains("validation failed") {
		t.Errorf("Expected validation failure error, got: %v", err)
	}
}

func TestWorkspaceSwitcher_RegisterStrategy(t *testing.T) {
	mockClient := NewMockLSPClient()
	mockLogger := NewMockLogger()
	switcher := NewWorkspaceSwitcher(mockClient, mockLogger)
	
	// Create a custom strategy
	customStrategy := NewJavaWorkspaceSwitchStrategy(mockLogger)
	
	// Register custom strategy for a new language
	switcher.RegisterStrategy("customlang", customStrategy)
	
	// Verify the strategy was registered
	if !switcher.CanSwitchWorkspace("customlang") {
		t.Error("Expected custom strategy to be registered")
	}
}

func TestWorkspaceSwitcher_Metrics(t *testing.T) {
	mockClient := NewMockLSPClient()
	mockLogger := NewMockLogger()
	switcher := NewWorkspaceSwitcher(mockClient, mockLogger)
	
	workspace1 := createTempWorkspace(t, "java")
	workspace2 := createTempWorkspace(t, "java")
	defer cleanupWorkspace(t, workspace1)
	defer cleanupWorkspace(t, workspace2)
	
	// Set initial workspace
	switcher.mu.Lock()
	switcher.currentWorkspace = workspace1
	switcher.mu.Unlock()
	
	// Mock successful responses
	mockClient.SetResponse("workspace/symbol", json.RawMessage(`[]`))
	
	ctx := context.Background()
	
	// Perform successful switch
	err := switcher.SwitchWorkspace(ctx, workspace2, "java")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Check metrics
	metrics := switcher.GetMetrics()
	if metrics.TotalSwitches != 1 {
		t.Errorf("Expected 1 total switch, got %d", metrics.TotalSwitches)
	}
	
	if metrics.SuccessfulSwitches != 1 {
		t.Errorf("Expected 1 successful switch, got %d", metrics.SuccessfulSwitches)
	}
	
	if metrics.FailedSwitches != 0 {
		t.Errorf("Expected 0 failed switches, got %d", metrics.FailedSwitches)
	}
	
	// Check language-specific metrics
	javaStats, exists := metrics.LanguageStats["java"]
	if !exists {
		t.Error("Expected Java language stats to exist")
	} else {
		if javaStats.TotalSwitches != 1 {
			t.Errorf("Expected 1 Java switch, got %d", javaStats.TotalSwitches)
		}
		if !javaStats.LastSwitchSuccess {
			t.Error("Expected last Java switch to be successful")
		}
	}
	
	// Check success rate
	successRate := switcher.GetSuccessRate()
	if successRate != 1.0 {
		t.Errorf("Expected success rate of 1.0, got %f", successRate)
	}
	
	langSuccessRate := switcher.GetLanguageSuccessRate("java")
	if langSuccessRate != 1.0 {
		t.Errorf("Expected Java success rate of 1.0, got %f", langSuccessRate)
	}
}

// Tests for WorkspaceSwitchStrategy implementations

func TestJavaWorkspaceSwitchStrategy_CanSwitch(t *testing.T) {
	mockLogger := NewMockLogger()
	strategy := NewJavaWorkspaceSwitchStrategy(mockLogger)
	
	javaWorkspace := createTempWorkspace(t, "java")
	nonJavaWorkspace := createTempWorkspace(t, "generic")
	defer cleanupWorkspace(t, javaWorkspace)
	defer cleanupWorkspace(t, nonJavaWorkspace)
	
	// Test with Java workspace
	if !strategy.CanSwitch("", javaWorkspace) {
		t.Error("Expected Java strategy to handle Java workspace")
	}
	
	// Test with non-Java workspace
	if strategy.CanSwitch("", nonJavaWorkspace) {
		t.Error("Expected Java strategy to reject non-Java workspace")
	}
}

func TestJavaWorkspaceSwitchStrategy_ExecuteSwitch(t *testing.T) {
	mockClient := NewMockLSPClient()
	mockLogger := NewMockLogger()
	strategy := NewJavaWorkspaceSwitchStrategy(mockLogger)
	
	fromWorkspace := createTempWorkspace(t, "java")
	toWorkspace := createTempWorkspace(t, "java")
	defer cleanupWorkspace(t, fromWorkspace)
	defer cleanupWorkspace(t, toWorkspace)
	
	ctx := context.Background()
	err := strategy.ExecuteSwitch(ctx, mockClient, fromWorkspace, toWorkspace)
	
	if err != nil {
		t.Errorf("Expected successful execute switch, got error: %v", err)
	}
	
	// Verify the notification was sent
	notifications := mockClient.GetNotifications()
	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(notifications))
	} else {
		if notifications[0].Method != "workspace/didChangeWorkspaceFolders" {
			t.Errorf("Expected workspace/didChangeWorkspaceFolders notification, got %s", notifications[0].Method)
		}
	}
}

func TestTypeScriptWorkspaceSwitchStrategy_CanSwitch(t *testing.T) {
	mockLogger := NewMockLogger()
	strategy := NewTypeScriptWorkspaceSwitchStrategy(mockLogger)
	
	tsWorkspace := createTempWorkspace(t, "typescript")
	nonTsWorkspace := createTempWorkspace(t, "generic")
	defer cleanupWorkspace(t, tsWorkspace)
	defer cleanupWorkspace(t, nonTsWorkspace)
	
	// Test with TypeScript workspace
	if !strategy.CanSwitch("", tsWorkspace) {
		t.Error("Expected TypeScript strategy to handle TypeScript workspace")
	}
	
	// Test with non-TypeScript workspace
	if strategy.CanSwitch("", nonTsWorkspace) {
		t.Error("Expected TypeScript strategy to reject non-TypeScript workspace")
	}
}

func TestGoWorkspaceSwitchStrategy_CanSwitch(t *testing.T) {
	mockLogger := NewMockLogger()
	strategy := NewGoWorkspaceSwitchStrategy(mockLogger)
	
	goWorkspace := createTempWorkspace(t, "go")
	nonGoWorkspace := createTempWorkspace(t, "generic")
	defer cleanupWorkspace(t, goWorkspace)
	defer cleanupWorkspace(t, nonGoWorkspace)
	
	// Test with Go workspace
	if !strategy.CanSwitch("", goWorkspace) {
		t.Error("Expected Go strategy to handle Go workspace")
	}
	
	// Test with non-Go workspace
	if strategy.CanSwitch("", nonGoWorkspace) {
		t.Error("Expected Go strategy to reject non-Go workspace")
	}
}

func TestPythonWorkspaceSwitchStrategy_CanSwitch(t *testing.T) {
	mockLogger := NewMockLogger()
	strategy := NewPythonWorkspaceSwitchStrategy(mockLogger)
	
	pythonWorkspace := createTempWorkspace(t, "python")
	nonPythonWorkspace := createTempWorkspace(t, "generic")
	defer cleanupWorkspace(t, pythonWorkspace)
	defer cleanupWorkspace(t, nonPythonWorkspace)
	
	// Test with Python workspace
	if !strategy.CanSwitch("", pythonWorkspace) {
		t.Error("Expected Python strategy to handle Python workspace")
	}
	
	// Test with non-Python workspace
	if strategy.CanSwitch("", nonPythonWorkspace) {
		t.Error("Expected Python strategy to reject non-Python workspace")
	}
}

// Tests for WorkspaceValidator

func TestWorkspaceValidator_ValidateWorkspace(t *testing.T) {
	mockLogger := NewMockLogger()
	validator := NewWorkspaceValidator(mockLogger)
	
	// Test with valid workspace
	validWorkspace := createTempWorkspace(t, "java")
	defer cleanupWorkspace(t, validWorkspace)
	
	err := validator.ValidateWorkspace(validWorkspace)
	if err != nil {
		t.Errorf("Expected valid workspace to pass validation, got error: %v", err)
	}
	
	// Test with empty path
	err = validator.ValidateWorkspace("")
	if err == nil {
		t.Error("Expected error for empty workspace path")
	}
	
	// Test with non-existent path
	err = validator.ValidateWorkspace("/non/existent/path")
	if err == nil {
		t.Error("Expected error for non-existent workspace path")
	}
	
	// Test with file instead of directory
	tempFile, err := os.CreateTemp("", "not_a_dir")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()
	
	err = validator.ValidateWorkspace(tempFile.Name())
	if err == nil {
		t.Error("Expected error when workspace path is a file")
	}
}

func TestWorkspaceValidator_DetectProjectType(t *testing.T) {
	mockLogger := NewMockLogger()
	validator := NewWorkspaceValidator(mockLogger)
	
	testCases := []struct {
		projectType string
		expected    ProjectType
	}{
		{"java", ProjectTypeJava},
		{"typescript", ProjectTypeTypeScript},
		{"go", ProjectTypeGo},
		{"python", ProjectTypePython},
		{"generic", ProjectTypeUnknown},
	}
	
	for _, tc := range testCases {
		t.Run(string(tc.expected), func(t *testing.T) {
			workspace := createTempWorkspace(t, tc.projectType)
			defer cleanupWorkspace(t, workspace)
			
			detected := validator.detectProjectType(workspace)
			if detected != tc.expected {
				t.Errorf("Expected project type %s, got %s", tc.expected, detected)
			}
		})
	}
}

func TestWorkspaceValidator_GetWorkspaceInfo(t *testing.T) {
	mockLogger := NewMockLogger()
	validator := NewWorkspaceValidator(mockLogger)
	
	workspace := createTempWorkspace(t, "java")
	defer cleanupWorkspace(t, workspace)
	
	info, err := validator.GetWorkspaceInfo(workspace)
	if err != nil {
		t.Errorf("Expected successful workspace info retrieval, got error: %v", err)
	}
	
	if info == nil {
		t.Fatal("Expected workspace info to be returned")
	}
	
	if info.ProjectType != ProjectTypeJava {
		t.Errorf("Expected project type %s, got %s", ProjectTypeJava, info.ProjectType)
	}
	
	if info.Name != filepath.Base(workspace) {
		t.Errorf("Expected workspace name %s, got %s", filepath.Base(workspace), info.Name)
	}
	
	// Check that important Java files are detected
	if !info.ProjectFiles["pom.xml"] {
		t.Error("Expected pom.xml to be detected in project files")
	}
}

// Benchmark tests

func BenchmarkWorkspaceSwitcher_SwitchWorkspace(b *testing.B) {
	mockClient := NewMockLSPClient()
	mockLogger := NewMockLogger()
	switcher := NewWorkspaceSwitcher(mockClient, mockLogger)
	
	workspaces := make([]string, 10)
	for i := 0; i < 10; i++ {
		workspaces[i] = createTempWorkspace(b, "java")
		defer cleanupWorkspace(b, workspaces[i])
	}
	
	// Mock successful responses
	mockClient.SetResponse("workspace/symbol", json.RawMessage(`[]`))
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		workspace := workspaces[i%len(workspaces)]
		ctx := context.Background()
		
		err := switcher.SwitchWorkspace(ctx, workspace, "java")
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

// Helper function to check if string contains substring (for older Go versions)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 containsInternal(s, substr)))
}

func containsInternal(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}