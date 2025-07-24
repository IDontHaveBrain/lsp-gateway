package installer_test

import (
	"lsp-gateway/internal/installer"
	"testing"
	"time"
)

func TestServerVerifier_Basic(t *testing.T) {
	runtimeInstaller := installer.NewRuntimeInstaller()

	verifier := installer.NewServerVerifier(runtimeInstaller)

	servers := verifier.GetSupportedServers()
	expectedServers := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}

	if len(servers) != len(expectedServers) {
		t.Errorf("Expected %d servers, got %d", len(expectedServers), len(servers))
	}

	for _, expected := range expectedServers {
		found := false
		for _, server := range servers {
			if server == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected server %s not found in supported servers", expected)
		}
	}
}

func TestServerVerifier_VerifyUnknownServer(t *testing.T) {
	runtimeInstaller := installer.NewRuntimeInstaller()
	verifier := installer.NewServerVerifier(runtimeInstaller)

	_, err := verifier.VerifyServer("unknown-server")
	if err == nil {
		t.Error("Expected error for unknown server, got nil")
	}
}

func TestServerVerifier_VerifyGopls(t *testing.T) {
	runtimeInstaller := installer.NewRuntimeInstaller()
	verifier := installer.NewServerVerifier(runtimeInstaller)

	result, err := verifier.VerifyServer("gopls")
	if err != nil {
		t.Errorf("Unexpected error verifying gopls: %v", err)
	}

	if result == nil {
		t.Error("Expected verification result, got nil")
		return
	}

	if result.ServerName != "gopls" {
		t.Errorf("Expected server name 'gopls', got '%s'", result.ServerName)
	}

	if result.RuntimeRequired != "go" {
		t.Errorf("Expected runtime 'go', got '%s'", result.RuntimeRequired)
	}

	if result.Duration > 30*time.Second {
		t.Errorf("Verification took too long: %v", result.Duration)
	}
}

func TestServerVerifier_VerifyAllServers(t *testing.T) {
	runtimeInstaller := installer.NewRuntimeInstaller()
	verifier := installer.NewServerVerifier(runtimeInstaller)

	results, err := verifier.VerifyAllServers()
	if err != nil {
		t.Errorf("Unexpected error verifying all servers: %v", err)
	}

	expectedServers := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}

	if len(results) != len(expectedServers) {
		t.Errorf("Expected %d server results, got %d", len(expectedServers), len(results))
	}

	for _, serverName := range expectedServers {
		result, exists := results[serverName]
		if !exists {
			t.Errorf("Missing verification result for server: %s", serverName)
			continue
		}

		if result.ServerName != serverName {
			t.Errorf("Result server name mismatch: expected %s, got %s", serverName, result.ServerName)
		}
	}
}

func TestServerVerifier_HealthCheck(t *testing.T) {
	runtimeInstaller := installer.NewRuntimeInstaller()
	verifier := installer.NewServerVerifier(runtimeInstaller)

	servers := verifier.GetSupportedServers()

	for _, serverName := range servers {
		healthResult, err := verifier.HealthCheck(serverName)
		if err != nil {
			t.Logf("Health check failed for %s (expected if not installed): %v", serverName, err)
			continue
		}

		if healthResult == nil {
			t.Errorf("Expected health result for %s, got nil", serverName)
			continue
		}

		if healthResult.ServerName != serverName {
			t.Errorf("Health result server name mismatch: expected %s, got %s", serverName, healthResult.ServerName)
		}

		if healthResult.TestedAt.IsZero() {
			t.Errorf("Health check TestedAt time not set for %s", serverName)
		}
	}
}

func TestServerVerificationResult_Structure(t *testing.T) {
	result := &installer.ServerVerificationResult{
		ServerName:      "test-server",
		Installed:       true,
		Version:         "1.0.0",
		Path:            "/usr/local/bin/test-server",
		Compatible:      true,
		Functional:      true,
		RuntimeRequired: "go",
		Issues:          []installer.Issue{},
		Recommendations: []string{},
		VerifiedAt:      time.Now(),
		Duration:        time.Second,
		Metadata:        make(map[string]interface{}),
	}

	if result.ServerName != "test-server" {
		t.Error("ServerName field not working")
	}

	if !result.Installed {
		t.Error("Installed field not working")
	}

	if result.Version != "1.0.0" {
		t.Error("Version field not working")
	}

	if result.Path != "/usr/local/bin/test-server" {
		t.Error("Path field not working")
	}

	if !result.Compatible {
		t.Error("Compatible field not working")
	}

	if !result.Functional {
		t.Error("Functional field not working")
	}

	if result.RuntimeRequired != "go" {
		t.Error("RuntimeRequired field not working")
	}
}

func TestServerHealthResult_Structure(t *testing.T) {
	result := &installer.ServerHealthResult{
		ServerName:  "test-server",
		Responsive:  true,
		StartupTime: time.Millisecond * 100,
		ExitCode:    0,
		Output:      "test output",
		Error:       "",
		TestedAt:    time.Now(),
		TestMethod:  "version_check",
		Metadata:    make(map[string]interface{}),
	}

	if result.ServerName != "test-server" {
		t.Error("ServerName field not working")
	}

	if !result.Responsive {
		t.Error("Responsive field not working")
	}

	if result.StartupTime != time.Millisecond*100 {
		t.Error("StartupTime field not working")
	}

	if result.ExitCode != 0 {
		t.Error("ExitCode field not working")
	}

	if result.Output != "test output" {
		t.Error("Output field not working")
	}

	if result.TestMethod != "version_check" {
		t.Error("TestMethod field not working")
	}
}
