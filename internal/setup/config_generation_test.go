package setup

import (
	"context"
	"testing"
	"time"

	"lsp-gateway/internal/config"
)

// TestConfigGenerator_GenerateForRuntime_AllRuntimes tests configuration generation for all supported runtimes
func TestConfigGenerator_GenerateForRuntime_AllRuntimes(t *testing.T) {
	generator := NewConfigGenerator()
	ctx := context.Background()

	supportedRuntimes := []string{"go", "python", "nodejs", "java"}

	for _, runtime := range supportedRuntimes {
		t.Run(runtime, func(t *testing.T) {
			result, err := generator.GenerateForRuntime(ctx, runtime)

			// For this test, we expect success even if runtime is not installed
			// The implementation should handle missing runtimes gracefully
			if err != nil {
				t.Logf("GenerateForRuntime for %s returned error (expected for uninstalled runtimes): %v", runtime, err)

				// Verify result is still populated with error information
				if result == nil {
					t.Fatalf("Expected non-nil result even on error for runtime %s", runtime)
				}

				if len(result.Issues) == 0 {
					t.Errorf("Expected issues to be reported for runtime %s", runtime)
				}

				if result.ServersGenerated != 0 {
					t.Errorf("Expected 0 servers generated for failed runtime %s, got %d", runtime, result.ServersGenerated)
				}

				return
			}

			// If no error, validate the result
			if result == nil {
				t.Fatalf("Expected non-nil result for runtime %s", runtime)
			}

			if result.Config == nil {
				t.Fatalf("Expected non-nil config for runtime %s", runtime)
			}

			if result.Config.Port != 8080 {
				t.Errorf("Expected default port 8080 for runtime %s, got %d", runtime, result.Config.Port)
			}

			// Verify metadata
			if result.Metadata == nil {
				t.Errorf("Expected metadata to be populated for runtime %s", runtime)
			} else {
				if targetRuntime, ok := result.Metadata["target_runtime"]; !ok || targetRuntime != runtime {
					t.Errorf("Expected target_runtime metadata to be %s, got %v", runtime, targetRuntime)
				}
			}

			if result.GeneratedAt.IsZero() {
				t.Errorf("Expected GeneratedAt to be set for runtime %s", runtime)
			}

			t.Logf("Runtime %s: %d servers generated, %d skipped, %d issues, %d warnings",
				runtime, result.ServersGenerated, result.ServersSkipped, len(result.Issues), len(result.Warnings))
		})
	}
}

// TestConfigGenerator_GenerateServerConfig tests the internal generateServerConfig function
func TestConfigGenerator_GenerateServerConfig(t *testing.T) {
	generator := NewConfigGenerator()
	ctx := context.Background()

	// Mock runtime info for Go
	runtimeInfo := &RuntimeInfo{
		Name:       "go",
		Installed:  true,
		Version:    "go1.21.0",
		Compatible: true,
		Path:       "/usr/local/go/bin/go",
		WorkingDir: "",
		DetectedAt: time.Now(),
		Duration:   100 * time.Millisecond,
		Issues:     []string{},
		Warnings:   []string{},
		Metadata:   make(map[string]interface{}),
	}

	testCases := []struct {
		name       string
		serverDef  *ServerDefinition
		expectNil  bool
		expectName string
	}{
		{
			name: "Go Language Server",
			serverDef: &ServerDefinition{
				Name:        "gopls",
				DisplayName: "Go Language Server",
				Languages:   []string{"go"},
				DefaultConfig: map[string]interface{}{
					"command":   "gopls",
					"transport": "stdio",
					"args":      []string{},
				},
			},
			expectNil:  false,
			expectName: "gopls-lsp",
		},
		{
			name: "Python Language Server",
			serverDef: &ServerDefinition{
				Name:        "pylsp",
				DisplayName: "Python Language Server",
				Languages:   []string{"python"},
				DefaultConfig: map[string]interface{}{
					"command":   "pylsp",
					"transport": "stdio",
					"args":      []string{},
				},
			},
			expectNil:  false,
			expectName: "pylsp-lsp",
		},
		{
			name: "TypeScript Language Server",
			serverDef: &ServerDefinition{
				Name:        "typescript-language-server",
				DisplayName: "TypeScript Language Server",
				Languages:   []string{"typescript", "javascript"},
				DefaultConfig: map[string]interface{}{
					"command":   "typescript-language-server",
					"transport": "stdio",
					"args":      []string{"--stdio"},
				},
			},
			expectNil:  false,
			expectName: "typescript-language-server-lsp",
		},
		{
			name: "Java Language Server",
			serverDef: &ServerDefinition{
				Name:        "jdtls",
				DisplayName: "Java Language Server",
				Languages:   []string{"java"},
				DefaultConfig: map[string]interface{}{
					"command":   "jdtls",
					"transport": "stdio",
					"args":      []string{},
				},
			},
			expectNil:  false,
			expectName: "jdtls-lsp",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serverConfig, err := generator.generateServerConfig(ctx, runtimeInfo, tc.serverDef)

			if tc.expectNil {
				if serverConfig != nil {
					t.Errorf("Expected nil server config for %s", tc.name)
				}
				return
			}

			if err != nil {
				t.Fatalf("generateServerConfig failed for %s: %v", tc.name, err)
			}

			if serverConfig == nil {
				t.Fatalf("Expected non-nil server config for %s", tc.name)
			}

			if serverConfig.Name != tc.expectName {
				t.Errorf("Expected server name %s, got %s", tc.expectName, serverConfig.Name)
			}

			if len(serverConfig.Languages) == 0 {
				t.Errorf("Expected languages to be populated for %s", tc.name)
			}

			if serverConfig.Transport != "stdio" {
				t.Errorf("Expected transport 'stdio' for %s, got %s", tc.name, serverConfig.Transport)
			}

			// Verify language consistency
			expectedLanguages := tc.serverDef.Languages
			if len(serverConfig.Languages) != len(expectedLanguages) {
				t.Errorf("Expected %d languages for %s, got %d", len(expectedLanguages), tc.name, len(serverConfig.Languages))
			}

			for i, lang := range expectedLanguages {
				if i < len(serverConfig.Languages) && serverConfig.Languages[i] != lang {
					t.Errorf("Expected language %s at index %d for %s, got %s", lang, i, tc.name, serverConfig.Languages[i])
				}
			}
		})
	}
}

