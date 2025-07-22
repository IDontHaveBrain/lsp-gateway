package types

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestInstallOptions tests the InstallOptions struct
func TestInstallOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		options  InstallOptions
		expected InstallOptions
	}{
		{
			name: "default values",
			options: InstallOptions{
				Version:        "1.0.0",
				Force:          false,
				SkipVerify:     false,
				Timeout:        30 * time.Second,
				PackageManager: "npm",
				Platform:       "linux",
			},
			expected: InstallOptions{
				Version:        "1.0.0",
				Force:          false,
				SkipVerify:     false,
				Timeout:        30 * time.Second,
				PackageManager: "npm",
				Platform:       "linux",
			},
		},
		{
			name: "with force and skip verify",
			options: InstallOptions{
				Version:        "latest",
				Force:          true,
				SkipVerify:     true,
				Timeout:        60 * time.Second,
				PackageManager: "pip",
				Platform:       "windows",
			},
			expected: InstallOptions{
				Version:        "latest",
				Force:          true,
				SkipVerify:     true,
				Timeout:        60 * time.Second,
				PackageManager: "pip",
				Platform:       "windows",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.options.Version != tt.expected.Version {
				t.Errorf("Expected Version %s, got %s", tt.expected.Version, tt.options.Version)
			}
			if tt.options.Force != tt.expected.Force {
				t.Errorf("Expected Force %v, got %v", tt.expected.Force, tt.options.Force)
			}
			if tt.options.SkipVerify != tt.expected.SkipVerify {
				t.Errorf("Expected SkipVerify %v, got %v", tt.expected.SkipVerify, tt.options.SkipVerify)
			}
			if tt.options.Timeout != tt.expected.Timeout {
				t.Errorf("Expected Timeout %v, got %v", tt.expected.Timeout, tt.options.Timeout)
			}
			if tt.options.PackageManager != tt.expected.PackageManager {
				t.Errorf("Expected PackageManager %s, got %s", tt.expected.PackageManager, tt.options.PackageManager)
			}
			if tt.options.Platform != tt.expected.Platform {
				t.Errorf("Expected Platform %s, got %s", tt.expected.Platform, tt.options.Platform)
			}
		})
	}
}

// TestServerInstallOptions tests the ServerInstallOptions struct
func TestServerInstallOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		options  ServerInstallOptions
		expected ServerInstallOptions
	}{
		{
			name: "basic options",
			options: ServerInstallOptions{
				Version:             "2.0.0",
				Force:               false,
				SkipVerify:          false,
				SkipDependencyCheck: false,
				Timeout:             45 * time.Second,
				Platform:            "darwin",
				InstallMethod:       "homebrew",
				WorkingDir:          "/tmp",
			},
			expected: ServerInstallOptions{
				Version:             "2.0.0",
				Force:               false,
				SkipVerify:          false,
				SkipDependencyCheck: false,
				Timeout:             45 * time.Second,
				Platform:            "darwin",
				InstallMethod:       "homebrew",
				WorkingDir:          "/tmp",
			},
		},
		{
			name: "skip options enabled",
			options: ServerInstallOptions{
				Version:             "3.0.0",
				Force:               true,
				SkipVerify:          true,
				SkipDependencyCheck: true,
				Timeout:             120 * time.Second,
				Platform:            "linux",
				InstallMethod:       "apt",
				WorkingDir:          "/home/user",
			},
			expected: ServerInstallOptions{
				Version:             "3.0.0",
				Force:               true,
				SkipVerify:          true,
				SkipDependencyCheck: true,
				Timeout:             120 * time.Second,
				Platform:            "linux",
				InstallMethod:       "apt",
				WorkingDir:          "/home/user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.options, tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, tt.options)
			}
		})
	}
}

// TestInstallResult tests the InstallResult struct
func TestInstallResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result InstallResult
	}{
		{
			name: "successful installation",
			result: InstallResult{
				Success:  true,
				Runtime:  "go",
				Version:  "1.21.0",
				Path:     "/usr/local/go/bin/go",
				Method:   "download",
				Duration: 30 * time.Second,
				Errors:   []string{},
				Warnings: []string{"deprecated flag used"},
				Messages: []string{"installation completed"},
				Details:  map[string]interface{}{"size": "120MB"},
			},
		},
		{
			name: "failed installation",
			result: InstallResult{
				Success:  false,
				Runtime:  "python",
				Version:  "3.11.0",
				Path:     "",
				Method:   "package-manager",
				Duration: 10 * time.Second,
				Errors:   []string{"permission denied", "disk full"},
				Warnings: []string{},
				Messages: []string{"installation failed"},
				Details:  map[string]interface{}{"error_code": 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.result)
			if err != nil {
				t.Errorf("Failed to marshal InstallResult: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled InstallResult
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal InstallResult: %v", err)
			}

			// Verify specific fields
			if unmarshaled.Success != tt.result.Success {
				t.Errorf("Expected Success %v, got %v", tt.result.Success, unmarshaled.Success)
			}
			if unmarshaled.Runtime != tt.result.Runtime {
				t.Errorf("Expected Runtime %s, got %s", tt.result.Runtime, unmarshaled.Runtime)
			}
			if len(unmarshaled.Errors) != len(tt.result.Errors) {
				t.Errorf("Expected %d errors, got %d", len(tt.result.Errors), len(unmarshaled.Errors))
			}
		})
	}
}

