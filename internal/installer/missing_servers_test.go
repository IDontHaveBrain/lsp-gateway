package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
)

// MockFileSystem for missing server executable scenarios
type MockServerFileSystem struct {
	mu                sync.RWMutex
	existingFiles     map[string]bool
	executableFiles   map[string]bool
	pathDirectories   []string
	missingExecutable map[string]bool
}

func NewMockServerFileSystem() *MockServerFileSystem {
	return &MockServerFileSystem{
		existingFiles:     make(map[string]bool),
		executableFiles:   make(map[string]bool),
		pathDirectories:   []string{"/usr/bin", "/usr/local/bin", "/opt/bin"},
		missingExecutable: make(map[string]bool),
	}
}

func (m *MockServerFileSystem) AddExecutable(name, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fullPath := filepath.Join(path, name)
	m.existingFiles[fullPath] = true
	m.executableFiles[fullPath] = true
	delete(m.missingExecutable, name)
}

func (m *MockServerFileSystem) RemoveExecutable(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.missingExecutable[name] = true
	// Remove from all possible paths
	for _, dir := range m.pathDirectories {
		fullPath := filepath.Join(dir, name)
		delete(m.existingFiles, fullPath)
		delete(m.executableFiles, fullPath)
	}
}

func (m *MockServerFileSystem) IsExecutableAvailable(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.missingExecutable[name] {
		return false
	}
	
	for _, dir := range m.pathDirectories {
		fullPath := filepath.Join(dir, name)
		if m.executableFiles[fullPath] {
			return true
		}
	}
	return false
}

func (m *MockServerFileSystem) FindExecutablePath(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.missingExecutable[name] {
		return ""
	}
	
	for _, dir := range m.pathDirectories {
		fullPath := filepath.Join(dir, name)
		if m.executableFiles[fullPath] {
			return fullPath
		}
	}
	return ""
}

// MockServerCommandExecutor for missing server scenarios
type MockServerCommandExecutor struct {
	*MockCommandExecutor
	missingCommands map[string]bool
}

func NewMockServerCommandExecutor() *MockServerCommandExecutor {
	return &MockServerCommandExecutor{
		MockCommandExecutor: NewMockCommandExecutor(),
		missingCommands:     make(map[string]bool),
	}
}

func (m *MockServerCommandExecutor) SetCommandMissing(cmd string) {
	m.missingCommands[cmd] = true
}

func (m *MockServerCommandExecutor) IsCommandAvailable(command string) bool {
	if m.missingCommands[command] {
		return false
	}
	return m.MockCommandExecutor.IsCommandAvailable(command)
}

func (m *MockServerCommandExecutor) Execute(cmd string, args []string, timeout time.Duration) (*platform.Result, error) {
	if m.missingCommands[cmd] {
		return &platform.Result{
			ExitCode: 127,
			Stderr:   fmt.Sprintf("command not found: %s", cmd),
			Duration: 10 * time.Millisecond,
		}, fmt.Errorf("command not found: %s", cmd)
	}
	return m.MockCommandExecutor.Execute(cmd, args, timeout)
}

// Test missing Go language server (gopls)
func TestVerifyLanguageServers_MissingGopls(t *testing.T) {
	executor := NewMockServerCommandExecutor()
	executor.SetCommandMissing("gopls")

	// Test verification when gopls is missing
	_ = &types.VerificationResult{
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
	}

	// Simulate gopls verification attempt
	_, err := executor.Execute("gopls", []string{"version"}, 5*time.Second)
	if err == nil {
		t.Error("Expected error for missing gopls command")
	}

	if !strings.Contains(err.Error(), "command not found") {
		t.Errorf("Expected 'command not found' error, got: %v", err)
	}

	// Check that IsCommandAvailable returns false
	if executor.IsCommandAvailable("gopls") {
		t.Error("Expected gopls to be unavailable")
	}
}