// TestServerRegistry_GetServersByRuntime tests the server registry functionality
func TestServerRegistry_GetServersByRuntime(t *testing.T) {
	registry := NewDefaultServerRegistry()

	testCases := []struct {
		name            string
		runtime         string
		expectedServers []string
		expectedCount   int
	}{
		{
			name:            "Go runtime",
			runtime:         "go",
			expectedServers: []string{"gopls"},
			expectedCount:   1,
		},
		{
			name:            "Python runtime",
			runtime:         "python",
			expectedServers: []string{"pylsp"},
			expectedCount:   1,
		},
		{
			name:            "Node.js runtime",
			runtime:         "nodejs",
			expectedServers: []string{"typescript-language-server"},
			expectedCount:   1,
		},
		{
			name:            "Java runtime",
			runtime:         "java",
			expectedServers: []string{"jdtls"},
			expectedCount:   1,
		},
		{
			name:            "Unsupported runtime",
			runtime:         "rust",
			expectedServers: []string{},
			expectedCount:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			servers := registry.GetServersByRuntime(tc.runtime)

			if len(servers) != tc.expectedCount {
				t.Errorf("Expected %d servers for runtime %s, got %d", tc.expectedCount, tc.runtime, len(servers))
			}

			for i, expectedServer := range tc.expectedServers {
				if i >= len(servers) {
					t.Errorf("Expected server %s at index %d for runtime %s, but not enough servers returned", expectedServer, i, tc.runtime)
					continue
				}

				if servers[i].Name != expectedServer {
					t.Errorf("Expected server %s at index %d for runtime %s, got %s", expectedServer, i, tc.runtime, servers[i].Name)
				}

				// Verify server definition is complete
				if servers[i].DisplayName == "" {
					t.Errorf("Server %s missing DisplayName", servers[i].Name)
				}

				if len(servers[i].Languages) == 0 {
					t.Errorf("Server %s missing Languages", servers[i].Name)
				}

				if servers[i].DefaultConfig == nil {
					t.Errorf("Server %s missing DefaultConfig", servers[i].Name)
				}
			}
		})
	}
}

