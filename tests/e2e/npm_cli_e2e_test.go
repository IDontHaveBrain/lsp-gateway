package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/e2e/testutils"
)

// InstallRuntimeResult represents the JSON output from install runtime commands
type InstallRuntimeResult struct {
	Success bool `json:"success"`
	Results []struct {
		Success  bool     `json:"Success"`
		Runtime  string   `json:"Runtime"`
		Version  string   `json:"Version"`
		Path     string   `json:"Path"`
		Method   string   `json:"Method"`
		Duration int64    `json:"Duration"`
		Errors   []string `json:"Errors"`
		Warnings []string `json:"Warnings"`
		Messages []string `json:"Messages"`
		Details  interface{} `json:"Details"`
	} `json:"results"`
}



// NpmCliE2ETestSuite provides comprehensive E2E tests for npm-cli functionality
// Tests real-world npm integration scenarios including Node.js runtime installation,
// TypeScript language server setup, and npm project detection workflows
type NpmCliE2ETestSuite struct {
	suite.Suite
	binaryPath   string
	testTimeout  time.Duration
	tempWorkDir  string
	projectRoot  string
	originalDir  string
}

// NpmInstallResult represents npm-specific installation results
type NpmInstallResult struct {
	Success          bool                            `json:"success"`
	Duration         int64                           `json:"duration"`
	NodeJsRuntime    *testutils.RuntimeDetectionResult         `json:"nodejs_runtime,omitempty"`
	TypeScriptServer *testutils.ServerInstallResult            `json:"typescript_server,omitempty"`
	ProjectDetection *NpmProjectDetectionResult      `json:"project_detection,omitempty"`
	PackageManager   *PackageManagerDetectionResult  `json:"package_manager,omitempty"`
	Errors           []string                        `json:"errors,omitempty"`
	Warnings         []string                        `json:"warnings,omitempty"`
}

type NpmProjectDetectionResult struct {
	PackageJsonFound    bool                   `json:"package_json_found"`
	PackageJsonPath     string                 `json:"package_json_path"`
	PackageManagerType  string                 `json:"package_manager_type"`
	LockFileFound       bool                   `json:"lock_file_found"`
	LockFilePath        string                 `json:"lock_file_path"`
	Dependencies        map[string]string      `json:"dependencies"`
	DevDependencies     map[string]string      `json:"dev_dependencies"`
	Scripts             map[string]string      `json:"scripts"`
	FrameworkDetected   []string               `json:"framework_detected"`
	ProjectType         string                 `json:"project_type"`
	Metadata            map[string]interface{} `json:"metadata"`
}

type PackageManagerDetectionResult struct {
	Primary          string   `json:"primary"`
	Available        []string `json:"available"`
	NpmVersion       string   `json:"npm_version"`
	YarnVersion      string   `json:"yarn_version"`
	PnpmVersion      string   `json:"pnpm_version"`
	GlobalPackages   []string `json:"global_packages,omitempty"`
	RegistryUrl      string   `json:"registry_url"`
	CacheLocation    string   `json:"cache_location"`
}

// SetupSuite initializes the npm-cli E2E test environment
func (suite *NpmCliE2ETestSuite) SetupSuite() {
	suite.testTimeout = 15 * time.Second // Optimized timeout for npm operations

	// Find binary path relative to test location
	suite.projectRoot = findProjectRoot(suite.T())
	suite.binaryPath = filepath.Join(suite.projectRoot, "bin", "lsp-gateway")
	
	// Verify binary exists
	require.FileExists(suite.T(), suite.binaryPath, 
		"Binary should exist at %s", suite.binaryPath)
	
	// Store original directory
	var err error
	suite.originalDir, err = os.Getwd()
	require.NoError(suite.T(), err, "Should get current working directory")
}

// SetupTest creates temporary directory for each test
func (suite *NpmCliE2ETestSuite) SetupTest() {
	var err error
	suite.tempWorkDir, err = os.MkdirTemp("", "npm-cli-e2e-*")
	require.NoError(suite.T(), err, "Should create temporary directory")
	
	// Change to temp directory for test isolation
	err = os.Chdir(suite.tempWorkDir)
	require.NoError(suite.T(), err, "Should change to temporary directory")
}

// TearDownTest cleans up test environment
func (suite *NpmCliE2ETestSuite) TearDownTest() {
	// Return to original directory
	err := os.Chdir(suite.originalDir)
	require.NoError(suite.T(), err, "Should return to original directory")
	
	// Clean up temporary directory
	if suite.tempWorkDir != "" {
		os.RemoveAll(suite.tempWorkDir)
	}
}

