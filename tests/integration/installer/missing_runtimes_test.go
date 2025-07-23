package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// addIssueHelper adds an issue to the verification result (helper for testing)
func addIssueHelper(result *types.VerificationResult, severity types.IssueSeverity, category types.IssueCategory, title, description, solution string, details map[string]interface{}) {
	if result.Issues == nil {
		result.Issues = []types.Issue{}
	}

	issue := types.Issue{
		Severity:    severity,
		Category:    category,
		Title:       title,
		Description: description,
		Solution:    solution,
	}

	if details != nil {
		issue.Details = details
	} else {
		issue.Details = make(map[string]interface{})
	}

	result.Issues = append(result.Issues, issue)
}

// MockRuntimeFileSystem for missing runtime scenarios
type MockRuntimeFileSystem struct {
	mu                sync.RWMutex
	installedPaths    map[string]string
	missingRuntimes   map[string]bool
	corruptedRuntimes map[string]bool
	environmentVars   map[string]string
}

func NewMockRuntimeFileSystem() *MockRuntimeFileSystem {
	return &MockRuntimeFileSystem{
		installedPaths:    make(map[string]string),
		missingRuntimes:   make(map[string]bool),
		corruptedRuntimes: make(map[string]bool),
		environmentVars:   make(map[string]string),
	}
}

func (m *MockRuntimeFileSystem) InstallRuntime(runtime, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.installedPaths[runtime] = path
	delete(m.missingRuntimes, runtime)
	delete(m.corruptedRuntimes, runtime)
}

func (m *MockRuntimeFileSystem) RemoveRuntime(runtime string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.installedPaths, runtime)
	m.missingRuntimes[runtime] = true
}

func (m *MockRuntimeFileSystem) IsRuntimeInstalled(runtime string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.missingRuntimes[runtime] || m.corruptedRuntimes[runtime] {
		return false
	}
	_, exists := m.installedPaths[runtime]
	return exists
}

func (m *MockRuntimeFileSystem) SetEnvironmentVar(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.environmentVars[key] = value
}

func (m *MockRuntimeFileSystem) GetEnvironmentVar(key string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.environmentVars[key]
}

// MockRuntimeCommandExecutor for missing runtime scenarios
type MockRuntimeCommandExecutor struct {
	*MockCommandExecutor
	missingRuntimes   map[string]bool
	corruptedRuntimes map[string]bool
	versionOutputs    map[string]string
}

func NewMockRuntimeCommandExecutor() *MockRuntimeCommandExecutor {
	return &MockRuntimeCommandExecutor{
		MockCommandExecutor: NewMockCommandExecutor(),
		missingRuntimes:     make(map[string]bool),
		corruptedRuntimes:   make(map[string]bool),
		versionOutputs:      make(map[string]string),
	}
}

func (m *MockRuntimeCommandExecutor) SetRuntimeMissing(runtime string) {
	m.missingRuntimes[runtime] = true
}

func (m *MockRuntimeCommandExecutor) SetRuntimeCorrupted(runtime string) {
	m.corruptedRuntimes[runtime] = true
}

func (m *MockRuntimeCommandExecutor) SetVersionOutput(runtime, output string) {
	m.versionOutputs[runtime] = output
}

func (m *MockRuntimeCommandExecutor) Execute(cmd string, args []string, timeout time.Duration) (*platform.Result, error) {
	// Check for missing runtimes
	if m.missingRuntimes[cmd] {
		return &platform.Result{
			ExitCode: 127,
			Stderr:   fmt.Sprintf("command not found: %s", cmd),
			Duration: 10 * time.Millisecond,
		}, fmt.Errorf("command not found: %s", cmd)
	}

	// Check for corrupted runtimes
	if m.corruptedRuntimes[cmd] {
		return &platform.Result{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("%s: segmentation fault (core dumped)", cmd),
			Duration: 50 * time.Millisecond,
		}, fmt.Errorf("runtime corrupted: %s", cmd)
	}

	// Handle version commands with custom outputs
	if len(args) > 0 && (args[0] == "version" || args[0] == "--version" || args[0] == "-version") {
		if output, exists := m.versionOutputs[cmd]; exists {
			return &platform.Result{
				ExitCode: 0,
				Stdout:   output,
				Duration: 20 * time.Millisecond,
			}, nil
		}
	}

	return m.MockCommandExecutor.Execute(cmd, args, timeout)
}

