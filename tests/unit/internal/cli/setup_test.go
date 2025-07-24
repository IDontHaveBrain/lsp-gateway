package cli_test

import (
	"bytes"
	"lsp-gateway/internal/cli"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/types"
	"lsp-gateway/tests/mocks"
	"lsp-gateway/tests/testdata"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupCommand_MockIntegration(t *testing.T) {
	testRunner := testdata.NewTestRunner(time.Second * 30)
	defer testRunner.Cleanup()

	mockDetector := mocks.NewMockRuntimeDetector()
	mockRuntimeInstaller := mocks.NewMockRuntimeInstaller()
	mockServerInstaller := mocks.NewMockServerInstaller()
	mockConfigGenerator := mocks.NewMockConfigGenerator()

	t.Run("DetectRuntimes", func(t *testing.T) {
		report, err := mockDetector.DetectAll(testRunner.Context())
		require.NoError(t, err, "Runtime detection should succeed")
		assert.Equal(t, 4, report.Summary.TotalRuntimes, "Should detect 4 runtimes")
		assert.Equal(t, 4, report.Summary.InstalledRuntimes, "All runtimes should be installed")
		assert.Equal(t, 100.0, report.Summary.SuccessRate, "Success rate should be 100%")
	})

	t.Run("InstallMissingRuntimes", func(t *testing.T) {
		installOptions := testdata.CreateMockInstallOptions("latest", false)

		for _, runtime := range []string{"go", "python", "nodejs", "java"} {
			result, err := mockRuntimeInstaller.Install(runtime, installOptions)
			require.NoError(t, err, "Runtime installation should succeed for "+runtime)
			assert.True(t, result.Success, "Installation should be successful for "+runtime)
			assert.Equal(t, runtime, result.Runtime, "Runtime name should match for "+runtime)
		}

		assert.Len(t, mockRuntimeInstaller.InstallCalls, 4, "Should install 4 runtimes")
	})

	t.Run("InstallLanguageServers", func(t *testing.T) {
		serverOptions := testdata.CreateMockServerInstallOptions("latest", false)
		servers := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}

		for _, server := range servers {
			depResult, err := mockServerInstaller.ValidateDependencies(server)
			require.NoError(t, err, "Dependency validation should succeed for "+server)
			assert.True(t, depResult.Valid, "Dependencies should be valid for "+server)

			if depResult.CanInstall {
				result, err := mockServerInstaller.Install(server, serverOptions)
				require.NoError(t, err, "Server installation should succeed for "+server)
				assert.True(t, result.Success, "Server installation should be successful for "+server)
			}
		}

		assert.Len(t, mockServerInstaller.ValidateDependenciesCalls, 4, "Should validate dependencies for 4 servers")
		assert.Len(t, mockServerInstaller.InstallCalls, 4, "Should install 4 servers")
	})

	t.Run("GenerateConfiguration", func(t *testing.T) {
		configResult, err := mockConfigGenerator.GenerateFromDetected(testRunner.Context())
		require.NoError(t, err, "Config generation should succeed")
		assert.NotNil(t, configResult.Config, "Generated config should not be nil")
		assert.Equal(t, 4, configResult.ServersGenerated, "Should generate 4 server configurations")
		assert.True(t, configResult.AutoDetected, "Config should be auto-detected")

		validationResult, err := mockConfigGenerator.ValidateGenerated(configResult.Config)
		require.NoError(t, err, "Config validation should succeed")
		assert.True(t, validationResult.Valid, "Generated config should be valid")
		assert.Equal(t, 4, validationResult.ServersValidated, "Should validate 4 servers")
	})

	t.Run("VerifySetupResults", func(t *testing.T) {
		for _, runtime := range []string{"go", "python", "nodejs", "java"} {
			verifyResult, err := mockRuntimeInstaller.Verify(runtime)
			require.NoError(t, err, "Runtime verification should succeed for "+runtime)
			assert.True(t, verifyResult.Installed, "Runtime should be installed for "+runtime)
			assert.True(t, verifyResult.Compatible, "Runtime should be compatible for "+runtime)
		}

		for _, server := range []string{"gopls", "pylsp", "typescript-language-server", "jdtls"} {
			verifyResult, err := mockServerInstaller.Verify(server)
			require.NoError(t, err, "Server verification should succeed for "+server)
			assert.True(t, verifyResult.Installed, "Server should be installed for "+server)
			assert.True(t, verifyResult.Compatible, "Server should be compatible for "+server)
		}

		assert.Len(t, mockRuntimeInstaller.VerifyCalls, 4, "Should verify 4 runtimes")
		assert.Len(t, mockServerInstaller.VerifyCalls, 4, "Should verify 4 servers")
	})
}

