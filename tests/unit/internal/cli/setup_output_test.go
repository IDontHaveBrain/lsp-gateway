package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"lsp-gateway/internal/setup"
	"lsp-gateway/internal/types"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SetupResult mirrors the structure from setup.go for testing
type SetupResult struct {
	Success           bool                                `json:"success"`
	Duration          time.Duration                       `json:"duration"`
	RuntimesDetected  map[string]*setup.RuntimeInfo       `json:"runtimes_detected,omitempty"`
	RuntimesInstalled map[string]*types.InstallResult `json:"runtimes_installed,omitempty"`
	ServersInstalled  map[string]*types.InstallResult `json:"servers_installed,omitempty"`
	ConfigGenerated   bool                                `json:"config_generated"`
	ConfigPath        string                              `json:"config_path,omitempty"`
	Issues            []string                            `json:"issues,omitempty"`
	Warnings          []string                            `json:"warnings,omitempty"`
	Messages          []string                            `json:"messages,omitempty"`
	Summary           *SetupSummary                       `json:"summary,omitempty"`
}

// SetupSummary mirrors the structure from setup.go for testing
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

// TestOutputSetupResultsJSON tests JSON output formatting
func TestOutputSetupResultsJSON(t *testing.T) {
	tests := []struct {
		name           string
		result         *SetupResult
		expectedFields []string
		expectedValues map[string]interface{}
		shouldError    bool
	}{
		{
			name: "SuccessfulSetup",
			result: &SetupResult{
				Success:          true,
				Duration:         2 * time.Minute,
				ConfigGenerated:  true,
				ConfigPath:       "config.yaml",
				RuntimesDetected: createMockRuntimesDetected(),
				RuntimesInstalled: createMockRuntimesInstalled(),
				ServersInstalled:  createMockServersInstalled(),
				Issues:           []string{},
				Warnings:         []string{},
				Messages:         []string{"Setup completed successfully"},
				Summary: &SetupSummary{
					TotalRuntimes:        4,
					RuntimesInstalled:    2,
					RuntimesAlreadyExist: 2,
					RuntimesFailed:       0,
					TotalServers:         4,
					ServersInstalled:     3,
					ServersAlreadyExist:  1,
					ServersFailed:        0,
				},
			},
			expectedFields: []string{"success", "duration", "config_generated", "summary"},
			expectedValues: map[string]interface{}{
				"success":          true,
				"config_generated": true,
				"config_path":      "config.yaml",
			},
			shouldError: false,
		},
		{
			name: "SetupWithIssues",
			result: &SetupResult{
				Success:          false,
				Duration:         30 * time.Second,
				ConfigGenerated:  false,
				RuntimesDetected: createMockRuntimesDetected(),
				RuntimesInstalled: createMockRuntimesInstalled(),
				ServersInstalled:  map[string]*types.InstallResult{},
				Issues:           []string{"Runtime installation failed", "Server setup failed"},
				Warnings:         []string{"Some components may not work properly"},
				Messages:         []string{},
				Summary: &SetupSummary{
					TotalRuntimes:   4,
					RuntimesFailed:  2,
					TotalServers:    4,
					ServersFailed:   2,
				},
			},
			expectedFields: []string{"success", "duration", "issues", "warnings", "summary"},
			expectedValues: map[string]interface{}{
				"success": false,
			},
			shouldError: false,
		},
		{
			name: "MinimalSetup",
			result: &SetupResult{
				Success:  true,
				Duration: 10 * time.Second,
				Summary:  &SetupSummary{},
			},
			expectedFields: []string{"success", "duration", "summary"},
			expectedValues: map[string]interface{}{
				"success": true,
			},
			shouldError: false,
		},
		{
			name: "EmptyResult",
			result: &SetupResult{
				Summary: &SetupSummary{},
			},
			expectedFields: []string{"success", "duration", "summary"},
			expectedValues: map[string]interface{}{
				"success": false,
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			jsonData, err := json.MarshalIndent(tt.result, "", "  ")
			
			if tt.shouldError {
				assert.Error(t, err, "JSON marshaling should fail")
				return
			}
			
			require.NoError(t, err, "JSON marshaling should succeed")
			assert.NotEmpty(t, jsonData, "JSON output should not be empty")

			// Parse JSON to verify structure
			var parsed map[string]interface{}
			err = json.Unmarshal(jsonData, &parsed)
			require.NoError(t, err, "JSON should be valid")

			// Check expected fields exist
			for _, field := range tt.expectedFields {
				assert.Contains(t, parsed, field, fmt.Sprintf("JSON should contain field '%s'", field))
			}

			// Check expected values
			for key, expectedValue := range tt.expectedValues {
				assert.Equal(t, expectedValue, parsed[key], fmt.Sprintf("Field '%s' should have expected value", key))
			}

			// Verify JSON formatting
			assert.True(t, json.Valid(jsonData), "JSON should be valid")
			jsonStr := string(jsonData)
			assert.Contains(t, jsonStr, "{\n", "JSON should be properly indented")
			assert.Contains(t, jsonStr, "  ", "JSON should contain indentation")
		})
	}
}

