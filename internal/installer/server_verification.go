package installer

import (
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type ServerVerifier interface {
	VerifyServer(serverName string) (*ServerVerificationResult, error)

	VerifyAllServers() (map[string]*ServerVerificationResult, error)

	HealthCheck(serverName string) (*ServerHealthResult, error)

	GetSupportedServers() []string
}

type ServerVerificationResult struct {
	ServerName      string                 `json:"server_name"`
	Installed       bool                   `json:"installed"`
	Version         string                 `json:"version"`
	Path            string                 `json:"path"`
	Compatible      bool                   `json:"compatible"`
	Functional      bool                   `json:"functional"`
	RuntimeRequired string                 `json:"runtime_required"`
	RuntimeStatus   *VerificationResult    `json:"runtime_status,omitempty"`
	Issues          []Issue                `json:"issues"`
	Recommendations []string               `json:"recommendations"`
	HealthCheck     *ServerHealthResult    `json:"health_check,omitempty"`
	VerifiedAt      time.Time              `json:"verified_at"`
	Duration        time.Duration          `json:"duration"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type ServerHealthResult struct {
	ServerName  string                 `json:"server_name"`
	Responsive  bool                   `json:"responsive"`
	StartupTime time.Duration          `json:"startup_time"`
	ExitCode    int                    `json:"exit_code"`
	Output      string                 `json:"output"`
	Error       string                 `json:"error"`
	TestedAt    time.Time              `json:"tested_at"`
	TestMethod  string                 `json:"test_method"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type DefaultServerVerifier struct {
	runtimeVerifier RuntimeInstaller
	serverRegistry  *ServerRegistry
	executor        platform.CommandExecutor
}

func NewServerVerifier(runtimeVerifier RuntimeInstaller) *DefaultServerVerifier {
	return &DefaultServerVerifier{
		runtimeVerifier: runtimeVerifier,
		serverRegistry:  NewServerRegistry(),
		executor:        platform.NewCommandExecutor(),
	}
}

func (v *DefaultServerVerifier) VerifyServer(serverName string) (*ServerVerificationResult, error) {
	startTime := time.Now()

	serverDef, err := v.serverRegistry.GetServer(serverName)
	if err != nil {
		return nil, NewInstallerError(InstallerErrorTypeNotFound, serverName,
			fmt.Sprintf("unknown server: %s", serverName), err)
	}

	result := &ServerVerificationResult{
		ServerName:      serverName,
		Installed:       false,
		Version:         "",
		Path:            "",
		Compatible:      false,
		Functional:      false,
		RuntimeRequired: serverDef.Runtime,
		Issues:          []Issue{},
		Recommendations: []string{},
		VerifiedAt:      startTime,
		Metadata:        make(map[string]interface{}),
	}

	v.verifyServerRuntime(result, serverDef)
	if !result.RuntimeStatus.Installed || !result.RuntimeStatus.Compatible {
		v.addServerIssue(result, types.IssueSeverityCritical, types.IssueCategoryDependencies,
			"Runtime Not Available",
			fmt.Sprintf("Required runtime '%s' is not installed or compatible", serverDef.Runtime),
			fmt.Sprintf("Install %s runtime before installing %s", serverDef.Runtime, serverDef.DisplayName),
			map[string]interface{}{"required_runtime": serverDef.Runtime})
		result.Duration = time.Since(startTime)
		return result, nil
	}

	v.verifyServerInstallation(result, serverDef)
	if !result.Installed {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	v.verifyServerVersion(result, serverDef)

	v.verifyServerFunctionality(result, serverDef)

	healthResult, err := v.HealthCheck(serverName)
	if err == nil {
		result.HealthCheck = healthResult
		result.Functional = healthResult.Responsive
	}

	v.generateServerRecommendations(result, serverDef)

	result.Duration = time.Since(startTime)
	return result, nil
}

func (v *DefaultServerVerifier) VerifyAllServers() (map[string]*ServerVerificationResult, error) {
	results := make(map[string]*ServerVerificationResult)
	servers := v.serverRegistry.ListServers()

	for _, serverName := range servers {
		result, err := v.VerifyServer(serverName)
		if err != nil {
			results[serverName] = &ServerVerificationResult{
				ServerName: serverName,
				Installed:  false,
				Functional: false,
				Issues: []Issue{{
					Severity:    types.IssueSeverityCritical,
					Category:    types.IssueCategoryInstallation,
					Title:       "Verification Failed",
					Description: fmt.Sprintf("Failed to verify server: %v", err),
					Solution:    "Check server installation and try again",
					Details:     map[string]interface{}{"error": err.Error()},
				}},
				VerifiedAt: time.Now(),
				Metadata:   make(map[string]interface{}),
			}
		} else {
			results[serverName] = result
		}
	}

	return results, nil
}

func (v *DefaultServerVerifier) HealthCheck(serverName string) (*ServerHealthResult, error) {
	startTime := time.Now()

	result := &ServerHealthResult{
		ServerName: serverName,
		Responsive: false,
		ExitCode:   -1,
		TestedAt:   startTime,
		Metadata:   make(map[string]interface{}),
	}

	switch serverName {
	case ServerGopls:
		v.healthCheckGopls(result)
	case ServerPylsp:
		v.healthCheckPylsp(result)
	case ServerTypeScriptLanguageServer:
		v.healthCheckTypeScriptLS(result)
	case ServerJDTLS:
		v.healthCheckJdtls(result)
	default:
		return nil, fmt.Errorf("health check not implemented for server: %s", serverName)
	}

	result.StartupTime = time.Since(startTime)
	return result, nil
}

func (v *DefaultServerVerifier) GetSupportedServers() []string {
	return v.serverRegistry.ListServers()
}

func (v *DefaultServerVerifier) verifyServerRuntime(result *ServerVerificationResult, serverDef *ServerDefinition) {
	runtimeResult, err := v.runtimeVerifier.Verify(serverDef.Runtime)
	if err != nil {
		v.addServerIssue(result, types.IssueSeverityCritical, types.IssueCategoryDependencies,
			"Runtime Verification Failed",
			fmt.Sprintf("Failed to verify required runtime '%s': %v", serverDef.Runtime, err),
			fmt.Sprintf("Install and configure %s runtime", serverDef.Runtime),
			map[string]interface{}{"runtime": serverDef.Runtime, "error": err.Error()})
		return
	}

	result.RuntimeStatus = runtimeResult
	result.Metadata["runtime_info"] = runtimeResult
}

func (v *DefaultServerVerifier) verifyServerInstallation(result *ServerVerificationResult, serverDef *ServerDefinition) {
	switch serverDef.Name {
	case ServerGopls:
		v.verifyGoplsInstallation(result, serverDef)
	case ServerPylsp:
		v.verifyPylspInstallation(result, serverDef)
	case ServerTypeScriptLanguageServer:
		v.verifyTypeScriptLSInstallation(result, serverDef)
	case ServerJDTLS:
		v.verifyJdtlsInstallation(result)
	default:
		v.addServerIssue(result, types.IssueSeverityCritical, types.IssueCategoryInstallation,
			"Unsupported Server",
			fmt.Sprintf("Verification not implemented for server: %s", serverDef.Name),
			"Contact support for help with this server type",
			map[string]interface{}{"server": serverDef.Name})
	}
}

func (v *DefaultServerVerifier) verifyGoplsInstallation(result *ServerVerificationResult, serverDef *ServerDefinition) {
	if !platform.IsCommandAvailable("gopls") {
		v.addServerIssue(result, types.IssueSeverityCritical, types.IssueCategoryInstallation,
			"Gopls Not Found",
			"gopls is not available in PATH",
			"Install gopls using: go install golang.org/x/tools/gopls@latest",
			map[string]interface{}{"install_command": serverDef.InstallCmd})
		return
	}

	goplsPath, err := v.executor.Execute("which", []string{"gopls"}, 5*time.Second)
	if err == nil && goplsPath.ExitCode == 0 {
		result.Path = strings.TrimSpace(goplsPath.Stdout)
		result.Metadata["executable_path"] = result.Path
	}

	if result.Path != "" {
		if info, err := os.Stat(result.Path); err == nil {
			if !info.Mode().IsRegular() || (!platform.IsWindows() && info.Mode()&0111 == 0) {
				v.addServerIssue(result, types.IssueSeverityHigh, types.IssueCategoryPermissions,
					"Gopls Not Executable",
					"gopls file exists but is not executable",
					"Check file permissions: chmod +x "+result.Path,
					map[string]interface{}{"path": result.Path})
				return
			}
			result.Metadata["file_info"] = map[string]interface{}{
				"size":     info.Size(),
				"mode":     info.Mode().String(),
				"mod_time": info.ModTime(),
			}
		}
	}

	result.Installed = true
}

func (v *DefaultServerVerifier) verifyPylspInstallation(result *ServerVerificationResult, serverDef *ServerDefinition) {
	if !platform.IsCommandAvailable("pylsp") {
		v.addServerIssue(result, types.IssueSeverityCritical, types.IssueCategoryInstallation,
			"Pylsp Not Found",
			"pylsp is not available in PATH",
			"Install pylsp using: pip install python-lsp-server",
			map[string]interface{}{"install_command": serverDef.InstallCmd})
		return
	}

	pylspPath, err := v.executor.Execute("which", []string{"pylsp"}, 5*time.Second)
	if err == nil && pylspPath.ExitCode == 0 {
		result.Path = strings.TrimSpace(pylspPath.Stdout)
		result.Metadata["executable_path"] = result.Path
	}

	result.Installed = true
}

func (v *DefaultServerVerifier) verifyTypeScriptLSInstallation(result *ServerVerificationResult, serverDef *ServerDefinition) {
	if !platform.IsCommandAvailable("typescript-language-server") {
		v.addServerIssue(result, types.IssueSeverityCritical, types.IssueCategoryInstallation,
			"TypeScript Language Server Not Found",
			"typescript-language-server is not available in PATH",
			"Install using: npm install -g typescript-language-server typescript",
			map[string]interface{}{"install_command": serverDef.InstallCmd})
		return
	}

	tsPath, err := v.executor.Execute("which", []string{"typescript-language-server"}, 5*time.Second)
	if err == nil && tsPath.ExitCode == 0 {
		result.Path = strings.TrimSpace(tsPath.Stdout)
		result.Metadata["executable_path"] = result.Path
	}

	if !platform.IsCommandAvailable("tsc") {
		v.addServerIssue(result, types.IssueSeverityMedium, types.IssueCategoryDependencies,
			"TypeScript Compiler Missing",
			"TypeScript compiler (tsc) is not available",
			"Install TypeScript: npm install -g typescript",
			map[string]interface{}{"suggestion": "npm install -g typescript"})
	}

	result.Installed = true
}

func (v *DefaultServerVerifier) verifyJdtlsInstallation(result *ServerVerificationResult) {
	// Use the actual installation path first, then fallback to legacy paths
	actualInstallPath := getJDTLSInstallPath()
	possiblePaths := []string{
		actualInstallPath, // Primary installation path
		filepath.Join(os.Getenv("HOME"), ".local", "share", "eclipse.jdt.ls"),
		filepath.Join(os.Getenv("HOME"), ".eclipse", "jdt-language-server"),
		"/opt/eclipse.jdt.ls",
		"/usr/local/share/eclipse.jdt.ls",
	}

	if platform.IsWindows() {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			possiblePaths = append(possiblePaths, filepath.Join(appData, "eclipse.jdt.ls"))
		}
	}

	var foundPath string
	for _, path := range possiblePaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			jarPattern := filepath.Join(path, "plugins", "org.eclipse.jdt.ls.core_*.jar")
			matches, err := filepath.Glob(jarPattern)
			if err == nil && len(matches) > 0 {
				foundPath = matches[0]
				result.Path = foundPath
				result.Metadata["jar_path"] = foundPath
				result.Metadata["installation_dir"] = path
				break
			}
		}
	}

	if foundPath == "" {
		v.addServerIssue(result, types.IssueSeverityCritical, types.IssueCategoryInstallation,
			"JDTLS Not Found",
			"Eclipse JDT Language Server is not installed in common locations",
			"Download and extract JDTLS from Eclipse releases",
			map[string]interface{}{
				"searched_paths": possiblePaths,
				"download_url":   "https://download.eclipse.org/jdtls/",
			})
		return
	}

	result.Installed = true
}

