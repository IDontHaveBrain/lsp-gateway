package installer_test

import (
	"lsp-gateway/internal/installer"
	"testing"
	"time"

	"lsp-gateway/internal/types"
)

func TestNewRuntimeInstaller(t *testing.T) {
	installer := NewRuntimeInstaller()
	if installer == nil {
		t.Fatal("NewRuntimeInstaller returned nil")
	}

	if installer.registry == nil {
		t.Error("Registry is nil")
	}

	if installer.strategies == nil {
		t.Error("Strategies map is nil")
	}

	runtimes := installer.GetSupportedRuntimes()
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
			t.Errorf("Expected runtime %s not found", expected)
		}
	}
}

func TestGetRuntimeInfo(t *testing.T) {
	installer := NewRuntimeInstaller()

	testCases := []struct {
		runtime     string
		shouldExist bool
	}{
		{"go", true},
		{"python", true},
		{"nodejs", true},
		{"java", true},
		{"unknown", false},
	}

	for _, tc := range testCases {
		t.Run(tc.runtime, func(t *testing.T) {
			info, err := installer.GetRuntimeInfo(tc.runtime)

			if tc.shouldExist {
				if err != nil {
					t.Errorf("Expected runtime %s to exist, got error: %v", tc.runtime, err)
				}
				if info == nil {
					t.Errorf("Expected runtime info for %s, got nil", tc.runtime)
				}
				if info != nil && info.Name != tc.runtime {
					t.Errorf("Expected runtime name %s, got %s", tc.runtime, info.Name)
				}
			} else {
				if err == nil {
					t.Errorf("Expected error for unknown runtime %s", tc.runtime)
				}
				if info != nil {
					t.Errorf("Expected nil info for unknown runtime %s", tc.runtime)
				}
			}
		})
	}
}

func TestRuntimeDefinitions(t *testing.T) {
	installer := NewRuntimeInstaller()

	testCases := []struct {
		runtime             string
		expectedMinVersion  string
		expectedDisplayName string
	}{
		{"go", "1.19.0", "Go Programming Language"},
		{"python", "3.8.0", "Python Programming Language"},
		{"nodejs", "18.0.0", "Node.js JavaScript Runtime"},
		{"java", "17.0.0", "Java Development Kit"},
	}

	for _, tc := range testCases {
		t.Run(tc.runtime, func(t *testing.T) {
			info, err := installer.GetRuntimeInfo(tc.runtime)
			if err != nil {
				t.Fatalf("Failed to get runtime info for %s: %v", tc.runtime, err)
			}

			if info.MinVersion != tc.expectedMinVersion {
				t.Errorf("Expected min version %s for %s, got %s",
					tc.expectedMinVersion, tc.runtime, info.MinVersion)
			}

			if info.DisplayName != tc.expectedDisplayName {
				t.Errorf("Expected display name %s for %s, got %s",
					tc.expectedDisplayName, tc.runtime, info.DisplayName)
			}

			if len(info.VersionCommand) == 0 {
				t.Errorf("Expected version command for %s", tc.runtime)
			}
		})
	}
}

func TestVerificationResultStructure(t *testing.T) {
	result := &VerificationResult{
		Runtime:         "test",
		Installed:       true,
		Version:         "1.0.0",
		Compatible:      true,
		Path:            "/usr/bin/test",
		Issues:          []Issue{},
		Recommendations: []string{},
		WorkingDir:      "/tmp",
		EnvironmentVars: make(map[string]string),
		AdditionalPaths: []string{},
		VerifiedAt:      time.Now(),
		Duration:        time.Second,
		Metadata:        make(map[string]interface{}),
	}

	if result.Runtime != "test" {
		t.Error("Runtime field not set correctly")
	}

	if !result.Installed {
		t.Error("Installed field not set correctly")
	}

	if result.EnvironmentVars == nil {
		t.Error("EnvironmentVars map is nil")
	}

	if result.Metadata == nil {
		t.Error("Metadata map is nil")
	}
}

func TestIssueStructure(t *testing.T) {
	issue := Issue{
		Severity:    IssueSeverityHigh,
		Category:    IssueCategoryInstallation,
		Title:       "Test Issue",
		Description: "Test Description",
		Solution:    "Test Solution",
		Details:     make(map[string]interface{}),
	}

	if issue.Severity != IssueSeverityHigh {
		t.Error("Severity not set correctly")
	}

	if issue.Category != IssueCategoryInstallation {
		t.Error("Category not set correctly")
	}

	if issue.Details == nil {
		t.Error("Details map is nil")
	}
}

func TestIssueSeverityConstants(t *testing.T) {
	severities := []IssueSeverity{
		IssueSeverityCritical,
		IssueSeverityHigh,
		IssueSeverityMedium,
		IssueSeverityLow,
		IssueSeverityInfo,
	}

	expectedSeverities := []string{
		"critical",
		"high",
		"medium",
		"low",
		"info",
	}

	for i, severity := range severities {
		if string(severity) != expectedSeverities[i] {
			t.Errorf("Expected severity %s, got %s", expectedSeverities[i], string(severity))
		}
	}
}

func TestIssueCategoryConstants(t *testing.T) {
	categories := []types.IssueCategory{
		types.IssueCategoryInstallation,
		types.IssueCategoryVersion,
		types.IssueCategoryPath,
		types.IssueCategoryEnvironment,
		types.IssueCategoryPermissions,
		types.IssueCategoryDependencies,
		types.IssueCategoryConfiguration,
		types.IssueCategoryCorruption,
		types.IssueCategoryExecution,
	}

	expectedCategories := []string{
		"installation",
		"version",
		"path",
		"environment",
		"permissions",
		"dependencies",
		"configuration",
		"corruption",
		"execution",
	}

	for i, category := range categories {
		if string(category) != expectedCategories[i] {
			t.Errorf("Expected category %s, got %s", expectedCategories[i], string(category))
		}
	}
}

func TestVerifyUnsupportedRuntime(t *testing.T) {
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

func TestAddIssueHelper(t *testing.T) {
	installer := NewRuntimeInstaller()
	result := &VerificationResult{
		Issues: []Issue{},
	}

	installer.addIssue(result, IssueSeverityHigh, IssueCategoryInstallation,
		"Test Title", "Test Description", "Test Solution",
		map[string]interface{}{"key": "value"})

	if len(result.Issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(result.Issues))
	}

	issue := result.Issues[0]
	if issue.Severity != IssueSeverityHigh {
		t.Error("Issue severity not set correctly")
	}

	if issue.Category != IssueCategoryInstallation {
		t.Error("Issue category not set correctly")
	}

	if issue.Title != "Test Title" {
		t.Error("Issue title not set correctly")
	}

	if issue.Details["key"] != "value" {
		t.Error("Issue details not set correctly")
	}
}

func BenchmarkNewRuntimeInstaller(b *testing.B) {
	for i := 0; i < b.N; i++ {
		installer := NewRuntimeInstaller()
		_ = installer
	}
}

func BenchmarkGetRuntimeInfo(b *testing.B) {
	installer := NewRuntimeInstaller()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = installer.GetRuntimeInfo("go")
	}
}

func BenchmarkGetSupportedRuntimes(b *testing.B) {
	installer := NewRuntimeInstaller()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = installer.GetSupportedRuntimes()
	}
}
