package installer

import (
	"context"
	"errors"
	"lsp-gateway/internal/types"
	"testing"
	"time"
)

type MockRuntimeDetector struct {
	runtimes map[string]*types.RuntimeInfo
}

func NewMockRuntimeDetector() *MockRuntimeDetector {
	return &MockRuntimeDetector{
		runtimes: make(map[string]*types.RuntimeInfo),
	}
}

func (m *MockRuntimeDetector) SetRuntimeInfo(name string, info *types.RuntimeInfo) {
	m.runtimes[name] = info
}

func (m *MockRuntimeDetector) DetectGo(ctx context.Context) (*types.RuntimeInfo, error) {
	if info, exists := m.runtimes["go"]; exists {
		return info, nil
	}
	return &types.RuntimeInfo{
		Name:      "go",
		Installed: false,
	}, nil
}

func (m *MockRuntimeDetector) DetectPython(ctx context.Context) (*types.RuntimeInfo, error) {
	if info, exists := m.runtimes["python"]; exists {
		return info, nil
	}
	return &types.RuntimeInfo{
		Name:      "python",
		Installed: false,
	}, nil
}

func (m *MockRuntimeDetector) DetectNodejs(ctx context.Context) (*types.RuntimeInfo, error) {
	if info, exists := m.runtimes["nodejs"]; exists {
		return info, nil
	}
	return &types.RuntimeInfo{
		Name:      "nodejs",
		Installed: false,
	}, nil
}

func (m *MockRuntimeDetector) DetectJava(ctx context.Context) (*types.RuntimeInfo, error) {
	if info, exists := m.runtimes["java"]; exists {
		return info, nil
	}
	return &types.RuntimeInfo{
		Name:      "java",
		Installed: false,
	}, nil
}

func (m *MockRuntimeDetector) DetectAll(ctx context.Context) (*types.DetectionReport, error) {
	runtimes := make(map[string]*types.RuntimeInfo)

	for name, info := range m.runtimes {
		runtimes[name] = info
	}

	defaultNames := []string{"go", "python", "nodejs", "java"}
	for _, name := range defaultNames {
		if _, exists := runtimes[name]; !exists {
			runtimes[name] = &types.RuntimeInfo{
				Name:      name,
				Installed: false,
			}
		}
	}

	return &types.DetectionReport{
		Runtimes: runtimes,
	}, nil
}

func (m *MockRuntimeDetector) SetLogger(logger interface{})     {}
func (m *MockRuntimeDetector) SetTimeout(timeout time.Duration) {}

type MockRuntimePlatformStrategy struct{}

func (m *MockRuntimePlatformStrategy) InstallRuntime(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
	return &types.InstallResult{Success: true}, nil
}

func (m *MockRuntimePlatformStrategy) VerifyRuntime(runtime string) (*types.VerificationResult, error) {
	return &types.VerificationResult{Installed: true}, nil
}

func (m *MockRuntimePlatformStrategy) GetInstallCommand(runtime, version string) ([]string, error) {
	return []string{"echo", "mock install"}, nil
}

type MockRuntimeInstaller struct {
	runtimes map[string]*types.RuntimeDefinition
}

func NewMockRuntimeInstaller() *MockRuntimeInstaller {
	installer := &MockRuntimeInstaller{
		runtimes: make(map[string]*types.RuntimeDefinition),
	}

	installer.runtimes["go"] = &types.RuntimeDefinition{
		Name:               "go",
		DisplayName:        "Go Programming Language",
		MinVersion:         "1.19.0",
		RecommendedVersion: "1.21.0",
	}

	installer.runtimes["python"] = &types.RuntimeDefinition{
		Name:               "python",
		DisplayName:        "Python Programming Language",
		MinVersion:         "3.8.0",
		RecommendedVersion: "3.11.0",
	}

	installer.runtimes["nodejs"] = &types.RuntimeDefinition{
		Name:               "nodejs",
		DisplayName:        "Node.js JavaScript Runtime",
		MinVersion:         "18.0.0",
		RecommendedVersion: "20.0.0",
	}

	installer.runtimes["java"] = &types.RuntimeDefinition{
		Name:               "java",
		DisplayName:        "Java Development Kit",
		MinVersion:         "17.0.0",
		RecommendedVersion: "21.0.0",
	}

	return installer
}