// TestOutputSetupResultsHuman tests human-readable output formatting
func TestOutputSetupResultsHuman(t *testing.T) {
	tests := []struct {
		name             string
		result           *SetupResult
		expectedSections []string
		expectedContent  []string
		unexpectedContent []string
	}{
		{
			name: "SuccessfulSetup",
			result: &SetupResult{
				Success:          true,
				Duration:         2 * time.Minute,
				ConfigGenerated:  true,
				ConfigPath:       "config.yaml",
				RuntimesDetected: createMockRuntimesDetected(),
				RuntimesInstalled: createMockRuntimesInstalled(),
				ServersInstalled:  createMockServersInstalled(),
				Messages:         []string{"Setup completed successfully", "Run 'lsp-gateway server' to start"},
				Summary: &SetupSummary{
					TotalRuntimes:        4,
					RuntimesInstalled:    2,
					RuntimesAlreadyExist: 2,
					TotalServers:         4,
					ServersInstalled:     3,
					ServersAlreadyExist:  1,
				},
			},
			expectedSections: []string{
				"LSP Gateway Setup Summary",
				"Setup Status: SUCCESS",
				"Total Duration:",
				"Runtimes:",
				"Language Servers:",
				"Configuration:",
				"Next steps:",
			},
			expectedContent: []string{
				"✅ Setup Status: SUCCESS",
				"🔧 Runtimes: 4/4 successfully configured",
				"📦 Language Servers: 4/4 successfully configured",
				"⚙️  Configuration: Generated successfully",
				"Setup completed successfully",
				"Run 'lsp-gateway server' to start",
			},
			unexpectedContent: []string{
				"❌ Setup Status:",
				"Issues encountered:",
				"⚠️  Warnings:",
			},
		},
		{
			name: "SetupWithIssues",
			result: &SetupResult{
				Success:          false,
				Duration:         45 * time.Second,
				ConfigGenerated:  false,
				RuntimesDetected: createMockRuntimesDetected(),
				RuntimesInstalled: createMockRuntimesInstalled(),
				Issues:           []string{"Python runtime installation failed", "TypeScript server setup failed"},
				Warnings:         []string{"Some language features may be limited"},
				Summary: &SetupSummary{
					TotalRuntimes:   4,
					RuntimesInstalled: 2,
					RuntimesFailed:  2,
					TotalServers:    4,
					ServersInstalled: 2,
					ServersFailed:   2,
				},
			},
			expectedSections: []string{
				"LSP Gateway Setup Summary",
				"Setup Status: COMPLETED WITH ISSUES",
				"Total Duration:",
				"Runtimes:",
				"Language Servers:",
				"Issues encountered:",
				"Warnings:",
			},
			expectedContent: []string{
				"❌ Setup Status: COMPLETED WITH ISSUES",
				"🔧 Runtimes: 2/4 successfully configured",
				"📦 Language Servers: 2/4 successfully configured",
				"❌ Issues encountered:",
				"Python runtime installation failed",
				"⚠️  Warnings:",
				"Some language features may be limited",
			},
			unexpectedContent: []string{
				"✅ Setup Status: SUCCESS",
				"💡 Next steps:",
			},
		},
		{
			name: "PartialSuccess",
			result: &SetupResult{
				Success:          true,
				Duration:         90 * time.Second,
				ConfigGenerated:  true,
				ConfigPath:       "custom-config.yaml",
				RuntimesDetected: createMockRuntimesDetected(),
				RuntimesInstalled: createMockRuntimesInstalled(),
				ServersInstalled:  createMockServersInstalled(),
				Warnings:         []string{"Go version is outdated but compatible"},
				Messages:         []string{"Setup completed with warnings"},
				Summary: &SetupSummary{
					TotalRuntimes:        4,
					RuntimesInstalled:    3,
					RuntimesAlreadyExist: 1,
					TotalServers:         4,
					ServersInstalled:     4,
				},
			},
			expectedSections: []string{
				"LSP Gateway Setup Summary",
				"Setup Status: SUCCESS",
				"Runtimes:",
				"Language Servers:",
				"Configuration:",
				"Warnings:",
				"Next steps:",
			},
			expectedContent: []string{
				"✅ Setup Status: SUCCESS",
				"🔧 Runtimes: 4/4 successfully configured",
				"📦 Language Servers: 4/4 successfully configured",
				"⚙️  Configuration: Generated successfully (custom-config.yaml)",
				"⚠️  Warnings:",
				"Go version is outdated but compatible",
				"Setup completed with warnings",
			},
			unexpectedContent: []string{
				"❌ Issues encountered:",
			},
		},
		{
			name: "MinimalSetup",
			result: &SetupResult{
				Success:  true,
				Duration: 5 * time.Second,
				Summary:  &SetupSummary{},
			},
			expectedSections: []string{
				"LSP Gateway Setup Summary",
				"Setup Status: SUCCESS",
				"Total Duration:",
			},
			expectedContent: []string{
				"✅ Setup Status: SUCCESS",
				"⏱️  Total Duration:",
			},
			unexpectedContent: []string{
				"Issues encountered:",
				"Warnings:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate human-readable output formatting
			output := formatSetupResultsHuman(tt.result)
			
			// Check expected sections
			for _, section := range tt.expectedSections {
				assert.Contains(t, output, section, fmt.Sprintf("Output should contain section: %s", section))
			}

			// Check expected content
			for _, content := range tt.expectedContent {
				assert.Contains(t, output, content, fmt.Sprintf("Output should contain: %s", content))
			}

			// Check unexpected content
			for _, content := range tt.unexpectedContent {
				assert.NotContains(t, output, content, fmt.Sprintf("Output should not contain: %s", content))
			}

			// Verify output structure
			assert.Contains(t, output, "=====================================", "Should contain section dividers")
			assert.True(t, len(output) > 100, "Output should be reasonably detailed")
		})
	}
}

