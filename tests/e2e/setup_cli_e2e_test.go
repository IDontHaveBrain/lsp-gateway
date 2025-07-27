package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// SetupCliE2ETestSuite provides comprehensive E2E tests for setup CLI commands
// using actual binary execution to test real-world scenarios
type SetupCliE2ETestSuite struct {
	suite.Suite
	binaryPath   string
	testTimeout  time.Duration
	tempWorkDir  string
	projectRoot  string
	originalDir  string
}

// SetupResult represents the JSON output structure from setup commands
type SetupResult struct {
	Success               bool                               `json:"success"`
	Duration              int64                              `json:"duration"`
	RuntimesDetected      map[string]RuntimeDetectionResult `json:"runtimes_detected"`
	ServersInstalled      map[string]ServerInstallResult    `json:"servers_installed,omitempty"`
	ConfigGeneration      *ConfigGenerationResult           `json:"config_generation,omitempty"`
	ProjectDetection      *ProjectDetectionResult           `json:"project_detection,omitempty"`
	ValidationResults     map[string]ValidationResult       `json:"validation_results,omitempty"`
	Errors                []string                           `json:"errors,omitempty"`
	Warnings              []string                           `json:"warnings,omitempty"`
	Summary               *SetupSummary                     `json:"summary"`
	StatusMessages        []string                           `json:"status_messages,omitempty"`
}

type RuntimeDetectionResult struct {
	Name           string                 `json:"Name"`
	Installed      bool                   `json:"Installed"`
	Version        string                 `json:"Version"`
	ParsedVersion  interface{}            `json:"ParsedVersion"`
	Compatible     bool                   `json:"Compatible"`
	MinVersion     string                 `json:"MinVersion"`
	Path           string                 `json:"Path"`
	WorkingDir     string                 `json:"WorkingDir"`
	DetectionCmd   string                 `json:"DetectionCmd"`
	Issues         []string               `json:"Issues"`
	Warnings       []string               `json:"Warnings"`
	Metadata       map[string]interface{} `json:"Metadata"`
	DetectedAt     string                 `json:"DetectedAt"`
	Duration       int64                  `json:"Duration"`
}

type ServerInstallResult struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Error     string `json:"error,omitempty"`
}

type ConfigGenerationResult struct {
	Generated    bool   `json:"generated"`
	Path         string `json:"path"`
	TemplateUsed string `json:"template_used,omitempty"`
	Error        string `json:"error,omitempty"`
}

type ProjectDetectionResult struct {
	ProjectType     string              `json:"project_type"`
	Languages       []string            `json:"languages"`
	Frameworks      []string            `json:"frameworks"`
	BuildSystems    []string            `json:"build_systems"`
	Confidence      float64             `json:"confidence"`
	Recommendations map[string]string   `json:"recommendations"`
}

type ValidationResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type SetupSummary struct {
	TotalRuntimes        int `json:"total_runtimes"`
	RuntimesInstalled    int `json:"runtimes_installed"`
	RuntimesAlreadyExist int `json:"runtimes_already_exist"`
	RuntimesFailed       int `json:"runtimes_failed"`
	TotalServers         int `json:"total_servers"`
	ServersInstalled     int `json:"servers_installed"`
	ServersAlreadyExist  int `json:"servers_already_exist"`
	ServersFailed        int `json:"servers_failed"`
}

// SetupSuite initializes the test suite with binary path and test environment
func (suite *SetupCliE2ETestSuite) SetupSuite() {
	suite.testTimeout = 120 * time.Second
	
	// Get project root and binary path
	projectRoot, err := os.Getwd()
	require.NoError(suite.T(), err, "Should get current working directory")
	
	// Navigate to project root if we're in tests directory
	for strings.Contains(projectRoot, "/tests") {
		projectRoot = filepath.Dir(projectRoot)
	}
	suite.projectRoot = projectRoot
	
	suite.binaryPath = filepath.Join(projectRoot, "bin", "lsp-gateway")
	require.FileExists(suite.T(), suite.binaryPath, "Binary should exist at %s", suite.binaryPath)
	
	// Store original directory
	suite.originalDir, _ = os.Getwd()
}

// SetupTest creates a fresh test environment for each test
func (suite *SetupCliE2ETestSuite) SetupTest() {
	// Create temporary working directory for each test
	tempDir, err := os.MkdirTemp("", "lsp-gateway-setup-e2e-*")
	require.NoError(suite.T(), err, "Should create temp directory")
	suite.tempWorkDir = tempDir
	
	// Change to temp directory for test execution
	err = os.Chdir(suite.tempWorkDir)
	require.NoError(suite.T(), err, "Should change to temp directory")
}

