package testutils

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/stretchr/testify/suite"
)

// IsolatedTestSuite provides resource isolation for testify/suite based tests
type IsolatedTestSuite struct {
	suite.Suite
	isolation   *IsolatedTestSetup
	mu          sync.Mutex
	isSetup     bool
}

// SetupIsolatedSuite initializes the isolated test suite
func (its *IsolatedTestSuite) SetupIsolatedSuite(suiteName string) error {
	its.mu.Lock()
	defer its.mu.Unlock()

	if its.isSetup {
		return fmt.Errorf("suite already setup")
	}

	setup, err := SetupIsolatedTest(suiteName)
	if err != nil {
		return fmt.Errorf("failed to setup isolated suite: %w", err)
	}

	its.isolation = setup
	its.isSetup = true
	return nil
}

// SetupIsolatedSuiteWithLanguage initializes with language-specific configuration
func (its *IsolatedTestSuite) SetupIsolatedSuiteWithLanguage(suiteName, language string) error {
	its.mu.Lock()
	defer its.mu.Unlock()

	if its.isSetup {
		return fmt.Errorf("suite already setup")
	}

	setup, err := SetupIsolatedTestWithLanguage(suiteName, language)
	if err != nil {
		return fmt.Errorf("failed to setup isolated suite: %w", err)
	}

	its.isolation = setup
	its.isSetup = true
	return nil
}

// TearDownIsolatedSuite cleans up all isolated resources
func (its *IsolatedTestSuite) TearDownIsolatedSuite() error {
	its.mu.Lock()
	defer its.mu.Unlock()

	if !its.isSetup || its.isolation == nil {
		return nil
	}

	err := its.isolation.Cleanup()
	its.isolation = nil
	its.isSetup = false
	return err
}

// GetPort returns the allocated port for this suite
func (its *IsolatedTestSuite) GetPort() int {
	if its.isolation == nil {
		return 0
	}
	return its.isolation.Resources.Port
}

// GetPortString returns the allocated port as string
func (its *IsolatedTestSuite) GetPortString() string {
	return strconv.Itoa(its.GetPort())
}

// GetTempDir returns the isolated temporary directory
func (its *IsolatedTestSuite) GetTempDir() string {
	if its.isolation == nil {
		return ""
	}
	return its.isolation.Resources.Directory.Path
}

// GetConfigPath returns the path to the generated config file
func (its *IsolatedTestSuite) GetConfigPath() string {
	if its.isolation == nil {
		return ""
	}
	return its.isolation.ConfigPath
}

// GetHTTPClient returns an HTTP client configured for this suite's server
func (its *IsolatedTestSuite) GetHTTPClient() *HttpClient {
	if its.isolation == nil {
		return nil
	}
	return its.isolation.GetHTTPClient()
}

// StartServer starts the isolated server
func (its *IsolatedTestSuite) StartServer(command []string, timeout time.Duration) error {
	if its.isolation == nil {
		return fmt.Errorf("suite not initialized")
	}
	return its.isolation.StartServer(command, timeout)
}

// WaitForServerReady waits for server to be ready
func (its *IsolatedTestSuite) WaitForServerReady(timeout time.Duration) error {
	if its.isolation == nil {
		return fmt.Errorf("suite not initialized")
	}
	return its.isolation.WaitForServerReady(timeout)
}

// CreateTempConfig creates a temporary config file
func (its *IsolatedTestSuite) CreateTempConfig(content string) (string, error) {
	if its.isolation == nil {
		return "", fmt.Errorf("suite not initialized")
	}
	return its.isolation.CreateTempConfig(content)
}

// CreateTempConfigWithPort creates config with port substitution
func (its *IsolatedTestSuite) CreateTempConfigWithPort(template string) (string, error) {
	if its.isolation == nil {
		return "", fmt.Errorf("suite not initialized")
	}
	return its.isolation.CreateTempConfigWithPort(template)
}

// IsolatedSuiteAdapter provides backward compatibility for existing suites
type IsolatedSuiteAdapter struct {
	*IsolatedTestSuite
	gatewayPort int
	configPath  string
	tempDir     string
}

// NewIsolatedSuiteAdapter creates an adapter for existing test suites
func NewIsolatedSuiteAdapter(suiteName string) (*IsolatedSuiteAdapter, error) {
	adapter := &IsolatedSuiteAdapter{
		IsolatedTestSuite: &IsolatedTestSuite{},
	}

	if err := adapter.SetupIsolatedSuite(suiteName); err != nil {
		return nil, err
	}

	// Set up compatibility fields
	adapter.gatewayPort = adapter.GetPort()
	adapter.configPath = adapter.GetConfigPath()
	adapter.tempDir = adapter.GetTempDir()

	return adapter, nil
}

