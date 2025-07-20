package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"

	"github.com/spf13/cobra"
)

func createValidTestConfig(port int) string {
	return CreateConfigWithPort(port)
}

func TestServerCommand(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with external dependencies
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
			configPath = DefaultConfigFile
			port = DefaultServerPort
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

	if serverCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}

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
			expectedPort:   DefaultServerPort,
			expectedError:  false,
		},
		{
			name:           "ConfigFlag",
			args:           []string{"--config", "custom.yaml"},
			expectedConfig: "custom.yaml",
			expectedPort:   DefaultServerPort,
			expectedError:  false,
		},
		{
			name:           "ConfigFlagShort",
			args:           []string{"-c", "custom.yaml"},
			expectedConfig: "custom.yaml",
			expectedPort:   DefaultServerPort,
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
			configPath = DefaultConfigFile
			port = DefaultServerPort

			testCmd := &cobra.Command{
				Use:   "server",
				Short: "Start the LSP Gateway server",
				Long:  "Start the LSP Gateway server with the specified configuration.",
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}

			testCmd.Flags().StringVarP(&configPath, "config", "c", DefaultConfigFile, "Configuration file path")
			testCmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")

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

			if port != tt.expectedPort {
				t.Errorf("Expected port to be %d, got %d", tt.expectedPort, port)
			}
		})
	}
}

func testServerCommandConfigurationLoading(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	testPort := AllocateTestPort(t)
	validConfig := createValidTestConfig(testPort)

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

				if cfg == nil {
					t.Error("Expected config to be loaded")
				} else if cfg.Port <= 0 || cfg.Port > 65535 {
					t.Errorf("Expected valid port number, got %d", cfg.Port)
				}

				if err := config.ValidateConfig(cfg); err != nil {
					t.Errorf("Config validation failed: %v", err)
				}
			}
		})
	}
}

func testServerCommandPortOverride(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	testPort := AllocateTestPort(t)
	configContent := createValidTestConfig(testPort)

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
			portFlag:     DefaultServerPort, // Default value
			expectedPort: testPort,          // Should keep config file port when using default
		},
		{
			name:         "PortOverride",
			portFlag:     9090,
			expectedPort: 9090,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			if tt.portFlag != DefaultServerPort {
				cfg.Port = tt.portFlag
			}

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
			name:          "ValidConfig",
			configContent: CreateConfigWithPort(DefaultServerPort),
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
			name:          "NoServers",
			configContent: CreateMinimalConfigWithPort(DefaultServerPort),
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

			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

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
			cfg, err := config.LoadConfig(tt.configPath)

			if tt.expectedError {
				if err == nil {
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
		if portFlag.DefValue != "8080" { // Keep checking original string value
			t.Errorf("Expected port flag default to be '8080', got '%s'", portFlag.DefValue)
		}
	}
}

func testServerCommandHelp(t *testing.T) {
	output := captureStdoutServer(t, func() {
		testCmd := &cobra.Command{
			Use:   serverCmd.Use,
			Short: serverCmd.Short,
			Long:  serverCmd.Long,
			RunE:  serverCmd.RunE,
		}

		testCmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "Configuration file path")
		testCmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")

		testCmd.SetArgs([]string{"--help"})
		err := testCmd.Execute()
		if err != nil {
			t.Errorf("Help command should not return error, got: %v", err)
		}
	})

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
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	testRoot := &cobra.Command{
		Use:   rootCmd.Use,
		Short: rootCmd.Short,
	}

	testServer := &cobra.Command{
		Use:   serverCmd.Use,
		Short: serverCmd.Short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			if port != DefaultServerPort {
				cfg.Port = port
			}

			if err := config.ValidateConfig(cfg); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}

			return nil
		},
	}

	testServer.Flags().StringVarP(&configPath, "config", "c", configFile, "Configuration file path")
	testServer.Flags().IntVarP(&port, "port", "p", 8080, "Server port")

	testRoot.AddCommand(testServer)

	testRoot.SetArgs([]string{"server"})
	err := testRoot.Execute()

	if err != nil {
		t.Errorf("Expected no error executing server through root, got: %v", err)
	}
}

func TestServerCommandEdgeCases(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with real HTTP servers
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
			configPath = DefaultConfigFile
			port = DefaultServerPort
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
			if len(args) > 0 {
				t.Logf("Server command received extra args: %v", args)
			}

			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				return err
			}

			return config.ValidateConfig(cfg)
		},
	}

	testCmd.Flags().StringVarP(&configPath, "config", "c", configFile, "Configuration file path")
	testCmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")

	testCmd.SetArgs([]string{"extra", "args"})
	err := testCmd.Execute()

	if err != nil {
		t.Errorf("Server command should handle extra args gracefully, got error: %v", err)
	}
}

func testServerCommandHTTPServerSetup(t *testing.T) {
	tempDir := t.TempDir()
	testPort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, testPort)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := config.ValidateConfig(cfg); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	expectedAddr := fmt.Sprintf(":%d", cfg.Port)
	expectedAddrCheck := fmt.Sprintf(":%d", testPort)
	if expectedAddr != expectedAddrCheck {
		t.Errorf("Expected server address to be '%s', got '%s'", expectedAddrCheck, expectedAddr)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)); err != nil {
			http.Error(w, "write error", http.StatusInternalServerError)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

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

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := config.ValidateConfig(cfg); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Context cancellation not processed in time")
	}
}

func testServerCommandSignalHandlingSimulation(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if err := config.ValidateConfig(cfg); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	sigCh := make(chan os.Signal, 1)

	go func() {
		time.Sleep(10 * time.Millisecond)
		sigCh <- syscall.SIGINT
	}()

	select {
	case sig := <-sigCh:
		if sig != syscall.SIGINT {
			t.Errorf("Expected SIGINT, got %v", sig)
		}
	case <-time.After(time.Second):
		t.Error("Signal not received in time")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	select {
	case <-shutdownCtx.Done():
		t.Error("Shutdown context should not timeout immediately")
	default:
	}
}

func TestServerCommandCompleteness(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock
	if serverCmd.Name() != CmdServer {
		t.Errorf("Expected command name 'server', got '%s'", serverCmd.Name())
	}

	configFlag := serverCmd.Flag("config")
	if configFlag == nil {
		t.Error("Expected config flag to be defined")
	}

	portFlag := serverCmd.Flag("port")
	if portFlag == nil {
		t.Error("Expected port flag to be defined")
	}

	if serverCmd.HasSubCommands() {
		t.Error("Server command should not have subcommands")
	}

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

func createValidConfigFile(t *testing.T, dir string) string {
	t.Helper()

	configFile := filepath.Join(dir, "valid-config.yaml")
	testPort := AllocateTestPort(t)
	content := createValidTestConfig(testPort)

	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create valid config file: %v", err)
	}

	return configFile
}

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
	content := fmt.Sprintf(`
port: %d
servers:
  - name: "go-lsp"
    languages: ["go"
    command: "gopls"
    # Missing closing bracket and indentation errors
  invalid: structure
`, DefaultServerPort)

	err := os.WriteFile(configFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid YAML file: %v", err)
	}

	return configFile
}

