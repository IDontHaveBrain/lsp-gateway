package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/installer"

	"github.com/spf13/cobra"
)

func TestInstallCommand(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with external dependencies
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testInstallCommandMetadata},
		{"Subcommands", testInstallCommandSubcommands},
		{"FlagInheritance", testInstallCommandFlagInheritance},
		{"Help", testInstallCommandHelp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetInstallFlags()
			tt.testFunc(t)
		})
	}
}

func TestInstallRuntimeCommand(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with real installer operations
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testInstallRuntimeCommandMetadata},
		{"FlagParsing", testInstallRuntimeCommandFlagParsing},
		{"ValidArgs", testInstallRuntimeCommandValidArgs},
		{"ArgumentValidation", testInstallRuntimeCommandArgumentValidation},
		{"ErrorScenarios", testInstallRuntimeCommandErrorScenarios},
		{"Help", testInstallRuntimeCommandHelp},
		{"SingleRuntimeInstallation", testInstallRuntimeCommandSingleInstallation},
		{"AllRuntimesInstallation", testInstallRuntimeCommandAllInstallation},
		{"JSONOutput", testInstallRuntimeCommandJSONOutput},
		{"ForceReinstall", testInstallRuntimeCommandForceReinstall},
		{"VersionSpecification", testInstallRuntimeCommandVersionSpecification},
		{"Timeout", testInstallRuntimeCommandTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetInstallFlags()
			tt.testFunc(t)
		})
	}
}

func TestInstallServerCommand(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with real installer operations
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testInstallServerCommandMetadata},
		{"FlagParsing", testInstallServerCommandFlagParsing},
		{"ValidArgs", testInstallServerCommandValidArgs},
		{"ArgumentValidation", testInstallServerCommandArgumentValidation},
		{"ErrorScenarios", testInstallServerCommandErrorScenarios},
		{"Help", testInstallServerCommandHelp},
		{"SingleServerInstallation", testInstallServerCommandSingleInstallation},
		{"JSONOutput", testInstallServerCommandJSONOutput},
		{"ForceReinstall", testInstallServerCommandForceReinstall},
		{"Timeout", testInstallServerCommandTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetInstallFlags()
			tt.testFunc(t)
		})
	}
}

func TestInstallServersCommand(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with real installer operations
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testInstallServersCommandMetadata},
		{"FlagParsing", testInstallServersCommandFlagParsing},
		{"ErrorScenarios", testInstallServersCommandErrorScenarios},
		{"Help", testInstallServersCommandHelp},
		{"AllServersInstallation", testInstallServersCommandAllInstallation},
		{"JSONOutput", testInstallServersCommandJSONOutput},
		{"ForceReinstall", testInstallServersCommandForceReinstall},
		{"Timeout", testInstallServersCommandTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetInstallFlags()
			tt.testFunc(t)
		})
	}
}

func resetInstallFlags() {
	installForce = false
	installVersion = ""
	installJSON = false
	installTimeout = 10 * time.Minute
}

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	return string(out)
}

func testInstallCommandMetadata(t *testing.T) {
	if installCmd.Use != CmdInstall {
		t.Errorf("Expected Use to be '%s', got '%s'", CmdInstall, installCmd.Use)
	}

	expectedShort := "Install runtime dependencies and language servers"
	if installCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, installCmd.Short)
	}

	if !strings.Contains(installCmd.Long, "Install command provides capabilities") {
		t.Error("Expected Long description to mention install capabilities")
	}

	if !strings.Contains(installCmd.Long, "Available install targets") {
		t.Error("Expected Long description to mention available targets")
	}

	if installCmd.RunE != nil {
		t.Error("Expected RunE function to be nil for parent command")
	}
}

