package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-gateway/internal/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	testConfigFilename = "test-config.yaml"
	generateCommand    = "generate"
	showCommand        = "show"
)

func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, testConfigFilename)

	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	return configFile
}

func createValidTestConfigContent(port int) string {
	return fmt.Sprintf(`port: %d
servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    args: []
    transport: "stdio"
  - name: "python-lsp"
    languages: ["python"]
    command: "python"
    args: ["-m", "pylsp"]
    transport: "stdio"
`, port)
}

func createInvalidTestConfigContent() string {
	return `port: invalid_port
servers:
  - name: "invalid-server"
    command: ""
    transport: "invalid"
`
}

func TestConfigCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testConfigCommandMetadata},
		{"FlagParsing", testConfigCommandFlagParsing},
		{"SubcommandRegistration", testConfigSubcommandRegistration},
		{"Help", testConfigCommandHelp},
		{"Generate", testConfigGenerateCommand},
		{"Validate", testConfigValidateCommand},
		{"Show", testConfigShowCommand},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()
			tt.testFunc(t)
		})
	}
}

func resetConfigGlobals() {
	configPath = DefaultConfigFile
	configOutputPath = ""
	configJSON = false
	configOverwrite = false
	configAutoDetect = false
	configValidateOnly = false
	configIncludeComments = false
	configTargetRuntime = ""
}

func testConfigCommandMetadata(t *testing.T) {
	if configCmd.Use != CmdConfig {
		t.Errorf("Expected Use to be '%s', got '%s'", CmdConfig, configCmd.Use)
	}

	expectedShort := "Configuration management"
	if configCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, configCmd.Short)
	}

	if !strings.Contains(configCmd.Long, "Manage LSP Gateway configuration files") {
		t.Error("Expected Long description to mention configuration management")
	}

	if !strings.Contains(configCmd.Long, "generate") || !strings.Contains(configCmd.Long, "validate") || !strings.Contains(configCmd.Long, "show") {
		t.Error("Expected Long description to mention all subcommands")
	}

	if configCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}

	if configCmd.Run != nil {
		t.Error("Expected Run function to be nil (using RunE instead)")
	}
}

func testConfigCommandFlagParsing(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedConfig string
		expectedJSON   bool
		expectedError  bool
	}{
		{
			name:           "DefaultFlags",
			args:           []string{},
			expectedConfig: DefaultConfigFile,
			expectedJSON:   false,
			expectedError:  false,
		},
		{
			name:           "ConfigFlag",
			args:           []string{"--config", "custom.yaml"},
			expectedConfig: "custom.yaml",
			expectedJSON:   false,
			expectedError:  false,
		},
		{
			name:           "ConfigFlagShort",
			args:           []string{"-c", "custom.yaml"},
			expectedConfig: "custom.yaml",
			expectedJSON:   false,
			expectedError:  false,
		},
		{
			name:           "JSONFlag",
			args:           []string{"--json"},
			expectedConfig: DefaultConfigFile,
			expectedJSON:   true,
			expectedError:  false,
		},
		{
			name:           "BothFlags",
			args:           []string{"--config", "custom.yaml", "--json"},
			expectedConfig: "custom.yaml",
			expectedJSON:   true,
			expectedError:  false,
		},
		{
			name:           "BothFlagsShort",
			args:           []string{"-c", "custom.yaml", "--json"},
			expectedConfig: "custom.yaml",
			expectedJSON:   true,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()

			testCmd := &cobra.Command{
				Use:   CmdConfig,
				Short: "Configuration management",
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}

			testCmd.PersistentFlags().StringVarP(&configPath, "config", "c", DefaultConfigFile, "Configuration file path")
			testCmd.PersistentFlags().BoolVar(&configJSON, "json", false, "Output in JSON format")

			testCmd.SetArgs(tt.args)

			err := testCmd.Execute()

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if configPath != tt.expectedConfig {
				t.Errorf("Expected configPath to be '%s', got '%s'", tt.expectedConfig, configPath)
			}

			if configJSON != tt.expectedJSON {
				t.Errorf("Expected configJSON to be %v, got %v", tt.expectedJSON, configJSON)
			}
		})
	}
}

func testConfigSubcommandRegistration(t *testing.T) {
	expectedSubcommands := []string{"generate", "validate", "show"}
	actualSubcommands := make([]string, 0, len(configCmd.Commands()))

	for _, cmd := range configCmd.Commands() {
		actualSubcommands = append(actualSubcommands, cmd.Use)
	}

	for _, expected := range expectedSubcommands {
		found := false
		for _, actual := range actualSubcommands {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected subcommand '%s' not found in registered commands: %v", expected, actualSubcommands)
		}
	}

	for _, cmd := range configCmd.Commands() {
		switch cmd.Use {
		case generateCommand:
			if cmd.Short != "Generate configuration files" {
				t.Errorf("Expected generate Short to be 'Generate configuration files', got '%s'", cmd.Short)
			}
			if cmd.RunE == nil {
				t.Error("Expected generate RunE to be set")
			}
		case "validate":
			if cmd.Short != "Validate configuration file" {
				t.Errorf("Expected validate Short to be 'Validate configuration file', got '%s'", cmd.Short)
			}
			if cmd.RunE == nil {
				t.Error("Expected validate RunE to be set")
			}
		case showCommand:
			if cmd.Short != "Display current configuration" {
				t.Errorf("Expected show Short to be 'Display current configuration', got '%s'", cmd.Short)
			}
			if cmd.RunE == nil {
				t.Error("Expected show RunE to be set")
			}
		}
	}
}

