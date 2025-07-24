package setup_test

import (
	"context"
	"lsp-gateway/internal/setup"
	"lsp-gateway/tests/mocks"
	"lsp-gateway/tests/testdata"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupOrchestrator_BasicFunctionality(t *testing.T) {
	testRunner := testdata.NewTestRunner(time.Second * 30)
	defer testRunner.Cleanup()

	mockDetector := mocks.NewMockRuntimeDetector()
	mockRuntimeInstaller := mocks.NewMockRuntimeInstaller()
	mockServerInstaller := mocks.NewMockServerInstaller()
	mockConfigGenerator := mocks.NewMockConfigGenerator()

	mockDetector.DetectAllFunc = func(ctx context.Context) (*setup.DetectionReport, error) {
		return testdata.CreateMockDetectionReport("linux", "amd64"), nil
	}

	mockConfigGenerator.GenerateFromDetectedFunc = func(ctx context.Context) (*setup.ConfigGenerationResult, error) {
		config := testdata.CreateMockGatewayConfig(8080, testdata.CreateDefaultTestServers())
		return testdata.CreateMockConfigGenerationResult(config, 4, true), nil
	}

	assert.NotNil(t, mockDetector, "Mock detector should be created")
	assert.NotNil(t, mockRuntimeInstaller, "Mock runtime installer should be created")
	assert.NotNil(t, mockServerInstaller, "Mock server installer should be created")
	assert.NotNil(t, mockConfigGenerator, "Mock config generator should be created")

	report, err := mockDetector.DetectAll(testRunner.Context())
	require.NoError(t, err, "DetectAll should not return an error")
	assert.True(t, report.Summary.TotalRuntimes > 0, "Detection report should contain runtimes")
	assert.Equal(t, "linux", string(report.Platform), "Platform should be linux")

	configResult, err := mockConfigGenerator.GenerateFromDetected(testRunner.Context())
	require.NoError(t, err, "GenerateFromDetected should not return an error")
	assert.NotNil(t, configResult.Config, "Generated config should not be nil")
	assert.Equal(t, 8080, configResult.Config.Port, "Config port should be 8080")
	assert.Equal(t, 4, configResult.ServersGenerated, "Should generate 4 servers")

	assert.Len(t, mockDetector.DetectAllCalls, 1, "DetectAll should be called once")
	assert.Len(t, mockConfigGenerator.GenerateFromDetectedCalls, 1, "GenerateFromDetected should be called once")
}

func TestMockRuntimeInstaller_InstallAndVerify(t *testing.T) {
	mockInstaller := mocks.NewMockRuntimeInstaller()

	installOptions := testdata.CreateMockInstallOptions("1.24.0", false)
	result, err := mockInstaller.Install("go", installOptions)

	require.NoError(t, err, "Install should not return an error")
	assert.True(t, result.Success, "Installation should be successful")
	assert.Equal(t, "go", result.Runtime, "Runtime should be 'go'")
	assert.Len(t, mockInstaller.InstallCalls, 1, "Install should be called once")

	verifyResult, err := mockInstaller.Verify("go")
	require.NoError(t, err, "Verify should not return an error")
	assert.True(t, verifyResult.Installed, "Runtime should be installed")
	assert.True(t, verifyResult.Compatible, "Runtime should be compatible")
	assert.Len(t, mockInstaller.VerifyCalls, 1, "Verify should be called once")

	supportedRuntimes := mockInstaller.GetSupportedRuntimes()
	assert.Contains(t, supportedRuntimes, "go", "Supported runtimes should include 'go'")
	assert.Contains(t, supportedRuntimes, "python", "Supported runtimes should include 'python'")
}

func TestMockServerInstaller_ServerOperations(t *testing.T) {
	mockInstaller := mocks.NewMockServerInstaller()

	serverOptions := testdata.CreateMockServerInstallOptions("1.0.0", false)
	result, err := mockInstaller.Install("gopls", serverOptions)

	require.NoError(t, err, "Server install should not return an error")
	assert.True(t, result.Success, "Server installation should be successful")
	assert.Equal(t, "gopls", result.Runtime, "Server should be 'gopls'")

	verifyResult, err := mockInstaller.Verify("gopls")
	require.NoError(t, err, "Server verify should not return an error")
	assert.True(t, verifyResult.Installed, "Server should be installed")
	assert.True(t, verifyResult.Compatible, "Server should be compatible")

	serverInfo, err := mockInstaller.GetServerInfo("gopls")
	require.NoError(t, err, "GetServerInfo should not return an error")
	assert.Equal(t, "gopls", serverInfo.Name, "Server name should be 'gopls'")
	assert.Equal(t, "Go Language Server", serverInfo.DisplayName, "Server display name should match")

	depResult, err := mockInstaller.ValidateDependencies("gopls")
	require.NoError(t, err, "ValidateDependencies should not return an error")
	assert.True(t, depResult.Valid, "Dependencies should be valid")
	assert.True(t, depResult.CanInstall, "Should be able to install")
}

func TestMockConfigGenerator_ConfigOperations(t *testing.T) {
	mockGenerator := mocks.NewMockConfigGenerator()

	defaultResult, err := mockGenerator.GenerateDefault()
	require.NoError(t, err, "GenerateDefault should not return an error")
	assert.NotNil(t, defaultResult.Config, "Default config should not be nil")
	assert.Equal(t, 8080, defaultResult.Config.Port, "Default port should be 8080")
	assert.True(t, defaultResult.ServersGenerated > 0, "Should generate at least one server")

	ctx := context.Background()
	runtimeResult, err := mockGenerator.GenerateForRuntime(ctx, "go")
	require.NoError(t, err, "GenerateForRuntime should not return an error")
	assert.NotNil(t, runtimeResult.Config, "Runtime config should not be nil")

	testConfig := testdata.CreateMockGatewayConfig(8080, testdata.CreateDefaultTestServers())
	validationResult, err := mockGenerator.ValidateConfig(testConfig)
	require.NoError(t, err, "ValidateConfig should not return an error")
	assert.True(t, validationResult.Valid, "Test config should be valid")
	assert.Equal(t, 4, validationResult.ServersValidated, "Should validate 4 servers")

	assert.Equal(t, 1, mockGenerator.GenerateDefaultCalls, "GenerateDefault should be called once")
	assert.Len(t, mockGenerator.GenerateForRuntimeCalls, 1, "GenerateForRuntime should be called once")
	assert.Len(t, mockGenerator.ValidateConfigCalls, 1, "ValidateConfig should be called once")
}

func TestTestHelpers_Functionality(t *testing.T) {
	testRunner := testdata.NewTestRunner(time.Second * 10)
	defer testRunner.Cleanup()

	runtimeInfo := testdata.CreateMockRuntimeInfo("go", "1.24.0", "/usr/local/go/bin/go", true, true)
	assert.Equal(t, "go", runtimeInfo.Name, "Runtime name should be 'go'")
	assert.True(t, runtimeInfo.Installed, "Runtime should be installed")
	assert.True(t, runtimeInfo.Compatible, "Runtime should be compatible")

	installResult := testdata.CreateMockInstallResult("python", "3.11.0", "/usr/bin/python3", true)
	assert.True(t, installResult.Success, "Install result should be successful")
	assert.Equal(t, "python", installResult.Runtime, "Runtime should be 'python'")

	verifyResult := testdata.CreateMockVerificationResult("nodejs", "20.0.0", "/usr/bin/node", true, true)
	assert.True(t, verifyResult.Installed, "Node.js should be installed")
	assert.True(t, verifyResult.Compatible, "Node.js should be compatible")

	config := testdata.CreateMockGatewayConfig(9090, testdata.CreateDefaultTestServers())
	assert.Equal(t, 9090, config.Port, "Config port should be 9090")
	assert.Len(t, config.Servers, 4, "Should have 4 servers")

	assert.False(t, testRunner.HasFailures(), "Test runner should have no failures")
}