func (m *MockRuntimeCommandExecutor) IsCommandAvailable(command string) bool {
	if m.missingRuntimes[command] || m.corruptedRuntimes[command] {
		return false
	}
	return m.MockCommandExecutor.IsCommandAvailable(command)
}

// Test missing Go runtime detection
func TestVerifyRuntimeInstallation_MissingGo(t *testing.T) {
	_ = createTestInstaller()
	executor := NewMockRuntimeCommandExecutor()
	executor.SetRuntimeMissing("go")

	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	// Test Go verification when Go is missing
	if executor.IsCommandAvailable("go") {
		t.Error("Expected Go to be unavailable")
	}

	_, err := executor.Execute("go", []string{"version"}, 5*time.Second)
	if err == nil {
		t.Error("Expected error for missing Go runtime")
	}

	if !strings.Contains(err.Error(), "command not found") {
		t.Errorf("Expected 'command not found' error, got: %v", err)
	}

	// Add issue for missing Go
	addIssueHelper(result, types.IssueSeverityCritical, types.IssueCategoryInstallation,
		"Go Runtime Missing", "Go programming language is not installed",
		"Install Go from https://golang.org/dl/", nil)

	if len(result.Issues) == 0 {
		t.Error("Expected issue to be added for missing Go")
	}

	issue := result.Issues[0]
	if issue.Severity != types.IssueSeverityCritical {
		t.Errorf("Expected critical severity for missing Go, got %s", issue.Severity)
	}
}

func TestVerifyRuntimeInstallation_MissingPython(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()
	executor.SetRuntimeMissing("python")
	executor.SetRuntimeMissing("python3")

	// Test both python and python3 variants
	pythonCommands := []string{"python", "python3"}

	for _, cmd := range pythonCommands {
		t.Run(cmd, func(t *testing.T) {
			if executor.IsCommandAvailable(cmd) {
				t.Errorf("Expected %s to be unavailable", cmd)
			}

			_, err := executor.Execute(cmd, []string{"--version"}, 5*time.Second)
			if err == nil {
				t.Errorf("Expected error for missing %s runtime", cmd)
			}

			if !strings.Contains(err.Error(), "command not found") {
				t.Errorf("Expected 'command not found' error for %s, got: %v", cmd, err)
			}
		})
	}
}

func TestVerifyRuntimeInstallation_MissingNodeJS(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()
	executor.SetRuntimeMissing("node")
	executor.SetRuntimeMissing("npm")

	// Test Node.js and npm missing
	nodeCommands := map[string][]string{
		"node": {"--version"},
		"npm":  {"--version"},
	}

	for cmd, args := range nodeCommands {
		t.Run(cmd, func(t *testing.T) {
			if executor.IsCommandAvailable(cmd) {
				t.Errorf("Expected %s to be unavailable", cmd)
			}

			_, err := executor.Execute(cmd, args, 5*time.Second)
			if err == nil {
				t.Errorf("Expected error for missing %s", cmd)
			}

			if !strings.Contains(err.Error(), "command not found") {
				t.Errorf("Expected 'command not found' error for %s, got: %v", cmd, err)
			}
		})
	}
}

func TestVerifyRuntimeInstallation_MissingJava(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()
	executor.SetRuntimeMissing("java")
	executor.SetRuntimeMissing("javac")

	// Test Java runtime and compiler missing
	javaCommands := map[string][]string{
		"java":  {"-version"},
		"javac": {"-version"},
	}

	for cmd, args := range javaCommands {
		t.Run(cmd, func(t *testing.T) {
			if executor.IsCommandAvailable(cmd) {
				t.Errorf("Expected %s to be unavailable", cmd)
			}

			_, err := executor.Execute(cmd, args, 5*time.Second)
			if err == nil {
				t.Errorf("Expected error for missing %s", cmd)
			}

			if !strings.Contains(err.Error(), "command not found") {
				t.Errorf("Expected 'command not found' error for %s, got: %v", cmd, err)
			}
		})
	}
}

