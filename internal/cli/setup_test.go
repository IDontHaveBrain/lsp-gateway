package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestSetupCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testSetupCommandMetadata},
		{"SubCommands", testSetupSubCommands},
		{"FlagParsing", testSetupCommandFlagParsing},
		{"AllSubcommand", testSetupAllCommand},
		{"WizardSubcommand", testSetupWizardCommand},
		{"InteractiveMode", testSetupInteractiveMode},
		{"Help", testSetupCommandHelp},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func testSetupCommandMetadata(t *testing.T) {
	cmd := setupCmd

	if cmd.Use != CmdSetup {
		t.Errorf("Expected Use to be %q, got %q", CmdSetup, cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	if cmd.Long == "" {
		t.Error("Expected Long description to be set")
	}

	if cmd.RunE == nil {
		t.Error("Expected RunE to be set for setup command")
	}
}

func testSetupSubCommands(t *testing.T) {
	cmd := setupCmd

	expectedSubcommands := []string{"all", "wizard"}
	foundSubcommands := make(map[string]bool)

	for _, sub := range cmd.Commands() {
		foundSubcommands[sub.Use] = true
	}

	for _, expected := range expectedSubcommands {
		if !foundSubcommands[expected] {
			t.Errorf("Expected subcommand %q not found", expected)
		}
	}

	if len(foundSubcommands) != len(expectedSubcommands) {
		t.Errorf("Found %d subcommands, expected %d", len(foundSubcommands), len(expectedSubcommands))
	}
}

func testSetupCommandFlagParsing(t *testing.T) {
	testCases := []struct {
		name       string
		args       []string
		wantForce  bool
		wantConfig string
		wantErr    bool
	}{
		{
			name:       "default flags",
			args:       []string{},
			wantForce:  false,
			wantConfig: DefaultConfigFile,
			wantErr:    false,
		},
		{
			name:       "force flag",
			args:       []string{"--force"},
			wantForce:  true,
			wantConfig: DefaultConfigFile,
			wantErr:    false,
		},
		{
			name:       "custom config",
			args:       []string{"--config", "/custom/path/config.yaml"},
			wantForce:  false,
			wantConfig: "/custom/path/config.yaml",
			wantErr:    false,
		},
		{
			name:       "skip runtimes",
			args:       []string{"--skip-runtimes", "python,java"},
			wantForce:  false,
			wantConfig: DefaultConfigFile,
			wantErr:    false,
		},
		{
			name:       "skip servers",
			args:       []string{"--skip-servers", "jdtls,pylsp"},
			wantForce:  false,
			wantConfig: DefaultConfigFile,
			wantErr:    false,
		},
		{
			name:       "timeout flag",
			args:       []string{"--timeout", "5m"},
			wantForce:  false,
			wantConfig: DefaultConfigFile,
			wantErr:    false,
		},
		{
			name:       "invalid flag",
			args:       []string{"--invalid-flag"},
			wantForce:  false,
			wantConfig: DefaultConfigFile,
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupForce = false
			setupConfigPath = DefaultConfigFile
			setupTimeout = 10 * time.Minute
			setupSkipRuntimes = []string{}
			setupSkipServers = []string{}

			cmd := &cobra.Command{}
			cmd.PersistentFlags().BoolVar(&setupForce, "force", false, "Force reinstallation")
			cmd.PersistentFlags().StringVarP(&setupConfigPath, "config", "c", DefaultConfigFile, "Config file path")
			cmd.PersistentFlags().DurationVar(&setupTimeout, "timeout", 10*time.Minute, "Setup timeout")
			cmd.PersistentFlags().StringSliceVar(&setupSkipRuntimes, "skip-runtimes", []string{}, "Runtimes to skip")
			cmd.PersistentFlags().StringSliceVar(&setupSkipServers, "skip-servers", []string{}, "Servers to skip")

			err := cmd.ParseFlags(tc.args)

			if tc.wantErr && err == nil {
				t.Error("Expected error but got none")
			} else if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tc.wantErr {
				if setupForce != tc.wantForce {
					t.Errorf("Expected force=%v, got %v", tc.wantForce, setupForce)
				}
				if setupConfigPath != tc.wantConfig {
					t.Errorf("Expected config=%q, got %q", tc.wantConfig, setupConfigPath)
				}
			}
		})
	}
}

func testSetupAllCommand(t *testing.T) {
	testCases := []struct {
		name           string
		args           []string
		skipRuntimes   []string
		skipServers    []string
		expectedOutput []string
	}{
		{
			name:         "basic execution",
			args:         []string{},
			skipRuntimes: []string{},
			skipServers:  []string{},
			expectedOutput: []string{
				"Starting complete automated setup",
				"Setup command structure implemented",
				"Configuration path:",
			},
		},
		{
			name:         "with skip runtimes",
			args:         []string{},
			skipRuntimes: []string{"python", "java"},
			skipServers:  []string{},
			expectedOutput: []string{
				"Skipping runtimes: [python java]",
				"Would skip runtimes: [python java]",
			},
		},
		{
			name:         "with skip servers",
			args:         []string{},
			skipRuntimes: []string{},
			skipServers:  []string{"jdtls", "pylsp"},
			expectedOutput: []string{
				"Skipping servers: [jdtls pylsp]",
				"Would skip servers: [jdtls pylsp]",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupSkipRuntimes = tc.skipRuntimes
			setupSkipServers = tc.skipServers
			setupConfigPath = "config.yaml"

			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := setupAll(cmd, tc.args)
			if err != nil {
				t.Fatalf("setupAll failed: %v", err)
			}

			output := buf.String()
			for _, expected := range tc.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got:\n%s", expected, output)
				}
			}
		})
	}
}