// TestVerificationResult tests the VerificationResult struct
func TestVerificationResult(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tests := []struct {
		name   string
		result VerificationResult
	}{
		{
			name: "fully verified",
			result: VerificationResult{
				Installed:       true,
				Compatible:      true,
				Version:         "1.21.0",
				Path:            "/usr/local/go/bin/go",
				Runtime:         "go",
				Issues:          []Issue{},
				Details:         map[string]interface{}{"arch": "amd64"},
				Metadata:        map[string]interface{}{"build": "official"},
				EnvironmentVars: map[string]string{"GOROOT": "/usr/local/go"},
				Recommendations: []string{"consider upgrading to 1.22"},
				WorkingDir:      "/home/user/project",
				AdditionalPaths: []string{"/usr/local/go/bin"},
				VerifiedAt:      now,
				Duration:        5 * time.Second,
			},
		},
		{
			name: "verification with issues",
			result: VerificationResult{
				Installed:  true,
				Compatible: false,
				Version:    "1.19.0",
				Path:       "/usr/bin/go",
				Runtime:    "go",
				Issues: []Issue{
					{
						Severity:    IssueSeverityHigh,
						Category:    IssueCategoryVersion,
						Title:       "Version too old",
						Description: "Go version 1.19.0 is below minimum required 1.21.0",
						Solution:    "Upgrade to Go 1.21 or newer",
						Details:     map[string]interface{}{"min_version": "1.21.0"},
					},
				},
				Details:         map[string]interface{}{},
				Metadata:        map[string]interface{}{},
				EnvironmentVars: map[string]string{},
				Recommendations: []string{"upgrade to newer version"},
				WorkingDir:      "/home/user",
				AdditionalPaths: []string{},
				VerifiedAt:      now,
				Duration:        3 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.result)
			if err != nil {
				t.Errorf("Failed to marshal VerificationResult: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled VerificationResult
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal VerificationResult: %v", err)
			}

			// Verify key fields
			if unmarshaled.Installed != tt.result.Installed {
				t.Errorf("Expected Installed %v, got %v", tt.result.Installed, unmarshaled.Installed)
			}
			if unmarshaled.Compatible != tt.result.Compatible {
				t.Errorf("Expected Compatible %v, got %v", tt.result.Compatible, unmarshaled.Compatible)
			}
			if len(unmarshaled.Issues) != len(tt.result.Issues) {
				t.Errorf("Expected %d issues, got %d", len(tt.result.Issues), len(unmarshaled.Issues))
			}
		})
	}
}

// TestIssue tests the Issue struct
func TestIssue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		issue Issue
	}{
		{
			name: "critical installation issue",
			issue: Issue{
				Severity:    IssueSeverityCritical,
				Category:    IssueCategoryInstallation,
				Title:       "Installation failed",
				Description: "Could not install runtime due to permission error",
				Solution:    "Run with elevated privileges",
				Details:     map[string]interface{}{"exit_code": 1, "stderr": "permission denied"},
			},
		},
		{
			name: "low severity path issue",
			issue: Issue{
				Severity:    IssueSeverityLow,
				Category:    IssueCategoryPath,
				Title:       "Non-standard path",
				Description: "Runtime installed in non-standard location",
				Solution:    "Add to PATH or reinstall in standard location",
				Details:     map[string]interface{}{"path": "/opt/custom/bin"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.issue)
			if err != nil {
				t.Errorf("Failed to marshal Issue: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled Issue
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal Issue: %v", err)
			}

			// Verify fields
			if unmarshaled.Severity != tt.issue.Severity {
				t.Errorf("Expected Severity %s, got %s", tt.issue.Severity, unmarshaled.Severity)
			}
			if unmarshaled.Category != tt.issue.Category {
				t.Errorf("Expected Category %s, got %s", tt.issue.Category, unmarshaled.Category)
			}
			if unmarshaled.Title != tt.issue.Title {
				t.Errorf("Expected Title %s, got %s", tt.issue.Title, unmarshaled.Title)
			}
		})
	}
}

// TestIssueCategory tests the IssueCategory enum
func TestIssueCategory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category IssueCategory
		expected string
	}{
		{"installation", IssueCategoryInstallation, "installation"},
		{"version", IssueCategoryVersion, "version"},
		{"path", IssueCategoryPath, "path"},
		{"environment", IssueCategoryEnvironment, "environment"},
		{"permissions", IssueCategoryPermissions, "permissions"},
		{"dependencies", IssueCategoryDependencies, "dependencies"},
		{"configuration", IssueCategoryConfiguration, "configuration"},
		{"corruption", IssueCategoryCorruption, "corruption"},
		{"execution", IssueCategoryExecution, "execution"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.category) != tt.expected {
				t.Errorf("Expected category string %s, got %s", tt.expected, string(tt.category))
			}
		})
	}
}

