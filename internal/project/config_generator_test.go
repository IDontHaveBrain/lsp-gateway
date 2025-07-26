package project

import (
	"context"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
)

// Test fixtures and mock implementations

// Helper function to create test logger
func createTestLogger() *setup.SetupLogger {
	return setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Level:     setup.LogLevelError, // Minimize noise in tests
		Component: "test",
	})
}

// MockServerRegistry is a mock implementation of setup.ServerRegistry
type MockServerRegistry struct {
	servers map[string]*setup.ServerDefinition
}

func NewMockServerRegistry() *MockServerRegistry {
	return &MockServerRegistry{
		servers: map[string]*setup.ServerDefinition{
			setup.SERVER_GOPLS: {
				Name:        setup.SERVER_GOPLS,
				DisplayName: "Go Language Server (gopls)",
				Languages:   []string{"go"},
				DefaultConfig: map[string]interface{}{
					"command":   "gopls",
					"transport": "stdio",
					"args":      []string{},
				},
			},
			setup.SERVER_PYLSP: {
				Name:        setup.SERVER_PYLSP,
				DisplayName: "Python LSP Server",
				Languages:   []string{"python"},
				DefaultConfig: map[string]interface{}{
					"command":   "pylsp",
					"transport": "stdio",
					"args":      []string{},
				},
			},
			setup.SERVER_JDTLS: {
				Name:        setup.SERVER_JDTLS,
				DisplayName: "Eclipse JDT Language Server",
				Languages:   []string{"java"},
				DefaultConfig: map[string]interface{}{
					"command":   "jdtls",
					"transport": "stdio",
					"args":      []string{},
				},
			},
			"typescript-language-server": {
				Name:        "typescript-language-server",
				DisplayName: "TypeScript Language Server",
				Languages:   []string{"typescript", "javascript"},
				DefaultConfig: map[string]interface{}{
					"command":   "typescript-language-server",
					"transport": "stdio",
					"args":      []string{"--stdio"},
				},
			},
		},
	}
}

func (m *MockServerRegistry) GetServersByRuntime(runtime string) []*setup.ServerDefinition {
	var result []*setup.ServerDefinition
	for _, server := range m.servers {
		for _, lang := range server.Languages {
			if lang == runtime {
				result = append(result, server)
				break
			}
		}
	}
	return result
}

func (m *MockServerRegistry) GetServer(serverName string) (*setup.ServerDefinition, error) {
	if server, exists := m.servers[serverName]; exists {
		return server, nil
	}
	return nil, setup.WrapWithContext(nil, "get-server", "mock-registry", map[string]interface{}{
		"server_name": serverName,
	})
}

// MockServerVerifier is a mock implementation of setup.ServerVerifier
type MockServerVerifier struct {
	verificationResults map[string]*setup.ServerVerificationResult
}

func NewMockServerVerifier() *MockServerVerifier {
	return &MockServerVerifier{
		verificationResults: map[string]*setup.ServerVerificationResult{
			setup.SERVER_GOPLS: {
				Installed:  true,
				Compatible: true,
				Functional: true,
				Path:       "/usr/local/bin/gopls",
			},
			setup.SERVER_PYLSP: {
				Installed:  true,
				Compatible: true,
				Functional: true,
				Path:       "/usr/local/bin/pylsp",
			},
			setup.SERVER_JDTLS: {
				Installed:  true,
				Compatible: true,
				Functional: true,
				Path:       "/opt/jdtls/bin/jdtls",
			},
			"typescript-language-server": {
				Installed:  true,
				Compatible: true,
				Functional: true,
				Path:       "/usr/local/bin/typescript-language-server",
			},
		},
	}
}

func (m *MockServerVerifier) VerifyServer(serverName string) (*setup.ServerVerificationResult, error) {
	if result, exists := m.verificationResults[serverName]; exists {
		return result, nil
	}
	return &setup.ServerVerificationResult{
		Installed:  false,
		Compatible: false,
		Functional: false,
		Path:       "",
	}, nil
}

func (m *MockServerVerifier) SetServerInstalled(serverName string, installed bool) {
	if result, exists := m.verificationResults[serverName]; exists {
		result.Installed = installed
	} else {
		m.verificationResults[serverName] = &setup.ServerVerificationResult{
			Installed:  installed,
			Compatible: installed,
			Functional: installed,
		}
	}
}

// Test data fixtures

