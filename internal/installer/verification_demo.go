package installer

import (
	"fmt"
	"lsp-gateway/internal/types"
	"time"
)

func DemoVerificationSystem() {
	fmt.Println("=== LSP Gateway Installation Verification System Demo ===")
	fmt.Println()

	installer := NewRuntimeInstaller()
	fmt.Println("✅ Runtime installer created successfully")

	runtimes := installer.GetSupportedRuntimes()
	fmt.Printf("📋 Supported runtimes (%d): %v\n", len(runtimes), runtimes)
	fmt.Println()

	fmt.Println("🔍 Runtime Definitions:")
	for _, runtime := range runtimes {
		if def, err := installer.GetRuntimeInfo(runtime); err == nil {
			fmt.Printf("  • %s (%s)\n", def.DisplayName, def.Name)
			fmt.Printf("    Min Version: %s\n", def.MinVersion)
			fmt.Printf("    Recommended: %s\n", def.RecommendedVersion)
			fmt.Printf("    Verification Command: %v\n", def.VerificationCmd)
			fmt.Println()
		}
	}

	fmt.Println("🔧 Verification Result Structure:")
	result := &types.VerificationResult{
		Runtime:         "go",
		Installed:       true,
		Version:         "1.21.0",
		Compatible:      true,
		Path:            "/usr/bin/go",
		Issues:          []types.Issue{},
		Recommendations: []string{},
		WorkingDir:      "/tmp",
		EnvironmentVars: map[string]string{"GOPATH": "/home/user/go"},
		AdditionalPaths: []string{"/usr/local/go/bin"},
		VerifiedAt:      time.Now(),
		Duration:        2 * time.Second,
		Metadata:        map[string]interface{}{"test_mode": true},
	}

	fmt.Printf("  Runtime: %s\n", result.Runtime)
	fmt.Printf("  Installed: %t\n", result.Installed)
	fmt.Printf("  Version: %s\n", result.Version)
	fmt.Printf("  Compatible: %t\n", result.Compatible)
	fmt.Printf("  Path: %s\n", result.Path)
	fmt.Printf("  Verification Duration: %v\n", result.Duration)
	fmt.Println()

	fmt.Println("⚠️ Issue Tracking:")
	testIssue := types.Issue{
		Severity:    types.IssueSeverityHigh,
		Category:    types.IssueCategoryVersion,
		Title:       "Version Compatibility Issue",
		Description: "Installed version is below minimum requirement",
		Solution:    "Upgrade to a newer version",
		Details:     map[string]interface{}{"current": "1.18.0", "required": "1.19.0"},
	}

	fmt.Printf("  Severity: %s\n", testIssue.Severity)
	fmt.Printf("  Category: %s\n", testIssue.Category)
	fmt.Printf("  Title: %s\n", testIssue.Title)
	fmt.Printf("  Description: %s\n", testIssue.Description)
	fmt.Printf("  Solution: %s\n", testIssue.Solution)
	fmt.Println()

	fmt.Println("❌ Error Handling:")

	if _, err := installer.Verify("unknown"); err != nil {
		fmt.Printf("  Unknown runtime error: %v\n", err)
	}

	fmt.Printf("  Dependency and installation error handling available\n")

	fmt.Println()

	fmt.Println("🖥️ Platform Strategies:")
	platforms := []string{"windows", "linux", "darwin"}
	for _, platform := range platforms {
		strategy := installer.GetPlatformStrategy(platform)
		if strategy != nil {
			fmt.Printf("  %s: platform strategy available\n", platform)
		}
	}
	fmt.Println()

	fmt.Println("🔄 Verification Phases:")
	phases := []string{
		"1. Basic Installation Check",
		"2. Runtime-Specific Environment Verification",
		"3. Dependency and Toolchain Verification",
		"4. Functional Testing and Validation",
		"5. Recommendation Generation",
	}

	for _, phase := range phases {
		fmt.Printf("  %s\n", phase)
	}
	fmt.Println()

	fmt.Println("📂 Issue Categories:")
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

	for _, category := range categories {
		fmt.Printf("  • %s\n", category)
	}
	fmt.Println()

	fmt.Println("🚨 Issue Severity Levels:")
	severities := []types.IssueSeverity{
		types.IssueSeverityCritical,
		types.IssueSeverityHigh,
		types.IssueSeverityMedium,
		types.IssueSeverityLow,
		types.IssueSeverityInfo,
	}

	for _, severity := range severities {
		fmt.Printf("  • %s\n", severity)
	}
	fmt.Println()

	fmt.Println("✅ Verification system demonstration complete!")
	fmt.Println("📖 The system provides comprehensive runtime verification with:")
	fmt.Println("   - Detection integration with Phase 2 setup logic")
	fmt.Println("   - Cross-platform support via Phase 1 platform detection")
	fmt.Println("   - Detailed issue categorization and severity tracking")
	fmt.Println("   - Actionable recommendations for problem resolution")
	fmt.Println("   - Comprehensive error handling and reporting")
	fmt.Println("   - Runtime-specific verification (Go, Python, Node.js, Java)")
}