func TestSetupCommand_ErrorScenarios(t *testing.T) {
	testRunner := testdata.NewTestRunner(time.Second * 30)
	defer testRunner.Cleanup()

	t.Run("InstallationFailure", func(t *testing.T) {
		mockInstaller := mocks.NewMockRuntimeInstaller()

		mockInstaller.InstallFunc = func(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
			return testdata.CreateMockInstallResult(runtime, "", "", false), nil
		}

		installOptions := testdata.CreateMockInstallOptions("1.24.0", false)
		result, err := mockInstaller.Install("go", installOptions)

		require.NoError(t, err, "Install call should not return an error")
		assert.False(t, result.Success, "Installation should fail")
		assert.Len(t, result.Errors, 1, "Should have one error")
	})

	t.Run("VerificationFailure", func(t *testing.T) {
		mockInstaller := mocks.NewMockRuntimeInstaller()

		mockInstaller.VerifyFunc = func(runtime string) (*types.VerificationResult, error) {
			return testdata.CreateMockVerificationResult(runtime, "", "", false, false), nil
		}

		result, err := mockInstaller.Verify("python")
		require.NoError(t, err, "Verify call should not return an error")
		assert.False(t, result.Installed, "Runtime should not be installed")
		assert.False(t, result.Compatible, "Runtime should not be compatible")
		assert.Len(t, result.Issues, 2, "Should have installation and compatibility issues")
	})

	t.Run("ConfigValidationFailure", func(t *testing.T) {
		mockGenerator := mocks.NewMockConfigGenerator()

		invalidConfig := testdata.CreateMockGatewayConfig(0, []config.ServerConfig{})
		result, err := mockGenerator.ValidateConfig(invalidConfig)

		require.NoError(t, err, "ValidateConfig call should not return an error")
		assert.False(t, result.Valid, "Config should be invalid")
		assert.Len(t, result.Issues, 1, "Should have port validation issue")
		assert.Len(t, result.Warnings, 1, "Should have no servers warning")
	})
}

func TestMockReset_Functionality(t *testing.T) {
	mockDetector := mocks.NewMockRuntimeDetector()
	mockInstaller := mocks.NewMockRuntimeInstaller()
	mockServerInstaller := mocks.NewMockServerInstaller()
	mockGenerator := mocks.NewMockConfigGenerator()

	ctx := testdata.NewTestContext(time.Second * 5)
	defer ctx.Cleanup()

	_, _ = mockDetector.DetectGo(ctx.Context)
	_, _ = mockInstaller.Install("go", testdata.CreateMockInstallOptions("1.24.0", false))
	_, _ = mockServerInstaller.Install("gopls", testdata.CreateMockServerInstallOptions("1.0.0", false))
	_, _ = mockGenerator.GenerateDefault()

	assert.Len(t, mockDetector.DetectGoCalls, 1, "Should have one DetectGo call before reset")
	assert.Len(t, mockInstaller.InstallCalls, 1, "Should have one Install call before reset")
	assert.Len(t, mockServerInstaller.InstallCalls, 1, "Should have one Server Install call before reset")
	assert.Equal(t, 1, mockGenerator.GenerateDefaultCalls, "Should have one GenerateDefault call before reset")

	mockDetector.Reset()
	mockInstaller.Reset()
	mockServerInstaller.Reset()
	mockGenerator.Reset()

	assert.Len(t, mockDetector.DetectGoCalls, 0, "Should have no DetectGo calls after reset")
	assert.Len(t, mockInstaller.InstallCalls, 0, "Should have no Install calls after reset")
	assert.Len(t, mockServerInstaller.InstallCalls, 0, "Should have no Server Install calls after reset")
	assert.Equal(t, 0, mockGenerator.GenerateDefaultCalls, "Should have no GenerateDefault calls after reset")
}