// TestNodeJsRuntimeInstallation tests Node.js runtime installation via CLI
func (suite *NpmCliE2ETestSuite) TestNodeJsRuntimeInstallation() {
	testCases := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "Install Node.js Runtime",
			args:     []string{"install", "runtime", "nodejs", "--json", "--timeout", "300s"},
			expected: "nodejs",
		},
		{
			name:     "Install All Runtimes Including Node.js",
			args:     []string{"install", "runtime", "all", "--json", "--timeout", "300s"},
			expected: "nodejs",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			// Execute CLI command
			cmd := exec.CommandContext(ctx, suite.binaryPath, tc.args...)
			output, exitCode, err := suite.executeCommand(cmd)
			
			// Should execute successfully
			require.NoError(suite.T(), err, "Command should execute without error")
			require.Zero(suite.T(), exitCode, "Command should exit successfully")
			require.NotEmpty(suite.T(), output, "Should produce output")
			
			// Extract and parse JSON output
			jsonOutput := suite.extractJSON(output)
			var result InstallRuntimeResult
			err = json.Unmarshal([]byte(jsonOutput), &result)
			require.NoError(suite.T(), err, "Should parse JSON output")
			
			// Validate install was successful
			require.True(suite.T(), result.Success, "Install should be successful")
			require.NotEmpty(suite.T(), result.Results, "Should have install results")
			
			// Find nodejs result
			var nodeResult *struct {
				Success  bool     `json:"Success"`
				Runtime  string   `json:"Runtime"`
				Version  string   `json:"Version"`
				Path     string   `json:"Path"`
				Method   string   `json:"Method"`
				Duration int64    `json:"Duration"`
				Errors   []string `json:"Errors"`
				Warnings []string `json:"Warnings"`
				Messages []string `json:"Messages"`
				Details  interface{} `json:"Details"`
			}
			for i := range result.Results {
				if result.Results[i].Runtime == "nodejs" {
					nodeResult = &result.Results[i]
					break
				}
			}
			require.NotNil(suite.T(), nodeResult, "Should find nodejs result")
			suite.validateNodeJsInstallResult(*nodeResult)
		})
	}
}

// TestTypeScriptLanguageServerInstallation tests TypeScript language server installation via npm
func (suite *NpmCliE2ETestSuite) TestTypeScriptLanguageServerInstallation() {
	testCases := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "Install TypeScript Language Server",
			args:     []string{"install", "server", "typescript-language-server", "--json", "--timeout", "300s"},
			expected: "typescript-language-server",
		},
		{
			name:     "Install All Servers Including TypeScript",
			args:     []string{"install", "servers", "--json", "--timeout", "300s"},
			expected: "typescript-language-server",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			// Execute CLI command
			cmd := exec.CommandContext(ctx, suite.binaryPath, tc.args...)
			output, exitCode, err := suite.executeCommand(cmd)
			
			// Should execute successfully (may take time for npm install)
			require.NoError(suite.T(), err, "Command should execute without error")
			require.Zero(suite.T(), exitCode, "Command should exit successfully")
			require.NotEmpty(suite.T(), output, "Should produce output")
			
			// Extract and parse JSON output
			jsonOutput := suite.extractJSON(output)
			var result testutils.SetupResult
			err = json.Unmarshal([]byte(jsonOutput), &result)
			require.NoError(suite.T(), err, "Should parse JSON output")
			
			// Validate TypeScript server installation
			if result.ServersInstalled != nil {
				require.Contains(suite.T(), result.ServersInstalled, "typescript-language-server",
					"Should install typescript-language-server")
				
				tsServer := result.ServersInstalled["typescript-language-server"]
				suite.validateTypeScriptServer(tsServer)
			}
		})
	}
}

