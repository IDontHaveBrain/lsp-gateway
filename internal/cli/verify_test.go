package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"lsp-gateway/internal/installer"

	"github.com/spf13/cobra"
)

func TestVerifyCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "verify go runtime",
			args:        []string{"runtime", "go"},
			expectError: false,
		},
		{
			name:        "verify python runtime",
			args:        []string{"runtime", "python"},
			expectError: false,
		},
		{
			name:        "verify nodejs runtime",
			args:        []string{"runtime", "nodejs"},
			expectError: false,
		},
		{
			name:        "verify java runtime",
			args:        []string{"runtime", "java"},
			expectError: false,
		},
		{
			name:        "verify all runtimes",
			args:        []string{"runtime", "all"},
			expectError: false,
		},
		{
			name:        "verify invalid runtime",
			args:        []string{"runtime", "invalid"},
			expectError: true,
		},
		{
			name:        "verify no args",
			args:        []string{"runtime"},
			expectError: true,
		},
		{
			name:        "verify too many args",
			args:        []string{"runtime", "go", "extra"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create fresh command
			cmd := &cobra.Command{
				Use:       "runtime",
				Args:      cobra.ExactArgs(1),
				ValidArgs: []string{"go", "python", "nodejs", "java", "all"},
				RunE:      runVerifyRuntime,
			}
			cmd.Flags().Bool("json", false, "JSON output")
			cmd.Flags().Bool("verbose", false, "Verbose output")
			cmd.Flags().Bool("all", false, "Verify all")

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			if tt.expectError && err == nil {
				t.Errorf("Expected error for args %v, but got none", tt.args)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for args %v: %v", tt.args, err)
			}

			// Check output was generated for successful cases
			if !tt.expectError && len(output) == 0 {
				t.Errorf("Expected output for args %v, but got none", tt.args)
			}
		})
	}
}

func TestRunVerifyRuntime(t *testing.T) {
	tests := []struct {
		name        string
		runtime     string
		jsonFlag    bool
		verboseFlag bool
		allFlag     bool
		expectError bool
	}{
		{
			name:        "verify go",
			runtime:     "go",
			jsonFlag:    false,
			verboseFlag: false,
			allFlag:     false,
			expectError: false,
		},
		{
			name:        "verify go with json",
			runtime:     "go",
			jsonFlag:    true,
			verboseFlag: false,
			allFlag:     false,
			expectError: false,
		},
		{
			name:        "verify go with verbose",
			runtime:     "go",
			jsonFlag:    false,
			verboseFlag: true,
			allFlag:     false,
			expectError: false,
		},
		{
			name:        "verify all runtimes",
			runtime:     "all",
			jsonFlag:    false,
			verboseFlag: false,
			allFlag:     false,
			expectError: false,
		},
		{
			name:        "verify with all flag",
			runtime:     "go",
			jsonFlag:    false,
			verboseFlag: false,
			allFlag:     true,
			expectError: false,
		},
		{
			name:        "verify invalid runtime",
			runtime:     "invalid",
			jsonFlag:    false,
			verboseFlag: false,
			allFlag:     false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set flags
			verifyJSON = tt.jsonFlag
			verifyVerbose = tt.verboseFlag
			verifyAll = tt.allFlag

			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cmd := &cobra.Command{Use: "runtime"}
			err := runVerifyRuntime(cmd, []string{tt.runtime})

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			// Reset flags
			verifyJSON = false
			verifyVerbose = false
			verifyAll = false

			if tt.expectError && err == nil {
				t.Errorf("Expected error for runtime %s, but got none", tt.runtime)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for runtime %s: %v", tt.runtime, err)
			}

			// Check output format
			if !tt.expectError {
				if tt.jsonFlag {
					var jsonOutput map[string]interface{}
					if err := json.Unmarshal(output, &jsonOutput); err != nil {
						t.Errorf("Expected valid JSON output, but got error: %v. Output: %s", err, string(output))
					}
				} else {
					outputStr := string(output)
					if len(outputStr) == 0 {
						t.Errorf("Expected output for runtime %s, but got none", tt.runtime)
					}
				}
			}
		})
	}
}

func TestVerifyAllRuntimes(t *testing.T) {
	tests := []struct {
		name        string
		jsonFlag    bool
		verboseFlag bool
	}{
		{
			name:        "verify all default",
			jsonFlag:    false,
			verboseFlag: false,
		},
		{
			name:        "verify all json",
			jsonFlag:    true,
			verboseFlag: false,
		},
		{
			name:        "verify all verbose",
			jsonFlag:    false,
			verboseFlag: true,
		},
		{
			name:        "verify all json verbose",
			jsonFlag:    true,
			verboseFlag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set flags
			verifyJSON = tt.jsonFlag
			verifyVerbose = tt.verboseFlag

			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create mock installer
			runtimeInstaller := installer.NewRuntimeInstaller()
			if runtimeInstaller == nil {
				t.Skip("Runtime installer not available")
			}

			results, err := verifyAllRuntimes(runtimeInstaller)

			w.Close()
			_, _ = io.ReadAll(r)
			os.Stdout = oldStdout

			// Reset flags
			verifyJSON = false
			verifyVerbose = false

			if err != nil && !strings.Contains(err.Error(), "runtime installer not available") {
				t.Errorf("verifyAllRuntimes() error = %v", err)
			}

			if len(results) == 0 {
				t.Error("Expected some runtime results, but got none")
			}
		})
	}
}

