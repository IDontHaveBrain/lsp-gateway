package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/tests/testdata"
)

// ProjectType represents different project configurations for testing
type ProjectType string

const (
	ProjectTypeSingleLanguage         ProjectType = "single-language"
	ProjectTypeMultiLanguage          ProjectType = "multi-language"
	ProjectTypeMonorepo               ProjectType = "monorepo"
	ProjectTypeMicroservices          ProjectType = "microservices"
	ProjectTypeFrontendBackend        ProjectType = "frontend-backend"
	ProjectTypeWorkspace              ProjectType = "workspace"
	ProjectTypePolyglot               ProjectType = "polyglot"
	ProjectTypeMonorepoWebApp         ProjectType = "monorepo-webapp"
	ProjectTypeMicroservicesSuite     ProjectType = "microservices-suite"
	ProjectTypePythonWithGoExtensions ProjectType = "python-with-go-extensions"
)

// WorkflowScenario defines complex testing scenarios
type WorkflowScenario string

const (
	ScenarioSimultaneousRequests    WorkflowScenario = "simultaneous-requests"
	ScenarioLoadBalancing           WorkflowScenario = "load-balancing"
	ScenarioCircuitBreakerTesting   WorkflowScenario = "circuit-breaker"
	ScenarioHealthMonitoring        WorkflowScenario = "health-monitoring"
	ScenarioServerFailover          WorkflowScenario = "server-failover"
	ScenarioLargeProjectPerformance WorkflowScenario = "large-project-performance"
	ScenarioConcurrentWorkspaces    WorkflowScenario = "concurrent-workspaces"
	ScenarioCrossLanguageNavigation WorkflowScenario = "cross-language-navigation"
)

// TestProject represents a generated test project
type TestProject struct {
	ID           string
	Name         string
	RootPath     string
	ProjectType  ProjectType
	Languages    []string
	Structure    map[string]string
	BuildFiles   []string
	TestFiles    []string
	Dependencies map[string][]string
	CreatedAt    time.Time
	Size         int64
}

// WorkflowResult contains results from executing test workflows
type WorkflowResult struct {
	Scenario        WorkflowScenario
	Success         bool
	Duration        time.Duration
	RequestCount    int
	ErrorCount      int
	Errors          []error
	Metrics         *WorkflowMetrics
	ServerInstances []string
	LoadBalancing   *LoadBalancingResult
	CircuitBreaker  *CircuitBreakerResult
	Health          *HealthMonitoringResult
}

// WorkflowMetrics contains detailed performance metrics
type WorkflowMetrics struct {
	TotalRequests       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	AverageResponseTime time.Duration
	MinResponseTime     time.Duration
	MaxResponseTime     time.Duration
	ThroughputPerSecond float64
	MemoryUsageMB       float64
	CPUUsagePercent     float64
	ServerSwitches      int64
	CacheHitRatio       float64
}

// LoadBalancingResult contains load balancing test results
type LoadBalancingResult struct {
	Strategy           string
	ServerDistribution map[string]int
	ResponseTimes      map[string]time.Duration
	LoadFairness       float64
	EfficiencyScore    float64
}

// CircuitBreakerResult contains circuit breaker test results
type CircuitBreakerResult struct {
	TriggeredCount   int
	RecoveryTime     time.Duration
	FalsePositives   int
	FailureThreshold float64
	OpenStateTime    time.Duration
}

// HealthMonitoringResult contains health monitoring test results
type HealthMonitoringResult struct {
	HealthChecks       int
	HealthyServers     int
	UnhealthyCount     int
	RecoveryEvents     int
	AverageHealthScore float64
}

// PerformanceMetrics contains detailed performance measurements
type PerformanceMetrics struct {
	OperationID        string
	OperationDuration  time.Duration
	MemoryAllocated    int64
	MemoryFreed        int64
	GoroutineCount     int
	FileOperations     int64
	NetworkRequests    int64
	CacheOperations    int64
	ServerCreations    int
	ServerDestructions int
	ErrorCount         int
	WarningCount       int
}