var (
	validGoProject = &ProjectContext{
		ProjectType:      "go",
		RootPath:         "/test/go-project",
		WorkspaceRoot:    "/test/go-project",
		Languages:        []string{"go"},
		PrimaryLanguage:  "go",
		RequiredServers:  []string{setup.SERVER_GOPLS},
		ModuleName:       "github.com/example/go-project",
		DisplayName:      "Go Test Project",
		MarkerFiles:      []string{"go.mod"},
		DetectedAt:       time.Now(),
		Confidence:       0.95,
		IsValid:          true,
		Platform:         "linux",
		Architecture:     "x64",
		ProjectSize: types.ProjectSize{
			TotalFiles:      100,
			SourceFiles:     80,
			TestFiles:       15,
			ConfigFiles:     5,
			TotalSizeBytes:  1024000,
		},
		Metadata: map[string]interface{}{
			"go_context": &GoProjectContext{
				GoVersion: "1.21.0",
				GoMod: GoModInfo{
					ModuleName: "github.com/example/go-project",
					GoVersion:  "1.21",
				},
			},
		},
	}

	validPythonProject = &ProjectContext{
		ProjectType:     "python",
		RootPath:        "/test/python-project",
		WorkspaceRoot:   "/test/python-project",
		Languages:       []string{"python"},
		PrimaryLanguage: "python",
		RequiredServers: []string{setup.SERVER_PYLSP},
		DisplayName:     "Python Test Project",
		MarkerFiles:     []string{"requirements.txt"},
		DetectedAt:      time.Now(),
		Confidence:      0.90,
		IsValid:         true,
		Platform:        "linux",
		Architecture:    "x64",
		ProjectSize: types.ProjectSize{
			TotalFiles:      50,
			SourceFiles:     40,
			TestFiles:       8,
			ConfigFiles:     2,
			TotalSizeBytes:  512000,
		},
		Metadata: map[string]interface{}{
			"python_context": &PythonProjectContext{
				PythonVersion: "3.11.0",
				VirtualEnv:    "/test/python-project/.venv",
			},
		},
	}

	validJavaProject = &ProjectContext{
		ProjectType:     "java",
		RootPath:        "/test/java-project",
		WorkspaceRoot:   "/test/java-project",
		Languages:       []string{"java"},
		PrimaryLanguage: "java",
		RequiredServers: []string{setup.SERVER_JDTLS},
		DisplayName:     "Java Test Project",
		MarkerFiles:     []string{"pom.xml"},
		BuildSystem:     "maven",
		DetectedAt:      time.Now(),
		Confidence:      0.85,
		IsValid:         true,
		Platform:        "linux",
		Architecture:    "x64",
		ProjectSize: types.ProjectSize{
			TotalFiles:      200,
			SourceFiles:     150,
			TestFiles:       40,
			ConfigFiles:     10,
			TotalSizeBytes:  2048000,
		},
		Metadata: map[string]interface{}{
			"java_context": &JavaProjectContext{
				JavaVersion: "17.0.0",
				BuildSystem: "maven",
			},
		},
	}

	validNodeJSProject = &ProjectContext{
		ProjectType:     "nodejs",
		RootPath:        "/test/nodejs-project",
		WorkspaceRoot:   "/test/nodejs-project",
		Languages:       []string{"typescript", "javascript"},
		PrimaryLanguage: "typescript",
		RequiredServers: []string{"typescript-language-server"},
		DisplayName:     "Node.js Test Project",
		MarkerFiles:     []string{"package.json", "tsconfig.json"},
		PackageManager:  "npm",
		DetectedAt:      time.Now(),
		Confidence:      0.88,
		IsValid:         true,
		Platform:        "linux",
		Architecture:    "x64",
		ProjectSize: types.ProjectSize{
			TotalFiles:      300,
			SourceFiles:     250,
			TestFiles:       40,
			ConfigFiles:     10,
			TotalSizeBytes:  3072000,
		},
		Metadata: map[string]interface{}{
			"nodejs_context": &NodeJSProjectContext{
				NodeVersion:    "20.0.0",
				PackageManager: "npm",
			},
		},
	}

	mixedLanguageProject = &ProjectContext{
		ProjectType:     "mixed",
		RootPath:        "/test/mixed-project",
		WorkspaceRoot:   "/test/mixed-project",
		Languages:       []string{"go", "python", "typescript"},
		PrimaryLanguage: "go",
		RequiredServers: []string{setup.SERVER_GOPLS, setup.SERVER_PYLSP, "typescript-language-server"},
		DisplayName:     "Mixed Language Test Project",
		MarkerFiles:     []string{"go.mod", "requirements.txt", "package.json"},
		DetectedAt:      time.Now(),
		Confidence:      0.80,
		IsValid:         true,
		Platform:        "linux",
		Architecture:    "x64",
		ProjectSize: types.ProjectSize{
			TotalFiles:      500,
			SourceFiles:     400,
			TestFiles:       80,
			ConfigFiles:     20,
			TotalSizeBytes:  5120000,
		},
		IsMonorepo: false,
	}

	largeGoProject = &ProjectContext{
		ProjectType:     "go",
		RootPath:        "/test/large-go-project",
		WorkspaceRoot:   "/test/large-go-project",
		Languages:       []string{"go"},
		PrimaryLanguage: "go",
		RequiredServers: []string{setup.SERVER_GOPLS},
		DisplayName:     "Large Go Test Project",
		MarkerFiles:     []string{"go.mod"},
		DetectedAt:      time.Now(),
		Confidence:      0.95,
		IsValid:         true,
		Platform:        "linux",
		Architecture:    "x64",
		ProjectSize: types.ProjectSize{
			TotalFiles:      1500, // Large project
			SourceFiles:     1200,
			TestFiles:       250,
			ConfigFiles:     50,
			TotalSizeBytes:  15360000,
		},
	}

	monorepoProject = &ProjectContext{
		ProjectType:     "monorepo",
		RootPath:        "/test/monorepo",
		WorkspaceRoot:   "/test/monorepo",
		Languages:       []string{"go", "python", "typescript", "java"},
		PrimaryLanguage: "go",
		RequiredServers: []string{setup.SERVER_GOPLS, setup.SERVER_PYLSP, "typescript-language-server", setup.SERVER_JDTLS},
		DisplayName:     "Monorepo Test Project",
		MarkerFiles:     []string{"go.mod", "requirements.txt", "package.json", "pom.xml"},
		DetectedAt:      time.Now(),
		Confidence:      0.75,
		IsValid:         true,
		Platform:        "linux",
		Architecture:    "x64",
		ProjectSize: types.ProjectSize{
			TotalFiles:      2000,
			SourceFiles:     1600,
			TestFiles:       300,
			ConfigFiles:     100,
			TotalSizeBytes:  20480000,
		},
		IsMonorepo:  true,
		SubProjects: []string{"/test/monorepo/go-service", "/test/monorepo/python-api", "/test/monorepo/web-ui"},
	}
)

// Test constructor

func TestNewProjectConfigGenerator(t *testing.T) {
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	if generator == nil {
		t.Fatal("NewProjectConfigGenerator should not return nil")
	}

	if generator.logger != logger {
		t.Error("Logger not set correctly")
	}

	if generator.serverRegistry != registry {
		t.Error("Server registry not set correctly")
	}

	if generator.serverVerifier != verifier {
		t.Error("Server verifier not set correctly")
	}

	if generator.templates == nil {
		t.Error("Templates map should be initialized")
	}

	if generator.optimizers == nil {
		t.Error("Optimizers map should be initialized")
	}

	// Verify templates are initialized for supported languages
	expectedLanguages := []string{"go", "python", "typescript", "java"}
	for _, lang := range expectedLanguages {
		if _, exists := generator.templates[lang]; !exists {
			t.Errorf("Template for %s language not initialized", lang)
		}
	}

	// Verify optimizers are initialized for supported languages
	expectedOptimizers := []string{"go", "python", "typescript", "javascript", "java"}
	for _, lang := range expectedOptimizers {
		if _, exists := generator.optimizers[lang]; !exists {
			t.Errorf("Optimizer for %s language not initialized", lang)
		}
	}
}

