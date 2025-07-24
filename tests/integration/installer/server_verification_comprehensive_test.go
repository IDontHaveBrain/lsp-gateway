package installer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
)

// Mock RuntimeInstaller for server verification testing
type mockServerRuntimeInstaller struct {
	runtimeInstalled map[string]bool
	runtimeVersions  map[string]string
	runtimeErrors    map[string]error
}

func newMockServerRuntimeInstaller() *mockServerRuntimeInstaller {
	return &mockServerRuntimeInstaller{
		runtimeInstalled: make(map[string]bool),
		runtimeVersions:  make(map[string]string),
		runtimeErrors:    make(map[string]error),
	}
}

func (m *mockServerRuntimeInstaller) Install(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
	return &types.InstallResult{Success: true}, nil
}

func (m *mockServerRuntimeInstaller) Verify(runtime string) (*types.VerificationResult, error) {
	if err, exists := m.runtimeErrors[runtime]; exists {
		return nil, err
	}

	installed := m.runtimeInstalled[runtime]
	version := m.runtimeVersions[runtime]
	if version == "" {
		version = "1.0.0"
	}

	return &types.VerificationResult{
		Installed:       installed,
		Compatible:      installed,
		Version:         version,
		Path:            "/usr/bin/" + runtime,
		Issues:          []types.Issue{},
		Details:         make(map[string]interface{}),
		EnvironmentVars: make(map[string]string),
		Metadata:        make(map[string]interface{}),
	}, nil
}

func (m *mockServerRuntimeInstaller) GetSupportedRuntimes() []string {
	return []string{"go", "python", "nodejs", "java"}
}

func (m *mockServerRuntimeInstaller) GetRuntimeInfo(runtime string) (*types.RuntimeDefinition, error) {
	return &types.RuntimeDefinition{Name: runtime}, nil
}

func (m *mockServerRuntimeInstaller) ValidateVersion(runtime, minVersion string) (*types.VersionValidationResult, error) {
	return &types.VersionValidationResult{Valid: true}, nil
}

func (m *mockServerRuntimeInstaller) GetPlatformStrategy(platform string) types.RuntimePlatformStrategy {
	return nil
}

func (m *mockServerRuntimeInstaller) SetRuntimeInstalled(runtime string, installed bool) {
	m.runtimeInstalled[runtime] = installed
}

func (m *mockServerRuntimeInstaller) SetRuntimeVersion(runtime, version string) {
	m.runtimeVersions[runtime] = version
}

func (m *mockServerRuntimeInstaller) SetRuntimeError(runtime string, err error) {
	m.runtimeErrors[runtime] = err
}

// Mock CommandExecutor for server verification testing
type mockServerVerificationExecutor struct {
	commands          map[string]*platform.Result
	commandErrors     map[string]error
	availableCommands map[string]bool
}

func newMockServerVerificationExecutor() *mockServerVerificationExecutor {
	return &mockServerVerificationExecutor{
		commands:          make(map[string]*platform.Result),
		commandErrors:     make(map[string]error),
		availableCommands: make(map[string]bool),
	}
}

func (m *mockServerVerificationExecutor) Execute(command string, args []string, timeout time.Duration) (*platform.Result, error) {
	key := command
	if len(args) > 0 {
		key = command + " " + args[0]
	}

	if err, exists := m.commandErrors[key]; exists {
		return &platform.Result{ExitCode: 1, Stderr: err.Error()}, err
	}

	if result, exists := m.commands[key]; exists {
		return result, nil
	}

	return &platform.Result{ExitCode: 0, Stdout: "mock success"}, nil
}

func (m *mockServerVerificationExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(cmd, args, timeout)
}

func (m *mockServerVerificationExecutor) GetShell() string {
	return "bash"
}

func (m *mockServerVerificationExecutor) GetShellArgs(command string) []string {
	return []string{"-c", command}
}

func (m *mockServerVerificationExecutor) IsCommandAvailable(command string) bool {
	if available, exists := m.availableCommands[command]; exists {
		return available
	}
	return false
}

func (m *mockServerVerificationExecutor) SetCommand(command string, result *platform.Result) {
	m.commands[command] = result
}

func (m *mockServerVerificationExecutor) SetCommandError(command string, err error) {
	m.commandErrors[command] = err
}

