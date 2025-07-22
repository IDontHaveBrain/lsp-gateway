package installer

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
)

// MockCommandExecutor implements platform.CommandExecutor for testing
type MockCommandExecutor struct {
	mu       sync.RWMutex
	commands map[string]*platform.Result
	failures map[string]error
	timeouts map[string]bool
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		commands: make(map[string]*platform.Result),
		failures: make(map[string]error),
		timeouts: make(map[string]bool),
	}
}

func (m *MockCommandExecutor) Execute(cmd string, args []string, timeout time.Duration) (*platform.Result, error) {
	return m.ExecuteWithEnv(cmd, args, nil, timeout)
}

func (m *MockCommandExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s %v", cmd, args)

	// Check for timeout scenario
	if m.timeouts[key] {
		return &platform.Result{
			ExitCode: -1,
			Stderr:   "command timed out",
			Duration: timeout,
		}, fmt.Errorf("command timed out after %v", timeout)
	}

	// Check for failures
	if err, exists := m.failures[key]; exists {
		return &platform.Result{
			ExitCode: 1,
			Stderr:   err.Error(),
			Duration: 10 * time.Millisecond,
		}, err
	}

	// Check for mocked results
	if result, exists := m.commands[key]; exists {
		return result, nil
	}

	// Default failure case
	return &platform.Result{
		ExitCode: 127,
		Stderr:   "command not found",
		Duration: 10 * time.Millisecond,
	}, fmt.Errorf("command not found: %s", cmd)
}

func (m *MockCommandExecutor) IsCommandAvailable(command string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := fmt.Sprintf("%s []", command)
	_, exists := m.commands[key]
	return exists
}

func (m *MockCommandExecutor) AddCommand(cmd string, args []string, result *platform.Result) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s %v", cmd, args)
	m.commands[key] = result
}

func (m *MockCommandExecutor) AddFailure(cmd string, args []string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s %v", cmd, args)
	m.failures[key] = err
}

// Helper function to create a test installer
func createTestInstaller() *DefaultRuntimeInstaller {
	return &DefaultRuntimeInstaller{
		registry: NewRuntimeRegistry(),
	}
}

// Test Go Environment Verification - Environment Variable Handling
func TestVerifyGoEnvironment_EnvironmentVariableHandling(t *testing.T) {
	// Test valid GOPATH and GOROOT
	t.Setenv("GOPATH", "/tmp") // Use /tmp as it should exist
	t.Setenv("GOROOT", "/usr") // Use /usr as it should exist

	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	installer.verifyGoEnvironment(result)

	// Check that environment variables are captured
	if result.EnvironmentVars["GOPATH"] != "/tmp" {
		t.Errorf("Expected GOPATH to be set to '/tmp', got '%s'", result.EnvironmentVars["GOPATH"])
	}

	if result.EnvironmentVars["GOROOT"] != "/usr" {
		t.Errorf("Expected GOROOT to be set to '/usr', got '%s'", result.EnvironmentVars["GOROOT"])
	}
}

func TestVerifyGoEnvironment_InvalidGOPATH(t *testing.T) {
	t.Setenv("GOPATH", "/nonexistent/path/that/should/not/exist")

	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	installer.verifyGoEnvironment(result)

	foundInvalidGOPATH := false
	for _, issue := range result.Issues {
		if issue.Title == "Invalid GOPATH" {
			foundInvalidGOPATH = true
			if issue.Severity != types.IssueSeverityMedium {
				t.Errorf("Expected medium severity for invalid GOPATH, got %s", issue.Severity)
			}
			if issue.Category != types.IssueCategoryEnvironment {
				t.Errorf("Expected environment category for invalid GOPATH, got %s", issue.Category)
			}
		}
	}

	if !foundInvalidGOPATH {
		t.Error("Expected invalid GOPATH issue to be reported")
	}
}