// TestIssueSeverity tests the IssueSeverity enum
func TestIssueSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		severity IssueSeverity
		expected string
	}{
		{"info", IssueSeverityInfo, "info"},
		{"low", IssueSeverityLow, "low"},
		{"medium", IssueSeverityMedium, "medium"},
		{"high", IssueSeverityHigh, "high"},
		{"critical", IssueSeverityCritical, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.severity) != tt.expected {
				t.Errorf("Expected severity string %s, got %s", tt.expected, string(tt.severity))
			}
		})
	}
}

// TestRuntimeInfo tests the RuntimeInfo struct
func TestRuntimeInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime RuntimeInfo
	}{
		{
			name: "go runtime info",
			runtime: RuntimeInfo{
				Name:       "go",
				Version:    "1.21.0",
				Path:       "/usr/local/go/bin/go",
				Installed:  true,
				Compatible: true,
				Issues:     []string{},
				Metadata: map[string]interface{}{
					"GOOS":   "linux",
					"GOARCH": "amd64",
				},
			},
		},
		{
			name: "python runtime with issues",
			runtime: RuntimeInfo{
				Name:       "python",
				Version:    "3.8.0",
				Path:       "/usr/bin/python3",
				Installed:  true,
				Compatible: false,
				Issues:     []string{"version too old", "missing pip"},
				Metadata: map[string]interface{}{
					"pip_version": "20.0.2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.runtime)
			if err != nil {
				t.Errorf("Failed to marshal RuntimeInfo: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled RuntimeInfo
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal RuntimeInfo: %v", err)
			}

			// Verify fields
			if unmarshaled.Name != tt.runtime.Name {
				t.Errorf("Expected Name %s, got %s", tt.runtime.Name, unmarshaled.Name)
			}
			if unmarshaled.Installed != tt.runtime.Installed {
				t.Errorf("Expected Installed %v, got %v", tt.runtime.Installed, unmarshaled.Installed)
			}
			if len(unmarshaled.Issues) != len(tt.runtime.Issues) {
				t.Errorf("Expected %d issues, got %d", len(tt.runtime.Issues), len(unmarshaled.Issues))
			}
		})
	}
}

// TestDetectionReport tests the DetectionReport struct
func TestDetectionReport(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tests := []struct {
		name   string
		report DetectionReport
	}{
		{
			name: "full detection report",
			report: DetectionReport{
				Platform:     "linux",
				Architecture: "amd64",
				Runtimes: map[string]*RuntimeInfo{
					"go": {
						Name:       "go",
						Version:    "1.21.0",
						Path:       "/usr/local/go/bin/go",
						Installed:  true,
						Compatible: true,
						Issues:     []string{},
						Metadata:   map[string]interface{}{"GOOS": "linux"},
					},
				},
				Summary: &DetectionSummary{
					TotalRuntimes:      4,
					InstalledRuntimes:  3,
					CompatibleRuntimes: 2,
					IssuesFound:        1,
					ReadinessScore:     75.0,
				},
				Issues:    []string{"nodejs not found"},
				Timestamp: now,
				Duration:  10 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.report)
			if err != nil {
				t.Errorf("Failed to marshal DetectionReport: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled DetectionReport
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal DetectionReport: %v", err)
			}

			// Verify fields
			if unmarshaled.Platform != tt.report.Platform {
				t.Errorf("Expected Platform %s, got %s", tt.report.Platform, unmarshaled.Platform)
			}
			if len(unmarshaled.Runtimes) != len(tt.report.Runtimes) {
				t.Errorf("Expected %d runtimes, got %d", len(tt.report.Runtimes), len(unmarshaled.Runtimes))
			}
			if unmarshaled.Summary == nil {
				t.Error("Expected Summary to be non-nil")
			}
		})
	}
}

// TestDetectionSummary tests the DetectionSummary struct
func TestDetectionSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		summary DetectionSummary
	}{
		{
			name: "good readiness score",
			summary: DetectionSummary{
				TotalRuntimes:      4,
				InstalledRuntimes:  4,
				CompatibleRuntimes: 4,
				IssuesFound:        0,
				ReadinessScore:     100.0,
			},
		},
		{
			name: "poor readiness score",
			summary: DetectionSummary{
				TotalRuntimes:      4,
				InstalledRuntimes:  1,
				CompatibleRuntimes: 0,
				IssuesFound:        3,
				ReadinessScore:     25.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.summary)
			if err != nil {
				t.Errorf("Failed to marshal DetectionSummary: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled DetectionSummary
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal DetectionSummary: %v", err)
			}

			// Verify fields
			if unmarshaled.ReadinessScore != tt.summary.ReadinessScore {
				t.Errorf("Expected ReadinessScore %f, got %f", tt.summary.ReadinessScore, unmarshaled.ReadinessScore)
			}
			if unmarshaled.TotalRuntimes != tt.summary.TotalRuntimes {
				t.Errorf("Expected TotalRuntimes %d, got %d", tt.summary.TotalRuntimes, unmarshaled.TotalRuntimes)
			}
		})
	}
}