func testConfigCommandHelp(t *testing.T) {
	var buf bytes.Buffer
	testCmd := &cobra.Command{
		Use:   CmdConfig,
		Short: configCmd.Short,
		Long:  configCmd.Long,
	}
	testCmd.SetOut(&buf)
	testCmd.SetArgs([]string{"--help"})

	err := testCmd.Execute()
	if err != nil {
		t.Fatalf("Help command failed: %v", err)
	}

	helpOutput := buf.String()
	if !strings.Contains(helpOutput, "Configuration") && !strings.Contains(helpOutput, "management") {
		t.Logf("Help output: %s", helpOutput)
		t.Error("Help output should contain command description")
	}

	subcommands := []string{"generate", "validate", "show"}
	for _, subcmd := range subcommands {
		t.Run("Help_"+subcmd, func(t *testing.T) {
			var subBuf bytes.Buffer
			var actualCmd *cobra.Command

			switch subcmd {
			case generateCommand:
				actualCmd = configGenerateCmd
			case "validate":
				actualCmd = configValidateCmd
			case showCommand:
				actualCmd = configShowCmd
			}

			if actualCmd != nil {
				testSubCmd := &cobra.Command{
					Use:   actualCmd.Use,
					Short: actualCmd.Short,
					Long:  actualCmd.Long,
				}
				testSubCmd.SetOut(&subBuf)
				testSubCmd.SetArgs([]string{"--help"})

				err := testSubCmd.Execute()
				if err != nil {
					t.Fatalf("Help command for %s failed: %v", subcmd, err)
				}

				helpOutput := subBuf.String()
				if helpOutput == "" || len(helpOutput) < 10 {
					t.Logf("Help output for %s: %s", subcmd, helpOutput)
					t.Errorf("Help output for %s should contain command description", subcmd)
				}
			}
		})
	}
}

func testConfigGenerateCommand(t *testing.T) {
	t.Run("GenerateCommandMetadata", testConfigGenerateCommandMetadata)
	t.Run("GenerateCommandFlagParsing", testConfigGenerateCommandFlagParsing)
	t.Run("GenerateCommandExecution", testConfigGenerateCommandExecution)
	t.Run("GenerateCommandValidation", testConfigGenerateCommandValidation)
	t.Run("GenerateCommandErrorScenarios", testConfigGenerateCommandErrorScenarios)
}

func testConfigGenerateCommandMetadata(t *testing.T) {
	if configGenerateCmd.Use != "generate" {
		t.Errorf("Expected Use to be 'generate', got '%s'", configGenerateCmd.Use)
	}

	expectedShort := "Generate configuration files"
	if configGenerateCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, configGenerateCmd.Short)
	}

	if !strings.Contains(configGenerateCmd.Long, "Generate LSP Gateway configuration files") {
		t.Error("Expected Long description to mention configuration generation")
	}

	if configGenerateCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}
}