func TestVerifyLanguageServers_MissingPylsp(t *testing.T) {
	executor := NewMockServerCommandExecutor()
	executor.SetCommandMissing("python")
	executor.SetCommandMissing("pylsp")

	// Test Python LSP server missing scenarios
	testCases := []struct {
		command string
		args    []string
		name    string
	}{
		{"python", []string{"-m", "pylsp", "--version"}, "Python pylsp module"},
		{"pylsp", []string{"--version"}, "pylsp executable"},
		{"python3", []string{"-m", "pylsp", "--version"}, "Python3 pylsp module"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			executor.SetCommandMissing(tc.command)

			_, err := executor.Execute(tc.command, tc.args, 5*time.Second)
			if err == nil {
				t.Errorf("Expected error for missing %s", tc.command)
			}

			if !strings.Contains(err.Error(), "command not found") {
				t.Errorf("Expected 'command not found' error for %s, got: %v", tc.command, err)
			}

			if executor.IsCommandAvailable(tc.command) {
				t.Errorf("Expected %s to be unavailable", tc.command)
			}
		})
	}
}

func TestVerifyLanguageServers_MissingTypeScriptLanguageServer(t *testing.T) {
	executor := NewMockServerCommandExecutor()
	executor.SetCommandMissing("typescript-language-server")
	executor.SetCommandMissing("tsserver")

	// Test TypeScript language server missing scenarios
	testCases := []string{
		"typescript-language-server",
		"tsserver",
	}

	for _, cmd := range testCases {
		t.Run(cmd, func(t *testing.T) {
			_, err := executor.Execute(cmd, []string{"--version"}, 5*time.Second)
			if err == nil {
				t.Errorf("Expected error for missing %s", cmd)
			}

			if !strings.Contains(err.Error(), "command not found") {
				t.Errorf("Expected 'command not found' error for %s, got: %v", cmd, err)
			}

			if executor.IsCommandAvailable(cmd) {
				t.Errorf("Expected %s to be unavailable", cmd)
			}
		})
	}
}

func TestVerifyLanguageServers_MissingJdtls(t *testing.T) {
	executor := NewMockServerCommandExecutor()
	executor.SetCommandMissing("jdtls")

	// Test Eclipse JDT Language Server missing
	_, err := executor.Execute("jdtls", []string{"--version"}, 5*time.Second)
	if err == nil {
		t.Error("Expected error for missing jdtls command")
	}

	if !strings.Contains(err.Error(), "command not found") {
		t.Errorf("Expected 'command not found' error, got: %v", err)
	}

	if executor.IsCommandAvailable("jdtls") {
		t.Error("Expected jdtls to be unavailable")
	}
}

// Test language server PATH resolution failures
func TestLanguageServerPathResolution_MissingFromPath(t *testing.T) {
	mockFS := NewMockServerFileSystem()

	// Test servers that should be in PATH but aren't
	serverCommands := []string{
		"gopls",
		"pylsp",
		"typescript-language-server",
		"jdtls",
		"rust-analyzer",
		"clangd",
	}

	for _, cmd := range serverCommands {
		t.Run(cmd, func(t *testing.T) {
			// Ensure command is not available
			mockFS.RemoveExecutable(cmd)

			if mockFS.IsExecutableAvailable(cmd) {
				t.Errorf("Expected %s to be unavailable", cmd)
			}

			path := mockFS.FindExecutablePath(cmd)
			if path != "" {
				t.Errorf("Expected empty path for missing %s, got: %s", cmd, path)
			}
		})
	}
}

func TestLanguageServerPathResolution_PartialInstallation(t *testing.T) {
	mockFS := NewMockServerFileSystem()

	// Add some servers but not others
	mockFS.AddExecutable("gopls", "/usr/local/bin")
	mockFS.AddExecutable("python", "/usr/bin")
	// Missing: pylsp, typescript-language-server, jdtls

	// Test available servers
	if !mockFS.IsExecutableAvailable("gopls") {
		t.Error("Expected gopls to be available")
	}

	if !mockFS.IsExecutableAvailable("python") {
		t.Error("Expected python to be available")
	}

	// Test missing servers
	missingServers := []string{"pylsp", "typescript-language-server", "jdtls"}
	for _, server := range missingServers {
		if mockFS.IsExecutableAvailable(server) {
			t.Errorf("Expected %s to be unavailable", server)
		}
	}
}