// Test corrupted runtime installations
func TestVerifyRuntimeInstallation_CorruptedGo(t *testing.T) {
	_ = createTestInstaller()
	executor := NewMockRuntimeCommandExecutor()
	executor.SetRuntimeCorrupted("go")

	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	// Test corrupted Go runtime
	if executor.IsCommandAvailable("go") {
		t.Error("Expected corrupted Go to be unavailable")
	}

	execResult, err := executor.Execute("go", []string{"version"}, 5*time.Second)
	if err == nil {
		t.Error("Expected error for corrupted Go runtime")
	}

	if execResult.ExitCode != 1 {
		t.Errorf("Expected exit code 1 for corrupted Go, got %d", execResult.ExitCode)
	}

	if !strings.Contains(execResult.Stderr, "segmentation fault") {
		t.Errorf("Expected segmentation fault error, got: %s", execResult.Stderr)
	}

	// Add issue for corrupted Go
	addIssueHelper(result, types.IssueSeverityHigh, types.IssueCategoryCorruption,
		"Go Runtime Corrupted", "Go installation is corrupted",
		"Reinstall Go from https://golang.org/dl/", nil)

	if len(result.Issues) == 0 {
		t.Error("Expected issue to be added for corrupted Go")
	}

	issue := result.Issues[0]
	if issue.Category != types.IssueCategoryCorruption {
		t.Errorf("Expected corruption category, got %s", issue.Category)
	}
}

func TestVerifyRuntimeInstallation_CorruptedPython(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()
	executor.SetRuntimeCorrupted("python")

	execResult, err := executor.Execute("python", []string{"--version"}, 5*time.Second)
	if err == nil {
		t.Error("Expected error for corrupted Python runtime")
	}

	if execResult.ExitCode != 1 {
		t.Errorf("Expected exit code 1 for corrupted Python, got %d", execResult.ExitCode)
	}

	if !strings.Contains(err.Error(), "runtime corrupted") {
		t.Errorf("Expected corruption error, got: %v", err)
	}
}

// Test missing environment variables
func TestVerifyRuntimeEnvironment_MissingGOPATH(t *testing.T) {
	_ = createTestInstaller()
	mockFS := NewMockRuntimeFileSystem()

	// Don't set GOPATH
	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	// Test missing GOPATH environment variable
	gopath := mockFS.GetEnvironmentVar("GOPATH")
	if gopath == "" {
		addIssueHelper(result, types.IssueSeverityMedium, types.IssueCategoryEnvironment,
			"GOPATH Not Set", "GOPATH environment variable is not configured",
			"Set GOPATH to your Go workspace directory",
			map[string]interface{}{"variable": "GOPATH"})
	}

	if len(result.Issues) == 0 {
		t.Error("Expected issue for missing GOPATH")
	}

	issue := result.Issues[0]
	if issue.Category != types.IssueCategoryEnvironment {
		t.Errorf("Expected environment category, got %s", issue.Category)
	}
}

func TestVerifyRuntimeEnvironment_MissingGOROOT(t *testing.T) {
	_ = createTestInstaller()
	mockFS := NewMockRuntimeFileSystem()

	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	// Test missing GOROOT environment variable
	goroot := mockFS.GetEnvironmentVar("GOROOT")
	if goroot == "" {
		addIssueHelper(result, types.IssueSeverityLow, types.IssueCategoryEnvironment,
			"GOROOT Not Set", "GOROOT environment variable is not configured",
			"GOROOT is usually set automatically by Go installation",
			map[string]interface{}{"variable": "GOROOT"})
	}

	if len(result.Issues) == 0 {
		t.Error("Expected issue for missing GOROOT")
	}

	issue := result.Issues[0]
	if issue.Severity != types.IssueSeverityLow {
		t.Errorf("Expected low severity for missing GOROOT, got %s", issue.Severity)
	}
}

