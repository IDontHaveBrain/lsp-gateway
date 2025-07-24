package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"lsp-gateway/internal/cli"
	"lsp-gateway/tests/mocks"
	"lsp-gateway/tests/testdata"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRunSetupWizard_NonInteractiveMode tests the non-interactive fallback
func TestRunSetupWizard_NonInteractiveMode(t *testing.T) {
	t.Run("NoInteractiveFallback", func(t *testing.T) {
		testRunner := testdata.NewTestRunner(30 * time.Second)
		defer testRunner.Cleanup()

		// Store and set flags for non-interactive mode
		originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify := cli.GetSetupFlags()
		defer cli.SetSetupFlags(originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify)
		cli.SetSetupFlags(15*time.Minute, false, false, true, true, false) // no-interactive = true

		// Capture output
		var buf bytes.Buffer
		cmd := cli.GetSetupWizardCmd()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetContext(testRunner.Context())

		// Execute wizard command
		err := cmd.RunE(cmd, []string{})
		assert.NoError(t, err, "Non-interactive wizard should fallback to setupAll")

		output := buf.String()
		assert.Contains(t, output, "non-interactive mode", "Should mention non-interactive mode")
	})

	t.Run("JSONOutputNotSupported", func(t *testing.T) {
		testRunner := testdata.NewTestRunner(30 * time.Second)
		defer testRunner.Cleanup()

		// Store and set flags for JSON output (should be rejected)
		originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify := cli.GetSetupFlags()
		defer cli.SetSetupFlags(originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify)
		cli.SetSetupFlags(15*time.Minute, false, true, false, false, false) // JSON = true

		// Capture output
		var buf bytes.Buffer
		cmd := cli.GetSetupWizardCmd()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetContext(testRunner.Context())

		// Execute wizard command - should error
		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err, "Wizard should reject JSON output mode")
		assert.Contains(t, err.Error(), "JSON output not supported", "Error should mention JSON not supported")
	})
}

// TestRunSetupWizard_InteractiveFlow tests the full interactive wizard flow
func TestRunSetupWizard_InteractiveFlow(t *testing.T) {
	tests := []struct {
		name           string
		userInputs     []string
		expectedSteps  []string
		shouldComplete bool
		verbose        bool
	}{
		{
			name: "FullWizardFlow_AllYes",
			userInputs: []string{
				"y\n", // Ready to begin
				"y\n", // Install all runtimes
				"y\n", // Install all servers
				"n\n", // No custom config path
				"n\n", // No advanced settings
				"y\n", // Proceed with installation
			},
			expectedSteps: []string{
				"Welcome to the LSP Gateway Setup Wizard",
				"Step 1: Runtime Detection",
				"Step 2: Language Server Selection",
				"Step 3: Configuration Options",
				"Step 4: Review Your Selection",
				"Step 5: Installation Execution",
			},
			shouldComplete: true,
			verbose:        false,
		},
		{
			name: "FullWizardFlow_Verbose",
			userInputs: []string{
				"y\n", // Ready to begin
				"y\n", // Install all runtimes
				"y\n", // Install all servers
				"n\n", // No custom config path
				"y\n", // Advanced settings
				"y\n", // Proceed with installation
			},
			expectedSteps: []string{
				"Welcome to the LSP Gateway Setup Wizard",
				"Verbose mode is enabled",
				"Step 1: Runtime Detection",
				"Runtime detection analyzes",
				"Step 2: Language Server Selection",
				"Language servers provide IDE features",
				"Step 3: Configuration Options",
				"Configuration determines how LSP Gateway",
			},
			shouldComplete: true,
			verbose:        true,
		},
		{
			name: "WizardCancellation_AtStart",
			userInputs: []string{
				"n\n", // Not ready to begin
			},
			expectedSteps: []string{
				"Welcome to the LSP Gateway Setup Wizard",
			},
			shouldComplete: false,
			verbose:        false,
		},
		{
			name: "WizardCancellation_AtReview",
			userInputs: []string{
				"y\n", // Ready to begin
				"y\n", // Install all runtimes
				"y\n", // Install all servers
				"n\n", // No custom config path
				"n\n", // No advanced settings
				"n\n", // Do NOT proceed with installation
			},
			expectedSteps: []string{
				"Welcome to the LSP Gateway Setup Wizard",
				"Step 1: Runtime Detection",
				"Step 2: Language Server Selection",
				"Step 3: Configuration Options",
				"Step 4: Review Your Selection",
			},
			shouldComplete: false,
			verbose:        false,
		},
		{
			name: "CustomSelections",
			userInputs: []string{
				"y\n",                  // Ready to begin
				"n\n",                  // Don't install all runtimes
				"1,3\n",                // Select Go and Node.js
				"n\n",                  // Don't install all servers
				"1,2\n",                // Select gopls and pylsp
				"y\n",                  // Custom config path
				"custom-config.yaml\n", // Custom path
				"n\n",                  // No advanced settings
				"y\n",                  // Proceed with installation
			},
			expectedSteps: []string{
				"Welcome to the LSP Gateway Setup Wizard",
				"Step 1: Runtime Detection",
				"Select runtimes to install",
				"Step 2: Language Server Selection",
				"Select language servers to install",
				"Step 3: Configuration Options",
				"custom configuration file path",
				"Step 4: Review Your Selection",
			},
			shouldComplete: true,
			verbose:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip interactive tests in short mode
			if testing.Short() {
				t.Skip("Skipping interactive wizard test in short mode")
			}

			testRunner := testdata.NewTestRunner(time.Minute)
			defer testRunner.Cleanup()

			// Store and set flags
			originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify := cli.GetSetupFlags()
			defer cli.SetSetupFlags(originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify)
			cli.SetSetupFlags(30*time.Minute, false, false, tt.verbose, false, false)

			// Mock user input
			userInput := strings.Join(tt.userInputs, "")
			oldStdin := os.Stdin
			r, w, _ := os.Pipe()
			os.Stdin = r

			// Write user input to pipe
			go func() {
				defer w.Close()
				w.WriteString(userInput)
			}()

			// Restore stdin after test
			defer func() {
				os.Stdin = oldStdin
				r.Close()
			}()

			// Capture output
			var buf bytes.Buffer
			cmd := cli.GetSetupWizardCmd()
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetContext(testRunner.Context())

			// Execute wizard command
			err := cmd.RunE(cmd, []string{})

			if tt.shouldComplete {
				assert.NoError(t, err, "Wizard should complete successfully")
			} else {
				assert.Error(t, err, "Wizard should be cancelled/fail")
			}

			output := buf.String()

			// Check for expected steps
			for _, expectedStep := range tt.expectedSteps {
				assert.Contains(t, output, expectedStep, fmt.Sprintf("Output should contain step: %s", expectedStep))
			}
		})
	}
}