func TestVerifySingleRuntime(t *testing.T) {
	tests := []struct {
		name        string
		runtime     string
		expectError bool
	}{
		{
			name:        "verify go",
			runtime:     "go",
			expectError: false,
		},
		{
			name:        "verify python",
			runtime:     "python",
			expectError: false,
		},
		{
			name:        "verify nodejs",
			runtime:     "nodejs",
			expectError: false,
		},
		{
			name:        "verify java",
			runtime:     "java",
			expectError: false,
		},
		{
			name:        "verify invalid",
			runtime:     "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Create mock installer
			runtimeInstaller := installer.NewRuntimeInstaller()
			if runtimeInstaller == nil {
				t.Skip("Runtime installer not available")
			}

			result, err := verifySingleRuntime(runtimeInstaller, tt.runtime)

			w.Close()
			_, _ = io.ReadAll(r)
			os.Stdout = oldStdout

			if tt.expectError && err == nil {
				t.Errorf("Expected error for runtime %s, but got none", tt.runtime)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for runtime %s: %v", tt.runtime, err)
			}

			// Check result structure
			if !tt.expectError {
				if result.Runtime != tt.runtime {
					t.Errorf("Expected result.Runtime = %s, got %s", tt.runtime, result.Runtime)
				}
			}
		})
	}
}

func TestDisplaySingleVerificationResult(t *testing.T) {
	tests := []struct {
		name         string
		runtimeName  string
		result       *installer.VerificationResult
		err          error
		expectedText string
	}{
		{
			name:        "installed and compatible",
			runtimeName: "go",
			result: &installer.VerificationResult{
				Installed:  true,
				Compatible: true,
				Version:    "1.20.0",
				Path:       "/usr/bin/go",
			},
			err:          nil,
			expectedText: "✓ go: Installed and working correctly",
		},
		{
			name:        "installed but incompatible",
			runtimeName: "python",
			result: &installer.VerificationResult{
				Installed:  true,
				Compatible: false,
				Version:    "2.7.0",
				Path:       "/usr/bin/python",
			},
			err:          nil,
			expectedText: "⚠ python: Installed but version incompatible",
		},
		{
			name:        "not installed",
			runtimeName: "nodejs",
			result: &installer.VerificationResult{
				Installed:  false,
				Compatible: false,
				Version:    "",
				Path:       "",
			},
			err:          nil,
			expectedText: "✗ nodejs: Not installed",
		},
		{
			name:         "verification error",
			runtimeName:  "java",
			result:       nil,
			err:          fmt.Errorf("verification failed"),
			expectedText: "✗ java: Verification failed",
		},
		{
			name:         "nil result",
			runtimeName:  "go",
			result:       nil,
			err:          nil,
			expectedText: "✗ go: No verification result available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			verifyVerbose = false
			displaySingleVerificationResult(tt.runtimeName, tt.result, tt.err)

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			outputStr := string(output)
			if !strings.Contains(outputStr, tt.expectedText) {
				t.Errorf("Expected output to contain %q, but got: %s", tt.expectedText, outputStr)
			}
		})
	}
}

func TestDisplayVerificationSummary(t *testing.T) {
	tests := []struct {
		name     string
		results  []RuntimeVerificationResult
		expected []string
	}{
		{
			name: "all working",
			results: []RuntimeVerificationResult{
				{
					Runtime: "go",
					Result: &installer.VerificationResult{
						Installed:  true,
						Compatible: true,
					},
				},
				{
					Runtime: "python",
					Result: &installer.VerificationResult{
						Installed:  true,
						Compatible: true,
					},
				},
			},
			expected: []string{
				"Total runtimes checked: 2",
				"Installed: 2/2",
				"Working: 2/2",
				"✓ All runtimes are working correctly!",
			},
		},
		{
			name: "some missing",
			results: []RuntimeVerificationResult{
				{
					Runtime: "go",
					Result: &installer.VerificationResult{
						Installed:  true,
						Compatible: true,
					},
				},
				{
					Runtime: "python",
					Result: &installer.VerificationResult{
						Installed:  false,
						Compatible: false,
					},
				},
			},
			expected: []string{
				"Total runtimes checked: 2",
				"Installed: 1/2",
				"Working: 1/2",
				"✗ Some runtimes are missing or have issues",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			displayVerificationSummary(tt.results)

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			outputStr := string(output)
			for _, expected := range tt.expected {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected output to contain %q, but got: %s", expected, outputStr)
				}
			}
		})
	}
}

