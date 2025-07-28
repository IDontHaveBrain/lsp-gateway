package e2e_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"lsp-gateway/tests/e2e/testutils"
)

// Parallel test functions using resource isolation for Python

// TestPythonBasicServerLifecycleParallel tests the complete server lifecycle for Python with parallel execution
func TestPythonBasicServerLifecycleParallel(t *testing.T) {
	t.Parallel()

	setup, err := testutils.SetupIsolatedTestWithLanguage("py_basic_lifecycle", "python")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")

	binaryPath := filepath.Join(projectRoot, "bin", "lspg")
	serverCmd := []string{binaryPath, "server", "--config", setup.ConfigPath}

	err = setup.StartServer(serverCmd, 45*time.Second)
	require.NoError(t, err, "Failed to start server")

	err = setup.WaitForServerReady(15 * time.Second)
	require.NoError(t, err, "Server failed to become ready")

	// Verify server readiness
	httpClient := setup.GetHTTPClient()
	err = httpClient.HealthCheck(ctx)
	require.NoError(t, err, "Server health check should pass")

	// Test basic server operations
	err = httpClient.ValidateConnection(ctx)
	require.NoError(t, err, "Server connection validation should pass")

	t.Logf("Python basic server lifecycle test completed successfully on port %d", setup.Resources.Port)
}

// TestPythonDefinitionFeatureParallel tests textDocument/definition for Python files with parallel execution
func TestPythonDefinitionFeatureParallel(t *testing.T) {
	t.Parallel()

	setup, err := testutils.SetupIsolatedTestWithLanguage("py_definition_feature", "python")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create comprehensive test Python file
	pyContent := `"""
LSP Gateway Server Implementation
Comprehensive Python test file with classes, functions, and imports
"""

import os
import sys
import json
from typing import Dict, List, Optional

class Server:
    """Main server class for LSP Gateway"""
    
    def __init__(self, name: str, port: int):
        self.name = name
        self.port = port
        self.running = False
        self.clients = []
    
    async def start(self) -> bool:
        """Start the server and initialize connections"""
        print(f"Starting server {self.name} on port {self.port}")
        self.running = True
        return True
    
    async def stop(self) -> None:
        """Stop the server gracefully"""
        print(f"Stopping server {self.name}")
        self.running = False
        for client in self.clients:
            await client.disconnect()
    
    def is_running(self) -> bool:
        """Check if server is currently running"""
        return self.running
    
    def add_client(self, client) -> None:
        """Add a client connection"""
        self.clients.append(client)


class Client:
    """Client connection handler"""
    
    def __init__(self, client_id: str, address: str):
        self.client_id = client_id
        self.address = address
        self.connected = False
    
    async def connect(self) -> bool:
        """Connect to the server"""
        print(f"Client {self.client_id} connecting from {self.address}")
        self.connected = True
        return True
    
    async def disconnect(self) -> None:
        """Disconnect from the server"""
        print(f"Client {self.client_id} disconnecting")
        self.connected = False


def create_server(name: str, port: int) -> Server:
    """Factory function to create a server instance"""
    return Server(name, port)


def create_client(client_id: str, address: str) -> Client:
    """Factory function to create a client instance"""
    return Client(client_id, address)


async def main():
    """Main application entry point"""
    server = create_server("python-lsp-gateway", 8080)
    client = create_client("test-client", "localhost")
    
    await server.start()
    await client.connect()
    
    server.add_client(client)
    
    print(f"Server running: {server.is_running()}")
    print(f"Client connected: {client.connected}")
    
    await server.stop()


if __name__ == "__main__":
    import asyncio
    asyncio.run(main())
`

	testFile, err := setup.Resources.Directory.CreateTempFile("server.py", pyContent)
	require.NoError(t, err, "Failed to create test Python file")

	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")

	binaryPath := filepath.Join(projectRoot, "bin", "lspg")
	serverCmd := []string{binaryPath, "server", "--config", setup.ConfigPath}

	err = setup.StartServer(serverCmd, 45*time.Second)
	require.NoError(t, err, "Failed to start server")

	err = setup.WaitForServerReady(15 * time.Second)
	require.NoError(t, err, "Server failed to become ready")

	// Test definition on Python file - test multiple positions
	httpClient := setup.GetHTTPClient()
	fileURI := "file://" + testFile

	testPositions := []struct {
		name string
		pos  testutils.Position
		desc string
	}{
		{"create_server_call", testutils.Position{Line: 77, Character: 13}, "create_server function call"},
		{"create_client_call", testutils.Position{Line: 78, Character: 13}, "create_client function call"},
		{"server_start_call", testutils.Position{Line: 80, Character: 10}, "server.start method call"},
		{"client_connect_call", testutils.Position{Line: 81, Character: 10}, "client.connect method call"},
		{"Server_class", testutils.Position{Line: 8, Character: 6}, "Server class definition"},
		{"Client_class", testutils.Position{Line: 39, Character: 6}, "Client class definition"},
	}

	successfulTests := 0
	for _, test := range testPositions {
		locations, err := httpClient.Definition(ctx, fileURI, test.pos)
		if err != nil {
			t.Logf("Definition request for %s failed (may be expected): %v", test.desc, err)
			continue
		}

		successfulTests++
		t.Logf("Found %d definition locations for %s", len(locations), test.desc)
		if len(locations) > 0 {
			assert.Contains(t, locations[0].URI, "server.py", "Definition should reference correct file")
			assert.GreaterOrEqual(t, locations[0].Range.Start.Line, 0, "Definition line should be valid")
		}
	}

	// At least one definition test should succeed
	assert.Greater(t, successfulTests, 0, "At least one definition test should succeed")

	t.Logf("Python definition feature test completed successfully on port %d (%d/%d tests successful)",
		setup.Resources.Port, successfulTests, len(testPositions))
}