func TestVerifyGoEnvironment_InvalidGOROOT(t *testing.T) {
	t.Setenv("GOROOT", "/nonexistent/go/root")

	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	installer.verifyGoEnvironment(result)

	foundInvalidGOROOT := false
	for _, issue := range result.Issues {
		if issue.Title == "Invalid GOROOT" {
			foundInvalidGOROOT = true
			if issue.Severity != types.IssueSeverityMedium {
				t.Errorf("Expected medium severity for invalid GOROOT, got %s", issue.Severity)
			}
		}
	}

	if !foundInvalidGOROOT {
		t.Error("Expected invalid GOROOT issue to be reported")
	}
}

// Test parseGoEnv function directly
func TestParseGoEnv(t *testing.T) {
	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	testOutput := `GOARCH="amd64"
GOOS="linux"
GOPATH="/home/user/go"
GOROOT="/usr/local/go"
GOVERSION="go1.21.0"
GOCACHE="/home/user/.cache/go-build"`

	installer.parseGoEnv(result, testOutput)

	goEnv, exists := result.Metadata["go_env"].(map[string]string)
	if !exists {
		t.Fatal("Expected go_env metadata to be set")
	}

	expectedValues := map[string]string{
		"GOARCH":    "amd64",
		"GOOS":      "linux",
		"GOPATH":    "/home/user/go",
		"GOROOT":    "/usr/local/go",
		"GOVERSION": "go1.21.0",
		"GOCACHE":   "/home/user/.cache/go-build",
	}

	for key, expectedValue := range expectedValues {
		if goEnv[key] != expectedValue {
			t.Errorf("Expected %s to be '%s', got '%s'", key, expectedValue, goEnv[key])
		}
	}

	// Check metadata extraction
	if result.Metadata["go_os"] != "linux" {
		t.Errorf("Expected go_os to be 'linux', got '%v'", result.Metadata["go_os"])
	}

	if result.Metadata["go_arch"] != "amd64" {
		t.Errorf("Expected go_arch to be 'amd64', got '%v'", result.Metadata["go_arch"])
	}

	if result.Metadata["go_version_env"] != "go1.21.0" {
		t.Errorf("Expected go_version_env to be 'go1.21.0', got '%v'", result.Metadata["go_version_env"])
	}
}

func TestParseGoEnv_MalformedOutput(t *testing.T) {
	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	// Test malformed output
	malformedOutput := `malformed=output
no_equals_sign
=empty_key
valid_key=valid_value
GOARCH="amd64"
another_line_without_equals`

	installer.parseGoEnv(result, malformedOutput)

	goEnv, exists := result.Metadata["go_env"].(map[string]string)
	if !exists {
		t.Fatal("Expected go_env metadata to be set")
	}

	// Should parse valid entries and ignore malformed ones
	if goEnv["valid_key"] != "valid_value" {
		t.Error("Expected valid key-value pairs to be parsed correctly")
	}

	if goEnv["GOARCH"] != "amd64" {
		t.Error("Expected quoted values to be parsed correctly")
	}

	// Malformed entries should be ignored
	if goEnv["no_equals_sign"] != "" {
		t.Error("Expected malformed entries to be ignored")
	}
}

func TestParseGoEnv_EmptyOutput(t *testing.T) {
	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	installer.parseGoEnv(result, "")

	goEnv, exists := result.Metadata["go_env"].(map[string]string)
	if !exists {
		t.Fatal("Expected go_env metadata to be set")
	}

	if len(goEnv) != 0 {
		t.Errorf("Expected empty go_env for empty output, got %v", goEnv)
	}
}

// Test Java Environment Verification - Environment Variable Handling
func TestVerifyJavaEnvironment_ValidJavaHome(t *testing.T) {
	t.Setenv("JAVA_HOME", "/tmp") // Use /tmp as it should exist

	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	installer.verifyJavaEnvironment(result)

	// Check that JAVA_HOME is captured
	if result.EnvironmentVars[ENV_VAR_JAVA_HOME] != "/tmp" {
		t.Errorf("Expected JAVA_HOME to be set to '/tmp', got '%s'", result.EnvironmentVars[ENV_VAR_JAVA_HOME])
	}
}