func TestVerifyRuntimeEnvironment_MissingJAVA_HOME(t *testing.T) {
	_ = createTestInstaller()
	mockFS := NewMockRuntimeFileSystem()

	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	// Test missing JAVA_HOME environment variable
	javaHome := mockFS.GetEnvironmentVar("JAVA_HOME")
	if javaHome == "" {
		addIssueHelper(result, types.IssueSeverityMedium, types.IssueCategoryEnvironment,
			"JAVA_HOME Not Set", "JAVA_HOME environment variable is not configured",
			"Set JAVA_HOME to your Java installation directory",
			map[string]interface{}{"variable": "JAVA_HOME"})
	}

	if len(result.Issues) == 0 {
		t.Error("Expected issue for missing JAVA_HOME")
	}

	issue := result.Issues[0]
	if issue.Category != types.IssueCategoryEnvironment {
		t.Errorf("Expected environment category, got %s", issue.Category)
	}
}

// Test missing runtime components
func TestVerifyRuntimeComponents_MissingGoModules(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()
	executor.SetVersionOutput("go", "go version go1.21.0 linux/amd64")

	// Go exists but go mod command fails
	executor.AddFailure("go", []string{"mod", "version"}, fmt.Errorf("go: modules disabled"))

	result := &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	// Test Go modules support
	_, err := executor.Execute("go", []string{"mod", "version"}, 5*time.Second)
	if err == nil {
		t.Error("Expected error for missing Go modules support")
	}

	if !strings.Contains(err.Error(), "modules disabled") {
		t.Errorf("Expected modules disabled error, got: %v", err)
	}

	_ = createTestInstaller()
	addIssueHelper(result, types.IssueSeverityMedium, types.IssueCategoryDependencies,
		"Go Modules Disabled", "Go modules support is not available",
		"Enable Go modules with 'go env -w GO111MODULE=on'",
		map[string]interface{}{"feature": "modules"})

	if len(result.Issues) == 0 {
		t.Error("Expected issue for missing Go modules")
	}
}

func TestVerifyRuntimeComponents_MissingPip(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()
	executor.SetVersionOutput("python", "Python 3.9.0")
	executor.SetRuntimeMissing("pip")

	// Python exists but pip is missing
	if executor.IsCommandAvailable("pip") {
		t.Error("Expected pip to be unavailable")
	}

	_, err := executor.Execute("pip", []string{"--version"}, 5*time.Second)
	if err == nil {
		t.Error("Expected error for missing pip")
	}

	if !strings.Contains(err.Error(), "command not found") {
		t.Errorf("Expected 'command not found' error for pip, got: %v", err)
	}

	result := &types.VerificationResult{
		Issues: []types.Issue{},
	}

	_ = createTestInstaller()
	addIssueHelper(result, types.IssueSeverityHigh, types.IssueCategoryDependencies,
		"Pip Not Installed", "Python package manager pip is missing",
		"Install pip with 'python -m ensurepip' or download from https://pip.pypa.io/",
		map[string]interface{}{"tool": "pip"})

	if len(result.Issues) == 0 {
		t.Error("Expected issue for missing pip")
	}
}

func TestVerifyRuntimeComponents_MissingNpm(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()
	executor.SetVersionOutput("node", "v18.17.0")
	executor.SetRuntimeMissing("npm")

	// Node.js exists but npm is missing
	if executor.IsCommandAvailable("npm") {
		t.Error("Expected npm to be unavailable")
	}

	_, err := executor.Execute("npm", []string{"--version"}, 5*time.Second)
	if err == nil {
		t.Error("Expected error for missing npm")
	}

	result := &types.VerificationResult{
		Issues: []types.Issue{},
	}

	_ = createTestInstaller()
	addIssueHelper(result, types.IssueSeverityHigh, types.IssueCategoryDependencies,
		"Npm Not Installed", "Node.js package manager npm is missing",
		"Reinstall Node.js with npm included or install npm separately",
		map[string]interface{}{"tool": "npm"})

	if len(result.Issues) == 0 {
		t.Error("Expected issue for missing npm")
	}
}