// TearDownTest cleans up the test environment
func (suite *SetupCliE2ETestSuite) TearDownTest() {
	// Restore original directory
	if suite.originalDir != "" {
		os.Chdir(suite.originalDir)
	}
	
	// Clean up temporary directory
	if suite.tempWorkDir != "" {
		os.RemoveAll(suite.tempWorkDir)
	}
}

// TestSetupAllCommand_BasicExecution tests the basic setup all command execution
func (suite *SetupCliE2ETestSuite) TestSetupAllCommand_BasicExecution() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	// Execute setup all command with JSON output and short timeout
	cmd := exec.CommandContext(ctx, suite.binaryPath, "setup", "all", "--json", "--timeout", "30s", "--skip-verify")
	output, exitCode, err := suite.executeCommand(cmd)
	
	// Validate command execution
	suite.Nil(err, "Command should execute without system errors")
	suite.True(exitCode == 0 || exitCode == 1, "Exit code should be 0 (success) or 1 (partial success)")
	suite.NotEmpty(output, "Should produce output")
	
	// Validate JSON output structure
	var result SetupResult
	suite.validateJSONOutput(output, &result)
	
	// Validate basic result structure
	suite.NotNil(result.Summary, "Summary should be present")
	suite.NotNil(result.RuntimesDetected, "Runtime detection should be present")
	if result.ServersInstalled != nil {
		suite.NotNil(result.ServersInstalled, "Server installation should be present if available")
	}
	
	if exitCode == 0 {
		suite.True(result.Success, "Result should be marked as successful when exit code is 0")
		suite.GreaterOrEqual(result.Summary.TotalRuntimes, 0, "Should have checked some runtimes")
	}
	
	// Validate that basic runtimes are checked
	expectedRuntimes := []string{"go", "python", "nodejs", "java"}
	for _, runtime := range expectedRuntimes {
		if detection, exists := result.RuntimesDetected[runtime]; exists {
			suite.NotEmpty(detection.Path, "Runtime path should be provided if found")
			if detection.Installed {
				suite.NotEmpty(detection.Version, "Version should be provided for found runtime")
			}
		}
	}
}

// TestSetupAllCommand_JSONOutputValidation tests comprehensive JSON output validation
func (suite *SetupCliE2ETestSuite) TestSetupAllCommand_JSONOutputValidation() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()
	
	// Execute with verbose JSON output
	cmd := exec.CommandContext(ctx, suite.binaryPath, "setup", "all", "--json", "--verbose", "--timeout", "45s")
	output, exitCode, err := suite.executeCommand(cmd)
	
	suite.Nil(err, "Command should execute without system errors")
	suite.Contains([]int{0, 1}, exitCode, "Exit code should be 0 or 1")
	
	// Parse and validate comprehensive JSON structure
	var result SetupResult
	suite.validateJSONOutput(output, &result)
	
	// Validate required fields are present
	suite.Greater(result.Duration, int64(0), "Duration should be present and positive")
	suite.NotNil(result.Summary, "Summary should be present")
	suite.GreaterOrEqual(result.Summary.TotalRuntimes, 0, "Should have checked runtimes")
	
	// Validate runtime detection contains expected fields
	for runtime, detection := range result.RuntimesDetected {
		suite.NotEmpty(runtime, "Runtime name should not be empty")
		if detection.Installed {
			suite.NotEmpty(detection.Version, "Found runtime should have version")
			suite.NotEmpty(detection.Path, "Found runtime should have path")
		}
		if len(detection.Issues) > 0 {
			suite.False(detection.Installed, "Runtime with issues should not be marked as installed")
		}
	}
	
	// Validate server installation results structure
	if result.ServersInstalled != nil {
		for server, installation := range result.ServersInstalled {
			suite.NotEmpty(server, "Server name should not be empty")
			if installation.Installed {
				suite.NotEmpty(installation.Path, "Installed server should have path")
			}
		}
	}
	
	// If config generation occurred, validate its structure
	if result.ConfigGeneration != nil {
		if result.ConfigGeneration.Generated {
			suite.NotEmpty(result.ConfigGeneration.Path, "Generated config should have path")
			suite.FileExists(result.ConfigGeneration.Path, "Generated config file should exist")
		}
	}
}

