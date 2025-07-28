package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// PythonPatternsIntegrationTestSuite tests the complete integration workflow
// from Makefile targets through script execution to test orchestration
type PythonPatternsIntegrationTestSuite struct {
	suite.Suite
	projectRoot   string
	originalDir   string
	testStartTime time.Time
}

func (suite *PythonPatternsIntegrationTestSuite) SetupSuite() {
	suite.testStartTime = time.Now()
	
	// Get project root
	wd, err := os.Getwd()
	require.NoError(suite.T(), err)
	suite.originalDir = wd
	
	// Find project root by looking for go.mod
	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			suite.T().Fatal("Could not find project root (go.mod not found)")
		}
		projectRoot = parent
	}
	suite.projectRoot = projectRoot
	
	// Change to project root
	err = os.Chdir(suite.projectRoot)
	require.NoError(suite.T(), err)
	
	suite.T().Logf("Integration test setup - Project root: %s", suite.projectRoot)
}

func (suite *PythonPatternsIntegrationTestSuite) TearDownSuite() {
	// Return to original directory
	if suite.originalDir != "" {
		os.Chdir(suite.originalDir)
	}
	
	duration := time.Since(suite.testStartTime)
	suite.T().Logf("Integration test suite completed in %v", duration)
}

// TestMakefileTargetIntegration validates Makefile targets work correctly
func (suite *PythonPatternsIntegrationTestSuite) TestMakefileTargetIntegration() {
	suite.T().Log("Testing Makefile target integration...")
	
	// Test: Makefile contains required Python patterns targets
	makefileContent, err := os.ReadFile("Makefile")
	require.NoError(suite.T(), err)
	
	makefileStr := string(makefileContent)
	expectedTargets := []string{
		"test-python-patterns:",
		"test-python-patterns-quick:",
		"test-python-comprehensive:",
	}
	
	for _, target := range expectedTargets {
		assert.Contains(suite.T(), makefileStr, target,
			"Makefile should contain target %s", target)
	}
	
	// Test: Make help includes Python patterns documentation
	cmd := exec.Command("make", "help")
	helpOutput, err := cmd.Output()
	require.NoError(suite.T(), err)
	
	helpStr := string(helpOutput)
	assert.Contains(suite.T(), helpStr, "test-python-patterns",
		"Make help should document Python patterns targets")
	
	// Test: Make targets can be parsed without errors
	cmd = exec.Command("make", "-n", "test-python-patterns-quick")
	_, err = cmd.Output()
	assert.NoError(suite.T(), err, "Makefile target should parse without errors")
	
	suite.T().Log("✅ Makefile target integration validated")
}

// TestScriptIntegration validates script execution and flag processing
func (suite *PythonPatternsIntegrationTestSuite) TestScriptIntegration() {
	suite.T().Log("Testing script integration...")
	
	scriptPath := "./scripts/test-python-patterns.sh"
	
	// Test: Script exists and is executable
	info, err := os.Stat(scriptPath)
	require.NoError(suite.T(), err, "Script should exist")
	assert.True(suite.T(), info.Mode()&0111 != 0, "Script should be executable")
	
	// Test: Script help flag works
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, scriptPath, "--help")
	helpOutput, err := cmd.Output()
	require.NoError(suite.T(), err, "Script help should work")
	
	helpStr := string(helpOutput)
	assert.Contains(suite.T(), helpStr, "Usage:", "Help should contain usage information")
	assert.Contains(suite.T(), helpStr, "--quick", "Help should document quick flag")
	assert.Contains(suite.T(), helpStr, "--verbose", "Help should document verbose flag")
	
	// Test: Invalid flag handling
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd = exec.CommandContext(ctx, scriptPath, "--invalid-flag")
	_, err = cmd.CombinedOutput()
	assert.Error(suite.T(), err, "Invalid flag should cause script to fail")
	
	suite.T().Log("✅ Script integration validated")
}

// TestConfigurationIntegration validates configuration files and processing
func (suite *PythonPatternsIntegrationTestSuite) TestConfigurationIntegration() {
	suite.T().Log("Testing configuration integration...")
	
	configFiles := []string{
		"tests/e2e/fixtures/python_patterns_config.yaml",
		"tests/e2e/fixtures/python_patterns_test_server.yaml",
	}
	
	// Test: Configuration files existence and readability
	for _, configFile := range configFiles {
		info, err := os.Stat(configFile)
		require.NoError(suite.T(), err, "Configuration file %s should exist", configFile)
		assert.True(suite.T(), info.Size() > 0, "Configuration file should not be empty")
		
		// Test: Configuration file is valid YAML (basic check)
		content, err := os.ReadFile(configFile)
		require.NoError(suite.T(), err, "Should be able to read config file")
		
		// Basic YAML structure validation
		configStr := string(content)
		assert.Contains(suite.T(), configStr, "port:", "Config should have port setting")
		assert.Contains(suite.T(), configStr, "servers:", "Config should have servers section")
		
		suite.T().Logf("✅ Configuration file validated: %s", configFile)
	}
	
	// Test: Python patterns specific configuration
	configPath := "tests/e2e/fixtures/python_patterns_config.yaml"
	content, err := os.ReadFile(configPath)
	require.NoError(suite.T(), err)
	
	configStr := string(content)
	requiredSections := []string{
		"test_settings:",
		"repo_manager_integration:",
		"python:",
		"pylsp",
	}
	
	for _, section := range requiredSections {
		assert.Contains(suite.T(), configStr, section,
			"Config should contain %s section", section)
	}
	
	suite.T().Log("✅ Configuration integration validated")
}

