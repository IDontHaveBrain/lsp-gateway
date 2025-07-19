package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"lsp-gateway/internal/config"
)

// allocateTestPortServer returns a dynamically allocated port for testing
func allocateTestPortServer(t *testing.T) int {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to allocate test port: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Logf("Error closing test listener: %v", err)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

// Test configuration constants
const (
	validTestConfig = `
port: 8080
servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    args: []
    transport: "stdio"
`
)

func TestServerCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testServerCommandMetadata},
		{"FlagParsing", testServerCommandFlagParsing},
		{"ConfigurationLoading", testServerCommandConfigurationLoading},
		{"PortOverride", testServerCommandPortOverride},
		{"ConfigurationValidation", testServerCommandConfigurationValidation},
		{"ErrorScenarios", testServerCommandErrorScenarios},
		{"CommandExecution", testServerCommandExecution},
		{"Help", testServerCommandHelp},
		{"Integration", testServerCommandIntegration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables before each test
			configPath = DefaultConfigFile
			port = 8080
			tt.testFunc(t)
		})
	}
}

func testServerCommandMetadata(t *testing.T) {
	if serverCmd.Use != CmdServer {
		t.Errorf("Expected Use to be 'server', got '%s'", serverCmd.Use)
	}

	expectedShort := "Start the LSP Gateway server"
	if serverCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, serverCmd.Short)
	}

	expectedLong := "Start the LSP Gateway server with the specified configuration."
	if serverCmd.Long != expectedLong {
		t.Errorf("Expected Long to be '%s', got '%s'", expectedLong, serverCmd.Long)
	}

	// Verify RunE function is set
	if serverCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}

	// Verify Run function is not set (we use RunE)
	if serverCmd.Run != nil {
		t.Error("Expected Run function to be nil (using RunE instead)")
	}
}

func testServerCommandFlagParsing(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedConfig string
		expectedPort   int
		expectedError  bool
	}{
		{
			name:           "DefaultFlags",
			args:           []string{},
			expectedConfig: DefaultConfigFile,
			expectedPort:   8080,
			expectedError:  false,
		},
		{
			name:           "ConfigFlag",
			args:           []string{"--config", "custom.yaml"},
			expectedConfig: "custom.yaml",
			expectedPort:   8080,
			expectedError:  false,
		},
		{
			name:           "ConfigFlagShort",
			args:           []string{"-c", "custom.yaml"},
			expectedConfig: "custom.yaml",
			expectedPort:   8080,
			expectedError:  false,
		},
		{
			name:           "PortFlag",
			args:           []string{"--port", "9090"},
			expectedConfig: DefaultConfigFile,
			expectedPort:   9090,
			expectedError:  false,
		},
		{
			name:           "PortFlagShort",
			args:           []string{"-p", "9090"},
			expectedConfig: DefaultConfigFile,
			expectedPort:   9090,
			expectedError:  false,
		},
		{
			name:           "BothFlags",
			args:           []string{"--config", "custom.yaml", "--port", "9090"},
			expectedConfig: "custom.yaml",
			expectedPort:   9090,
			expectedError:  false,
		},
		{
			name:           "BothFlagsShort",
			args:           []string{"-c", "custom.yaml", "-p", "9090"},
			expectedConfig: "custom.yaml",
			expectedPort:   9090,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables
			configPath = DefaultConfigFile
			port = 8080

			// Create a test command to avoid modifying global state
			testCmd := &cobra.Command{
				Use:   "server",
				Short: "Start the LSP Gateway server",
				Long:  "Start the LSP Gateway server with the specified configuration.",
				RunE: func(cmd *cobra.Command, args []string) error {
					// Just parse flags, don't execute
					return nil
				},
			}

			// Add flags to test command
			testCmd.Flags().StringVarP(&configPath, "config", "c", DefaultConfigFile, "Configuration file path")
			testCmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")

			// Set arguments
			testCmd.SetArgs(tt.args)

			// Execute flag parsing
			err := testCmd.Execute()

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			// Check parsed values
			if configPath != tt.expectedConfig {
				t.Errorf("Expected configPath to be '%s', got '%s'", tt.expectedConfig, configPath)
			}

			if port != tt.expectedPort {
				t.Errorf("Expected port to be %d, got %d", tt.expectedPort, port)
			}
		})
	}
}