// Test GenerateFromProject method

func TestGenerateFromProject_GoProject(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	result, err := generator.GenerateFromProject(ctx, validGoProject)

	if err != nil {
		t.Fatalf("GenerateFromProject failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// Verify result structure
	if result.GatewayConfig == nil {
		t.Error("GatewayConfig should not be nil")
	}

	if result.ProjectConfig == nil {
		t.Error("ProjectConfig should not be nil")
	}

	if result.ServersGenerated != 1 {
		t.Errorf("Expected 1 server generated, got %d", result.ServersGenerated)
	}

	if result.ServersSkipped != 0 {
		t.Errorf("Expected 0 servers skipped, got %d", result.ServersSkipped)
	}

	// Verify gateway config
	gatewayConfig := result.GatewayConfig
	if gatewayConfig.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", gatewayConfig.Port)
	}

	if !gatewayConfig.ProjectAware {
		t.Error("ProjectAware should be true")
	}

	if len(gatewayConfig.Servers) != 1 {
		t.Errorf("Expected 1 server in gateway config, got %d", len(gatewayConfig.Servers))
	}

	// Verify server configuration
	server := gatewayConfig.Servers[0]
	if server.Name != "gopls-project" {
		t.Errorf("Expected server name 'gopls-project', got %s", server.Name)
	}

	if len(server.Languages) != 1 || server.Languages[0] != "go" {
		t.Errorf("Expected server to support 'go' language, got %v", server.Languages)
	}

	// Verify Go-specific optimizations were applied
	if server.Settings == nil {
		t.Error("Server settings should not be nil")
	}

	if goplsSettings, exists := server.Settings["gopls"]; exists {
		goplsMap, ok := goplsSettings.(map[string]interface{})
		if !ok {
			t.Error("gopls settings should be a map")
		} else {
			if gofumpt, exists := goplsMap["gofumpt"]; !exists || gofumpt != true {
				t.Error("gofumpt should be enabled in gopls settings")
			}
		}
	} else {
		t.Error("gopls settings should be present")
	}

	// Verify project config
	projectConfig := result.ProjectConfig
	if projectConfig.ProjectID == "" {
		t.Error("ProjectID should not be empty")
	}

	if projectConfig.Name != validGoProject.DisplayName {
		t.Errorf("Expected project name %s, got %s", validGoProject.DisplayName, projectConfig.Name)
	}

	if len(projectConfig.EnabledServers) != 1 || projectConfig.EnabledServers[0] != setup.SERVER_GOPLS {
		t.Errorf("Expected enabled server 'gopls', got %v", projectConfig.EnabledServers)
	}

	// Verify metadata
	if result.Metadata["generation_method"] != "project_aware" {
		t.Error("generation_method metadata should be 'project_aware'")
	}

	if result.Metadata["project_type"] != "go" {
		t.Error("project_type metadata should be 'go'")
	}
}

func TestGenerateFromProject_PythonProject(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	result, err := generator.GenerateFromProject(ctx, validPythonProject)

	if err != nil {
		t.Fatalf("GenerateFromProject failed: %v", err)
	}

	// Verify Python-specific server configuration
	if len(result.GatewayConfig.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(result.GatewayConfig.Servers))
	}

	server := result.GatewayConfig.Servers[0]
	if server.Name != "pylsp-project" {
		t.Errorf("Expected server name 'pylsp-project', got %s", server.Name)
	}

	// Verify Python-specific optimizations
	if pylspSettings, exists := server.Settings["pylsp"]; exists {
		pylspMap, ok := pylspSettings.(map[string]interface{})
		if !ok {
			t.Error("pylsp settings should be a map")
		} else {
			if plugins, exists := pylspMap["plugins"]; exists {
				pluginsMap, ok := plugins.(map[string]interface{})
				if !ok {
					t.Error("plugins should be a map")
				} else {
					if pycodestyle, exists := pluginsMap["pycodestyle"]; exists {
						pycodestyleMap, ok := pycodestyle.(map[string]interface{})
						if !ok {
							t.Error("pycodestyle should be a map")
						} else {
							if enabled, exists := pycodestyleMap["enabled"]; !exists || enabled != true {
								t.Error("pycodestyle should be enabled")
							}
						}
					}
				}
			}
		}
	} else {
		t.Error("pylsp settings should be present")
	}

	// Verify virtual environment detection was applied
	if pythonCtx, exists := validPythonProject.Metadata["python_context"]; exists {
		if pythonProjectCtx, ok := pythonCtx.(*PythonProjectContext); ok && pythonProjectCtx.VirtualEnv != "" {
			// Check if jedi_completion was configured with virtual environment
			if pylspSettings, exists := server.Settings["pylsp"]; exists {
				pylspMap := pylspSettings.(map[string]interface{})
				if plugins, exists := pylspMap["plugins"]; exists {
					pluginsMap := plugins.(map[string]interface{})
					if jediCompletion, exists := pluginsMap["jedi_completion"]; exists {
						jediMap, ok := jediCompletion.(map[string]interface{})
						if ok {
							if env, exists := jediMap["environment"]; !exists || env != pythonProjectCtx.VirtualEnv {
								t.Error("jedi_completion should be configured with virtual environment")
							}
						}
					}
				}
			}
		}
	}
}