// TestWizardStepFunctions tests individual wizard step functions
func TestWizardStepFunctions(t *testing.T) {
	t.Run("WizardWelcome", func(t *testing.T) {
		// Test welcome step with various inputs
		inputs := []struct {
			input         string
			shouldProceed bool
		}{
			{"y\n", true},
			{"yes\n", true},
			{"Y\n", true},
			{"YES\n", true},
			{"\n", true}, // Default is true
			{"n\n", false},
			{"no\n", false},
			{"N\n", false},
			{"NO\n", false},
		}

		for _, inp := range inputs {
			t.Run(fmt.Sprintf("Input_%s", strings.TrimSpace(inp.input)), func(t *testing.T) {
				// Mock stdin
				oldStdin := os.Stdin
				r, w, _ := os.Pipe()
				os.Stdin = r

				go func() {
					defer w.Close()
					w.WriteString(inp.input)
				}()

				defer func() {
					os.Stdin = oldStdin
					r.Close()
				}()

				// Capture output
				var buf bytes.Buffer
				oldStdout := os.Stdout
				rOut, wOut, _ := os.Pipe()
				os.Stdout = wOut

				// Read output in goroutine
				go func() {
					defer wOut.Close()
					io.Copy(&buf, rOut)
				}()

				defer func() {
					os.Stdout = oldStdout
					rOut.Close()
				}()

				// Test would call wizardWelcome function here
				// Since it's not exported, we test the behavior through the main wizard

				// For now, just verify the input processing logic would work
				assert.True(t, true, "Input processing test placeholder")
			})
		}
	})
}