// TestSetupDetectCommand_ProjectDetection tests the setup detect command
func (suite *SetupCliE2ETestSuite) TestSetupDetectCommand_ProjectDetection() {
	// Create a sample Go project structure
	suite.createSampleGoProject()
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Execute setup detect command
	cmd := exec.CommandContext(ctx, suite.binaryPath, "setup", "detect", "--project-path", ".", "--verbose", "--json")
	output, exitCode, err := suite.executeCommand(cmd)
	
	suite.Nil(err, "Command should execute without system errors")
	suite.Equal(0, exitCode, "Detect command should succeed")
	suite.NotEmpty(output, "Should produce output")
	
	// Parse JSON output
	var result SetupResult
	suite.validateJSONOutput(output, &result)
	
	// Validate project detection results
	suite.NotNil(result.ProjectDetection, "Project detection should be present")
	if result.ProjectDetection != nil {
		suite.Contains(result.ProjectDetection.Languages, "go", "Should detect Go language")
		suite.Greater(result.ProjectDetection.Confidence, 0.5, "Should have reasonable confidence")
		suite.NotEmpty(result.ProjectDetection.ProjectType, "Should have a project type")
		suite.NotNil(result.ProjectDetection.Recommendations, "Should have recommendations")
	} else {
		suite.Fail("ProjectDetection is nil - project detection failed to populate results")
	}
	
	// Validate that Go runtime is detected
	if goDetection, exists := result.RuntimesDetected["go"]; exists && goDetection.Installed {
		suite.NotEmpty(goDetection.Version, "Go version should be detected")
		suite.NotEmpty(goDetection.Path, "Go path should be detected")
	}
}

// TestSetupCommand_ErrorHandling tests error scenarios and invalid flags
func (suite *SetupCliE2ETestSuite) TestSetupCommand_ErrorHandling() {
	tests := []struct {
		name           string
		args           []string
		expectedExit   int
		shouldContain  []string
		shouldNotContain []string
	}{
		{
			name:         "InvalidSubcommand",
			args:         []string{"setup", "invalid-subcommand"},
			expectedExit: 1,
			shouldContain: []string{"Error", "invalid-subcommand"},
		},
		{
			name:         "InvalidFlag",
			args:         []string{"setup", "all", "--invalid-flag"},
			expectedExit: 1,
			shouldContain: []string{"unknown flag"},
		},
		{
			name:         "InvalidTimeoutValue",
			args:         []string{"setup", "all", "--timeout", "invalid"},
			expectedExit: 1,
			shouldContain: []string{"invalid duration"},
		},
		{
			name:         "InvalidProjectPath",
			args:         []string{"setup", "detect", "--project-path", "/nonexistent/path"},
			expectedExit: 1,
			shouldContain: []string{"path", "not exist"},
		},
		{
			name:         "HelpFlag",
			args:         []string{"setup", "--help"},
			expectedExit: 0,
			shouldContain: []string{"Usage", "setup", "Examples"},
		},
	}
	
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			cmd := exec.CommandContext(ctx, suite.binaryPath)
			cmd.Args = append([]string{suite.binaryPath}, tt.args...)
			
			output, exitCode, err := suite.executeCommand(cmd)
			
			suite.Nil(err, "Command should execute without system errors")
			suite.Equal(tt.expectedExit, exitCode, "Exit code should match expected")
			
			for _, shouldContain := range tt.shouldContain {
				suite.Contains(strings.ToLower(output), strings.ToLower(shouldContain), 
					"Output should contain '%s'", shouldContain)
			}
			
			for _, shouldNotContain := range tt.shouldNotContain {
				suite.NotContains(strings.ToLower(output), strings.ToLower(shouldNotContain), 
					"Output should not contain '%s'", shouldNotContain)
			}
		})
	}
}

// TestSetupCommand_TimeoutScenarios tests timeout handling
func (suite *SetupCliE2ETestSuite) TestSetupCommand_TimeoutScenarios() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	// Test with very short timeout
	cmd := exec.CommandContext(ctx, suite.binaryPath, "setup", "all", "--timeout", "1ms", "--json")
	output, exitCode, err := suite.executeCommand(cmd)
	
	// Should handle timeout gracefully
	suite.Nil(err, "Command should execute without system errors")
	suite.True(exitCode != 0, "Should exit with non-zero code on timeout")
	
	if strings.Contains(output, "{") {
		// If JSON output is present, validate it
		var result SetupResult
		if suite.validateJSONOutputNoFail(output, &result) {
			suite.False(result.Success, "Result should not be successful on timeout")
			suite.Greater(len(result.Errors), 0, "Should have error messages")
		}
	} else {
		// If not JSON, should contain timeout indication
		suite.Contains(strings.ToLower(output), "timeout", "Output should mention timeout")
	}
}