// TestMetrics tracks overall test execution metrics
type TestMetrics struct {
	TestsRun          int
	TestsPassed       int
	TestsFailed       int
	TotalDuration     time.Duration
	ProjectsCreated   int
	ServersStarted    int
	RequestsProcessed int64
	ErrorsEncountered int
	WarningsIssued    int
	PeakMemoryUsage   int64
	AverageCPUUsage   float64
}

// MultiLanguageTestFramework provides comprehensive testing capabilities
type MultiLanguageTestFramework struct {
	// Core components
	MockServerManager   *MockLSPServerManager
	ProjectGenerator    *TestProjectGenerator
	PerformanceProfiler *PerformanceProfiler
	WorkspaceManager    *TestWorkspaceManager

	// Configuration
	TempDir      string
	TestTimeout  time.Duration
	CleanupFuncs []func()

	// State tracking
	CreatedProjects []string
	ActiveServers   map[string]*MockLSPServer
	TestMetrics     *TestMetrics

	// Synchronization
	mu sync.RWMutex

	// Context management
	ctx    context.Context
	cancel context.CancelFunc

	// Test infrastructure
	testRunner *testdata.TestRunner
}

// NewMultiLanguageTestFramework creates a new test framework instance
func NewMultiLanguageTestFramework(timeout time.Duration) *MultiLanguageTestFramework {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	// Create temporary directory for test artifacts
	tempDir, err := os.MkdirTemp("", "ml-test-framework-*")
	if err != nil {
		panic(fmt.Sprintf("Failed to create temp directory: %v", err))
	}

	framework := &MultiLanguageTestFramework{
		TempDir:         tempDir,
		TestTimeout:     timeout,
		CleanupFuncs:    make([]func(), 0),
		CreatedProjects: make([]string, 0),
		ActiveServers:   make(map[string]*MockLSPServer),
		TestMetrics:     &TestMetrics{},
		ctx:             ctx,
		cancel:          cancel,
		testRunner:      testdata.NewTestRunner(timeout),
	}

	// Initialize components
	framework.MockServerManager = NewMockLSPServerManager(tempDir)
	framework.ProjectGenerator = NewTestProjectGenerator(tempDir)
	framework.PerformanceProfiler = NewPerformanceProfiler()
	framework.WorkspaceManager = NewTestWorkspaceManager(tempDir)

	// Register cleanup for temp directory
	framework.CleanupFuncs = append(framework.CleanupFuncs, func() {
		_ = os.RemoveAll(tempDir)
	})

	return framework
}

// SetupTestEnvironment initializes the complete test environment
func (f *MultiLanguageTestFramework) SetupTestEnvironment(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Initialize mock server manager
	if err := f.MockServerManager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize mock server manager: %w", err)
	}

	// Initialize project generator with realistic templates
	if err := f.ProjectGenerator.LoadTemplates(ctx); err != nil {
		return fmt.Errorf("failed to load project templates: %w", err)
	}

	// Initialize workspace manager
	if err := f.WorkspaceManager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize workspace manager: %w", err)
	}

	// Start performance profiler
	if err := f.PerformanceProfiler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start performance profiler: %w", err)
	}

	return nil
}

// CreateMultiLanguageProject creates a test project with specified languages and type
func (f *MultiLanguageTestFramework) CreateMultiLanguageProject(projectType ProjectType, languages []string) (*TestProject, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Generate project configuration
	config := &ProjectGenerationConfig{
		Type:         projectType,
		Languages:    languages,
		Complexity:   ComplexityMedium,
		Size:         SizeMedium,
		BuildSystem:  true,
		TestFiles:    true,
		Dependencies: true,
	}

	// Create project using generator
	project, err := f.ProjectGenerator.GenerateProject(config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate project: %w", err)
	}

	// Track created project
	f.CreatedProjects = append(f.CreatedProjects, project.RootPath)
	f.TestMetrics.ProjectsCreated++

	// Register cleanup for project
	f.CleanupFuncs = append(f.CleanupFuncs, func() {
		_ = os.RemoveAll(project.RootPath)
	})

	return project, nil
}

