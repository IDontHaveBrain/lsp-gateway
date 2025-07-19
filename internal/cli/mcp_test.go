package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// allocateTestPort returns a dynamically allocated port for testing
func allocateTestPort(t *testing.T) int {
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

// allocateTestPortBench returns a dynamically allocated port for benchmarking
func allocateTestPortBench(b *testing.B) int {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		b.Fatalf("Failed to allocate test port: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			b.Logf("Error closing test listener: %v", err)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func TestMCPCommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"Metadata", testMCPCommandMetadata},
		{"FlagParsing", testMCPCommandFlagParsing},
		{"ConfigurationValidation", testMCPCommandConfigurationValidation},
		{"TransportTypes", testMCPCommandTransportTypes},
		{"ErrorScenarios", testMCPCommandErrorScenarios},
		{"CommandExecution", testMCPCommandExecution},
		{"Help", testMCPCommandHelp},
		{"Integration", testMCPCommandIntegration},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables before each test
			mcpConfigPath = ""
			mcpGatewayURL = DefaultLSPGatewayURL
			mcpPort = 3000
			mcpTransport = transport.TransportStdio
			mcpTimeout = 30 * time.Second
			mcpMaxRetries = 3
			tt.testFunc(t)
		})
	}
}

func testMCPCommandMetadata(t *testing.T) {
	if mcpCmd.Use != CmdMCP {
		t.Errorf("Expected Use to be 'mcp', got '%s'", mcpCmd.Use)
	}

	expectedShort := "Start the MCP server"
	if mcpCmd.Short != expectedShort {
		t.Errorf("Expected Short to be '%s', got '%s'", expectedShort, mcpCmd.Short)
	}

	if !strings.Contains(mcpCmd.Long, "Model Context Protocol") {
		t.Error("Expected Long description to mention Model Context Protocol")
	}

	if !strings.Contains(mcpCmd.Long, "LSP functionality") {
		t.Error("Expected Long description to mention LSP functionality")
	}

	// Verify RunE function is set
	if mcpCmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}

	// Verify Run function is not set (we use RunE)
	if mcpCmd.Run != nil {
		t.Error("Expected Run function to be nil (using RunE instead)")
	}
}

// testCaseExpectation holds expected values for a flag parsing test case
type testCaseExpectation struct {
	name               string
	args               []string
	expectedConfigPath string
	expectedGatewayURL string
	expectedPort       int
	expectedTransport  string
	expectedTimeout    time.Duration
	expectedMaxRetries int
	expectedError      bool
}