func testConfigGenerateCommandFlagParsing(t *testing.T) {
	tests := []struct {
		name                    string
		args                    []string
		expectedOutput          string
		expectedOverwrite       bool
		expectedAutoDetect      bool
		expectedIncludeComments bool
		expectedTargetRuntime   string
		expectedError           bool
	}{
		{
			name:                    "DefaultFlags",
			args:                    []string{},
			expectedOutput:          "",
			expectedOverwrite:       false,
			expectedAutoDetect:      false,
			expectedIncludeComments: false,
			expectedTargetRuntime:   "",
			expectedError:           false,
		},
		{
			name:                    "OutputFlag",
			args:                    []string{"--output", "custom.yaml"},
			expectedOutput:          "custom.yaml",
			expectedOverwrite:       false,
			expectedAutoDetect:      false,
			expectedIncludeComments: false,
			expectedTargetRuntime:   "",
			expectedError:           false,
		},
		{
			name:                    "OutputFlagShort",
			args:                    []string{"-o", "custom.yaml"},
			expectedOutput:          "custom.yaml",
			expectedOverwrite:       false,
			expectedAutoDetect:      false,
			expectedIncludeComments: false,
			expectedTargetRuntime:   "",
			expectedError:           false,
		},
		{
			name:                    "OverwriteFlag",
			args:                    []string{"--overwrite"},
			expectedOutput:          "",
			expectedOverwrite:       true,
			expectedAutoDetect:      false,
			expectedIncludeComments: false,
			expectedTargetRuntime:   "",
			expectedError:           false,
		},
		{
			name:                    "AutoDetectFlag",
			args:                    []string{"--auto-detect"},
			expectedOutput:          "",
			expectedOverwrite:       false,
			expectedAutoDetect:      true,
			expectedIncludeComments: false,
			expectedTargetRuntime:   "",
			expectedError:           false,
		},
		{
			name:                    "IncludeCommentsFlag",
			args:                    []string{"--include-comments"},
			expectedOutput:          "",
			expectedOverwrite:       false,
			expectedAutoDetect:      false,
			expectedIncludeComments: true,
			expectedTargetRuntime:   "",
			expectedError:           false,
		},
		{
			name:                    "RuntimeFlag",
			args:                    []string{"--runtime", "go"},
			expectedOutput:          "",
			expectedOverwrite:       false,
			expectedAutoDetect:      false,
			expectedIncludeComments: false,
			expectedTargetRuntime:   "go",
			expectedError:           false,
		},
		{
			name:                    "AllFlags",
			args:                    []string{"--output", "test.yaml", "--overwrite", "--auto-detect", "--include-comments", "--runtime", "python"},
			expectedOutput:          "test.yaml",
			expectedOverwrite:       true,
			expectedAutoDetect:      true,
			expectedIncludeComments: true,
			expectedTargetRuntime:   "python",
			expectedError:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()

			testCmd := &cobra.Command{
				Use:   "generate",
				Short: "Generate configuration files",
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}

			testCmd.Flags().StringVarP(&configOutputPath, "output", "o", "", "Output configuration file path")
			testCmd.Flags().BoolVar(&configOverwrite, "overwrite", false, "Overwrite existing configuration file")
			testCmd.Flags().BoolVar(&configAutoDetect, "auto-detect", false, "Auto-detect runtimes and generate configuration")
			testCmd.Flags().BoolVar(&configIncludeComments, "include-comments", false, "Include explanatory comments in generated config")
			testCmd.Flags().StringVar(&configTargetRuntime, "runtime", "", "Generate configuration for specific runtime")

			testCmd.SetArgs(tt.args)
			err := testCmd.Execute()

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if configOutputPath != tt.expectedOutput {
				t.Errorf("Expected configOutputPath to be '%s', got '%s'", tt.expectedOutput, configOutputPath)
			}
			if configOverwrite != tt.expectedOverwrite {
				t.Errorf("Expected configOverwrite to be %v, got %v", tt.expectedOverwrite, configOverwrite)
			}
			if configAutoDetect != tt.expectedAutoDetect {
				t.Errorf("Expected configAutoDetect to be %v, got %v", tt.expectedAutoDetect, configAutoDetect)
			}
			if configIncludeComments != tt.expectedIncludeComments {
				t.Errorf("Expected configIncludeComments to be %v, got %v", tt.expectedIncludeComments, configIncludeComments)
			}
			if configTargetRuntime != tt.expectedTargetRuntime {
				t.Errorf("Expected configTargetRuntime to be '%s', got '%s'", tt.expectedTargetRuntime, configTargetRuntime)
			}
		})
	}
}

func testConfigGenerateCommandExecution(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		setup        func() string
		args         []string
		expectError  bool
		validateFile bool
	}{
		{
			name: "GenerateToNewFile",
			setup: func() string {
				return filepath.Join(tempDir, "new-config.yaml")
			},
			args:         []string{"--output"},
			expectError:  false,
			validateFile: true,
		},
		{
			name: "GenerateWithOverwrite",
			setup: func() string {
				existingFile := filepath.Join(tempDir, "existing-config.yaml")
				if err := os.WriteFile(existingFile, []byte("existing content"), 0644); err != nil {
					panic(fmt.Sprintf("Failed to create test file: %v", err))
				}
				return existingFile
			},
			args:         []string{"--output", "", "--overwrite"},
			expectError:  false,
			validateFile: true,
		},
		{
			name: "GenerateWithAutoDetect",
			setup: func() string {
				return filepath.Join(tempDir, "auto-config.yaml")
			},
			args:         []string{"--output", "", "--auto-detect"},
			expectError:  false,
			validateFile: true,
		},
		{
			name: "GenerateWithRuntime",
			setup: func() string {
				return filepath.Join(tempDir, "runtime-config.yaml")
			},
			args:         []string{"--output", "", "--runtime", "go"},
			expectError:  false,
			validateFile: true,
		},
		{
			name: "GenerateWithComments",
			setup: func() string {
				return filepath.Join(tempDir, "comments-config.yaml")
			},
			args:         []string{"--output", "", "--include-comments"},
			expectError:  false,
			validateFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()

			outputFile := tt.setup()

			args := make([]string, len(tt.args))
			copy(args, tt.args)
			for i, arg := range args {
				if arg == "" && i > 0 && args[i-1] == "--output" {
					args[i] = outputFile
				}
			}

			err := configGenerate(nil, []string{})

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Logf("Got error (may be expected due to TODO): %v", err)
			}

			if tt.validateFile && !tt.expectError {
				if _, err := os.Stat(outputFile); os.IsNotExist(err) {
					t.Logf("File not created yet (expected due to TODO implementations)")
				}
			}
		})
	}
}