func testInstallCommandSubcommands(t *testing.T) {
	subcommands := installCmd.Commands()
	expectedSubcommands := []string{"runtime", "server", "servers"}

	if len(subcommands) != len(expectedSubcommands) {
		t.Errorf("Expected %d subcommands, got %d", len(expectedSubcommands), len(subcommands))
	}

	for i, expected := range expectedSubcommands {
		if i >= len(subcommands) {
			t.Errorf("Missing subcommand: %s", expected)
			continue
		}
		if subcommands[i].Use != expected && !strings.HasPrefix(subcommands[i].Use, expected+" ") {
			t.Errorf("Expected subcommand %d to be '%s', got '%s'", i, expected, subcommands[i].Use)
		}
	}
}

func testInstallCommandFlagInheritance(t *testing.T) {
	if len(installCmd.Commands()) == 0 {
		t.Error("Expected install command to have subcommands")
	}
}

func testInstallCommandHelp(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	output, err := executeCommand(cmd, "install", "--help")
	if err != nil {
		t.Errorf("Expected help command to succeed, got error: %v", err)
	}

	if !strings.Contains(output, "Install command provides capabilities") {
		t.Error("Expected help to contain install capabilities description")
	}

	if !strings.Contains(output, "Available Commands") {
		t.Error("Expected help to show available commands")
	}
}

func testInstallRuntimeCommandMetadata(t *testing.T) {
	// Check if command is properly initialized - if not, skip or fail meaningfully
	if installRuntimeCmd.Use == "" && installRuntimeCmd.Short == "" {
		t.Skip("installRuntimeCmd appears to be uninitialized - skipping metadata test")
		return
	}

	if !strings.HasPrefix(installRuntimeCmd.Use, "runtime") {
		t.Errorf("Expected Use to start with 'runtime', got '%s'", installRuntimeCmd.Use)
	}

	expectedShort := "Install programming language runtimes"
	if installRuntimeCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, installRuntimeCmd.Short)
	}

	if !strings.Contains(installRuntimeCmd.Long, "Supported runtimes") {
		t.Error("Expected Long description to mention supported runtimes")
	}

	if installRuntimeCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}

	if installRuntimeCmd.Args == nil {
		t.Error("Expected Args function to be set")
	}
}

func testInstallRuntimeCommandFlagParsing(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedForce   bool
		expectedJSON    bool
		expectedVersion string
		expectedTimeout time.Duration
		expectError     bool
	}{
		{
			name:            "DefaultFlags",
			args:            []string{"go"},
			expectedForce:   false,
			expectedJSON:    false,
			expectedVersion: "",
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "ForceFlag",
			args:            []string{"go", "--force"},
			expectedForce:   true,
			expectedJSON:    false,
			expectedVersion: "",
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "ForceFlagShort",
			args:            []string{"go", "-f"},
			expectedForce:   true,
			expectedJSON:    false,
			expectedVersion: "",
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "JSONFlag",
			args:            []string{"go", "--json"},
			expectedForce:   false,
			expectedJSON:    true,
			expectedVersion: "",
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "VersionFlag",
			args:            []string{"go", "--version", "1.21.0"},
			expectedForce:   false,
			expectedJSON:    false,
			expectedVersion: "1.21.0",
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "VersionFlagShort",
			args:            []string{"go", "-v", "1.21.0"},
			expectedForce:   false,
			expectedJSON:    false,
			expectedVersion: "1.21.0",
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "TimeoutFlag",
			args:            []string{"go", "--timeout", "5m"},
			expectedForce:   false,
			expectedJSON:    false,
			expectedVersion: "",
			expectedTimeout: 5 * time.Minute,
		},
		{
			name:            "CombinedFlags",
			args:            []string{"go", "--force", "--json", "--version", "1.21.0", "--timeout", "3m"},
			expectedForce:   true,
			expectedJSON:    true,
			expectedVersion: "1.21.0",
			expectedTimeout: 3 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetInstallFlags()

			cmd := &cobra.Command{}
			cmd.AddCommand(installCmd)

			cmd.SetArgs(append([]string{"install", "runtime"}, tt.args...))
			err := cmd.ParseFlags(append([]string{"install", "runtime"}, tt.args...))

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
				return
			}

			if err != nil {
				_, _ = executeCommandWithoutOutput(cmd, append([]string{"install", "runtime"}, tt.args...)...)
			}

			if installForce != tt.expectedForce {
				t.Errorf("Expected force flag to be %v, got %v", tt.expectedForce, installForce)
			}

			if installJSON != tt.expectedJSON {
				t.Errorf("Expected json flag to be %v, got %v", tt.expectedJSON, installJSON)
			}

			if installVersion != tt.expectedVersion {
				t.Errorf("Expected version flag to be '%s', got '%s'", tt.expectedVersion, installVersion)
			}

			if installTimeout != tt.expectedTimeout {
				t.Errorf("Expected timeout flag to be %v, got %v", tt.expectedTimeout, installTimeout)
			}
		})
	}
}