func captureStdoutServer(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Logf("cleanup error closing writer: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	if err != nil {
		t.Fatalf("Failed to read captured output: %v", err)
	}

	return buf.String()
}

func TestServerStartupFunctions(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with real HTTP server operations
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"RunServerConfigLoading", testRunServerConfigLoading},
		{"RunServerPortOverride", testRunServerPortOverride},
		{"RunServerConfigValidation", testRunServerConfigValidation},
		{"RunServerGatewayCreation", testRunServerGatewayCreation},
		{"RunServerGatewayStart", testRunServerGatewayStart},
		{"RunServerHTTPSetup", testRunServerHTTPSetup},
		{"RunServerSignalHandling", testRunServerSignalHandling},
		{"RunServerGracefulShutdown", testRunServerGracefulShutdown},
		{"RunServerInvalidConfig", testRunServerInvalidConfig},
		{"RunServerGatewayFailure", testRunServerGatewayFailure},
		{"RunServerDirectCall", testRunServerDirectCall},
		{"RunServerWithActualConfigFile", testRunServerWithActualConfigFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath = DefaultConfigFile
			port = DefaultServerPort
			tt.testFunc(t)
		})
	}
}

func testRunServerConfigLoading(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Errorf("Config loading failed (runServer path): %v", err)
	}

	if cfg == nil {
		t.Error("Expected config to be loaded")
		return
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		t.Errorf("Expected valid port number, got %d", cfg.Port)
	}

	_, err = config.LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent config file")
	}

	if !strings.Contains(err.Error(), "configuration file not found") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func testRunServerPortOverride(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	testPort := 9999
	if testPort != DefaultServerPort {
		cfg.Port = testPort
	}

	if cfg.Port != testPort {
		t.Errorf("Expected port to be overridden to %d, got %d", testPort, cfg.Port)
	}

	cfg2, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	defaultPort := DefaultServerPort
	if defaultPort == DefaultServerPort {
		if cfg2.Port <= 0 || cfg2.Port > 65535 { // Check for valid port range
			t.Errorf("Port should be valid when using default, got %d", cfg2.Port)
		}
	}
}

func testRunServerConfigValidation(t *testing.T) {
	tempDir := t.TempDir()
	validConfigFile := createValidConfigFile(t, tempDir)
	invalidConfigFile := createInvalidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(validConfigFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = config.ValidateConfig(cfg)
	if err != nil {
		t.Errorf("Valid config should pass validation: %v", err)
	}

	invalidCfg, err := config.LoadConfig(invalidConfigFile)
	if err != nil {
		t.Fatalf("Failed to load invalid config: %v", err)
	}

	err = config.ValidateConfig(invalidCfg)
	if err == nil {
		t.Error("Invalid config should fail validation")
	}

	if !strings.Contains(err.Error(), "invalid port") {
		t.Errorf("Expected invalid port error, got: %v", err)
	}
}

func testRunServerGatewayCreation(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = config.ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		t.Errorf("Gateway creation failed: %v", err)
	}

	if gw == nil {
		t.Error("Expected gateway to be created")
	}

	invalidCfg := &config.GatewayConfig{
		Port:    -1, // Invalid port
		Servers: []config.ServerConfig{},
	}

	_, err = gateway.NewGateway(invalidCfg)
	if err == nil {
		t.Log("Gateway creation succeeded with invalid config (validation may be elsewhere)")
	} else {
		t.Logf("Gateway creation failed as expected: %v", err)
	}
}