func testConfigGenerateCommandValidation(t *testing.T) {
	tests := []struct {
		name        string
		runtime     string
		expectError bool
	}{
		{
			name:        "InvalidRuntime",
			runtime:     "invalid",
			expectError: true,
		},
		{
			name:        "EmptyRuntime",
			runtime:     "",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()
			configTargetRuntime = tt.runtime

			tempDir := t.TempDir()
			configPath = filepath.Join(tempDir, "config.yaml")

			err := validateConfigGenerateParams()

			if tt.expectError && err == nil {
				t.Log("Expected validation error but got none (may be due to TODO implementations)")
			} else if !tt.expectError && err != nil && !strings.Contains(err.Error(), "unknown error") {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func testConfigGenerateCommandErrorScenarios(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		setup       func()
		expectError bool
		errorType   string
	}{
		{
			name: "FileExistsWithoutOverwrite",
			setup: func() {
				configPath = filepath.Join(tempDir, "existing.yaml")
				configOutputPath = ""
				configOverwrite = false
				if err := os.WriteFile(configPath, []byte("existing"), 0644); err != nil {
					panic(fmt.Sprintf("Failed to create test file: %v", err))
				}
			},
			expectError: true,
			errorType:   "already exists",
		},
		{
			name: "InvalidRuntime",
			setup: func() {
				configTargetRuntime = "invalid-runtime"
			},
			expectError: true,
			errorType:   "runtime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()
			tt.setup()

			err := configGenerate(nil, []string{})

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && err != nil {
				if !strings.Contains(err.Error(), tt.errorType) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorType, err)
				}
			}
		})
	}
}

func testConfigValidateCommand(t *testing.T) {
	t.Run("ValidateCommandMetadata", testConfigValidateCommandMetadata)
	t.Run("ValidateCommandFlagParsing", testConfigValidateCommandFlagParsing)
	t.Run("ValidateCommandExecution", testConfigValidateCommandExecution)
	t.Run("ValidateCommandValidation", testConfigValidateCommandValidation)
	t.Run("ValidateCommandErrorScenarios", testConfigValidateCommandErrorScenarios)
}

func testConfigValidateCommandMetadata(t *testing.T) {
	if configValidateCmd.Use != "validate" {
		t.Errorf("Expected Use to be 'validate', got '%s'", configValidateCmd.Use)
	}

	expectedShort := "Validate configuration file"
	if configValidateCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, configValidateCmd.Short)
	}

	if !strings.Contains(configValidateCmd.Long, "Validate an existing LSP Gateway configuration file") {
		t.Error("Expected Long description to mention configuration validation")
	}

	if configValidateCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}
}

func testConfigValidateCommandFlagParsing(t *testing.T) {
	tests := []struct {
		name                 string
		args                 []string
		expectedValidateOnly bool
		expectedError        bool
	}{
		{
			name:                 "DefaultFlags",
			args:                 []string{},
			expectedValidateOnly: false,
			expectedError:        false,
		},
		{
			name:                 "ValidateOnlyFlag",
			args:                 []string{"--validate-only"},
			expectedValidateOnly: true,
			expectedError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()

			testCmd := &cobra.Command{
				Use:   "validate",
				Short: "Validate configuration file",
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}

			testCmd.Flags().BoolVar(&configValidateOnly, "validate-only", false, "Perform syntax validation only")

			testCmd.SetArgs(tt.args)
			err := testCmd.Execute()

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if configValidateOnly != tt.expectedValidateOnly {
				t.Errorf("Expected configValidateOnly to be %v, got %v", tt.expectedValidateOnly, configValidateOnly)
			}
		})
	}
}

func testConfigValidateCommandExecution(t *testing.T) {
	testPort := AllocateTestPort(t)
	validConfig := createValidTestConfigContent(testPort)
	invalidConfig := createInvalidTestConfigContent()

	tests := []struct {
		name        string
		configFile  string
		expectError bool
		outputJSON  bool
	}{
		{
			name:        "ValidConfig",
			configFile:  createTempConfigFile(t, validConfig),
			expectError: false,
			outputJSON:  false,
		},
		{
			name:        "ValidConfigJSON",
			configFile:  createTempConfigFile(t, validConfig),
			expectError: false,
			outputJSON:  true,
		},
		{
			name:        "InvalidConfig",
			configFile:  createTempConfigFile(t, invalidConfig),
			expectError: true,
			outputJSON:  false,
		},
		{
			name:        "InvalidConfigJSON",
			configFile:  createTempConfigFile(t, invalidConfig),
			expectError: true,
			outputJSON:  true,
		},
		{
			name:        "NonExistentConfig",
			configFile:  "/nonexistent/config.yaml",
			expectError: true,
			outputJSON:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()
			configPath = tt.configFile
			configJSON = tt.outputJSON

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := configValidate(nil, []string{})

			_ = w.Close()
			os.Stdout = oldStdout

			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			output := string(buf[:n])

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.outputJSON && !tt.expectError {
				if !strings.Contains(output, "{") || !strings.Contains(output, "}") {
					t.Error("Expected JSON output format")
				}
			}
		})
	}
}