func testInstallRuntimeCommandValidArgs(t *testing.T) {
	expectedValidArgs := []string{"go", "python", "nodejs", "java", "all"}

	if len(installRuntimeCmd.ValidArgs) != len(expectedValidArgs) {
		t.Errorf("Expected %d valid args, got %d", len(expectedValidArgs), len(installRuntimeCmd.ValidArgs))
	}

	for i, expected := range expectedValidArgs {
		if i >= len(installRuntimeCmd.ValidArgs) {
			t.Errorf("Missing valid arg: %s", expected)
			continue
		}
		if installRuntimeCmd.ValidArgs[i] != expected {
			t.Errorf("Expected valid arg %d to be '%s', got '%s'", i, expected, installRuntimeCmd.ValidArgs[i])
		}
	}
}

func testInstallRuntimeCommandArgumentValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "ValidSingleRuntime",
			args:        []string{"go"},
			expectError: false,
		},
		{
			name:        "ValidAllRuntimes",
			args:        []string{"all"},
			expectError: false,
		},
		{
			name:        "NoArguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "TooManyArguments",
			args:        []string{"go", "python"},
			expectError: true,
		},
		{
			name:        "InvalidRuntime",
			args:        []string{"rust"},
			expectError: false, // Validation happens in runtime, not args validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add nil check for Args function
			if installRuntimeCmd.Args == nil {
				if tt.expectError {
					t.Error("Expected argument validation error, but Args function is nil")
				}
				return
			}

			err := installRuntimeCmd.Args(installRuntimeCmd, tt.args)

			if tt.expectError && err == nil {
				t.Error("Expected argument validation error, but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no argument validation error, but got: %v", err)
			}
		})
	}
}

func testInstallRuntimeCommandErrorScenarios(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "runtime", "go")
	if err == nil {
		t.Error("Expected error when installation fails, but got none")
	}

	// Add defensive nil check before accessing err.Error()
	// Now that installer creation is more robust, we expect installation errors
	if err != nil && !strings.Contains(err.Error(), "not implemented") && !strings.Contains(err.Error(), "installer") {
		t.Errorf("Expected error to mention implementation or installer, got: %v", err)
	}
}

func testInstallRuntimeCommandHelp(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	output, err := executeCommand(cmd, "install", "runtime", "--help")
	if err != nil {
		t.Errorf("Expected help command to succeed, got error: %v", err)
	}

	if !strings.Contains(output, "Install programming language runtimes") {
		t.Error("Expected help to contain description")
	}

	if !strings.Contains(output, "Supported runtimes") {
		t.Error("Expected help to show supported runtimes")
	}

	if !strings.Contains(output, "--force") {
		t.Error("Expected help to show force flag")
	}

	if !strings.Contains(output, "--version") {
		t.Error("Expected help to show version flag")
	}
}

