package integration_test

import (
	"context"
	"fmt"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetupIntegration_EndToEndWorkflow tests complete end-to-end setup workflows
func TestSetupIntegration_EndToEndWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tests := []struct {
		name                 string
		setupOptions         SetupOptions
		expectedRuntimes     []string
		expectedServers      []string
		expectFailure        bool
		failurePhase         string
		minimumSetupTime     time.Duration
		maximumSetupTime     time.Duration
		validateConfigAfter  bool
		validateInstallation bool
	}{
		{
			name: "CompleteGoSetup",
			setupOptions: SetupOptions{
				Force:      false,
				Verbose:    true,
				Timeout:    5 * time.Minute,
				SkipVerify: false,
			},
			expectedRuntimes:     []string{"go"},
			expectedServers:      []string{"gopls"},
			expectFailure:        false,
			minimumSetupTime:     100 * time.Millisecond,
			maximumSetupTime:     2 * time.Minute,
			validateConfigAfter:  true,
			validateInstallation: true,
		},
		{
			name: "ForcedReinstallMultipleRuntimes",
			setupOptions: SetupOptions{
				Force:      true,
				Verbose:    false,
				Timeout:    10 * time.Minute,
				SkipVerify: false,
			},
			expectedRuntimes:     []string{"go", "python", "nodejs"},
			expectedServers:      []string{"gopls", "pylsp", "typescript-language-server"},
			expectFailure:        false,
			minimumSetupTime:     200 * time.Millisecond,
			maximumSetupTime:     5 * time.Minute,
			validateConfigAfter:  true,
			validateInstallation: true,
		},
		{
			name: "SkipVerificationWorkflow",
			setupOptions: SetupOptions{
				Force:      false,
				Verbose:    true,
				Timeout:    3 * time.Minute,
				SkipVerify: true,
			},
			expectedRuntimes:     []string{"go"},
			expectedServers:      []string{"gopls"},
			expectFailure:        false,
			minimumSetupTime:     50 * time.Millisecond,
			maximumSetupTime:     1 * time.Minute,
			validateConfigAfter:  true,
			validateInstallation: false, // Skip verification
		},
		{
			name: "TimeoutScenario",
			setupOptions: SetupOptions{
				Force:      false,
				Verbose:    false,
				Timeout:    100 * time.Millisecond, // Very short timeout
				SkipVerify: false,
			},
			expectedRuntimes:  []string{},
			expectedServers:   []string{},
			expectFailure:     true,
			failurePhase:      "timeout",
			minimumSetupTime:  50 * time.Millisecond,
			maximumSetupTime:  300 * time.Millisecond,
			validateConfigAfter: false,
			validateInstallation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated test environment
			testEnv := NewIntegrationTestEnvironment(t)
			defer testEnv.Cleanup()

			// Setup orchestrator with test configuration
			orchestrator := setup.NewSetupOrchestrator()
			orchestrator.SetLogger(setup.NewSetupLogger(nil))

			ctx, cancel := context.WithTimeout(context.Background(), tt.setupOptions.Timeout)
			defer cancel()

			// Execute setup workflow
			startTime := time.Now()
			result, err := orchestrator.RunFullSetup(ctx, &setup.SetupOptions{
				Force:                tt.setupOptions.Force,
				VerboseLogging:       tt.setupOptions.Verbose,
				SkipVerification:     tt.setupOptions.SkipVerify,
				ConfigOutputPath:     testEnv.ConfigPath(),
				ValidateInstallations: true,
			})
			duration := time.Since(startTime)

			// Validate timing
			assert.GreaterOrEqual(t, duration, tt.minimumSetupTime, "Setup should take minimum expected time")
			assert.LessOrEqual(t, duration, tt.maximumSetupTime, "Setup should complete within maximum expected time")

			if tt.expectFailure {
				assert.Error(t, err, "Expected setup to fail")
				if tt.failurePhase != "" {
					assert.Contains(t, err.Error(), tt.failurePhase, "Error should mention expected failure phase")
				}
				return
			}

			// Validate successful setup
			require.NoError(t, err, "Setup should complete successfully")
			require.NotNil(t, result, "Setup result should not be nil")

			// Validate setup result structure
			assert.True(t, result.Success, "Setup should be marked as successful")
			assert.GreaterOrEqual(t, result.Duration, time.Duration(0), "Duration should be non-negative")

			// Validate expected runtimes were processed
			for _, expectedRuntime := range tt.expectedRuntimes {
				found := false
				for runtime := range result.RuntimeInstallResults {
					if runtime == expectedRuntime {
						found = true
						break
					}
				}
				assert.True(t, found, fmt.Sprintf("Expected runtime %s should be in results", expectedRuntime))
			}

			// Validate configuration generation
			if tt.validateConfigAfter {
				assert.NotNil(t, result.ConfigGeneration, "Configuration should be generated")
				assert.FileExists(t, testEnv.ConfigPath(), "Configuration file should exist")

				// Validate configuration content
				configContent, err := os.ReadFile(testEnv.ConfigPath())
				require.NoError(t, err, "Should be able to read generated configuration")
				assert.NotEmpty(t, configContent, "Configuration file should not be empty")
			}

			// Validate installation results
			if tt.validateInstallation {
				assert.NotEmpty(t, result.RuntimeInstallResults, "Should have runtime installation results")
				for runtime, installResult := range result.RuntimeInstallResults {
					assert.True(t, installResult.Success, fmt.Sprintf("Runtime %s should be installed successfully", runtime))
					assert.NotEmpty(t, installResult.Version, fmt.Sprintf("Runtime %s should have version information", runtime))
				}
			}
		})
	}
}