// TestSetupCommand_Structure tests the command structure and registration
func TestSetupCommand_Structure(t *testing.T) {
	t.Run("ParentCommand", func(t *testing.T) {
		setupCmd := cli.GetSetupCmd()
		require.NotNil(t, setupCmd, "Setup command should not be nil")
		assert.Equal(t, "setup", setupCmd.Use, "Command use should be 'setup'")
		assert.Contains(t, setupCmd.Short, "Setup LSP Gateway", "Command short description should mention setup")
		assert.Contains(t, setupCmd.Long, "automated configuration", "Command long description should mention automation")
	})

	t.Run("SubCommands", func(t *testing.T) {
		setupCmd := cli.GetSetupCmd()
		subCommands := setupCmd.Commands()
		require.Len(t, subCommands, 2, "Setup should have 2 subcommands")

		// Find all and wizard commands
		var allCmd, wizardCmd *cobra.Command
		for _, cmd := range subCommands {
			switch cmd.Use {
			case "all":
				allCmd = cmd
			case "wizard":
				wizardCmd = cmd
			}
		}

		require.NotNil(t, allCmd, "'all' subcommand should exist")
		require.NotNil(t, wizardCmd, "'wizard' subcommand should exist")

		// Test all command
		assert.Equal(t, "all", allCmd.Use, "All command use should be 'all'")
		assert.Contains(t, allCmd.Short, "automated setup", "All command should mention automation")
		assert.Contains(t, allCmd.Long, "DETECTION PHASE", "All command should describe phases")

		// Test wizard command
		assert.Equal(t, "wizard", wizardCmd.Use, "Wizard command use should be 'wizard'")
		assert.Contains(t, wizardCmd.Short, "Interactive setup", "Wizard command should mention interactivity")
		assert.Contains(t, wizardCmd.Long, "WIZARD FEATURES", "Wizard command should describe features")
	})

	t.Run("ExportedHelpers", func(t *testing.T) {
		// Test exported command getters
		setupCmd := cli.GetSetupCmd()
		allCmd := cli.GetSetupAllCmd()
		wizardCmd := cli.GetSetupWizardCmd()

		assert.NotNil(t, setupCmd, "GetSetupCmd should return non-nil")
		assert.NotNil(t, allCmd, "GetSetupAllCmd should return non-nil")
		assert.NotNil(t, wizardCmd, "GetSetupWizardCmd should return non-nil")

		// Test flag getters and setters
		originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify := cli.GetSetupFlags()

		// Set new values
		cli.SetSetupFlags(30*time.Minute, true, true, true, true, true)
		newTimeout, newForce, newJSON, newVerbose, newNoInteractive, newSkipVerify := cli.GetSetupFlags()

		assert.Equal(t, 30*time.Minute, newTimeout, "Timeout should be updated")
		assert.True(t, newForce, "Force should be updated")
		assert.True(t, newJSON, "JSON should be updated")
		assert.True(t, newVerbose, "Verbose should be updated")
		assert.True(t, newNoInteractive, "NoInteractive should be updated")
		assert.True(t, newSkipVerify, "SkipVerify should be updated")

		// Restore original values
		cli.SetSetupFlags(originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify)
	})
}