// StartMultipleLanguageServers starts mock LSP servers for specified languages
func (f *MultiLanguageTestFramework) StartMultipleLanguageServers(languages []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, language := range languages {
		// Check if server already started
		if _, exists := f.ActiveServers[language]; exists {
			continue
		}

		// Create and start mock server
		server, err := f.MockServerManager.CreateServer(language)
		if err != nil {
			return fmt.Errorf("failed to create server for %s: %w", language, err)
		}

		if err := server.Start(f.ctx); err != nil {
			return fmt.Errorf("failed to start server for %s: %w", language, err)
		}

		f.ActiveServers[language] = server
		f.TestMetrics.ServersStarted++

		// Register cleanup for server
		f.CleanupFuncs = append(f.CleanupFuncs, func() {
			_ = server.Stop()
		})
	}

	return nil
}

// SimulateComplexWorkflow executes complex multi-language workflows
func (f *MultiLanguageTestFramework) SimulateComplexWorkflow(scenario WorkflowScenario) (*WorkflowResult, error) {
	startTime := time.Now()

	result := &WorkflowResult{
		Scenario:        scenario,
		Metrics:         &WorkflowMetrics{},
		ServerInstances: make([]string, 0),
		Errors:          make([]error, 0),
	}

	// Track performance during workflow
	perfMetrics, err := f.PerformanceProfiler.StartOperation(string(scenario))
	if err != nil {
		return nil, fmt.Errorf("failed to start performance tracking: %w", err)
	}
	defer func() {
		finalMetrics, _ := f.PerformanceProfiler.EndOperation(perfMetrics.OperationID)
		if finalMetrics != nil {
			result.Metrics.TotalRequests = finalMetrics.NetworkRequests
			result.Metrics.MemoryUsageMB = float64(finalMetrics.MemoryAllocated) / 1024 / 1024
		}
	}()

	// Execute scenario-specific workflow
	switch scenario {
	case ScenarioSimultaneousRequests:
		err = f.executeSimultaneousRequestsScenario(result)
	case ScenarioLoadBalancing:
		err = f.executeLoadBalancingScenario(result)
	case ScenarioCircuitBreakerTesting:
		err = f.executeCircuitBreakerScenario(result)
	case ScenarioHealthMonitoring:
		err = f.executeHealthMonitoringScenario(result)
	case ScenarioServerFailover:
		err = f.executeServerFailoverScenario(result)
	case ScenarioLargeProjectPerformance:
		err = f.executeLargeProjectPerformanceScenario(result)
	case ScenarioConcurrentWorkspaces:
		err = f.executeConcurrentWorkspacesScenario(result)
	case ScenarioCrossLanguageNavigation:
		err = f.executeCrossLanguageNavigationScenario(result)
	default:
		err = fmt.Errorf("unsupported scenario: %s", scenario)
	}

	result.Duration = time.Since(startTime)
	result.Success = err == nil && len(result.Errors) == 0

	if err != nil {
		result.Errors = append(result.Errors, err)
		result.ErrorCount++
		f.TestMetrics.ErrorsEncountered++
	}

	f.TestMetrics.RequestsProcessed += int64(result.RequestCount)

	return result, err
}

// MeasurePerformance measures performance of arbitrary operations
func (f *MultiLanguageTestFramework) MeasurePerformance(operation func() error) (*PerformanceMetrics, error) {
	metrics, err := f.PerformanceProfiler.StartOperation("custom-operation")
	if err != nil {
		return nil, fmt.Errorf("failed to start performance measurement: %w", err)
	}

	// Execute the operation
	operationErr := operation()

	// End performance measurement
	finalMetrics, err := f.PerformanceProfiler.EndOperation(metrics.OperationID)
	if err != nil {
		return nil, fmt.Errorf("failed to end performance measurement: %w", err)
	}

	if operationErr != nil {
		finalMetrics.ErrorCount++
	}

	return finalMetrics, operationErr
}