// TestRuntimeDefinition tests the RuntimeDefinition struct
func TestRuntimeDefinition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		definition RuntimeDefinition
	}{
		{
			name: "go runtime definition",
			definition: RuntimeDefinition{
				Name:               "go",
				DisplayName:        "Go Programming Language",
				MinVersion:         "1.19.0",
				RecommendedVersion: "1.21.0",
				InstallMethods: map[string]InstallMethod{
					"download": {
						Name:         "download",
						Platform:     "all",
						Method:       "download",
						Commands:     []string{"wget", "tar"},
						Description:  "Download from golang.org",
						Requirements: []string{"wget", "tar"},
					},
				},
				VerificationCmd: []string{"go", "version"},
				VersionCommand:  []string{"go", "version"},
				EnvVars:         map[string]string{"GOROOT": "/usr/local/go"},
				Dependencies:    []string{"gcc"},
				PostInstall:     []string{"go mod tidy"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.definition)
			if err != nil {
				t.Errorf("Failed to marshal RuntimeDefinition: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled RuntimeDefinition
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal RuntimeDefinition: %v", err)
			}

			// Verify fields
			if unmarshaled.Name != tt.definition.Name {
				t.Errorf("Expected Name %s, got %s", tt.definition.Name, unmarshaled.Name)
			}
			if unmarshaled.MinVersion != tt.definition.MinVersion {
				t.Errorf("Expected MinVersion %s, got %s", tt.definition.MinVersion, unmarshaled.MinVersion)
			}
		})
	}
}

// TestServerDefinition tests the ServerDefinition struct
func TestServerDefinition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		definition ServerDefinition
	}{
		{
			name: "gopls server definition",
			definition: ServerDefinition{
				Name:              "gopls",
				DisplayName:       "Go Language Server",
				Runtime:           "go",
				MinVersion:        "0.14.0",
				MinRuntimeVersion: "1.19.0",
				InstallCmd:        []string{"go", "install", "golang.org/x/tools/gopls@latest"},
				VerifyCmd:         []string{"gopls", "version"},
				ConfigKey:         "gopls",
				Description:       "Official Go Language Server",
				Homepage:          "https://pkg.go.dev/golang.org/x/tools/gopls",
				Languages:         []string{"go"},
				Extensions:        []string{".go"},
				InstallMethods: []InstallMethod{
					{
						Name:        "go-install",
						Platform:    "all",
						Method:      "go-install",
						Commands:    []string{"go", "install", "golang.org/x/tools/gopls@latest"},
						Description: "Install via go install",
					},
				},
				VersionCommand: []string{"gopls", "version"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.definition)
			if err != nil {
				t.Errorf("Failed to marshal ServerDefinition: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled ServerDefinition
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal ServerDefinition: %v", err)
			}

			// Verify fields
			if unmarshaled.Name != tt.definition.Name {
				t.Errorf("Expected Name %s, got %s", tt.definition.Name, unmarshaled.Name)
			}
			if len(unmarshaled.Languages) != len(tt.definition.Languages) {
				t.Errorf("Expected %d languages, got %d", len(tt.definition.Languages), len(unmarshaled.Languages))
			}
		})
	}
}

// TestInstallMethod tests the InstallMethod struct
func TestInstallMethod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method InstallMethod
	}{
		{
			name: "package manager method",
			method: InstallMethod{
				Name:          "apt",
				Platform:      "linux",
				Method:        "package-manager",
				Commands:      []string{"apt", "update", "&&", "apt", "install", "-y"},
				Description:   "Install via APT package manager",
				Requirements:  []string{"apt"},
				PreRequisites: []string{"sudo"},
				Verification:  []string{"which"},
				PostInstall:   []string{"ldconfig"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.method)
			if err != nil {
				t.Errorf("Failed to marshal InstallMethod: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled InstallMethod
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal InstallMethod: %v", err)
			}

			// Verify fields
			if unmarshaled.Name != tt.method.Name {
				t.Errorf("Expected Name %s, got %s", tt.method.Name, unmarshaled.Name)
			}
			if unmarshaled.Platform != tt.method.Platform {
				t.Errorf("Expected Platform %s, got %s", tt.method.Platform, unmarshaled.Platform)
			}
		})
	}
}