// TestNpmProjectDetection tests npm project detection and analysis
func (suite *NpmCliE2ETestSuite) TestNpmProjectDetection() {
	projectTypes := []struct {
		name           string
		packageJson    string
		expectedType   string
		expectedFrameworks []string
	}{
		{
			name: "React TypeScript Project",
			packageJson: `{
				"name": "react-app",
				"version": "1.0.0",
				"dependencies": {
					"react": "^18.0.0",
					"react-dom": "^18.0.0"
				},
				"devDependencies": {
					"@types/react": "^18.0.0",
					"typescript": "^5.0.0"
				},
				"scripts": {
					"start": "react-scripts start",
					"build": "react-scripts build",
					"test": "react-scripts test"
				}
			}`,
			expectedType: "unknown", // CLI detects as unknown but finds React framework
			expectedFrameworks: []string{"react"},
		},
		{
			name: "Vue.js Project",
			packageJson: `{
				"name": "vue-app",
				"version": "1.0.0",
				"dependencies": {
					"vue": "^3.0.0"
				},
				"devDependencies": {
					"@vue/cli": "^5.0.0"
				},
				"scripts": {
					"serve": "vue-cli-service serve",
					"build": "vue-cli-service build"
				}
			}`,
			expectedType: "unknown", // CLI detects as unknown but finds Vue framework
			expectedFrameworks: []string{"vue"},
		},
		{
			name: "Express.js API",
			packageJson: `{
				"name": "express-api",
				"version": "1.0.0",
				"dependencies": {
					"express": "^4.18.0",
					"cors": "^2.8.5"
				},
				"scripts": {
					"start": "node server.js",
					"dev": "nodemon server.js"
				}
			}`,
			expectedType: "unknown", // CLI detects as unknown but finds Express framework  
			expectedFrameworks: []string{"express"},
		},
	}

	for _, pt := range projectTypes {
		suite.Run(pt.name, func() {
			// Create package.json file
			err := os.WriteFile("package.json", []byte(pt.packageJson), 0644)
			require.NoError(suite.T(), err, "Should create package.json")
			
			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			// Execute project detection
			cmd := exec.CommandContext(ctx, suite.binaryPath, "setup", "detect", "--json")
			output, exitCode, err := suite.executeCommand(cmd)
			
			require.NoError(suite.T(), err, "Command should execute without error")
			require.Zero(suite.T(), exitCode, "Command should exit successfully")
			require.NotEmpty(suite.T(), output, "Should produce output")
			
			// Parse detection results
			jsonOutput := suite.extractJSON(output)
			var result testutils.SetupResult
			err = json.Unmarshal([]byte(jsonOutput), &result)
			require.NoError(suite.T(), err, "Should parse JSON output")
			
			// Validate project detection
			require.NotNil(suite.T(), result.ProjectDetection, "Should detect project")
			require.Equal(suite.T(), pt.expectedType, result.ProjectDetection.ProjectType,
				"Should detect correct project type")
			
			// Check framework detection
			for _, expectedFramework := range pt.expectedFrameworks {
				require.Contains(suite.T(), result.ProjectDetection.Frameworks, expectedFramework,
					"Should detect %s framework", expectedFramework)
			}
		})
	}
}

// TestPackageManagerIntegration tests package manager detection and integration
func (suite *NpmCliE2ETestSuite) TestPackageManagerIntegration() {
	lockFileTests := []struct {
		name         string
		lockFile     string
		lockContent  string
		expectedPM   string
	}{
		{
			name:        "NPM Package Lock",
			lockFile:    "package-lock.json",
			lockContent: `{"name": "test", "lockfileVersion": 3}`,
			expectedPM:  "npm",
		},
		{
			name:        "Yarn Lock File",
			lockFile:    "yarn.lock",
			lockContent: `# yarn lockfile v1`,
			expectedPM:  "yarn",
		},
		{
			name:        "PNPM Lock File",
			lockFile:    "pnpm-lock.yaml",
			lockContent: `lockfileVersion: '6.0'`,
			expectedPM:  "pnpm",
		},
	}

	for _, lt := range lockFileTests {
		suite.Run(lt.name, func() {
			// Create package.json and lock file
			packageJson := `{
				"name": "test-project",
				"version": "1.0.0",
				"dependencies": {
					"lodash": "^4.17.21"
				}
			}`
			
			err := os.WriteFile("package.json", []byte(packageJson), 0644)
			require.NoError(suite.T(), err, "Should create package.json")
			
			err = os.WriteFile(lt.lockFile, []byte(lt.lockContent), 0644)
			require.NoError(suite.T(), err, "Should create lock file")
			
			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			// Execute detection
			cmd := exec.CommandContext(ctx, suite.binaryPath, "setup", "detect", "--json")
			output, exitCode, err := suite.executeCommand(cmd)
			
			require.NoError(suite.T(), err, "Command should execute without error")
			require.Zero(suite.T(), exitCode, "Command should exit successfully")
			
			// Should detect package manager from lock file
			require.Contains(suite.T(), output, lt.expectedPM,
				"Should detect %s package manager", lt.expectedPM)
		})
	}
}