func TestVerifyJavaEnvironment_InvalidJavaHome(t *testing.T) {
	t.Setenv("JAVA_HOME", "/nonexistent/java/home")

	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	installer.verifyJavaEnvironment(result)

	foundInvalidJavaHome := false
	for _, issue := range result.Issues {
		if issue.Title == "Invalid JAVA_HOME" {
			foundInvalidJavaHome = true
			if issue.Severity != types.IssueSeverityMedium {
				t.Errorf("Expected medium severity for invalid JAVA_HOME, got %s", issue.Severity)
			}
			if issue.Category != types.IssueCategoryEnvironment {
				t.Errorf("Expected environment category for invalid JAVA_HOME, got %s", issue.Category)
			}
		}
	}

	if !foundInvalidJavaHome {
		t.Error("Expected invalid JAVA_HOME issue to be reported")
	}
}

func TestVerifyJavaEnvironment_NoJavaHome(t *testing.T) {
	// Ensure JAVA_HOME is not set by using empty value
	t.Setenv("JAVA_HOME", "")

	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	installer.verifyJavaEnvironment(result)

	foundJavaHomeNotSet := false
	for _, issue := range result.Issues {
		if issue.Title == "JAVA_HOME Not Set" {
			foundJavaHomeNotSet = true
			if issue.Severity != types.IssueSeverityLow {
				t.Errorf("Expected low severity for JAVA_HOME not set, got %s", issue.Severity)
			}
			if issue.Category != types.IssueCategoryEnvironment {
				t.Errorf("Expected environment category for JAVA_HOME not set, got %s", issue.Category)
			}
		}
	}

	if !foundJavaHomeNotSet {
		t.Error("Expected JAVA_HOME not set issue to be reported")
	}
}

// Test parseJavaSystemInfo function directly
func TestParseJavaSystemInfo(t *testing.T) {
	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	testOutput := `Property settings:
    java.version = 17.0.2
    java.home = /usr/lib/jvm/java-17-openjdk
    java.vendor = Eclipse Adoptium
    java.vm.version = 17.0.2+8
    java.vm.vendor = Eclipse Adoptium
java version "17.0.2" 2022-01-18 LTS`

	installer.parseJavaSystemInfo(result, testOutput)

	expectedValues := map[string]string{
		"java_version_property": "17.0.2",
		"java_home_property":    "/usr/lib/jvm/java-17-openjdk",
		"java_vendor":           "Eclipse Adoptium",
	}

	for key, expectedValue := range expectedValues {
		if result.Metadata[key] != expectedValue {
			t.Errorf("Expected %s to be '%s', got '%v'", key, expectedValue, result.Metadata[key])
		}
	}
}

func TestParseJavaSystemInfo_MalformedOutput(t *testing.T) {
	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	malformedOutput := `Property settings:
    java.version 17.0.2
    = empty_key
    java.home = /usr/lib/jvm/java-17-openjdk
    not_a_java_property = some_value
    java.vendor = Oracle Corporation`

	installer.parseJavaSystemInfo(result, malformedOutput)

	// Should parse valid Java properties and ignore malformed ones
	if result.Metadata["java_home_property"] != "/usr/lib/jvm/java-17-openjdk" {
		t.Error("Expected valid Java properties to be parsed correctly")
	}

	if result.Metadata["java_vendor"] != "Oracle Corporation" {
		t.Error("Expected valid Java vendor to be parsed correctly")
	}

	// Malformed entries should be ignored
	if result.Metadata["java_version_property"] != nil {
		t.Error("Expected malformed entries to be ignored")
	}
}