// runFlagParsingTestCase executes a single flag parsing test case
func runFlagParsingTestCase(t *testing.T, tc testCaseExpectation) {
	// Generate dynamic ports for tests with hardcoded ports
	args := tc.args
	expectedGatewayURL := tc.expectedGatewayURL
	if tc.name == "GatewayFlag" || tc.name == "AllFlags" {
		testPort := allocateTestPort(t)
		gatewayURL := fmt.Sprintf("http://localhost:%d", testPort)
		// Update args to use dynamic port
		for i, arg := range args {
			if arg == "--gateway" && i+1 < len(args) {
				args[i+1] = gatewayURL
				expectedGatewayURL = gatewayURL
				break
			}
		}
	}

	// Reset global variables
	mcpConfigPath = ""
	mcpGatewayURL = DefaultLSPGatewayURL
	mcpPort = 3000
	mcpTransport = transport.TransportStdio
	mcpTimeout = 30 * time.Second
	mcpMaxRetries = 3

	// Create a test command to avoid modifying global state
	testCmd := &cobra.Command{
		Use:   CmdMCP,
		Short: "Start the MCP server",
		Long:  "Start the Model Context Protocol (MCP) server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Just parse flags, don't execute
			return nil
		},
	}

	// Add flags to test command
	testCmd.Flags().StringVarP(&mcpConfigPath, "config", "c", "", "MCP configuration file path (optional)")
	testCmd.Flags().StringVarP(&mcpGatewayURL, "gateway", "g", DefaultLSPGatewayURL, "LSP Gateway URL")
	testCmd.Flags().IntVarP(&mcpPort, "port", "p", 3000, "MCP server port (for HTTP transport)")
	testCmd.Flags().StringVarP(&mcpTransport, "transport", "t", transport.TransportStdio, "Transport type (stdio, http)")
	testCmd.Flags().DurationVar(&mcpTimeout, "timeout", 30*time.Second, "Request timeout duration")
	testCmd.Flags().IntVar(&mcpMaxRetries, "max-retries", 3, "Maximum retries for failed requests")

	// Set arguments (potentially modified for dynamic ports)
	testCmd.SetArgs(args)

	// Execute flag parsing
	err := testCmd.Execute()

	if tc.expectedError && err == nil {
		t.Error("Expected error but got none")
	} else if !tc.expectedError && err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	// Check parsed values
	if mcpConfigPath != tc.expectedConfigPath {
		t.Errorf("Expected mcpConfigPath to be '%s', got '%s'", tc.expectedConfigPath, mcpConfigPath)
	}

	if mcpGatewayURL != expectedGatewayURL {
		t.Errorf("Expected mcpGatewayURL to be '%s', got '%s'", expectedGatewayURL, mcpGatewayURL)
	}

	if mcpPort != tc.expectedPort {
		t.Errorf("Expected mcpPort to be %d, got %d", tc.expectedPort, mcpPort)
	}

	if mcpTransport != tc.expectedTransport {
		t.Errorf("Expected mcpTransport to be '%s', got '%s'", tc.expectedTransport, mcpTransport)
	}

	if mcpTimeout != tc.expectedTimeout {
		t.Errorf("Expected mcpTimeout to be %v, got %v", tc.expectedTimeout, mcpTimeout)
	}

	if mcpMaxRetries != tc.expectedMaxRetries {
		t.Errorf("Expected mcpMaxRetries to be %d, got %d", tc.expectedMaxRetries, mcpMaxRetries)
	}
}

func testMCPCommandFlagParsing(t *testing.T) {
	tests := []testCaseExpectation{
		{
			name:               "DefaultFlags",
			args:               []string{},
			expectedConfigPath: "",
			expectedGatewayURL: DefaultLSPGatewayURL,
			expectedPort:       3000,
			expectedTransport:  transport.TransportStdio,
			expectedTimeout:    30 * time.Second,
			expectedMaxRetries: 3,
			expectedError:      false,
		},
		{
			name:               "ConfigFlag",
			args:               []string{"--config", "mcp-config.yaml"},
			expectedConfigPath: "mcp-config.yaml",
			expectedGatewayURL: DefaultLSPGatewayURL,
			expectedPort:       3000,
			expectedTransport:  transport.TransportStdio,
			expectedTimeout:    30 * time.Second,
			expectedMaxRetries: 3,
			expectedError:      false,
		},
		{
			name:               "GatewayFlag",
			args:               []string{"--gateway", "http://localhost:9090"},
			expectedConfigPath: "",
			expectedGatewayURL: "http://localhost:9090",
			expectedPort:       3000,
			expectedTransport:  transport.TransportStdio,
			expectedTimeout:    30 * time.Second,
			expectedMaxRetries: 3,
			expectedError:      false,
		},
		{
			name:               "PortFlag",
			args:               []string{"--port", "4000"},
			expectedConfigPath: "",
			expectedGatewayURL: DefaultLSPGatewayURL,
			expectedPort:       4000,
			expectedTransport:  transport.TransportStdio,
			expectedTimeout:    30 * time.Second,
			expectedMaxRetries: 3,
			expectedError:      false,
		},
		{
			name:               "TransportFlag",
			args:               []string{"--transport", "http"},
			expectedConfigPath: "",
			expectedGatewayURL: DefaultLSPGatewayURL,
			expectedPort:       3000,
			expectedTransport:  "http",
			expectedTimeout:    30 * time.Second,
			expectedMaxRetries: 3,
			expectedError:      false,
		},
		{
			name:               "TimeoutFlag",
			args:               []string{"--timeout", "60s"},
			expectedConfigPath: "",
			expectedGatewayURL: DefaultLSPGatewayURL,
			expectedPort:       3000,
			expectedTransport:  transport.TransportStdio,
			expectedTimeout:    60 * time.Second,
			expectedMaxRetries: 3,
			expectedError:      false,
		},
		{
			name:               "MaxRetriesFlag",
			args:               []string{"--max-retries", "5"},
			expectedConfigPath: "",
			expectedGatewayURL: DefaultLSPGatewayURL,
			expectedPort:       3000,
			expectedTransport:  transport.TransportStdio,
			expectedTimeout:    30 * time.Second,
			expectedMaxRetries: 5,
			expectedError:      false,
		},
		{
			name:               "AllFlags",
			args:               []string{"--config", "custom.yaml", "--gateway", "http://localhost:9090", "--port", "4000", "--transport", "http", "--timeout", "45s", "--max-retries", "2"},
			expectedConfigPath: "custom.yaml",
			expectedGatewayURL: "http://localhost:9090",
			expectedPort:       4000,
			expectedTransport:  "http",
			expectedTimeout:    45 * time.Second,
			expectedMaxRetries: 2,
			expectedError:      false,
		},
		{
			name:               "ShortFlags",
			args:               []string{"-c", "short.yaml", "-g", "http://localhost:7070", "-p", "5000", "-t", transport.TransportStdio},
			expectedConfigPath: "short.yaml",
			expectedGatewayURL: "http://localhost:7070",
			expectedPort:       5000,
			expectedTransport:  transport.TransportStdio,
			expectedTimeout:    30 * time.Second,
			expectedMaxRetries: 3,
			expectedError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runFlagParsingTestCase(t, tt)
		})
	}
}