// TestDependencyValidationResult tests the DependencyValidationResult struct
func TestDependencyValidationResult(t *testing.T) {
	t.Parallel()

	now := time.Now()
	tests := []struct {
		name   string
		result DependencyValidationResult
	}{
		{
			name: "valid dependencies",
			result: DependencyValidationResult{
				Server:            "gopls",
				Valid:             true,
				RuntimeRequired:   "go",
				RuntimeInstalled:  true,
				RuntimeVersion:    "1.21.0",
				RuntimeCompatible: true,
				Issues:            []Issue{},
				CanInstall:        true,
				MissingRuntimes:   []string{},
				VersionIssues:     []VersionIssue{},
				Recommendations:   []string{"all dependencies satisfied"},
				ValidatedAt:       now,
				Duration:          2 * time.Second,
			},
		},
		{
			name: "invalid dependencies",
			result: DependencyValidationResult{
				Server:            "pylsp",
				Valid:             false,
				RuntimeRequired:   "python",
				RuntimeInstalled:  false,
				RuntimeVersion:    "",
				RuntimeCompatible: false,
				Issues: []Issue{
					{
						Severity:    IssueSeverityCritical,
						Category:    IssueCategoryDependencies,
						Title:       "Runtime missing",
						Description: "Python runtime not installed",
						Solution:    "Install Python 3.8 or newer",
					},
				},
				CanInstall:      false,
				MissingRuntimes: []string{"python"},
				VersionIssues: []VersionIssue{
					{
						Component:        "python",
						RequiredVersion:  "3.8.0",
						InstalledVersion: "",
						Severity:         IssueSeverityCritical,
					},
				},
				Recommendations: []string{"install python runtime first"},
				ValidatedAt:     now,
				Duration:        1 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.result)
			if err != nil {
				t.Errorf("Failed to marshal DependencyValidationResult: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled DependencyValidationResult
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal DependencyValidationResult: %v", err)
			}

			// Verify fields
			if unmarshaled.Valid != tt.result.Valid {
				t.Errorf("Expected Valid %v, got %v", tt.result.Valid, unmarshaled.Valid)
			}
			if unmarshaled.Server != tt.result.Server {
				t.Errorf("Expected Server %s, got %s", tt.result.Server, unmarshaled.Server)
			}
		})
	}
}

// TestVersionIssue tests the VersionIssue struct
func TestVersionIssue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		issue VersionIssue
	}{
		{
			name: "version too old",
			issue: VersionIssue{
				Component:        "go",
				RequiredVersion:  "1.21.0",
				InstalledVersion: "1.19.0",
				Severity:         IssueSeverityHigh,
			},
		},
		{
			name: "component not found",
			issue: VersionIssue{
				Component:        "python",
				RequiredVersion:  "3.8.0",
				InstalledVersion: "",
				Severity:         IssueSeverityCritical,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.issue)
			if err != nil {
				t.Errorf("Failed to marshal VersionIssue: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled VersionIssue
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal VersionIssue: %v", err)
			}

			// Verify fields
			if unmarshaled.Component != tt.issue.Component {
				t.Errorf("Expected Component %s, got %s", tt.issue.Component, unmarshaled.Component)
			}
			if unmarshaled.Severity != tt.issue.Severity {
				t.Errorf("Expected Severity %s, got %s", tt.issue.Severity, unmarshaled.Severity)
			}
		})
	}
}

// TestVersionValidationResult tests the VersionValidationResult struct
func TestVersionValidationResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result VersionValidationResult
	}{
		{
			name: "valid version",
			result: VersionValidationResult{
				Valid:            true,
				RequiredVersion:  "1.19.0",
				InstalledVersion: "1.21.0",
				Issues:           []Issue{},
			},
		},
		{
			name: "invalid version",
			result: VersionValidationResult{
				Valid:            false,
				RequiredVersion:  "1.21.0",
				InstalledVersion: "1.19.0",
				Issues: []Issue{
					{
						Severity:    IssueSeverityHigh,
						Category:    IssueCategoryVersion,
						Title:       "Version too old",
						Description: "Installed version is below minimum requirement",
						Solution:    "Upgrade to newer version",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.result)
			if err != nil {
				t.Errorf("Failed to marshal VersionValidationResult: %v", err)
			}

			// Test JSON unmarshaling
			var unmarshaled VersionValidationResult
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Errorf("Failed to unmarshal VersionValidationResult: %v", err)
			}

			// Verify fields
			if unmarshaled.Valid != tt.result.Valid {
				t.Errorf("Expected Valid %v, got %v", tt.result.Valid, unmarshaled.Valid)
			}
		})
	}
}

// Mock implementations for interface testing

// MockRuntimeDetector is a mock implementation of RuntimeDetector
type MockRuntimeDetector struct {
	logger  interface{}
	timeout time.Duration
}

func (m *MockRuntimeDetector) DetectGo(ctx context.Context) (*RuntimeInfo, error) {
	return &RuntimeInfo{
		Name:       "go",
		Version:    "1.21.0",
		Path:       "/usr/local/go/bin/go",
		Installed:  true,
		Compatible: true,
		Issues:     []string{},
		Metadata:   map[string]interface{}{"GOOS": "linux"},
	}, nil
}

func (m *MockRuntimeDetector) DetectPython(ctx context.Context) (*RuntimeInfo, error) {
	return &RuntimeInfo{
		Name:       "python",
		Version:    "3.11.0",
		Path:       "/usr/bin/python3",
		Installed:  true,
		Compatible: true,
		Issues:     []string{},
		Metadata:   map[string]interface{}{"pip_version": "23.0"},
	}, nil
}

func (m *MockRuntimeDetector) DetectNodejs(ctx context.Context) (*RuntimeInfo, error) {
	return &RuntimeInfo{
		Name:       "nodejs",
		Version:    "18.0.0",
		Path:       "/usr/bin/node",
		Installed:  false,
		Compatible: false,
		Issues:     []string{"not installed"},
		Metadata:   map[string]interface{}{},
	}, nil
}