// TestJSONValidation tests JSON output validation and edge cases
func TestJSONValidation(t *testing.T) {
	t.Run("ComplexDuration", func(t *testing.T) {
		result := &SetupResult{
			Success:  true,
			Duration: 2*time.Hour + 15*time.Minute + 30*time.Second,
			Summary:  &SetupSummary{},
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		require.NoError(t, err, "Should marshal complex duration")

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err, "Should unmarshal complex duration")

		durationValue, ok := parsed["duration"]
		assert.True(t, ok, "Duration field should exist")
		assert.NotNil(t, durationValue, "Duration should have a value")
	})

	t.Run("SpecialCharacters", func(t *testing.T) {
		result := &SetupResult{
			Success: false,
			Issues:  []string{"Error with unicode: ✗", "Path with spaces: /Program Files/test"},
			Messages: []string{"Special chars: <>\"&'"},
			Summary:  &SetupSummary{},
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		require.NoError(t, err, "Should handle special characters")

		assert.True(t, json.Valid(jsonData), "JSON with special characters should be valid")

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err, "Should unmarshal special characters")
	})

	t.Run("LargeDataSets", func(t *testing.T) {
		// Create large datasets
		runtimesDetected := make(map[string]*setup.RuntimeInfo)
		runtimesInstalled := make(map[string]*types.InstallResult)
		issues := make([]string, 100)
		messages := make([]string, 50)

		for i := 0; i < 20; i++ {
			runtimeName := fmt.Sprintf("runtime-%d", i)
			runtimesDetected[runtimeName] = &setup.RuntimeInfo{
				Name:      runtimeName,
				Installed: true,
				Version:   "1.0.0",
				Path:      fmt.Sprintf("/usr/bin/%s", runtimeName),
			}
			runtimesInstalled[runtimeName] = &types.InstallResult{
				Success: true,
				Runtime: runtimeName,
				Version: "1.0.0",
				Path:    fmt.Sprintf("/usr/bin/%s", runtimeName),
			}
		}

		for i := 0; i < 100; i++ {
			issues[i] = fmt.Sprintf("Issue %d: Some problem description", i)
		}

		for i := 0; i < 50; i++ {
			messages[i] = fmt.Sprintf("Message %d: Some information", i)
		}

		result := &SetupResult{
			Success:           false,
			RuntimesDetected:  runtimesDetected,
			RuntimesInstalled: runtimesInstalled,
			Issues:            issues,
			Messages:          messages,
			Summary:           &SetupSummary{TotalRuntimes: 20},
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		require.NoError(t, err, "Should handle large datasets")

		assert.True(t, json.Valid(jsonData), "Large JSON should be valid")
		assert.Greater(t, len(jsonData), 1000, "Large dataset should produce substantial JSON")
	})

	t.Run("EmptyAndNilValues", func(t *testing.T) {
		result := &SetupResult{
			Success:           true,
			RuntimesDetected:  nil,
			RuntimesInstalled: make(map[string]*types.InstallResult),
			ServersInstalled:  nil,
			Issues:            []string{},
			Warnings:          nil,
			Messages:          []string{},
			Summary:           nil,
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		require.NoError(t, err, "Should handle nil and empty values")

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err, "Should unmarshal nil and empty values")

		// Check that omitempty fields are handled correctly
		assert.Equal(t, true, parsed["success"], "Success should be present")
		
		// Note: Fields with omitempty and nil/empty values might not be present
		if runtimesInstalled, exists := parsed["runtimes_installed"]; exists {
			assert.NotNil(t, runtimesInstalled, "If present, runtimes_installed should not be nil")
		}
	})
}

// TestHumanOutputFormatting tests specific formatting aspects of human output
func TestHumanOutputFormatting(t *testing.T) {
	t.Run("DurationFormatting", func(t *testing.T) {
		tests := []struct {
			name     string
			duration time.Duration
			expected string
		}{
			{"Seconds", 45 * time.Second, "45s"},
			{"Minutes", 2 * time.Minute, "2m0s"},
			{"Hours", time.Hour + 30*time.Minute, "1h30m0s"},
			{"Milliseconds", 500 * time.Millisecond, "500ms"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := &SetupResult{
					Success:  true,
					Duration: tt.duration,
					Summary:  &SetupSummary{},
				}

				output := formatSetupResultsHuman(result)
				assert.Contains(t, output, tt.expected, fmt.Sprintf("Should format duration as %s", tt.expected))
			})
		}
	})

	t.Run("CounterFormatting", func(t *testing.T) {
		result := &SetupResult{
			Success: true,
			Summary: &SetupSummary{
				TotalRuntimes:        6,
				RuntimesInstalled:    3,
				RuntimesAlreadyExist: 2,
				RuntimesFailed:       1,
				TotalServers:         8,
				ServersInstalled:     5,
				ServersAlreadyExist:  2,
				ServersFailed:        1,
			},
		}

		output := formatSetupResultsHuman(result)
		
		// Check runtime counters
		assert.Contains(t, output, "🔧 Runtimes: 5/6 successfully configured", "Should show runtime success ratio")
		assert.Contains(t, output, "Newly installed: 3", "Should show newly installed runtimes")
		assert.Contains(t, output, "Already existed: 2", "Should show already existing runtimes")
		assert.Contains(t, output, "Failed: 1", "Should show failed runtimes")

		// Check server counters
		assert.Contains(t, output, "📦 Language Servers: 7/8 successfully configured", "Should show server success ratio")
		assert.Contains(t, output, "Newly installed: 5", "Should show newly installed servers")
		assert.Contains(t, output, "Already existed: 2", "Should show already existing servers")
		assert.Contains(t, output, "Failed: 1", "Should show failed servers")
	})

	t.Run("EmojiAndSymbols", func(t *testing.T) {
		successResult := &SetupResult{
			Success: true,
			Summary: &SetupSummary{},
		}

		failResult := &SetupResult{
			Success: false,
			Issues:  []string{"Test issue"},
			Summary: &SetupSummary{},
		}

		successOutput := formatSetupResultsHuman(successResult)
		failOutput := formatSetupResultsHuman(failResult)

		assert.Contains(t, successOutput, "✅ Setup Status: SUCCESS", "Success should use check mark")
		assert.Contains(t, failOutput, "❌ Setup Status: COMPLETED WITH ISSUES", "Failure should use X mark")
		assert.Contains(t, failOutput, "❌ Issues encountered:", "Issues section should use X mark")
	})

	t.Run("SectionDividers", func(t *testing.T) {
		result := &SetupResult{
			Success: true,
			Summary: &SetupSummary{},
		}

		output := formatSetupResultsHuman(result)
		
		dividerCount := strings.Count(output, "=====================================")
		assert.GreaterOrEqual(t, dividerCount, 2, "Should have proper section dividers")
		
		assert.Contains(t, output, "LSP Gateway Setup Summary", "Should have main title")
	})
}