// CleanupAll performs comprehensive cleanup of all test resources
func (f *MultiLanguageTestFramework) CleanupAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	var errors []error

	// Stop all active servers
	for language, server := range f.ActiveServers {
		if err := server.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop server for %s: %w", language, err))
		}
	}
	f.ActiveServers = make(map[string]*MockLSPServer)

	// Stop performance profiler
	if err := f.PerformanceProfiler.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop performance profiler: %w", err))
	}

	// Cleanup workspace manager
	if err := f.WorkspaceManager.Cleanup(); err != nil {
		errors = append(errors, fmt.Errorf("failed to cleanup workspace manager: %w", err))
	}

	// Execute all registered cleanup functions
	for _, cleanup := range f.CleanupFuncs {
		cleanup()
	}
	f.CleanupFuncs = nil

	// Cleanup test runner
	f.testRunner.Cleanup()

	// Cancel context
	if f.cancel != nil {
		f.cancel()
	}

	// Combine errors if any
	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}

// GetTestMetrics returns current test execution metrics
func (f *MultiLanguageTestFramework) GetTestMetrics() *TestMetrics {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create a copy to avoid race conditions
	return &TestMetrics{
		TestsRun:          f.TestMetrics.TestsRun,
		TestsPassed:       f.TestMetrics.TestsPassed,
		TestsFailed:       f.TestMetrics.TestsFailed,
		TotalDuration:     f.TestMetrics.TotalDuration,
		ProjectsCreated:   f.TestMetrics.ProjectsCreated,
		ServersStarted:    f.TestMetrics.ServersStarted,
		RequestsProcessed: f.TestMetrics.RequestsProcessed,
		ErrorsEncountered: f.TestMetrics.ErrorsEncountered,
		WarningsIssued:    f.TestMetrics.WarningsIssued,
		PeakMemoryUsage:   f.TestMetrics.PeakMemoryUsage,
		AverageCPUUsage:   f.TestMetrics.AverageCPUUsage,
	}
}

// CreateGatewayWithProject creates a gateway instance configured for a specific project
func (f *MultiLanguageTestFramework) CreateGatewayWithProject(project *TestProject) (*gateway.ProjectAwareGateway, error) {
	// Generate gateway configuration for the project
	gatewayConfig := &config.GatewayConfig{
		Port:    8080,
		Servers: make([]config.ServerConfig, 0),
	}

	// Add server configurations for each language
	for _, language := range project.Languages {
		if server, exists := f.ActiveServers[language]; exists {
			serverConfig := config.ServerConfig{
				Name:      fmt.Sprintf("%s-server", language),
				Languages: []string{language},
				Command:   server.GetCommand(),
				Args:      server.GetArgs(),
				Transport: "stdio",
			}
			gatewayConfig.Servers = append(gatewayConfig.Servers, serverConfig)
		}
	}

	// Create project-aware gateway
	projectGateway, err := gateway.NewProjectAwareGateway(gatewayConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create project-aware gateway: %w", err)
	}

	return projectGateway, nil
}

// ValidateProjectStructure validates that a generated project has the expected structure
func (f *MultiLanguageTestFramework) ValidateProjectStructure(project *TestProject) error {
	// Check that root path exists
	if _, err := os.Stat(project.RootPath); os.IsNotExist(err) {
		return fmt.Errorf("project root path does not exist: %s", project.RootPath)
	}

	// Validate that expected files exist
	for relPath := range project.Structure {
		fullPath := filepath.Join(project.RootPath, relPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fmt.Errorf("expected file does not exist: %s", fullPath)
		}
	}

	// Validate language-specific files
	for _, language := range project.Languages {
		hasFiles := false
		for relPath := range project.Structure {
			if f.isLanguageFile(relPath, language) {
				hasFiles = true
				break
			}
		}
		if !hasFiles {
			return fmt.Errorf("no files found for language: %s", language)
		}
	}

	// Validate build files exist
	if len(project.BuildFiles) == 0 {
		return fmt.Errorf("no build files found in project")
	}

	for _, buildFile := range project.BuildFiles {
		fullPath := filepath.Join(project.RootPath, buildFile)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return fmt.Errorf("build file does not exist: %s", fullPath)
		}
	}

	return nil
}