func testMCPCommandConfigurationValidation(t *testing.T) {
	tests := []struct {
		name          string
		gatewayURL    string
		transport     string
		timeout       time.Duration
		maxRetries    int
		expectedError bool
		errorContains string
	}{
		{
			name:          "ValidConfig",
			gatewayURL:    DefaultLSPGatewayURL,
			transport:     transport.TransportStdio,
			timeout:       30 * time.Second,
			maxRetries:    3,
			expectedError: false,
		},
		{
			name:          "ValidHTTPTransport",
			gatewayURL:    DefaultLSPGatewayURL,
			transport:     "http",
			timeout:       30 * time.Second,
			maxRetries:    3,
			expectedError: false,
		},
		{
			name:          "EmptyGatewayURL",
			gatewayURL:    "",
			transport:     transport.TransportStdio,
			timeout:       30 * time.Second,
			maxRetries:    3,
			expectedError: true,
			errorContains: "LSP Gateway URL cannot be empty",
		},
		{
			name:          "InvalidTransport",
			gatewayURL:    DefaultLSPGatewayURL,
			transport:     "invalid",
			timeout:       30 * time.Second,
			maxRetries:    3,
			expectedError: true,
			errorContains: "invalid transport type",
		},
		{
			name:          "NegativeTimeout",
			gatewayURL:    DefaultLSPGatewayURL,
			transport:     transport.TransportStdio,
			timeout:       -1 * time.Second,
			maxRetries:    3,
			expectedError: true,
			errorContains: "timeout must be positive",
		},
		{
			name:          "NegativeMaxRetries",
			gatewayURL:    DefaultLSPGatewayURL,
			transport:     transport.TransportStdio,
			timeout:       30 * time.Second,
			maxRetries:    -1,
			expectedError: true,
			errorContains: "max retries cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &mcp.ServerConfig{
				Name:          "lsp-gateway-mcp",
				Description:   "MCP server providing LSP functionality through LSP Gateway",
				Version:       "0.1.0",
				LSPGatewayURL: tt.gatewayURL,
				Transport:     tt.transport,
				Timeout:       tt.timeout,
				MaxRetries:    tt.maxRetries,
			}

			err := cfg.Validate()

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

func testMCPCommandTransportTypes(t *testing.T) {
	tests := []struct {
		name      string
		transport string
		valid     bool
	}{
		{"StdioTransport", transport.TransportStdio, true},
		{"HTTPTransport", "http", true},
		{"WebSocketTransport", "websocket", true},
		{"InvalidTransport", "invalid", false},
		{"EmptyTransport", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &mcp.ServerConfig{
				Name:          "lsp-gateway-mcp",
				Description:   "MCP server providing LSP functionality through LSP Gateway",
				Version:       "0.1.0",
				LSPGatewayURL: DefaultLSPGatewayURL,
				Transport:     tt.transport,
				Timeout:       30 * time.Second,
				MaxRetries:    3,
			}

			err := cfg.Validate()

			if tt.valid && err != nil {
				t.Errorf("Expected transport '%s' to be valid, got error: %v", tt.transport, err)
			} else if !tt.valid && err == nil {
				t.Errorf("Expected transport '%s' to be invalid, but validation passed", tt.transport)
			}
		})
	}
}

func testMCPCommandErrorScenarios(t *testing.T) {
	tests := []struct {
		name          string
		setupError    func() error
		expectedError bool
		errorContains string
	}{
		{
			name: "ValidConfiguration",
			setupError: func() error {
				cfg := &mcp.ServerConfig{
					Name:          "lsp-gateway-mcp",
					Description:   "MCP server providing LSP functionality through LSP Gateway",
					Version:       "0.1.0",
					LSPGatewayURL: DefaultLSPGatewayURL,
					Transport:     transport.TransportStdio,
					Timeout:       30 * time.Second,
					MaxRetries:    3,
				}
				return cfg.Validate()
			},
			expectedError: false,
		},
		{
			name: "InvalidConfiguration",
			setupError: func() error {
				cfg := &mcp.ServerConfig{
					Name:          "",
					Description:   "MCP server providing LSP functionality through LSP Gateway",
					Version:       "0.1.0",
					LSPGatewayURL: "",
					Transport:     "invalid",
					Timeout:       -1 * time.Second,
					MaxRetries:    -1,
				}
				return cfg.Validate()
			},
			expectedError: true,
			errorContains: "server name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setupError()

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

func testMCPCommandExecution(t *testing.T) {
	// Verify command is properly registered
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == CmdMCP {
			found = true
			break
		}
	}

	if !found {
		t.Error("mcp command should be added to root command")
	}

	// Test flag definitions
	configFlag := mcpCmd.Flag("config")
	if configFlag == nil {
		t.Error("Expected config flag to be defined")
	} else {
		if configFlag.Shorthand != "c" {
			t.Errorf("Expected config flag shorthand to be 'c', got '%s'", configFlag.Shorthand)
		}
		if configFlag.DefValue != "" {
			t.Errorf("Expected config flag default to be empty, got '%s'", configFlag.DefValue)
		}
	}

	gatewayFlag := mcpCmd.Flag("gateway")
	if gatewayFlag == nil {
		t.Error("Expected gateway flag to be defined")
	} else {
		if gatewayFlag.Shorthand != "g" {
			t.Errorf("Expected gateway flag shorthand to be 'g', got '%s'", gatewayFlag.Shorthand)
		}
		if gatewayFlag.DefValue != DefaultLSPGatewayURL {
			t.Errorf("Expected gateway flag default to be 'http://localhost:8080', got '%s'", gatewayFlag.DefValue)
		}
	}

	portFlag := mcpCmd.Flag("port")
	if portFlag == nil {
		t.Error("Expected port flag to be defined")
	} else {
		if portFlag.Shorthand != "p" {
			t.Errorf("Expected port flag shorthand to be 'p', got '%s'", portFlag.Shorthand)
		}
		if portFlag.DefValue != "3000" {
			t.Errorf("Expected port flag default to be '3000', got '%s'", portFlag.DefValue)
		}
	}

	transportFlag := mcpCmd.Flag("transport")
	if transportFlag == nil {
		t.Error("Expected transport flag to be defined")
	} else {
		if transportFlag.Shorthand != "t" {
			t.Errorf("Expected transport flag shorthand to be 't', got '%s'", transportFlag.Shorthand)
		}
		if transportFlag.DefValue != transport.TransportStdio {
			t.Errorf("Expected transport flag default to be 'stdio', got '%s'", transportFlag.DefValue)
		}
	}

	timeoutFlag := mcpCmd.Flag("timeout")
	if timeoutFlag == nil {
		t.Error("Expected timeout flag to be defined")
	} else {
		if timeoutFlag.DefValue != "30s" {
			t.Errorf("Expected timeout flag default to be '30s', got '%s'", timeoutFlag.DefValue)
		}
	}

	maxRetriesFlag := mcpCmd.Flag("max-retries")
	if maxRetriesFlag == nil {
		t.Error("Expected max-retries flag to be defined")
	} else {
		if maxRetriesFlag.DefValue != "3" {
			t.Errorf("Expected max-retries flag default to be '3', got '%s'", maxRetriesFlag.DefValue)
		}
	}
}

func testMCPCommandHelp(t *testing.T) {
	// Test help output
	output := captureStdoutMCP(t, func() {
		testCmd := &cobra.Command{
			Use:   mcpCmd.Use,
			Short: mcpCmd.Short,
			Long:  mcpCmd.Long,
			RunE:  mcpCmd.RunE,
		}

		// Add flags to test command for help
		testCmd.Flags().StringVarP(&mcpConfigPath, "config", "c", "", "MCP configuration file path (optional)")
		testCmd.Flags().StringVarP(&mcpGatewayURL, "gateway", "g", DefaultLSPGatewayURL, "LSP Gateway URL")
		testCmd.Flags().IntVarP(&mcpPort, "port", "p", 3000, "MCP server port (for HTTP transport)")
		testCmd.Flags().StringVarP(&mcpTransport, "transport", "t", transport.TransportStdio, "Transport type (stdio, http)")
		testCmd.Flags().DurationVar(&mcpTimeout, "timeout", 30*time.Second, "Request timeout duration")
		testCmd.Flags().IntVar(&mcpMaxRetries, "max-retries", 3, "Maximum retries for failed requests")

		testCmd.SetArgs([]string{"--help"})
		err := testCmd.Execute()
		if err != nil {
			t.Errorf("Help command should not return error, got: %v", err)
		}
	})

	// Verify help output contains expected elements
	expectedElements := []string{
		"Model Context Protocol (MCP) server",
		"Model Context Protocol",
		CmdMCP,
		"Usage:",
		"Flags:",
		"--config",
		"--gateway",
		"--port",
		"--transport",
		"--timeout",
		"--max-retries",
		"-c",
		"-g",
		"-p",
		"-t",
		"Examples:",
		"stdio transport",
		"HTTP transport",
	}

	for _, element := range expectedElements {
		if !strings.Contains(output, element) {
			t.Errorf("Expected help output to contain '%s', got:\n%s", element, output)
		}
	}
}

func testMCPCommandIntegration(t *testing.T) {
	// Test execution through root command with basic config validation

	// Create test root command
	testRoot := &cobra.Command{
		Use:   rootCmd.Use,
		Short: rootCmd.Short,
	}

	// Create test MCP command that tests config creation but doesn't start server
	testMCP := &cobra.Command{
		Use:   mcpCmd.Use,
		Short: mcpCmd.Short,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create MCP server configuration
			cfg := &mcp.ServerConfig{
				Name:          "lsp-gateway-mcp",
				Description:   "MCP server providing LSP functionality through LSP Gateway",
				Version:       "0.1.0",
				LSPGatewayURL: mcpGatewayURL,
				Transport:     mcpTransport,
				Timeout:       mcpTimeout,
				MaxRetries:    mcpMaxRetries,
			}

			// Validate configuration
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("invalid MCP configuration: %w", err)
			}

			// Don't actually create or start MCP server for testing
			return nil
		},
	}

	// Add flags
	testMCP.Flags().StringVarP(&mcpConfigPath, "config", "c", "", "MCP configuration file path (optional)")
	testMCP.Flags().StringVarP(&mcpGatewayURL, "gateway", "g", DefaultLSPGatewayURL, "LSP Gateway URL")
	testMCP.Flags().IntVarP(&mcpPort, "port", "p", 3000, "MCP server port (for HTTP transport)")
	testMCP.Flags().StringVarP(&mcpTransport, "transport", "t", transport.TransportStdio, "Transport type (stdio, http)")
	testMCP.Flags().DurationVar(&mcpTimeout, "timeout", 30*time.Second, "Request timeout duration")
	testMCP.Flags().IntVar(&mcpMaxRetries, "max-retries", 3, "Maximum retries for failed requests")

	testRoot.AddCommand(testMCP)

	// Execute MCP command through root
	testRoot.SetArgs([]string{CmdMCP})
	err := testRoot.Execute()

	if err != nil {
		t.Errorf("Expected no error executing mcp through root, got: %v", err)
	}
}

// Test edge cases and special scenarios
func TestMCPCommandEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"WithExtraArgs", testMCPCommandWithExtraArgs},
		{"ContextCancellation", testMCPCommandContextCancellation},
		{"SignalHandlingSimulation", testMCPCommandSignalHandlingSimulation},
		{"ServerCreation", testMCPCommandServerCreation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global variables before each test
			mcpConfigPath = ""
			mcpGatewayURL = DefaultLSPGatewayURL
			mcpPort = 3000
			mcpTransport = transport.TransportStdio
			mcpTimeout = 30 * time.Second
			mcpMaxRetries = 3
			tt.testFunc(t)
		})
	}
}