func testSetupWizardCommand(t *testing.T) {
	testCases := []struct {
		name           string
		args           []string
		interactive    bool
		expectedOutput []string
	}{
		{
			name:        "interactive mode",
			args:        []string{},
			interactive: true,
			expectedOutput: []string{
				"LSP Gateway Setup Wizard",
				"Welcome to the LSP Gateway setup wizard",
				"The wizard will:",
				"Setup wizard command structure implemented",
			},
		},
		{
			name:        "non-interactive mode",
			args:        []string{},
			interactive: false,
			expectedOutput: []string{
				"LSP Gateway Setup Wizard",
				"Welcome to the LSP Gateway setup wizard",
				"Setup wizard command structure implemented",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupInteractive = tc.interactive
			setupConfigPath = "config.yaml"

			var buf bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := setupWizard(cmd, tc.args)
			if err != nil {
				t.Fatalf("setupWizard failed: %v", err)
			}

			output := buf.String()
			for _, expected := range tc.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, got:\n%s", expected, output)
				}
			}

			if !tc.interactive {
				if strings.Contains(output, "Press Enter to continue") {
					t.Error("Non-interactive mode should not show prompts")
				}
			}
		})
	}
}

func testSetupInteractiveMode(t *testing.T) {
	cmd := setupWizardCmd

	flagFound := false
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "no-interactive" {
			flagFound = true
		}
	})

	if !flagFound {
		t.Error("Expected 'no-interactive' flag to be defined on wizard command")
	}
}

func testSetupCommandHelp(t *testing.T) {
	cmd := setupCmd

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Help()
	if err != nil {
		t.Fatalf("Failed to get help: %v", err)
	}

	helpOutput := buf.String()

	expectedSections := []string{
		"Setup and configure LSP Gateway",
		"Available subcommands:",
		"Examples:",
		"Flags:",
		"--force",
		"--config",
		"--timeout",
		"--skip-runtimes",
		"--skip-servers",
	}

	for _, section := range expectedSections {
		if !strings.Contains(helpOutput, section) {
			t.Errorf("Expected help to contain %q", section)
		}
	}
}

func BenchmarkSetupAll(b *testing.B) {
	cmd := &cobra.Command{}
	args := []string{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = setupAll(cmd, args)
	}
}

func BenchmarkSetupWizard(b *testing.B) {
	cmd := &cobra.Command{}
	args := []string{}
	setupInteractive = false // Disable interactive mode for benchmarking

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = setupWizard(cmd, args)
	}
}