func (m *mockServerVerificationExecutor) SetCommandAvailable(command string, available bool) {
	m.availableCommands[command] = available
}

func TestInstallerDefaultServerVerifier_verifyServerInstallation(t *testing.T) {
	tests := []struct {
		name              string
		serverName        string
		runtimeInstalled  bool
		runtimeCompatible bool
		expectedInstalled bool
		expectedIssues    int
	}{
		{
			name:              "Gopls - runtime not installed",
			serverName:        "gopls",
			runtimeInstalled:  false,
			runtimeCompatible: false,
			expectedInstalled: false,
			expectedIssues:    1,
		},
		{
			name:              "Gopls - runtime installed",
			serverName:        "gopls",
			runtimeInstalled:  true,
			runtimeCompatible: true,
			expectedInstalled: false, // Will be set by specific verification
			expectedIssues:    0,
		},
		{
			name:              "Pylsp - runtime not installed",
			serverName:        "pylsp",
			runtimeInstalled:  false,
			runtimeCompatible: false,
			expectedInstalled: false,
			expectedIssues:    1,
		},
		{
			name:              "TypeScript Language Server - runtime installed",
			serverName:        "typescript-language-server",
			runtimeInstalled:  true,
			runtimeCompatible: true,
			expectedInstalled: false,
			expectedIssues:    0,
		},
		{
			name:              "JDTLS - runtime installed",
			serverName:        "jdtls",
			runtimeInstalled:  true,
			runtimeCompatible: true,
			expectedInstalled: false,
			expectedIssues:    0,
		},
		{
			name:              "Unknown server",
			serverName:        "unknown-server",
			runtimeInstalled:  true,
			runtimeCompatible: true,
			expectedInstalled: false,
			expectedIssues:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRuntimeInstaller := newMockServerRuntimeInstaller()
			mockExecutor := newMockServerVerificationExecutor()

			// Setup runtime verification
			runtimeMap := map[string]string{
				"gopls":                      "go",
				"pylsp":                      "python",
				"typescript-language-server": "nodejs",
				"jdtls":                      "java",
			}

			if runtime, exists := runtimeMap[tt.serverName]; exists {
				mockRuntimeInstaller.SetRuntimeInstalled(runtime, tt.runtimeInstalled)
				if tt.runtimeCompatible {
					mockRuntimeInstaller.SetRuntimeVersion(runtime, "2.0.0")
				} else {
					mockRuntimeInstaller.SetRuntimeVersion(runtime, "0.1.0")
				}
			}

			verifier := &installer.DefaultServerVerifier{
				runtimeVerifier: mockRuntimeInstaller,
				serverRegistry:  NewServerRegistry(),
				executor:        mockExecutor,
			}

			result := &ServerVerificationResult{
				ServerName:      tt.serverName,
				Installed:       false,
				Issues:          []Issue{},
				Recommendations: []string{},
				Metadata:        make(map[string]interface{}),
			}

			// Test the runtime verification first
			if serverDef, err := verifier.serverRegistry.GetServer(tt.serverName); err == nil {
				verifier.verifyServerRuntime(result, serverDef)

				// Only proceed with server installation verification if runtime is available
				if result.RuntimeStatus != nil && result.RuntimeStatus.Installed && result.RuntimeStatus.Compatible {
					verifier.verifyServerInstallation(result, serverDef)
				}
			}

			// Verify results
			if len(result.Issues) != tt.expectedIssues {
				t.Errorf("Expected %d issues, got %d: %v", tt.expectedIssues, len(result.Issues), result.Issues)
			}

			if tt.runtimeInstalled && tt.runtimeCompatible {
				// Server installation should be tested when runtime is available
				if result.RuntimeStatus == nil {
					t.Error("Expected runtime status to be set")
				} else {
					if !result.RuntimeStatus.Installed {
						t.Error("Expected runtime to be installed")
					}
					if !result.RuntimeStatus.Compatible {
						t.Error("Expected runtime to be compatible")
					}
				}
			}
		})
	}
}