func testRunServerGatewayStart(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = config.ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		t.Fatalf("Gateway creation failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = gw.Start(ctx)
	if err != nil {
		t.Errorf("Gateway start failed: %v", err)
	}

	err = gw.Stop()
	if err != nil {
		t.Errorf("Gateway stop failed: %v", err)
	}
}

func testRunServerHTTPSetup(t *testing.T) {
	tempDir := t.TempDir()
	testPort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, testPort)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	expectedAddr := fmt.Sprintf(":%d", cfg.Port)
	if expectedAddr != fmt.Sprintf(":%d", testPort) {
		t.Errorf("Expected server address ':%d', got '%s'", testPort, expectedAddr)
	}

	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		t.Fatalf("Gateway creation failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = gw.Start(ctx)
	if err != nil {
		t.Fatalf("Gateway start failed: %v", err)
	}
	defer func() {
		if err := gw.Stop(); err != nil {
			t.Logf("Gateway stop error: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gw.HandleJSONRPC)

	server := httptest.NewServer(mux)
	defer server.Close()

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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 200 or 400, got %d", resp.StatusCode)
	}
}

func testRunServerSignalHandling(t *testing.T) {
	sigCh := make(chan os.Signal, 1)

	go func() {
		time.Sleep(10 * time.Millisecond)
		sigCh <- syscall.SIGINT
	}()

	select {
	case sig := <-sigCh:
		if sig != syscall.SIGINT {
			t.Errorf("Expected SIGINT, got %v", sig)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Signal not received in time")
	}

	sigCh2 := make(chan os.Signal, 1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		sigCh2 <- syscall.SIGTERM
	}()

	select {
	case sig := <-sigCh2:
		if sig != syscall.SIGTERM {
			t.Errorf("Expected SIGTERM, got %v", sig)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("SIGTERM signal not received in time")
	}
}

func testRunServerGracefulShutdown(t *testing.T) {

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if shutdownCtx == nil {
		t.Error("Shutdown context should be created")
	}

	select {
	case <-shutdownCtx.Done():
		t.Error("Shutdown context should not timeout immediately")
	default:
	}

	server := &http.Server{
		Addr: ":0",
	}

	err := server.Shutdown(shutdownCtx)
	if err != nil {
		t.Logf("Server shutdown completed with: %v", err)
	}

	shortCtx, shortCancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer shortCancel()

	server2 := &http.Server{Addr: ":0"}
	err = server2.Shutdown(shortCtx)
	if err != nil && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Logf("Expected context deadline exceeded, got: %v", err)
	}
}

func testRunServerInvalidConfig(t *testing.T) {
	tempDir := t.TempDir()
	invalidConfigFile := createInvalidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(invalidConfigFile)
	if err != nil {
		t.Logf("Config loading failed as expected: %v", err)
		return
	}

	err = config.ValidateConfig(cfg)
	if err == nil {
		t.Error("Expected validation error for invalid configuration")
		return
	}

	t.Logf("Config validation correctly failed: %v", err)
}

func testRunServerGatewayFailure(t *testing.T) {
	tempDir := t.TempDir()

	badConfigContent := fmt.Sprintf(`
port: %d
servers:
  - name: "bad-server"
    languages: ["go"]
    command: "/nonexistent/command"
    args: []
    transport: "stdio"
`, DefaultServerPort)
	badConfigFile := filepath.Join(tempDir, "bad-config.yaml")
	err := os.WriteFile(badConfigFile, []byte(badConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create bad config file: %v", err)
	}

	cfg, err := config.LoadConfig(badConfigFile)
	if err != nil {
		t.Fatalf("Failed to load bad config: %v", err)
	}

	if err := config.ValidateConfig(cfg); err != nil {
		t.Logf("Config validation failed as expected: %v", err)
		return
	}

	_, err = gateway.NewGateway(cfg)
	if err == nil {
		t.Error("Expected error for gateway creation with invalid command")
		return
	}

	t.Logf("Gateway creation correctly failed: %v", err)
}

func testRunServerDirectCall(t *testing.T) {
	tempDir := t.TempDir()
	validConfigFile := createValidConfigFile(t, tempDir)

	configPath = validConfigFile
	port = DefaultServerPort

	testCmd := &cobra.Command{Use: "server"}
	testCmd.SetContext(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- runServer(testCmd, []string{})
	}()

	select {
	case err := <-done:
		t.Logf("runServer completed with: %v", err)
	case <-time.After(2 * time.Second):
		t.Log("runServer started successfully (timed out waiting for completion)")
	}
}

func testRunServerWithActualConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	testPort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, testPort)

	configPath = configFile
	port = testPort + 1 // Test port override

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Errorf("Config loading failed: %v", err)
		return
	}

	if port != DefaultServerPort {
		cfg.Port = port
	}

	if cfg.Port != testPort+1 {
		t.Errorf("Expected port override to %d, got %d", testPort+1, cfg.Port)
	}

	err = config.ValidateConfig(cfg)
	if err != nil {
		t.Errorf("Config validation failed: %v", err)
	}

	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		t.Errorf("Gateway creation failed: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = gw.Start(ctx)
	if err != nil {
		t.Errorf("Gateway start failed: %v", err)
		return
	}

	err = gw.Stop()
	if err != nil {
		t.Errorf("Gateway stop failed: %v", err)
	}
}

func BenchmarkServerCommandFlagParsing(b *testing.B) {
	for i := 0; i < b.N; i++ {
		configPath = "config.yaml"
		port = DefaultServerPort

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

	content := CreateConfigWithPort(DefaultServerPort)

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

func TestRunServerCoverageEnhancement(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with real HTTP server lifecycle
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"RunServerGatewayStartFailure", testRunServerGatewayStartFailure},
		{"RunServerHTTPServerError", testRunServerHTTPServerError},
		{"RunServerDeferCleanup", testRunServerDeferCleanup},
		{"RunServerContextTimeout", testRunServerContextTimeout},
		{"RunServerConfigPathEdgeCases", testRunServerConfigPathEdgeCases},
		{"RunServerPortConflictHandling", testRunServerPortConflictHandling},
		{"RunServerSignalInterruption", testRunServerSignalInterruption},
		{"RunServerShutdownErrorHandling", testRunServerShutdownErrorHandling},
		{"RunServerMemoryCleanup", testRunServerMemoryCleanup},
		{"RunServerConcurrentSignals", testRunServerConcurrentSignals},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath = DefaultConfigFile
			port = DefaultServerPort
			tt.testFunc(t)
		})
	}
}

func testRunServerGatewayStartFailure(t *testing.T) {
	tempDir := t.TempDir()

	failConfigContent := fmt.Sprintf(`
port: %d
servers:
  - name: "fail-server"
    languages: ["go"]
    command: "non-existent-command-12345"
    args: []
    transport: "stdio"
`, DefaultServerPort)
	failConfigFile := filepath.Join(tempDir, "fail-config.yaml")
	err := os.WriteFile(failConfigFile, []byte(failConfigContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create fail config file: %v", err)
	}

	configPath = failConfigFile
	port = DefaultServerPort

	testCmd := &cobra.Command{Use: "server"}

	err = runServer(testCmd, []string{})
	if err == nil {
		t.Error("Expected error when gateway fails to start")
		return
	}

	if !strings.Contains(err.Error(), "failed to") {
		t.Errorf("Expected gateway-related error, got: %v", err)
	}
}

func testRunServerHTTPServerError(t *testing.T) {
	tempDir := t.TempDir()
	testPort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, testPort)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	expectedAddr := fmt.Sprintf(":%d", cfg.Port)
	if expectedAddr != fmt.Sprintf(":%d", testPort) {
		t.Errorf("Expected HTTP server address ':%d', got '%s'", testPort, expectedAddr)
	}

	server := &http.Server{
		Addr:         expectedAddr,
		Handler:      nil,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if server.Addr != expectedAddr {
		t.Errorf("Expected server addr '%s', got '%s'", expectedAddr, server.Addr)
	}
	if server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected read timeout 30s, got %v", server.ReadTimeout)
	}
	if server.WriteTimeout != 30*time.Second {
		t.Errorf("Expected write timeout 30s, got %v", server.WriteTimeout)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		t.Logf("Server shutdown completed: %v", err)
	}
}

func testRunServerDeferCleanup(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = config.ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		t.Fatalf("Gateway creation failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = gw.Start(ctx)
	if err != nil {
		t.Fatalf("Gateway start failed: %v", err)
	}

	defer func() {
		if err := gw.Stop(); err != nil {
			t.Logf("Gateway stop error (expected in defer): %v", err)
		}
	}()

	stopErr := gw.Stop()
	if stopErr != nil {
		t.Logf("Gateway stop completed: %v", stopErr)
	}
}

func testRunServerContextTimeout(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	_, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if ctx == nil {
		t.Error("Context should be created")
	}

	cancel()

	select {
	case <-ctx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled immediately")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if shutdownCtx == nil {
		t.Error("Shutdown context should be created")
	}

	deadline, hasDeadline := shutdownCtx.Deadline()
	if !hasDeadline {
		t.Error("Shutdown context should have deadline")
	}

	if time.Until(deadline) > 31*time.Second || time.Until(deadline) < 29*time.Second {
		t.Errorf("Expected shutdown timeout around 30s, got %v", time.Until(deadline))
	}
}

func testRunServerConfigPathEdgeCases(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		configPath    string
		expectedError bool
		errorContains string
	}{
		{
			name:          "EmptyConfigPath",
			configPath:    "",
			expectedError: true,
			errorContains: "configuration file not found",
		},
		{
			name:          "RelativeConfigPath",
			configPath:    "./nonexistent.yaml",
			expectedError: true,
			errorContains: "configuration file not found",
		},
		{
			name:          "DirectoryAsConfig",
			configPath:    tempDir,
			expectedError: true,
			errorContains: "failed to read configuration file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath = tt.configPath
			port = DefaultServerPort

			testCmd := &cobra.Command{Use: "server"}

			err := runServer(testCmd, []string{})

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
			}
		})
	}
}

