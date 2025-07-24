# Test Infrastructure for Auto-Setup Functionality

This directory contains comprehensive test infrastructure for the LSP Gateway auto-setup functionality, including mock implementations, test utilities, and example tests.

## Directory Structure

```
tests/
├── unit/                    # Unit tests
│   └── internal/
│       ├── cli/            # CLI command tests
│       └── setup/          # Setup component tests
├── integration/            # Integration tests
│   ├── setup_integration_test.go    # End-to-end setup workflow tests
│   ├── jdtls_integration_test.go    # JDTLS installation pipeline tests
│   └── JDTLS_TESTING.md            # JDTLS integration test documentation
├── mocks/                  # Mock implementations
│   ├── runtime_detector_mock.go
│   ├── runtime_installer_mock.go
│   ├── server_installer_mock.go
│   └── config_generator_mock.go
├── testdata/              # Test utilities and helpers
│   └── test_helpers.go
└── README.md             # This file
```

## Mock Implementations

### RuntimeDetector Mock (`mocks/runtime_detector_mock.go`)

Mocks the `setup.RuntimeDetector` interface for testing runtime detection functionality.

**Key Features:**
- Tracks all method calls for verification
- Supports custom behavior via function fields
- Provides realistic default responses
- Includes `Reset()` method for test cleanup

**Usage Example:**
```go
mockDetector := mocks.NewMockRuntimeDetector()

// Use default behavior
report, err := mockDetector.DetectAll(ctx)

// Or customize behavior
mockDetector.DetectGoFunc = func(ctx context.Context) (*setup.RuntimeInfo, error) {
    return &setup.RuntimeInfo{Name: "go", Installed: false}, nil
}

// Verify calls
assert.Len(t, mockDetector.DetectAllCalls, 1)
```

### RuntimeInstaller Mock (`mocks/runtime_installer_mock.go`)

Mocks the `types.RuntimeInstaller` interface for testing runtime installation functionality.

**Key Features:**
- Complete InstallResult and VerificationResult mocking
- Platform strategy mocking included
- Realistic server definitions for supported runtimes
- Call tracking for all methods

**Usage Example:**
```go
mockInstaller := mocks.NewMockRuntimeInstaller()

result, err := mockInstaller.Install("go", installOptions)
assert.True(t, result.Success)

// Verify installation calls
assert.Len(t, mockInstaller.InstallCalls, 1)
assert.Equal(t, "go", mockInstaller.InstallCalls[0].Runtime)
```

### ServerInstaller Mock (`mocks/server_installer_mock.go`)

Mocks the `types.ServerInstaller` interface for testing language server installation.

**Key Features:**
- Comprehensive server definitions for gopls, pylsp, typescript-language-server, jdtls
- Dependency validation mocking
- Platform strategy support
- Realistic server metadata

**Usage Example:**
```go
mockServerInstaller := mocks.NewMockServerInstaller()

// Install a language server
result, err := mockServerInstaller.Install("gopls", serverOptions)
assert.True(t, result.Success)

// Get server information
serverInfo, err := mockServerInstaller.GetServerInfo("gopls")
assert.Equal(t, "Go Language Server", serverInfo.DisplayName)
```

### ConfigGenerator Mock (`mocks/config_generator_mock.go`)

Mocks the `setup.ConfigGenerator` interface for testing configuration generation.

**Key Features:**
- Auto-detected configuration generation
- Runtime-specific configuration
- Configuration validation mocking
- Update operation support

**Usage Example:**
```go
mockGenerator := mocks.NewMockConfigGenerator()

// Generate configuration from detected runtimes
result, err := mockGenerator.GenerateFromDetected(ctx)
assert.Equal(t, 4, result.ServersGenerated)

// Validate configuration
validation, err := mockGenerator.ValidateConfig(config)
assert.True(t, validation.Valid)
```

## Test Utilities (`testdata/test_helpers.go`)

Comprehensive helper functions for creating test data and managing test contexts.

### Test Context Management