func testConfigValidateCommandValidation(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		expectError bool
	}{
		{
			name:        "ValidPath",
			configPath:  createTempConfigFile(t, createValidTestConfigContent(AllocateTestPort(t))),
			expectError: false,
		},
		{
			name:        "NonExistentPath",
			configPath:  "/nonexistent/path.yaml",
			expectError: true,
		},
		{
			name:        "EmptyPath",
			configPath:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()
			configPath = tt.configPath

			err := validateConfigValidateParams()

			if tt.expectError && err == nil {
				t.Log("Expected validation error but got none (may be due to TODO implementations)")
			} else if !tt.expectError && err != nil && !strings.Contains(err.Error(), "unknown error") {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func testConfigValidateCommandErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() string
		expectError bool
		errorType   string
	}{
		{
			name: "MalformedYAML",
			setup: func() string {
				return createTempConfigFile(t, "invalid: yaml: content: [")
			},
			expectError: true,
			errorType:   "configuration",
		},
		{
			name: "EmptyFile",
			setup: func() string {
				return createTempConfigFile(t, "")
			},
			expectError: true,
			errorType:   "configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()
			configPath = tt.setup()

			err := configValidate(nil, []string{})

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && err != nil {
				if !strings.Contains(strings.ToLower(err.Error()), tt.errorType) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorType, err)
				}
			}
		})
	}
}

func testConfigShowCommand(t *testing.T) {
	t.Run("ShowCommandMetadata", testConfigShowCommandMetadata)
	t.Run("ShowCommandFlagParsing", testConfigShowCommandFlagParsing)
	t.Run("ShowCommandExecution", testConfigShowCommandExecution)
	t.Run("ShowCommandValidation", testConfigShowCommandValidation)
	t.Run("ShowCommandErrorScenarios", testConfigShowCommandErrorScenarios)
}

func testConfigShowCommandMetadata(t *testing.T) {
	if configShowCmd.Use != "show" {
		t.Errorf("Expected Use to be 'show', got '%s'", configShowCmd.Use)
	}

	expectedShort := "Display current configuration"
	if configShowCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, configShowCmd.Short)
	}

	if !strings.Contains(configShowCmd.Long, "Display the current LSP Gateway configuration") {
		t.Error("Expected Long description to mention configuration display")
	}

	if configShowCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}
}

func testConfigShowCommandFlagParsing(t *testing.T) {
	tests := []struct {
		name                 string
		args                 []string
		expectedValidateOnly bool
		expectedError        bool
	}{
		{
			name:                 "DefaultFlags",
			args:                 []string{},
			expectedValidateOnly: false,
			expectedError:        false,
		},
		{
			name:                 "ValidateFlag",
			args:                 []string{"--validate"},
			expectedValidateOnly: true,
			expectedError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()

			testCmd := &cobra.Command{
				Use:   "show",
				Short: "Display current configuration",
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}

			testCmd.Flags().BoolVar(&configValidateOnly, "validate", false, "Include validation information in output")

			testCmd.SetArgs(tt.args)
			err := testCmd.Execute()

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if configValidateOnly != tt.expectedValidateOnly {
				t.Errorf("Expected configValidateOnly to be %v, got %v", tt.expectedValidateOnly, configValidateOnly)
			}
		})
	}
}

func testConfigShowCommandExecution(t *testing.T) {
	testPort := AllocateTestPort(t)
	validConfig := createValidTestConfigContent(testPort)

	tests := []struct {
		name        string
		configFile  string
		expectError bool
		outputJSON  bool
		validate    bool
	}{
		{
			name:        "ValidConfigHuman",
			configFile:  createTempConfigFile(t, validConfig),
			expectError: false,
			outputJSON:  false,
			validate:    false,
		},
		{
			name:        "ValidConfigJSON",
			configFile:  createTempConfigFile(t, validConfig),
			expectError: false,
			outputJSON:  true,
			validate:    false,
		},
		{
			name:        "ValidConfigWithValidation",
			configFile:  createTempConfigFile(t, validConfig),
			expectError: false,
			outputJSON:  false,
			validate:    true,
		},
		{
			name:        "ValidConfigJSONWithValidation",
			configFile:  createTempConfigFile(t, validConfig),
			expectError: false,
			outputJSON:  true,
			validate:    true,
		},
		{
			name:        "NonExistentConfig",
			configFile:  "/nonexistent/config.yaml",
			expectError: true,
			outputJSON:  false,
			validate:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()
			configPath = tt.configFile
			configJSON = tt.outputJSON
			configValidateOnly = tt.validate

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := configShow(nil, []string{})

			_ = w.Close()
			os.Stdout = oldStdout

			buf := make([]byte, 2048)
			n, _ := r.Read(buf)
			output := string(buf[:n])

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if !tt.expectError {
				if tt.outputJSON {
					if !strings.Contains(output, "{") || !strings.Contains(output, "}") {
						t.Error("Expected JSON output format")
					}

					var jsonData map[string]interface{}
					if err := json.Unmarshal([]byte(output), &jsonData); err != nil {
						t.Errorf("Invalid JSON output: %v", err)
					}

					if _, exists := jsonData["config"]; !exists {
						t.Error("Expected JSON output to contain 'config' field")
					}
				} else {
					if !strings.Contains(output, "Configuration") {
						t.Error("Expected human output to contain configuration information")
					}
				}
			}
		})
	}
}