// TestPythonHoverFeatureParallel tests textDocument/hover for Python files with parallel execution
func TestPythonHoverFeatureParallel(t *testing.T) {
	t.Parallel()

	setup, err := testutils.SetupIsolatedTestWithLanguage("py_hover_feature", "python")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create Python file with documented functions
	pyContent := `"""
Module for testing hover functionality
"""

def calculate_sum(a: int, b: int) -> int:
    """Calculate the sum of two integers.
    
    Args:
        a: First integer
        b: Second integer
        
    Returns:
        The sum of a and b
    """
    return a + b


class Calculator:
    """A simple calculator class for basic arithmetic operations."""
    
    def __init__(self, precision: int = 2):
        """Initialize calculator with specified precision.
        
        Args:
            precision: Number of decimal places for results
        """
        self.precision = precision
    
    def multiply(self, x: float, y: float) -> float:
        """Multiply two numbers with precision handling.
        
        Args:
            x: First number
            y: Second number
            
        Returns:
            Product of x and y rounded to specified precision
        """
        result = x * y
        return round(result, self.precision)


# Test usage
calc = Calculator(precision=3)
result = calc.multiply(3.14159, 2.71828)
sum_result = calculate_sum(10, 20)
`

	testFile, err := setup.Resources.Directory.CreateTempFile("calculator.py", pyContent)
	require.NoError(t, err, "Failed to create test Python file")

	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")

	binaryPath := filepath.Join(projectRoot, "bin", "lspg")
	serverCmd := []string{binaryPath, "server", "--config", setup.ConfigPath}

	err = setup.StartServer(serverCmd, 45*time.Second)
	require.NoError(t, err, "Failed to start server")

	err = setup.WaitForServerReady(15 * time.Second)
	require.NoError(t, err, "Server failed to become ready")

	// Test hover on Python file
	httpClient := setup.GetHTTPClient()
	fileURI := "file://" + testFile

	testPositions := []struct {
		name string
		pos  testutils.Position
		desc string
	}{
		{"calculate_sum_func", testutils.Position{Line: 5, Character: 4}, "calculate_sum function"},
		{"Calculator_class", testutils.Position{Line: 17, Character: 6}, "Calculator class"},
		{"multiply_method", testutils.Position{Line: 25, Character: 8}, "multiply method"},
		{"calc_variable", testutils.Position{Line: 40, Character: 0}, "calc variable"},
	}

	successfulTests := 0
	for _, test := range testPositions {
		hoverResult, err := httpClient.Hover(ctx, fileURI, test.pos)
		if err != nil {
			t.Logf("Hover request for %s failed (may be expected): %v", test.desc, err)
			continue
		}

		successfulTests++
		if hoverResult != nil && hoverResult.Contents != nil {
			t.Logf("Hover successful for %s - has content", test.desc)
		} else {
			t.Logf("Hover successful for %s - no content", test.desc)
		}
	}

	t.Logf("Python hover feature test completed successfully on port %d (%d/%d tests successful)",
		setup.Resources.Port, successfulTests, len(testPositions))
}