func testMCPCommandWithExtraArgs(t *testing.T) {
	testCmd := &cobra.Command{
		Use: CmdMCP,
		RunE: func(cmd *cobra.Command, args []string) error {
			// MCP command should ignore extra arguments
			if len(args) > 0 {
				t.Logf("MCP command received extra args: %v", args)
			}

			// Minimal test execution - just config validation
			cfg := &mcp.ServerConfig{
				Name:          "lsp-gateway-mcp",
				Description:   "MCP server providing LSP functionality through LSP Gateway",
				Version:       "0.1.0",
				LSPGatewayURL: DefaultLSPGatewayURL,
				Transport:     transport.TransportStdio,
				Timeout:       30 * time.Second,
				MaxRetries:    3,
			}

			return cfg.Validate()
		},
	}

	testCmd.Flags().StringVarP(&mcpConfigPath, "config", "c", "", "MCP configuration file path (optional)")
	testCmd.Flags().StringVarP(&mcpGatewayURL, "gateway", "g", DefaultLSPGatewayURL, "LSP Gateway URL")
	testCmd.Flags().IntVarP(&mcpPort, "port", "p", 3000, "MCP server port (for HTTP transport)")
	testCmd.Flags().StringVarP(&mcpTransport, "transport", "t", transport.TransportStdio, "Transport type (stdio, http)")
	testCmd.Flags().DurationVar(&mcpTimeout, "timeout", 30*time.Second, "Request timeout duration")
	testCmd.Flags().IntVar(&mcpMaxRetries, "max-retries", 3, "Maximum retries for failed requests")

	// Test with extra arguments
	testCmd.SetArgs([]string{"extra", "args"})
	err := testCmd.Execute()

	if err != nil {
		t.Errorf("MCP command should handle extra args gracefully, got error: %v", err)
	}
}

