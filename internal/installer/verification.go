package installer

import (
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (r *DefaultRuntimeInstaller) verifyBasicInstallation(result *types.VerificationResult, def *types.RuntimeDefinition) {
	executor := platform.NewCommandExecutor()
	
	// Check if runtime command is available
	commandName := def.Name
	commandPath := ""
	
	if def.Name == "nodejs" {
		commandName = "node"
	}
	
	// For Java, prefer JAVA_HOME if set
	if def.Name == "java" {
		if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
			possiblePath := filepath.Join(javaHome, "bin", "java")
			if platform.IsWindows() {
				possiblePath += ".exe"
			}
			if _, err := os.Stat(possiblePath); err == nil {
				commandPath = possiblePath
				commandName = possiblePath
			}
		}
	}
	
	// Check if command is available - either full path exists or command in PATH
	commandAvailable := false
	if commandPath != "" {
		// We have a specific path (like JAVA_HOME), check if it exists
		if _, err := os.Stat(commandPath); err == nil {
			commandAvailable = true
		}
	} else {
		// Check if command is available in PATH
		commandAvailable = executor.IsCommandAvailable(commandName)
	}
	
	if !commandAvailable {
		result.Installed = false
		result.Compatible = false
		r.addIssue(result, types.IssueSeverityHigh, types.IssueCategoryDependencies,
			fmt.Sprintf("%s Not Found", strings.Title(def.Name)),
			fmt.Sprintf("%s runtime is not installed or not in PATH", def.Name),
			fmt.Sprintf("Install %s runtime and ensure it's in your PATH", def.Name),
			map[string]interface{}{"runtime": def.Name})
		return
	}
	
	// Get version information
	versionArgs := []string{"-version"}
	if def.Name == "nodejs" {
		versionArgs = []string{"--version"}
	} else if def.Name == "python" {
		versionArgs = []string{"--version"}
	}
	
	versionResult, err := executor.Execute(commandName, versionArgs, 10*time.Second)
	if err != nil || versionResult.ExitCode != 0 {
		result.Installed = false
		result.Compatible = false
		r.addIssue(result, types.IssueSeverityHigh, types.IssueCategoryDependencies,
			fmt.Sprintf("%s Version Check Failed", strings.Title(def.Name)),
			fmt.Sprintf("Failed to get %s version information", def.Name),
			fmt.Sprintf("Check %s installation and try reinstalling", def.Name),
			map[string]interface{}{"runtime": def.Name, "error": err})
		return
	}
	
	// Parse version from output
	versionOutput := versionResult.Stdout
	if versionOutput == "" {
		versionOutput = versionResult.Stderr
	}
	
	version := r.parseVersionFromOutput(def.Name, versionOutput)
	if version == "" {
		result.Installed = true
		result.Compatible = false
		result.Version = "unknown"
		r.addIssue(result, types.IssueSeverityMedium, types.IssueCategoryVersion,
			fmt.Sprintf("%s Version Unknown", strings.Title(def.Name)),
			fmt.Sprintf("Could not parse %s version from output: %s", def.Name, versionOutput),
			fmt.Sprintf("Check %s installation", def.Name),
			map[string]interface{}{"runtime": def.Name, "output": versionOutput})
		return
	}
	
	// Set basic information
	result.Installed = true
	result.Version = version
	result.Runtime = def.Name
	
	// Check version compatibility
	if def.MinVersion != "" {
		compatible := r.isVersionCompatible(version, def.MinVersion)
		result.Compatible = compatible
		
		if !compatible {
			r.addIssue(result, types.IssueSeverityHigh, types.IssueCategoryVersion,
				fmt.Sprintf("%s Version Incompatible", strings.Title(def.Name)),
				fmt.Sprintf("%s version %s does not meet minimum requirement %s", def.Name, version, def.MinVersion),
				fmt.Sprintf("Upgrade %s to version %s or later", def.Name, def.MinVersion),
				map[string]interface{}{"runtime": def.Name, "current_version": version, "min_version": def.MinVersion})
		}
	} else {
		result.Compatible = true
	}
	
	// Set path information
	pathResult, err := r.getExecutablePath(commandName)
	if err == nil {
		result.Path = pathResult
	}
}

func (r *DefaultRuntimeInstaller) parseVersionFromOutput(runtime, output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return ""
	}
	
	firstLine := strings.TrimSpace(lines[0])
	
	switch runtime {
	case "java":
		// Parse Java version (e.g., "openjdk version "21.0.7"")
		re := regexp.MustCompile(`"(\d+(?:\.\d+)*(?:_\d+)?)"`)
		matches := re.FindStringSubmatch(firstLine)
		if len(matches) > 1 {
			version := matches[1]
			// Convert old format (1.8.0) to new format (8.0)
			if strings.HasPrefix(version, "1.") {
				parts := strings.SplitN(version, ".", 3)
				if len(parts) >= 2 {
					return parts[1] + strings.Join(parts[2:], ".")
				}
			}
			return version
		}
	case "python":
		// Parse Python version (e.g., "Python 3.11.0")
		re := regexp.MustCompile(`Python\s+(\d+(?:\.\d+)*)`)
		matches := re.FindStringSubmatch(firstLine)
		if len(matches) > 1 {
			return matches[1]
		}
	case "nodejs":
		// Parse Node.js version (e.g., "v18.17.0")
		re := regexp.MustCompile(`v?(\d+(?:\.\d+)*)`)
		matches := re.FindStringSubmatch(firstLine)
		if len(matches) > 1 {
			return matches[1]
		}
	case "go":
		// Parse Go version (e.g., "go version go1.21.0 linux/amd64")
		re := regexp.MustCompile(`go(\d+(?:\.\d+)*)`)
		matches := re.FindStringSubmatch(firstLine)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	
	return ""
}

func (r *DefaultRuntimeInstaller) getExecutablePath(command string) (string, error) {
	executor := platform.NewCommandExecutor()
	
	var cmd []string
	if platform.IsWindows() {
		cmd = []string{"where", command}
	} else {
		cmd = []string{"which", command}
	}
	
	result, err := executor.Execute(cmd[0], cmd[1:], 5*time.Second)
	if err != nil {
		return "", err
	}
	
	path := strings.TrimSpace(result.Stdout)
	if path == "" {
		return "", fmt.Errorf("command not found in PATH")
	}
	
	return path, nil
}

func (r *DefaultRuntimeInstaller) isVersionCompatible(currentVersion, minVersion string) bool {
	current := r.parseVersionNumbers(currentVersion)
	minimum := r.parseVersionNumbers(minVersion)
	
	// Compare version numbers
	for i := 0; i < len(current) && i < len(minimum); i++ {
		if current[i] > minimum[i] {
			return true
		}
		if current[i] < minimum[i] {
			return false
		}
	}
	
	// If all compared parts are equal, current version is compatible if it has same or more parts
	return len(current) >= len(minimum)
}

func (r *DefaultRuntimeInstaller) parseVersionNumbers(version string) []int {
	parts := strings.Split(version, ".")
	numbers := make([]int, 0, len(parts))
	
	for _, part := range parts {
		// Remove any non-numeric suffixes (e.g., "1.21.0-rc1" -> "1.21.0")
		numericPart := regexp.MustCompile(`^\d+`).FindString(part)
		if numericPart != "" {
			if num, err := strconv.Atoi(numericPart); err == nil {
				numbers = append(numbers, num)
			}
		}
	}
	
	return numbers
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
		r.addIssue(result, types.IssueSeverityHigh, types.IssueCategoryDependencies,
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
