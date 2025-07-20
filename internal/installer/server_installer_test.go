package installer

import (
	"testing"
	"time"
)

func TestServerRegistry(t *testing.T) {
	registry := NewServerRegistry()

	servers := registry.ListServers()
	if len(servers) != 4 {
		t.Errorf("Expected 4 default servers, got %d", len(servers))
	}

	expectedServers := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}
	for _, expected := range expectedServers {
		found := false
		for _, server := range servers {
			if server == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected server %s not found in registry", expected)
		}
	}
}

func TestServerDefinitions(t *testing.T) {
	registry := NewServerRegistry()

	gopls, err := registry.GetServer("gopls")
	if err != nil {
		t.Fatalf("Failed to get gopls definition: %v", err)
	}

	if gopls.Name != "gopls" {
		t.Errorf("Expected name 'gopls', got '%s'", gopls.Name)
	}

	if gopls.Runtime != "go" {
		t.Errorf("Expected runtime 'go', got '%s'", gopls.Runtime)
	}

	if gopls.MinRuntimeVersion != "1.19.0" {
		t.Errorf("Expected minimum Go version '1.19.0', got '%s'", gopls.MinRuntimeVersion)
	}

	if len(gopls.InstallMethods) == 0 {
		t.Error("Expected install methods to be defined for gopls")
	}

	if len(gopls.VersionCommand) == 0 {
		t.Error("Expected version command to be defined for gopls")
	}
}

func TestServerRegistryOperations(t *testing.T) {
	registry := NewServerRegistry()

	goServers := registry.GetServersByRuntime("go")
	if len(goServers) != 1 {
		t.Errorf("Expected 1 Go server, got %d", len(goServers))
	}

	pythonServers := registry.GetServersByRuntime("python")
	if len(pythonServers) != 1 {
		t.Errorf("Expected 1 Python server, got %d", len(pythonServers))
	}

	goLangServers := registry.GetServersByLanguage("go")
	if len(goLangServers) != 1 {
		t.Errorf("Expected 1 server for Go language, got %d", len(goLangServers))
	}

	_, err := registry.GetServer("unknown")
	if err == nil {
		t.Error("Expected error for unknown server")
	}
}

func TestDependencyValidationResult(t *testing.T) {
	result := &DependencyValidationResult{
		Server:            "gopls",
		Valid:             true,
		RuntimeRequired:   "go",
		RuntimeInstalled:  true,
		RuntimeVersion:    "1.21.0",
		RuntimeCompatible: true,
		Issues:            []Issue{},
		Recommendations:   []string{},
		ValidatedAt:       time.Now(),
		Duration:          100 * time.Millisecond,
	}

	if result.Server != "gopls" {
		t.Errorf("Expected server 'gopls', got '%s'", result.Server)
	}

	if !result.Valid {
		t.Error("Expected validation to be valid")
	}

	if result.RuntimeRequired != "go" {
		t.Errorf("Expected runtime 'go', got '%s'", result.RuntimeRequired)
	}
}

func TestServerInstallOptions(t *testing.T) {
	options := ServerInstallOptions{
		Version:             "latest",
		Force:               false,
		SkipVerify:          false,
		SkipDependencyCheck: false,
		Timeout:             5 * time.Minute,
		Platform:            "linux",
		InstallMethod:       "default",
		WorkingDir:          "/tmp",
	}

	if options.Version != "latest" {
		t.Errorf("Expected version 'latest', got '%s'", options.Version)
	}

	if options.Platform != "linux" {
		t.Errorf("Expected platform 'linux', got '%s'", options.Platform)
	}

	if options.Timeout != 5*time.Minute {
		t.Errorf("Expected timeout 5m, got %v", options.Timeout)
	}
}

func TestServerInstallMethod(t *testing.T) {
	method := ServerInstallMethod{
		Platform:      "all",
		Method:        "go_install",
		Commands:      []string{"go", "install", "golang.org/x/tools/gopls@latest"},
		PreRequisites: []string{"go version"},
		Verification:  []string{"gopls", "version"},
		PostInstall:   []string{},
	}

	if method.Platform != "all" {
		t.Errorf("Expected platform 'all', got '%s'", method.Platform)
	}

	if method.Method != "go_install" {
		t.Errorf("Expected method 'go_install', got '%s'", method.Method)
	}

	if len(method.Commands) != 3 {
		t.Errorf("Expected 3 commands, got %d", len(method.Commands))
	}
}