func (m *MockRuntimeInstaller) Install(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
	return &types.InstallResult{Success: true}, nil
}

func (m *MockRuntimeInstaller) Verify(runtime string) (*types.VerificationResult, error) {
	return &types.VerificationResult{Installed: true}, nil
}

func (m *MockRuntimeInstaller) GetPlatformStrategy(platform string) types.RuntimePlatformStrategy {
	return &MockRuntimePlatformStrategy{}
}

func (m *MockRuntimeInstaller) GetSupportedRuntimes() []string {
	names := make([]string, 0, len(m.runtimes))
	for name := range m.runtimes {
		names = append(names, name)
	}
	return names
}

func (m *MockRuntimeInstaller) GetRuntimeInfo(runtime string) (*types.RuntimeDefinition, error) {
	if def, exists := m.runtimes[runtime]; exists {
		return def, nil
	}
	return nil, &InstallerError{
		Type:    InstallerErrorTypeNotFound,
		Message: "runtime not found",
	}
}

func (m *MockRuntimeInstaller) ValidateVersion(runtime, minVersion string) (*types.VersionValidationResult, error) {
	return &types.VersionValidationResult{
		Valid:            true,
		RequiredVersion:  minVersion,
		InstalledVersion: "1.0.0",
		Issues:           []types.Issue{},
	}, nil
}

func TestNewDependencyValidator(t *testing.T) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)

	if validator == nil {
		t.Fatal("NewDependencyValidator returned nil")
	}

	if validator.serverRegistry != serverRegistry {
		t.Error("Server registry not set correctly")
	}

	if validator.runtimeDetector != runtimeDetector {
		t.Error("Runtime detector not set correctly")
	}

	if validator.runtimeInstaller != runtimeInstaller {
		t.Error("Runtime installer not set correctly")
	}

	if validator.dependencyRegistry == nil {
		t.Error("Dependency registry not initialized")
	}
}

func TestValidateServerDependencies_Success(t *testing.T) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	runtimeDetector.SetRuntimeInfo("go", &types.RuntimeInfo{
		Name:       "go",
		Installed:  true,
		Version:    "go version go1.21.0 linux/amd64",
		Compatible: true,
		Path:       "/usr/local/go/bin/go",
	})

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)

	ctx := context.Background()
	result, err := validator.ValidateServerDependencies(ctx, "gopls")

	if err != nil {
		t.Fatalf("ValidateServerDependencies failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	if result.ServerName != "gopls" {
		t.Errorf("Expected server name 'gopls', got '%s'", result.ServerName)
	}

	if !result.CanInstall {
		t.Error("Expected CanInstall to be true when all dependencies are met")
	}

	if result.RequiredRuntime == nil {
		t.Error("Expected RequiredRuntime to be set")
	}

	if result.RequiredRuntime.Name != "go" {
		t.Errorf("Expected required runtime to be 'go', got '%s'", result.RequiredRuntime.Name)
	}
}

func TestValidateServerDependencies_MissingRuntime(t *testing.T) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	runtimeDetector.SetRuntimeInfo("go", &types.RuntimeInfo{
		Name:      "go",
		Installed: false,
	})

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)

	ctx := context.Background()
	result, err := validator.ValidateServerDependencies(ctx, "gopls")

	if err != nil {
		t.Fatalf("ValidateServerDependencies failed: %v", err)
	}

	if result.CanInstall {
		t.Error("Expected CanInstall to be false when required runtime is missing")
	}

	if len(result.MissingRequired) == 0 {
		t.Error("Expected MissingRequired to contain the missing runtime")
	}

	found := false
	for _, missing := range result.MissingRequired {
		if missing.Name == "go" && missing.Type == "runtime" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Missing Go runtime not found in MissingRequired")
	}
}