func (m *MockRuntimeDetector) DetectJava(ctx context.Context) (*RuntimeInfo, error) {
	return &RuntimeInfo{
		Name:       "java",
		Version:    "17.0.0",
		Path:       "/usr/bin/java",
		Installed:  true,
		Compatible: true,
		Issues:     []string{},
		Metadata:   map[string]interface{}{"vendor": "openjdk"},
	}, nil
}

func (m *MockRuntimeDetector) DetectAll(ctx context.Context) (*DetectionReport, error) {
	runtimes := make(map[string]*RuntimeInfo)

	goInfo, _ := m.DetectGo(ctx)
	runtimes["go"] = goInfo

	pythonInfo, _ := m.DetectPython(ctx)
	runtimes["python"] = pythonInfo

	return &DetectionReport{
		Platform:     "linux",
		Architecture: "amd64",
		Runtimes:     runtimes,
		Summary: &DetectionSummary{
			TotalRuntimes:      4,
			InstalledRuntimes:  3,
			CompatibleRuntimes: 3,
			IssuesFound:        1,
			ReadinessScore:     75.0,
		},
		Issues:    []string{"nodejs not installed"},
		Timestamp: time.Now(),
		Duration:  5 * time.Second,
	}, nil
}

func (m *MockRuntimeDetector) SetLogger(logger interface{}) {
	m.logger = logger
}

func (m *MockRuntimeDetector) SetTimeout(timeout time.Duration) {
	m.timeout = timeout
}

// MockSetupLogger is a mock implementation of SetupLogger
type MockSetupLogger struct {
	messages []string
	fields   map[string]interface{}
}

func (m *MockSetupLogger) Info(msg string) {
	m.messages = append(m.messages, "INFO: "+msg)
}

func (m *MockSetupLogger) Warn(msg string) {
	m.messages = append(m.messages, "WARN: "+msg)
}

func (m *MockSetupLogger) Error(msg string) {
	m.messages = append(m.messages, "ERROR: "+msg)
}

func (m *MockSetupLogger) Debug(msg string) {
	m.messages = append(m.messages, "DEBUG: "+msg)
}

func (m *MockSetupLogger) WithField(key string, value interface{}) SetupLogger {
	newLogger := &MockSetupLogger{
		messages: make([]string, len(m.messages)),
		fields:   make(map[string]interface{}),
	}
	copy(newLogger.messages, m.messages)
	for k, v := range m.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return newLogger
}