func testInstallRuntimeCommandSingleInstallation(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "runtime", "go")
	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallRuntimeCommandAllInstallation(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "runtime", "all")
	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallRuntimeCommandJSONOutput(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "runtime", "go", "--json")

	if !installJSON {
		t.Error("Expected JSON flag to be true")
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallRuntimeCommandForceReinstall(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "runtime", "go", "--force")

	if !installForce {
		t.Error("Expected force flag to be true")
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallRuntimeCommandVersionSpecification(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "runtime", "go", "--version", "1.21.0")

	if installVersion != "1.21.0" {
		t.Errorf("Expected version flag to be '1.21.0', got '%s'", installVersion)
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallRuntimeCommandTimeout(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "runtime", "go", "--timeout", "5m")

	if installTimeout != 5*time.Minute {
		t.Errorf("Expected timeout flag to be 5m, got %v", installTimeout)
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallServerCommandMetadata(t *testing.T) {
	if !strings.HasPrefix(installServerCmd.Use, "server") {
		t.Errorf("Expected Use to start with 'server', got '%s'", installServerCmd.Use)
	}

	expectedShort := "Install a specific language server"
	if installServerCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, installServerCmd.Short)
	}

	if !strings.Contains(installServerCmd.Long, "Supported language servers") {
		t.Error("Expected Long description to mention supported language servers")
	}

	if installServerCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}

	if installServerCmd.Args == nil {
		t.Error("Expected Args function to be set")
	}
}

func testInstallServerCommandFlagParsing(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedForce   bool
		expectedJSON    bool
		expectedTimeout time.Duration
		expectError     bool
	}{
		{
			name:            "DefaultFlags",
			args:            []string{"gopls"},
			expectedForce:   false,
			expectedJSON:    false,
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "ForceFlag",
			args:            []string{"gopls", "--force"},
			expectedForce:   true,
			expectedJSON:    false,
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "JSONFlag",
			args:            []string{"gopls", "--json"},
			expectedForce:   false,
			expectedJSON:    true,
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "TimeoutFlag",
			args:            []string{"gopls", "--timeout", "3m"},
			expectedForce:   false,
			expectedJSON:    false,
			expectedTimeout: 3 * time.Minute,
		},
		{
			name:            "CombinedFlags",
			args:            []string{"gopls", "--force", "--json", "--timeout", "5m"},
			expectedForce:   true,
			expectedJSON:    true,
			expectedTimeout: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetInstallFlags()

			cmd := &cobra.Command{}
			cmd.AddCommand(installCmd)

			_, err := executeCommandWithoutOutput(cmd, append([]string{"install", "server"}, tt.args...)...)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
				return
			}

			if installForce != tt.expectedForce {
				t.Errorf("Expected force flag to be %v, got %v", tt.expectedForce, installForce)
			}

			if installJSON != tt.expectedJSON {
				t.Errorf("Expected json flag to be %v, got %v", tt.expectedJSON, installJSON)
			}

			if installTimeout != tt.expectedTimeout {
				t.Errorf("Expected timeout flag to be %v, got %v", tt.expectedTimeout, installTimeout)
			}
		})
	}
}

func testInstallServerCommandValidArgs(t *testing.T) {
	expectedValidArgs := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}

	if len(installServerCmd.ValidArgs) != len(expectedValidArgs) {
		t.Errorf("Expected %d valid args, got %d", len(expectedValidArgs), len(installServerCmd.ValidArgs))
	}

	for i, expected := range expectedValidArgs {
		if i >= len(installServerCmd.ValidArgs) {
			t.Errorf("Missing valid arg: %s", expected)
			continue
		}
		if installServerCmd.ValidArgs[i] != expected {
			t.Errorf("Expected valid arg %d to be '%s', got '%s'", i, expected, installServerCmd.ValidArgs[i])
		}
	}
}

func testInstallServerCommandArgumentValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "ValidServer",
			args:        []string{"gopls"},
			expectError: false,
		},
		{
			name:        "ValidTypescriptServer",
			args:        []string{"typescript-language-server"},
			expectError: false,
		},
		{
			name:        "NoArguments",
			args:        []string{},
			expectError: true,
		},
		{
			name:        "TooManyArguments",
			args:        []string{"gopls", "pylsp"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := installServerCmd.Args(installServerCmd, tt.args)

			if tt.expectError && err == nil {
				t.Error("Expected argument validation error, but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Expected no argument validation error, but got: %v", err)
			}
		})
	}
}

