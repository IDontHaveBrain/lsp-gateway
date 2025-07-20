package cli

import (
	"strings"
	"testing"
)

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"STATUS_FAILED", STATUS_FAILED, "failed"},
		{"STATUS_WARNING", STATUS_WARNING, "warning"},
		{"STATUS_PASSED", STATUS_PASSED, "passed"},
		{"STATUS_SKIPPED", STATUS_SKIPPED, "skipped"},
		{"STATUS_UNKNOWN", STATUS_UNKNOWN, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestFlagConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		notEmpty bool
	}{
		{"FLAG_TIMEOUT", FLAG_TIMEOUT, true},
		{"FLAG_FORCE", FLAG_FORCE, true},
		{"FLAG_CONFIG", FLAG_CONFIG, true},
		{"FLAG_OUTPUT", FLAG_OUTPUT, true},
		{"FLAG_JSON", FLAG_JSON, true},
		{"FLAG_VERBOSE", FLAG_VERBOSE, true},
		{"FLAG_ALL", FLAG_ALL, true},
		{"FLAG_RUNTIME", FLAG_RUNTIME, true},
		{"FLAG_SERVER", FLAG_SERVER, true},
		{"FLAG_GATEWAY", FLAG_GATEWAY, true},
		{"FLAG_PORT", FLAG_PORT, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.notEmpty && tt.constant == "" {
				t.Errorf("%s should not be empty", tt.name)
			}
		})
	}
}

func TestFlagDescriptionConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"FLAG_DESCRIPTION_SERVER_PORT", FLAG_DESCRIPTION_SERVER_PORT, "Server port"},
		{"FLAG_DESCRIPTION_CONFIG_PATH", FLAG_DESCRIPTION_CONFIG_PATH, "Configuration file path"},
		{"FLAG_DESCRIPTION_JSON_OUTPUT", FLAG_DESCRIPTION_JSON_OUTPUT, "Output in JSON format"},
		{"FLAG_DESCRIPTION_JSON_RESULTS", FLAG_DESCRIPTION_JSON_RESULTS, "Output results in JSON format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestValueConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"VALUE_CONFIGURATION", VALUE_CONFIGURATION, "configuration"},
		{"VALUE_CONFIG_YAML", VALUE_CONFIG_YAML, "config.yaml"},
		{"VALUE_LOCALHOST_ZERO", VALUE_LOCALHOST_ZERO, "localhost:0"},
		{"VALUE_TCP", VALUE_TCP, "tcp"},
		{"VALUE_JSON_RPC_PATH", VALUE_JSON_RPC_PATH, "/jsonrpc"},
		{"VALUE_YES", VALUE_YES, "Yes"},
		{"VALUE_N_A", VALUE_N_A, "N/A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestFormatConstants(t *testing.T) {
	// Test that format strings contain expected format verbs
	tests := []struct {
		name         string
		constant     string
		expectedVerb string
	}{
		{"FORMAT_EXECUTABLE_PATH", FORMAT_EXECUTABLE_PATH, "%s"},
		{"FORMAT_VERIFIED_AT", FORMAT_VERIFIED_AT, "%s"},
		{"FORMAT_PATH", FORMAT_PATH, "%s"},
		{"FORMAT_RUNTIME_NAME", FORMAT_RUNTIME_NAME, "%s"},
		{"FORMAT_SERVER_NAME", FORMAT_SERVER_NAME, "%s"},
		{"FORMAT_INSTALLING", FORMAT_INSTALLING, "%s"},
		{"FORMAT_INSTALLING_RUNTIME", FORMAT_INSTALLING_RUNTIME, "%s"},
		{"FORMAT_VERSION", FORMAT_VERSION, "%s"},
		{"FORMAT_BUILD_TIME", FORMAT_BUILD_TIME, "%s"},
		{"FORMAT_PORT_FORMAT", VALUE_PORT_FORMAT, "%d"},
		{"FORMAT_PATH_FORMAT", VALUE_PATH_FORMAT, "%s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.constant, tt.expectedVerb) {
				t.Errorf("%s should contain format verb %s, but got: %s", tt.name, tt.expectedVerb, tt.constant)
			}
		})
	}
}

func TestLabelConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		notEmpty bool
	}{
		{"LABEL_RUNTIME_STATUS", LABEL_RUNTIME_STATUS, true},
		{"LABEL_WARNINGS", LABEL_WARNINGS, true},
		{"LABEL_MESSAGES", LABEL_MESSAGES, true},
		{"LABEL_WARNING_ITEMS", LABEL_WARNING_ITEMS, true},
		{"LABEL_ISSUES", LABEL_ISSUES, true},
		{"LABEL_SUCCESS_CHECK", LABEL_SUCCESS_CHECK, true},
		{"LABEL_ERROR_CHECK", LABEL_ERROR_CHECK, true},
		{"LABEL_WARNING_CHECK", LABEL_WARNING_CHECK, true},
		{"LABEL_NOT_INSTALLED", LABEL_NOT_INSTALLED, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.notEmpty && tt.constant == "" {
				t.Errorf("%s should not be empty", tt.name)
			}
		})
	}
}

func TestMessageConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		hasVerb  bool
	}{
		{"MSG_CHECK_INSTALL", MSG_CHECK_INSTALL, true},
		{"MSG_INSTALL_RUNTIME", MSG_INSTALL_RUNTIME, true},
		{"MSG_INSTALL_SERVER", MSG_INSTALL_SERVER, true},
		{"MSG_RUNTIME_NOT_INSTALLED", MSG_RUNTIME_NOT_INSTALLED, true},
		{"MSG_VERSION_NOT_MEET", MSG_VERSION_NOT_MEET, true},
		{"MSG_PROPERLY_INSTALLED", MSG_PROPERLY_INSTALLED, true},
		{"MSG_SERVER_FUNCTIONAL", MSG_SERVER_FUNCTIONAL, true},
		{"MSG_ADD_LANGUAGE_CONFIGS", MSG_ADD_LANGUAGE_CONFIGS, false},
		{"MSG_TRY_INSTALL_SERVER", MSG_TRY_INSTALL_SERVER, true},
		{"MSG_TRY_INSTALL_RUNTIME", MSG_TRY_INSTALL_RUNTIME, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasVerb && !strings.Contains(tt.constant, "%s") {
				t.Errorf("%s should contain format verb %%s for string formatting", tt.name)
			}
			if tt.constant == "" {
				t.Errorf("%s should not be empty", tt.name)
			}
		})
	}
}

func TestErrorConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		hasVerb  bool
	}{
		{"ERROR_UNKNOWN_VALUE", ERROR_UNKNOWN_VALUE, false},
		{"ERROR_FILE_NOT_EXIST", ERROR_FILE_NOT_EXIST, true},
		{"ERROR_PATH_IS_DIRECTORY", ERROR_PATH_IS_DIRECTORY, true},
		{"ERROR_KILL_PROCESS", ERROR_KILL_PROCESS, true},
		{"ERROR_CHECK_PORT_USAGE", ERROR_CHECK_PORT_USAGE, true},
		{"ERROR_ADDRESS_IN_USE", ERROR_ADDRESS_IN_USE, false},
		{"ERROR_JSON_MARSHAL", ERROR_JSON_MARSHAL, true},
		{"ERROR_FLUSH_TABLE", ERROR_FLUSH_TABLE, true},
		{"ERROR_WRITE_TABLE_HEADER", ERROR_WRITE_TABLE_HEADER, true},
		{"ERROR_LOAD_CONFIGURATION", ERROR_LOAD_CONFIGURATION, true},
		{"ERROR_VERIFY_SERVER", ERROR_VERIFY_SERVER, true},
		{"ERROR_VERIFY_RUNTIME", ERROR_VERIFY_RUNTIME, true},
		{"ERROR_UNSUPPORTED_RUNTIME", ERROR_UNSUPPORTED_RUNTIME, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasVerb && !strings.Contains(tt.constant, "%") {
				t.Errorf("%s should contain format verbs for formatting", tt.name)
			}
			if tt.constant == "" {
				t.Errorf("%s should not be empty", tt.name)
			}
		})
	}
}

func TestSuggestionConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		hasVerb  bool
	}{
		{"SUGGESTION_LIST_DIRECTORY", SUGGESTION_LIST_DIRECTORY, true},
		{"SUGGESTION_CHECK_PERMISSIONS", SUGGESTION_CHECK_PERMISSIONS, true},
		{"SUGGESTION_FIX_PERMISSIONS", SUGGESTION_FIX_PERMISSIONS, true},
		{"SUGGESTION_SUPPORTED_SERVERS", SUGGESTION_SUPPORTED_SERVERS, true},
		{"SUGGESTION_SUPPORTED_RUNTIMES", SUGGESTION_SUPPORTED_RUNTIMES, true},
		{"AUTO_INSTALL_SUGGESTION", AUTO_INSTALL_SUGGESTION, true},
		{"SERVER_NOT_INSTALLED", SERVER_NOT_INSTALLED, true},
		{"SERVER_COMPATIBILITY_ISSUES", SERVER_COMPATIBILITY_ISSUES, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasVerb && !strings.Contains(tt.constant, "%s") {
				t.Errorf("%s should contain format verb %%s for string formatting", tt.name)
			}
			if tt.constant == "" {
				t.Errorf("%s should not be empty", tt.name)
			}
		})
	}
}

func TestWordConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"WORD_RUNTIME", WORD_RUNTIME, "runtime"},
		{"WORD_PLATFORM", WORD_PLATFORM, "platform"},
		{"WORD_SHARE", WORD_SHARE, "share"},
		{"WORD_LIST", WORD_LIST, "list"},
		{"WORD_MAJOR", WORD_MAJOR, "major"},
		{"WORD_MINOR", WORD_MINOR, "minor"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

func TestConstantUniqueness(t *testing.T) {
	// Test that all status constants have unique values
	statusConstants := []string{STATUS_FAILED, STATUS_WARNING, STATUS_PASSED, STATUS_SKIPPED, STATUS_UNKNOWN}
	seen := make(map[string]bool)

	for _, constant := range statusConstants {
		if seen[constant] {
			t.Errorf("Duplicate status constant value: %s", constant)
		}
		seen[constant] = true
	}
}

func TestFormatStringConsistency(t *testing.T) {
	// Test that format strings have consistent patterns
	formatTests := []struct {
		name     string
		constant string
		pattern  string
	}{
		{"FORMAT_LIST_ITEM", FORMAT_LIST_ITEM, "    - %s\n"},
		{"FORMAT_LIST_ITEM_DASH", FORMAT_LIST_ITEM_DASH, "  - %s\n"},
		{"FORMAT_LIST_ITEM_BRACKET", FORMAT_LIST_ITEM_BRACKET, "  [%s] %s\n"},
		{"FORMAT_SOLUTION_INSTRUCTION", FORMAT_SOLUTION_INSTRUCTION, "    Solution: %s\n"},
		{"FORMAT_SPACES_INSTRUCTION", FORMAT_SPACES_INSTRUCTION, "    %s\n"},
	}

	for _, tt := range formatTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.pattern {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.pattern)
			}
		})
	}
}

// Benchmark constant access (should be instant)
func BenchmarkConstantAccess(b *testing.B) {
	var result string
	for i := 0; i < b.N; i++ {
		result = STATUS_PASSED
	}
	_ = result
}

func BenchmarkFormatConstantUsage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Simulate typical usage of format constants
		_ = strings.Contains(FORMAT_EXECUTABLE_PATH, "%s")
		_ = strings.Contains(FORMAT_VERSION, "%s")
		_ = strings.Contains(VALUE_PORT_FORMAT, "%d")
	}
}