func (v *DefaultServerVerifier) verifyServerVersion(result *ServerVerificationResult, serverDef *ServerDefinition) {
	if !result.Installed {
		return
	}

	switch serverDef.Name {
	case ServerGopls:
		v.verifyGoplsVersion(result)
	case ServerPylsp:
		v.verifyPylspVersion(result)
	case ServerTypeScriptLanguageServer:
		v.verifyTypeScriptLSVersion(result)
	case ServerJDTLS:
		v.verifyJdtlsVersion(result)
	}
}

func (v *DefaultServerVerifier) verifyGoplsVersion(result *ServerVerificationResult) {
	versionResult, err := v.executor.Execute("gopls", []string{"version"}, 10*time.Second)
	if err != nil || versionResult.ExitCode != 0 {
		v.addServerIssue(result, types.IssueSeverityMedium, types.IssueCategoryExecution,
			"Gopls Version Check Failed",
			"Failed to get gopls version information",
			"Reinstall gopls using: go install golang.org/x/tools/gopls@latest",
			map[string]interface{}{"error": err, "stderr": versionResult.Stderr})
		return
	}

	output := strings.TrimSpace(versionResult.Stdout)
	result.Version = output
	result.Metadata["version_output"] = output

	versionRegex := regexp.MustCompile(`gopls v(\d+\.\d+\.\d+)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) > 1 {
		result.Version = matches[1]
		result.Compatible = true // Most gopls versions are compatible
		result.Metadata["parsed_version"] = matches[1]
	} else {
		v.addServerIssue(result, types.IssueSeverityLow, IssueCategoryVersion,
			"Gopls Version Parse Failed",
			"Could not parse gopls version from output",
			"This may indicate an unusual gopls installation",
			map[string]interface{}{"version_output": output})
	}
}

func (v *DefaultServerVerifier) verifyPylspVersion(result *ServerVerificationResult) {
	versionResult, err := v.executor.Execute("pylsp", []string{"--version"}, 10*time.Second)
	if err != nil || versionResult.ExitCode != 0 {
		v.addServerIssue(result, types.IssueSeverityMedium, types.IssueCategoryExecution,
			"Pylsp Version Check Failed",
			"Failed to get pylsp version information",
			"Reinstall pylsp using: pip install python-lsp-server",
			map[string]interface{}{"error": err, "stderr": versionResult.Stderr})
		return
	}

	output := strings.TrimSpace(versionResult.Stdout)
	result.Version = output
	result.Metadata["version_output"] = output
	result.Compatible = true // Most pylsp versions are compatible
}

func (v *DefaultServerVerifier) verifyTypeScriptLSVersion(result *ServerVerificationResult) {
	versionResult, err := v.executor.Execute("typescript-language-server", []string{"--version"}, 10*time.Second)
	if err != nil || versionResult.ExitCode != 0 {
		v.addServerIssue(result, types.IssueSeverityMedium, types.IssueCategoryExecution,
			"TypeScript Language Server Version Check Failed",
			"Failed to get typescript-language-server version",
			"Reinstall using: npm install -g typescript-language-server",
			map[string]interface{}{"error": err, "stderr": versionResult.Stderr})
		return
	}

	output := strings.TrimSpace(versionResult.Stdout)
	result.Version = output
	result.Metadata["version_output"] = output
	result.Compatible = true // Most versions are compatible
}

func (v *DefaultServerVerifier) verifyJdtlsVersion(result *ServerVerificationResult) {
	if result.Path == "" {
		return
	}

	installDir := result.Metadata["installation_dir"].(string)
	if installDir != "" {
		versionFile := filepath.Join(installDir, "VERSION")
		if content, err := os.ReadFile(versionFile); err == nil {
			result.Version = strings.TrimSpace(string(content))
			result.Compatible = true
			result.Metadata["version_source"] = "version_file"
		} else {
			if strings.Contains(installDir, "jdt") {
				result.Version = StatusUnknown
				result.Compatible = true
				result.Metadata["version_source"] = "installation_detected"
			}
		}
	}
}

func (v *DefaultServerVerifier) verifyServerFunctionality(result *ServerVerificationResult, serverDef *ServerDefinition) {
	if !result.Installed {
		return
	}

	switch serverDef.Name {
	case ServerGopls:
		v.testGoplsFunctionality(result)
	case ServerPylsp:
		v.testPylspFunctionality(result)
	case ServerTypeScriptLanguageServer:
		v.testTypeScriptLSFunctionality(result)
	case ServerJDTLS:
		v.testJdtlsFunctionality(result)
	}
}

func (v *DefaultServerVerifier) testGoplsFunctionality(result *ServerVerificationResult) {
	helpResult, err := v.executor.Execute("gopls", []string{"help"}, 5*time.Second)
	if err != nil || helpResult.ExitCode != 0 {
		v.addServerIssue(result, types.IssueSeverityMedium, types.IssueCategoryExecution,
			"Gopls Help Command Failed",
			"gopls help command failed to execute",
			"Check gopls installation integrity",
			map[string]interface{}{"error": err, "stderr": helpResult.Stderr})
		return
	}

	result.Functional = true
	result.Metadata["help_output"] = strings.TrimSpace(helpResult.Stdout)
}

func (v *DefaultServerVerifier) testPylspFunctionality(result *ServerVerificationResult) {
	helpResult, err := v.executor.Execute("pylsp", []string{"--help"}, 5*time.Second)
	if err != nil || helpResult.ExitCode != 0 {
		v.addServerIssue(result, types.IssueSeverityMedium, types.IssueCategoryExecution,
			"Pylsp Help Command Failed",
			"pylsp help command failed to execute",
			"Check pylsp installation and Python environment",
			map[string]interface{}{"error": err, "stderr": helpResult.Stderr})
		return
	}

	result.Functional = true
	result.Metadata["help_output"] = strings.TrimSpace(helpResult.Stdout)
}

func (v *DefaultServerVerifier) testTypeScriptLSFunctionality(result *ServerVerificationResult) {
	helpResult, err := v.executor.Execute("typescript-language-server", []string{"--help"}, 5*time.Second)
	if err != nil || helpResult.ExitCode != 0 {
		v.addServerIssue(result, types.IssueSeverityMedium, types.IssueCategoryExecution,
			"TypeScript Language Server Help Failed",
			"typescript-language-server help command failed",
			"Check installation and Node.js environment",
			map[string]interface{}{"error": err, "stderr": helpResult.Stderr})
		return
	}

	result.Functional = true
	result.Metadata["help_output"] = strings.TrimSpace(helpResult.Stdout)
}

func (v *DefaultServerVerifier) testJdtlsFunctionality(result *ServerVerificationResult) {
	if result.Path == "" {
		return
	}

	javaResult, err := v.executor.Execute("java", []string{"-jar", result.Path, "--help"}, 10*time.Second)
	if err != nil || javaResult.ExitCode != 0 {
		v.addServerIssue(result, types.IssueSeverityMedium, types.IssueCategoryExecution,
			"JDTLS Execution Test Failed",
			"Failed to execute JDTLS JAR file",
			"Check Java installation and JDTLS JAR integrity",
			map[string]interface{}{"error": err, "jar_path": result.Path})
		return
	}

	result.Functional = true
	result.Metadata["execution_test"] = StatusPassed
}

func (v *DefaultServerVerifier) healthCheckGopls(result *ServerHealthResult) {
	result.TestMethod = CheckTypeVersion

	versionResult, err := v.executor.Execute("gopls", []string{"version"}, 10*time.Second)
	result.ExitCode = versionResult.ExitCode
	result.Output = versionResult.Stdout

	if err != nil {
		result.Error = err.Error()
		result.Responsive = false
	} else if versionResult.ExitCode == 0 {
		result.Responsive = true
	} else {
		result.Error = versionResult.Stderr
		result.Responsive = false
	}
}

func (v *DefaultServerVerifier) healthCheckPylsp(result *ServerHealthResult) {
	result.TestMethod = "help_check"

	helpResult, err := v.executor.Execute("pylsp", []string{"--help"}, 10*time.Second)
	result.ExitCode = helpResult.ExitCode
	result.Output = helpResult.Stdout

	if err != nil {
		result.Error = err.Error()
		result.Responsive = false
	} else if helpResult.ExitCode == 0 {
		result.Responsive = true
	} else {
		result.Error = helpResult.Stderr
		result.Responsive = false
	}
}

func (v *DefaultServerVerifier) healthCheckTypeScriptLS(result *ServerHealthResult) {
	result.TestMethod = CheckTypeVersion

	versionResult, err := v.executor.Execute("typescript-language-server", []string{"--version"}, 10*time.Second)
	result.ExitCode = versionResult.ExitCode
	result.Output = versionResult.Stdout

	if err != nil {
		result.Error = err.Error()
		result.Responsive = false
	} else if versionResult.ExitCode == 0 {
		result.Responsive = true
	} else {
		result.Error = versionResult.Stderr
		result.Responsive = false
	}
}

func (v *DefaultServerVerifier) healthCheckJdtls(result *ServerHealthResult) {
	result.TestMethod = "installation_check"

	// Use the actual installation path first, then fallback to legacy paths
	actualInstallPath := getJDTLSInstallPath()
	possiblePaths := []string{
		actualInstallPath, // Primary installation path
		filepath.Join(os.Getenv("HOME"), ".local", "share", "eclipse.jdt.ls"),
		"/opt/eclipse.jdt.ls",
	}

	for _, path := range possiblePaths {
		jarPattern := filepath.Join(path, "plugins", "org.eclipse.jdt.ls.core_*.jar")
		matches, err := filepath.Glob(jarPattern)
		if err == nil && len(matches) > 0 {
			result.Responsive = true
			result.ExitCode = 0
			result.Output = fmt.Sprintf("Found JDTLS at: %s", matches[0])
			return
		}
	}

	result.Responsive = false
	result.ExitCode = 1
	result.Error = "JDTLS installation not found"
}

func (v *DefaultServerVerifier) generateServerRecommendations(result *ServerVerificationResult, serverDef *ServerDefinition) {
	if !result.Installed {
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Install %s using: %s", serverDef.DisplayName, serverDef.InstallCmd))
	}

	if !result.RuntimeStatus.Compatible {
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Upgrade %s runtime to meet minimum requirements", serverDef.Runtime))
	}

	if result.Installed && !result.Functional {
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Verify %s installation integrity and reinstall if necessary", serverDef.DisplayName))
	}

	switch serverDef.Name {
	case ServerTypeScriptLanguageServer:
		if !platform.IsCommandAvailable("tsc") {
			result.Recommendations = append(result.Recommendations,
				"Install TypeScript compiler: npm install -g typescript")
		}
	case ServerJDTLS:
		if result.Installed && len(result.Issues) > 0 {
			result.Recommendations = append(result.Recommendations,
				"Ensure JAVA_HOME is set and points to a valid JDK installation")
		}
	}
}

func (v *DefaultServerVerifier) addServerIssue(result *ServerVerificationResult, severity IssueSeverity, category IssueCategory, title, description, solution string, details map[string]interface{}) {
	issue := Issue{
		Severity:    severity,
		Category:    category,
		Title:       title,
		Description: description,
		Solution:    solution,
		Details:     details,
	}
	result.Issues = append(result.Issues, issue)
}