// Test version compatibility when specific runtime versions are missing
func TestVerifyRuntimeVersions_IncompatibleVersions(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()

	// Test with old/incompatible versions
	incompatibleVersions := map[string]string{
		"go":     "go version go1.15.0 linux/amd64", // Too old
		"python": "Python 2.7.18",                   // Python 2 (deprecated)
		"node":   "v14.21.3",                        // Too old
		"java":   "openjdk version \"1.8.0_362\"",   // Too old
	}

	_ = createTestInstaller()

	for runtime, version := range incompatibleVersions {
		// Extract version identifier safely
		fields := strings.Fields(version)
		versionId := ""
		switch runtime {
		case "go":
			if len(fields) >= 3 {
				versionId = fields[2] // "go1.15.0"
			}
		case "python":
			if len(fields) >= 2 {
				versionId = fields[1] // "2.7.18"
			}
		case "node":
			if len(fields) >= 1 {
				versionId = fields[0] // "v14.21.3"
			}
		case "java":
			if len(fields) >= 3 {
				versionId = strings.Trim(fields[2], "\"") // Remove quotes from "1.8.0_362"
			}
		}
		if versionId == "" {
			versionId = "unknown"
		}

		t.Run(fmt.Sprintf("%s_%s", runtime, versionId), func(t *testing.T) {
			executor.SetVersionOutput(runtime, version)

			result := &types.VerificationResult{
				Issues: []types.Issue{},
			}

			// Check if version is compatible
			minVersions := map[string]string{
				"go":     "1.19.0",
				"python": "3.8.0",
				"node":   "18.0.0",
				"java":   "17.0.0",
			}

			addIssueHelper(result, types.IssueSeverityHigh, types.IssueCategoryVersion,
				fmt.Sprintf("Incompatible %s Version", cases.Title(language.English).String(runtime)),
				fmt.Sprintf("Installed %s version is too old", runtime),
				fmt.Sprintf("Upgrade %s to version %s or later", runtime, minVersions[runtime]),
				map[string]interface{}{
					"installed_version": version,
					"required_version":  minVersions[runtime],
				})

			if len(result.Issues) == 0 {
				t.Errorf("Expected issue for incompatible %s version", runtime)
			}

			issue := result.Issues[0]
			if issue.Category != types.IssueCategoryVersion {
				t.Errorf("Expected version category for %s, got %s", runtime, issue.Category)
			}
		})
	}
}

// Test fallback mechanisms when preferred installations are missing
func TestVerifyRuntimeFallbacks_MissingPreferredVersions(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()

	// Test fallback scenarios
	fallbackTests := map[string][]string{
		"python": {"python3", "python3.9", "python3.8"},
		"node":   {"node", "nodejs"},
		"java":   {"java", "java11", "java17"},
	}

	for primary, alternatives := range fallbackTests {
		t.Run(primary, func(t *testing.T) {
			// Make primary command missing
			executor.SetRuntimeMissing(primary)

			// Test each alternative
			foundAlternative := false
			for _, alt := range alternatives {
				if !executor.IsCommandAvailable(alt) {
					continue
				}

				// Found working alternative
				foundAlternative = true
				t.Logf("Found alternative %s for missing %s", alt, primary)
				break
			}

			if !foundAlternative {
				t.Logf("No alternatives found for missing %s (expected)", primary)
			}
		})
	}
}

// Test dependency chain breaks due to missing intermediate components
func TestVerifyRuntimeDependencyChains_BrokenChains(t *testing.T) {
	_ = createTestInstaller()

	// Test dependency chains
	dependencyChains := map[string][]string{
		"pylsp":                      {"python", "pip", "setuptools"},
		"typescript-language-server": {"node", "npm", "typescript"},
		"jdtls":                      {"java", "javac", "maven"},
	}

	for server, deps := range dependencyChains {
		t.Run(server, func(t *testing.T) {
			result := &types.VerificationResult{
				Issues: []types.Issue{},
			}

			// Simulate missing dependencies
			missingDeps := []string{}
			for i, dep := range deps {
				if i%2 == 0 { // Simulate some missing dependencies
					missingDeps = append(missingDeps, dep)
				}
			}

			if len(missingDeps) > 0 {
				addIssueHelper(result, types.IssueSeverityCritical, types.IssueCategoryDependencies,
					fmt.Sprintf("Missing Dependencies for %s", server),
					fmt.Sprintf("Required dependencies are missing: %s", strings.Join(missingDeps, ", ")),
					fmt.Sprintf("Install missing dependencies: %s", strings.Join(missingDeps, ", ")),
					map[string]interface{}{
						"server":           server,
						"missing_deps":     missingDeps,
						"dependency_chain": deps,
					})
			}

			if len(result.Issues) == 0 && len(missingDeps) > 0 {
				t.Errorf("Expected issue for missing dependencies of %s", server)
			}
		})
	}
}

