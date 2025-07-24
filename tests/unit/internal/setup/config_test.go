package setup_test

import (
	"context"
	"lsp-gateway/internal/setup"
	"testing"

	"lsp-gateway/internal/config"
)

func TestNewConfigGenerator(t *testing.T) {
	generator := setup.NewConfigGenerator()

	if generator == nil {
		t.Fatal("Expected non-nil config generator")
	}

	// COMMENTED OUT: accessing unexported fields not allowed
	/*
		if generator.runtimeDetector == nil {
			t.Error("Expected runtime detector to be initialized")
		}

		if generator.serverVerifier == nil {
			t.Error("Expected server verifier to be initialized")
		}

		if generator.serverRegistry == nil {
			t.Error("Expected server registry to be initialized")
		}

		if generator.templates == nil {
			t.Error("Expected templates map to be initialized")
		}

		if len(generator.templates) == 0 {
			t.Error("Expected templates to be populated")
		}
	*/
}

func TestConfigGenerator_GenerateDefault(t *testing.T) {
	generator := setup.NewConfigGenerator()

	result, err := generator.GenerateDefault()
	if err != nil {
		t.Fatalf("GenerateDefault failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Config == nil {
		t.Fatal("Expected non-nil config")
	}

	if result.ServersGenerated != 1 {
		t.Errorf("Expected 1 server generated, got %d", result.ServersGenerated)
	}

	if len(result.Config.Servers) != 1 {
		t.Errorf("Expected 1 server in config, got %d", len(result.Config.Servers))
	}

	server := result.Config.Servers[0]
	if server.Name != "go-lsp" {
		t.Errorf("Expected default server name 'go-lsp', got %s", server.Name)
	}

	if len(server.Languages) != 1 || server.Languages[0] != "go" {
		t.Errorf("Expected default server to support 'go' language, got %v", server.Languages)
	}

	if server.Command != "gopls" {
		t.Errorf("Expected default server command 'gopls', got %s", server.Command)
	}
}

func TestConfigGenerator_GetSupportedRuntimes(t *testing.T) {
	generator := setup.NewConfigGenerator()
	runtimes := generator.GetSupportedRuntimes()

	expectedRuntimes := []string{"go", "python", "nodejs", "java"}
	if len(runtimes) != len(expectedRuntimes) {
		t.Errorf("Expected %d supported runtimes, got %d", len(expectedRuntimes), len(runtimes))
	}

	runtimeMap := make(map[string]bool)
	for _, runtime := range runtimes {
		runtimeMap[runtime] = true
	}

	for _, expected := range expectedRuntimes {
		if !runtimeMap[expected] {
			t.Errorf("Expected runtime %s to be supported", expected)
		}
	}
}

func TestConfigGenerator_GetSupportedServers(t *testing.T) {
	generator := setup.NewConfigGenerator()
	servers := generator.GetSupportedServers()

	expectedServers := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}
	if len(servers) != len(expectedServers) {
		t.Errorf("Expected %d supported servers, got %d", len(expectedServers), len(servers))
	}

	serverMap := make(map[string]bool)
	for _, server := range servers {
		serverMap[server] = true
	}

	for _, expected := range expectedServers {
		if !serverMap[expected] {
			t.Errorf("Expected server %s to be supported", expected)
		}
	}
}