// Test language server version validation with missing binaries
func TestLanguageServerVersionValidation_MissingBinaries(t *testing.T) {
	executor := NewMockServerCommandExecutor()

	// Set up servers with missing binaries
	missingServers := map[string][]string{
		"gopls":                      {"version"},
		"pylsp":                      {"--version"},
		"typescript-language-server": {"--version"},
		"jdtls":                      {"--version"},
	}

	for server, args := range missingServers {
		t.Run(server, func(t *testing.T) {
			executor.SetCommandMissing(server)

			// Attempt version check
			result, err := executor.Execute(server, args, 5*time.Second)
			if err == nil {
				t.Errorf("Expected error for missing %s binary", server)
			}

			if result.ExitCode != 127 {
				t.Errorf("Expected exit code 127 for missing %s, got %d", server, result.ExitCode)
			}

			if !strings.Contains(result.Stderr, "command not found") {
				t.Errorf("Expected 'command not found' in stderr for %s, got: %s", server, result.Stderr)
			}
		})
	}
}

// Test graceful degradation when multiple language servers are missing
func TestLanguageServers_MultipleServersMissing(t *testing.T) {
	executor := NewMockServerCommandExecutor()

	// Remove all language servers
	allServers := []string{
		"gopls",
		"python",
		"pylsp",
		"typescript-language-server",
		"tsserver",
		"jdtls",
		"rust-analyzer",
		"clangd",
	}

	for _, server := range allServers {
		executor.SetCommandMissing(server)
	}

	// Test that all servers report as missing
	missingCount := 0
	for _, server := range allServers {
		if !executor.IsCommandAvailable(server) {
			missingCount++
		}
	}

	if missingCount != len(allServers) {
		t.Errorf("Expected all %d servers to be missing, got %d missing", len(allServers), missingCount)
	}

	// Test error messages for all missing servers
	for _, server := range allServers {
		t.Run(fmt.Sprintf("missing_%s", server), func(t *testing.T) {
			_, err := executor.Execute(server, []string{"--version"}, 1*time.Second)
			if err == nil {
				t.Errorf("Expected error for missing %s", server)
			}

			if !strings.Contains(err.Error(), "command not found") {
				t.Errorf("Expected 'command not found' error for %s, got: %v", server, err)
			}
		})
	}
}

// Test language server detection and installation failure scenarios
func TestLanguageServerInstallation_DetectionFailure(t *testing.T) {
	runtimeInstaller := NewRuntimeInstaller()
	installer := NewServerInstaller(runtimeInstaller)

	// Test detection of missing servers
	missingServers := []string{
		"nonexistent-lsp",
		"fake-language-server",
		"missing-server",
	}

	for _, server := range missingServers {
		t.Run(server, func(t *testing.T) {
			// Attempt to get server info for non-existent server
			_, err := installer.GetServerInfo(server)
			if err == nil {
				t.Errorf("Expected error for non-existent server %s", server)
			}

			if installerErr, ok := err.(*InstallerError); ok {
				if installerErr.Type != InstallerErrorTypeNotFound {
					t.Errorf("Expected NotFound error type for %s, got %s", server, installerErr.Type)
				}
			} else {
				t.Errorf("Expected InstallerError for %s, got %T", server, err)
			}
		})
	}
}

func TestLanguageServerInstallation_InstallationFailure(t *testing.T) {
	runtimeInstaller := NewRuntimeInstaller()
	installer := NewServerInstaller(runtimeInstaller)

	// Test installation failure scenarios
	options := types.ServerInstallOptions{
		Version:             "latest",
		Force:               false,
		SkipVerify:          false,
		SkipDependencyCheck: false,
		Timeout:             30 * time.Second,
		Platform:            "linux",
		InstallMethod:       "auto",
	}

	// Try to install missing/unsupported servers
	unsupportedServers := []string{
		"unknown-server",
		"fake-lsp",
		"nonexistent-language-server",
	}

	for _, server := range unsupportedServers {
		t.Run(server, func(t *testing.T) {
			result, err := installer.Install(server, options)
			if err == nil && (result == nil || result.Success) {
				t.Errorf("Expected installation failure for unsupported server %s", server)
			}

			if result != nil && len(result.Errors) == 0 {
				t.Errorf("Expected installation errors for unsupported server %s", server)
			}
		})
	}
}