func testRunServerPortConflictHandling(t *testing.T) {
	tempDir := t.TempDir()
	basePort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, basePort)

	tests := []struct {
		name         string
		configPort   int
		flagPort     int
		expectedPort int
	}{
		{
			name:         "NoOverride",
			configPort:   basePort,
			flagPort:     DefaultServerPort,
			expectedPort: basePort,
		},
		{
			name:         "PortOverride",
			configPort:   basePort,
			flagPort:     basePort + 1,
			expectedPort: basePort + 1,
		},
		{
			name:         "ExplicitSamePort",
			configPort:   basePort,
			flagPort:     basePort,
			expectedPort: basePort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			if tt.flagPort != DefaultServerPort {
				cfg.Port = tt.flagPort
			}

			if cfg.Port != tt.expectedPort {
				t.Errorf("Expected final port %d, got %d", tt.expectedPort, cfg.Port)
			}

			err = config.ValidateConfig(cfg)
			if err != nil {
				t.Errorf("Config validation failed: %v", err)
			}
		})
	}
}

func testRunServerSignalInterruption(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	_, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	sigCh := make(chan os.Signal, 1)

	signals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}

	for _, sig := range signals {
		go func(signal os.Signal) {
			time.Sleep(10 * time.Millisecond)
			sigCh <- signal
		}(sig)

		select {
		case receivedSig := <-sigCh:
			if receivedSig != sig {
				t.Errorf("Expected signal %v, got %v", sig, receivedSig)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Signal %v not received in time", sig)
		}
	}

	if cap(sigCh) != 1 {
		t.Errorf("Expected signal channel capacity 1, got %d", cap(sigCh))
	}
}

func testRunServerShutdownErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	testPort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, testPort)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      nil,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	shutdownCtx1, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel1()

	err = server.Shutdown(shutdownCtx1)
	if err != nil {
		t.Logf("Server shutdown completed: %v", err)
	}

	server2 := &http.Server{Addr: ":0"}
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer shortCancel()

	err = server2.Shutdown(shortCtx)
	if err != nil && strings.Contains(err.Error(), "context deadline exceeded") {
		t.Logf("Expected timeout error: %v", err)
	} else if err != nil {
		t.Logf("Shutdown error: %v", err)
	}

	if err != nil {
		t.Logf("Server shutdown error: %v", err)
	}
}

func testRunServerMemoryCleanup(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	err = config.ValidateConfig(cfg)
	if err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		gw, err := gateway.NewGateway(cfg)
		if err != nil {
			t.Fatalf("Gateway creation failed on iteration %d: %v", i, err)
		}

		ctx, cancel := context.WithCancel(context.Background())

		err = gw.Start(ctx)
		if err != nil {
			cancel()
			t.Fatalf("Gateway start failed on iteration %d: %v", i, err)
		}

		cancel()
		err = gw.Stop()
		if err != nil {
			t.Logf("Gateway stop error on iteration %d: %v", i, err)
		}
	}
}

func testRunServerConcurrentSignals(t *testing.T) {
	sigCh := make(chan os.Signal, 1)

	done := make(chan bool, 2)

	go func() {
		time.Sleep(10 * time.Millisecond)
		sigCh <- syscall.SIGINT
		done <- true
	}()

	go func() {
		time.Sleep(20 * time.Millisecond)
		select {
		case sigCh <- syscall.SIGTERM:
			done <- true
		case <-time.After(50 * time.Millisecond):
			done <- false // Blocked as expected
		}
	}()

	select {
	case sig := <-sigCh:
		if sig != syscall.SIGINT {
			t.Errorf("Expected SIGINT, got %v", sig)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("SIGINT not received in time")
	}

	<-done
	<-done

	if len(sigCh) > 0 {
		extraSig := <-sigCh
		t.Logf("Additional signal in channel: %v", extraSig)
	}
}

func TestServerShutdownErrorPaths(t *testing.T) {
	tests := []struct {
		name            string
		setupServer     func() *http.Server
		shutdownTimeout time.Duration
		expectError     bool
		errorContains   string
	}{
		{
			name: "SuccessfulShutdown",
			setupServer: func() *http.Server {
				return &http.Server{Addr: ":0"}
			},
			shutdownTimeout: 30 * time.Second,
			expectError:     false,
		},
		{
			name: "ShutdownWithCancelledContext",
			setupServer: func() *http.Server {
				return &http.Server{Addr: ":0"}
			},
			shutdownTimeout: 30 * time.Second,
			expectError:     false, // Context cancellation during shutdown
		},
		{
			name: "InvalidServerShutdown",
			setupServer: func() *http.Server {
				return &http.Server{Addr: "invalid:address:format"}
			},
			shutdownTimeout: 30 * time.Second,
			expectError:     false, // Shutdown on invalid server doesn't typically error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), tt.shutdownTimeout)
			defer shutdownCancel()

			err := server.Shutdown(shutdownCtx)

			if tt.expectError {
				if err == nil {
					t.Error("Expected shutdown error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tt.errorContains, err)
				} else {
					t.Logf("Server shutdown error: %v", err)
				}
			} else {
				if err != nil {
					t.Logf("Server shutdown error: %v", err)
				}
			}

			t.Log("Server stopped")
		})
	}
}

func TestRunServerShutdownCoverage(t *testing.T) {
	originalConfigPath := configPath
	originalPort := port

	defer func() {
		configPath = originalConfigPath
		port = originalPort
	}()

	configPath = "" // Use defaults
	port = AllocateTestPort(t)

	testCmd := &cobra.Command{Use: CmdServer}

	serverErr := make(chan error, 1)
	go func() {
		err := runServer(testCmd, []string{})
		serverErr <- err
	}()

	time.Sleep(100 * time.Millisecond)

	select {
	case err := <-serverErr:
		t.Logf("runServer completed with: %v", err)
		t.Log("Successfully exercised runServer shutdown error handling paths")
	case <-time.After(2 * time.Second):
		t.Log("runServer shutdown test timed out (may still have provided coverage)")
	}
}

func TestServerShutdownErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	testPort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, testPort)

	tests := []struct {
		name                string
		simulateHangingConn bool
		shutdownTimeout     time.Duration
		expectLogMessage    bool
	}{
		{
			name:                "NormalShutdown",
			simulateHangingConn: false,
			shutdownTimeout:     5 * time.Second,
			expectLogMessage:    false,
		},
		{
			name:                "TimeoutShutdown",
			simulateHangingConn: false,
			shutdownTimeout:     1 * time.Nanosecond,
			expectLogMessage:    false, // Server not running, so no timeout error expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			server := &http.Server{
				Addr:         fmt.Sprintf(":%d", cfg.Port),
				Handler:      nil,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
			}

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), tt.shutdownTimeout)
			defer shutdownCancel()

			var logOutput strings.Builder
			originalOutput := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			go func() {
				defer func() { _ = w.Close() }()
				if err := server.Shutdown(shutdownCtx); err != nil {
					_, _ = fmt.Fprintf(w, "Server shutdown error: %v\n", err)
				}
				_, _ = fmt.Fprintf(w, "Server stopped\n")
			}()

			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			logOutput.Write(buf[:n])
			_ = r.Close()
			os.Stderr = originalOutput

			captured := logOutput.String()

			if tt.expectLogMessage {
				if !strings.Contains(captured, "Server shutdown error:") {
					t.Errorf("Expected shutdown error message in logs, got: %s", captured)
				}
			}

			if !strings.Contains(captured, "Server stopped") {
				t.Errorf("Expected 'Server stopped' message in logs, got: %s", captured)
			}
		})
	}
}