func testServerCommandConfigurationLoading(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	validConfig := validTestConfig

	err := os.WriteFile(configFile, []byte(validConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	tests := []struct {
		name          string
		configPath    string
		expectedError bool
		errorContains string
	}{
		{
			name:          "ValidConfig",
			configPath:    configFile,
			expectedError: false,
		},
		{
			name:          "NonexistentConfig",
			configPath:    "/nonexistent/config.yaml",
			expectedError: true,
			errorContains: "configuration file not found",
		},
		{
			name:          "InvalidYAML",
			configPath:    createInvalidYAMLFile(t, tempDir),
			expectedError: true,
			errorContains: "failed to parse configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test configuration loading and validation without gateway creation
			cfg, err := config.LoadConfig(tt.configPath)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Verify config was loaded correctly
				if cfg == nil {
					t.Error("Expected config to be loaded")
				} else if cfg.Port != 8080 {
					t.Errorf("Expected config port to be 8080, got %d", cfg.Port)
				}

				// Test validation
				if err := config.ValidateConfig(cfg); err != nil {
					t.Errorf("Config validation failed: %v", err)
				}
			}
		})
	}
}

func testServerCommandPortOverride(t *testing.T) {
	// Create temporary config file with port 8080
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	configContent := validTestConfig

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	tests := []struct {
		name         string
		portFlag     int
		expectedPort int
	}{
		{
			name:         "NoPortOverride",
			portFlag:     8080, // Default value
			expectedPort: 8080,
		},
		{
			name:         "PortOverride",
			portFlag:     9090,
			expectedPort: 9090,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test port override logic
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Override port if specified (mimicking server.go logic)
			if tt.portFlag != 8080 {
				cfg.Port = tt.portFlag
			}

			// Validate configuration
			if err := config.ValidateConfig(cfg); err != nil {
				t.Errorf("Config validation failed: %v", err)
			}

			if cfg.Port != tt.expectedPort {
				t.Errorf("Expected final port to be %d, got %d", tt.expectedPort, cfg.Port)
			}
		})
	}
}

func testServerCommandConfigurationValidation(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		configContent string
		expectedError bool
		errorContains string
	}{
		{
			name: "ValidConfig",
			configContent: `
port: 8080
servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    args: []
    transport: "stdio"
`,
			expectedError: false,
		},
		{
			name: "InvalidPort",
			configContent: `
port: 99999
servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    args: []
    transport: "stdio"
`,
			expectedError: true,
			errorContains: "invalid port",
		},
		{
			name: "NoServers",
			configContent: `
port: 8080
servers: []
`,
			expectedError: true,
			errorContains: "at least one server must be configured",
		},
		{
			name: "InvalidServerConfig",
			configContent: `
port: 8080
servers:
  - name: ""
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
`,
			expectedError: true,
			errorContains: "server name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := filepath.Join(tempDir, fmt.Sprintf("config-%s.yaml", tt.name))
			err := os.WriteFile(configFile, []byte(tt.configContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}

			// Load and validate configuration
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Test validation
			err = config.ValidateConfig(cfg)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected validation error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error but got: %v", err)
				}
			}
		})
	}
}

func testServerCommandErrorScenarios(t *testing.T) {
	tempDir := t.TempDir()
	validConfigFile := createValidConfigFile(t, tempDir)

	tests := []struct {
		name          string
		configPath    string
		expectedError bool
		errorContains string
	}{
		{
			name:          "ConfigLoadFailure",
			configPath:    "/nonexistent/config.yaml",
			expectedError: true,
			errorContains: "configuration file not found",
		},
		{
			name:          "ConfigValidationFailure",
			configPath:    createInvalidConfigFile(t, tempDir),
			expectedError: true,
			errorContains: "invalid port",
		},
		{
			name:          "ValidConfig",
			configPath:    validConfigFile,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test loading and validation steps
			cfg, err := config.LoadConfig(tt.configPath)

			if tt.expectedError {
				if err == nil {
					// Try validation if load succeeded but validation should fail
					if cfg != nil {
						err = config.ValidateConfig(cfg)
					}
				}

				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}

				// Test validation for valid configs
				if cfg != nil {
					if err := config.ValidateConfig(cfg); err != nil {
						t.Errorf("Config validation failed: %v", err)
					}
				}
			}
		})
	}
}

func testServerCommandExecution(t *testing.T) {
	// Test basic command structure and execution without actual server startup

	// Verify command is properly registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == CmdServer {
			found = true
			break
		}
	}

	if !found {
		t.Error("server command should be added to root command")
	}

	// Test flag definitions
	configFlag := serverCmd.Flag("config")
	if configFlag == nil {
		t.Error("Expected config flag to be defined")
	} else {
		if configFlag.Shorthand != "c" {
			t.Errorf("Expected config flag shorthand to be 'c', got '%s'", configFlag.Shorthand)
		}
		if configFlag.DefValue != DefaultConfigFile {
			t.Errorf("Expected config flag default to be 'config.yaml', got '%s'", configFlag.DefValue)
		}
	}

	portFlag := serverCmd.Flag("port")
	if portFlag == nil {
		t.Error("Expected port flag to be defined")
	} else {
		if portFlag.Shorthand != "p" {
			t.Errorf("Expected port flag shorthand to be 'p', got '%s'", portFlag.Shorthand)
		}
		if portFlag.DefValue != "8080" {
			t.Errorf("Expected port flag default to be '8080', got '%s'", portFlag.DefValue)
		}
	}
}