// TestSetupCommand_Flags tests all CLI flags and their validation
func TestSetupCommand_Flags(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        []string
		expectedErr bool
		description string
	}{
		{
			name:        "AllCommand_ValidFlags",
			command:     "all",
			args:        []string{"--timeout", "20m", "--force", "--json", "--verbose", "--skip-verify", "--config", "test.yaml"},
			expectedErr: false,
			description: "All command with all valid flags should parse successfully",
		},
		{
			name:        "WizardCommand_ValidFlags",
			command:     "wizard",
			args:        []string{"--timeout", "45m", "--force", "--json", "--verbose", "--no-interactive", "--config", "wizard.yaml"},
			expectedErr: false,
			description: "Wizard command with all valid flags should parse successfully",
		},
		{
			name:        "AllCommand_ShortFlags",
			command:     "all",
			args:        []string{"-f", "-v"},
			expectedErr: false,
			description: "Short flags should be accepted",
		},
		{
			name:        "AllCommand_InvalidTimeout",
			command:     "all",
			args:        []string{"--timeout", "invalid"},
			expectedErr: true,
			description: "Invalid timeout format should cause error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cmd *cobra.Command
			switch tt.command {
			case "all":
				cmd = cli.GetSetupAllCmd()
			case "wizard":
				cmd = cli.GetSetupWizardCmd()
			}

			require.NotNil(t, cmd, "Command should not be nil")

			// Parse flags
			cmd.SetArgs(tt.args)
			err := cmd.ParseFlags(tt.args)

			if tt.expectedErr {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestSetupCommand_FlagDefaults tests default flag values
func TestSetupCommand_FlagDefaults(t *testing.T) {
	t.Run("AllCommand_Defaults", func(t *testing.T) {
		allCmd := cli.GetSetupAllCmd()
		require.NotNil(t, allCmd, "All command should not be nil")

		// Test flag defaults
		timeoutFlag := allCmd.Flag("timeout")
		require.NotNil(t, timeoutFlag, "Timeout flag should exist")
		assert.Equal(t, "15m0s", timeoutFlag.DefValue, "Default timeout should be 15 minutes")

		forceFlag := allCmd.Flag("force")
		require.NotNil(t, forceFlag, "Force flag should exist")
		assert.Equal(t, "false", forceFlag.DefValue, "Default force should be false")

		jsonFlag := allCmd.Flag("json")
		require.NotNil(t, jsonFlag, "JSON flag should exist")
		assert.Equal(t, "false", jsonFlag.DefValue, "Default JSON should be false")

		verboseFlag := allCmd.Flag("verbose")
		require.NotNil(t, verboseFlag, "Verbose flag should exist")
		assert.Equal(t, "false", verboseFlag.DefValue, "Default verbose should be false")

		skipVerifyFlag := allCmd.Flag("skip-verify")
		require.NotNil(t, skipVerifyFlag, "Skip-verify flag should exist")
		assert.Equal(t, "false", skipVerifyFlag.DefValue, "Default skip-verify should be false")

		configFlag := allCmd.Flag("config")
		require.NotNil(t, configFlag, "Config flag should exist")
		assert.Equal(t, "config.yaml", configFlag.DefValue, "Default config should be config.yaml")
	})

	t.Run("WizardCommand_Defaults", func(t *testing.T) {
		wizardCmd := cli.GetSetupWizardCmd()
		require.NotNil(t, wizardCmd, "Wizard command should not be nil")

		// Test flag defaults
		timeoutFlag := wizardCmd.Flag("timeout")
		require.NotNil(t, timeoutFlag, "Timeout flag should exist")
		assert.Equal(t, "30m0s", timeoutFlag.DefValue, "Default timeout should be 30 minutes")

		noInteractiveFlag := wizardCmd.Flag("no-interactive")
		require.NotNil(t, noInteractiveFlag, "No-interactive flag should exist")
		assert.Equal(t, "false", noInteractiveFlag.DefValue, "Default no-interactive should be false")
	})
}

// TestSetupCommand_HelpText tests help text and usage information
func TestSetupCommand_HelpText(t *testing.T) {
	t.Run("SetupCommand_Help", func(t *testing.T) {
		setupCmd := cli.GetSetupCmd()
		require.NotNil(t, setupCmd, "Setup command should not be nil")

		// Capture help output
		buf := new(bytes.Buffer)
		setupCmd.SetOut(buf)
		setupCmd.SetArgs([]string{"--help"})

		err := setupCmd.Execute()
		assert.NoError(t, err, "Help command should execute without error")

		helpOutput := buf.String()
		assert.Contains(t, helpOutput, "Setup LSP Gateway", "Help should contain command description")
		assert.Contains(t, helpOutput, "Available Commands:", "Help should list available commands")
		assert.Contains(t, helpOutput, "all", "Help should mention 'all' subcommand")
		assert.Contains(t, helpOutput, "wizard", "Help should mention 'wizard' subcommand")
	})

	t.Run("AllCommand_Help", func(t *testing.T) {
		allCmd := cli.GetSetupAllCmd()
		require.NotNil(t, allCmd, "All command should not be nil")

		// Capture help output
		buf := new(bytes.Buffer)
		allCmd.SetOut(buf)
		allCmd.SetArgs([]string{"--help"})

		err := allCmd.Execute()
		assert.NoError(t, err, "Help command should execute without error")

		helpOutput := buf.String()
		assert.Contains(t, helpOutput, "DETECTION PHASE", "Help should describe detection phase")
		assert.Contains(t, helpOutput, "INSTALLATION PHASE", "Help should describe installation phase")
		assert.Contains(t, helpOutput, "CONFIGURATION PHASE", "Help should describe configuration phase")
		assert.Contains(t, helpOutput, "VERIFICATION PHASE", "Help should describe verification phase")
		assert.Contains(t, helpOutput, "--timeout duration", "Help should show timeout flag")
		assert.Contains(t, helpOutput, "--force", "Help should show force flag")
	})

	t.Run("WizardCommand_Help", func(t *testing.T) {
		wizardCmd := cli.GetSetupWizardCmd()
		require.NotNil(t, wizardCmd, "Wizard command should not be nil")

		// Capture help output
		buf := new(bytes.Buffer)
		wizardCmd.SetOut(buf)
		wizardCmd.SetArgs([]string{"--help"})

		err := wizardCmd.Execute()
		assert.NoError(t, err, "Help command should execute without error")

		helpOutput := buf.String()
		assert.Contains(t, helpOutput, "WIZARD FEATURES", "Help should describe wizard features")
		assert.Contains(t, helpOutput, "WIZARD STEPS", "Help should describe wizard steps")
		assert.Contains(t, helpOutput, "CUSTOMIZATION OPTIONS", "Help should describe customization options")
		assert.Contains(t, helpOutput, "--no-interactive", "Help should show no-interactive flag")
	})
}
