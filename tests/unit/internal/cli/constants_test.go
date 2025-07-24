package cli_test

import (
	"lsp-gateway/internal/cli"
	"strings"
	"testing"
)

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"cli.StatusFailed", cli.StatusFailed, "failed"},
		{"cli.StatusWarning", cli.StatusWarning, "warning"},
		{"cli.StatusPassed", cli.StatusPassed, "passed"},
		{"cli.StatusSkipped", cli.StatusSkipped, "skipped"},
		{"cli.StatusUnknown", cli.StatusUnknown, "Unknown"},
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
		{"cli.FLAG_TIMEOUT", cli.FLAG_TIMEOUT, true},
		{"cli.FLAG_FORCE", cli.FLAG_FORCE, true},
		{"cli.FLAG_CONFIG", cli.FLAG_CONFIG, true},
		{"cli.FLAG_OUTPUT", cli.FLAG_OUTPUT, true},
		{"cli.FLAG_JSON", cli.FLAG_JSON, true},
		{"cli.FLAG_VERBOSE", cli.FLAG_VERBOSE, true},
		{"cli.FLAG_ALL", cli.FLAG_ALL, true},
		{"cli.FLAG_RUNTIME", cli.FLAG_RUNTIME, true},
		{"cli.FLAG_SERVER", cli.FLAG_SERVER, true},
		{"cli.FLAG_GATEWAY", cli.FLAG_GATEWAY, true},
		{"cli.FLAG_PORT", cli.FLAG_PORT, true},
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
		{"FLAG_DESCRIPTION_SERVER_PORT", cli.FLAG_DESCRIPTION_SERVER_PORT, "Server port"},
		{"FLAG_DESCRIPTION_CONFIG_PATH", cli.FLAG_DESCRIPTION_CONFIG_PATH, "Configuration file path"},
		{"FLAG_DESCRIPTION_JSON_OUTPUT", cli.FLAG_DESCRIPTION_JSON_OUTPUT, "Output in JSON format"},
		{"FLAG_DESCRIPTION_JSON_RESULTS", cli.FLAG_DESCRIPTION_JSON_RESULTS, "Output results in JSON format"},
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
		{"VALUE_CONFIGURATION", cli.VALUE_CONFIGURATION, "configuration"},
		{"VALUE_CONFIG_YAML", cli.VALUE_CONFIG_YAML, "config.yaml"},
		{"VALUE_LOCALHOST_ZERO", cli.VALUE_LOCALHOST_ZERO, "localhost:0"},
		{"VALUE_TCP", cli.VALUE_TCP, "tcp"},
		{"VALUE_JSON_RPC_PATH", cli.VALUE_JSON_RPC_PATH, "/jsonrpc"},
		{"VALUE_YES", cli.VALUE_YES, "Yes"},
		{"VALUE_N_A", cli.VALUE_N_A, "N/A"},
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
		{"FORMAT_EXECUTABLE_PATH", cli.FORMAT_EXECUTABLE_PATH, "%s"},
		{"FORMAT_VERIFIED_AT", cli.FORMAT_VERIFIED_AT, "%s"},
		{"FORMAT_PATH", cli.FORMAT_PATH, "%s"},
		{"FORMAT_RUNTIME_NAME", cli.FORMAT_RUNTIME_NAME, "%s"},
		{"FORMAT_SERVER_NAME", cli.FORMAT_SERVER_NAME, "%s"},
		{"FORMAT_INSTALLING", cli.FORMAT_INSTALLING, "%s"},
		{"FORMAT_INSTALLING_RUNTIME", cli.FORMAT_INSTALLING_RUNTIME, "%s"},
		{"FORMAT_VERSION", cli.FORMAT_VERSION, "%s"},
		{"FORMAT_BUILD_TIME", cli.FORMAT_BUILD_TIME, "%s"},
		{"FORMAT_PORT_FORMAT", cli.VALUE_PORT_FORMAT, "%d"},
		{"FORMAT_PATH_FORMAT", cli.VALUE_PATH_FORMAT, "%s"},
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
		{"LABEL_RUNTIME_STATUS", cli.LABEL_RUNTIME_STATUS, true},
		{"LABEL_WARNINGS", cli.LABEL_WARNINGS, true},
		{"LABEL_MESSAGES", cli.LABEL_MESSAGES, true},
		{"LABEL_WARNING_ITEMS", cli.LABEL_WARNING_ITEMS, true},
		{"LABEL_ISSUES", cli.LABEL_ISSUES, true},
		{"LABEL_SUCCESS_CHECK", cli.LABEL_SUCCESS_CHECK, true},
		{"LABEL_ERROR_CHECK", cli.LABEL_ERROR_CHECK, true},
		{"LABEL_WARNING_CHECK", cli.LABEL_WARNING_CHECK, true},
		{"LABEL_NOT_INSTALLED", cli.LABEL_NOT_INSTALLED, true},
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
		{"MSG_CHECK_INSTALL", cli.MSG_CHECK_INSTALL, true},
		{"MSG_INSTALL_RUNTIME", cli.MSG_INSTALL_RUNTIME, true},
		{"MSG_INSTALL_SERVER", cli.MSG_INSTALL_SERVER, true},
		{"MSG_RUNTIME_NOT_INSTALLED", cli.MSG_RUNTIME_NOT_INSTALLED, true},
		{"MSG_VERSION_NOT_MEET", cli.MSG_VERSION_NOT_MEET, true},
		{"MSG_PROPERLY_INSTALLED", cli.MSG_PROPERLY_INSTALLED, true},
		{"MSG_SERVER_FUNCTIONAL", cli.MSG_SERVER_FUNCTIONAL, true},
		{"MSG_ADD_LANGUAGE_CONFIGS", cli.MSG_ADD_LANGUAGE_CONFIGS, false},
		{"MSG_TRY_INSTALL_SERVER", cli.MSG_TRY_INSTALL_SERVER, true},
		{"MSG_TRY_INSTALL_RUNTIME", cli.MSG_TRY_INSTALL_RUNTIME, true},
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
		{"ERROR_UNKNOWN_VALUE", cli.ERROR_UNKNOWN_VALUE, false},
		{"ERROR_FILE_NOT_EXIST", cli.ERROR_FILE_NOT_EXIST, true},
		{"ERROR_PATH_IS_DIRECTORY", cli.ERROR_PATH_IS_DIRECTORY, true},
		{"ERROR_KILL_PROCESS", cli.ERROR_KILL_PROCESS, true},
		{"ERROR_CHECK_PORT_USAGE", cli.ERROR_CHECK_PORT_USAGE, true},
		{"ERROR_ADDRESS_IN_USE", cli.ERROR_ADDRESS_IN_USE, false},
		{"ERROR_JSON_MARSHAL", cli.ERROR_JSON_MARSHAL, true},
		{"ERROR_FLUSH_TABLE", cli.ERROR_FLUSH_TABLE, true},
		{"ERROR_WRITE_TABLE_HEADER", cli.ERROR_WRITE_TABLE_HEADER, true},
		{"ERROR_LOAD_CONFIGURATION", cli.ERROR_LOAD_CONFIGURATION, true},
		{"ERROR_VERIFY_SERVER", cli.ERROR_VERIFY_SERVER, true},
		{"ERROR_VERIFY_RUNTIME", cli.ERROR_VERIFY_RUNTIME, true},
		{"ERROR_UNSUPPORTED_RUNTIME", cli.ERROR_UNSUPPORTED_RUNTIME, true},
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
		{"SUGGESTION_LIST_DIRECTORY", cli.SUGGESTION_LIST_DIRECTORY, true},
		{"SUGGESTION_CHECK_PERMISSIONS", cli.SUGGESTION_CHECK_PERMISSIONS, true},
		{"SUGGESTION_FIX_PERMISSIONS", cli.SUGGESTION_FIX_PERMISSIONS, true},
		{"SUGGESTION_SUPPORTED_SERVERS", cli.SUGGESTION_SUPPORTED_SERVERS, true},
		{"SUGGESTION_SUPPORTED_RUNTIMES", cli.SUGGESTION_SUPPORTED_RUNTIMES, true},
		{"AUTO_INSTALL_SUGGESTION", cli.AUTO_INSTALL_SUGGESTION, true},
		{"SERVER_NOT_INSTALLED", cli.SERVER_NOT_INSTALLED, true},
		{"SERVER_COMPATIBILITY_ISSUES", cli.SERVER_COMPATIBILITY_ISSUES, true},
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
		{"WORD_RUNTIME", cli.WORD_RUNTIME, "runtime"},
		{"WORD_PLATFORM", cli.WORD_PLATFORM, "platform"},
		{"WORD_SHARE", cli.WORD_SHARE, "share"},
		{"WORD_LIST", cli.WORD_LIST, "list"},
		{"WORD_MAJOR", cli.WORD_MAJOR, "major"},
		{"WORD_MINOR", cli.WORD_MINOR, "minor"},
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
	statusConstants := []string{cli.StatusFailed, cli.StatusWarning, cli.StatusPassed, cli.StatusSkipped, cli.StatusUnknown}
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
		{"FORMAT_LIST_ITEM", cli.FORMAT_LIST_ITEM, "    - %s\n"},
		{"FORMAT_LIST_ITEM_DASH", cli.FORMAT_LIST_ITEM_DASH, "  - %s\n"},
		{"FORMAT_LIST_ITEM_BRACKET", cli.FORMAT_LIST_ITEM_BRACKET, "  [%s] %s\n"},
		{"FORMAT_SOLUTION_INSTRUCTION", cli.FORMAT_SOLUTION_INSTRUCTION, "    Solution: %s\n"},
		{"FORMAT_SPACES_INSTRUCTION", cli.FORMAT_SPACES_INSTRUCTION, "    %s\n"},
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
		result = cli.StatusPassed
	}
	_ = result
}

func BenchmarkFormatConstantUsage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Simulate typical usage of format constants
		_ = strings.Contains(cli.FORMAT_EXECUTABLE_PATH, "%s")
		_ = strings.Contains(cli.FORMAT_VERSION, "%s")
		_ = strings.Contains(cli.VALUE_PORT_FORMAT, "%d")
	}
}