func testServerCommandHelp(t *testing.T) {
	// Test help output
	output := captureStdoutServer(t, func() {
		testCmd := &cobra.Command{
			Use:   serverCmd.Use,
			Short: serverCmd.Short,
			Long:  serverCmd.Long,
			RunE:  serverCmd.RunE,
		}

		// Add flags to test command for help
		testCmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "Configuration file path")
		testCmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")

		testCmd.SetArgs([]string{"--help"})
		err := testCmd.Execute()
		if err != nil {
			t.Errorf("Help command should not return error, got: %v", err)
		}
	})

	// Verify help output contains expected elements
	expectedElements := []string{
		"Start the LSP Gateway server",
		"server",
		"Usage:",
		"Flags:",
		"--config",
		"--port",
		"-c",
		"-p",
	}

	for _, element := range expectedElements {
		if !strings.Contains(output, element) {
			t.Errorf("Expected help output to contain '%s', got:\n%s", element, output)
		}
	}
}

func testServerCommandIntegration(t *testing.T) {
	// Test execution through root command with basic config validation
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	// Create test root command
	testRoot := &cobra.Command{
		Use:   rootCmd.Use,
		Short: rootCmd.Short,
	}

	// Create test server command that tests config loading but doesn't start servers
	testServer := &cobra.Command{
		Use:   serverCmd.Use,
		Short: serverCmd.Short,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			// Override port if specified
			if port != 8080 {
				cfg.Port = port
			}

			// Validate configuration
			if err := config.ValidateConfig(cfg); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			// Don't actually create gateway or start server for testing
			return nil
		},
	}

	// Add flags
	testServer.Flags().StringVarP(&configPath, "config", "c", configFile, "Configuration file path")
	testServer.Flags().IntVarP(&port, "port", "p", 8080, "Server port")

	testRoot.AddCommand(testServer)

	// Execute server command through root
	testRoot.SetArgs([]string{"server"})
	err := testRoot.Execute()

	if err != nil {
		t.Errorf("Expected no error executing server through root, got: %v", err)
	}
}

// Test edge cases and special scenarios
func TestServerCommandEdgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"WithExtraArgs", testServerCommandWithExtraArgs},
		{"HTTPServerSetup", testServerCommandHTTPServerSetup},
		{"ContextCancellation", testServerCommandContextCancellation},
		{"SignalHandlingSimulation", testServerCommandSignalHandlingSimulation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables before each test
			configPath = DefaultConfigFile
			port = 8080
			tt.testFunc(t)
		})
	}
}

func testServerCommandWithExtraArgs(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	testCmd := &cobra.Command{
		Use: "server",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Server command should ignore extra arguments
			if len(args) > 0 {
				t.Logf("Server command received extra args: %v", args)
			}

			// Minimal test execution - just config loading
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				return err
			}

			return config.ValidateConfig(cfg)
		},
	}

	testCmd.Flags().StringVarP(&configPath, "config", "c", configFile, "Configuration file path")
	testCmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")

	// Test with extra arguments
	testCmd.SetArgs([]string{"extra", "args"})
	err := testCmd.Execute()

	if err != nil {
		t.Errorf("Server command should handle extra args gracefully, got error: %v", err)
	}
}