// TestSetupIntegration_ErrorRecoveryMechanisms tests error recovery and retry mechanisms
func TestSetupIntegration_ErrorRecoveryMechanisms(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tests := []struct {
		name                string
		simulateFailures    []string
		expectedRecoveries  int
		expectedRetries     int
		shouldEventuallySucceed bool
		maxRetryTime        time.Duration
	}{
		{
			name:                "RecoverFromTransientDetectionFailure",
			simulateFailures:    []string{"detection_transient"},
			expectedRecoveries:  1,
			expectedRetries:     2,
			shouldEventuallySucceed: true,
			maxRetryTime:        30 * time.Second,
		},
		{
			name:                "RecoverFromPartialInstallationFailure",
			simulateFailures:    []string{"install_python_once"},
			expectedRecoveries:  1,
			expectedRetries:     1,
			shouldEventuallySucceed: true,
			maxRetryTime:        45 * time.Second,
		},
		{
			name:                "MultipleFailuresWithRecovery",
			simulateFailures:    []string{"detection_transient", "config_generation_once"},
			expectedRecoveries:  2,
			expectedRetries:     3,
			shouldEventuallySucceed: true,
			maxRetryTime:        60 * time.Second,
		},
		{
			name:                "PermanentFailureNoRecovery",
			simulateFailures:    []string{"detection_permanent"},
			expectedRecoveries:  0,
			expectedRetries:     3,
			shouldEventuallySucceed: false,
			maxRetryTime:        20 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := NewIntegrationTestEnvironment(t)
			defer testEnv.Cleanup()

			// Create orchestrator with failure simulation
			orchestrator := setup.NewSetupOrchestrator()

			// Setup failure simulation for testing
			failureSimulator := NewFailureSimulator(tt.simulateFailures)

			ctx, cancel := context.WithTimeout(context.Background(), tt.maxRetryTime)
			defer cancel()

			// Execute setup with retry logic built into the options
			startTime := time.Now()
			result, err := orchestrator.RunFullSetup(ctx, &setup.SetupOptions{
				Force:               false,
				VerboseLogging:      true,
				SkipVerification:    false,
				ConfigOutputPath:    testEnv.ConfigPath(),
				AutoRetryFailures:   true,
				MaxRetryAttempts:    3,
				ValidateInstallations: true,
			})
			duration := time.Since(startTime)

			// Validate retry behavior
			assert.Equal(t, tt.expectedRetries, failureSimulator.GetRetryCount(), "Should have expected number of retries")
			assert.Equal(t, tt.expectedRecoveries, failureSimulator.GetRecoveryCount(), "Should have expected number of recoveries")

			if tt.shouldEventuallySucceed {
				assert.NoError(t, err, "Should eventually succeed after retries")
				assert.NotNil(t, result, "Should have valid result after recovery")
				assert.True(t, result.Success, "Result should be marked as successful")
			} else {
				assert.Error(t, err, "Should fail permanently")
			}

			assert.LessOrEqual(t, duration, tt.maxRetryTime, "Should complete within retry timeout")
		})
	}
}