// NewIsolatedSuiteAdapterWithLanguage creates a language-specific adapter
func NewIsolatedSuiteAdapterWithLanguage(suiteName, language string) (*IsolatedSuiteAdapter, error) {
	adapter := &IsolatedSuiteAdapter{
		IsolatedTestSuite: &IsolatedTestSuite{},
	}

	if err := adapter.SetupIsolatedSuiteWithLanguage(suiteName, language); err != nil {
		return nil, err
	}

	// Set up compatibility fields
	adapter.gatewayPort = adapter.GetPort()
	adapter.configPath = adapter.GetConfigPath()
	adapter.tempDir = adapter.GetTempDir()

	return adapter, nil
}

// Compatibility methods for existing test patterns
func (isa *IsolatedSuiteAdapter) GetGatewayPort() int {
	return isa.gatewayPort
}

func (isa *IsolatedSuiteAdapter) GetConfigPath() string {
	return isa.configPath
}

func (isa *IsolatedSuiteAdapter) GetTempDir() string {
	return isa.tempDir
}

// UpdateConfigPort provides compatibility with existing updateConfigPort patterns
func (isa *IsolatedSuiteAdapter) UpdateConfigPort() {
	// No-op since port is already allocated and config created
}

// PortCompatibilityHelper provides port allocation without full isolation
type PortCompatibilityHelper struct {
	allocatedPort int
	testID        string
}

// AllocatePortForTest allocates a port for backward compatibility
func AllocatePortForTest(testName string) (*PortCompatibilityHelper, error) {
	rm := GetGlobalResourceManager()
	testID := fmt.Sprintf("%s_%d", testName, time.Now().UnixNano())
	
	port, err := rm.portAllocator.AllocatePort(testID)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate port: %w", err)
	}

	return &PortCompatibilityHelper{
		allocatedPort: port,
		testID:        testID,
	}, nil
}

// GetPort returns the allocated port
func (pch *PortCompatibilityHelper) GetPort() int {
	return pch.allocatedPort
}

// GetPortString returns the port as string
func (pch *PortCompatibilityHelper) GetPortString() string {
	return strconv.Itoa(pch.allocatedPort)
}

// Release releases the allocated port
func (pch *PortCompatibilityHelper) Release() {
	rm := GetGlobalResourceManager()
	rm.portAllocator.ReleasePort(pch.allocatedPort)
}

// MigrationHelper provides utilities for migrating existing tests
type MigrationHelper struct{}

// WrapExistingSuite wraps an existing test suite to provide isolation
func (mh *MigrationHelper) WrapExistingSuite(existingSuite interface{}, suiteName string) (*IsolatedSuiteAdapter, error) {
	return NewIsolatedSuiteAdapter(suiteName)
}

// CreatePortOnlyIsolation provides just port isolation for minimal migration
func (mh *MigrationHelper) CreatePortOnlyIsolation(testName string) (*PortCompatibilityHelper, error) {
	return AllocatePortForTest(testName)
}

// GetMigrationHelper returns a migration helper instance
func GetMigrationHelper() *MigrationHelper {
	return &MigrationHelper{}
}

// ParallelTestHelper provides utilities specifically for parallel test execution
type ParallelTestHelper struct {
	activeTests map[string]*IsolatedTestSetup
	mu          sync.RWMutex
}

// NewParallelTestHelper creates a helper for managing parallel tests
func NewParallelTestHelper() *ParallelTestHelper {
	return &ParallelTestHelper{
		activeTests: make(map[string]*IsolatedTestSetup),
	}
}

// StartParallelTest starts a test with full isolation
func (pth *ParallelTestHelper) StartParallelTest(testName string) (*IsolatedTestSetup, error) {
	pth.mu.Lock()
	defer pth.mu.Unlock()

	if _, exists := pth.activeTests[testName]; exists {
		return nil, fmt.Errorf("test %s already running", testName)
	}

	setup, err := SetupIsolatedTest(testName)
	if err != nil {
		return nil, err
	}

	pth.activeTests[testName] = setup
	return setup, nil
}

// FinishParallelTest cleans up a parallel test
func (pth *ParallelTestHelper) FinishParallelTest(testName string) error {
	pth.mu.Lock()
	defer pth.mu.Unlock()

	setup, exists := pth.activeTests[testName]
	if !exists {
		return fmt.Errorf("test %s not found", testName)
	}

	err := setup.Cleanup()
	delete(pth.activeTests, testName)
	return err
}

// CleanupAllParallelTests cleans up all active parallel tests
func (pth *ParallelTestHelper) CleanupAllParallelTests() error {
	pth.mu.Lock()
	defer pth.mu.Unlock()

	var errors []error
	for testName, setup := range pth.activeTests {
		if err := setup.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup test %s: %w", testName, err))
		}
	}

	pth.activeTests = make(map[string]*IsolatedTestSetup)

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	return nil
}

// GetActiveTestCount returns the number of active parallel tests
func (pth *ParallelTestHelper) GetActiveTestCount() int {
	pth.mu.RLock()
	defer pth.mu.RUnlock()
	return len(pth.activeTests)
}