```go
// Create test context with timeout
testRunner := testdata.NewTestRunner(time.Second * 30)
defer testRunner.Cleanup()

// Use context in tests
ctx := testRunner.Context()
```

### Mock Data Creation

```go
// Create runtime information
runtimeInfo := testdata.CreateMockRuntimeInfo("go", "1.24.0", "/usr/local/go/bin/go", true, true)

// Create installation results
installResult := testdata.CreateMockInstallResult("python", "3.11.0", "/usr/bin/python3", true)

// Create verification results
verifyResult := testdata.CreateMockVerificationResult("nodejs", "20.0.0", "/usr/bin/node", true, true)

// Create gateway configuration
config := testdata.CreateMockGatewayConfig(8080, testdata.CreateDefaultTestServers())
```

### Detection Reports

```go
// Create comprehensive detection report
report := testdata.CreateMockDetectionReport(platform.PlatformLinux, platform.ArchAMD64)
assert.Equal(t, 4, report.Summary.TotalRuntimes)
```

## Example Tests

### Unit Test Example (`tests/unit/internal/setup/orchestrator_test.go`)

Demonstrates testing individual components with mocks:

```go
func TestSetupOrchestrator_BasicFunctionality(t *testing.T) {
    mockDetector := mocks.NewMockRuntimeDetector()
    mockInstaller := mocks.NewMockRuntimeInstaller()
    
    // Test runtime detection
    report, err := mockDetector.DetectAll(ctx)
    require.NoError(t, err)
    assert.Equal(t, 4, report.Summary.TotalRuntimes)
    
    // Verify mock was called
    assert.Len(t, mockDetector.DetectAllCalls, 1)
}
```

### Integration Test Example (`tests/unit/internal/cli/setup_test.go`)

Shows testing complete auto-setup workflow:

```go
func TestSetupCommand_MockIntegration(t *testing.T) {
    // Setup all mocks
    mockDetector := mocks.NewMockRuntimeDetector()
    mockRuntimeInstaller := mocks.NewMockRuntimeInstaller()
    mockServerInstaller := mocks.NewMockServerInstaller()
    mockConfigGenerator := mocks.NewMockConfigGenerator()
    
    // Test complete workflow
    t.Run("DetectRuntimes", func(t *testing.T) { /* ... */ })
    t.Run("InstallMissingRuntimes", func(t *testing.T) { /* ... */ })
    t.Run("InstallLanguageServers", func(t *testing.T) { /* ... */ })
    t.Run("GenerateConfiguration", func(t *testing.T) { /* ... */ })
}
```

## Integration Tests

### JDTLS Integration Tests (`integration/jdtls_integration_test.go`)

Comprehensive integration tests for Eclipse JDT Language Server (JDTLS) installation pipeline.

**Test Coverage:**
- Complete installation pipeline (download → extract → script creation → verification)
- Cross-platform path resolution (Linux, macOS, Windows)
- SHA256 checksum verification and mismatch handling
- Installation failure recovery with retry logic
- Platform-specific executable script generation
- Installation verification and issue detection

**Key Features:**
- Mock HTTP servers for download simulation
- Configurable failure scenarios (network timeouts, server errors, checksum failures)
- Temporary file system with automatic cleanup
- Platform-specific environment simulation
- Comprehensive directory structure validation

**Usage:**
```bash
# Run all JDTLS integration tests
make test-jdtls-integration

# Run specific JDTLS test functions
go test -v -run "TestJDTLSIntegration_CompleteInstallationPipeline" ./tests/integration/...
go test -v -run "TestJDTLSIntegration_CrossPlatformPathResolution" ./tests/integration/...
go test -v -run "TestJDTLSIntegration_ChecksumVerification" ./tests/integration/...
```

For detailed information, see [`integration/JDTLS_TESTING.md`](integration/JDTLS_TESTING.md).

### Setup Integration Tests (`integration/setup_integration_test.go`)

End-to-end workflow tests for the complete setup orchestration process.

