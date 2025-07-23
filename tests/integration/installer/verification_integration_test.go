package installer

import (
	"testing"
	"time"
)

func TestVerificationStructuresWithoutExternalDeps(t *testing.T) {
	installer := NewRuntimeInstaller()
	if installer == nil {
		t.Fatal("NewRuntimeInstaller returned nil")
	}

	registry := NewRuntimeRegistry()
	if registry == nil {
		t.Fatal("NewRuntimeRegistry returned nil")
	}

	goRuntime, err := registry.GetRuntime("go")
	if err != nil {
		t.Fatalf("Failed to get Go runtime: %v", err)
	}

	if goRuntime.Name != "go" {
		t.Errorf("Expected runtime name 'go', got '%s'", goRuntime.Name)
	}

	if goRuntime.MinVersion != "1.19.0" {
		t.Errorf("Expected min version '1.19.0', got '%s'", goRuntime.MinVersion)
	}

	_, err = registry.GetRuntime("unknown")
	if err == nil {
		t.Error("Expected error for unknown runtime")
	}
}

func TestVerificationResultInitialization(t *testing.T) {
	result := &VerificationResult{
		Runtime:         "go",
		Installed:       false,
		Version:         "",
		Compatible:      false,
		Path:            "",
		Issues:          []Issue{},
		Recommendations: []string{},
		WorkingDir:      "",
		EnvironmentVars: make(map[string]string),
		AdditionalPaths: []string{},
		VerifiedAt:      time.Now(),
		Duration:        0,
		Metadata:        make(map[string]interface{}),
	}

	if result.Runtime != "go" {
		t.Error("Runtime not initialized correctly")
	}

	if result.EnvironmentVars == nil {
		t.Error("EnvironmentVars map not initialized")
	}

	if result.Metadata == nil {
		t.Error("Metadata map not initialized")
	}

	if len(result.Issues) != 0 {
		t.Error("Issues slice should be empty initially")
	}
}

func TestIssueCreation(t *testing.T) {
	issue := Issue{
		Severity:    IssueSeverityHigh,
		Category:    IssueCategoryInstallation,
		Title:       "Test Issue",
		Description: "This is a test issue",
		Solution:    "Fix the test issue",
		Details:     map[string]interface{}{"test": true},
	}

	if issue.Severity != IssueSeverityHigh {
		t.Error("Issue severity not set correctly")
	}

	if issue.Category != IssueCategoryInstallation {
		t.Error("Issue category not set correctly")
	}

	if issue.Details["test"] != true {
		t.Error("Issue details not set correctly")
	}
}

func TestErrorTypes(t *testing.T) {
	err := NewInstallerError(InstallerErrorTypeNotFound, "test", "test message", nil)
	if err.Type != InstallerErrorTypeNotFound {
		t.Error("InstallerError type not set correctly")
	}

	if err.Component != "test" {
		t.Error("InstallerError component not set correctly")
	}

	if err.Message != "test message" {
		t.Error("InstallerError message not set correctly")
	}

	errorString := err.Error()
	if errorString == "" {
		t.Error("InstallerError string should not be empty")
	}

	depErr := NewRuntimeDependencyError("go", "1.19.0", "1.18.0", false)
	if depErr.Runtime != "go" {
		t.Error("RuntimeDependencyError runtime not set correctly")
	}

	if depErr.Required != "1.19.0" {
		t.Error("RuntimeDependencyError required version not set correctly")
	}

	if depErr.Installed != "1.18.0" {
		t.Error("RuntimeDependencyError installed version not set correctly")
	}

	depErrorString := depErr.Error()
	if depErrorString == "" {
		t.Error("RuntimeDependencyError string should not be empty")
	}
}