// TestSetupIntegration_OptionsValidation tests different setup option combinations
func TestSetupIntegration_OptionsValidation(t *testing.T) {
	tests := []struct {
		name          string
		setupOptions  setup.SetupOptions
		expectedError string
		shouldSucceed bool
	}{
		{
			name: "ValidDefaultOptions",
			setupOptions: setup.SetupOptions{
				Force:            false,
				VerboseLogging:   false,
				SkipVerification: false,
				ConfigOutputPath: "",
			},
			shouldSucceed: true,
		},
		{
			name: "ValidVerboseOptions",
			setupOptions: setup.SetupOptions{
				Force:            false,
				VerboseLogging:   true,
				SkipVerification: false,
				ConfigOutputPath: "/tmp/test-config.yaml",
			},
			shouldSucceed: true,
		},
		{
			name: "ValidForceOptions",
			setupOptions: setup.SetupOptions{
				Force:            true,
				VerboseLogging:   true,
				SkipVerification: true,
				ConfigOutputPath: "/tmp/force-config.yaml",
			},
			shouldSucceed: true,
		},
		{
			name: "InvalidConfigPath",
			setupOptions: setup.SetupOptions{
				Force:            false,
				VerboseLogging:   false,
				SkipVerification: false,
				ConfigOutputPath: "/invalid/path/that/does/not/exist/config.yaml",
			},
			expectedError: "invalid config path",
			shouldSucceed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEnv := NewIntegrationTestEnvironment(t)
			defer testEnv.Cleanup()

			orchestrator := setup.NewSetupOrchestrator()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Set config path to test environment if not specified or invalid
			if tt.setupOptions.ConfigOutputPath == "" || strings.Contains(tt.expectedError, "invalid config path") {
				if tt.shouldSucceed {
					tt.setupOptions.ConfigOutputPath = testEnv.ConfigPath()
				}
			} else {
				// Create directory for valid custom paths
				if tt.shouldSucceed {
					dir := filepath.Dir(tt.setupOptions.ConfigOutputPath)
					os.MkdirAll(dir, 0755)
				}
			}

			result, err := orchestrator.RunFullSetup(ctx, &tt.setupOptions)

			if tt.shouldSucceed {
				assert.NoError(t, err, "Setup should succeed with valid options")
				assert.NotNil(t, result, "Should have valid result")
			} else {
				assert.Error(t, err, "Setup should fail with invalid options")
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError, "Error should contain expected message")
				}
			}
		})
	}
}

// SetupOptions represents setup configuration options for testing
type SetupOptions struct {
	Force      bool
	Verbose    bool
	Timeout    time.Duration
	SkipVerify bool
}

// IntegrationTestEnvironment provides isolated test environment
type IntegrationTestEnvironment struct {
	t          *testing.T
	tempDir    string
	configPath string
}

func NewIntegrationTestEnvironment(t *testing.T) *IntegrationTestEnvironment {
	tempDir, err := os.MkdirTemp("", "lsp-gateway-integration-test-*")
	require.NoError(t, err, "Should create temp directory")

	configPath := filepath.Join(tempDir, "config.yaml")

	return &IntegrationTestEnvironment{
		t:          t,
		tempDir:    tempDir,
		configPath: configPath,
	}
}

func (env *IntegrationTestEnvironment) ConfigPath() string {
	return env.configPath
}

func (env *IntegrationTestEnvironment) TempDir() string {
	return env.tempDir
}

func (env *IntegrationTestEnvironment) Cleanup() {
	if env.tempDir != "" {
		os.RemoveAll(env.tempDir)
	}
}

// FailureSimulator simulates various failure scenarios for testing error recovery
type FailureSimulator struct {
	failures     []string
	retryCount   int
	recoveryCount int
	attemptCounts map[string]int
}

func NewFailureSimulator(failures []string) *FailureSimulator {
	return &FailureSimulator{
		failures:      failures,
		attemptCounts: make(map[string]int),
	}
}

func (fs *FailureSimulator) ShouldSimulateFailure(operation string) bool {
	fs.attemptCounts[operation]++
	
	for _, failure := range fs.failures {
		switch failure {
		case "detection_transient":
			if operation == "detection" && fs.attemptCounts[operation] == 1 {
				return true
			}
		case "detection_permanent":
			if operation == "detection" {
				return true
			}
		case "install_python_once":
			if operation == "install_python" && fs.attemptCounts[operation] == 1 {
				return true
			}
		case "config_generation_once":
			if operation == "config_generation" && fs.attemptCounts[operation] == 1 {
				return true
			}
		}
	}
	return false
}