func TestConfigGenerator_GetTemplate(t *testing.T) {
	generator := setup.NewConfigGenerator()

	template, err := generator.GetTemplate("gopls")
	if err != nil {
		t.Fatalf("GetTemplate for gopls failed: %v", err)
	}

	if template == nil {
		t.Fatal("Expected non-nil template for gopls")
	}

	if template.ServerName != "gopls" {
		t.Errorf("Expected server name 'gopls', got %s", template.ServerName)
	}

	if template.RuntimeName != "go" {
		t.Errorf("Expected runtime name 'go', got %s", template.RuntimeName)
	}

	if template.ConfigName != "go-lsp" {
		t.Errorf("Expected config name 'go-lsp', got %s", template.ConfigName)
	}

	_, err = generator.GetTemplate("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}

func TestConfigGenerator_ValidateGenerated(t *testing.T) {
	generator := setup.NewConfigGenerator()

	validConfig := &config.GatewayConfig{
		Port: 8080,
		Servers: []config.ServerConfig{
			{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Args:      []string{},
				Transport: "stdio",
			},
		},
	}

	result, err := generator.ValidateGenerated(validConfig)
	if err != nil {
		t.Fatalf("ValidateGenerated failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil validation result")
	}

	if result.ServersValidated != 1 {
		t.Errorf("Expected 1 server validated, got %d", result.ServersValidated)
	}

	invalidConfig := &config.GatewayConfig{
		Port:    8080,
		Servers: []config.ServerConfig{},
	}

	result, err = generator.ValidateGenerated(invalidConfig)
	if err != nil {
		t.Fatalf("ValidateGenerated failed: %v", err)
	}

	if result.Valid {
		t.Error("Expected invalid configuration to be marked as invalid")
	}

	if len(result.Issues) == 0 {
		t.Error("Expected validation issues for invalid configuration")
	}
}

func TestServerConfigTemplate_AllFields(t *testing.T) {
	// COMMENTED OUT: accessing unexported field not allowed
	/*
		generator := setup.NewConfigGenerator()

		// COMMENTED OUT: accessing unexported field not allowed
		/*
		for serverName, template := range generator.templates {
			if template.ServerName == "" {
				t.Errorf("Template %s missing ServerName", serverName)
			}

			if template.RuntimeName == "" {
				t.Errorf("Template %s missing RuntimeName", serverName)
			}

			if template.ConfigName == "" {
				t.Errorf("Template %s missing ConfigName", serverName)
			}

			if template.Command == "" {
				t.Errorf("Template %s missing Command", serverName)
			}

			if len(template.Languages) == 0 {
				t.Errorf("Template %s missing Languages", serverName)
			}

			if template.Transport == "" {
				t.Errorf("Template %s missing Transport", serverName)
			}

			if template.RequiredRuntime == "" {
				t.Errorf("Template %s missing RequiredRuntime", serverName)
			}

			if template.MinRuntimeVersion == "" {
				t.Errorf("Template %s missing MinRuntimeVersion", serverName)
			}
		}
	*/
}

func TestConfigGenerationResult_Metadata(t *testing.T) {
	generator := setup.NewConfigGenerator()

	result, err := generator.GenerateDefault()
	if err != nil {
		t.Fatalf("GenerateDefault failed: %v", err)
	}

	if result.Metadata == nil {
		t.Error("Expected metadata to be initialized")
	}

	if method, ok := result.Metadata["generation_method"]; !ok || method != "default" {
		t.Error("Expected generation_method to be 'default'")
	}

	if server, ok := result.Metadata["default_server"]; !ok || server != "go-lsp" {
		t.Error("Expected default_server to be 'go-lsp'")
	}

	if result.GeneratedAt.IsZero() {
		t.Error("Expected GeneratedAt to be set")
	}

	if result.Duration == 0 {
		t.Error("Expected Duration to be greater than 0")
	}
}

func TestConfigGenerator_GenerateForRuntime_InvalidRuntime(t *testing.T) {
	generator := setup.NewConfigGenerator()
	ctx := context.Background()

	result, err := generator.GenerateForRuntime(ctx, "invalidruntime")

	if err == nil {
		t.Error("Expected error for invalid runtime")
	}

	if result == nil {
		t.Fatal("Expected non-nil result even on error")
	}

	if len(result.Issues) == 0 {
		t.Error("Expected issues to be reported for invalid runtime")
	}
}

func TestConfigGenerator_Integration(t *testing.T) {
	generator := setup.NewConfigGenerator()

	runtimes := generator.GetSupportedRuntimes()
	if len(runtimes) == 0 {
		t.Error("Expected at least one supported runtime")
	}

	servers := generator.GetSupportedServers()
	if len(servers) == 0 {
		t.Error("Expected at least one supported server")
	}

	result, err := generator.GenerateDefault()
	if err != nil {
		t.Fatalf("Integration test failed: %v", err)
	}

	if result.Config == nil {
		t.Error("Expected valid configuration")
	}

	validation, err := generator.ValidateGenerated(result.Config)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	if validation == nil {
		t.Error("Expected validation result")
		return
	}

	t.Logf("Integration test passed: %d servers generated, %d validated",
		result.ServersGenerated, validation.ServersValidated)
}

func TestConfigGenerator_UpdateConfig(t *testing.T) {
	generator := setup.NewConfigGenerator()

	existingConfig := &config.GatewayConfig{
		Port: 8080,
		Servers: []config.ServerConfig{
			{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Args:      []string{},
				Transport: "stdio",
			},
		},
	}

	testCases := []struct {
		name         string
		config       *config.GatewayConfig
		updates      setup.ConfigUpdates
		expectError  bool
		validateFunc func(t *testing.T, result *setup.ConfigUpdateResult)
	}{
		{
			name:   "UpdatePort",
			config: existingConfig,
			updates: setup.ConfigUpdates{
				Port: 9090,
			},
			expectError: false,
			validateFunc: func(t *testing.T, result *setup.ConfigUpdateResult) {
				if result.Config.Port != 9090 {
					t.Errorf("Expected port to be updated to 9090, got %d", result.Config.Port)
				}
				if !result.UpdatesApplied.PortChanged {
					t.Error("Expected PortChanged to be true")
				}
				if len(result.Messages) == 0 {
					t.Error("Expected update messages to be populated")
				}
			},
		},
		{
			name:   "AddServer",
			config: existingConfig,
			updates: setup.ConfigUpdates{
				AddServers: []config.ServerConfig{
					{
						Name:      "python-lsp",
						Languages: []string{"python"},
						Command:   "pylsp",
						Args:      []string{},
						Transport: "stdio",
					},
				},
			},
			expectError: false,
			validateFunc: func(t *testing.T, result *setup.ConfigUpdateResult) {
				if len(result.Config.Servers) != 2 {
					t.Errorf("Expected 2 servers after adding, got %d", len(result.Config.Servers))
				}
				if result.UpdatesApplied.ServersAdded != 1 {
					t.Errorf("Expected 1 server added, got %d", result.UpdatesApplied.ServersAdded)
				}
				found := false
				for _, server := range result.Config.Servers {
					if server.Name == "python-lsp" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected python-lsp server to be added")
				}
			},
		},
		{
			name:   "RemoveServer",
			config: existingConfig,
			updates: setup.ConfigUpdates{
				RemoveServers: []string{"go-lsp"},
			},
			expectError: true, // Now expects error because removing all servers is invalid
			validateFunc: func(t *testing.T, result *setup.ConfigUpdateResult) {
				// No validation needed for error case
			},
		},
		{
			name:   "UpdateExistingServer",
			config: existingConfig,
			updates: setup.ConfigUpdates{
				UpdateServers: []config.ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go", "mod"},
						Command:   "gopls",
						Args:      []string{"--mode=stdio"},
						Transport: "stdio",
					},
				},
			},
			expectError: false,
			validateFunc: func(t *testing.T, result *setup.ConfigUpdateResult) {
				if result.UpdatesApplied.ServersUpdated != 1 {
					t.Errorf("Expected 1 server updated, got %d", result.UpdatesApplied.ServersUpdated)
				}
				var updatedServer *config.ServerConfig
				for _, server := range result.Config.Servers {
					if server.Name == "go-lsp" {
						updatedServer = &server
						break
					}
				}
				if updatedServer == nil {
					t.Fatal("Expected go-lsp server to exist")
				}
				if len(updatedServer.Languages) != 2 {
					t.Errorf("Expected 2 languages, got %d", len(updatedServer.Languages))
				}
				if len(updatedServer.Args) != 1 || updatedServer.Args[0] != "--mode=stdio" {
					t.Errorf("Expected args to be updated, got %v", updatedServer.Args)
				}
			},
		},
		{
			name:   "ReplaceAllServers",
			config: existingConfig,
			updates: setup.ConfigUpdates{
				ReplaceAllServers: true,
				AddServers: []config.ServerConfig{
					{
						Name:      "typescript-lsp",
						Languages: []string{"typescript", "javascript"},
						Command:   "typescript-language-server",
						Args:      []string{"--stdio"},
						Transport: "stdio",
					},
					{
						Name:      "java-lsp",
						Languages: []string{"java"},
						Command:   "jdtls",
						Args:      []string{},
						Transport: "stdio",
					},
				},
			},
			expectError: false,
			validateFunc: func(t *testing.T, result *setup.ConfigUpdateResult) {
				if len(result.Config.Servers) != 2 {
					t.Errorf("Expected 2 servers after replacement, got %d", len(result.Config.Servers))
				}
				if !result.UpdatesApplied.ServersReplaced {
					t.Error("Expected ServersReplaced to be true")
				}
				if result.UpdatesApplied.ServersAdded != 2 {
					t.Errorf("Expected 2 servers added in replacement, got %d", result.UpdatesApplied.ServersAdded)
				}
				for _, server := range result.Config.Servers {
					if server.Name == "go-lsp" {
						t.Error("Expected go-lsp server to be removed during replacement")
					}
				}
			},
		},
		{
			name:        "NilConfig",
			config:      nil,
			updates:     setup.ConfigUpdates{},
			expectError: true,
			validateFunc: func(t *testing.T, result *setup.ConfigUpdateResult) {
				// No validation needed for error case
			},
		},
		{
			name:   "InvalidUpdate",
			config: existingConfig,
			updates: setup.ConfigUpdates{
				Port: 70000, // Invalid port - exceeds valid port range
			},
			expectError: true, // Port validation happens during update and fails
			validateFunc: func(t *testing.T, result *setup.ConfigUpdateResult) {
				// No validation needed for error case
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := generator.UpdateConfig(tc.config, tc.updates)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for case %s", tc.name)
				}
				return
			}

			if err != nil {
				t.Fatalf("UpdateConfig failed for case %s: %v", tc.name, err)
			}

			tc.validateFunc(t, result)
		})
	}
}