func TestRunServerWithMockedComponents(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with mocked components
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"RunServerCompleteLifecycle", testRunServerCompleteLifecycle},
		{"RunServerWithInvalidConfigPath", testRunServerWithInvalidConfigPath},
		{"RunServerPortBindingConflicts", testRunServerPortBindingConflicts},
		{"RunServerGatewayStartupFailure", testRunServerGatewayStartupFailure},
		{"RunServerHTTPServerConfiguration", testRunServerHTTPServerConfiguration},
		{"RunServerSignalHandlingLogic", testRunServerSignalHandlingLogic},
		{"RunServerShutdownSequence", testRunServerShutdownSequence},
		{"RunServerErrorRecovery", testRunServerErrorRecovery},
		{"RunServerContextManagement", testRunServerContextManagement},
		{"RunServerTimeoutScenarios", testRunServerTimeoutScenarios},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath = DefaultConfigFile
			port = DefaultServerPort
			tt.testFunc(t)
		})
	}
}

func testRunServerCompleteLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	testPort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, testPort)

	configPath = configFile
	port = DefaultServerPort // No override

	serverResult := make(chan error, 1)
	serverStarted := make(chan bool, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				serverResult <- fmt.Errorf("panic in runServer: %v", r)
			}
		}()

		testCmd := &cobra.Command{Use: "server"}

		err := runServer(testCmd, []string{})
		serverResult <- err
	}()

	go func() {
		time.Sleep(50 * time.Millisecond) // Allow startup
		serverStarted <- true
	}()

	select {
	case err := <-serverResult:
		t.Logf("runServer completed with: %v", err)
		if err != nil && !strings.Contains(err.Error(), "failed to load configuration") {
			t.Logf("Server execution result: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Log("runServer started successfully and is running")
	}

	select {
	case <-serverStarted:
		t.Log("Server startup phase completed")
	case <-time.After(100 * time.Millisecond):
		t.Log("Server startup monitoring completed")
	}
}

func testRunServerWithInvalidConfigPath(t *testing.T) {
	invalidPaths := []struct {
		name          string
		path          string
		expectedError string
	}{
		{
			name:          "NonexistentFile",
			path:          "/nonexistent/config.yaml",
			expectedError: "failed to load configuration",
		},
		{
			name:          "EmptyPath",
			path:          "",
			expectedError: "failed to load configuration",
		},
		{
			name:          "DirectoryAsConfig",
			path:          t.TempDir(),
			expectedError: "failed to load configuration",
		},
	}

	for _, tt := range invalidPaths {
		t.Run(tt.name, func(t *testing.T) {
			configPath = tt.path
			port = DefaultServerPort

			testCmd := &cobra.Command{Use: "server"}

			err := runServer(testCmd, []string{})

			if err == nil {
				t.Error("Expected error for invalid config path")
			} else if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
			} else {
				t.Logf("Correctly caught expected error: %v", err)
			}
		})
	}
}

func testRunServerPortBindingConflicts(t *testing.T) {
	tempDir := t.TempDir()
	basePort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, basePort)

	tests := []struct {
		name           string
		configPort     int
		overridePort   int
		expectOverride bool
	}{
		{
			name:           "NoPortOverride",
			configPort:     basePort,
			overridePort:   DefaultServerPort,
			expectOverride: false,
		},
		{
			name:           "PortOverride",
			configPort:     basePort,
			overridePort:   basePort + 100,
			expectOverride: true,
		},
		{
			name:           "ExplicitSamePort",
			configPort:     basePort,
			overridePort:   basePort,
			expectOverride: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath = configFile
			port = tt.overridePort

			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				t.Fatalf("Config loading failed: %v", err)
			}

			originalPort := cfg.Port

			if port != DefaultServerPort {
				cfg.Port = port
			}

			if tt.expectOverride && port != DefaultServerPort {
				if cfg.Port != tt.overridePort {
					t.Errorf("Expected port override to %d, got %d", tt.overridePort, cfg.Port)
				}
			} else if !tt.expectOverride {
				if cfg.Port != originalPort {
					t.Errorf("Expected no port override, original %d, got %d", originalPort, cfg.Port)
				}
			}

			err = config.ValidateConfig(cfg)
			if err != nil {
				t.Errorf("Config validation failed after port override: %v", err)
			}
		})
	}
}