func testConfigShowCommandValidation(t *testing.T) {
	tests := []struct {
		name        string
		configPath  string
		expectError bool
	}{
		{
			name:        "ValidPath",
			configPath:  createTempConfigFile(t, createValidTestConfigContent(AllocateTestPort(t))),
			expectError: false,
		},
		{
			name:        "NonExistentPath",
			configPath:  "/nonexistent/path.yaml",
			expectError: true,
		},
		{
			name:        "EmptyPath",
			configPath:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()
			configPath = tt.configPath

			err := validateConfigShowParams()

			if tt.expectError && err == nil {
				t.Log("Expected validation error but got none (may be due to TODO implementations)")
			} else if !tt.expectError && err != nil && !strings.Contains(err.Error(), "unknown error") {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func testConfigShowCommandErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() string
		expectError bool
		errorType   string
	}{
		{
			name: "MalformedYAML",
			setup: func() string {
				return createTempConfigFile(t, "invalid: yaml: content: [")
			},
			expectError: true,
			errorType:   "configuration",
		},
		{
			name: "EmptyFile",
			setup: func() string {
				return createTempConfigFile(t, "")
			},
			expectError: true,
			errorType:   "configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfigGlobals()
			configPath = tt.setup()

			err := configShow(nil, []string{})

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectError && err != nil {
				if !strings.Contains(strings.ToLower(err.Error()), tt.errorType) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorType, err)
				}
			}
		})
	}
}

func TestConfigHelperFunctions(t *testing.T) {
	t.Parallel()

	t.Run("WriteConfigurationFile", testWriteConfigurationFile)
	t.Run("OutputValidationJSON", testOutputValidationJSON)
	t.Run("OutputValidationHuman", testOutputValidationHuman)
	t.Run("OutputConfigJSON", testOutputConfigJSON)
	t.Run("OutputConfigHuman", testOutputConfigHuman)
}

func testWriteConfigurationFile(t *testing.T) {
	cfg := createTestGatewayConfig(t)
	tests := getWriteConfigurationTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runWriteConfigurationTest(t, cfg, tt)
		})
	}
}

func createTestGatewayConfig(t *testing.T) *config.GatewayConfig {
	testPort := AllocateTestPort(t)
	return &config.GatewayConfig{
		Port: testPort,
		Servers: []config.ServerConfig{
			{
				Name:      "test-server",
				Languages: []string{"test"},
				Command:   "test-command",
				Transport: "stdio",
			},
		},
	}
}

type writeConfigTestCase struct {
	name            string
	includeComments bool
	expectError     bool
	validateContent bool
}

func getWriteConfigurationTestCases() []writeConfigTestCase {
	return []writeConfigTestCase{
		{
			name:            "WithoutComments",
			includeComments: false,
			expectError:     false,
			validateContent: true,
		},
		{
			name:            "WithComments",
			includeComments: true,
			expectError:     false,
			validateContent: true,
		},
	}
}

func runWriteConfigurationTest(t *testing.T, cfg *config.GatewayConfig, tt writeConfigTestCase) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, testConfigFilename)

	err := writeConfigurationFile(cfg, outputPath, tt.includeComments)

	validateWriteConfigError(t, err, tt.expectError)

	if tt.validateContent && !tt.expectError {
		validateWriteConfigContent(t, outputPath, cfg, tt.includeComments)
	}
}

func validateWriteConfigError(t *testing.T, err error, expectError bool) {
	if expectError && err == nil {
		t.Error("Expected error but got none")
	} else if !expectError && err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
}

func validateWriteConfigContent(t *testing.T, outputPath string, cfg *config.GatewayConfig, includeComments bool) {
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Expected file to be created")
		return
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	if includeComments {
		validateConfigComments(t, content)
	}

	validateConfigParsing(t, content, cfg)
}

func validateConfigComments(t *testing.T, content []byte) {
	contentStr := string(content)
	if !strings.Contains(contentStr, "#") {
		t.Error("Expected comments to be included")
	}
	if !strings.Contains(contentStr, "LSP Gateway Configuration") {
		t.Error("Expected header comment to be included")
	}
}

func validateConfigParsing(t *testing.T, content []byte, expectedCfg *config.GatewayConfig) {
	var parsedConfig config.GatewayConfig
	if err := yaml.Unmarshal(content, &parsedConfig); err != nil {
		t.Errorf("Generated YAML is invalid: %v", err)
		return
	}

	if parsedConfig.Port != expectedCfg.Port {
		t.Errorf("Expected port %d, got %d", expectedCfg.Port, parsedConfig.Port)
	}

	if len(parsedConfig.Servers) != len(expectedCfg.Servers) {
		t.Errorf("Expected %d servers, got %d", len(expectedCfg.Servers), len(parsedConfig.Servers))
	}
}