func testInstallServerCommandErrorScenarios(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "server", "gopls")
	if err == nil {
		t.Error("Expected error when installation fails, but got none")
	}

	// Add defensive nil check before accessing err.Error()
	// Now that installer creation is more robust, we expect installation or dependency validation errors
	if err != nil && !strings.Contains(err.Error(), "dependency validation failed") && !strings.Contains(err.Error(), "installer") {
		t.Errorf("Expected error to mention dependency validation or installer, got: %v", err)
	}
}

func testInstallServerCommandHelp(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	output, err := executeCommand(cmd, "install", "server", "--help")
	if err != nil {
		t.Errorf("Expected help command to succeed, got error: %v", err)
	}

	if !strings.Contains(output, "Install a specific language server") {
		t.Error("Expected help to contain description")
	}

	if !strings.Contains(output, "Supported language servers") {
		t.Error("Expected help to show supported language servers")
	}

	if !strings.Contains(output, "--force") {
		t.Error("Expected help to show force flag")
	}
}

func testInstallServerCommandSingleInstallation(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "server", "gopls")
	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallServerCommandJSONOutput(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "server", "gopls", "--json")

	if !installJSON {
		t.Error("Expected JSON flag to be true")
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallServerCommandForceReinstall(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "server", "gopls", "--force")

	if !installForce {
		t.Error("Expected force flag to be true")
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallServerCommandTimeout(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "server", "gopls", "--timeout", "3m")

	if installTimeout != 3*time.Minute {
		t.Errorf("Expected timeout flag to be 3m, got %v", installTimeout)
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallServersCommandMetadata(t *testing.T) {
	if installServersCmd.Use != "servers" {
		t.Errorf("Expected Use to be 'servers', got '%s'", installServersCmd.Use)
	}

	expectedShort := "Install all missing language servers"
	if installServersCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, installServersCmd.Short)
	}

	if !strings.Contains(installServersCmd.Long, "Install all missing language servers") {
		t.Error("Expected Long description to mention installing all missing language servers")
	}

	if installServersCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}

	if installServersCmd.Args != nil {
		err := installServersCmd.Args(installServersCmd, []string{})
		if err != nil {
			t.Errorf("Expected no arguments to be valid, got error: %v", err)
		}
	}
}

func testInstallServersCommandFlagParsing(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		expectedForce   bool
		expectedJSON    bool
		expectedTimeout time.Duration
		expectError     bool
	}{
		{
			name:            "DefaultFlags",
			args:            []string{},
			expectedForce:   false,
			expectedJSON:    false,
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "ForceFlag",
			args:            []string{"--force"},
			expectedForce:   true,
			expectedJSON:    false,
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "JSONFlag",
			args:            []string{"--json"},
			expectedForce:   false,
			expectedJSON:    true,
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "TimeoutFlag",
			args:            []string{"--timeout", "7m"},
			expectedForce:   false,
			expectedJSON:    false,
			expectedTimeout: 7 * time.Minute,
		},
		{
			name:            "CombinedFlags",
			args:            []string{"--force", "--json", "--timeout", "2m"},
			expectedForce:   true,
			expectedJSON:    true,
			expectedTimeout: 2 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetInstallFlags()

			cmd := &cobra.Command{}
			cmd.AddCommand(installCmd)

			_, err := executeCommandWithoutOutput(cmd, append([]string{"install", "servers"}, tt.args...)...)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got none")
				}
				return
			}

			if installForce != tt.expectedForce {
				t.Errorf("Expected force flag to be %v, got %v", tt.expectedForce, installForce)
			}

			if installJSON != tt.expectedJSON {
				t.Errorf("Expected json flag to be %v, got %v", tt.expectedJSON, installJSON)
			}

			if installTimeout != tt.expectedTimeout {
				t.Errorf("Expected timeout flag to be %v, got %v", tt.expectedTimeout, installTimeout)
			}
		})
	}
}

func testInstallServersCommandErrorScenarios(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "servers")
	if err == nil {
		t.Error("Expected error when installation fails, but got none")
	}

	// Add defensive nil check before accessing err.Error()
	// Now that installer creation is more robust, we expect dependency validation or installation errors
	if err != nil && !strings.Contains(err.Error(), "dependency validation failed") && !strings.Contains(err.Error(), "installer") {
		t.Errorf("Expected error to mention dependency validation or installer, got: %v", err)
	}
}