func TestOutputVerifyResultsJSON(t *testing.T) {
	tests := []struct {
		name         string
		results      []RuntimeVerificationResult
		verifyError  error
		expectError  bool
		expectOutput bool
	}{
		{
			name: "successful verification",
			results: []RuntimeVerificationResult{
				{
					Runtime: "go",
					Result: &installer.VerificationResult{
						Installed:  true,
						Compatible: true,
						Version:    "1.20.0",
						Path:       "/usr/bin/go",
					},
				},
			},
			verifyError:  nil,
			expectError:  false,
			expectOutput: true,
		},
		{
			name: "failed verification",
			results: []RuntimeVerificationResult{
				{
					Runtime: "python",
					Result: &installer.VerificationResult{
						Installed:  false,
						Compatible: false,
					},
				},
			},
			verifyError:  nil,
			expectError:  true,
			expectOutput: true,
		},
		{
			name:         "verification error",
			results:      []RuntimeVerificationResult{},
			verifyError:  fmt.Errorf("runtime installer failed"),
			expectError:  true,
			expectOutput: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := outputVerifyResultsJSON(tt.results, tt.verifyError)

			w.Close()
			output, _ := io.ReadAll(r)
			os.Stdout = oldStdout

			if tt.expectError && err == nil {
				t.Errorf("Expected error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectOutput {
				var jsonOutput map[string]interface{}
				if err := json.Unmarshal(output, &jsonOutput); err != nil {
					t.Errorf("Expected valid JSON output, but got error: %v. Output: %s", err, string(output))
				}

				// Check JSON structure
				if _, ok := jsonOutput["success"]; !ok {
					t.Error("Expected 'success' field in JSON output")
				}
				if _, ok := jsonOutput["summary"]; !ok {
					t.Error("Expected 'summary' field in JSON output")
				}
				if _, ok := jsonOutput["results"]; !ok {
					t.Error("Expected 'results' field in JSON output")
				}
			}
		})
	}
}

func TestVerifyCommandMetadata(t *testing.T) {
	if verifyCmd.Use != "verify" {
		t.Errorf("verifyCmd.Use = %s, want verify", verifyCmd.Use)
	}

	if verifyCmd.Short == "" {
		t.Error("verifyCmd.Short should not be empty")
	}

	if verifyCmd.Long == "" {
		t.Error("verifyCmd.Long should not be empty")
	}

	if verifyRuntimeCmd.Use != "runtime <name|all>" {
		t.Errorf("verifyRuntimeCmd.Use = %s, want 'runtime <name|all>'", verifyRuntimeCmd.Use)
	}

	expectedValidArgs := []string{"go", "python", "nodejs", "java", "all"}
	if !stringSlicesEqual(verifyRuntimeCmd.ValidArgs, expectedValidArgs) {
		t.Errorf("verifyRuntimeCmd.ValidArgs = %v, want %v", verifyRuntimeCmd.ValidArgs, expectedValidArgs)
	}
}

func TestVerifyCommandFlags(t *testing.T) {
	cmd := verifyRuntimeCmd

	jsonFlag := cmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Error("json flag not found")
	}

	verboseFlag := cmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("verbose flag not found")
	}

	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Error("all flag not found")
	}
}

func TestVerifyTypes(t *testing.T) {
	// Test RuntimeVerificationResult type
	result := RuntimeVerificationResult{
		Runtime: "go",
		Result: &installer.VerificationResult{
			Installed:  true,
			Compatible: true,
			Version:    "1.20.0",
		},
	}

	if result.Runtime != "go" {
		t.Errorf("Expected Runtime = go, got %s", result.Runtime)
	}

	// Test VerificationSummary type
	summary := VerificationSummary{
		Total:      2,
		Installed:  1,
		Working:    1,
		Compatible: 1,
	}

	if summary.Total != 2 {
		t.Errorf("Expected Total = 2, got %d", summary.Total)
	}

	// Test VerificationResultJSON type
	jsonResult := VerificationResultJSON{
		Runtime:    "python",
		Installed:  true,
		Version:    "3.9.0",
		Path:       "/usr/bin/python3",
		Working:    true,
		Compatible: true,
		Issues:     []string{},
	}

	if jsonResult.Runtime != "python" {
		t.Errorf("Expected Runtime = python, got %s", jsonResult.Runtime)
	}
}

// Benchmark verify operations
func BenchmarkVerifyAllRuntimes(b *testing.B) {
	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		b.Skip("Runtime installer not available")
	}

	for i := 0; i < b.N; i++ {
		// Capture output using pipe
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		
		verifyJSON = true
		verifyAllRuntimes(runtimeInstaller)
		
		w.Close()
		_, _ = io.ReadAll(r)
		os.Stdout = oldStdout
	}
}

func BenchmarkDisplayVerificationResult(b *testing.B) {
	result := &installer.VerificationResult{
		Installed:  true,
		Compatible: true,
		Version:    "1.20.0",
		Path:       "/usr/bin/go",
	}

	for i := 0; i < b.N; i++ {
		// Capture output using pipe
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		
		displaySingleVerificationResult("go", result, nil)
		
		w.Close()
		_, _ = io.ReadAll(r)
		os.Stdout = oldStdout
	}
}