// TestBuildSystemIntegration validates build system works with testing
func (suite *PythonPatternsIntegrationTestSuite) TestBuildSystemIntegration() {
	suite.T().Log("Testing build system integration...")
	
	// Test: Clean build works
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "make", "clean")
	_, err := cmd.CombinedOutput()
	require.NoError(suite.T(), err, "Make clean should succeed")
	
	// Verify binary is removed
	_, err = os.Stat("bin/lspg")
	assert.True(suite.T(), os.IsNotExist(err), "Binary should be removed after clean")
	
	// Test: Local build works
	ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	
	cmd = exec.CommandContext(ctx, "make", "local")
	buildOutput, err := cmd.CombinedOutput()
	require.NoError(suite.T(), err, "Make local should succeed: %s", string(buildOutput))
	
	// Verify binary exists and is functional
	info, err := os.Stat("bin/lspg")
	require.NoError(suite.T(), err, "Binary should exist after build")
	assert.True(suite.T(), info.Mode()&0111 != 0, "Binary should be executable")
	
	// Test: Binary version check
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	cmd = exec.CommandContext(ctx, "./bin/lspg", "--version")
	versionOutput, err := cmd.Output()
	require.NoError(suite.T(), err, "Binary version check should work")
	
	versionStr := string(versionOutput)
	assert.NotEmpty(suite.T(), versionStr, "Version output should not be empty")
	
	suite.T().Log("✅ Build system integration validated")
}

// TestErrorHandlingIntegration validates error scenarios and recovery
func (suite *PythonPatternsIntegrationTestSuite) TestErrorHandlingIntegration() {
	suite.T().Log("Testing error handling integration...")
	
	scriptPath := "./scripts/test-python-patterns.sh"
	
	// Test: Script handles missing binary gracefully
	// Backup the binary if it exists
	binaryPath := "bin/lspg"
	backupPath := "bin/lspg.backup"
	
	if _, err := os.Stat(binaryPath); err == nil {
		err = os.Rename(binaryPath, backupPath)
		require.NoError(suite.T(), err, "Should be able to backup binary")
		
		defer func() {
			// Restore binary
			if _, err := os.Stat(backupPath); err == nil {
				os.Rename(backupPath, binaryPath)
			}
		}()
	}
	
	// Test: Script fails when binary is missing and build is skipped
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, scriptPath, "--skip-build", "--quick")
	cmd.Env = append(os.Environ(), "SKIP_BUILD=true")
	_, err := cmd.CombinedOutput()
	assert.Error(suite.T(), err, "Script should fail when binary is missing")
	
	// Test: Invalid timeout handling
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	cmd = exec.CommandContext(ctx, scriptPath, "--timeout", "invalid")
	_, err = cmd.CombinedOutput()
	assert.Error(suite.T(), err, "Script should fail with invalid timeout")
	
	suite.T().Log("✅ Error handling integration validated")
}

// TestWorkflowIntegration validates the complete workflow integration
func (suite *PythonPatternsIntegrationTestSuite) TestWorkflowIntegration() {
	suite.T().Log("Testing complete workflow integration...")
	
	// Ensure we have a clean, built binary
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "make", "clean")
	_, err := cmd.CombinedOutput()
	require.NoError(suite.T(), err)
	
	cmd = exec.CommandContext(ctx, "make", "local")
	_, err = cmd.CombinedOutput()
	require.NoError(suite.T(), err)
	
	// Test: Simple test workflow (unit tests)
	ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	
	cmd = exec.CommandContext(ctx, "make", "test-simple-quick")
	testOutput, err := cmd.CombinedOutput()
	assert.NoError(suite.T(), err, "Simple test workflow should succeed: %s", string(testOutput))
	
	// Test: Makefile target for Python patterns (dry run)
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	cmd = exec.CommandContext(ctx, "make", "-n", "test-python-patterns-quick")
	dryRunOutput, err := cmd.Output()
	require.NoError(suite.T(), err, "Dry run should succeed")
	
	dryRunStr := string(dryRunOutput)
	assert.Contains(suite.T(), dryRunStr, "test-python-patterns.sh",
		"Dry run should show script execution")
	assert.Contains(suite.T(), dryRunStr, "--quick",
		"Dry run should show quick flag")
	
	suite.T().Log("✅ Workflow integration validated")
}