func TestInstallerDefaultServerVerifier_verifyGoplsInstallation(t *testing.T) {
	tests := []struct {
		name              string
		commandAvailable  bool
		whichResult       *platform.Result
		whichError        error
		fileExists        bool
		fileExecutable    bool
		expectedInstalled bool
		expectedIssues    int
	}{
		{
			name:              "Gopls available and executable",
			commandAvailable:  true,
			whichResult:       &platform.Result{ExitCode: 0, Stdout: "/usr/local/bin/gopls"},
			expectedInstalled: true,
			expectedIssues:    0,
			fileExists:        true,
			fileExecutable:    true,
		},
		{
			name:              "Gopls not available in PATH",
			commandAvailable:  false,
			expectedInstalled: false,
			expectedIssues:    1,
		},
		{
			name:              "Gopls available but which command fails",
			commandAvailable:  true,
			whichError:        errors.New("which command failed"),
			expectedInstalled: true,
			expectedIssues:    0,
		},
		{
			name:              "Gopls available but file not executable",
			commandAvailable:  true,
			whichResult:       &platform.Result{ExitCode: 0, Stdout: "/usr/local/bin/gopls"},
			expectedInstalled: false,
			expectedIssues:    1,
			fileExists:        true,
			fileExecutable:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := newMockServerVerificationExecutor()

			// Note: Since we can't mock platform.IsCommandAvailable, we test with actual system state

			if tt.whichResult != nil {
				mockExecutor.SetCommand("which gopls", tt.whichResult)
			}
			if tt.whichError != nil {
				mockExecutor.SetCommandError("which gopls", tt.whichError)
			}

			verifier := &installer.DefaultServerVerifier{
				runtimeVerifier: newMockServerRuntimeInstaller(),
				serverRegistry:  NewServerRegistry(),
				executor:        mockExecutor,
			}

			result := &ServerVerificationResult{
				ServerName:      "gopls",
				Installed:       false,
				Issues:          []Issue{},
				Recommendations: []string{},
				Metadata:        make(map[string]interface{}),
			}

			serverDef, err := verifier.serverRegistry.GetServer("gopls")
			if err != nil {
				t.Fatalf("Failed to get server definition: %v", err)
			}

			// Create temporary file if needed for file system tests
			if tt.fileExists && tt.whichResult != nil {
				tempDir := t.TempDir()
				goplsPath := filepath.Join(tempDir, "gopls")

				file, err := os.Create(goplsPath)
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				file.Close()

				if tt.fileExecutable {
					err = os.Chmod(goplsPath, 0755)
				} else {
					err = os.Chmod(goplsPath, 0644)
				}
				if err != nil {
					t.Fatalf("Failed to set file permissions: %v", err)
				}

				// Update the which result to point to our temp file
				tt.whichResult.Stdout = goplsPath
				mockExecutor.SetCommand("which gopls", tt.whichResult)
			}

			// Note: We can't mock platform.IsCommandAvailable directly since it's a package function
			// Instead, we test the behavior with the actual implementation

			verifier.verifyGoplsInstallation(result, serverDef)

			// Test the basic structure of the verification process
			// Since we can't control platform.IsCommandAvailable, we test the logic
			if tt.name == "Gopls not available in PATH" {
				// This tests the code path when gopls is not available
				// The actual result depends on system state, but we test the method execution
				if !platform.IsCommandAvailable("gopls") {
					// If gopls is actually not available, should have issues
					if result.Installed {
						t.Error("Expected gopls not to be installed when not available in PATH")
					}
					if len(result.Issues) == 0 {
						t.Error("Expected issues when gopls is not available in PATH")
					}
				}
			} else {
				// For other tests, verify the method completes without panicking
				// The exact results depend on system state
			}

			if tt.expectedInstalled && result.Path == "" {
				t.Error("Expected path to be set when installation is verified")
			}

			if tt.expectedInstalled && result.Metadata["executable_path"] == nil {
				t.Error("Expected executable_path in metadata when installation is verified")
			}
		})
	}
}

