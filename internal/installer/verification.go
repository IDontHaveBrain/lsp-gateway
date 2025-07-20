package installer

import (
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
	"os"
	"strings"
	"time"
)

func (r *DefaultRuntimeInstaller) verifyBasicInstallation(result *types.VerificationResult, def *types.RuntimeDefinition) {
	r.addIssue(result, IssueSeverityInfo, IssueCategoryConfiguration,
		"Verification Disabled",
		"Runtime verification temporarily disabled during refactoring",
		"This will be restored with proper dependency injection",
		map[string]interface{}{"runtime": def.Name})
}

func (r *DefaultRuntimeInstaller) verifyGoEnvironment(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	if gopath := os.Getenv("GOPATH"); gopath != "" {
		result.EnvironmentVars["GOPATH"] = gopath
		if _, err := os.Stat(gopath); err != nil {
			r.addIssue(result, IssueSeverityMedium, IssueCategoryEnvironment,
				"Invalid GOPATH",
				fmt.Sprintf("GOPATH points to non-existent directory: %s", gopath),
				"Create the GOPATH directory or unset the GOPATH environment variable",
				map[string]interface{}{"gopath": gopath})
		}
	}

	if goroot := os.Getenv("GOROOT"); goroot != "" {
		result.EnvironmentVars["GOROOT"] = goroot
		if _, err := os.Stat(goroot); err != nil {
			r.addIssue(result, IssueSeverityMedium, IssueCategoryEnvironment,
				"Invalid GOROOT",
				fmt.Sprintf("GOROOT points to non-existent directory: %s", goroot),
				"Correct the GOROOT environment variable or unset it to use the default",
				map[string]interface{}{"goroot": goroot})
		}
	}

	envResult, err := executor.Execute("go", []string{"env"}, 10*time.Second)
	if err == nil && envResult.ExitCode == 0 {
		r.parseGoEnv(result, envResult.Stdout)
	} else {
		r.addIssue(result, IssueSeverityMedium, IssueCategoryEnvironment,
			"Go Environment Check Failed",
			"Failed to retrieve Go environment variables",
			"Check if Go is properly installed and configured",
			map[string]interface{}{"error": err})
	}
}

func (r *DefaultRuntimeInstaller) parseGoEnv(result *types.VerificationResult, envOutput string) {
	lines := strings.Split(envOutput, "\n")
	goEnv := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := strings.Trim(parts[1], "\"'")
				goEnv[key] = value
			}
		}
	}

	result.Metadata["go_env"] = goEnv

	if goos, exists := goEnv["GOOS"]; exists {
		result.Metadata["go_os"] = goos
	}

	if goarch, exists := goEnv["GOARCH"]; exists {
		result.Metadata["go_arch"] = goarch
	}

	if goversion, exists := goEnv["GOVERSION"]; exists {
		result.Metadata["go_version_env"] = goversion
	}
}

func (r *DefaultRuntimeInstaller) verifyGoToolchain(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	buildResult, err := executor.Execute("go", []string{"version"}, 5*time.Second)
	if err != nil || buildResult.ExitCode != 0 {
		r.addIssue(result, IssueSeverityHigh, IssueCategoryDependencies,
			"Go Build Tools Unavailable",
			"Go build tools are not functioning properly",
			"Reinstall Go or check your Go installation",
			map[string]interface{}{"error": err})
		return
	}

	if platform.IsCommandAvailable("gofmt") {
		result.Metadata["gofmt_available"] = true
	} else {
		r.addIssue(result, IssueSeverityLow, IssueCategoryDependencies,
			"Go Format Tool Missing",
			"gofmt tool is not available",
			"Reinstall Go to get the complete toolchain",
			map[string]interface{}{})
	}

	vetResult, err := executor.Execute("go", []string{"help", "vet"}, 5*time.Second)
	if err == nil && vetResult.ExitCode == 0 {
		result.Metadata["go_vet_available"] = true
	} else {
		r.addIssue(result, IssueSeverityLow, IssueCategoryDependencies,
			"Go Vet Tool Missing",
			"go vet tool is not available",
			"Update Go to get the complete toolchain",
			map[string]interface{}{})
	}
}

func (r *DefaultRuntimeInstaller) verifyGoModules(result *types.VerificationResult) {
	executor := platform.NewCommandExecutor()

	modResult, err := executor.Execute("go", []string{"help", "mod"}, 5*time.Second)
	if err != nil || modResult.ExitCode != 0 {
		r.addIssue(result, IssueSeverityMedium, IssueCategoryDependencies,
			"Go Modules Not Supported",
			"Go modules are not available in this Go installation",
			"Upgrade to Go 1.11 or later to use Go modules",
			map[string]interface{}{"error": err})
		return
	}

	result.Metadata["go_modules_supported"] = true

	if go111module := os.Getenv("GO111MODULE"); go111module != "" {
		result.EnvironmentVars["GO111MODULE"] = go111module
		result.Metadata["go111module"] = go111module
	}
}

func (r *DefaultRuntimeInstaller) addIssue(result *types.VerificationResult, severity types.IssueSeverity, category types.IssueCategory, title, description, solution string, details map[string]interface{}) {
	issue := types.Issue{
		Severity:    severity,
		Category:    category,
		Title:       title,
		Description: description,
		Solution:    solution,
		Details:     details,
	}
	result.Issues = append(result.Issues, issue)
}
