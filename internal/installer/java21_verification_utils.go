package installer

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
)

// IsJava21Installed checks if Java 21 runtime exists at expected path and is functional
func IsJava21Installed() bool {
	executablePath := GetJava21ExecutablePath()
	
	// Check if executable file exists
	if info, err := os.Stat(executablePath); err != nil || info.IsDir() {
		return false
	}
	
	// Verify it's executable (on Unix systems)
	if !platform.IsWindows() {
		if info, err := os.Stat(executablePath); err == nil {
			if info.Mode()&0111 == 0 {
				return false
			}
		}
	}
	
	// Quick functional test - try to execute java -version
	executor := platform.NewCommandExecutor()
	result, err := executor.Execute(executablePath, []string{"-version"}, 5*time.Second)
	if err != nil || result.ExitCode != 0 {
		return false
	}
	
	// Verify output contains Java 21 version information
	output := strings.ToLower(result.Stderr + result.Stdout)
	return strings.Contains(output, "openjdk 21") || 
		   strings.Contains(output, "java 21") || 
		   strings.Contains(output, "21.0.")
}

// VerifyJava21Installation performs comprehensive verification of Java 21 installation
func VerifyJava21Installation() (*types.VerificationResult, error) {
	startTime := time.Now()
	
	result := &types.VerificationResult{
		Installed:       false,
		Compatible:      false,
		Version:         "",
		Path:            "",
		Runtime:         RuntimeJava,
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
		Recommendations: []string{},
		VerifiedAt:      startTime,
	}
	
	executablePath := GetJava21ExecutablePath()
	result.Path = executablePath
	result.Metadata["executable_path"] = executablePath
	result.Metadata["install_path"] = GetJava21InstallPath()
	
	// Check if executable exists
	if info, err := os.Stat(executablePath); err != nil {
		addVerificationIssue(result, types.IssueSeverityCritical, types.IssueCategoryInstallation,
			JAVA21_NOT_FOUND,
			fmt.Sprintf("Java 21 executable not found at: %s", executablePath),
			"Install Java 21 runtime using the setup command",
			map[string]interface{}{
				"expected_path": executablePath,
				"install_path": GetJava21InstallPath(),
			})
		result.Duration = time.Since(startTime)
		return result, nil
	} else if info.IsDir() {
		addVerificationIssue(result, types.IssueSeverityCritical, types.IssueCategoryInstallation,
			"Java 21 Path Is Directory",
			fmt.Sprintf("Expected Java 21 executable path is a directory: %s", executablePath),
			"Remove the directory and reinstall Java 21 runtime",
			map[string]interface{}{"path": executablePath})
		result.Duration = time.Since(startTime)
		return result, nil
	} else {
		result.Metadata["file_info"] = map[string]interface{}{
			"size":     info.Size(),
			"mode":     info.Mode().String(),
			"mod_time": info.ModTime(),
		}
	}
	
	// Check executable permissions (Unix systems)
	if !platform.IsWindows() {
		if info, err := os.Stat(executablePath); err == nil {
			if info.Mode()&0111 == 0 {
				addVerificationIssue(result, types.IssueSeverityHigh, types.IssueCategoryPermissions,
					"Java 21 Not Executable",
					"Java 21 executable exists but is not executable",
					fmt.Sprintf("Fix permissions: chmod +x %s", executablePath),
					map[string]interface{}{"path": executablePath})
				result.Duration = time.Since(startTime)
				return result, nil
			}
		}
	}
	
	result.Installed = true
	
	// Verify functionality and version
	if err := verifyJava21Functionality(result, executablePath); err != nil {
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("Java 21 functionality verification failed: %w", err)
	}
	
	// Parse and validate version
	if err := parseJava21Version(result, executablePath); err != nil {
		addVerificationIssue(result, types.IssueSeverityMedium, types.IssueCategoryVersion,
			"Java 21 Version Parse Failed",
			fmt.Sprintf("Could not parse Java 21 version: %v", err),
			"Verify Java 21 installation integrity",
			map[string]interface{}{"error": err.Error()})
	}
	
	// Check Java home environment
	if javaHome := os.Getenv(ENV_VAR_JAVA_HOME); javaHome != "" {
		result.EnvironmentVars[ENV_VAR_JAVA_HOME] = javaHome
		result.Metadata["java_home_set"] = true
		
		// Verify JAVA_HOME points to our Java 21 installation
		expectedJavaHome := GetJava21InstallPath()
		if !strings.HasPrefix(javaHome, expectedJavaHome) {
			addVerificationIssue(result, types.IssueSeverityMedium, types.IssueCategoryEnvironment,
				"JAVA_HOME Mismatch",
				fmt.Sprintf("JAVA_HOME (%s) does not point to Java 21 installation (%s)", javaHome, expectedJavaHome),
				fmt.Sprintf("Set JAVA_HOME to %s", expectedJavaHome),
				map[string]interface{}{
					"current_java_home": javaHome,
					"expected_java_home": expectedJavaHome,
				})
		}
	} else {
		result.EnvironmentVars[ENV_VAR_JAVA_HOME] = ""
		result.Metadata["java_home_set"] = false
	}
	
	// Generate recommendations
	generateJava21Recommendations(result)
	
	result.Duration = time.Since(startTime)
	return result, nil
}