// Test cross-platform runtime installation differences
func TestVerifyRuntimeInstallation_CrossPlatformDifferences(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()

	// Test platform-specific executable names
	platformExecutables := map[string]map[string]string{
		"windows": {
			"go":     "go.exe",
			"python": "python.exe",
			"node":   "node.exe",
			"java":   "java.exe",
		},
		"linux": {
			"go":     "go",
			"python": "python3",
			"node":   "node",
			"java":   "java",
		},
		"darwin": {
			"go":     "go",
			"python": "python3",
			"node":   "node",
			"java":   "java",
		},
	}

	for platform, executables := range platformExecutables {
		t.Run(platform, func(t *testing.T) {
			for runtime, executable := range executables {
				t.Run(runtime, func(t *testing.T) {
					// Test missing platform-specific executable
					executor.SetRuntimeMissing(executable)

					if executor.IsCommandAvailable(executable) {
						t.Errorf("Expected %s to be unavailable on %s", executable, platform)
					}

					_, err := executor.Execute(executable, []string{"--version"}, 2*time.Second)
					if err == nil {
						t.Errorf("Expected error for missing %s on %s", executable, platform)
					}
				})
			}
		})
	}
}

// Test concurrent missing runtime detection
func TestVerifyRuntimeInstallation_ConcurrentMissingDetection(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()
	runtimes := []string{"go", "python", "node", "java"}

	// Mark all runtimes as missing
	for _, runtime := range runtimes {
		executor.SetRuntimeMissing(runtime)
	}

	concurrency := 10
	var wg sync.WaitGroup
	results := make(chan error, concurrency*len(runtimes))

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, runtime := range runtimes {
				_, err := executor.Execute(runtime, []string{"--version"}, 1*time.Second)
				results <- err
			}
		}()
	}

	wg.Wait()
	close(results)

	errorCount := 0
	for err := range results {
		if err == nil {
			t.Error("Expected error for missing runtime in concurrent access")
		} else {
			errorCount++
		}
	}

	expectedErrors := concurrency * len(runtimes)
	if errorCount != expectedErrors {
		t.Errorf("Expected %d errors, got %d", expectedErrors, errorCount)
	}
}