// Test missing executable permissions
func TestLanguageServers_MissingExecutablePermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()

	// Create executable files without execute permissions
	servers := map[string]string{
		"gopls":                      "#!/bin/bash\necho gopls version",
		"pylsp":                      "#!/bin/bash\necho pylsp version",
		"typescript-language-server": "#!/bin/bash\necho tsserver version",
	}

	for server, content := range servers {
		serverPath := filepath.Join(tmpDir, server)
		err := os.WriteFile(serverPath, []byte(content), 0644) // No execute permission
		if err != nil {
			t.Fatalf("Failed to create %s file: %v", server, err)
		}

		// Test that file exists but is not executable
		if _, statErr := os.Stat(serverPath); statErr != nil {
			t.Errorf("File %s should exist: %v", serverPath, statErr)
		}

		// Test execution (should fail due to permissions)
		executor := NewMockServerCommandExecutor()
		result, execErr := executor.Execute(serverPath, []string{"--version"}, 5*time.Second)
		if execErr == nil {
			t.Errorf("Expected execution error for non-executable %s", server)
		}

		if result != nil && result.ExitCode == 0 {
			t.Errorf("Expected non-zero exit code for non-executable %s", server)
		}
	}
}

// Test missing dependency chain for language servers
func TestLanguageServers_MissingDependencyChain(t *testing.T) {
	runtimeInstaller := NewRuntimeInstaller()
	installer := NewServerInstaller(runtimeInstaller)

	// Test dependency validation for servers
	serverDependencies := map[string]string{
		"pylsp":                      "python",
		"typescript-language-server": "nodejs",
		"jdtls":                      "java",
	}

	for server, dependency := range serverDependencies {
		t.Run(fmt.Sprintf("%s_missing_%s", server, dependency), func(t *testing.T) {
			result, err := installer.ValidateDependencies(server)
			if err != nil {
				t.Logf("Dependency validation failed as expected for %s: %v", server, err)
			}

			if result != nil {
				if result.RuntimeRequired != dependency {
					t.Errorf("Expected required runtime %s for %s, got %s", 
						dependency, server, result.RuntimeRequired)
				}

				if len(result.MissingRuntimes) == 0 && !result.RuntimeInstalled {
					t.Logf("Missing runtime detected for %s as expected", server)
				}
			}
		})
	}
}

// Test cross-platform executable differences
func TestLanguageServers_CrossPlatformMissingExecutables(t *testing.T) {
	executor := NewMockServerCommandExecutor()

	// Test Windows executable extensions
	windowsExecutables := []string{
		"gopls.exe",
		"python.exe",
		"node.exe",
		"java.exe",
	}

	// Test Unix/Linux executables
	unixExecutables := []string{
		"gopls",
		"python",
		"python3",
		"node",
		"java",
	}

	allExecutables := append(windowsExecutables, unixExecutables...)

	for _, exe := range allExecutables {
		t.Run(exe, func(t *testing.T) {
			executor.SetCommandMissing(exe)

			if executor.IsCommandAvailable(exe) {
				t.Errorf("Expected %s to be unavailable", exe)
			}

			_, err := executor.Execute(exe, []string{"--version"}, 2*time.Second)
			if err == nil {
				t.Errorf("Expected error for missing executable %s", exe)
			}

			if !strings.Contains(err.Error(), "command not found") {
				t.Errorf("Expected 'command not found' error for %s, got: %v", exe, err)
			}
		})
	}
}

// Test symbolic link targets missing
func TestLanguageServers_BrokenSymbolicLinks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create broken symbolic links
	brokenLinks := []string{"gopls", "pylsp", "typescript-language-server"}

	for _, link := range brokenLinks {
		linkPath := filepath.Join(tmpDir, link)
		targetPath := filepath.Join("/nonexistent", "target", link)

		// Create symbolic link to non-existent target
		err := os.Symlink(targetPath, linkPath)
		if err != nil {
			t.Fatalf("Failed to create broken symlink for %s: %v", link, err)
		}

		// Verify link exists but target doesn't
		if _, statErr := os.Lstat(linkPath); statErr != nil {
			t.Errorf("Symlink %s should exist: %v", linkPath, statErr)
		}

		if _, statErr := os.Stat(linkPath); statErr == nil {
			t.Errorf("Symlink target for %s should not exist", linkPath)
		}

		// Test execution of broken symlink (should fail)
		executor := NewMockServerCommandExecutor()
		result, execErr := executor.Execute(linkPath, []string{"--version"}, 2*time.Second)
		if execErr == nil {
			t.Errorf("Expected execution error for broken symlink %s", link)
		}

		if result != nil && result.ExitCode == 0 {
			t.Errorf("Expected non-zero exit code for broken symlink %s", link)
		}
	}
}