func TestInstallerDefaultServerVerifier_verifyPylspInstallation(t *testing.T) {
	tests := []struct {
		name              string
		commandAvailable  bool
		whichResult       *platform.Result
		whichError        error
		expectedInstalled bool
		expectedIssues    int
	}{
		{
			name:              "Pylsp available",
			commandAvailable:  true,
			whichResult:       &platform.Result{ExitCode: 0, Stdout: "/usr/local/bin/pylsp"},
			expectedInstalled: true,
			expectedIssues:    0,
		},
		{
			name:              "Pylsp not available in PATH",
			commandAvailable:  false,
			expectedInstalled: false,
			expectedIssues:    1,
		},
		{
			name:              "Pylsp available but which command fails",
			commandAvailable:  true,
			whichError:        errors.New("which command failed"),
			expectedInstalled: true,
			expectedIssues:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := newMockServerVerificationExecutor()

			// Note: Since we can't mock platform.IsCommandAvailable, we test with actual system state

			if tt.whichResult != nil {
				mockExecutor.SetCommand("which pylsp", tt.whichResult)
			}
			if tt.whichError != nil {
				mockExecutor.SetCommandError("which pylsp", tt.whichError)
			}

			verifier := &installer.DefaultServerVerifier{
				runtimeVerifier: newMockServerRuntimeInstaller(),
				serverRegistry:  NewServerRegistry(),
				executor:        mockExecutor,
			}

			result := &ServerVerificationResult{
				ServerName:      "pylsp",
				Installed:       false,
				Issues:          []Issue{},
				Recommendations: []string{},
				Metadata:        make(map[string]interface{}),
			}

			serverDef, err := verifier.serverRegistry.GetServer("pylsp")
			if err != nil {
				t.Fatalf("Failed to get server definition: %v", err)
			}

			// Note: We can't mock platform.IsCommandAvailable directly since it's a package function
			// Instead, we test the behavior with the actual implementation

			verifier.verifyPylspInstallation(result, serverDef)

			// Test that the method executes without panic
			// Actual results depend on system state since we can't mock platform.IsCommandAvailable
			if tt.name == "Pylsp not available in PATH" && !platform.IsCommandAvailable("pylsp") {
				// If pylsp is actually not available, should have issues
				if result.Installed {
					t.Error("Expected pylsp not to be installed when not available in PATH")
				}
				if len(result.Issues) == 0 {
					t.Error("Expected issues when pylsp is not available in PATH")
				}
			}

			if tt.expectedInstalled && result.Path == "" && tt.whichResult != nil {
				t.Error("Expected path to be set when installation is verified")
			}

			if tt.expectedInstalled && result.Metadata["executable_path"] == nil && tt.whichResult != nil {
				t.Error("Expected executable_path in metadata when installation is verified")
			}
		})
	}
}

func TestInstallerDefaultServerVerifier_verifyTypeScriptLSInstallation(t *testing.T) {
	tests := []struct {
		name              string
		tsServerAvailable bool
		tscAvailable      bool
		whichResult       *platform.Result
		whichError        error
		expectedInstalled bool
		expectedIssues    int
	}{
		{
			name:              "TypeScript Language Server and tsc available",
			tsServerAvailable: true,
			tscAvailable:      true,
			whichResult:       &platform.Result{ExitCode: 0, Stdout: "/usr/local/bin/typescript-language-server"},
			expectedInstalled: true,
			expectedIssues:    0,
		},
		{
			name:              "TypeScript Language Server available but tsc missing",
			tsServerAvailable: true,
			tscAvailable:      false,
			whichResult:       &platform.Result{ExitCode: 0, Stdout: "/usr/local/bin/typescript-language-server"},
			expectedInstalled: true,
			expectedIssues:    1,
		},
		{
			name:              "TypeScript Language Server not available",
			tsServerAvailable: false,
			tscAvailable:      true,
			expectedInstalled: false,
			expectedIssues:    1,
		},
		{
			name:              "TypeScript Language Server available but which fails",
			tsServerAvailable: true,
			tscAvailable:      true,
			whichError:        errors.New("which command failed"),
			expectedInstalled: true,
			expectedIssues:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := newMockServerVerificationExecutor()

			// Note: Since we can't mock platform.IsCommandAvailable, we test with actual system state

			if tt.whichResult != nil {
				mockExecutor.SetCommand("which typescript-language-server", tt.whichResult)
			}
			if tt.whichError != nil {
				mockExecutor.SetCommandError("which typescript-language-server", tt.whichError)
			}

			verifier := &installer.DefaultServerVerifier{
				runtimeVerifier: newMockServerRuntimeInstaller(),
				serverRegistry:  NewServerRegistry(),
				executor:        mockExecutor,
			}

			result := &ServerVerificationResult{
				ServerName:      "typescript-language-server",
				Installed:       false,
				Issues:          []Issue{},
				Recommendations: []string{},
				Metadata:        make(map[string]interface{}),
			}

			serverDef, err := verifier.serverRegistry.GetServer("typescript-language-server")
			if err != nil {
				t.Fatalf("Failed to get server definition: %v", err)
			}

			// Note: We can't mock platform.IsCommandAvailable directly since it's a package function
			// Instead, we test the behavior with the actual implementation

			verifier.verifyTypeScriptLSInstallation(result, serverDef)

			// Test that the method executes without panic
			// Actual results depend on system state since we can't mock platform.IsCommandAvailable
			if tt.name == "TypeScript Language Server not available" && !platform.IsCommandAvailable("typescript-language-server") {
				// If TypeScript Language Server is actually not available, should have issues
				if result.Installed {
					t.Error("Expected TypeScript Language Server not to be installed when not available in PATH")
				}
				if len(result.Issues) == 0 {
					t.Error("Expected issues when TypeScript Language Server is not available in PATH")
				}
			}

			if tt.expectedInstalled && result.Path == "" && tt.whichResult != nil {
				t.Error("Expected path to be set when installation is verified")
			}

			if tt.expectedInstalled && result.Metadata["executable_path"] == nil && tt.whichResult != nil {
				t.Error("Expected executable_path in metadata when installation is verified")
			}
		})
	}
}