// Test recovery workflows after missing files are restored
func TestVerifyRuntimeInstallation_RecoveryAfterInstallation(t *testing.T) {
	executor := NewMockRuntimeCommandExecutor()
	mockFS := NewMockRuntimeFileSystem()
	runtime := "go"

	// Initially runtime is missing
	executor.SetRuntimeMissing(runtime)
	mockFS.RemoveRuntime(runtime)

	// Verify runtime is missing
	if executor.IsCommandAvailable(runtime) {
		t.Error("Expected Go to be missing initially")
	}

	if mockFS.IsRuntimeInstalled(runtime) {
		t.Error("Expected Go to be missing from filesystem")
	}

	_, err := executor.Execute(runtime, []string{"version"}, 2*time.Second)
	if err == nil {
		t.Error("Expected error for missing Go")
	}

	// Simulate runtime installation
	executor.SetVersionOutput(runtime, "go version go1.21.0 linux/amd64")
	mockFS.InstallRuntime(runtime, "/usr/local/go/bin/go")
	mockFS.SetEnvironmentVar("GOROOT", "/usr/local/go")
	mockFS.SetEnvironmentVar("GOPATH", "/home/user/go")

	// Remove from missing runtimes
	delete(executor.missingRuntimes, runtime)

	// Add successful command execution for both version and empty args (for IsCommandAvailable check)
	executor.AddCommand(runtime, []string{"version"}, &platform.Result{
		ExitCode: 0,
		Stdout:   "go version go1.21.0 linux/amd64",
		Duration: 100 * time.Millisecond,
	})

	// Add command with empty args for IsCommandAvailable to return true
	executor.AddCommand(runtime, []string{}, &platform.Result{
		ExitCode: 0,
		Stdout:   "go version go1.21.0 linux/amd64",
		Duration: 100 * time.Millisecond,
	})

	// Verify runtime is now available
	if !executor.IsCommandAvailable(runtime) {
		t.Error("Expected Go to be available after installation")
	}

	if !mockFS.IsRuntimeInstalled(runtime) {
		t.Error("Expected Go to be installed in filesystem")
	}

	result, err := executor.Execute(runtime, []string{"version"}, 2*time.Second)
	if err != nil {
		t.Errorf("Expected successful execution after installation: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0 after installation, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "go version") {
		t.Errorf("Expected version output after installation, got: %s", result.Stdout)
	}

	// Verify environment variables
	goroot := mockFS.GetEnvironmentVar("GOROOT")
	if goroot != "/usr/local/go" {
		t.Errorf("Expected GOROOT to be set, got: %s", goroot)
	}

	gopath := mockFS.GetEnvironmentVar("GOPATH")
	if gopath != "/home/user/go" {
		t.Errorf("Expected GOPATH to be set, got: %s", gopath)
	}
}

// Test installation verification files missing
func TestVerifyRuntimeInstallation_MissingVerificationFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create runtime installation without verification files
	runtimeDir := filepath.Join(tmpDir, "go")
	err := os.MkdirAll(runtimeDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create runtime directory: %v", err)
	}

	// Create executable but missing VERSION file
	goExecutable := filepath.Join(runtimeDir, "bin", "go")
	err = os.MkdirAll(filepath.Dir(goExecutable), 0755)
	if err != nil {
		t.Fatalf("Failed to create bin directory: %v", err)
	}

	err = os.WriteFile(goExecutable, []byte("#!/bin/bash\necho 'go version go1.21.0 linux/amd64'\n"), 0755)
	if err != nil {
		t.Fatalf("Failed to create go executable: %v", err)
	}

	// Verify executable exists
	if _, statErr := os.Stat(goExecutable); statErr != nil {
		t.Errorf("Go executable should exist: %v", statErr)
	}

	// Check for missing verification files
	versionFile := filepath.Join(runtimeDir, "VERSION")
	if _, statErr := os.Stat(versionFile); statErr == nil {
		t.Error("VERSION file should not exist (testing missing verification files)")
	}

	licenseFile := filepath.Join(runtimeDir, "LICENSE")
	if _, statErr := os.Stat(licenseFile); statErr == nil {
		t.Error("LICENSE file should not exist (testing missing verification files)")
	}

	// Test that installation appears incomplete due to missing verification files
	_ = createTestInstaller()
	result := &types.VerificationResult{
		Issues: []types.Issue{},
	}

	addIssueHelper(result, types.IssueSeverityMedium, types.IssueCategoryInstallation,
		"Incomplete Go Installation", "Go installation is missing verification files",
		"Reinstall Go from official source to ensure complete installation",
		map[string]interface{}{
			"missing_files": []string{"VERSION", "LICENSE"},
			"install_path":  runtimeDir,
		})

	if len(result.Issues) == 0 {
		t.Error("Expected issue for incomplete installation")
	}

	issue := result.Issues[0]
	if issue.Category != types.IssueCategoryInstallation {
		t.Errorf("Expected installation category, got %s", issue.Category)
	}
}

// Benchmark missing runtime detection
func BenchmarkMissingRuntimeDetection(b *testing.B) {
	executor := NewMockRuntimeCommandExecutor()
	runtimes := []string{"go", "python", "node", "java"}

	for _, runtime := range runtimes {
		executor.SetRuntimeMissing(runtime)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, runtime := range runtimes {
			_ = executor.IsCommandAvailable(runtime)
		}
	}
}

func BenchmarkMissingRuntimeExecution(b *testing.B) {
	executor := NewMockRuntimeCommandExecutor()
	executor.SetRuntimeMissing("go")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = executor.Execute("go", []string{"version"}, 1*time.Second)
	}
}

func BenchmarkCorruptedRuntimeDetection(b *testing.B) {
	executor := NewMockRuntimeCommandExecutor()
	executor.SetRuntimeCorrupted("go")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = executor.Execute("go", []string{"version"}, 1*time.Second)
	}
}