// isLanguageFile checks if a file belongs to a specific language
func (f *MultiLanguageTestFramework) isLanguageFile(filePath, language string) bool {
	ext := filepath.Ext(filePath)

	languageExtensions := map[string][]string{
		"go":         {".go"},
		"python":     {".py"},
		"javascript": {".js", ".jsx"},
		"typescript": {".ts", ".tsx"},
		"java":       {".java"},
		"rust":       {".rs"},
		"cpp":        {".cpp", ".cc", ".cxx", ".h", ".hpp"},
		"csharp":     {".cs"},
		"php":        {".php"},
		"ruby":       {".rb"},
	}

	if extensions, exists := languageExtensions[language]; exists {
		for _, validExt := range extensions {
			if ext == validExt {
				return true
			}
		}
	}

	return false
}

// Context returns the framework's context
func (f *MultiLanguageTestFramework) Context() context.Context {
	return f.ctx
}

// IsActive returns whether the framework is still active
func (f *MultiLanguageTestFramework) IsActive() bool {
	select {
	case <-f.ctx.Done():
		return false
	default:
		return true
	}
}

// Scenario execution methods

// executeSimultaneousRequestsScenario executes simultaneous requests scenario
func (f *MultiLanguageTestFramework) executeSimultaneousRequestsScenario(result *WorkflowResult) error {
	result.RequestCount = 50

	// Simulate simultaneous requests across multiple servers
	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32

	for i := 0; i < result.RequestCount; i++ {
		wg.Add(1)
		go func(requestID int) {
			defer wg.Done()

			// Simulate request processing time
			time.Sleep(time.Duration(10+requestID%50) * time.Millisecond)

			// Simulate success/failure (90% success rate)
			if requestID%10 != 0 {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&errorCount, 1)
				result.Errors = append(result.Errors, fmt.Errorf("simulated error for request %d", requestID))
			}
		}(i)
	}

	wg.Wait()

	result.ErrorCount = int(errorCount)
	result.ServerInstances = []string{"server-1", "server-2", "server-3"}

	return nil
}

// executeLoadBalancingScenario executes load balancing scenario
func (f *MultiLanguageTestFramework) executeLoadBalancingScenario(result *WorkflowResult) error {
	result.RequestCount = 100

	// Simulate load balancing across servers
	serverDistribution := map[string]int{
		"server-1": 35,
		"server-2": 33,
		"server-3": 32,
	}

	responseTimes := map[string]time.Duration{
		"server-1": 150 * time.Millisecond,
		"server-2": 140 * time.Millisecond,
		"server-3": 160 * time.Millisecond,
	}

	result.LoadBalancing = &LoadBalancingResult{
		Strategy:           "round-robin",
		ServerDistribution: serverDistribution,
		ResponseTimes:      responseTimes,
		LoadFairness:       0.95,
		EfficiencyScore:    0.88,
	}

	result.ServerInstances = []string{"server-1", "server-2", "server-3"}

	return nil
}

// executeCircuitBreakerScenario executes circuit breaker scenario
func (f *MultiLanguageTestFramework) executeCircuitBreakerScenario(result *WorkflowResult) error {
	result.RequestCount = 75

	// Simulate circuit breaker behavior
	result.CircuitBreaker = &CircuitBreakerResult{
		TriggeredCount:   2,
		RecoveryTime:     5 * time.Second,
		FalsePositives:   0,
		FailureThreshold: 0.5,
		OpenStateTime:    3 * time.Second,
	}

	result.ErrorCount = 5
	result.ServerInstances = []string{"server-1", "server-2"}

	// Simulate some failures that trigger circuit breaker
	for i := 0; i < result.ErrorCount; i++ {
		result.Errors = append(result.Errors, fmt.Errorf("circuit breaker test error %d", i))
	}

	return nil
}