func TestValidateServerDependencies_IncompatibleVersion(t *testing.T) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	runtimeDetector.SetRuntimeInfo("go", &types.RuntimeInfo{
		Name:       "go",
		Installed:  true,
		Version:    "go version go1.18.0 linux/amd64",
		Compatible: false, // Below minimum requirement
		Path:       "/usr/local/go/bin/go",
	})

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)

	ctx := context.Background()
	result, err := validator.ValidateServerDependencies(ctx, "gopls")

	if err != nil {
		t.Fatalf("ValidateServerDependencies failed: %v", err)
	}

	if result.CanInstall {
		t.Error("Expected CanInstall to be false when runtime version is incompatible")
	}

	if len(result.VersionIssues) == 0 {
		t.Error("Expected VersionIssues to contain version compatibility issue")
	}

	found := false
	for _, issue := range result.VersionIssues {
		if issue.DependencyName == "go" {
			found = true
			if issue.Severity != types.IssueSeverityCritical {
				t.Errorf("Expected critical severity for required runtime version issue, got %s", issue.Severity)
			}
			break
		}
	}

	if !found {
		t.Error("Go version issue not found in VersionIssues")
	}
}

func TestValidateServerDependencies_UnknownServer(t *testing.T) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)

	ctx := context.Background()
	_, err := validator.ValidateServerDependencies(ctx, "unknown-server")

	if err == nil {
		t.Fatal("Expected error for unknown server")
	}

	var installerErr *InstallerError
	if !errors.As(err, &installerErr) {
		t.Errorf("Expected InstallerError, got %T", err)
	} else if installerErr.Type != InstallerErrorTypeNotFound {
		t.Errorf("Expected InstallerErrorTypeNotFound, got %s", installerErr.Type)
	}
}

func TestValidateAllDependencies(t *testing.T) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	runtimeDetector.SetRuntimeInfo("go", &types.RuntimeInfo{
		Name:       "go",
		Installed:  true,
		Compatible: true,
	})

	runtimeDetector.SetRuntimeInfo("python", &types.RuntimeInfo{
		Name:      "python",
		Installed: false,
	})

	runtimeDetector.SetRuntimeInfo("nodejs", &types.RuntimeInfo{
		Name:       "nodejs",
		Installed:  true,
		Compatible: true,
	})

	runtimeDetector.SetRuntimeInfo("java", &types.RuntimeInfo{
		Name:      "java",
		Installed: false,
	})

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)

	ctx := context.Background()
	result, err := validator.ValidateAllDependencies(ctx)

	if err != nil {
		t.Fatalf("ValidateAllDependencies failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	expectedServers := []string{"gopls", "pylsp", "typescript-language-server", "jdtls"}
	for _, server := range expectedServers {
		if _, exists := result.ServerResults[server]; !exists {
			t.Errorf("Missing result for server: %s", server)
		}
	}

	if len(result.InstallableServers) < 2 {
		t.Errorf("Expected at least 2 installable servers, got %d", len(result.InstallableServers))
	}

	expectedMissing := []string{"python", "java"}
	for _, runtime := range expectedMissing {
		if !containsString(result.MissingRuntimes, runtime) {
			t.Errorf("Expected missing runtime '%s' not found", runtime)
		}
	}

	if result.Summary == nil {
		t.Error("Summary is nil")
	} else {
		if result.Summary.TotalServers != len(expectedServers) {
			t.Errorf("Expected %d total servers, got %d", len(expectedServers), result.Summary.TotalServers)
		}
	}
}

func TestValidateRuntimeForServers(t *testing.T) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	runtimeDetector.SetRuntimeInfo("go", &types.RuntimeInfo{
		Name:       "go",
		Installed:  true,
		Version:    "go version go1.21.0 linux/amd64",
		Compatible: true,
	})

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)

	ctx := context.Background()
	result, err := validator.ValidateRuntimeForServers(ctx, "go")

	if err != nil {
		t.Fatalf("ValidateRuntimeForServers failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result is nil")
	}

	if result.Runtime != "go" {
		t.Errorf("Expected runtime 'go', got '%s'", result.Runtime)
	}

	if !result.RuntimeAvailable {
		t.Error("Expected RuntimeAvailable to be true")
	}

	if len(result.SupportedServers) == 0 {
		t.Error("Expected at least one supported server for Go runtime")
	}

	if !containsString(result.InstallableServers, "gopls") {
		t.Error("Expected gopls to be in installable servers")
	}
}