// GetJava21Version extracts and parses the Java 21 version string
func GetJava21Version() (string, error) {
	executablePath := GetJava21ExecutablePath()
	
	// Check if executable exists
	if _, err := os.Stat(executablePath); err != nil {
		return "", fmt.Errorf("Java 21 executable not found at: %s", executablePath)
	}
	
	executor := platform.NewCommandExecutor()
	result, err := executor.Execute(executablePath, []string{"-version"}, 10*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to execute java -version: %w", err)
	}
	
	if result.ExitCode != 0 {
		return "", fmt.Errorf("java -version command failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	
	// Parse version from stderr (Java outputs version info to stderr)
	versionOutput := result.Stderr
	if versionOutput == "" {
		versionOutput = result.Stdout
	}
	
	// Extract version using regex patterns
	version, err := parseJavaVersionString(versionOutput)
	if err != nil {
		return "", fmt.Errorf("failed to parse Java version from output: %w", err)
	}
	
	// Verify it's Java 21
	if !isJava21Version(version) {
		return "", fmt.Errorf("installed Java is not version 21, found: %s", version)
	}
	
	return version, nil
}

// verifyJava21Functionality tests that Java 21 executable is functional
func verifyJava21Functionality(result *types.VerificationResult, executablePath string) error {
	executor := platform.NewCommandExecutor()
	
	// Test basic functionality with -version flag
	versionResult, err := executor.Execute(executablePath, []string{"-version"}, 10*time.Second)
	if err != nil {
		addVerificationIssue(result, types.IssueSeverityCritical, types.IssueCategoryExecution,
			JAVA21_VERIFY_FAILED,
			fmt.Sprintf("Failed to execute java -version: %v", err),
			"Check Java 21 installation integrity and permissions",
			map[string]interface{}{"error": err.Error(), "executable": executablePath})
		return err
	}
	
	if versionResult.ExitCode != 0 {
		addVerificationIssue(result, types.IssueSeverityCritical, types.IssueCategoryExecution,
			JAVA21_VERIFY_FAILED,
			fmt.Sprintf("java -version failed with exit code %d", versionResult.ExitCode),
			"Reinstall Java 21 runtime",
			map[string]interface{}{
				"exit_code": versionResult.ExitCode,
				"stderr": versionResult.Stderr,
				"executable": executablePath,
			})
		return fmt.Errorf("java -version failed with exit code %d", versionResult.ExitCode)
	}
	
	result.Metadata["version_output"] = versionResult.Stderr
	result.Metadata["version_command_duration"] = versionResult.Duration
	
	// Test help functionality to ensure comprehensive functionality
	helpResult, err := executor.Execute(executablePath, []string{"-help"}, 5*time.Second)
	if err == nil && helpResult.ExitCode == 0 {
		result.Metadata["help_test"] = StatusPassed
		result.Metadata["help_output_length"] = len(helpResult.Stdout)
	} else {
		// Help test failure is not critical, but worth noting
		result.Metadata["help_test"] = "failed"
		result.Metadata["help_error"] = fmt.Sprintf("%v", err)
	}
	
	return nil
}

// parseJava21Version extracts and validates Java 21 version from java -version output
func parseJava21Version(result *types.VerificationResult, executablePath string) error {
	executor := platform.NewCommandExecutor()
	versionResult, err := executor.Execute(executablePath, []string{"-version"}, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}
	
	if versionResult.ExitCode != 0 {
		return fmt.Errorf("version command failed with exit code %d", versionResult.ExitCode)
	}
	
	versionOutput := versionResult.Stderr
	if versionOutput == "" {
		versionOutput = versionResult.Stdout
	}
	
	version, err := parseJavaVersionString(versionOutput)
	if err != nil {
		return err
	}
	
	result.Version = version
	result.Metadata["parsed_version"] = version
	result.Metadata["version_source"] = "java_version_command"
	
	// Verify it's Java 21
	if isJava21Version(version) {
		result.Compatible = true
		result.Metadata["version_compatibility"] = "java21_compatible"
	} else {
		result.Compatible = false
		result.Metadata["version_compatibility"] = "not_java21"
		addVerificationIssue(result, types.IssueSeverityCritical, types.IssueCategoryVersion,
			"Wrong Java Version",
			fmt.Sprintf("Expected Java 21, but found: %s", version),
			"Install Java 21 runtime",
			map[string]interface{}{
				"found_version": version,
				"expected_version": "21.x.x",
			})
	}
	
	return nil
}

// parseJavaVersionString extracts version from Java version output using multiple regex patterns
func parseJavaVersionString(output string) (string, error) {
	if output == "" {
		return "", fmt.Errorf("empty version output")
	}
	
	// Try multiple regex patterns to match different Java version formats
	patterns := []string{
		// Pattern for OpenJDK format: openjdk version "21.0.1" 2023-10-17
		`openjdk version "([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:\+[0-9]+)?)"`,
		// Pattern for Oracle JDK format: java version "21.0.1"
		`java version "([0-9]+\.[0-9]+(?:\.[0-9]+)?)"`,
		// Pattern for simple version numbers: 21.0.1
		`([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:\+[0-9]+)?)`,
		// Pattern for build info: OpenJDK Runtime Environment (build 21.0.1+12-Ubuntu-120.04)
		`build ([0-9]+\.[0-9]+(?:\.[0-9]+)?(?:\+[0-9]+)?)`,
	}
	
	for _, pattern := range patterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindStringSubmatch(output)
		if len(matches) > 1 && matches[1] != "" {
			return strings.TrimSpace(matches[1]), nil
		}
	}
	
	// If no regex patterns match, try to find any sequence that looks like a version
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "21") && 
		   (strings.Contains(strings.ToLower(line), "openjdk") || 
		    strings.Contains(strings.ToLower(line), "java")) {
			return strings.TrimSpace(line), nil
		}
	}
	
	return "", fmt.Errorf("could not parse Java version from output: %s", output)
}