// Test versionsMatch function directly
func TestVersionsMatch(t *testing.T) {
	installer := createTestInstaller()

	testCases := []struct {
		version1 string
		version2 string
		expected bool
		name     string
	}{
		{"17.0.2", "17.0.1", true, "same major.minor, different patch"},
		{"17.0.2", "17.0.2", true, "identical versions"},
		{"17.1.0", "17.2.0", false, "different minor versions"},
		{"17.0.0", "18.0.0", false, "different major versions"},
		{"javac 17.0.2", "java version \"17.0.1\"", true, "versions with extra text"},
		{"invalid", "17.0.2", false, "invalid version format"},
		{"17.0.2", "invalid", false, "invalid second version"},
		{"1.8.0", "1.8.1", true, "Java 8 style versioning"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := installer.versionsMatch(tc.version1, tc.version2)
			if result != tc.expected {
				t.Errorf("versionsMatch(%q, %q) = %v; expected %v", tc.version1, tc.version2, result, tc.expected)
			}
		})
	}
}

// Test Python Environment Verification - Environment Variable Handling
func TestVerifyPythonEnvironment_PythonPath(t *testing.T) {
	t.Setenv("PYTHONPATH", "/usr/local/lib/python3.9/site-packages")

	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	installer.verifyPythonEnvironment(result)

	// Check that PYTHONPATH is captured when Python command is available
	if result.EnvironmentVars["PYTHONPATH"] != "/usr/local/lib/python3.9/site-packages" {
		t.Errorf("Expected PYTHONPATH to be set, got '%s'", result.EnvironmentVars["PYTHONPATH"])
	}

	if result.Metadata["python_path"] != "/usr/local/lib/python3.9/site-packages" {
		t.Errorf("Expected python_path metadata to be set, got '%v'", result.Metadata["python_path"])
	}
}

// Test Node.js Environment Verification - Environment Variable Handling
func TestVerifyNodejsEnvironment_NodePath(t *testing.T) {
	t.Setenv("NODE_PATH", "/usr/local/lib/node_modules")

	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	installer.verifyNodejsEnvironment(result)

	// Check that NODE_PATH is captured
	if result.EnvironmentVars["NODE_PATH"] != "/usr/local/lib/node_modules" {
		t.Errorf("Expected NODE_PATH to be set, got '%s'", result.EnvironmentVars["NODE_PATH"])
	}

	if result.Metadata["node_path"] != "/usr/local/lib/node_modules" {
		t.Errorf("Expected node_path metadata to be set, got '%v'", result.Metadata["node_path"])
	}
}

// Test Issue Creation Helper for Runtime Verification
func TestRuntimeVerification_AddIssueHelper(t *testing.T) {
	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues: []types.Issue{},
	}

	installer.addIssue(result, types.IssueSeverityHigh, types.IssueCategoryInstallation,
		"Test Title", "Test Description", "Test Solution",
		map[string]interface{}{"key": "value"})

	if len(result.Issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.Severity != types.IssueSeverityHigh {
		t.Error("Issue severity not set correctly")
	}

	if issue.Category != types.IssueCategoryInstallation {
		t.Error("Issue category not set correctly")
	}

	if issue.Title != "Test Title" {
		t.Error("Issue title not set correctly")
	}

	if issue.Description != "Test Description" {
		t.Error("Issue description not set correctly")
	}

	if issue.Solution != "Test Solution" {
		t.Error("Issue solution not set correctly")
	}

	if issue.Details["key"] != "value" {
		t.Error("Issue details not set correctly")
	}
}

// Test Multiple Issues
func TestAddMultipleIssues(t *testing.T) {
	installer := createTestInstaller()
	result := &types.VerificationResult{
		Issues: []types.Issue{},
	}

	// Add multiple issues with different severities
	installer.addIssue(result, types.IssueSeverityCritical, types.IssueCategoryInstallation,
		"Critical Issue", "Critical Description", "Critical Solution", nil)

	installer.addIssue(result, types.IssueSeverityLow, types.IssueCategoryEnvironment,
		"Low Issue", "Low Description", "Low Solution", nil)

	if len(result.Issues) != 2 {
		t.Errorf("Expected 2 issues, got %d", len(result.Issues))
	}

	// Check that issues are added in order
	if result.Issues[0].Severity != types.IssueSeverityCritical {
		t.Error("First issue should be critical")
	}

	if result.Issues[1].Severity != types.IssueSeverityLow {
		t.Error("Second issue should be low")
	}
}