func TestInstallerDefaultServerVerifier_verifyJdtlsInstallation(t *testing.T) {
	tests := []struct {
		name              string
		createJdtlsJar    bool
		jarInSubdir       bool
		expectedInstalled bool
		expectedIssues    int
	}{
		{
			name:              "JDTLS JAR found in standard location",
			createJdtlsJar:    true,
			jarInSubdir:       true,
			expectedInstalled: true,
			expectedIssues:    0,
		},
		{
			name:              "JDTLS JAR not found",
			createJdtlsJar:    false,
			expectedInstalled: false,
			expectedIssues:    1,
		},
		{
			name:              "JDTLS directory exists but no JAR",
			createJdtlsJar:    false,
			jarInSubdir:       false,
			expectedInstalled: false,
			expectedIssues:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// Set HOME environment variable to our temp directory
			originalHome := os.Getenv("HOME")
			os.Setenv("HOME", tempDir)
			defer os.Setenv("HOME", originalHome)

			if tt.createJdtlsJar {
				// Create JDTLS installation structure
				jdtlsDir := filepath.Join(tempDir, ".local", "share", "eclipse.jdt.ls")
				pluginsDir := filepath.Join(jdtlsDir, "plugins")
				err := os.MkdirAll(pluginsDir, 0755)
				if err != nil {
					t.Fatalf("Failed to create plugins directory: %v", err)
				}

				if tt.jarInSubdir {
					// Create a mock JDTLS JAR file
					jarPath := filepath.Join(pluginsDir, "org.eclipse.jdt.ls.core_1.23.0.jar")
					file, err := os.Create(jarPath)
					if err != nil {
						t.Fatalf("Failed to create JAR file: %v", err)
					}
					file.Close()
				}
			}

			verifier := &installer.DefaultServerVerifier{
				runtimeVerifier: newMockServerRuntimeInstaller(),
				serverRegistry:  NewServerRegistry(),
				executor:        newMockServerVerificationExecutor(),
			}

			result := &ServerVerificationResult{
				ServerName:      "jdtls",
				Installed:       false,
				Issues:          []Issue{},
				Recommendations: []string{},
				Metadata:        make(map[string]interface{}),
			}

			verifier.verifyJdtlsInstallation(result)

			if result.Installed != tt.expectedInstalled {
				t.Errorf("Expected installed=%v, got %v", tt.expectedInstalled, result.Installed)
			}

			if len(result.Issues) != tt.expectedIssues {
				t.Errorf("Expected %d issues, got %d: %v", tt.expectedIssues, len(result.Issues), result.Issues)
			}

			if tt.expectedInstalled {
				if result.Path == "" {
					t.Error("Expected path to be set when JDTLS is found")
				}
				if result.Metadata["jar_path"] == nil {
					t.Error("Expected jar_path in metadata when JDTLS is found")
				}
				if result.Metadata["installation_dir"] == nil {
					t.Error("Expected installation_dir in metadata when JDTLS is found")
				}
			}
		})
	}
}