func (m *MockSetupLogger) WithFields(fields map[string]interface{}) SetupLogger {
	newLogger := &MockSetupLogger{
		messages: make([]string, len(m.messages)),
		fields:   make(map[string]interface{}),
	}
	copy(newLogger.messages, m.messages)
	for k, v := range m.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

func (m *MockSetupLogger) WithError(err error) SetupLogger {
	return m.WithField("error", err.Error())
}

func (m *MockSetupLogger) WithOperation(op string) SetupLogger {
	return m.WithField("operation", op)
}

// MockRuntimeInstaller is a mock implementation of RuntimeInstaller
type MockRuntimeInstaller struct{}

func (m *MockRuntimeInstaller) Install(runtime string, options InstallOptions) (*InstallResult, error) {
	return &InstallResult{
		Success:  true,
		Runtime:  runtime,
		Version:  options.Version,
		Path:     "/usr/local/bin/" + runtime,
		Method:   "mock",
		Duration: 10 * time.Second,
		Errors:   []string{},
		Warnings: []string{},
		Messages: []string{"mock installation completed"},
		Details:  map[string]interface{}{},
	}, nil
}

func (m *MockRuntimeInstaller) Verify(runtime string) (*VerificationResult, error) {
	return &VerificationResult{
		Installed:       true,
		Compatible:      true,
		Version:         "1.0.0",
		Path:            "/usr/local/bin/" + runtime,
		Runtime:         runtime,
		Issues:          []Issue{},
		Details:         map[string]interface{}{},
		Metadata:        map[string]interface{}{},
		EnvironmentVars: map[string]string{},
		Recommendations: []string{},
		WorkingDir:      "/tmp",
		AdditionalPaths: []string{},
		VerifiedAt:      time.Now(),
		Duration:        1 * time.Second,
	}, nil
}

func (m *MockRuntimeInstaller) GetSupportedRuntimes() []string {
	return []string{"go", "python", "nodejs", "java"}
}

func (m *MockRuntimeInstaller) GetRuntimeInfo(runtime string) (*RuntimeDefinition, error) {
	return &RuntimeDefinition{
		Name:               runtime,
		DisplayName:        strings.Title(runtime),
		MinVersion:         "1.0.0",
		RecommendedVersion: "2.0.0",
		InstallMethods:     map[string]InstallMethod{},
		VerificationCmd:    []string{runtime, "version"},
		VersionCommand:     []string{runtime, "version"},
		EnvVars:            map[string]string{},
		Dependencies:       []string{},
		PostInstall:        []string{},
	}, nil
}

func (m *MockRuntimeInstaller) ValidateVersion(runtime, minVersion string) (*VersionValidationResult, error) {
	return &VersionValidationResult{
		Valid:            true,
		RequiredVersion:  minVersion,
		InstalledVersion: "2.0.0",
		Issues:           []Issue{},
	}, nil
}

func (m *MockRuntimeInstaller) GetPlatformStrategy(platform string) RuntimePlatformStrategy {
	return &MockRuntimePlatformStrategy{}
}

// MockRuntimePlatformStrategy is a mock implementation of RuntimePlatformStrategy
type MockRuntimePlatformStrategy struct{}

func (m *MockRuntimePlatformStrategy) InstallRuntime(runtime string, options InstallOptions) (*InstallResult, error) {
	return &InstallResult{
		Success:  true,
		Runtime:  runtime,
		Version:  options.Version,
		Path:     "/usr/local/bin/" + runtime,
		Method:   "platform-specific",
		Duration: 15 * time.Second,
		Errors:   []string{},
		Warnings: []string{},
		Messages: []string{"platform-specific installation completed"},
		Details:  map[string]interface{}{},
	}, nil
}

func (m *MockRuntimePlatformStrategy) VerifyRuntime(runtime string) (*VerificationResult, error) {
	return &VerificationResult{
		Installed:       true,
		Compatible:      true,
		Version:         "1.0.0",
		Path:            "/usr/local/bin/" + runtime,
		Runtime:         runtime,
		Issues:          []Issue{},
		Details:         map[string]interface{}{},
		Metadata:        map[string]interface{}{},
		EnvironmentVars: map[string]string{},
		Recommendations: []string{},
		WorkingDir:      "/tmp",
		AdditionalPaths: []string{},
		VerifiedAt:      time.Now(),
		Duration:        1 * time.Second,
	}, nil
}

func (m *MockRuntimePlatformStrategy) GetInstallCommand(runtime, version string) ([]string, error) {
	return []string{"mock-install", runtime, version}, nil
}

// TestMockRuntimeDetector tests the mock implementation
func TestMockRuntimeDetector(t *testing.T) {
	t.Parallel()

	detector := &MockRuntimeDetector{}
	ctx := context.Background()

	t.Run("detect go", func(t *testing.T) {
		info, err := detector.DetectGo(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if info.Name != "go" {
			t.Errorf("Expected name 'go', got %s", info.Name)
		}
		if !info.Installed {
			t.Error("Expected Go to be installed in mock")
		}
	})

	t.Run("detect all", func(t *testing.T) {
		report, err := detector.DetectAll(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if report.Platform != "linux" {
			t.Errorf("Expected platform 'linux', got %s", report.Platform)
		}
		if len(report.Runtimes) != 2 {
			t.Errorf("Expected 2 runtimes in report, got %d", len(report.Runtimes))
		}
	})

	t.Run("set logger and timeout", func(t *testing.T) {
		mockLogger := &MockSetupLogger{}
		detector.SetLogger(mockLogger)
		detector.SetTimeout(30 * time.Second)

		if detector.logger != mockLogger {
			t.Error("Logger was not set correctly")
		}
		if detector.timeout != 30*time.Second {
			t.Error("Timeout was not set correctly")
		}
	})
}

// TestMockSetupLogger tests the mock logger implementation
func TestMockSetupLogger(t *testing.T) {
	t.Parallel()

	logger := &MockSetupLogger{
		messages: []string{},
		fields:   make(map[string]interface{}),
	}

	t.Run("log messages", func(t *testing.T) {
		logger.Info("test info")
		logger.Warn("test warn")
		logger.Error("test error")
		logger.Debug("test debug")

		if len(logger.messages) != 4 {
			t.Errorf("Expected 4 messages, got %d", len(logger.messages))
		}

		expectedMessages := []string{
			"INFO: test info",
			"WARN: test warn",
			"ERROR: test error",
			"DEBUG: test debug",
		}

		for i, expected := range expectedMessages {
			if logger.messages[i] != expected {
				t.Errorf("Expected message %s, got %s", expected, logger.messages[i])
			}
		}
	})

	t.Run("with field", func(t *testing.T) {
		newLogger := logger.WithField("key", "value")
		if mockLogger, ok := newLogger.(*MockSetupLogger); ok {
			if mockLogger.fields["key"] != "value" {
				t.Error("Field was not set correctly")
			}
		} else {
			t.Error("WithField should return MockSetupLogger")
		}
	})

	t.Run("with fields", func(t *testing.T) {
		fields := map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
		}
		newLogger := logger.WithFields(fields)
		if mockLogger, ok := newLogger.(*MockSetupLogger); ok {
			if mockLogger.fields["key1"] != "value1" || mockLogger.fields["key2"] != "value2" {
				t.Error("Fields were not set correctly")
			}
		} else {
			t.Error("WithFields should return MockSetupLogger")
		}
	})
}

// TestMockRuntimeInstaller tests the mock runtime installer
func TestMockRuntimeInstaller(t *testing.T) {
	t.Parallel()

	installer := &MockRuntimeInstaller{}

	t.Run("install runtime", func(t *testing.T) {
		options := InstallOptions{
			Version: "1.21.0",
			Force:   false,
		}
		result, err := installer.Install("go", options)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !result.Success {
			t.Error("Expected successful installation")
		}
		if result.Runtime != "go" {
			t.Errorf("Expected runtime 'go', got %s", result.Runtime)
		}
	})

	t.Run("verify runtime", func(t *testing.T) {
		result, err := installer.Verify("python")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !result.Installed {
			t.Error("Expected runtime to be installed")
		}
		if result.Runtime != "python" {
			t.Errorf("Expected runtime 'python', got %s", result.Runtime)
		}
	})

	t.Run("get supported runtimes", func(t *testing.T) {
		runtimes := installer.GetSupportedRuntimes()
		expected := []string{"go", "python", "nodejs", "java"}
		if len(runtimes) != len(expected) {
			t.Errorf("Expected %d runtimes, got %d", len(expected), len(runtimes))
		}
		for i, runtime := range runtimes {
			if runtime != expected[i] {
				t.Errorf("Expected runtime %s, got %s", expected[i], runtime)
			}
		}
	})

	t.Run("validate version", func(t *testing.T) {
		result, err := installer.ValidateVersion("go", "1.19.0")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !result.Valid {
			t.Error("Expected version to be valid")
		}
		if result.RequiredVersion != "1.19.0" {
			t.Errorf("Expected required version '1.19.0', got %s", result.RequiredVersion)
		}
	})
}

// TestEnumConstants tests that all enum constants are defined correctly
func TestEnumConstants(t *testing.T) {
	t.Parallel()

	t.Run("issue categories complete", func(t *testing.T) {
		categories := []IssueCategory{
			IssueCategoryInstallation,
			IssueCategoryVersion,
			IssueCategoryPath,
			IssueCategoryEnvironment,
			IssueCategoryPermissions,
			IssueCategoryDependencies,
			IssueCategoryConfiguration,
			IssueCategoryCorruption,
			IssueCategoryExecution,
		}

		if len(categories) != 9 {
			t.Errorf("Expected 9 issue categories, got %d", len(categories))
		}

		// Test that all categories are non-empty strings
		for i, category := range categories {
			if string(category) == "" {
				t.Errorf("Category at index %d is empty", i)
			}
		}
	})

	t.Run("issue severities complete", func(t *testing.T) {
		severities := []IssueSeverity{
			IssueSeverityInfo,
			IssueSeverityLow,
			IssueSeverityMedium,
			IssueSeverityHigh,
			IssueSeverityCritical,
		}

		if len(severities) != 5 {
			t.Errorf("Expected 5 issue severities, got %d", len(severities))
		}

		// Test that all severities are non-empty strings
		for i, severity := range severities {
			if string(severity) == "" {
				t.Errorf("Severity at index %d is empty", i)
			}
		}
	})
}

// TestStructDefaults tests default values and zero values of structs
func TestStructDefaults(t *testing.T) {
	t.Parallel()

	t.Run("InstallOptions defaults", func(t *testing.T) {
		var options InstallOptions
		if options.Force != false {
			t.Error("Expected Force to default to false")
		}
		if options.SkipVerify != false {
			t.Error("Expected SkipVerify to default to false")
		}
		if options.Timeout != 0 {
			t.Error("Expected Timeout to default to 0")
		}
	})

	t.Run("InstallResult defaults", func(t *testing.T) {
		var result InstallResult
		if result.Success != false {
			t.Error("Expected Success to default to false")
		}
		if len(result.Errors) != 0 {
			t.Error("Expected Errors to be empty slice by default")
		}
		if result.Details != nil {
			t.Error("Expected Details to be nil by default")
		}
	})

	t.Run("Issue defaults", func(t *testing.T) {
		var issue Issue
		if issue.Severity != "" {
			t.Error("Expected Severity to be empty string by default")
		}
		if issue.Category != "" {
			t.Error("Expected Category to be empty string by default")
		}
	})
}

// TestJSONSerializationEdgeCases tests edge cases in JSON serialization
func TestJSONSerializationEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty structs", func(t *testing.T) {
		structs := []interface{}{
			InstallOptions{},
			ServerInstallOptions{},
			InstallResult{},
			VerificationResult{},
			Issue{},
			RuntimeInfo{},
			DetectionReport{},
			DetectionSummary{},
			RuntimeDefinition{},
			ServerDefinition{},
			InstallMethod{},
			DependencyValidationResult{},
			VersionIssue{},
			VersionValidationResult{},
		}

		for i, s := range structs {
			data, err := json.Marshal(s)
			if err != nil {
				t.Errorf("Failed to marshal empty struct at index %d: %v", i, err)
				continue
			}

			// Should be able to unmarshal back
			newValue := reflect.New(reflect.TypeOf(s))
			if err := json.Unmarshal(data, newValue.Interface()); err != nil {
				t.Errorf("Failed to unmarshal empty struct at index %d: %v", i, err)
			}
		}
	})

	t.Run("nil maps and slices", func(t *testing.T) {
		result := InstallResult{
			Details: nil,
			Errors:  nil,
		}

		data, err := json.Marshal(result)
		if err != nil {
			t.Errorf("Failed to marshal struct with nil fields: %v", err)
		}

		var unmarshaled InstallResult
		if err := json.Unmarshal(data, &unmarshaled); err != nil {
			t.Errorf("Failed to unmarshal struct with nil fields: %v", err)
		}
	})
}