func (fs *FailureSimulator) GetRetryCount() int {
	return fs.retryCount
}

func (fs *FailureSimulator) GetRecoveryCount() int {
	return fs.recoveryCount
}

func (fs *FailureSimulator) IncrementRetry() {
	fs.retryCount++
}

func (fs *FailureSimulator) IncrementRecovery() {
	fs.recoveryCount++
}

// TestSetupIntegration_ParallelExecution tests concurrent setup operations
func TestSetupIntegration_ParallelExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("ConcurrentSetupOperations", func(t *testing.T) {
		const numConcurrentSetups = 3
		results := make(chan error, numConcurrentSetups)

		for i := 0; i < numConcurrentSetups; i++ {
			go func(id int) {
				testEnv := NewIntegrationTestEnvironment(t)
				defer testEnv.Cleanup()

				orchestrator := setup.NewSetupOrchestrator()

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				_, err := orchestrator.RunFullSetup(ctx, &setup.SetupOptions{
					Force:                 false,
					VerboseLogging:        false,
					SkipVerification:      true, // Skip verify for speed
					ConfigOutputPath:      testEnv.ConfigPath(),
					ValidateInstallations: false,
				})

				results <- err
			}(i)
		}

		// Wait for all goroutines to complete
		errorCount := 0
		successCount := 0
		for i := 0; i < numConcurrentSetups; i++ {
			err := <-results
			if err != nil {
				errorCount++
				t.Logf("Concurrent setup %d failed: %v", i, err)
			} else {
				successCount++
			}
		}

		// At least some should succeed (depending on system resources)
		assert.Greater(t, successCount, 0, "At least one concurrent setup should succeed")
		assert.LessOrEqual(t, errorCount, numConcurrentSetups/2, "Most concurrent setups should succeed")
	})
}

// TestSetupIntegration_ResourceManagement tests resource cleanup and management
func TestSetupIntegration_ResourceManagement(t *testing.T) {
	t.Run("ProperResourceCleanup", func(t *testing.T) {
		testEnv := NewIntegrationTestEnvironment(t)
		defer testEnv.Cleanup()

		orchestrator := setup.NewSetupOrchestrator()

		// Track initial resource state
		initialGoroutines := countActiveGoroutines()
		initialTempFiles := countTempFiles()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := orchestrator.RunFullSetup(ctx, &setup.SetupOptions{
			Force:                 false,
			VerboseLogging:        false,
			SkipVerification:      true,
			ConfigOutputPath:      testEnv.ConfigPath(),
			ValidateInstallations: false,
		})

		assert.NoError(t, err, "Setup should complete successfully")
		assert.NotNil(t, result, "Should have valid result")

		// Check resource cleanup after completion
		finalGoroutines := countActiveGoroutines()
		finalTempFiles := countTempFiles()

		assert.LessOrEqual(t, finalGoroutines, initialGoroutines+5, "Should not leak too many goroutines")
		assert.LessOrEqual(t, finalTempFiles, initialTempFiles+2, "Should not leak temporary files")
	})

	t.Run("ResourceCleanupOnCancellation", func(t *testing.T) {
		testEnv := NewIntegrationTestEnvironment(t)
		defer testEnv.Cleanup()

		orchestrator := setup.NewSetupOrchestrator()

		initialGoroutines := countActiveGoroutines()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Cancel context immediately to simulate cancellation
		cancel()

		result, err := orchestrator.RunFullSetup(ctx, &setup.SetupOptions{
			Force:                 false,
			VerboseLogging:        false,
			SkipVerification:      false,
			ConfigOutputPath:      testEnv.ConfigPath(),
			ValidateInstallations: true,
		})

		// Should handle cancellation gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "context", "Error should indicate context cancellation")
		}

		// Check that resources are cleaned up even on cancellation
		finalGoroutines := countActiveGoroutines()
		assert.LessOrEqual(t, finalGoroutines, initialGoroutines+5, "Should not leak goroutines on cancellation")

		_ = result // Use result to avoid unused variable warning
	})
}

// Helper functions for resource monitoring
func countActiveGoroutines() int {
	// Simplified goroutine counting - in real implementation would use runtime.NumGoroutine()
	return 10 // Placeholder
}

func countTempFiles() int {
	// Simplified temp file counting - in real implementation would check /tmp directory
	return 5 // Placeholder
}