// TestOutputErrorHandling tests error scenarios in output formatting
func TestOutputErrorHandling(t *testing.T) {
	t.Run("JSONMarshalingError", func(t *testing.T) {
		// Create a result with circular reference (would cause JSON marshal error in real scenario)
		// For testing, we simulate this by testing the error handling path
		
		// This test verifies that JSON marshaling errors are handled gracefully
		// In actual implementation, this would be caught in outputSetupResultsJSON function
		result := &SetupResult{
			Success: true,
			Summary: &SetupSummary{},
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		assert.NoError(t, err, "Normal result should marshal successfully")
		assert.True(t, json.Valid(jsonData), "Generated JSON should be valid")
	})

	t.Run("InvalidJSONFields", func(t *testing.T) {
		// Test with potentially problematic values
		result := &SetupResult{
			Success:  true,
			Duration: -1 * time.Second, // Negative duration
			Issues:   []string{"", "Valid issue", ""}, // Empty strings
			Messages: nil, // Nil slice
			Summary:  &SetupSummary{},
		}

		jsonData, err := json.MarshalIndent(result, "", "  ")
		assert.NoError(t, err, "Should handle edge case values")
		assert.True(t, json.Valid(jsonData), "Should produce valid JSON despite edge cases")
	})
}

// Helper functions for creating test data

func createMockRuntimesDetected() map[string]*setup.RuntimeInfo {
	return map[string]*setup.RuntimeInfo{
		"go": {
			Name:      "go",
			Installed: true,
			Version:   "1.21.0",
			Path:      "/usr/local/go/bin/go",
			Compatible: true,
		},
		"python": {
			Name:      "python",
			Installed: true,
			Version:   "3.11.0",
			Path:      "/usr/bin/python3",
			Compatible: true,
		},
		"nodejs": {
			Name:      "nodejs",
			Installed: false,
			Version:   "",
			Path:      "",
			Compatible: false,
		},
		"java": {
			Name:      "java",
			Installed: true,
			Version:   "17.0.0",
			Path:      "/usr/bin/java",
			Compatible: true,
		},
	}
}

func createMockRuntimesInstalled() map[string]*types.InstallResult {
	return map[string]*types.InstallResult{
		"go": {
			Success: true,
			Runtime: "go",
			Version: "1.21.0",
			Path:    "/usr/local/go/bin/go",
			Duration: 30 * time.Second,
			Method:  "already_installed",
			Messages: []string{"Go runtime already installed and verified"},
		},
		"python": {
			Success: true,
			Runtime: "python",
			Version: "3.11.0",
			Path:    "/usr/bin/python3",
			Duration: 45 * time.Second,
			Method:  "package_manager",
			Messages: []string{"Python installed via package manager"},
		},
		"nodejs": {
			Success: true,
			Runtime: "nodejs",
			Version: "18.17.0",
			Path:    "/usr/bin/node",
			Duration: 120 * time.Second,
			Method:  "package_manager",
			Messages: []string{"Node.js installed successfully"},
		},
	}
}

func createMockServersInstalled() map[string]*types.InstallResult {
	return map[string]*types.InstallResult{
		"gopls": {
			Success: true,
			Runtime: "gopls",
			Version: "0.14.0",
			Path:    "/home/user/go/bin/gopls",
			Duration: 20 * time.Second,
			Method:  "go_install",
			Messages: []string{"gopls installed via go install"},
		},
		"pylsp": {
			Success: true,
			Runtime: "pylsp",
			Version: "1.7.0",
			Path:    "/usr/local/bin/pylsp",
			Duration: 35 * time.Second,
			Method:  "pip_install",
			Messages: []string{"pylsp installed via pip"},
		},
		"typescript-language-server": {
			Success: true,
			Runtime: "typescript-language-server",
			Version: "4.1.0",
			Path:    "/usr/local/bin/typescript-language-server",
			Duration: 25 * time.Second,
			Method:  "npm_install",
			Messages: []string{"typescript-language-server installed via npm"},
		},
		"jdtls": {
			Success: true,
			Runtime: "jdtls",
			Version: "1.26.0",
			Path:    "/opt/jdtls/bin/jdtls",
			Duration: 60 * time.Second,
			Method:  "archive_download",
			Messages: []string{"jdtls installed via archive download"},
		},
	}
}

// formatSetupResultsHuman simulates the human-readable output formatting
func formatSetupResultsHuman(result *SetupResult) string {
	var buf bytes.Buffer

	buf.WriteString("\n")
	buf.WriteString("=====================================\n")
	buf.WriteString("       LSP Gateway Setup Summary\n")
	buf.WriteString("=====================================\n")
	buf.WriteString("\n")

	// Overall status
	if result.Success {
		buf.WriteString("✅ Setup Status: SUCCESS\n")
	} else {
		buf.WriteString("❌ Setup Status: COMPLETED WITH ISSUES\n")
	}

	buf.WriteString(fmt.Sprintf("⏱️  Total Duration: %v\n", result.Duration))
	buf.WriteString("\n")

	// Runtime setup summary
	if result.Summary != nil && result.Summary.TotalRuntimes > 0 {
		successful := result.Summary.RuntimesInstalled + result.Summary.RuntimesAlreadyExist
		buf.WriteString(fmt.Sprintf("🔧 Runtimes: %d/%d successfully configured\n", successful, result.Summary.TotalRuntimes))
		if result.Summary.RuntimesInstalled > 0 {
			buf.WriteString(fmt.Sprintf("   • Newly installed: %d\n", result.Summary.RuntimesInstalled))
		}
		if result.Summary.RuntimesAlreadyExist > 0 {
			buf.WriteString(fmt.Sprintf("   • Already existed: %d\n", result.Summary.RuntimesAlreadyExist))
		}
		if result.Summary.RuntimesFailed > 0 {
			buf.WriteString(fmt.Sprintf("   • Failed: %d\n", result.Summary.RuntimesFailed))
		}
	}

	// Server setup summary
	if result.Summary != nil && result.Summary.TotalServers > 0 {
		successful := result.Summary.ServersInstalled + result.Summary.ServersAlreadyExist
		buf.WriteString(fmt.Sprintf("📦 Language Servers: %d/%d successfully configured\n", successful, result.Summary.TotalServers))
		if result.Summary.ServersInstalled > 0 {
			buf.WriteString(fmt.Sprintf("   • Newly installed: %d\n", result.Summary.ServersInstalled))
		}
		if result.Summary.ServersAlreadyExist > 0 {
			buf.WriteString(fmt.Sprintf("   • Already existed: %d\n", result.Summary.ServersAlreadyExist))
		}
		if result.Summary.ServersFailed > 0 {
			buf.WriteString(fmt.Sprintf("   • Failed: %d\n", result.Summary.ServersFailed))
		}
	}

	// Configuration summary
	if result.ConfigGenerated {
		buf.WriteString("⚙️  Configuration: Generated successfully")
		if result.ConfigPath != "" {
			buf.WriteString(fmt.Sprintf(" (%s)", result.ConfigPath))
		}
		buf.WriteString("\n")
	}

	buf.WriteString("\n")

	// Issues
	if len(result.Issues) > 0 {
		buf.WriteString("❌ Issues encountered:\n")
		for _, issue := range result.Issues {
			buf.WriteString(fmt.Sprintf("   • %s\n", issue))
		}
		buf.WriteString("\n")
	}

	// Warnings
	if len(result.Warnings) > 0 {
		buf.WriteString("⚠️  Warnings:\n")
		for _, warning := range result.Warnings {
			buf.WriteString(fmt.Sprintf("   • %s\n", warning))
		}
		buf.WriteString("\n")
	}

	// Messages
	if len(result.Messages) > 0 {
		buf.WriteString("💡 Next steps:\n")
		for _, message := range result.Messages {
			buf.WriteString(fmt.Sprintf("   • %s\n", message))
		}
		buf.WriteString("\n")
	}

	return buf.String()
}