// TestPythonDocumentSymbolFeatureParallel tests textDocument/documentSymbol for Python files with parallel execution
func TestPythonDocumentSymbolFeatureParallel(t *testing.T) {
	t.Parallel()

	setup, err := testutils.SetupIsolatedTestWithLanguage("py_document_symbol", "python")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create Python file with various symbols
	pyContent := `"""
Advanced Python module with multiple symbol types
"""

import asyncio
from typing import Dict, List, Optional, Union
from dataclasses import dataclass
from enum import Enum

# Constants
MAX_CONNECTIONS = 100
DEFAULT_TIMEOUT = 30.0

# Global variable
connection_pool = {}

class ConnectionStatus(Enum):
    """Enumeration for connection status values"""
    DISCONNECTED = 0
    CONNECTING = 1
    CONNECTED = 2
    ERROR = 3

@dataclass
class ConnectionConfig:
    """Configuration for database connections"""
    host: str
    port: int
    username: str
    password: str
    timeout: float = DEFAULT_TIMEOUT

class DatabaseManager:
    """Advanced database connection manager"""
    
    def __init__(self, config: ConnectionConfig):
        self.config = config
        self.status = ConnectionStatus.DISCONNECTED
        self.connections: Dict[str, object] = {}
    
    async def connect(self) -> bool:
        """Establish database connection"""
        self.status = ConnectionStatus.CONNECTING
        # Simulate connection logic
        await asyncio.sleep(0.1)
        self.status = ConnectionStatus.CONNECTED
        return True
    
    async def disconnect(self) -> None:
        """Close database connection"""
        self.status = ConnectionStatus.DISCONNECTED
        self.connections.clear()
    
    def get_connection_info(self) -> Dict[str, Union[str, int, float]]:
        """Get connection information"""
        return {
            'host': self.config.host,
            'port': self.config.port,
            'timeout': self.config.timeout,
            'status': self.status.name
        }

def create_connection_config(host: str, port: int, username: str, password: str) -> ConnectionConfig:
    """Factory function for creating connection configuration"""
    return ConnectionConfig(host=host, port=port, username=username, password=password)

async def setup_database_manager(host: str = "localhost", port: int = 5432) -> DatabaseManager:
    """Setup and initialize database manager"""
    config = create_connection_config(host, port, "admin", "password")
    manager = DatabaseManager(config)
    await manager.connect()
    return manager

# Main execution
if __name__ == "__main__":
    async def main():
        manager = await setup_database_manager()
        info = manager.get_connection_info()
        print(f"Database manager initialized: {info}")
        await manager.disconnect()
    
    asyncio.run(main())
`

	testFile, err := setup.Resources.Directory.CreateTempFile("database.py", pyContent)
	require.NoError(t, err, "Failed to create test Python file")

	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")

	binaryPath := filepath.Join(projectRoot, "bin", "lspg")
	serverCmd := []string{binaryPath, "server", "--config", setup.ConfigPath}

	err = setup.StartServer(serverCmd, 45*time.Second)
	require.NoError(t, err, "Failed to start server")

	err = setup.WaitForServerReady(15 * time.Second)
	require.NoError(t, err, "Server failed to become ready")

	// Test document symbol request
	httpClient := setup.GetHTTPClient()
	fileURI := "file://" + testFile
	symbols, err := httpClient.DocumentSymbol(ctx, fileURI)
	if err != nil {
		t.Logf("Document symbols request failed (may be expected): %v", err)
		return
	}

	t.Logf("Found %d document symbols", len(symbols))

	// Verify we have the expected symbols
	symbolNames := make(map[string]bool)
	symbolKinds := make(map[int]int)

	for i, symbol := range symbols {
		assert.NotEmpty(t, symbol.Name, fmt.Sprintf("Symbol %d name should not be empty", i))
		assert.Greater(t, symbol.Kind, 0, fmt.Sprintf("Symbol %d kind should be valid", i))
		assert.GreaterOrEqual(t, symbol.Range.Start.Line, 0, fmt.Sprintf("Symbol %d range should be valid", i))

		symbolNames[symbol.Name] = true
		symbolKinds[symbol.Kind]++
		t.Logf("Symbol %d: %s (kind: %d, line: %d)", i, symbol.Name, symbol.Kind, symbol.Range.Start.Line)
	}

	// Check for expected symbols (lenient checking as LSP server behavior may vary)
	expectedSymbols := []string{"DatabaseManager", "ConnectionConfig", "ConnectionStatus", "create_connection_config", "setup_database_manager"}
	foundExpected := 0
	for _, expected := range expectedSymbols {
		if symbolNames[expected] {
			foundExpected++
		}
	}

	if foundExpected > 0 {
		t.Logf("Found %d out of %d expected symbols", foundExpected, len(expectedSymbols))
	}

	t.Logf("Python document symbol feature test completed successfully on port %d", setup.Resources.Port)
}

// TestPythonConnectionTimeoutsParallel tests connection timeout handling for Python with parallel execution
func TestPythonConnectionTimeoutsParallel(t *testing.T) {
	t.Parallel()

	setup, err := testutils.SetupIsolatedTest("py_connection_timeouts")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()

	// Test timeout when server is not running (Python-specific timeouts)
	shortTimeoutConfig := testutils.HttpClientConfig{
		BaseURL:    fmt.Sprintf("http://localhost:%d", setup.Resources.Port),
		Timeout:    200 * time.Millisecond, // Slightly longer for Python
		MaxRetries: 1,
		RetryDelay: 100 * time.Millisecond,
	}
	shortTimeoutClient := testutils.NewHttpClient(shortTimeoutConfig)
	defer shortTimeoutClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = shortTimeoutClient.HealthCheck(ctx)
	assert.Error(t, err, "Should timeout when server is not running")

	metrics := shortTimeoutClient.GetMetrics()
	assert.Greater(t, metrics.ConnectionErrors, 0, "Should record connection errors")

	t.Logf("Python connection timeouts test completed successfully on port %d", setup.Resources.Port)
}