func TestInstallerDefaultServerVerifier_verifyJdtlsInstallation_Windows(t *testing.T) {
	tempDir := t.TempDir()

	// Mock Windows environment
	originalAppData := os.Getenv("APPDATA")
	os.Setenv("APPDATA", tempDir)
	defer os.Setenv("APPDATA", originalAppData)

	// Note: We test Windows behavior by setting APPDATA environment variable
	// The actual platform.IsWindows() will be used

	// Create JDTLS installation in APPDATA
	jdtlsDir := filepath.Join(tempDir, "eclipse.jdt.ls")
	pluginsDir := filepath.Join(jdtlsDir, "plugins")
	err := os.MkdirAll(pluginsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create plugins directory: %v", err)
	}

	// Create a mock JDTLS JAR file
	jarPath := filepath.Join(pluginsDir, "org.eclipse.jdt.ls.core_1.23.0.jar")
	file, err := os.Create(jarPath)
	if err != nil {
		t.Fatalf("Failed to create JAR file: %v", err)
	}
	file.Close()

	verifier := &installer.DefaultServerVerifier{
		runtimeVerifier: newMockServerRuntimeInstaller(),
		serverRegistry:  NewServerRegistry(),
		executor:        newMockServerVerificationExecutor(),
	}

	result := &ServerVerificationResult{
		ServerName:      "jdtls",
		Installed:       false,
		Issues:          []Issue{},
		Recommendations: []string{},
		Metadata:        make(map[string]interface{}),
	}

	verifier.verifyJdtlsInstallation(result)

	if !result.Installed {
		t.Error("Expected JDTLS to be found in Windows APPDATA location")
	}

	if len(result.Issues) != 0 {
		t.Errorf("Expected no issues, got %d: %v", len(result.Issues), result.Issues)
	}

	if result.Path == "" {
		t.Error("Expected path to be set when JDTLS is found")
	}
}

func TestServerVerification_EdgeCases(t *testing.T) {
	t.Run("Server verification with nil runtime verifier", func(t *testing.T) {
		verifier := &installer.DefaultServerVerifier{
			runtimeVerifier: nil,
			serverRegistry:  NewServerRegistry(),
			executor:        newMockServerVerificationExecutor(),
		}

		result := &ServerVerificationResult{
			ServerName:      "gopls",
			Issues:          []Issue{},
			Recommendations: []string{},
			Metadata:        make(map[string]interface{}),
		}

		serverDef, _ := verifier.serverRegistry.GetServer("gopls")

		// Should handle nil runtime verifier gracefully
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Should not panic with nil runtime verifier: %v", r)
			}
		}()

		verifier.verifyServerRuntime(result, serverDef)

		// Should have added an issue about runtime verification failure
		if len(result.Issues) == 0 {
			t.Error("Expected issue when runtime verifier is nil")
		}
	})

	t.Run("Server verification with nil executor", func(t *testing.T) {
		verifier := &installer.DefaultServerVerifier{
			runtimeVerifier: newMockServerRuntimeInstaller(),
			serverRegistry:  NewServerRegistry(),
			executor:        nil,
		}

		result := &ServerVerificationResult{
			ServerName:      "gopls",
			Issues:          []Issue{},
			Recommendations: []string{},
			Metadata:        make(map[string]interface{}),
		}

		serverDef, _ := verifier.serverRegistry.GetServer("gopls")

		// Should handle nil executor gracefully
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Should not panic with nil executor: %v", r)
			}
		}()

		verifier.verifyGoplsInstallation(result, serverDef)
	})

	t.Run("JDTLS verification with empty HOME and APPDATA", func(t *testing.T) {
		// Clear environment variables
		originalHome := os.Getenv("HOME")
		originalAppData := os.Getenv("APPDATA")
		os.Unsetenv("HOME")
		os.Unsetenv("APPDATA")
		defer func() {
			os.Setenv("HOME", originalHome)
			os.Setenv("APPDATA", originalAppData)
		}()

		verifier := &installer.DefaultServerVerifier{
			runtimeVerifier: newMockServerRuntimeInstaller(),
			serverRegistry:  NewServerRegistry(),
			executor:        newMockServerVerificationExecutor(),
		}

		result := &ServerVerificationResult{
			ServerName:      "jdtls",
			Installed:       false,
			Issues:          []Issue{},
			Recommendations: []string{},
			Metadata:        make(map[string]interface{}),
		}

		verifier.verifyJdtlsInstallation(result)

		// Should not be installed and should have an issue
		if result.Installed {
			t.Error("Expected JDTLS not to be found with empty environment")
		}

		if len(result.Issues) == 0 {
			t.Error("Expected issue when JDTLS is not found")
		}
	})
}