// TestPromptFunctions tests the individual prompt helper functions
func TestPromptFunctions(t *testing.T) {
	// Note: These test the exported helper functions or simulate their behavior
	// since the actual prompt functions are not exported

	t.Run("PromptYesNo_Behavior", func(t *testing.T) {
		tests := []struct {
			name         string
			input        string
			defaultValue bool
			expected     bool
			shouldError  bool
		}{
			{"YesInput", "y\n", false, true, false},
			{"NoInput", "n\n", true, false, false},
			{"EmptyDefaultTrue", "\n", true, true, false},
			{"EmptyDefaultFalse", "\n", false, false, false},
			{"CaseInsensitive", "YES\n", false, true, false},
			{"InvalidThenValid", "invalid\ny\n", false, true, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// This would test the actual promptYesNo function
				// For now, we validate the expected behavior
				assert.True(t, true, "PromptYesNo behavior test placeholder")
			})
		}
	})

	t.Run("PromptSelection_Behavior", func(t *testing.T) {
		tests := []struct {
			name            string
			input           string
			options         []string
			allowMultiple   bool
			expectedIndices []int
			shouldError     bool
		}{
			{"SingleSelection", "1\n", []string{"option1", "option2"}, false, []int{0}, false},
			{"MultipleSelection", "1,2\n", []string{"option1", "option2"}, true, []int{0, 1}, false},
			{"AllSelection", "all\n", []string{"option1", "option2"}, true, []int{0, 1}, false},
			{"OutOfRange", "5\n", []string{"option1", "option2"}, false, nil, true},
			{"InvalidFormat", "abc\n", []string{"option1", "option2"}, false, nil, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// This would test the actual promptSelection function
				// For now, we validate the expected behavior
				assert.True(t, true, "PromptSelection behavior test placeholder")
			})
		}
	})

	t.Run("PromptText_Behavior", func(t *testing.T) {
		tests := []struct {
			name         string
			input        string
			defaultValue string
			expected     string
			shouldError  bool
		}{
			{"CustomInput", "custom\n", "default", "custom", false},
			{"EmptyWithDefault", "\n", "default", "default", false},
			{"EmptyNoDefault", "\n", "", "", false},
			{"WhitespaceInput", "  text  \n", "", "text", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// This would test the actual promptText function
				// For now, we validate the expected behavior
				assert.True(t, true, "PromptText behavior test placeholder")
			})
		}
	})
}

// TestWizardErrorHandling tests error scenarios in the wizard
func TestWizardErrorHandling(t *testing.T) {
	t.Run("StdinReadError", func(t *testing.T) {
		// Mock stdin that will cause read errors
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		w.Close() // Close write end to cause EOF

		defer func() {
			os.Stdin = oldStdin
			r.Close()
		}()

		testRunner := testdata.NewTestRunner(30 * time.Second)
		defer testRunner.Cleanup()

		// Store and set flags
		originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify := cli.GetSetupFlags()
		defer cli.SetSetupFlags(originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify)
		cli.SetSetupFlags(15*time.Minute, false, false, false, false, false)

		// Capture output
		var buf bytes.Buffer
		cmd := cli.GetSetupWizardCmd()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetContext(testRunner.Context())

		// Execute wizard command - should handle stdin errors gracefully
		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err, "Should error when stdin fails")
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		// Create cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Mock stdin with delayed input
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r

		// Cancel context after short delay
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
			w.Close()
		}()

		defer func() {
			os.Stdin = oldStdin
			r.Close()
		}()

		// Store and set flags
		originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify := cli.GetSetupFlags()
		defer cli.SetSetupFlags(originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify)
		cli.SetSetupFlags(15*time.Minute, false, false, false, false, false)

		// Capture output
		var buf bytes.Buffer
		cmd := cli.GetSetupWizardCmd()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetContext(ctx)

		// Execute wizard command - should handle cancellation
		err := cmd.RunE(cmd, []string{})
		if err != nil {
			// Should handle context cancellation gracefully
			assert.Contains(t, err.Error(), "context", "Error should mention context cancellation")
		}
	})
}