func TestGenerateFromProject_JavaProject(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	result, err := generator.GenerateFromProject(ctx, validJavaProject)

	if err != nil {
		t.Fatalf("GenerateFromProject failed: %v", err)
	}

	// Verify Java-specific server configuration
	server := result.GatewayConfig.Servers[0]
	if server.Name != "jdtls-project" {
		t.Errorf("Expected server name 'jdtls-project', got %s", server.Name)
	}

	// Verify Java-specific optimizations
	if jdtSettings, exists := server.Settings["java"]; exists {
		javaMap, ok := jdtSettings.(map[string]interface{})
		if !ok {
			t.Error("java settings should be a map")
		} else {
			if config, exists := javaMap["configuration"]; exists {
				configMap, ok := config.(map[string]interface{})
				if !ok {
					t.Error("configuration should be a map")
				} else {
					if updateBuild, exists := configMap["updateBuildConfiguration"]; !exists || updateBuild != "interactive" {
						t.Error("updateBuildConfiguration should be 'interactive'")
					}
				}
			}
		}
	}

	// Verify Maven-specific settings were applied
	if javaCtx, exists := validJavaProject.Metadata["java_context"]; exists {
		if javaProjectCtx, ok := javaCtx.(*JavaProjectContext); ok && javaProjectCtx.BuildSystem == "maven" {
			if jdtSettings, exists := server.Settings["java"]; exists {
				javaMap := jdtSettings.(map[string]interface{})
				if importSettings, exists := javaMap["import"]; exists {
					importMap, ok := importSettings.(map[string]interface{})
					if ok {
						if maven, exists := importMap["maven"]; exists {
							mavenMap, ok := maven.(map[string]interface{})
							if ok {
								if enabled, exists := mavenMap["enabled"]; !exists || enabled != true {
									t.Error("maven import should be enabled")
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestGenerateFromProject_NodeJSProject(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	result, err := generator.GenerateFromProject(ctx, validNodeJSProject)

	if err != nil {
		t.Fatalf("GenerateFromProject failed: %v", err)
	}

	// Verify TypeScript server configuration
	server := result.GatewayConfig.Servers[0]
	if server.Name != "typescript-language-server-project" {
		t.Errorf("Expected server name 'typescript-language-server-project', got %s", server.Name)
	}

	// Verify TypeScript-specific optimizations
	if tsSettings, exists := server.Settings["typescript"]; exists {
		tsMap, ok := tsSettings.(map[string]interface{})
		if !ok {
			t.Error("typescript settings should be a map")
		} else {
			if prefs, exists := tsMap["preferences"]; exists {
				prefsMap, ok := prefs.(map[string]interface{})
				if !ok {
					t.Error("preferences should be a map")
				} else {
					if disableSuggestions, exists := prefsMap["disableSuggestions"]; !exists || disableSuggestions != false {
						t.Error("disableSuggestions should be false")
					}
				}
			}
		}
	}
}

func TestGenerateFromProject_MixedLanguageProject(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	result, err := generator.GenerateFromProject(ctx, mixedLanguageProject)

	if err != nil {
		t.Fatalf("GenerateFromProject failed: %v", err)
	}

	// Should generate multiple servers for mixed language project
	expectedServerCount := 3 // gopls, pylsp, typescript-language-server
	if result.ServersGenerated != expectedServerCount {
		t.Errorf("Expected %d servers generated, got %d", expectedServerCount, result.ServersGenerated)
	}

	if len(result.GatewayConfig.Servers) != expectedServerCount {
		t.Errorf("Expected %d servers in config, got %d", expectedServerCount, len(result.GatewayConfig.Servers))
	}

	// Verify all expected servers are present
	serverNames := make(map[string]bool)
	for _, server := range result.GatewayConfig.Servers {
		serverNames[server.Name] = true
	}

	expectedServers := []string{"gopls-project", "pylsp-project", "typescript-language-server-project"}
	for _, expectedServer := range expectedServers {
		if !serverNames[expectedServer] {
			t.Errorf("Expected server %s not found", expectedServer)
		}
	}

	// Verify multi-language optimizations are applied
	if result.ProjectConfig.Optimizations == nil {
		t.Error("Optimizations should be set")
	} else {
		if multiLang, exists := result.ProjectConfig.Optimizations["multi_language_mode"]; !exists || multiLang != true {
			t.Error("multi_language_mode optimization should be enabled for mixed language project")
		}
	}
}

func TestGenerateFromProject_LargeProject(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	result, err := generator.GenerateFromProject(ctx, largeGoProject)

	if err != nil {
		t.Fatalf("GenerateFromProject failed: %v", err)
	}

	// Verify large project optimizations are applied
	if result.ProjectConfig.Optimizations == nil {
		t.Error("Optimizations should be set")
	} else {
		if largeProjectMode, exists := result.ProjectConfig.Optimizations["large_project_mode"]; !exists || largeProjectMode != true {
			t.Error("large_project_mode optimization should be enabled for large project")
		}

		if indexingStrategy, exists := result.ProjectConfig.Optimizations["indexing_strategy"]; !exists || indexingStrategy != "lazy" {
			t.Error("indexing_strategy should be 'lazy' for large project")
		}
	}

	// Verify server-level large project optimizations
	server := result.GatewayConfig.Servers[0]
	if goplsSettings, exists := server.Settings["gopls"]; exists {
		goplsMap, ok := goplsSettings.(map[string]interface{})
		if ok {
			if memoryMode, exists := goplsMap["memoryMode"]; !exists || memoryMode != "DegradeClosed" {
				t.Error("memoryMode should be 'DegradeClosed' for large Go project")
			}

			if analyses, exists := goplsMap["analyses"]; exists {
				analysesMap, ok := analyses.(map[string]interface{})
				if ok {
					if shadow, exists := analysesMap["shadow"]; !exists || shadow != false {
						t.Error("shadow analysis should be disabled for large project")
					}
				}
			}
		}
	}

	// Verify optimization level metadata
	if optimizationLevel, exists := result.Metadata["optimization_level"]; !exists || optimizationLevel != "balanced" {
		t.Error("optimization_level should be 'balanced' for large project")
	}
}

func TestGenerateFromProject_MonorepoProject(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	result, err := generator.GenerateFromProject(ctx, monorepoProject)

	if err != nil {
		t.Fatalf("GenerateFromProject failed: %v", err)
	}

	// Verify monorepo optimizations are applied
	if result.ProjectConfig.Optimizations == nil {
		t.Error("Optimizations should be set")
	} else {
		if monorepoMode, exists := result.ProjectConfig.Optimizations["monorepo_mode"]; !exists || monorepoMode != true {
			t.Error("monorepo_mode optimization should be enabled for monorepo project")
		}

		if workspaceSymbolCache, exists := result.ProjectConfig.Optimizations["workspace_symbol_cache"]; !exists || workspaceSymbolCache != true {
			t.Error("workspace_symbol_cache optimization should be enabled for monorepo project")
		}
	}

	// Verify metadata indicates monorepo
	if isMonorepo, exists := result.Metadata["is_monorepo"]; !exists || isMonorepo != true {
		t.Error("is_monorepo metadata should be true")
	}
}

func TestGenerateFromProject_ErrorConditions(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	// Test with nil project context
	generator := NewProjectConfigGenerator(logger, registry, verifier)
	_, err := generator.GenerateFromProject(ctx, nil)
	if err == nil {
		t.Error("Should return error with nil project context")
	}

	// Test with nil server registry
	generator.SetServerRegistry(nil)
	_, err = generator.GenerateFromProject(ctx, validGoProject)
	if err == nil {
		t.Error("Should return error with nil server registry")
	}

	// Test with servers not installed
	generator.SetServerRegistry(registry)
	verifier.SetServerInstalled(setup.SERVER_GOPLS, false)
	result, err := generator.GenerateFromProject(ctx, validGoProject)
	if err != nil {
		t.Fatalf("Should not fail with uninstalled server: %v", err)
	}

	// Should have skipped the uninstalled server
	if result.ServersSkipped != 1 {
		t.Errorf("Expected 1 server skipped, got %d", result.ServersSkipped)
	}

	if len(result.Warnings) == 0 {
		t.Error("Should have warnings about uninstalled server")
	}
}

// Test ApplyLanguageOptimizations method

func TestApplyLanguageOptimizations(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	// Create test server configs
	servers := []config.ServerConfig{
		{
			Name:      "gopls-test",
			Languages: []string{"go"},
			Command:   "gopls",
			Args:      []string{},
			Transport: "stdio",
			Settings:  make(map[string]interface{}),
		},
		{
			Name:      "pylsp-test",
			Languages: []string{"python"},
			Command:   "pylsp",
			Args:      []string{},
			Transport: "stdio",
			Settings:  make(map[string]interface{}),
		},
	}

	optimized, err := generator.ApplyLanguageOptimizations(ctx, servers, mixedLanguageProject)

	if err != nil {
		t.Fatalf("ApplyLanguageOptimizations failed: %v", err)
	}

	if len(optimized) != len(servers) {
		t.Errorf("Expected %d optimized servers, got %d", len(servers), len(optimized))
	}

	// Verify Go optimizations were applied
	goServer := optimized[0]
	if goServer.Settings["gopls"] == nil {
		t.Error("Go server should have gopls settings")
	}

	// Verify Python optimizations were applied
	pythonServer := optimized[1]
	if pythonServer.Settings["pylsp"] == nil {
		t.Error("Python server should have pylsp settings")
	}

	// Verify project size optimizations for large project
	optimizedLarge, err := generator.ApplyLanguageOptimizations(ctx, servers[:1], largeGoProject)
	if err != nil {
		t.Fatalf("ApplyLanguageOptimizations failed for large project: %v", err)
	}

	// Large project should have additional optimizations
	largeGoServer := optimizedLarge[0]
	if largeGoServer.Settings["maxFileSize"] == nil {
		t.Error("Large project should have maxFileSize setting")
	}

	if watchFiles, exists := largeGoServer.Settings["watchFiles"]; !exists || watchFiles != false {
		t.Error("Large project should have watchFiles disabled")
	}
}

// Test GenerateServerOverrides method

func TestGenerateServerOverrides(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	overrides, err := generator.GenerateServerOverrides(ctx, validGoProject)

	if err != nil {
		t.Fatalf("GenerateServerOverrides failed: %v", err)
	}

	if len(overrides) != 1 {
		t.Errorf("Expected 1 override, got %d", len(overrides))
	}

	override := overrides[0]
	if override.Name != setup.SERVER_GOPLS {
		t.Errorf("Expected override for %s, got %s", setup.SERVER_GOPLS, override.Name)
	}

	// Verify template settings were applied
	if override.Settings == nil {
		t.Error("Override settings should not be nil")
	}

	if len(override.Settings) == 0 {
		t.Error("Override should have settings from template")
	}

	// Test with project that triggers optimization rules
	overridesLarge, err := generator.GenerateServerOverrides(ctx, largeGoProject)
	if err != nil {
		t.Fatalf("GenerateServerOverrides failed for large project: %v", err)
	}

	// Should have optimization rule applied for large project
	if len(overridesLarge) == 0 {
		t.Error("Should have overrides for large project")
	}
}

// Test FilterGlobalServers method

func TestFilterGlobalServers(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	// Create global servers list
	globalServers := []config.ServerConfig{
		{Name: "gopls-global", Languages: []string{"go"}},
		{Name: "pylsp-global", Languages: []string{"python"}},
		{Name: "jdtls-global", Languages: []string{"java"}},
		{Name: "typescript-global", Languages: []string{"typescript", "javascript"}},
	}

	// Filter for Go project
	filtered, err := generator.FilterGlobalServers(ctx, globalServers, validGoProject)

	if err != nil {
		t.Fatalf("FilterGlobalServers failed: %v", err)
	}

	if len(filtered) != 1 {
		t.Errorf("Expected 1 filtered server for Go project, got %d", len(filtered))
	}

	if filtered[0].Name != "gopls-global" {
		t.Errorf("Expected gopls-global server, got %s", filtered[0].Name)
	}

	// Filter for mixed language project
	filteredMixed, err := generator.FilterGlobalServers(ctx, globalServers, mixedLanguageProject)

	if err != nil {
		t.Fatalf("FilterGlobalServers failed for mixed project: %v", err)
	}

	expectedCount := 3 // go, python, typescript
	if len(filteredMixed) != expectedCount {
		t.Errorf("Expected %d filtered servers for mixed project, got %d", expectedCount, len(filteredMixed))
	}

	// Verify correct servers were filtered
	serverNames := make(map[string]bool)
	for _, server := range filteredMixed {
		serverNames[server.Name] = true
	}

	expectedServers := []string{"gopls-global", "pylsp-global", "typescript-global"}
	for _, expectedServer := range expectedServers {
		if !serverNames[expectedServer] {
			t.Errorf("Expected server %s not found in filtered results", expectedServer)
		}
	}

	// Filter with explicitly required servers
	projectWithRequired := *validPythonProject
	projectWithRequired.RequiredServers = []string{"pylsp-global", "gopls-global"} // Include non-matching language

	filteredRequired, err := generator.FilterGlobalServers(ctx, globalServers, &projectWithRequired)

	if err != nil {
		t.Fatalf("FilterGlobalServers failed with required servers: %v", err)
	}

	if len(filteredRequired) != 2 {
		t.Errorf("Expected 2 servers (1 matching language + 1 explicitly required), got %d", len(filteredRequired))
	}
}

// Test ValidateProjectConfig method

func TestValidateProjectConfig_ValidConfig(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	// Create valid project config
	projectConfig := &config.ProjectConfig{
		ProjectID:     "test-go-project",
		Name:          "Test Go Project",
		RootDirectory: "/test/go-project",
		ServerOverrides: []config.ProjectServerOverride{
			{
				Name: setup.SERVER_GOPLS,
				Settings: map[string]interface{}{
					"gofumpt": true,
				},
			},
		},
		EnabledServers: []string{setup.SERVER_GOPLS},
		Optimizations:  make(map[string]interface{}),
		GeneratedAt:    time.Now(),
	}

	result, err := generator.ValidateProjectConfig(ctx, projectConfig, validGoProject)

	if err != nil {
		t.Fatalf("ValidateProjectConfig failed: %v", err)
	}

	if !result.IsValid {
		t.Error("Valid project config should be marked as valid")
	}

	if result.ServersValidated != 1 {
		t.Errorf("Expected 1 server validated, got %d", result.ServersValidated)
	}

	if len(result.ValidationErrors) > 0 {
		t.Errorf("Valid config should not have validation errors: %v", result.ValidationErrors)
	}
}

func TestValidateProjectConfig_InvalidConfig(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	// Create project config with non-existent server
	projectConfig := &config.ProjectConfig{
		ProjectID:     "test-invalid-project",
		Name:          "Test Invalid Project",
		RootDirectory: "/test/invalid-project",
		ServerOverrides: []config.ProjectServerOverride{
			{
				Name: "non-existent-server",
				Settings: map[string]interface{}{
					"test": true,
				},
			},
		},
		EnabledServers: []string{"non-existent-server"},
		Optimizations:  make(map[string]interface{}),
		GeneratedAt:    time.Now(),
	}

	result, err := generator.ValidateProjectConfig(ctx, projectConfig, validGoProject)

	if err != nil {
		t.Fatalf("ValidateProjectConfig failed: %v", err)
	}

	if len(result.CompatibilityIssues) == 0 {
		t.Error("Should have compatibility issues for non-existent server")
	}

	// Test with uninstalled server
	verifier.SetServerInstalled(setup.SERVER_GOPLS, false)
	
	validProjectConfig := &config.ProjectConfig{
		ProjectID:     "test-uninstalled-project",
		Name:          "Test Uninstalled Project",
		RootDirectory: "/test/uninstalled-project",
		ServerOverrides: []config.ProjectServerOverride{
			{
				Name: setup.SERVER_GOPLS,
				Settings: map[string]interface{}{
					"gofumpt": true,
				},
			},
		},
		EnabledServers: []string{setup.SERVER_GOPLS},
		Optimizations:  make(map[string]interface{}),
		GeneratedAt:    time.Now(),
	}

	resultUninstalled, err := generator.ValidateProjectConfig(ctx, validProjectConfig, validGoProject)

	if err != nil {
		t.Fatalf("ValidateProjectConfig failed for uninstalled server: %v", err)
	}

	if len(resultUninstalled.ValidationWarnings) == 0 {
		t.Error("Should have validation warnings for uninstalled server")
	}
}

func TestValidateProjectConfig_PerformanceWarnings(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	// Create project config for large project without optimizations
	projectConfig := &config.ProjectConfig{
		ProjectID:       "test-large-project",
		Name:            "Test Large Project",
		RootDirectory:   "/test/large-project",
		ServerOverrides: []config.ProjectServerOverride{},
		EnabledServers:  []string{setup.SERVER_GOPLS},
		Optimizations:   make(map[string]interface{}), // No optimizations
		GeneratedAt:     time.Now(),
	}

	result, err := generator.ValidateProjectConfig(ctx, projectConfig, largeGoProject)

	if err != nil {
		t.Fatalf("ValidateProjectConfig failed: %v", err)
	}

	if len(result.PerformanceWarnings) == 0 {
		t.Error("Should have performance warnings for large project without optimizations")
	}
}

func TestValidateProjectConfig_OptimizationSuggestions(t *testing.T) {
	ctx := context.Background()
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	// Create project config for monorepo without monorepo optimizations
	projectConfig := &config.ProjectConfig{
		ProjectID:       "test-monorepo-project",
		Name:            "Test Monorepo Project",
		RootDirectory:   "/test/monorepo-project",
		ServerOverrides: []config.ProjectServerOverride{},
		EnabledServers:  []string{setup.SERVER_GOPLS},
		Optimizations:   make(map[string]interface{}), // No optimizations
		GeneratedAt:     time.Now(),
	}

	result, err := generator.ValidateProjectConfig(ctx, projectConfig, monorepoProject)

	if err != nil {
		t.Fatalf("ValidateProjectConfig failed: %v", err)
	}

	if len(result.OptimizationSuggestions) == 0 {
		t.Error("Should have optimization suggestions for monorepo without optimizations")
	}
}

// Test configuration setter methods

func TestConfigurationSetters(t *testing.T) {
	generator := &ProjectConfigGeneratorImpl{}

	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator.SetLogger(logger)
	generator.SetServerRegistry(registry)
	generator.SetServerVerifier(verifier)

	if generator.logger != logger {
		t.Error("SetLogger failed")
	}

	if generator.serverRegistry != registry {
		t.Error("SetServerRegistry failed")
	}

	if generator.serverVerifier != verifier {
		t.Error("SetServerVerifier failed")
	}
}

// Test language optimizer functionality

func TestGoOptimizer(t *testing.T) {
	optimizer := &GoOptimizer{}
	ctx := context.Background()

	serverConfig := &config.ServerConfig{
		Name:      "gopls-test",
		Languages: []string{"go"},
		Settings:  make(map[string]interface{}),
	}

	err := optimizer.OptimizeForProject(ctx, serverConfig, validGoProject)

	if err != nil {
		t.Fatalf("Go optimizer failed: %v", err)
	}

	// Verify gopls settings were applied
	if goplsSettings, exists := serverConfig.Settings["gopls"]; !exists {
		t.Error("gopls settings should be present")
	} else {
		goplsMap, ok := goplsSettings.(map[string]interface{})
		if !ok {
			t.Error("gopls settings should be a map")
		} else {
			if gofumpt, exists := goplsMap["gofumpt"]; !exists || gofumpt != true {
				t.Error("gofumpt should be enabled")
			}
		}
	}

	// Test with large project
	err = optimizer.OptimizeForProject(ctx, serverConfig, largeGoProject)
	if err != nil {
		t.Fatalf("Go optimizer failed for large project: %v", err)
	}

	// Verify large project optimizations
	if goplsSettings, exists := serverConfig.Settings["gopls"]; exists {
		goplsMap := goplsSettings.(map[string]interface{})
		if memoryMode, exists := goplsMap["memoryMode"]; !exists || memoryMode != "DegradeClosed" {
			t.Error("memoryMode should be DegradeClosed for large project")
		}
	}

	// Test performance settings
	// Convert types.ProjectSize to local ProjectSize
	localProjectSize := ProjectSize{
		TotalFiles:     largeGoProject.ProjectSize.TotalFiles,
		SourceFiles:    largeGoProject.ProjectSize.SourceFiles,
		TestFiles:      largeGoProject.ProjectSize.TestFiles,
		ConfigFiles:    largeGoProject.ProjectSize.ConfigFiles,
		TotalSizeBytes: largeGoProject.ProjectSize.TotalSizeBytes,
	}
	perfSettings := optimizer.GetPerformanceSettings(localProjectSize)
	if memoryMode, exists := perfSettings["memoryMode"]; !exists || memoryMode != "DegradeClosed" {
		t.Error("Performance settings should include memoryMode for large project")
	}

	// Test workspace settings
	workspaceSettings := optimizer.GetWorkspaceSettings(validGoProject)
	if filters, exists := workspaceSettings["directoryFilters"]; !exists {
		t.Error("Workspace settings should include directoryFilters")
	} else {
		filtersSlice, ok := filters.([]string)
		if !ok || len(filtersSlice) == 0 {
			t.Error("directoryFilters should be a non-empty slice")
		}
	}
}

func TestPythonOptimizer(t *testing.T) {
	optimizer := &PythonOptimizer{}
	ctx := context.Background()

	serverConfig := &config.ServerConfig{
		Name:      "pylsp-test",
		Languages: []string{"python"},
		Settings:  make(map[string]interface{}),
	}

	err := optimizer.OptimizeForProject(ctx, serverConfig, validPythonProject)

	if err != nil {
		t.Fatalf("Python optimizer failed: %v", err)
	}

	// Verify pylsp settings were applied
	if pylspSettings, exists := serverConfig.Settings["pylsp"]; !exists {
		t.Error("pylsp settings should be present")
	} else {
		pylspMap, ok := pylspSettings.(map[string]interface{})
		if !ok {
			t.Error("pylsp settings should be a map")
		} else {
			if _, exists := pylspMap["plugins"]; !exists {
				t.Error("plugins should be present")
			}
		}
	}

	// Test performance settings for large project
	largePythonProject := *validPythonProject
	largePythonProject.ProjectSize.TotalFiles = 300 // Trigger performance optimizations

	// Convert types.ProjectSize to local ProjectSize
	localPythonProjectSize := ProjectSize{
		TotalFiles:     largePythonProject.ProjectSize.TotalFiles,
		SourceFiles:    largePythonProject.ProjectSize.SourceFiles,
		TestFiles:      largePythonProject.ProjectSize.TestFiles,
		ConfigFiles:    largePythonProject.ProjectSize.ConfigFiles,
		TotalSizeBytes: largePythonProject.ProjectSize.TotalSizeBytes,
	}
	perfSettings := optimizer.GetPerformanceSettings(localPythonProjectSize)
	if ropeCompletion, exists := perfSettings["rope_completion"]; exists {
		ropeMap, ok := ropeCompletion.(map[string]interface{})
		if ok {
			if enabled, exists := ropeMap["enabled"]; !exists || enabled != false {
				t.Error("rope_completion should be disabled for large Python project")
			}
		}
	}
}

func TestTypeScriptOptimizer(t *testing.T) {
	optimizer := &TypeScriptOptimizer{}
	ctx := context.Background()

	serverConfig := &config.ServerConfig{
		Name:      "typescript-test",
		Languages: []string{"typescript"},
		Settings:  make(map[string]interface{}),
	}

	err := optimizer.OptimizeForProject(ctx, serverConfig, validNodeJSProject)

	if err != nil {
		t.Fatalf("TypeScript optimizer failed: %v", err)
	}

	// Verify TypeScript settings were applied
	if _, exists := serverConfig.Settings["typescript"]; !exists {
		t.Error("typescript settings should be present")
	}

	if _, exists := serverConfig.Settings["javascript"]; !exists {
		t.Error("javascript settings should be present")
	}

	// Test with Yarn package manager
	nodeJSProjectWithYarn := *validNodeJSProject
	nodeJSProjectWithYarn.Metadata["nodejs_context"] = &NodeJSProjectContext{
		PackageManager: types.PKG_MGR_YARN,
	}

	err = optimizer.OptimizeForProject(ctx, serverConfig, &nodeJSProjectWithYarn)
	if err != nil {
		t.Fatalf("TypeScript optimizer failed with Yarn: %v", err)
	}

	// Should have added yarn-pnp argument
	foundYarnArg := false
	for _, arg := range serverConfig.Args {
		if arg == "--yarn-pnp" {
			foundYarnArg = true
			break
		}
	}
	if !foundYarnArg {
		t.Error("Should have added --yarn-pnp argument for Yarn projects")
	}
}

func TestJavaOptimizer(t *testing.T) {
	optimizer := &JavaOptimizer{}
	ctx := context.Background()

	serverConfig := &config.ServerConfig{
		Name:      "jdtls-test",
		Languages: []string{"java"},
		Settings:  make(map[string]interface{}),
	}

	err := optimizer.OptimizeForProject(ctx, serverConfig, validJavaProject)

	if err != nil {
		t.Fatalf("Java optimizer failed: %v", err)
	}

	// Verify Java settings were applied
	if javaSettings, exists := serverConfig.Settings["java"]; !exists {
		t.Error("java settings should be present")
	} else {
		javaMap, ok := javaSettings.(map[string]interface{})
		if !ok {
			t.Error("java settings should be a map")
		} else {
			if _, exists := javaMap["configuration"]; !exists {
				t.Error("configuration should be present")
			}
		}
	}

	// Test performance settings for large project
	largeJavaProject := *validJavaProject
	largeJavaProject.ProjectSize.TotalFiles = 600

	// Convert types.ProjectSize to local ProjectSize
	localJavaProjectSize := ProjectSize{
		TotalFiles:     largeJavaProject.ProjectSize.TotalFiles,
		SourceFiles:    largeJavaProject.ProjectSize.SourceFiles,
		TestFiles:      largeJavaProject.ProjectSize.TestFiles,
		ConfigFiles:    largeJavaProject.ProjectSize.ConfigFiles,
		TotalSizeBytes: largeJavaProject.ProjectSize.TotalSizeBytes,
	}
	perfSettings := optimizer.GetPerformanceSettings(localJavaProjectSize)
	if vmargs, exists := perfSettings["java.jdt.ls.vmargs"]; !exists || vmargs != "-Xmx2G" {
		t.Error("Should set JVM args for large Java project")
	}
}

// Test helper methods

func TestHelperMethods(t *testing.T) {
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	// Test evaluateCondition
	if !generator.evaluateCondition("project_size > 1000", largeGoProject) {
		t.Error("Should evaluate project_size > 1000 as true for large project")
	}

	if generator.evaluateCondition("project_size > 1000", validGoProject) {
		t.Error("Should evaluate project_size > 1000 as false for small project")
	}

	if !generator.evaluateCondition("has_virtual_env", validPythonProject) {
		t.Error("Should evaluate has_virtual_env as true for Python project with virtual env")
	}

	// Test containsLanguage
	if !generator.containsLanguage([]string{"go", "python"}, "go") {
		t.Error("Should find 'go' in language list")
	}

	if generator.containsLanguage([]string{"go", "python"}, "java") {
		t.Error("Should not find 'java' in language list")
	}

	// Test getRootMarkersForProject
	markers := generator.getRootMarkersForProject(validGoProject, "go")
	if len(markers) == 0 {
		t.Error("Should return root markers for Go language")
	}

	expectedMarkers := []string{"go.mod", "go.sum"}
	for _, expected := range expectedMarkers {
		found := false
		for _, marker := range markers {
			if marker == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Should include root marker %s for Go", expected)
		}
	}

	// Test generateProjectID
	projectID := generator.generateProjectID(validGoProject)
	if projectID == "" {
		t.Error("Project ID should not be empty")
	}

	// Test getOptimizationLevel
	if level := generator.getOptimizationLevel(validGoProject); level != "conservative" {
		t.Errorf("Expected 'conservative' optimization level for small project, got %s", level)
	}

	if level := generator.getOptimizationLevel(largeGoProject); level != "balanced" {
		t.Errorf("Expected 'balanced' optimization level for large project, got %s", level)
	}

	// Test very large project
	veryLargeProject := *largeGoProject
	veryLargeProject.ProjectSize.TotalFiles = 2500
	if level := generator.getOptimizationLevel(&veryLargeProject); level != "aggressive" {
		t.Errorf("Expected 'aggressive' optimization level for very large project, got %s", level)
	}
}

// Test edge cases and error conditions

func TestEdgeCases(t *testing.T) {
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)

	// Test with empty project context
	emptyProject := &ProjectContext{
		ProjectType:     "unknown",
		RootPath:        "/empty",
		Languages:       []string{},
		RequiredServers: []string{},
		ProjectSize:     types.ProjectSize{},
	}

	ctx := context.Background()
	result, err := generator.GenerateFromProject(ctx, emptyProject)

	if err != nil {
		t.Fatalf("Should handle empty project gracefully: %v", err)
	}

	if result.ServersGenerated != 0 {
		t.Error("Should not generate servers for empty project")
	}

	// Test with unknown language
	unknownLangProject := &ProjectContext{
		ProjectType:     "unknown",
		RootPath:        "/unknown",
		Languages:       []string{"unknown-language"},
		RequiredServers: []string{},
		ProjectSize:     types.ProjectSize{TotalFiles: 10},
	}

	result2, err := generator.GenerateFromProject(ctx, unknownLangProject)
	if err != nil {
		t.Fatalf("Should handle unknown language gracefully: %v", err)
	}

	if result2.ServersGenerated != 0 {
		t.Error("Should not generate servers for unknown language")
	}

	// Test ApplyLanguageOptimizations with empty servers
	optimized, err := generator.ApplyLanguageOptimizations(ctx, []config.ServerConfig{}, validGoProject)
	if err != nil {
		t.Fatalf("Should handle empty servers list: %v", err)
	}

	if len(optimized) != 0 {
		t.Error("Should return empty list for empty input")
	}

	// Test FilterGlobalServers with empty list
	filtered, err := generator.FilterGlobalServers(ctx, []config.ServerConfig{}, validGoProject)
	if err != nil {
		t.Fatalf("Should handle empty global servers list: %v", err)
	}

	if len(filtered) != 0 {
		t.Error("Should return empty list for empty input")
	}
}

// Performance and benchmarking tests

func BenchmarkGenerateFromProject(b *testing.B) {
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generator.GenerateFromProject(ctx, validGoProject)
		if err != nil {
			b.Fatalf("GenerateFromProject failed: %v", err)
		}
	}
}

func BenchmarkApplyLanguageOptimizations(b *testing.B) {
	logger := createTestLogger()
	registry := NewMockServerRegistry()
	verifier := NewMockServerVerifier()

	generator := NewProjectConfigGenerator(logger, registry, verifier)
	ctx := context.Background()

	servers := []config.ServerConfig{
		{
			Name:      "gopls-test",
			Languages: []string{"go"},
			Settings:  make(map[string]interface{}),
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generator.ApplyLanguageOptimizations(ctx, servers, validGoProject)
		if err != nil {
			b.Fatalf("ApplyLanguageOptimizations failed: %v", err)
		}
	}
}