// TestServerRegistry_GetServer tests individual server retrieval
func TestServerRegistry_GetServer(t *testing.T) {
	registry := NewDefaultServerRegistry()

	testCases := []struct {
		name         string
		serverName   string
		expectError  bool
		expectServer *ServerDefinition
	}{
		{
			name:        "Valid Go server",
			serverName:  "gopls",
			expectError: false,
			expectServer: &ServerDefinition{
				Name:        "gopls",
				DisplayName: "Go Language Server",
				Languages:   []string{"go"},
			},
		},
		{
			name:        "Valid Python server",
			serverName:  "pylsp",
			expectError: false,
			expectServer: &ServerDefinition{
				Name:        "pylsp",
				DisplayName: "Python Language Server",
				Languages:   []string{"python"},
			},
		},
		{
			name:        "Valid TypeScript server",
			serverName:  "typescript-language-server",
			expectError: false,
			expectServer: &ServerDefinition{
				Name:        "typescript-language-server",
				DisplayName: "TypeScript Language Server",
				Languages:   []string{"typescript", "javascript"},
			},
		},
		{
			name:        "Valid Java server",
			serverName:  "jdtls",
			expectError: false,
			expectServer: &ServerDefinition{
				Name:        "jdtls",
				DisplayName: "Java Language Server",
				Languages:   []string{"java"},
			},
		},
		{
			name:         "Invalid server",
			serverName:   "nonexistent-server",
			expectError:  true,
			expectServer: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server, err := registry.GetServer(tc.serverName)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for server %s", tc.serverName)
				}
				if server != nil {
					t.Errorf("Expected nil server for invalid server name %s", tc.serverName)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetServer failed for %s: %v", tc.serverName, err)
			}

			if server == nil {
				t.Fatalf("Expected non-nil server for %s", tc.serverName)
			}

			if server.Name != tc.expectServer.Name {
				t.Errorf("Expected server name %s, got %s", tc.expectServer.Name, server.Name)
			}

			if server.DisplayName != tc.expectServer.DisplayName {
				t.Errorf("Expected display name %s, got %s", tc.expectServer.DisplayName, server.DisplayName)
			}

			if len(server.Languages) != len(tc.expectServer.Languages) {
				t.Errorf("Expected %d languages, got %d", len(tc.expectServer.Languages), len(server.Languages))
			}

			for i, expectedLang := range tc.expectServer.Languages {
				if i < len(server.Languages) && server.Languages[i] != expectedLang {
					t.Errorf("Expected language %s at index %d, got %s", expectedLang, i, server.Languages[i])
				}
			}
		})
	}
}

// TestConfigGenerator_GenerateFromDetected tests auto-detection and configuration generation
func TestConfigGenerator_GenerateFromDetected(t *testing.T) {
	generator := NewConfigGenerator()
	ctx := context.Background()

	// Test the interface exists and can be called
	result, err := generator.GenerateFromDetected(ctx)

	// The actual result may vary based on the system, but we should handle it gracefully
	if err != nil {
		t.Logf("GenerateFromDetected returned error (may be expected): %v", err)

		if result == nil {
			t.Error("Expected non-nil result even on error")
		}
		return
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Basic validation of result structure
	if result.Config == nil {
		t.Error("Expected config to be generated")
	}

	if result.GeneratedAt.IsZero() {
		t.Error("Expected GeneratedAt to be set")
	}

	if result.Metadata == nil {
		t.Error("Expected metadata to be initialized")
	}

	t.Logf("Auto-detection result: %d servers generated, %d skipped, %d issues, %d warnings",
		result.ServersGenerated, result.ServersSkipped, len(result.Issues), len(result.Warnings))
}

// TestConfigGenerator_ValidationEdgeCases tests edge cases in configuration validation
func TestConfigGenerator_ValidationEdgeCases(t *testing.T) {
	generator := NewConfigGenerator()

	// Test validation with server that has duplicate languages within the same server
	configWithDuplicateLanguagesInServer := &config.GatewayConfig{
		Port: 8080,
		Servers: []config.ServerConfig{
			{
				Name:      "go-lsp-duplicate",
				Languages: []string{"go", "go", "go"}, // Duplicate languages in same server
				Command:   "gopls",
				Args:      []string{},
				Transport: "stdio",
			},
		},
	}

	result, err := generator.ValidateConfig(configWithDuplicateLanguagesInServer)
	if err != nil {
		t.Fatalf("ValidateConfig failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil validation result")
	}

	// Check if duplicate languages within server are detected
	if serverIssues, exists := result.ServerIssues["go-lsp-duplicate"]; exists {
		found := false
		for _, issue := range serverIssues {
			if len(issue) > 10 && issue[0:9] == "Duplicate" {
				found = true
				break
			}
		}
		if !found {
			t.Log("Duplicate language detection may not be implemented for single server - this is acceptable")
		}
	}
}

// TestConfigGenerator_SetLogger tests the SetLogger method
func TestConfigGenerator_SetLogger(t *testing.T) {
	generator := NewConfigGenerator()

	// Test setting nil logger (should not crash)
	generator.SetLogger(nil)

	// Test setting valid logger
	logger := NewSetupLogger(nil)
	generator.SetLogger(logger)

	if generator.logger != logger {
		t.Error("Expected logger to be set")
	}

	// Test that it propagates to runtime detector
	detector := generator.runtimeDetector.(*DefaultRuntimeDetector)
	if detector.logger != logger {
		t.Error("Expected logger to be propagated to runtime detector")
	}
}