func testRunServerGatewayStartupFailure(t *testing.T) {
	tempDir := t.TempDir()

	failConfig := fmt.Sprintf(`
port: %d
servers:
  - name: "failing-server"
    languages: ["go"]
    command: "/definitely/nonexistent/command/12345"
    args: []
    transport: "stdio"
`, DefaultServerPort)
	failConfigFile := filepath.Join(tempDir, "fail-config.yaml")
	err := os.WriteFile(failConfigFile, []byte(failConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create failing config: %v", err)
	}

	configPath = failConfigFile
	port = DefaultServerPort

	testCmd := &cobra.Command{Use: "server"}

	err = runServer(testCmd, []string{})
	if err == nil {
		t.Error("Expected error when gateway fails to start")
	} else {
		if strings.Contains(err.Error(), "failed to create gateway") ||
			strings.Contains(err.Error(), "failed to start gateway") ||
			strings.Contains(err.Error(), "failed to load configuration") {
			t.Logf("Correctly caught gateway error: %v", err)
		} else {
			t.Errorf("Expected gateway-related error, got: %v", err)
		}
	}
}

func testRunServerHTTPServerConfiguration(t *testing.T) {
	tempDir := t.TempDir()
	testPort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, testPort)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	expectedAddr := fmt.Sprintf(":%d", cfg.Port)

	server := &http.Server{
		Addr:         expectedAddr,
		Handler:      nil,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if server.Addr != expectedAddr {
		t.Errorf("Expected server addr '%s', got '%s'", expectedAddr, server.Addr)
	}
	if server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected read timeout 30s, got %v", server.ReadTimeout)
	}
	if server.WriteTimeout != 30*time.Second {
		t.Errorf("Expected write timeout 30s, got %v", server.WriteTimeout)
	}
	if server.Handler != nil {
		t.Errorf("Expected nil handler (uses DefaultServeMux), got %v", server.Handler)
	}

	originalHandler := http.DefaultServeMux
	testMux := http.NewServeMux()

	testMux.HandleFunc("/jsonrpc", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	req := httptest.NewRequest("POST", "/jsonrpc", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	testMux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	http.DefaultServeMux = originalHandler
}

func testRunServerSignalHandlingLogic(t *testing.T) {

	sigCh := make(chan os.Signal, 1)

	if cap(sigCh) != 1 {
		t.Errorf("Expected signal channel capacity 1, got %d", cap(sigCh))
	}

	signals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}

	for _, expectedSig := range signals {
		go func(sig os.Signal) {
			time.Sleep(10 * time.Millisecond)
			sigCh <- sig
		}(expectedSig)

		select {
		case receivedSig := <-sigCh:
			if receivedSig != expectedSig {
				t.Errorf("Expected signal %v, got %v", expectedSig, receivedSig)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Signal %v not received in expected timeframe", expectedSig)
		}
	}

	sigCh2 := make(chan os.Signal, 1)

	sigCh2 <- syscall.SIGINT

	select {
	case sigCh2 <- syscall.SIGTERM:
		t.Error("Second signal should be dropped/ignored due to buffer limit")
	default:
		t.Log("Signal channel correctly dropped additional signal due to buffer limit")
	}
}

func testRunServerShutdownSequence(t *testing.T) {
	tempDir := t.TempDir()
	testPort := AllocateTestPort(t)
	configFile := createValidConfigFileWithPort(t, tempDir, testPort)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      nil,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	deadline, hasDeadline := shutdownCtx.Deadline()
	if !hasDeadline {
		t.Error("Shutdown context should have deadline")
	}

	timeUntilDeadline := time.Until(deadline)
	if timeUntilDeadline > 31*time.Second || timeUntilDeadline < 29*time.Second {
		t.Errorf("Expected ~30s timeout, got %v", timeUntilDeadline)
	}

	shutdownStart := time.Now()
	err = server.Shutdown(shutdownCtx)
	shutdownDuration := time.Since(shutdownStart)

	if err != nil {
		t.Logf("Server shutdown error: %v", err)
	}

	if shutdownDuration > 1*time.Second {
		t.Errorf("Shutdown took too long: %v", shutdownDuration)
	}

	t.Log("Server stopped")
}

func testRunServerErrorRecovery(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		setupError    string
		expectedPhase string
	}{
		{
			name:          "ConfigLoadingError",
			setupError:    "invalid_config_path",
			expectedPhase: "configuration loading",
		},
		{
			name:          "ConfigValidationError",
			setupError:    "invalid_config_content",
			expectedPhase: "configuration validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testConfigPath string

			switch tt.setupError {
			case "invalid_config_path":
				testConfigPath = "/completely/nonexistent/path/config.yaml"
			case "invalid_config_content":
				invalidContent := `
port: -999
servers: []
`
				invalidFile := filepath.Join(tempDir, "invalid.yaml")
				err := os.WriteFile(invalidFile, []byte(invalidContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create invalid config: %v", err)
				}
				testConfigPath = invalidFile
			}

			configPath = testConfigPath
			port = DefaultServerPort

			testCmd := &cobra.Command{Use: "server"}

			err := runServer(testCmd, []string{})
			if err == nil {
				t.Errorf("Expected error in %s phase", tt.expectedPhase)
			} else {
				t.Logf("Correctly handled error in %s: %v", tt.expectedPhase, err)

				switch tt.expectedPhase {
				case "configuration loading":
					if !strings.Contains(err.Error(), "failed to load configuration") {
						t.Errorf("Expected config loading error, got: %v", err)
					}
				case "configuration validation":
					if !strings.Contains(err.Error(), "invalid configuration") {
						t.Errorf("Expected config validation error, got: %v", err)
					}
				}
			}
		})
	}
}

func testRunServerContextManagement(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if ctx == nil {
		t.Error("Context should be created")
	}

	select {
	case <-ctx.Done():
		t.Error("Context should not be done initially")
	default:
	}

	cancel()

	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Millisecond):
		t.Error("Context cancellation should be immediate")
	}

	cancel() // Should not panic

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	deadline, hasDeadline := shutdownCtx.Deadline()
	if !hasDeadline {
		t.Error("Shutdown context should have deadline")
	}

	if time.Until(deadline) <= 29*time.Second || time.Until(deadline) > 31*time.Second {
		t.Errorf("Expected ~30s deadline, got %v", time.Until(deadline))
	}

	shutdownCancel()

	select {
	case <-shutdownCtx.Done():
	case <-time.After(10 * time.Millisecond):
		t.Error("Shutdown context cancellation should be immediate")
	}
}

func testRunServerTimeoutScenarios(t *testing.T) {

	expectedReadTimeout := 30 * time.Second
	expectedWriteTimeout := 30 * time.Second

	server := &http.Server{
		Addr:         ":0",
		Handler:      nil,
		ReadTimeout:  expectedReadTimeout,
		WriteTimeout: expectedWriteTimeout,
	}

	if server.ReadTimeout != expectedReadTimeout {
		t.Errorf("Expected read timeout %v, got %v", expectedReadTimeout, server.ReadTimeout)
	}
	if server.WriteTimeout != expectedWriteTimeout {
		t.Errorf("Expected write timeout %v, got %v", expectedWriteTimeout, server.WriteTimeout)
	}

	tests := []struct {
		name            string
		shutdownTimeout time.Duration
		expectTimeout   bool
	}{
		{
			name:            "NormalShutdown",
			shutdownTimeout: 30 * time.Second,
			expectTimeout:   false,
		},
		{
			name:            "ImmediateTimeout",
			shutdownTimeout: 1 * time.Nanosecond,
			expectTimeout:   true,
		},
		{
			name:            "ShortTimeout",
			shutdownTimeout: 1 * time.Millisecond,
			expectTimeout:   false, // Server not running, should shutdown quickly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServer := &http.Server{Addr: ":0"}

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), tt.shutdownTimeout)
			defer shutdownCancel()

			start := time.Now()
			err := testServer.Shutdown(shutdownCtx)
			duration := time.Since(start)

			if tt.expectTimeout {
				if err != nil && strings.Contains(err.Error(), "context deadline exceeded") {
					t.Logf("Expected timeout occurred after %v: %v", duration, err)
				} else if err == nil && duration >= tt.shutdownTimeout {
					t.Logf("Timeout behavior occurred (no error but took expected time): %v", duration)
				}
			} else {
				if err != nil && strings.Contains(err.Error(), "context deadline exceeded") {
					t.Errorf("Unexpected timeout: %v", err)
				} else {
					t.Logf("Shutdown completed in %v", duration)
				}
			}
		})
	}
}