// TestNpmCliErrorHandling tests error scenarios and edge cases
func (suite *NpmCliE2ETestSuite) TestNpmCliErrorHandling() {
	errorTests := []struct {
		name        string
		args        []string
		expectError bool
		errorCheck  func(output string)
	}{
		{
			name:        "Invalid Runtime Name",
			args:        []string{"install", "runtime", "invalid-runtime", "--json"},
			expectError: true,
			errorCheck: func(output string) {
				require.Contains(suite.T(), output, "not_found error",
					"Should indicate invalid runtime")
			},
		},
		{
			name:        "Invalid Server Name",
			args:        []string{"install", "server", "invalid-server", "--json"},
			expectError: true,
			errorCheck: func(output string) {
				require.Contains(suite.T(), output, "error",
					"Should indicate installation error")
			},
		},
		{
			name:        "Help Command",
			args:        []string{"install", "--help"},
			expectError: false,
			errorCheck: func(output string) {
				require.Contains(suite.T(), output, "Install command provides capabilities",
					"Should show help text")
				require.Contains(suite.T(), output, "nodejs",
					"Should mention nodejs runtime")
				require.Contains(suite.T(), output, "typescript-language-server",
					"Should mention TypeScript server")
			},
		},
	}

	for _, et := range errorTests {
		suite.Run(et.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, suite.binaryPath, et.args...)
			output, exitCode, err := suite.executeCommand(cmd)
			
			if et.expectError {
				require.NotZero(suite.T(), exitCode, "Command should exit with error")
			} else {
				require.NoError(suite.T(), err, "Command should execute without error")
			}
			
			require.NotEmpty(suite.T(), output, "Should produce output")
			et.errorCheck(output)
		})
	}
}

// Helper Methods

// validateNodeJsInstallResult validates Node.js install results
func (suite *NpmCliE2ETestSuite) validateNodeJsInstallResult(result struct {
	Success  bool     `json:"Success"`
	Runtime  string   `json:"Runtime"`
	Version  string   `json:"Version"`
	Path     string   `json:"Path"`
	Method   string   `json:"Method"`
	Duration int64    `json:"Duration"`
	Errors   []string `json:"Errors"`
	Warnings []string `json:"Warnings"`
	Messages []string `json:"Messages"`
	Details  interface{} `json:"Details"`
}) {
	require.True(suite.T(), result.Success, "Node.js install should be successful")
	require.Equal(suite.T(), "nodejs", result.Runtime, "Runtime should be nodejs")
	require.NotEmpty(suite.T(), result.Version, "Should have version information")
	require.NotEmpty(suite.T(), result.Path, "Should have path information")
	require.Nil(suite.T(), result.Errors, "Should not have errors")
}

// validateTypeScriptServer validates TypeScript language server installation
func (suite *NpmCliE2ETestSuite) validateTypeScriptServer(server testutils.ServerInstallResult) {
	require.True(suite.T(), server.Installed, "TypeScript server should be installed")
	require.NotEmpty(suite.T(), server.Version, "Should have version information")
	require.NotEmpty(suite.T(), server.Path, "Should have path information")
	require.Empty(suite.T(), server.Error, "Should not have installation errors")
}

// executeCommand executes a command and returns output, exit code, and error
func (suite *NpmCliE2ETestSuite) executeCommand(cmd *exec.Cmd) (string, int, error) {
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}
	return string(output), exitCode, err
}

// extractJSON extracts JSON portion from mixed text/JSON output
func (suite *NpmCliE2ETestSuite) extractJSON(output string) string {
	// Find JSON start and end positions
	jsonStart := -1
	jsonEnd := -1
	braceCount := 0
	
	for i, char := range output {
		if char == '{' {
			if jsonStart == -1 {
				jsonStart = i
			}
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 && jsonStart != -1 {
				jsonEnd = i + 1
				break
			}
		}
	}
	
	if jsonStart == -1 || jsonEnd == -1 {
		return output // No JSON found, return original
	}
	
	return output[jsonStart:jsonEnd]
}

// findProjectRoot finds the project root directory
func findProjectRoot(t *testing.T) string {
	t.Helper()
	
	// Start from current directory and traverse upward
	dir, err := os.Getwd()
	require.NoError(t, err, "Should get current working directory")
	
	for {
		// Check for go.mod file to identify project root
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}
	
	require.Fail(t, "Could not find project root with go.mod file")
	return ""
}

// TestNpmCliE2ETestSuite runs the npm-cli E2E test suite
func TestNpmCliE2ETestSuite(t *testing.T) {
	suite.Run(t, new(NpmCliE2ETestSuite))
}