// Test concurrent missing server detection
func TestLanguageServers_ConcurrentMissingDetection(t *testing.T) {
	executor := NewMockServerCommandExecutor()
	servers := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}

	// Mark all servers as missing
	for _, server := range servers {
		executor.SetCommandMissing(server)
	}

	concurrency := 10
	var wg sync.WaitGroup
	results := make(chan error, concurrency*len(servers))

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, server := range servers {
				_, err := executor.Execute(server, []string{"--version"}, 1*time.Second)
				results <- err
			}
		}()
	}

	wg.Wait()
	close(results)

	errorCount := 0
	for err := range results {
		if err == nil {
			t.Error("Expected error for missing server in concurrent access")
		} else {
			errorCount++
		}
	}

	expectedErrors := concurrency * len(servers)
	if errorCount != expectedErrors {
		t.Errorf("Expected %d errors, got %d", expectedErrors, errorCount)
	}
}

// Test helpful error messages with installation guidance
func TestLanguageServers_InstallationGuidance(t *testing.T) {
	runtimeInstaller := NewRuntimeInstaller()
	installer := NewServerInstaller(runtimeInstaller)

	servers := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}

	for _, server := range servers {
		t.Run(server, func(t *testing.T) {
			// Try to get installation guidance for missing server
			serverInfo, err := installer.GetServerInfo(server)
			if err != nil {
				// Server not found in registry - expected for missing servers
				if installerErr, ok := err.(*InstallerError); ok {
					if installerErr.Type != InstallerErrorTypeNotFound {
						t.Errorf("Expected NotFound error for %s, got %s", server, installerErr.Type)
					}
				}
				return
			}

			// If server info is available, check installation methods
			if serverInfo != nil && len(serverInfo.InstallMethods) > 0 {
				t.Logf("Installation methods available for %s: %d", server, len(serverInfo.InstallMethods))
			}
		})
	}
}

// Test recovery scenarios after missing files are provided
func TestLanguageServers_RecoveryAfterInstallation(t *testing.T) {
	executor := NewMockServerCommandExecutor()
	server := "gopls"

	// Initially server is missing
	executor.SetCommandMissing(server)

	// Verify server is missing
	if executor.IsCommandAvailable(server) {
		t.Error("Expected gopls to be missing initially")
	}

	_, err := executor.Execute(server, []string{"version"}, 2*time.Second)
	if err == nil {
		t.Error("Expected error for missing gopls")
	}

	// Simulate server installation (add command)
	executor.AddCommand(server, []string{"version"}, &platform.Result{
		ExitCode: 0,
		Stdout:   "gopls version v0.14.0",
		Duration: 100 * time.Millisecond,
	})

	// Remove from missing commands
	delete(executor.missingCommands, server)

	// Verify server is now available
	if !executor.IsCommandAvailable(server) {
		t.Error("Expected gopls to be available after installation")
	}

	result, err := executor.Execute(server, []string{"version"}, 2*time.Second)
	if err != nil {
		t.Errorf("Expected successful execution after installation: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0 after installation, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "gopls version") {
		t.Errorf("Expected version output after installation, got: %s", result.Stdout)
	}
}

// Benchmark missing server detection
func BenchmarkMissingServerDetection(b *testing.B) {
	executor := NewMockServerCommandExecutor()
	servers := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}

	for _, server := range servers {
		executor.SetCommandMissing(server)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, server := range servers {
			_ = executor.IsCommandAvailable(server)
		}
	}
}

func BenchmarkMissingServerExecution(b *testing.B) {
	executor := NewMockServerCommandExecutor()
	executor.SetCommandMissing("gopls")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = executor.Execute("gopls", []string{"version"}, 1*time.Second)
	}
}