func testMCPCommandContextCancellation(t *testing.T) {
	// Test context cancellation behavior
	cfg := &mcp.ServerConfig{
		Name:          "lsp-gateway-mcp",
		Description:   "MCP server providing LSP functionality through LSP Gateway",
		Version:       "0.1.0",
		LSPGatewayURL: DefaultLSPGatewayURL,
		Transport:     transport.TransportStdio,
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}

	if err := cfg.Validate(); err != nil {
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

func testMCPCommandSignalHandlingSimulation(t *testing.T) {
	// Test signal handling simulation (without actual signals)
	cfg := &mcp.ServerConfig{
		Name:          "lsp-gateway-mcp",
		Description:   "MCP server providing LSP functionality through LSP Gateway",
		Version:       "0.1.0",
		LSPGatewayURL: DefaultLSPGatewayURL,
		Transport:     transport.TransportStdio,
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}

	if err := cfg.Validate(); err != nil {
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

func testMCPCommandServerCreation(t *testing.T) {
	// Test MCP server creation
	cfg := &mcp.ServerConfig{
		Name:          "lsp-gateway-mcp",
		Description:   "MCP server providing LSP functionality through LSP Gateway",
		Version:       "0.1.0",
		LSPGatewayURL: DefaultLSPGatewayURL,
		Transport:     transport.TransportStdio,
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Test server creation
	server := mcp.NewServer(cfg)
	if server == nil {
		t.Error("Expected MCP server to be created")
	}

	// Test server state
	if !server.IsRunning() {
		// Server is not running by default, which is expected
		t.Log("MCP server is not running by default (expected)")
	}
}

// Test command completeness and structure
func TestMCPCommandCompleteness(t *testing.T) {
	// Verify command structure
	if mcpCmd.Name() != CmdMCP {
		t.Errorf("Expected command name 'mcp', got '%s'", mcpCmd.Name())
	}

	// Verify flags are properly defined
	expectedFlags := []string{"config", "gateway", "port", "transport", "timeout", "max-retries"}
	for _, flagName := range expectedFlags {
		flag := mcpCmd.Flag(flagName)
		if flag == nil {
			t.Errorf("Expected %s flag to be defined", flagName)
		}
	}

	// Verify no subcommands
	if mcpCmd.HasSubCommands() {
		t.Error("MCP command should not have subcommands")
	}

	// Verify command is added to root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == CmdMCP {
			found = true
			break
		}
	}

	if !found {
		t.Error("MCP command should be added to root command")
	}
}

// Helper function to capture stdout during function execution
func captureStdoutMCP(t *testing.T, fn func()) string {
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

// Benchmark MCP command execution
func BenchmarkMCPCommandFlagParsing(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Reset global variables
		mcpConfigPath = ""
		mcpGatewayURL = DefaultLSPGatewayURL
		mcpPort = 3000
		mcpTransport = transport.TransportStdio
		mcpTimeout = 30 * time.Second
		mcpMaxRetries = 3

		testCmd := &cobra.Command{
			Use: CmdMCP,
			RunE: func(cmd *cobra.Command, args []string) error {
				return nil
			},
		}

		testCmd.Flags().StringVarP(&mcpConfigPath, "config", "c", "", "MCP configuration file path (optional)")
		testCmd.Flags().StringVarP(&mcpGatewayURL, "gateway", "g", DefaultLSPGatewayURL, "LSP Gateway URL")
		testCmd.Flags().IntVarP(&mcpPort, "port", "p", 3000, "MCP server port (for HTTP transport)")
		testCmd.Flags().StringVarP(&mcpTransport, "transport", "t", transport.TransportStdio, "Transport type (stdio, http)")
		testCmd.Flags().DurationVar(&mcpTimeout, "timeout", 30*time.Second, "Request timeout duration")
		testCmd.Flags().IntVar(&mcpMaxRetries, "max-retries", 3, "Maximum retries for failed requests")

		testPort := allocateTestPortBench(b)
		testCmd.SetArgs([]string{"--gateway", fmt.Sprintf("http://localhost:%d", testPort), "--transport", "http", "--port", "4000"})
		if err := testCmd.Execute(); err != nil {
			b.Logf("command execution error: %v", err)
		}
	}
}

func BenchmarkMCPCommandConfigValidation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg := &mcp.ServerConfig{
			Name:          "lsp-gateway-mcp",
			Description:   "MCP server providing LSP functionality through LSP Gateway",
			Version:       "0.1.0",
			LSPGatewayURL: DefaultLSPGatewayURL,
			Transport:     transport.TransportStdio,
			Timeout:       30 * time.Second,
			MaxRetries:    3,
		}

		err := cfg.Validate()
		if err != nil {
			b.Fatalf("Config validation failed: %v", err)
		}
	}
}