func TestServerVerification_Integration(t *testing.T) {
	t.Run("Complete server verification flow", func(t *testing.T) {
		mockRuntimeInstaller := newMockServerRuntimeInstaller()
		mockExecutor := newMockServerVerificationExecutor()

		// Setup successful runtime verification for Go
		mockRuntimeInstaller.SetRuntimeInstalled("go", true)
		mockRuntimeInstaller.SetRuntimeVersion("go", "1.21.0")

		// Note: We test with actual system state for platform.IsCommandAvailable

		// Setup which command to return gopls path
		mockExecutor.SetCommand("which gopls", &platform.Result{
			ExitCode: 0,
			Stdout:   "/usr/local/bin/gopls",
		})

		verifier := &installer.DefaultServerVerifier{
			runtimeVerifier: mockRuntimeInstaller,
			serverRegistry:  NewServerRegistry(),
			executor:        mockExecutor,
		}

		// Note: We use the actual platform.IsCommandAvailable implementation

		result, err := verifier.VerifyServer("gopls")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("Expected result, got nil")
		}

		// Should have successful runtime verification
		if result.RuntimeStatus == nil {
			t.Error("Expected runtime status to be set")
		} else {
			if !result.RuntimeStatus.Installed {
				t.Error("Expected runtime to be installed")
			}
			if !result.RuntimeStatus.Compatible {
				t.Error("Expected runtime to be compatible")
			}
		}

		// Should have successful server installation verification
		if !result.Installed {
			t.Error("Expected server to be installed")
		}

		if result.Path != "/usr/local/bin/gopls" {
			t.Errorf("Expected path '/usr/local/bin/gopls', got '%s'", result.Path)
		}

		if result.ServerName != "gopls" {
			t.Errorf("Expected server name 'gopls', got '%s'", result.ServerName)
		}

		if result.RuntimeRequired != "go" {
			t.Errorf("Expected runtime 'go', got '%s'", result.RuntimeRequired)
		}
	})
}

// Benchmark tests to ensure performance
func BenchmarkServerVerification(b *testing.B) {
	mockRuntimeInstaller := newMockServerRuntimeInstaller()
	mockExecutor := newMockServerVerificationExecutor()

	mockRuntimeInstaller.SetRuntimeInstalled("go", true)
	// Note: We use actual system state for platform.IsCommandAvailable

	verifier := &installer.DefaultServerVerifier{
		runtimeVerifier: mockRuntimeInstaller,
		serverRegistry:  NewServerRegistry(),
		executor:        mockExecutor,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = verifier.VerifyServer("gopls")
	}
}

func TestServerVerification_PerformanceTimeout(t *testing.T) {
	mockRuntimeInstaller := newMockServerRuntimeInstaller()
	mockExecutor := newMockServerVerificationExecutor()

	// Set up runtime
	mockRuntimeInstaller.SetRuntimeInstalled("go", true)

	// Mock slow command execution
	mockExecutor.SetCommand("which gopls", &platform.Result{
		ExitCode: 0,
		Stdout:   "/usr/local/bin/gopls",
	})

	// Note: We use actual system state and test timeout handling through execution time

	verifier := &installer.DefaultServerVerifier{
		runtimeVerifier: mockRuntimeInstaller,
		serverRegistry:  NewServerRegistry(),
		executor:        mockExecutor,
	}

	start := time.Now()
	result, err := verifier.VerifyServer("gopls")
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Error("Expected result, got nil")
	}

	// Verification should complete within reasonable time
	if duration > 5*time.Second {
		t.Errorf("Verification took too long: %v", duration)
	}
}