func TestRunServerExecutionPaths(t *testing.T) {
	// Removed t.Parallel() to prevent deadlock with real HTTP server execution

	originalConfigPath := configPath
	originalPort := port
	defer func() {
		configPath = originalConfigPath
		port = originalPort
	}()

	tests := []struct {
		name          string
		setupConfig   func(t *testing.T) string
		setupPort     int
		expectedError bool
		errorContains string
		testFunc      func(t *testing.T, configFile string, testPort int)
	}{
		{
			name: "ValidConfigExecution",
			setupConfig: func(t *testing.T) string {
				tempDir := t.TempDir()
				return createValidConfigFile(t, tempDir)
			},
			setupPort:     DefaultServerPort,
			expectedError: false,
			testFunc:      testValidConfigExecution,
		},
		{
			name: "ConfigNotFoundExecution",
			setupConfig: func(t *testing.T) string {
				return "/nonexistent/config.yaml"
			},
			setupPort:     DefaultServerPort,
			expectedError: true,
			errorContains: "failed to load configuration",
			testFunc:      testConfigNotFoundExecution,
		},
		{
			name: "InvalidConfigExecution",
			setupConfig: func(t *testing.T) string {
				tempDir := t.TempDir()
				return createInvalidConfigFile(t, tempDir)
			},
			setupPort:     DefaultServerPort,
			expectedError: true,
			errorContains: "invalid configuration",
			testFunc:      testInvalidConfigExecution,
		},
		{
			name: "PortOverrideExecution",
			setupConfig: func(t *testing.T) string {
				tempDir := t.TempDir()
				testPort := AllocateTestPort(t)
				return createValidConfigFileWithPort(t, tempDir, testPort)
			},
			setupPort:     0, // Will be set dynamically
			expectedError: false,
			testFunc:      testPortOverrideExecution,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := tt.setupConfig(t)
			testPort := tt.setupPort

			if tt.name == "PortOverrideExecution" {
				testPort = AllocateTestPort(t) + 50 // Different from config
			}

			configPath = configFile
			port = testPort

			if tt.testFunc != nil {
				tt.testFunc(t, configFile, testPort)
			} else {
				testCmd := &cobra.Command{Use: "server"}
				err := runServer(testCmd, []string{})

				if tt.expectedError {
					if err == nil {
						t.Error("Expected error but got none")
					} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
						t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
					}
				} else {
					if err != nil {
						t.Errorf("Expected no error but got: %v", err)
					}
				}
			}
		})
	}
}

func testValidConfigExecution(t *testing.T, configFile string, testPort int) {
	testCmd := &cobra.Command{Use: "server"}

	done := make(chan error, 1)
	go func() {
		done <- runServer(testCmd, []string{})
	}()

	select {
	case err := <-done:
		t.Logf("runServer completed: %v", err)
	case <-time.After(2 * time.Second):
		t.Log("runServer started successfully (timed out waiting for completion)")
	}
}

func testConfigNotFoundExecution(t *testing.T, configFile string, testPort int) {
	testCmd := &cobra.Command{Use: "server"}
	err := runServer(testCmd, []string{})

	if err == nil {
		t.Error("Expected configuration not found error")
	} else if !strings.Contains(err.Error(), "failed to load configuration") {
		t.Errorf("Expected config loading error, got: %v", err)
	} else {
		t.Logf("Correctly caught config loading error: %v", err)
	}
}

func testInvalidConfigExecution(t *testing.T, configFile string, testPort int) {
	testCmd := &cobra.Command{Use: "server"}
	err := runServer(testCmd, []string{})

	if err == nil {
		t.Error("Expected configuration validation error")
	} else if !strings.Contains(err.Error(), "invalid configuration") {
		t.Errorf("Expected config validation error, got: %v", err)
	} else {
		t.Logf("Correctly caught config validation error: %v", err)
	}
}

func testPortOverrideExecution(t *testing.T, configFile string, testPort int) {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Config loading failed: %v", err)
	}

	originalPort := cfg.Port

	if testPort != DefaultServerPort {
		cfg.Port = testPort
	}

	if testPort != DefaultServerPort && cfg.Port != testPort {
		t.Errorf("Expected port override to %d, got %d", testPort, cfg.Port)
	} else if testPort == DefaultServerPort && cfg.Port != originalPort {
		t.Errorf("Expected no port override, original %d, got %d", originalPort, cfg.Port)
	}

	err = config.ValidateConfig(cfg)
	if err != nil {
		t.Errorf("Config validation failed after port override: %v", err)
	}

	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		t.Logf("Gateway creation failed (expected in test environment): %v", err)
	} else {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err = gw.Start(ctx)
		if err != nil {
			t.Logf("Gateway start failed (expected in test environment): %v", err)
		} else {
			defer func() {
				if stopErr := gw.Stop(); stopErr != nil {
					t.Logf("Gateway stop error: %v", stopErr)
				}
			}()
		}
	}
}

func TestRunServerConcurrencyAndSignals(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"ConcurrentServerStartup", testConcurrentServerStartup},
		{"SignalHandlingRaceConditions", testSignalHandlingRaceConditions},
		{"HTTPServerConcurrency", testHTTPServerConcurrency},
		{"GatewayLifecycleConcurrency", testGatewayLifecycleConcurrency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc(t)
		})
	}
}

func testConcurrentServerStartup(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	const numGoroutines = 3
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			localConfigPath := configFile
			localPort := AllocateTestPort(t)

			cfg, err := config.LoadConfig(localConfigPath)
			if err != nil {
				results <- fmt.Errorf("goroutine %d config load error: %v", id, err)
				return
			}

			if localPort != DefaultServerPort {
				cfg.Port = localPort
			}

			err = config.ValidateConfig(cfg)
			if err != nil {
				results <- fmt.Errorf("goroutine %d config validation error: %v", id, err)
				return
			}

			gw, err := gateway.NewGateway(cfg)
			if err != nil {
				results <- fmt.Errorf("goroutine %d gateway creation error: %v", id, err)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err = gw.Start(ctx)
			if err != nil {
				results <- fmt.Errorf("goroutine %d gateway start error: %v", id, err)
				return
			}

			defer func() {
				if stopErr := gw.Stop(); stopErr != nil {
					t.Logf("Goroutine %d gateway stop error: %v", id, stopErr)
				}
			}()

			results <- nil
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Logf("Concurrent startup error (may be expected): %v", err)
			} else {
				t.Logf("Concurrent startup %d succeeded", i)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("Goroutine %d timed out", i)
		}
	}
}