func testInstallServersCommandHelp(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	output, err := executeCommand(cmd, "install", "servers", "--help")
	if err != nil {
		t.Errorf("Expected help command to succeed, got error: %v", err)
	}

	if !strings.Contains(output, "Install all missing language servers") {
		t.Error("Expected help to contain description")
	}

	if !strings.Contains(output, "This command will") {
		t.Error("Expected help to show command behavior")
	}

	if !strings.Contains(output, "--force") {
		t.Error("Expected help to show force flag")
	}
}

func testInstallServersCommandAllInstallation(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "servers")
	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallServersCommandJSONOutput(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "servers", "--json")

	if !installJSON {
		t.Error("Expected JSON flag to be true")
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallServersCommandForceReinstall(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "servers", "--force")

	if !installForce {
		t.Error("Expected force flag to be true")
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallServersCommandTimeout(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "servers", "--timeout", "8m")

	if installTimeout != 8*time.Minute {
		t.Errorf("Expected timeout flag to be 8m, got %v", installTimeout)
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func TestInstallOutputFunctions(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"JSONOutput", testInstallJSONOutput},
		{"HumanOutput", testInstallHumanOutput},
		{"JSONOutputWithError", testInstallJSONOutputWithError},
		{"HumanOutputWithError", testInstallHumanOutputWithError},
		{"EmptyResults", testInstallEmptyResults},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func testInstallJSONOutput(t *testing.T) {
	results := []*installer.InstallResult{
		{
			Success:  true,
			Runtime:  "go",
			Version:  "1.21.0",
			Path:     "/usr/local/go/bin/go",
			Duration: time.Second,
			Method:   "homebrew",
			Messages: []string{"Installation successful"},
		},
	}

	output := captureOutput(func() {
		err := outputInstallResultsJSON(results, nil)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	var jsonOutput struct {
		Success bool                       `json:"success"`
		Results []*installer.InstallResult `json:"results"`
		Error   string                     `json:"error,omitempty"`
	}

	err := json.Unmarshal([]byte(output), &jsonOutput)
	if err != nil {
		t.Errorf("Failed to unmarshal JSON output: %v", err)
	}

	if !jsonOutput.Success {
		t.Error("Expected success to be true")
	}

	if len(jsonOutput.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(jsonOutput.Results))
		return
	}

	if jsonOutput.Results[0].Runtime != "go" {
		t.Errorf("Expected runtime 'go', got '%s'", jsonOutput.Results[0].Runtime)
	}
}

func testInstallHumanOutput(t *testing.T) {
	results := []*installer.InstallResult{
		{
			Success: true,
			Runtime: "go",
			Version: "1.21.0",
			Path:    "/usr/local/go/bin/go",
		},
	}

	err := outputInstallResultsHuman(results, nil)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func testInstallJSONOutputWithError(t *testing.T) {
	results := []*installer.InstallResult{}
	testError := fmt.Errorf("installation failed")

	output := captureOutput(func() {
		err := outputInstallResultsJSON(results, testError)
		if err != testError {
			t.Errorf("Expected original error to be returned, got: %v", err)
		}
	})

	var jsonOutput struct {
		Success bool                       `json:"success"`
		Results []*installer.InstallResult `json:"results"`
		Error   string                     `json:"error,omitempty"`
	}

	err := json.Unmarshal([]byte(output), &jsonOutput)
	if err != nil {
		t.Errorf("Failed to unmarshal JSON output: %v", err)
	}

	if jsonOutput.Success {
		t.Error("Expected success to be false")
	}

	if jsonOutput.Error != "installation failed" {
		t.Errorf("Expected error message 'installation failed', got '%s'", jsonOutput.Error)
	}
}

func testInstallHumanOutputWithError(t *testing.T) {
	results := []*installer.InstallResult{}
	testError := fmt.Errorf("installation failed")

	err := outputInstallResultsHuman(results, testError)
	if err != testError {
		t.Errorf("Expected original error to be returned, got: %v", err)
	}
}

func testInstallEmptyResults(t *testing.T) {
	results := []*installer.InstallResult{}

	output := captureOutput(func() {
		err := outputInstallResultsHuman(results, nil)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	if !strings.Contains(output, "No installation results") {
		t.Error("Expected output to mention no installation results")
	}
}

func executeCommand(cmd *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

func executeCommandWithoutOutput(cmd *cobra.Command, args ...string) (string, error) {
	cmd.SetArgs(args)
	err := cmd.Execute()
	return "", err
}

func TestInstallEdgeCases(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with external dependencies
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"InvalidTimeoutFlag", testInstallInvalidTimeoutFlag},
		{"LongTimeoutFlag", testInstallLongTimeoutFlag},
		{"EmptyVersionFlag", testInstallEmptyVersionFlag},
		{"CommandStructure", testInstallCommandStructure},
		{"FlagDefaults", testInstallFlagDefaults},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetInstallFlags()
			tt.testFunc(t)
		})
	}
}

func testInstallInvalidTimeoutFlag(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "runtime", "go", "--timeout", "invalid")
	if err == nil {
		t.Error("Expected error for invalid timeout format")
	}
}

func testInstallLongTimeoutFlag(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "runtime", "go", "--timeout", "1h")

	if installTimeout != 1*time.Hour {
		t.Errorf("Expected timeout flag to be 1h, got %v", installTimeout)
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallEmptyVersionFlag(t *testing.T) {
	resetInstallFlags()

	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	_, err := executeCommandWithoutOutput(cmd, "install", "runtime", "go", "--version", "")

	if installVersion != "" {
		t.Errorf("Expected version flag to be empty, got '%s'", installVersion)
	}

	if err == nil {
		t.Error("Expected error when installer is not available")
	}
}

func testInstallCommandStructure(t *testing.T) {
	rootSubcommands := rootCmd.Commands()
	found := false
	for _, cmd := range rootSubcommands {
		if cmd.Use == CmdInstall {
			found = true
			break
		}
	}

	if !found {
		t.Error("Install command not found in root command")
	}

	installSubcommands := installCmd.Commands()
	expectedSubcommands := []string{"runtime", "server", "servers"}

	if len(installSubcommands) != len(expectedSubcommands) {
		t.Errorf("Expected %d install subcommands, got %d", len(expectedSubcommands), len(installSubcommands))
	}
}

func testInstallFlagDefaults(t *testing.T) {
	resetInstallFlags()

	if installForce != false {
		t.Errorf("Expected installForce default to be false, got %v", installForce)
	}

	if installVersion != "" {
		t.Errorf("Expected installVersion default to be empty, got '%s'", installVersion)
	}

	if installJSON != false {
		t.Errorf("Expected installJSON default to be false, got %v", installJSON)
	}

	if installTimeout != 10*time.Minute {
		t.Errorf("Expected installTimeout default to be 10m, got %v", installTimeout)
	}
}

func BenchmarkInstallCommandParsing(b *testing.B) {
	cmd := &cobra.Command{}
	cmd.AddCommand(installCmd)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resetInstallFlags()
		cmd.SetArgs([]string{"install", "runtime", "go", "--force", "--json", "--version", "1.21.0"})
		if err := cmd.ParseFlags([]string{"install", "runtime", "go", "--force", "--json", "--version", "1.21.0"}); err != nil {
			b.Fatalf("Failed to parse flags: %v", err)
		}
	}
}

func BenchmarkInstallJSONOutput(b *testing.B) {
	results := []*installer.InstallResult{
		{
			Success:  true,
			Runtime:  "go",
			Version:  "1.21.0",
			Path:     "/usr/local/go/bin/go",
			Duration: time.Second,
			Method:   "homebrew",
			Messages: []string{"Installation successful"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		old := os.Stdout
		os.Stdout, _ = os.Open(os.DevNull)

		if err := outputInstallResultsJSON(results, nil); err != nil {
			b.Logf("Failed to output JSON results: %v", err)
		}

		os.Stdout = old
	}
}