func CreateSampleVerificationReport(runtime string, simulateIssues bool) *types.VerificationResult {
	result := &types.VerificationResult{
		Runtime:         runtime,
		Installed:       true,
		Version:         "1.21.0",
		Compatible:      true,
		Path:            fmt.Sprintf("/usr/bin/%s", runtime),
		Issues:          []types.Issue{},
		Recommendations: []string{},
		WorkingDir:      "/tmp",
		EnvironmentVars: make(map[string]string),
		AdditionalPaths: []string{},
		VerifiedAt:      time.Now(),
		Duration:        time.Duration(1+len(runtime)) * time.Second,
		Metadata:        make(map[string]interface{}),
	}

	switch runtime {
	case "go":
		result.EnvironmentVars["GOPATH"] = "/home/user/go"
		result.EnvironmentVars["GOROOT"] = "/usr/local/go"
		result.Metadata["go_modules_supported"] = true
	case "python":
		result.EnvironmentVars["PYTHONPATH"] = "/usr/lib/python3/site-packages"
		result.Metadata["pip_available"] = true
	case "nodejs":
		result.EnvironmentVars["NODE_PATH"] = "/usr/lib/node_modules"
		result.Metadata["npm_available"] = true
	case "java":
		result.EnvironmentVars["JAVA_HOME"] = "/usr/lib/jvm/java-17-openjdk"
		result.Metadata["javac_available"] = true
	}

	if simulateIssues {
		result.Issues = append(result.Issues, types.Issue{
			Severity:    types.IssueSeverityMedium,
			Category:    types.IssueCategoryEnvironment,
			Title:       "Environment Variable Warning",
			Description: fmt.Sprintf("%s environment variable could be optimized", runtime),
			Solution:    "Review and optimize environment configuration",
			Details:     map[string]interface{}{"runtime": runtime},
		})

		result.Issues = append(result.Issues, types.Issue{
			Severity:    types.IssueSeverityLow,
			Category:    types.IssueCategoryConfiguration,
			Title:       "Configuration Recommendation",
			Description: fmt.Sprintf("%s configuration could be improved", runtime),
			Solution:    "Apply recommended configuration changes",
			Details:     map[string]interface{}{"runtime": runtime},
		})

		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Consider upgrading %s to the latest version", runtime))
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Review %s environment configuration", runtime))
	} else {
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("✅ %s runtime is properly installed and configured", runtime))
	}

	return result
}

func PrintVerificationReport(result *types.VerificationResult) {
	fmt.Printf("=== Verification Report: %s ===\n", result.Runtime)
	fmt.Printf("Status: %s\n", map[bool]string{true: "✅ INSTALLED", false: "❌ NOT INSTALLED"}[result.Installed])
	fmt.Printf("Version: %s\n", result.Version)
	fmt.Printf("Compatible: %s\n", map[bool]string{true: "✅ YES", false: "❌ NO"}[result.Compatible])
	fmt.Printf("Path: %s\n", result.Path)
	fmt.Printf("Verification Time: %v\n", result.Duration)
	fmt.Printf("Verified At: %s\n", result.VerifiedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	if len(result.EnvironmentVars) > 0 {
		fmt.Println("Environment Variables:")
		for key, value := range result.EnvironmentVars {
			fmt.Printf("  %s = %s\n", key, value)
		}
		fmt.Println()
	}

	if len(result.Issues) > 0 {
		fmt.Printf("Issues Found (%d):\n", len(result.Issues))
		for i, issue := range result.Issues {
			fmt.Printf("  %d. [%s] %s\n", i+1, issue.Severity, issue.Title)
			fmt.Printf("     %s\n", issue.Description)
			fmt.Printf("     Solution: %s\n", issue.Solution)
		}
		fmt.Println()
	}

	if len(result.Recommendations) > 0 {
		fmt.Printf("Recommendations (%d):\n", len(result.Recommendations))
		for i, rec := range result.Recommendations {
			fmt.Printf("  %d. %s\n", i+1, rec)
		}
		fmt.Println()
	}

	if len(result.Metadata) > 0 {
		fmt.Println("Additional Information:")
		for key, value := range result.Metadata {
			fmt.Printf("  %s: %v\n", key, value)
		}
		fmt.Println()
	}
}