func TestGetServerDependencies(t *testing.T) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)

	depInfo, err := validator.GetServerDependencies("gopls")

	if err != nil {
		t.Fatalf("GetServerDependencies failed: %v", err)
	}

	if depInfo == nil {
		t.Fatal("Dependency info is nil")
	}

	if depInfo.ServerName != "gopls" {
		t.Errorf("Expected server name 'gopls', got '%s'", depInfo.ServerName)
	}

	if depInfo.RequiredRuntime == nil {
		t.Error("Expected RequiredRuntime to be set")
	} else if depInfo.RequiredRuntime.Name != "go" {
		t.Errorf("Expected required runtime 'go', got '%s'", depInfo.RequiredRuntime.Name)
	}

	if len(depInfo.VerificationSteps) == 0 {
		t.Error("Expected verification steps to be defined")
	}
}

func TestGetInstallationSuggestions(t *testing.T) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)

	result := &DetailedDependencyValidationResult{
		ServerName: "gopls",
		CanInstall: false,
		MissingRequired: []*MissingDependency{
			{
				Type:            "runtime",
				Name:            "go",
				Required:        true,
				MinVersion:      "1.19.0",
				InstallCommands: []string{"npm", "run", "runtime-install", "go"},
				Reason:          "Required to run Go language server",
			},
		},
	}

	ctx := context.Background()
	suggestions, err := validator.GetInstallationSuggestions(ctx, result)

	if err != nil {
		t.Fatalf("GetInstallationSuggestions failed: %v", err)
	}

	if suggestions == nil {
		t.Fatal("Suggestions is nil")
	}

	if len(suggestions.RuntimeInstallations) == 0 {
		t.Error("Expected runtime installation suggestions")
	}

	if len(suggestions.OrderedInstallSequence) == 0 {
		t.Error("Expected ordered installation sequence")
	}

	if suggestions.EstimatedTime == 0 {
		t.Error("Expected non-zero estimated time")
	}

	found := false
	for _, runtime := range suggestions.RuntimeInstallations {
		if runtime.Runtime == "go" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Go runtime installation not found in suggestions")
	}
}

func BenchmarkValidateServerDependencies(b *testing.B) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	runtimes := []string{"go", "python", "nodejs", "java"}
	for _, runtime := range runtimes {
		runtimeDetector.SetRuntimeInfo(runtime, &types.RuntimeInfo{
			Name:       runtime,
			Installed:  true,
			Compatible: true,
		})
	}

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := validator.ValidateServerDependencies(ctx, "gopls")
		if err != nil {
			b.Fatalf("Validation failed: %v", err)
		}
	}
}

func BenchmarkValidateAllDependencies(b *testing.B) {
	serverRegistry := NewServerRegistry()
	runtimeDetector := NewMockRuntimeDetector()
	runtimeInstaller := NewMockRuntimeInstaller()

	runtimes := []string{"go", "python", "nodejs", "java"}
	for _, runtime := range runtimes {
		runtimeDetector.SetRuntimeInfo(runtime, &types.RuntimeInfo{
			Name:       runtime,
			Installed:  true,
			Compatible: true,
		})
	}

	validator := NewDependencyValidator(serverRegistry, runtimeDetector, runtimeInstaller)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := validator.ValidateAllDependencies(ctx)
		if err != nil {
			b.Fatalf("Validation failed: %v", err)
		}
	}
}