**Test Coverage:**
- Complete setup workflows with different configurations
- Error recovery mechanisms and retry logic
- Options validation and configuration
- Parallel execution scenarios
- Resource management and cleanup

## Running Tests

### Run All Tests
```bash
make test
# or
go test ./tests/... -v
```

### Run Unit Tests Only
```bash
make test-unit
# or
go test ./tests/unit/... -v
```

### Run Integration Tests
```bash
make test-integration
# or
go test ./tests/integration/... -v
```

### Run JDTLS Integration Tests
```bash
make test-jdtls-integration
# or
go test -v -timeout 600s -run "TestJDTLS" ./tests/integration/...
```

### Run Specific Test Suite
```bash
go test ./tests/unit/internal/setup/ -v
go test ./tests/unit/internal/cli/ -v
go test ./tests/integration/ -v
```

### Run with Coverage
```bash
go test ./tests/... -cover -v
```

### Run Tests in Short Mode (Skip Long-Running Tests)
```bash
go test ./tests/... -short -v
```

## Writing New Tests

### 1. Create Test File
Create test files in the appropriate directory:
- CLI tests: `tests/unit/internal/cli/`
- Setup tests: `tests/unit/internal/setup/`
- Integration tests: `tests/integration/`

### 2. Use Mocks and Helpers
```go
func TestMyFeature(t *testing.T) {
    // Create test context
    testRunner := testdata.NewTestRunner(time.Second * 30)
    defer testRunner.Cleanup()
    
    // Create mocks
    mockDetector := mocks.NewMockRuntimeDetector()
    
    // Create test data
    runtimeInfo := testdata.CreateMockRuntimeInfo("go", "1.24.0", "/usr/local/go/bin/go", true, true)
    
    // Test your functionality
    result, err := myFunction(mockDetector, runtimeInfo)
    
    // Assert results
    require.NoError(t, err)
    assert.True(t, result.Success)
    
    // Verify mock interactions
    assert.Len(t, mockDetector.SomeMethodCalls, 1)
}
```

### 3. Test Error Scenarios
```go
func TestMyFeature_ErrorHandling(t *testing.T) {
    mockInstaller := mocks.NewMockRuntimeInstaller()
    
    // Configure mock to return error
    mockInstaller.InstallFunc = func(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
        return testdata.CreateMockInstallResult(runtime, "", "", false), nil
    }
    
    // Test error handling
    result, err := myFunction(mockInstaller)
    require.NoError(t, err)
    assert.False(t, result.Success)
}
```

## Dependencies

The test infrastructure uses the following dependencies:

- **testify/assert**: Assertion library for readable test assertions
- **testify/require**: Assertion library that stops test execution on failure
- **testify/mock**: Mock object framework (available but not required)
- **testify/suite**: Test suite framework (available for complex test scenarios)

## Best Practices

1. **Use testdata helpers** for creating consistent test data
2. **Reset mocks** between tests or use fresh instances
3. **Verify mock interactions** to ensure correct behavior
4. **Test both success and error scenarios**
5. **Use meaningful test names** that describe what is being tested
6. **Group related tests** using subtests (`t.Run()`)
7. **Clean up resources** using defer and test contexts
8. **Keep tests focused** on single functionality

## Mock Behavior Customization

All mocks support behavior customization via function fields:

```go
// Customize runtime detection
mockDetector.DetectGoFunc = func(ctx context.Context) (*setup.RuntimeInfo, error) {
    return &setup.RuntimeInfo{
        Name: "go",
        Installed: false,
        Issues: []string{"Go not found in PATH"},
    }, nil
}

// Customize installation behavior
mockInstaller.InstallFunc = func(runtime string, options types.InstallOptions) (*types.InstallResult, error) {
    if runtime == "unsupported" {
        return nil, errors.New("unsupported runtime")
    }
    return testdata.CreateMockInstallResult(runtime, "1.0.0", "/usr/bin/"+runtime, true), nil
}
```

This allows for comprehensive testing of edge cases, error conditions, and specific scenarios without requiring real installations or complex setup.