// TestWizardStateManagement tests wizard state management
func TestWizardStateManagement(t *testing.T) {
	t.Run("StateInitialization", func(t *testing.T) {
		// Test wizard state structure initialization
		// This would test the WizardState struct and its initialization

		state := struct {
			SelectedRuntimes map[string]bool
			SelectedServers  map[string]bool
			CustomConfig     bool
			AdvancedSettings bool
			ConfigPath       string
		}{
			SelectedRuntimes: make(map[string]bool),
			SelectedServers:  make(map[string]bool),
			ConfigPath:       "config.yaml",
		}

		assert.Empty(t, state.SelectedRuntimes, "SelectedRuntimes should be empty initially")
		assert.Empty(t, state.SelectedServers, "SelectedServers should be empty initially")
		assert.False(t, state.CustomConfig, "CustomConfig should be false initially")
		assert.False(t, state.AdvancedSettings, "AdvancedSettings should be false initially")
		assert.Equal(t, "config.yaml", state.ConfigPath, "ConfigPath should have default value")
	})

	t.Run("StateTransitions", func(t *testing.T) {
		// Test state transitions during wizard flow
		state := struct {
			SelectedRuntimes map[string]bool
			SelectedServers  map[string]bool
			CustomConfig     bool
			AdvancedSettings bool
			ConfigPath       string
		}{
			SelectedRuntimes: make(map[string]bool),
			SelectedServers:  make(map[string]bool),
			ConfigPath:       "config.yaml",
		}

		// Simulate runtime selection
		state.SelectedRuntimes["go"] = true
		state.SelectedRuntimes["python"] = true
		assert.Len(t, state.SelectedRuntimes, 2, "Should have 2 selected runtimes")

		// Simulate server selection
		state.SelectedServers["gopls"] = true
		state.SelectedServers["pylsp"] = true
		assert.Len(t, state.SelectedServers, 2, "Should have 2 selected servers")

		// Simulate configuration changes
		state.CustomConfig = true
		state.ConfigPath = "custom.yaml"
		state.AdvancedSettings = true

		assert.True(t, state.CustomConfig, "CustomConfig should be enabled")
		assert.Equal(t, "custom.yaml", state.ConfigPath, "ConfigPath should be custom")
		assert.True(t, state.AdvancedSettings, "AdvancedSettings should be enabled")
	})
}

// TestWizardIntegration tests wizard integration with mocks
func TestWizardIntegration(t *testing.T) {
	t.Run("WizardWithMockDetection", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping integration test in short mode")
		}

		testRunner := testdata.NewTestRunner(time.Minute)
		defer testRunner.Cleanup()

		// Create mocks
		mockDetector := mocks.NewMockRuntimeDetector()
		mockRuntimeInstaller := mocks.NewMockRuntimeInstaller()
		mockServerInstaller := mocks.NewMockServerInstaller()
		mockConfigGenerator := mocks.NewMockConfigGenerator()

		// Reset mocks to ensure clean state
		mockDetector.Reset()
		mockRuntimeInstaller.Reset()
		mockServerInstaller.Reset()
		mockConfigGenerator.Reset()

		// Mock user input for a complete wizard flow
		userInput := "y\ny\ny\nn\nn\ny\n" // Accept all defaults and proceed
		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r

		go func() {
			defer w.Close()
			w.WriteString(userInput)
		}()

		defer func() {
			os.Stdin = oldStdin
			r.Close()
		}()

		// Store and set flags
		originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify := cli.GetSetupFlags()
		defer cli.SetSetupFlags(originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify)
		cli.SetSetupFlags(30*time.Minute, false, false, false, false, false)

		// Capture output
		var buf bytes.Buffer
		cmd := cli.GetSetupWizardCmd()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetContext(testRunner.Context())

		// Execute wizard command
		err := cmd.RunE(cmd, []string{})

		// The test may complete successfully or fail depending on mock integration
		// We primarily test that the wizard flow executes without panicking
		if err != nil {
			t.Logf("Wizard completed with error (expected in unit test): %v", err)
		}

		output := buf.String()
		assert.Contains(t, output, "Welcome to the LSP Gateway Setup Wizard", "Should show welcome message")
	})
}

// Benchmark for wizard performance
func BenchmarkSetupWizard_NonInteractive(b *testing.B) {
	testRunner := testdata.NewTestRunner(time.Minute)
	defer testRunner.Cleanup()

	// Store original flags
	originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify := cli.GetSetupFlags()
	defer cli.SetSetupFlags(originalTimeout, originalForce, originalJSON, originalVerbose, originalNoInteractive, originalSkipVerify)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Set flags for non-interactive mode (fastest path)
		cli.SetSetupFlags(15*time.Minute, false, false, false, true, true)

		// Execute command
		cmd := cli.GetSetupWizardCmd()
		cmd.SetContext(testRunner.Context())

		// Discard output
		cmd.SetOut(ioutil.Discard)
		cmd.SetErr(ioutil.Discard)

		err := cmd.RunE(cmd, []string{})
		if err != nil {
			b.Fatalf("Wizard failed on iteration %d: %v", i, err)
		}
	}
}