// Test Result Structure Initialization for Runtime Verification
func TestRuntimeVerification_VerificationResultStructure(t *testing.T) {
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	if result.Issues == nil {
		t.Error("Issues slice should be initialized")
	}

	if result.Details == nil {
		t.Error("Details map should be initialized")
	}

	if result.Metadata == nil {
		t.Error("Metadata map should be initialized")
	}

	if result.EnvironmentVars == nil {
		t.Error("EnvironmentVars map should be initialized")
	}

	// Test that we can add to all collections
	result.Issues = append(result.Issues, types.Issue{Title: "test"})
	result.Details["test"] = "value"
	result.Metadata["test"] = "value"
	result.EnvironmentVars["TEST"] = "value"

	if len(result.Issues) != 1 {
		t.Error("Should be able to add issues")
	}

	if result.Details["test"] != "value" {
		t.Error("Should be able to add details")
	}

	if result.Metadata["test"] != "value" {
		t.Error("Should be able to add metadata")
	}

	if result.EnvironmentVars["TEST"] != "value" {
		t.Error("Should be able to add environment variables")
	}
}

// Test MockCommandExecutor Thread Safety
func TestMockCommandExecutor_Concurrency(t *testing.T) {
	executor := NewMockCommandExecutor()
	executor.AddCommand("test", []string{"arg"}, &platform.Result{
		ExitCode: 0,
		Stdout:   "success",
		Duration: 1 * time.Millisecond,
	})

	// Test concurrent access
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			result, err := executor.Execute("test", []string{"arg"}, 5*time.Second)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result.ExitCode != 0 {
				t.Errorf("Expected exit code 0, got %d", result.ExitCode)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Benchmark tests
func BenchmarkParseGoEnv(b *testing.B) {
	installer := createTestInstaller()
	testOutput := `GOARCH="amd64"
GOOS="linux"
GOPATH="/home/user/go"
GOROOT="/usr/local/go"
GOVERSION="go1.21.0"
GOCACHE="/home/user/.cache/go-build"
GOMODCACHE="/home/user/go/pkg/mod"`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &types.VerificationResult{
			Issues:          []types.Issue{},
			Details:         make(map[string]interface{}),
			Metadata:        make(map[string]interface{}),
			EnvironmentVars: make(map[string]string),
		}
		installer.parseGoEnv(result, testOutput)
	}
}

func BenchmarkParseJavaSystemInfo(b *testing.B) {
	installer := createTestInstaller()
	testOutput := `Property settings:
    java.version = 17.0.2
    java.home = /usr/lib/jvm/java-17-openjdk
    java.vendor = Eclipse Adoptium
    java.vm.version = 17.0.2+8
    java.vm.vendor = Eclipse Adoptium`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &types.VerificationResult{
			Issues:          []types.Issue{},
			Details:         make(map[string]interface{}),
			Metadata:        make(map[string]interface{}),
			EnvironmentVars: make(map[string]string),
		}
		installer.parseJavaSystemInfo(result, testOutput)
	}
}

func BenchmarkVersionsMatch(b *testing.B) {
	installer := createTestInstaller()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		installer.versionsMatch("17.0.2", "17.0.1")
	}
}

func BenchmarkAddIssue(b *testing.B) {
	installer := createTestInstaller()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &types.VerificationResult{
			Issues: []types.Issue{},
		}
		installer.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryEnvironment,
			"Test Issue", "Test Description", "Test Solution",
			map[string]interface{}{"iteration": i})
	}
}