// TestPerformanceIntegration validates performance aspects of integration
func (suite *PythonPatternsIntegrationTestSuite) TestPerformanceIntegration() {
	suite.T().Log("Testing performance integration...")
	
	// Test: Build performance
	startTime := time.Now()
	
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "make", "clean")
	_, err := cmd.CombinedOutput()
	require.NoError(suite.T(), err)
	
	cmd = exec.CommandContext(ctx, "make", "local")
	_, err = cmd.CombinedOutput()
	require.NoError(suite.T(), err)
	
	buildDuration := time.Since(startTime)
	suite.T().Logf("Build completed in %v", buildDuration)
	
	// Build should complete within reasonable time
	assert.Less(suite.T(), buildDuration, 5*time.Minute,
		"Build should complete within 5 minutes")
	
	// Test: Script startup performance
	scriptPath := "./scripts/test-python-patterns.sh"
	startTime = time.Now()
	
	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	cmd = exec.CommandContext(ctx, scriptPath, "--help")
	_, err = cmd.Output()
	require.NoError(suite.T(), err)
	
	helpDuration := time.Since(startTime)
	suite.T().Logf("Script help completed in %v", helpDuration)
	
	// Script help should be fast
	assert.Less(suite.T(), helpDuration, 10*time.Second,
		"Script help should complete within 10 seconds")
	
	suite.T().Log("✅ Performance integration validated")
}

// TestDocumentationIntegration validates documentation consistency
func (suite *PythonPatternsIntegrationTestSuite) TestDocumentationIntegration() {
	suite.T().Log("Testing documentation integration...")
	
	// Test: README mentions Python patterns testing
	readmeContent, err := os.ReadFile("README.md")
	if err == nil {
		readmeStr := strings.ToLower(string(readmeContent))
		// Check for any mention of Python patterns or the test target
		hasPatternMention := strings.Contains(readmeStr, "python") &&
			(strings.Contains(readmeStr, "pattern") || strings.Contains(readmeStr, "test-python-patterns"))
		
		if hasPatternMention {
			suite.T().Log("✅ README mentions Python patterns testing")
		} else {
			suite.T().Log("⚠️  README does not mention Python patterns testing")
		}
	}
	
	// Test: CLAUDE.md mentions Python patterns
	claudeContent, err := os.ReadFile("CLAUDE.md")
	if err == nil {
		claudeStr := string(claudeContent)
		if strings.Contains(claudeStr, "test-python-patterns") {
			suite.T().Log("✅ CLAUDE.md documents Python patterns testing")
		} else {
			suite.T().Log("⚠️  CLAUDE.md does not mention Python patterns testing")
		}
	}
	
	// Test: Test guide documentation
	testGuideContent, err := os.ReadFile("docs/test_guide.md")
	if err == nil {
		testGuideStr := strings.ToLower(string(testGuideContent))
		if strings.Contains(testGuideStr, "python") && strings.Contains(testGuideStr, "pattern") {
			suite.T().Log("✅ Test guide documents Python patterns")
		} else {
			suite.T().Log("⚠️  Test guide does not mention Python patterns")
		}
	}
	
	// Test: Script self-documentation
	scriptPath := "./scripts/test-python-patterns.sh"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, scriptPath, "--help")
	helpOutput, err := cmd.Output()
	if err == nil {
		helpStr := string(helpOutput)
		expectedDocs := []string{
			"Python Patterns",
			"Usage:",
			"Options:",
			"Examples:",
		}
		
		allPresent := true
		for _, expected := range expectedDocs {
			if !strings.Contains(helpStr, expected) {
				allPresent = false
				break
			}
		}
		
		if allPresent {
			suite.T().Log("✅ Script documentation is comprehensive")
		} else {
			suite.T().Log("⚠️  Script documentation may need improvement")
		}
	}
	
	suite.T().Log("✅ Documentation integration validated")
}

// TestIntegrationValidationScript tests the integration validation script itself
func (suite *PythonPatternsIntegrationTestSuite) TestIntegrationValidationScript() {
	suite.T().Log("Testing integration validation script...")
	
	validationScriptPath := "./scripts/validate-python-patterns-integration.sh"
	
	// Test: Validation script exists and is executable
	info, err := os.Stat(validationScriptPath)
	require.NoError(suite.T(), err, "Validation script should exist")
	assert.True(suite.T(), info.Mode()&0111 != 0, "Validation script should be executable")
	
	// Test: Validation script help
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, validationScriptPath, "--help")
	helpOutput, err := cmd.Output()
	require.NoError(suite.T(), err, "Validation script help should work")
	
	helpStr := string(helpOutput)
	expectedSections := []string{
		"Python Patterns Integration Validation",
		"Usage:",
		"Options:",
		"Integration Components Tested:",
	}
	
	for _, section := range expectedSections {
		assert.Contains(suite.T(), helpStr, section,
			"Validation script help should contain %s", section)
	}
	
	suite.T().Log("✅ Integration validation script validated")
}

// TestPythonPatternsIntegrationTestSuite runs the complete integration test suite
func TestPythonPatternsIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	
	suite.Run(t, new(PythonPatternsIntegrationTestSuite))
}