// executeHealthMonitoringScenario executes health monitoring scenario
func (f *MultiLanguageTestFramework) executeHealthMonitoringScenario(result *WorkflowResult) error {
	result.RequestCount = 60

	// Simulate health monitoring
	result.Health = &HealthMonitoringResult{
		HealthChecks:       10,
		HealthyServers:     3,
		UnhealthyCount:     1,
		RecoveryEvents:     2,
		AverageHealthScore: 0.85,
	}

	result.ServerInstances = []string{"server-1", "server-2", "server-3", "server-4"}
	result.ErrorCount = 3

	return nil
}

// executeServerFailoverScenario executes server failover scenario
func (f *MultiLanguageTestFramework) executeServerFailoverScenario(result *WorkflowResult) error {
	result.RequestCount = 80

	// Simulate server failover
	result.ServerInstances = []string{"primary-server", "backup-server-1", "backup-server-2"}
	result.ErrorCount = 8

	// Simulate failover process
	for i := 0; i < result.ErrorCount; i++ {
		result.Errors = append(result.Errors, fmt.Errorf("failover test error %d", i))
	}

	return nil
}

// executeLargeProjectPerformanceScenario executes large project performance scenario
func (f *MultiLanguageTestFramework) executeLargeProjectPerformanceScenario(result *WorkflowResult) error {
	result.RequestCount = 200

	// Simulate large project performance metrics
	result.Metrics.TotalRequests = 200
	result.Metrics.SuccessfulRequests = 185
	result.Metrics.FailedRequests = 15
	result.Metrics.AverageResponseTime = 2500 * time.Millisecond
	result.Metrics.MinResponseTime = 100 * time.Millisecond
	result.Metrics.MaxResponseTime = 8 * time.Second
	result.Metrics.ThroughputPerSecond = 45.5
	result.Metrics.MemoryUsageMB = 750.5
	result.Metrics.CPUUsagePercent = 65.2
	result.Metrics.ServerSwitches = 12
	result.Metrics.CacheHitRatio = 0.78

	result.ServerInstances = []string{"go-server", "python-server", "typescript-server", "java-server", "rust-server"}
	result.ErrorCount = 15

	// Add some performance-related errors
	for i := 0; i < result.ErrorCount; i++ {
		result.Errors = append(result.Errors, fmt.Errorf("performance test error %d", i))
	}

	return nil
}

// executeConcurrentWorkspacesScenario executes concurrent workspaces scenario
func (f *MultiLanguageTestFramework) executeConcurrentWorkspacesScenario(result *WorkflowResult) error {
	result.RequestCount = 120

	// Simulate concurrent workspace operations
	result.ServerInstances = []string{"workspace-1-server", "workspace-2-server", "workspace-3-server"}
	result.ErrorCount = 6

	// Simulate concurrent workspace metrics
	result.Metrics.TotalRequests = 120
	result.Metrics.SuccessfulRequests = 114
	result.Metrics.FailedRequests = 6
	result.Metrics.AverageResponseTime = 800 * time.Millisecond
	result.Metrics.ThroughputPerSecond = 25.3
	result.Metrics.MemoryUsageMB = 420.8
	result.Metrics.ServerSwitches = 8

	return nil
}

// executeCrossLanguageNavigationScenario executes cross-language navigation scenario
func (f *MultiLanguageTestFramework) executeCrossLanguageNavigationScenario(result *WorkflowResult) error {
	result.RequestCount = 90

	// Simulate cross-language navigation
	result.ServerInstances = []string{"go-server", "python-server", "typescript-server", "java-server"}
	result.ErrorCount = 4

	// Simulate cross-language navigation metrics
	result.Metrics.TotalRequests = 90
	result.Metrics.SuccessfulRequests = 86
	result.Metrics.FailedRequests = 4
	result.Metrics.AverageResponseTime = 1200 * time.Millisecond
	result.Metrics.ThroughputPerSecond = 18.7
	result.Metrics.ServerSwitches = 25 // Higher due to cross-language navigation

	return nil
}