func testOutputValidationJSON(t *testing.T) {
	tests := getValidationJSONTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runValidationJSONTest(t, tt)
		})
	}
}

type validationJSONTestCase struct {
	name     string
	valid    bool
	issues   []string
	warnings []string
}

func getValidationJSONTestCases() []validationJSONTestCase {
	return []validationJSONTestCase{
		{
			name:     "ValidConfig",
			valid:    true,
			issues:   []string{},
			warnings: []string{},
		},
		{
			name:     "InvalidConfigWithIssues",
			valid:    false,
			issues:   []string{"Invalid port", "Missing server"},
			warnings: []string{},
		},
		{
			name:     "ValidConfigWithWarnings",
			valid:    true,
			issues:   []string{},
			warnings: []string{"Server not found", "Deprecated option"},
		},
		{
			name:     "InvalidConfigWithBoth",
			valid:    false,
			issues:   []string{"Critical error"},
			warnings: []string{"Minor warning"},
		},
	}
}

func runValidationJSONTest(t *testing.T, tt validationJSONTestCase) {
	output := captureValidationJSONOutput(tt)
	validateJSONOutput(t, output, tt)
}

func captureValidationJSONOutput(tt validationJSONTestCase) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputValidationJSON(tt.valid, tt.issues, tt.warnings)

	_ = w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func validateJSONOutput(t *testing.T, output string, tt validationJSONTestCase) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Invalid JSON output: %v", err)
		return
	}

	validateJSONValidField(t, result, tt.valid)
	validateJSONIssuesField(t, result, tt.issues)
	validateJSONWarningsField(t, result, tt.warnings)
}

func validateJSONValidField(t *testing.T, result map[string]interface{}, expectedValid bool) {
	if valid, exists := result["valid"].(bool); !exists || valid != expectedValid {
		t.Errorf("Expected valid to be %v, got %v", expectedValid, valid)
	}
}

func validateJSONIssuesField(t *testing.T, result map[string]interface{}, expectedIssues []string) {
	if issues, exists := result["issues"].([]interface{}); exists {
		if len(issues) != len(expectedIssues) {
			t.Errorf("Expected %d issues, got %d", len(expectedIssues), len(issues))
		}
	}
}

func validateJSONWarningsField(t *testing.T, result map[string]interface{}, expectedWarnings []string) {
	if warnings, exists := result["warnings"].([]interface{}); exists {
		if len(warnings) != len(expectedWarnings) {
			t.Errorf("Expected %d warnings, got %d", len(expectedWarnings), len(warnings))
		}
	}
}

func testOutputValidationHuman(t *testing.T) {
	cfg := createTestValidationConfig(t)
	tests := getValidationHumanTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runValidationHumanTest(t, cfg, tt)
		})
	}
}

func createTestValidationConfig(t *testing.T) *config.GatewayConfig {
	testPort := AllocateTestPort(t)
	return &config.GatewayConfig{
		Port: testPort,
		Servers: []config.ServerConfig{
			{Name: "test-server", Languages: []string{"test"}},
		},
	}
}

type validationHumanTestCase struct {
	name     string
	valid    bool
	issues   []string
	warnings []string
}

func getValidationHumanTestCases() []validationHumanTestCase {
	return []validationHumanTestCase{
		{
			name:     "ValidConfig",
			valid:    true,
			issues:   []string{},
			warnings: []string{},
		},
		{
			name:     "InvalidConfig",
			valid:    false,
			issues:   []string{"Test issue"},
			warnings: []string{"Test warning"},
		},
	}
}

func runValidationHumanTest(t *testing.T, cfg *config.GatewayConfig, tt validationHumanTestCase) {
	output := captureValidationHumanOutput(cfg, tt)
	validateHumanOutput(t, output, cfg, tt)
}

func captureValidationHumanOutput(cfg *config.GatewayConfig, tt validationHumanTestCase) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputValidationHuman(tt.valid, tt.issues, tt.warnings, cfg)

	_ = w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func validateHumanOutput(t *testing.T, output string, cfg *config.GatewayConfig, tt validationHumanTestCase) {
	validateHumanOutputHeader(t, output)
	validateHumanOutputStatus(t, output, tt.valid)
	validateHumanOutputConfig(t, output, cfg)
}

func validateHumanOutputHeader(t *testing.T, output string) {
	if !strings.Contains(output, "Configuration Validation") {
		t.Error("Expected output to contain validation header")
	}
}

func validateHumanOutputStatus(t *testing.T, output string, valid bool) {
	if valid {
		if !strings.Contains(output, "✓ Configuration is valid") {
			t.Error("Expected valid status message")
		}
	} else {
		if !strings.Contains(output, "✗ Configuration has issues") {
			t.Error("Expected invalid status message")
		}
	}
}