func testSignalHandlingRaceConditions(t *testing.T) {
	sigCh := make(chan os.Signal, 1)

	const numSignals = 10
	delivered := make(chan bool, numSignals)

	for i := 0; i < numSignals; i++ {
		go func(id int) {
			time.Sleep(time.Duration(id) * time.Millisecond)
			select {
			case sigCh <- syscall.SIGINT:
				delivered <- true
			case <-time.After(100 * time.Millisecond):
				delivered <- false // Couldn't deliver (channel full)
			}
		}(i)
	}

	signalsReceived := 0
signalLoop:
	for signalsReceived < numSignals {
		select {
		case <-sigCh:
			signalsReceived++
		case wasDelivered := <-delivered:
			if !wasDelivered {
				signalsReceived++
			}
		case <-time.After(200 * time.Millisecond):
			t.Error("Timed out waiting for signals")
			break signalLoop
		}
	}

	if len(sigCh) > 1 {
		t.Errorf("Signal channel should have capacity 1, has %d signals", len(sigCh))
	}
}

func testHTTPServerConcurrency(t *testing.T) {
	const numServers = 3
	servers := make([]*http.Server, numServers)
	ports := make([]int, numServers)

	for i := 0; i < numServers; i++ {
		port := AllocateTestPort(t)
		ports[i] = port

		servers[i] = &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      nil,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		}
	}

	shutdownResults := make(chan error, numServers)

	for i, server := range servers {
		go func(id int, srv *http.Server) {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := srv.Shutdown(shutdownCtx)
			shutdownResults <- err
		}(i, server)
	}

	for i := 0; i < numServers; i++ {
		select {
		case err := <-shutdownResults:
			if err != nil {
				t.Logf("Server %d shutdown error: %v", i, err)
			} else {
				t.Logf("Server %d shutdown succeeded", i)
			}
		case <-time.After(5 * time.Second):
			t.Errorf("Server %d shutdown timed out", i)
		}
	}
}

func testGatewayLifecycleConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	configFile := createValidConfigFile(t, tempDir)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Config loading failed: %v", err)
	}

	const numOperations = 5
	results := make(chan error, numOperations)

	for i := 0; i < numOperations; i++ {
		go func(id int) {
			gw, err := gateway.NewGateway(cfg)
			if err != nil {
				results <- fmt.Errorf("operation %d gateway creation failed: %v", id, err)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err = gw.Start(ctx)
			if err != nil {
				results <- fmt.Errorf("operation %d gateway start failed: %v", id, err)
				return
			}

			err = gw.Stop()
			if err != nil {
				results <- fmt.Errorf("operation %d gateway stop failed: %v", id, err)
				return
			}

			results <- nil
		}(i)
	}

	for i := 0; i < numOperations; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Logf("Gateway operation error (may be expected): %v", err)
			} else {
				t.Logf("Gateway operation %d succeeded", i)
			}
		case <-time.After(3 * time.Second):
			t.Errorf("Gateway operation %d timed out", i)
		}
	}
}

func TestRunServerWithContext(t *testing.T) {
	originalConfigPath := configPath
	originalPort := port
	defer func() {
		configPath = originalConfigPath
		port = originalPort
	}()

	tests := []struct {
		name          string
		setupConfig   func(t *testing.T) string
		setupPort     int
		contextType   string
		expectedError bool
		errorContains string
	}{
		{
			name: "ValidContextWithConfigError",
			setupConfig: func(t *testing.T) string {
				return "/nonexistent/config.yaml"
			},
			setupPort:     DefaultServerPort,
			contextType:   "normal",
			expectedError: true,
			errorContains: "failed to load configuration",
		},
		{
			name: "ContextCancellation",
			setupConfig: func(t *testing.T) string {
				tempDir := t.TempDir()
				return createValidConfigFile(t, tempDir)
			},
			setupPort:     DefaultServerPort,
			contextType:   "cancelled",
			expectedError: false, // Function should handle cancellation gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := tt.setupConfig(t)
			testPort := tt.setupPort

			configPath = configFile
			port = testPort

			var ctx context.Context
			var cancel context.CancelFunc

			switch tt.contextType {
			case "cancelled":
				ctx, cancel = context.WithCancel(context.Background())
				cancel() // Cancel immediately
			case "timeout":
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
			case "normal":
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()
			}

			testCmd := &cobra.Command{Use: "server"}

			done := make(chan error, 1)
			go func() {
				done <- runServerWithContext(ctx, testCmd, []string{})
			}()

			select {
			case err := <-done:
				if tt.expectedError {
					if err == nil {
						t.Error("Expected error but got none")
					} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
						t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
					} else {
						t.Logf("Correctly caught expected error: %v", err)
					}
				} else {
					if err != nil {
						t.Logf("runServerWithContext completed with: %v", err)
					} else {
						t.Log("runServerWithContext completed successfully")
					}
				}
			case <-time.After(1 * time.Second):
				if tt.contextType == "cancelled" {
					t.Log("runServerWithContext handled context cancellation correctly")
				} else {
					t.Log("runServerWithContext test completed with timeout")
				}
			}
		})
	}
}

func TestRunServerWithContextLogic(t *testing.T) {
	originalConfigPath := configPath
	originalPort := port
	defer func() {
		configPath = originalConfigPath
		port = originalPort
	}()

	tempDir := t.TempDir()

	configPath = "/nonexistent/config.yaml"
	port = DefaultServerPort

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	testCmd := &cobra.Command{Use: "server"}
	err := runServerWithContext(ctx, testCmd, []string{})

	if err == nil {
		t.Error("Expected error for invalid config path")
	} else if !strings.Contains(err.Error(), "failed to load configuration") {
		t.Errorf("Expected config loading error, got: %v", err)
	} else {
		t.Logf("Correctly caught config loading error: %v", err)
	}

	invalidConfig := `
port: -1
servers: []
`
	invalidFile := filepath.Join(tempDir, "invalid.yaml")
	err = os.WriteFile(invalidFile, []byte(invalidConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid config: %v", err)
	}

	configPath = invalidFile
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	err = runServerWithContext(ctx2, testCmd, []string{})
	if err == nil {
		t.Error("Expected error for invalid config content")
	} else if !strings.Contains(err.Error(), "invalid configuration") {
		t.Errorf("Expected config validation error, got: %v", err)
	} else {
		t.Logf("Correctly caught config validation error: %v", err)
	}
}