func TestPlatformStrategies(t *testing.T) {
	installer := NewRuntimeInstaller()

	windowsStrategy := installer.GetPlatformStrategy("windows")
	if windowsStrategy == nil {
		t.Error("Windows strategy should not be nil")
	}

	linuxStrategy := installer.GetPlatformStrategy("linux")
	if linuxStrategy == nil {
		t.Error("Linux strategy should not be nil")
	}

	macosStrategy := installer.GetPlatformStrategy("darwin")
	if macosStrategy == nil {
		t.Error("macOS strategy should not be nil")
	}

	unknownStrategy := installer.GetPlatformStrategy("unknown")
	if unknownStrategy == nil {
		t.Error("Unknown platform strategy should default to linux strategy")
	}
}

func TestRuntimeRegistry(t *testing.T) {
	registry := NewRuntimeRegistry()

	runtimes := registry.ListRuntimes()
	expectedRuntimes := []string{"go", "python", "nodejs", "java"}

	if len(runtimes) != len(expectedRuntimes) {
		t.Errorf("Expected %d runtimes, got %d", len(expectedRuntimes), len(runtimes))
	}

	for _, expected := range expectedRuntimes {
		found := false
		for _, runtime := range runtimes {
			if runtime == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected runtime %s not found in registry", expected)
		}
	}

	newRuntime := &RuntimeDefinition{
		Name:               "test",
		DisplayName:        "Test Runtime",
		MinVersion:         "1.0.0",
		RecommendedVersion: "2.0.0",
		VersionCommand:     []string{"test", "--version"},
		InstallMethods:     make(map[string]InstallMethod),
		Dependencies:       []string{},
		PostInstall:        []string{},
	}

	registry.RegisterRuntime(newRuntime)

	retrievedRuntime, err := registry.GetRuntime("test")
	if err != nil {
		t.Fatalf("Failed to retrieve registered runtime: %v", err)
	}

	if retrievedRuntime.Name != "test" {
		t.Error("Retrieved runtime name doesn't match")
	}

	if retrievedRuntime.DisplayName != "Test Runtime" {
		t.Error("Retrieved runtime display name doesn't match")
	}
}

func TestMockVerification(t *testing.T) {
	installer := NewRuntimeInstaller()

	result, err := installer.Verify("unsupported")
	if err == nil {
		t.Error("Expected error for unsupported runtime")
	}

	if result != nil {
		t.Error("Expected nil result for unsupported runtime")
	}

	if installerErr, ok := err.(*InstallerError); ok {
		if installerErr.Type != InstallerErrorTypeNotFound {
			t.Errorf("Expected error type %s, got %s", InstallerErrorTypeNotFound, installerErr.Type)
		}
	} else {
		t.Error("Expected InstallerError type")
	}
}

func TestInstallResult(t *testing.T) {
	result := &InstallResult{
		Success:  true,
		Runtime:  "go",
		Version:  "1.21.0",
		Path:     "/usr/bin/go",
		Duration: time.Second,
		Method:   "package_manager",
		Messages: []string{"Installation successful"},
		Warnings: []string{},
		Errors:   []string{},
	}

	if !result.Success {
		t.Error("InstallResult success not set correctly")
	}

	if result.Runtime != "go" {
		t.Error("InstallResult runtime not set correctly")
	}

	if len(result.Messages) != 1 {
		t.Error("InstallResult messages not set correctly")
	}
}

func TestInstallOptions(t *testing.T) {
	options := InstallOptions{
		Version:        "1.21.0",
		Force:          true,
		SkipVerify:     false,
		Timeout:        30 * time.Second,
		PackageManager: "apt",
		Platform:       "linux",
	}

	if options.Version != "1.21.0" {
		t.Error("InstallOptions version not set correctly")
	}

	if !options.Force {
		t.Error("InstallOptions force not set correctly")
	}

	if options.Timeout != 30*time.Second {
		t.Error("InstallOptions timeout not set correctly")
	}
}

func BenchmarkRegistryGetRuntime(b *testing.B) {
	registry := NewRuntimeRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = registry.GetRuntime("go")
	}
}

func BenchmarkRegistryListRuntimes(b *testing.B) {
	registry := NewRuntimeRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = registry.ListRuntimes()
	}
}

func BenchmarkInstallerCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		installer := NewRuntimeInstaller()
		_ = installer
	}
}