// TestSetupWizardCommand_NonInteractive tests wizard command in non-interactive scenarios
func (suite *SetupCliE2ETestSuite) TestSetupWizardCommand_NonInteractive() {
	// Test wizard command help
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, suite.binaryPath, "setup", "wizard", "--help")
	output, exitCode, err := suite.executeCommand(cmd)
	
	suite.Nil(err, "Command should execute without system errors")
	suite.Equal(0, exitCode, "Help should succeed")
	suite.Contains(output, "wizard", "Help should mention wizard")
	suite.Contains(output, "Interactive", "Help should mention interactive setup")
}

// TestSetupTemplateCommand_TemplateValidation tests template command with various templates
func (suite *SetupCliE2ETestSuite) TestSetupTemplateCommand_TemplateValidation() {
	tests := []struct {
		name     string
		template string
		shouldSucceed bool
	}{
		{"ValidMonorepoTemplate", "monorepo", true},
		{"ValidSingleLanguageTemplate", "single-language", true},
		{"InvalidTemplate", "nonexistent-template", false},
	}
	
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			
			cmd := exec.CommandContext(ctx, suite.binaryPath, "setup", "template", 
				"--template", tt.template, "--json", "--timeout", "30s")
			output, exitCode, err := suite.executeCommand(cmd)
			
			suite.Nil(err, "Command should execute without system errors")
			
			if tt.shouldSucceed {
				suite.Contains([]int{0, 1}, exitCode, "Valid template should succeed or partially succeed")
			} else {
				suite.NotEqual(0, exitCode, "Invalid template should fail")
				suite.Contains(strings.ToLower(output), "template", "Error should mention template")
			}
		})
	}
}

// Helper methods

// executeCommand executes a command and returns output, exit code, and error
func (suite *SetupCliE2ETestSuite) executeCommand(cmd *exec.Cmd) (string, int, error) {
	output, err := cmd.CombinedOutput()
	exitCode := 0
	
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return string(output), -1, err
		}
	}
	
	return string(output), exitCode, nil
}

// validateJSONOutput validates that output is valid JSON and unmarshals it
func (suite *SetupCliE2ETestSuite) validateJSONOutput(output string, result *SetupResult) {
	output = suite.extractJSONFromOutput(output)
	suite.True(json.Valid([]byte(output)), "Output should be valid JSON: %s", output)
	
	err := json.Unmarshal([]byte(output), result)
	suite.NoError(err, "Should be able to unmarshal JSON output")
}

// validateJSONOutputNoFail same as validateJSONOutput but doesn't fail test, returns success
func (suite *SetupCliE2ETestSuite) validateJSONOutputNoFail(output string, result *SetupResult) bool {
	output = suite.extractJSONFromOutput(output)
	if !json.Valid([]byte(output)) {
		return false
	}
	
	err := json.Unmarshal([]byte(output), result)
	return err == nil
}

// extractJSONFromOutput extracts JSON content from mixed output
func (suite *SetupCliE2ETestSuite) extractJSONFromOutput(output string) string {
	// First try to find a JSON object by looking for braces
	startIdx := strings.Index(output, "{")
	if startIdx == -1 {
		return output
	}
	
	// Count braces to find the end of the JSON object
	braceCount := 0
	inString := false
	escape := false
	
	for i := startIdx; i < len(output); i++ {
		char := output[i]
		
		if !inString {
			switch char {
			case '{':
				braceCount++
			case '}':
				braceCount--
				if braceCount == 0 {
					return output[startIdx : i+1]
				}
			case '"':
				inString = true
			}
		} else {
			if escape {
				escape = false
			} else if char == '\\' {
				escape = true
			} else if char == '"' {
				inString = false
			}
		}
	}
	
	// If we couldn't find the end, return from start to end
	return output[startIdx:]
}

// createSampleGoProject creates a minimal Go project for testing
func (suite *SetupCliE2ETestSuite) createSampleGoProject() {
	// Create go.mod file
	goMod := `module test-project

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
)
`
	err := os.WriteFile("go.mod", []byte(goMod), 0644)
	require.NoError(suite.T(), err, "Should create go.mod file")
	
	// Create main.go file
	mainGo := `package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	fmt.Println("Starting server...")
	r.Run()
}
`
	err = os.WriteFile("main.go", []byte(mainGo), 0644)
	require.NoError(suite.T(), err, "Should create main.go file")
}

// TestSetupCliE2ETestSuite runs the complete setup CLI E2E test suite
func TestSetupCliE2ETestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}
	
	suite.Run(t, new(SetupCliE2ETestSuite))
}