// isJava21Version checks if the parsed version string indicates Java 21
func isJava21Version(version string) bool {
	if version == "" {
		return false
	}
	
	version = strings.ToLower(version)
	
	// Check for explicit Java 21 indicators
	if strings.Contains(version, "21.") || 
	   strings.Contains(version, "openjdk 21") ||
	   strings.Contains(version, "java 21") {
		return true
	}
	
	// Parse major version number
	versionRegex := regexp.MustCompile(`([0-9]+)\.`)
	matches := versionRegex.FindStringSubmatch(version)
	if len(matches) > 1 {
		return matches[1] == "21"
	}
	
	return false
}

// generateJava21Recommendations generates recommendations based on verification results
func generateJava21Recommendations(result *types.VerificationResult) {
	if !result.Installed {
		result.Recommendations = append(result.Recommendations,
			"Install Java 21 runtime using: lsp-gateway setup java21")
	}
	
	if result.Installed && !result.Compatible {
		result.Recommendations = append(result.Recommendations,
			"Install Java 21 runtime (current installation is not Java 21)")
	}
	
	if result.Installed && len(result.Issues) > 0 {
		hasPermissionIssues := false
		hasExecutionIssues := false
		
		for _, issue := range result.Issues {
			switch issue.Category {
			case types.IssueCategoryPermissions:
				hasPermissionIssues = true
			case types.IssueCategoryExecution:
				hasExecutionIssues = true
			}
		}
		
		if hasPermissionIssues {
			result.Recommendations = append(result.Recommendations,
				fmt.Sprintf("Fix Java 21 executable permissions: chmod +x %s", result.Path))
		}
		
		if hasExecutionIssues {
			result.Recommendations = append(result.Recommendations,
				"Reinstall Java 21 runtime to resolve execution issues")
		}
	}
	
	// Environment recommendations
	if result.EnvironmentVars[ENV_VAR_JAVA_HOME] == "" {
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Set JAVA_HOME environment variable to: %s", GetJava21InstallPath()))
	}
	
	// Path recommendations
	if result.Installed && result.Compatible {
		result.Recommendations = append(result.Recommendations,
			"Add Java 21 bin directory to PATH for global access")
	}
}

// addVerificationIssue adds an issue to the verification result
func addVerificationIssue(result *types.VerificationResult, severity types.IssueSeverity, category types.IssueCategory, title, description, solution string, details map[string]interface{}) {
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