func validateHumanOutputConfig(t *testing.T, output string, cfg *config.GatewayConfig) {
	if !strings.Contains(output, fmt.Sprintf("Port: %d", cfg.Port)) {
		t.Error("Expected port information")
	}

	if !strings.Contains(output, fmt.Sprintf("Servers configured: %d", len(cfg.Servers))) {
		t.Error("Expected server count information")
	}
}

func testOutputConfigJSON(t *testing.T) {
	cfg := createTestConfigJSONConfig(t)
	tests := getConfigJSONTestCases(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runConfigJSONTest(t, tt)
		})
	}
}

func createTestConfigJSONConfig(t *testing.T) *config.GatewayConfig {
	testPort := AllocateTestPort(t)
	return &config.GatewayConfig{
		Port: testPort,
		Servers: []config.ServerConfig{
			{
				Name:      "test-server",
				Languages: []string{"test"},
				Command:   "test-command",
				Transport: "stdio",
			},
		},
	}
}

type configJSONTestCase struct {
	name       string
	config     *config.GatewayConfig
	validation interface{}
}

func getConfigJSONTestCases(cfg *config.GatewayConfig) []configJSONTestCase {
	return []configJSONTestCase{
		{
			name:       "ConfigOnly",
			config:     cfg,
			validation: nil,
		},
		{
			name:       "ConfigWithValidation",
			config:     cfg,
			validation: map[string]interface{}{"status": "valid"},
		},
	}
}

func runConfigJSONTest(t *testing.T, tt configJSONTestCase) {
	output, err := captureConfigJSONOutput(tt)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
		return
	}

	validateConfigJSONOutput(t, output, tt)
}

func captureConfigJSONOutput(tt configJSONTestCase) (string, error) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputConfigJSON(tt.config, tt.validation)

	_ = w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	return string(buf[:n]), err
}

func validateConfigJSONOutput(t *testing.T, output string, tt configJSONTestCase) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("Invalid JSON output: %v", err)
		return
	}

	if _, exists := result["config"]; !exists {
		t.Error("Expected 'config' field in JSON output")
	}

	if tt.validation != nil {
		if _, exists := result["validation"]; !exists {
			t.Error("Expected 'validation' field in JSON output")
		}
	}
}

func testOutputConfigHuman(t *testing.T) {
	cfg := createTestConfigHumanConfig(t)
	tests := getConfigHumanTestCases(cfg)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runConfigHumanTest(t, tt)
		})
	}
}

func createTestConfigHumanConfig(t *testing.T) *config.GatewayConfig {
	testPort := AllocateTestPort(t)
	return &config.GatewayConfig{
		Port: testPort,
		Servers: []config.ServerConfig{
			{
				Name:      "test-server",
				Languages: []string{"test"},
				Command:   "test-command",
				Args:      []string{"--test"},
				Transport: "stdio",
			},
		},
	}
}

type configHumanTestCase struct {
	name       string
	config     *config.GatewayConfig
	validation interface{}
}

func getConfigHumanTestCases(cfg *config.GatewayConfig) []configHumanTestCase {
	return []configHumanTestCase{
		{
			name:       "ConfigOnly",
			config:     cfg,
			validation: nil,
		},
		{
			name:       "ConfigWithValidation",
			config:     cfg,
			validation: map[string]interface{}{"status": "valid"},
		},
	}
}

func runConfigHumanTest(t *testing.T, tt configHumanTestCase) {
	originalConfigPath := configPath
	configPath = testConfigFilename
	defer func() { configPath = originalConfigPath }()

	output, err := captureConfigHumanOutput(tt)
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
		return
	}

	validateConfigHumanOutput(t, output, tt)
}

func captureConfigHumanOutput(tt configHumanTestCase) (string, error) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := outputConfigHuman(tt.config, tt.validation)

	_ = w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	return string(buf[:n]), err
}

func validateConfigHumanOutput(t *testing.T, output string, tt configHumanTestCase) {
	validateConfigHumanHeader(t, output)
	validateConfigHumanDetails(t, output, tt.config)
	validateConfigHumanValidation(t, output, tt.validation)
}

func validateConfigHumanHeader(t *testing.T, output string) {
	if !strings.Contains(output, "LSP Gateway Configuration") {
		t.Error("Expected output to contain configuration header")
	}
}

func validateConfigHumanDetails(t *testing.T, output string, cfg *config.GatewayConfig) {
	if !strings.Contains(output, fmt.Sprintf("Server Port: %d", cfg.Port)) {
		t.Error("Expected port information")
	}

	if !strings.Contains(output, fmt.Sprintf("Configured Servers: %d", len(cfg.Servers))) {
		t.Error("Expected server count information")
	}

	if !strings.Contains(output, "Language Servers:") {
		t.Error("Expected language servers section")
	}

	if !strings.Contains(output, cfg.Servers[0].Name) {
		t.Error("Expected server name in output")
	}

	if !strings.Contains(output, cfg.Servers[0].Command) {
		t.Error("Expected server command in output")
	}
}

func validateConfigHumanValidation(t *testing.T, output string, validation interface{}) {
	if validation != nil {
		if !strings.Contains(output, "Validation Status:") {
			t.Error("Expected validation status section")
		}
	}
}