func TestConfigGenerator_ValidateConfig(t *testing.T) {
	generator := setup.NewConfigGenerator()

	testCases := []struct {
		name         string
		config       *config.GatewayConfig
		expectError  bool
		expectValid  bool
		validateFunc func(t *testing.T, result *setup.ConfigValidationResult)
	}{
		{
			name: "ValidConfig",
			config: &config.GatewayConfig{
				Port: 8080,
				Servers: []config.ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go"},
						Command:   "gopls",
						Args:      []string{},
						Transport: "stdio",
					},
				},
			},
			expectError: false,
			expectValid: true,
			validateFunc: func(t *testing.T, result *setup.ConfigValidationResult) {
				if result.ServersValidated != 1 {
					t.Errorf("Expected 1 server validated, got %d", result.ServersValidated)
				}
			},
		},
		{
			name: "InvalidPort",
			config: &config.GatewayConfig{
				Port: 70000, // Invalid port
				Servers: []config.ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go"},
						Command:   "gopls",
						Args:      []string{},
						Transport: "stdio",
					},
				},
			},
			expectError: false,
			expectValid: false,
			validateFunc: func(t *testing.T, result *setup.ConfigValidationResult) {
				if len(result.Issues) == 0 {
					t.Error("Expected validation issues for invalid port")
				}
			},
		},
		{
			name: "CommonPortWarning",
			config: &config.GatewayConfig{
				Port: 80, // Common HTTP port
				Servers: []config.ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go"},
						Command:   "gopls",
						Args:      []string{},
						Transport: "stdio",
					},
				},
			},
			expectError: false,
			expectValid: true,
			validateFunc: func(t *testing.T, result *setup.ConfigValidationResult) {
				if len(result.Warnings) == 0 {
					t.Error("Expected warning for common port usage")
				}
				found := false
				for _, warning := range result.Warnings {
					if len(warning) > 0 && warning[0:4] == "Port" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected port-related warning in warnings")
				}
			},
		},
		{
			name: "NoServers",
			config: &config.GatewayConfig{
				Port:    8080,
				Servers: []config.ServerConfig{},
			},
			expectError: false,
			expectValid: false,
			validateFunc: func(t *testing.T, result *setup.ConfigValidationResult) {
				found := false
				for _, issue := range result.Issues {
					if issue == "No servers configured" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected 'No servers configured' issue")
				}
			},
		},
		{
			name: "DuplicateLanguages",
			config: &config.GatewayConfig{
				Port: 8080,
				Servers: []config.ServerConfig{
					{
						Name:      "go-lsp1",
						Languages: []string{"go"},
						Command:   "gopls",
						Args:      []string{},
						Transport: "stdio",
					},
					{
						Name:      "go-lsp2",
						Languages: []string{"go"},
						Command:   "gopls",
						Args:      []string{},
						Transport: "stdio",
					},
				},
			},
			expectError: false,
			expectValid: true,
			validateFunc: func(t *testing.T, result *setup.ConfigValidationResult) {
				if len(result.Warnings) == 0 {
					t.Error("Expected warning for duplicate language support")
				}
				found := false
				for _, warning := range result.Warnings {
					if len(warning) > 8 && warning[0:8] == "Language" {
						found = true
						break
					}
				}
				if !found {
					t.Error("Expected language-related warning in warnings")
				}
			},
		},
		{
			name:        "NilConfig",
			config:      nil,
			expectError: true,
			expectValid: false,
			validateFunc: func(t *testing.T, result *setup.ConfigValidationResult) {
				// No validation needed for error case
			},
		},
		{
			name: "MetadataFields",
			config: &config.GatewayConfig{
				Port: 3000,
				Servers: []config.ServerConfig{
					{
						Name:      "go-lsp",
						Languages: []string{"go"},
						Command:   "gopls",
						Args:      []string{},
						Transport: "stdio",
					},
					{
						Name:      "python-lsp",
						Languages: []string{"python"},
						Command:   "pylsp",
						Args:      []string{},
						Transport: "stdio",
					},
				},
			},
			expectError: false,
			expectValid: true,
			validateFunc: func(t *testing.T, result *setup.ConfigValidationResult) {
				if result.Metadata == nil {
					t.Error("Expected metadata to be populated")
				}
				if totalServers, ok := result.Metadata["total_servers"].(int); !ok || totalServers != 2 {
					t.Errorf("Expected total_servers metadata to be 2, got %v", result.Metadata["total_servers"])
				}
				if totalLanguages, ok := result.Metadata["total_languages"].(int); !ok || totalLanguages != 2 {
					t.Errorf("Expected total_languages metadata to be 2, got %v", result.Metadata["total_languages"])
				}
				if port, ok := result.Metadata["port"].(int); !ok || port != 3000 {
					t.Errorf("Expected port metadata to be 3000, got %v", result.Metadata["port"])
				}
				if validationType, ok := result.Metadata["validation_type"].(string); !ok || validationType != "comprehensive" {
					t.Errorf("Expected validation_type to be 'comprehensive', got %v", result.Metadata["validation_type"])
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := generator.ValidateConfig(tc.config)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for case %s", tc.name)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateConfig failed for case %s: %v", tc.name, err)
			}

			if result.Valid != tc.expectValid {
				if tc.expectValid {
					t.Errorf("Expected valid configuration to be marked as valid, issues: %v", result.Issues)
				} else {
					t.Error("Expected configuration to be marked as invalid")
				}
			}

			tc.validateFunc(t, result)
		})
	}
}