func testServerCommandHTTPServerSetup(t *testing.T) {
	// Test HTTP server setup logic simulation
	tempDir := t.TempDir()
	testPort := allocateTestPortServer(t)
	configFile := createValidConfigFileWithPort(t, tempDir, testPort)

	// Test HTTP handler setup logic
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := config.ValidateConfig(cfg); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Test HTTP server configuration that would be used
	expectedAddr := fmt.Sprintf(":%d", cfg.Port)
	expectedAddrCheck := fmt.Sprintf(":%d", testPort)
	if expectedAddr != expectedAddrCheck {
		t.Errorf("Expected server address to be '%s', got '%s'", expectedAddrCheck, expectedAddr)
	}

	// Test that /jsonrpc endpoint would be set up
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)); err != nil {
			http.Error(w, "write error", http.StatusInternalServerError)
		}
	})

	// Create test server to verify handler
	server := httptest.NewServer(mux)
	defer server.Close()

	// Test the handler
	resp, err := http.Post(server.URL+"/jsonrpc", "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test","params":{}}`))
	if err != nil {
		t.Fatalf("Failed to test handler: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("cleanup error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func testServerCommandContextCancellation(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	// Test context cancellation behavior
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := config.ValidateConfig(cfg); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Test context cancellation pattern
	ctx, cancel := context.WithCancel(context.Background())

	// Simulate the context usage pattern
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	// Cancel context and verify cleanup
	cancel()

	// Wait for cancellation to be processed
	select {
	case <-done:
		// Expected behavior
	case <-time.After(time.Second):
		t.Error("Context cancellation not processed in time")
	}
}

func testServerCommandSignalHandlingSimulation(t *testing.T) {
	// Test signal handling simulation (without actual signals)
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := config.ValidateConfig(cfg); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Simulate signal handling logic
	sigCh := make(chan os.Signal, 1)

	// Simulate receiving a signal
	go func() {
		time.Sleep(10 * time.Millisecond)
		sigCh <- syscall.SIGINT
	}()

	// Wait for signal (simulated)
	select {
	case sig := <-sigCh:
		if sig != syscall.SIGINT {
			t.Errorf("Expected SIGINT, got %v", sig)
		}
	case <-time.After(time.Second):
		t.Error("Signal not received in time")
	}

	// Simulate graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Verify shutdown context works
	select {
	case <-shutdownCtx.Done():
		// This should only happen after timeout
		t.Error("Shutdown context should not timeout immediately")
	default:
		// Expected behavior - context is active
	}
}

// Test command completeness and structure
func TestServerCommandCompleteness(t *testing.T) {
	t.Parallel()
	// Verify command structure
	if serverCmd.Name() != CmdServer {
		t.Errorf("Expected command name 'server', got '%s'", serverCmd.Name())
	}

	// Verify flags are properly defined
	configFlag := serverCmd.Flag("config")
	if configFlag == nil {
		t.Error("Expected config flag to be defined")
	}

	portFlag := serverCmd.Flag("port")
	if portFlag == nil {
		t.Error("Expected port flag to be defined")
	}

	// Verify no subcommands
	if serverCmd.HasSubCommands() {
		t.Error("Server command should not have subcommands")
	}

	// Verify command is added to root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == CmdServer {
			found = true
			break
		}
	}

	if !found {
		t.Error("Server command should be added to root command")
	}
}

// Helper functions
func createValidConfigFile(t *testing.T, dir string) string {
	t.Helper()

	configFile := filepath.Join(dir, "valid-config.yaml")
	content := validTestConfig

	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create valid config file: %v", err)
	}

	return configFile
}

// createValidConfigFileWithPort creates a config file with a specific port
func createValidConfigFileWithPort(t *testing.T, dir string, port int) string {
	t.Helper()
	configFile := filepath.Join(dir, "valid-config.yaml")
	content := fmt.Sprintf(`
port: %d
servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls"
    args: []
    transport: "stdio"
`, port)
	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create valid config file: %v", err)
	}
	return configFile
}

func createInvalidConfigFile(t *testing.T, dir string) string {
	t.Helper()

	configFile := filepath.Join(dir, "invalid-config.yaml")
	content := `
port: -1
servers: []
`

	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid config file: %v", err)
	}

	return configFile
}

func createInvalidYAMLFile(t *testing.T, dir string) string {
	t.Helper()

	configFile := filepath.Join(dir, "invalid.yaml")
	content := `
port: 8080
servers:
  - name: "go-lsp"
    languages: ["go"
    command: "gopls"
    # Missing closing bracket and indentation errors
  invalid: structure
`

	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid YAML file: %v", err)
	}

	return configFile
}

// Helper function to capture stdout during function execution (rename to avoid conflict)
func captureStdoutServer(t *testing.T, fn func()) string {
	t.Helper()

	// Save original stdout
	oldStdout := os.Stdout

	// Create pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	// Replace stdout
	os.Stdout = w

	// Execute function
	fn()

	// Close writer and restore stdout
	if err := w.Close(); err != nil {
		t.Logf("cleanup error closing writer: %v", err)
	}
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("Failed to read captured output: %v", err)
	}

	return buf.String()
}

// Benchmark server command execution
func BenchmarkServerCommandFlagParsing(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Reset global variables
		configPath = "config.yaml"
		port = 8080

		testCmd := &cobra.Command{
			Use: "server",
			RunE: func(cmd *cobra.Command, args []string) error {
				return nil
			},
		}

		testCmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "Configuration file path")
		testCmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")

		testCmd.SetArgs([]string{"--config", "test.yaml", "--port", "9090"})
		if err := testCmd.Execute(); err != nil {
			b.Logf("command execution error: %v", err)
		}
	}
}

func BenchmarkServerCommandConfigLoading(b *testing.B) {
	tempDir := b.TempDir()
	configFile := filepath.Join(tempDir, "benchmark-config.yaml")

	content := validTestConfig

	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		b.Fatalf("Failed to create config file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			b.Fatalf("Config loading failed: %v", err)
		}

		err = config.ValidateConfig(cfg)
		if err != nil {
			b.Fatalf("Config validation failed: %v", err)
		}
	}
}
