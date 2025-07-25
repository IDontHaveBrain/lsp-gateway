package e2e_test

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// TestCategory represents different categories of E2E tests
type TestCategory string

const (
	CategorySetup        TestCategory = "setup"
	CategoryHTTPProtocol TestCategory = "http_protocol"
	CategoryMCPProtocol  TestCategory = "mcp_protocol"
	CategoryMultiLang    TestCategory = "multi_language"
	CategoryPerformance  TestCategory = "performance"
	CategoryIntegration  TestCategory = "integration"
	CategoryWorkflow     TestCategory = "workflow"
	CategoryCleanup      TestCategory = "cleanup"
)

// TestPriority defines execution priority levels
type TestPriority int

const (
	PriorityLow    TestPriority = 1
	PriorityMedium TestPriority = 5
	PriorityHigh   TestPriority = 10
	PriorityCritical TestPriority = 15
)

// ExecutionMode defines how tests can be executed
type ExecutionMode string

const (
	ModeSequential ExecutionMode = "sequential"
	ModeParallel   ExecutionMode = "parallel"
	ModeConditional ExecutionMode = "conditional"
)

// ResourceRequirement defines what resources a test needs
type ResourceRequirement struct {
	Type         ResourceType
	Count        int
	MinMemoryMB  int64
	MinCPUCores  int
	NetworkPorts []int
	Exclusive    bool // Whether resource needs exclusive access
}

// TestDiscoveryMetadata contains metadata about discovered tests
type TestDiscoveryMetadata struct {
	Name           string
	Category       TestCategory
	Priority       TestPriority
	ExecutionMode  ExecutionMode
	Dependencies   []string
	Resources      []ResourceRequirement
	Tags           []string
	Timeout        time.Duration
	RetryCount     int
	Description    string
	TestFunction   interface{}
	SkipConditions []string
	
	// Discovery information
	PackageName    string
	FileName       string
	LineNumber     int
	FunctionName   string
	DiscoveredAt   time.Time
}

// TestDependencyNode represents a node in the test dependency graph
type TestDependencyNode struct {
	Test         *TestDiscoveryMetadata
	Dependencies []*TestDependencyNode
	Dependents   []*TestDependencyNode
	Level        int // Execution level based on dependencies
	Visited      bool
	InProgress   bool
	Completed    bool
}

// TestDiscoveryEngine discovers and analyzes E2E tests
type TestDiscoveryEngine struct {
	// Core components
	discoveredTests    map[string]*TestDiscoveryMetadata
	dependencyGraph    map[string]*TestDependencyNode
	categoryTests      map[TestCategory][]*TestDiscoveryMetadata
	priorityTests      map[TestPriority][]*TestDiscoveryMetadata
	
	// Discovery configuration
	testPatterns       []*regexp.Regexp
	packagePaths       []string
	excludePatterns    []*regexp.Regexp
	maxDiscoveryDepth  int
	
	// Execution planning
	executionLevels    [][]string // Tests grouped by execution level
	parallelGroups     [][]string // Tests that can run in parallel
	sequentialChains   [][]string // Tests that must run sequentially
	
	// State management
	mu                 sync.RWMutex
	discoveryComplete  bool
	lastDiscoveryTime  time.Time
	
	// Metrics
	discoveryMetrics   *TestDiscoveryMetrics
}

// TestDiscoveryMetrics tracks discovery and analysis metrics
type TestDiscoveryMetrics struct {
	TotalTestsDiscovered     int
	TestsByCategory          map[TestCategory]int
	TestsByPriority          map[TestPriority]int
	TestsByExecutionMode     map[ExecutionMode]int
	DependencyComplexity     float64
	DiscoveryDurationMs      int64
	AnalysisDurationMs       int64
	CyclicDependencies       []string
	OrphanedTests           []string
	ResourceConflicts       []string
}

// TestFilter defines criteria for filtering discovered tests
type TestFilter struct {
	Categories         []TestCategory
	Priorities         []TestPriority
	Tags               []string
	ExcludeTags        []string
	MinPriority        TestPriority
	ExecutionModes     []ExecutionMode
	MaxDuration        time.Duration
	RequiredResources  []ResourceType
	SkipFailed         bool
	IncludePatterns    []*regexp.Regexp
	ExcludePatterns    []*regexp.Regexp
}

// NewTestDiscoveryEngine creates a new test discovery engine
func NewTestDiscoveryEngine() *TestDiscoveryEngine {
	return &TestDiscoveryEngine{
		discoveredTests:   make(map[string]*TestDiscoveryMetadata),
		dependencyGraph:   make(map[string]*TestDependencyNode),
		categoryTests:     make(map[TestCategory][]*TestDiscoveryMetadata),
		priorityTests:     make(map[TestPriority][]*TestDiscoveryMetadata),
		testPatterns:      []*regexp.Regexp{
			regexp.MustCompile(`^Test.*E2E.*`),
			regexp.MustCompile(`^.*E2ETest$`),
			regexp.MustCompile(`^E2E.*Test.*`),
			regexp.MustCompile(`^.*E2EScenario.*`),
		},
		excludePatterns: []*regexp.Regexp{
			regexp.MustCompile(`.*_test\.go$`), // Exclude regular unit tests
			regexp.MustCompile(`.*Helper.*`),   // Exclude helper functions
			regexp.MustCompile(`.*Mock.*`),     // Exclude mock functions
		},
		maxDiscoveryDepth: 5,
		discoveryMetrics:  &TestDiscoveryMetrics{
			TestsByCategory:     make(map[TestCategory]int),
			TestsByPriority:     make(map[TestPriority]int),
			TestsByExecutionMode: make(map[ExecutionMode]int),
			CyclicDependencies:  make([]string, 0),
			OrphanedTests:       make([]string, 0),
			ResourceConflicts:   make([]string, 0),
		},
	}
}

// DiscoverTests discovers all E2E tests in the specified packages
func (engine *TestDiscoveryEngine) DiscoverTests(ctx context.Context, packagePaths ...string) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	
	startTime := time.Now()
	defer func() {
		engine.discoveryMetrics.DiscoveryDurationMs = time.Since(startTime).Milliseconds()
		engine.lastDiscoveryTime = time.Now()
		engine.discoveryComplete = true
	}()
	
	engine.packagePaths = packagePaths
	if len(packagePaths) == 0 {
		engine.packagePaths = []string{"."} // Default to current package
	}
	
	// Discover test functions through reflection and analysis
	for _, packagePath := range engine.packagePaths {
		if err := engine.discoverPackageTests(ctx, packagePath); err != nil {
			return fmt.Errorf("failed to discover tests in package %s: %w", packagePath, err)
		}
	}
	
	// Analyze dependencies and build execution graph
	analysisStart := time.Now()
	if err := engine.analyzeDependencies(ctx); err != nil {
		return fmt.Errorf("failed to analyze test dependencies: %w", err)
	}
	engine.discoveryMetrics.AnalysisDurationMs = time.Since(analysisStart).Milliseconds()
	
	// Build execution plan
	if err := engine.buildExecutionPlan(ctx); err != nil {
		return fmt.Errorf("failed to build execution plan: %w", err)
	}
	
	// Update metrics
	engine.updateDiscoveryMetrics()
	
	return nil
}

// discoverPackageTests discovers tests in a specific package
func (engine *TestDiscoveryEngine) discoverPackageTests(ctx context.Context, packagePath string) error {
	// In a real implementation, this would use Go's reflection capabilities
	// or static analysis to discover test functions. For this implementation,
	// we'll register the known test functions from the E2E test suite.
	
	// Register core E2E test scenarios
	engine.registerCoreE2ETests()
	
	// Register MCP E2E test scenarios
	engine.registerMCPE2ETests()
	
	// Register workflow test scenarios
	engine.registerWorkflowTests()
	
	return nil
}

// registerCoreE2ETests registers the core E2E test functions
func (engine *TestDiscoveryEngine) registerCoreE2ETests() {
	coreTests := []*TestDiscoveryMetadata{
		{
			Name:          "gateway_startup",
			Category:      CategorySetup,
			Priority:      PriorityCritical,
			ExecutionMode: ModeSequential,
			Dependencies:  []string{},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 1, MinMemoryMB: 256},
				{Type: ResourceTypeConfig, Count: 1},
			},
			Tags:        []string{"startup", "gateway", "critical"},
			Timeout:     5 * time.Minute,
			RetryCount:  2,
			Description: "Tests LSP Gateway startup and initialization",
			PackageName: "e2e_test",
			FileName:    "e2e_test_suite.go",
			FunctionName: "TestGatewayStartup",
		},
		{
			Name:          "mcp_startup",
			Category:      CategorySetup,
			Priority:      PriorityCritical,
			ExecutionMode: ModeSequential,
			Dependencies:  []string{"gateway_startup"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 1, MinMemoryMB: 128},
				{Type: ResourceTypeConfig, Count: 1},
			},
			Tags:        []string{"startup", "mcp", "critical"},
			Timeout:     5 * time.Minute,
			RetryCount:  2,
			Description: "Tests MCP server startup and initialization",
			PackageName: "e2e_test",
			FileName:    "e2e_test_suite.go",
			FunctionName: "TestMCPStartup",
		},
		{
			Name:          "lsp_methods",
			Category:      CategoryHTTPProtocol,
			Priority:      PriorityHigh,
			ExecutionMode: ModeParallel,
			Dependencies:  []string{"gateway_startup"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 1, MinMemoryMB: 256},
				{Type: ResourceTypeClient, Count: 1},
			},
			Tags:        []string{"lsp", "protocol", "methods"},
			Timeout:     10 * time.Minute,
			RetryCount:  1,
			Description: "Tests LSP method implementations and compliance",
			PackageName: "e2e_test",
			FileName:    "e2e_test_suite.go",
			FunctionName: "TestLSPMethods",
		},
		{
			Name:          "error_handling",
			Category:      CategoryHTTPProtocol,
			Priority:      PriorityHigh,
			ExecutionMode: ModeParallel,
			Dependencies:  []string{"gateway_startup"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 1, MinMemoryMB: 128},
				{Type: ResourceTypeClient, Count: 1},
			},
			Tags:        []string{"error", "handling", "protocol"},
			Timeout:     10 * time.Minute,
			RetryCount:  1,
			Description: "Tests error handling and edge cases",
			PackageName: "e2e_test",
			FileName:    "e2e_test_suite.go",
			FunctionName: "TestErrorHandling",
		},
		{
			Name:          "concurrent_requests",
			Category:      CategoryPerformance,
			Priority:      PriorityMedium,
			ExecutionMode: ModeSequential,
			Dependencies:  []string{"lsp_methods"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 1, MinMemoryMB: 512, MinCPUCores: 2},
				{Type: ResourceTypeClient, Count: 3},
			},
			Tags:        []string{"concurrent", "performance", "load"},
			Timeout:     15 * time.Minute,
			RetryCount:  1,
			Description: "Tests concurrent request handling and performance",
			PackageName: "e2e_test",
			FileName:    "e2e_test_suite.go",
			FunctionName: "TestConcurrentRequests",
		},
	}
	
	for _, test := range coreTests {
		test.DiscoveredAt = time.Now()
		engine.discoveredTests[test.Name] = test
	}
}

// registerMCPE2ETests registers MCP-specific E2E test functions
func (engine *TestDiscoveryEngine) registerMCPE2ETests() {
	mcpTests := []*TestDiscoveryMetadata{
		{
			Name:          "tool_availability",
			Category:      CategoryMCPProtocol,
			Priority:      PriorityHigh,
			ExecutionMode: ModeParallel,
			Dependencies:  []string{"mcp_startup"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 1, MinMemoryMB: 256},
			},
			Tags:        []string{"mcp", "tools", "availability"},
			Timeout:     10 * time.Minute,
			RetryCount:  1,
			Description: "Tests MCP tool availability and registration",
			PackageName: "e2e_test",
			FileName:    "mcp_e2e_test_framework.go",
			FunctionName: "TestToolAvailability",
		},
		{
			Name:          "ai_integration",
			Category:      CategoryMCPProtocol,
			Priority:      PriorityHigh,
			ExecutionMode: ModeSequential,
			Dependencies:  []string{"mcp_startup", "tool_availability"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 1, MinMemoryMB: 512},
				{Type: ResourceTypeClient, Count: 1},
			},
			Tags:        []string{"mcp", "ai", "integration"},
			Timeout:     15 * time.Minute,
			RetryCount:  1,
			Description: "Tests AI assistant integration via MCP",
			PackageName: "e2e_test",
			FileName:    "mcp_e2e_test_framework.go",
			FunctionName: "TestAIIntegration",
		},
		{
			Name:          "cross_protocol",
			Category:      CategoryIntegration,
			Priority:      PriorityMedium,
			ExecutionMode: ModeSequential,
			Dependencies:  []string{"gateway_startup", "mcp_startup"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 2, MinMemoryMB: 384},
			},
			Tags:        []string{"cross-protocol", "integration"},
			Timeout:     20 * time.Minute,
			RetryCount:  1,
			Description: "Tests cross-protocol communication between HTTP and MCP",
			PackageName: "e2e_test",
			FileName:    "mcp_e2e_test_framework.go",
			FunctionName: "TestCrossProtocol",
		},
	}
	
	for _, test := range mcpTests {
		test.DiscoveredAt = time.Now()
		engine.discoveredTests[test.Name] = test
	}
}

// registerWorkflowTests registers workflow and integration test functions
func (engine *TestDiscoveryEngine) registerWorkflowTests() {
	workflowTests := []*TestDiscoveryMetadata{
		{
			Name:          "developer_workflows",
			Category:      CategoryWorkflow,
			Priority:      PriorityHigh,
			ExecutionMode: ModeSequential,
			Dependencies:  []string{"gateway_startup", "mcp_startup"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 2, MinMemoryMB: 512},
				{Type: ResourceTypeProject, Count: 1},
				{Type: ResourceTypeWorkspace, Count: 1},
			},
			Tags:        []string{"workflow", "developer", "real-world"},
			Timeout:     25 * time.Minute,
			RetryCount:  1,
			Description: "Tests real-world developer workflows",
			PackageName: "e2e_test",
			FileName:    "e2e_test_suite.go",
			FunctionName: "TestDeveloperWorkflows",
		},
		{
			Name:          "multi_language",
			Category:      CategoryMultiLang,
			Priority:      PriorityHigh,
			ExecutionMode: ModeParallel,
			Dependencies:  []string{"developer_workflows"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 4, MinMemoryMB: 1024, MinCPUCores: 2},
				{Type: ResourceTypeProject, Count: 2},
			},
			Tags:        []string{"multi-language", "polyglot", "integration"},
			Timeout:     30 * time.Minute,
			RetryCount:  1,
			Description: "Tests multi-language project support",
			PackageName: "e2e_test",
			FileName:    "multi_language_e2e_test.go",
			FunctionName: "TestMultiLanguage",
		},
		{
			Name:          "project_detection",
			Category:      CategoryWorkflow,
			Priority:      PriorityMedium,
			ExecutionMode: ModeParallel,
			Dependencies:  []string{"developer_workflows"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeProject, Count: 3},
				{Type: ResourceTypeWorkspace, Count: 1},
			},
			Tags:        []string{"project", "detection", "analysis"},
			Timeout:     15 * time.Minute,
			RetryCount:  1,
			Description: "Tests project structure detection and analysis",
			PackageName: "e2e_test",
			FileName:    "e2e_test_suite.go",
			FunctionName: "TestProjectDetection",
		},
		{
			Name:          "circuit_breaker",
			Category:      CategoryIntegration,
			Priority:      PriorityMedium,
			ExecutionMode: ModeSequential,
			Dependencies:  []string{"gateway_startup", "mcp_startup"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 3, MinMemoryMB: 384},
			},
			Tags:        []string{"circuit-breaker", "resilience", "failover"},
			Timeout:     20 * time.Minute,
			RetryCount:  2,
			Description: "Tests circuit breaker and failover mechanisms",
			PackageName: "e2e_test",
			FileName:    "circuit_breaker_e2e_test.go",
			FunctionName: "TestCircuitBreaker",
		},
		{
			Name:          "health_monitoring",
			Category:      CategoryIntegration,
			Priority:      PriorityMedium,
			ExecutionMode: ModeParallel,
			Dependencies:  []string{"gateway_startup"},
			Resources: []ResourceRequirement{
				{Type: ResourceTypeServer, Count: 2, MinMemoryMB: 256},
			},
			Tags:        []string{"health", "monitoring", "observability"},
			Timeout:     15 * time.Minute,
			RetryCount:  1,
			Description: "Tests health monitoring and observability features",
			PackageName: "e2e_test",
			FileName:    "e2e_test_suite.go",
			FunctionName: "TestHealthMonitoring",
		},
	}
	
	for _, test := range workflowTests {
		test.DiscoveredAt = time.Now()
		engine.discoveredTests[test.Name] = test
	}
}

// analyzeDependencies analyzes test dependencies and builds dependency graph
func (engine *TestDiscoveryEngine) analyzeDependencies(ctx context.Context) error {
	// Build dependency nodes
	for name, test := range engine.discoveredTests {
		node := &TestDependencyNode{
			Test:         test,
			Dependencies: make([]*TestDependencyNode, 0),
			Dependents:   make([]*TestDependencyNode, 0),
		}
		engine.dependencyGraph[name] = node
	}
	
	// Connect dependencies
	for name, node := range engine.dependencyGraph {
		for _, depName := range node.Test.Dependencies {
			if depNode, exists := engine.dependencyGraph[depName]; exists {
				node.Dependencies = append(node.Dependencies, depNode)
				depNode.Dependents = append(depNode.Dependents, node)
			} else {
				engine.discoveryMetrics.OrphanedTests = append(
					engine.discoveryMetrics.OrphanedTests,
					fmt.Sprintf("%s depends on missing test: %s", name, depName),
				)
			}
		}
	}
	
	// Check for cyclic dependencies
	if cycles := engine.detectCyclicDependencies(); len(cycles) > 0 {
		engine.discoveryMetrics.CyclicDependencies = cycles
		return fmt.Errorf("cyclic dependencies detected: %v", cycles)
	}
	
	// Calculate execution levels
	engine.calculateExecutionLevels()
	
	// Check for resource conflicts
	engine.detectResourceConflicts()
	
	return nil
}

// detectCyclicDependencies detects cyclic dependencies in the test graph
func (engine *TestDiscoveryEngine) detectCyclicDependencies() []string {
	cycles := make([]string, 0)
	visited := make(map[string]bool)
	inProgress := make(map[string]bool)
	
	var visit func(string, []string) []string
	visit = func(testName string, path []string) []string {
		if inProgress[testName] {
			// Found a cycle
			cycleStart := -1
			for i, name := range path {
				if name == testName {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := append(path[cycleStart:], testName)
				return []string{strings.Join(cycle, " -> ")}
			}
		}
		
		if visited[testName] {
			return nil
		}
		
		visited[testName] = true
		inProgress[testName] = true
		path = append(path, testName)
		
		var foundCycles []string
		if node, exists := engine.dependencyGraph[testName]; exists {
			for _, depNode := range node.Dependencies {
				if subCycles := visit(depNode.Test.Name, path); subCycles != nil {
					foundCycles = append(foundCycles, subCycles...)
				}
			}
		}
		
		inProgress[testName] = false
		return foundCycles
	}
	
	for testName := range engine.dependencyGraph {
		if !visited[testName] {
			if foundCycles := visit(testName, make([]string, 0)); foundCycles != nil {
				cycles = append(cycles, foundCycles...)
			}
		}
	}
	
	return cycles
}

// calculateExecutionLevels calculates execution levels for dependency-based scheduling
func (engine *TestDiscoveryEngine) calculateExecutionLevels() {
	// Reset levels
	for _, node := range engine.dependencyGraph {
		node.Level = 0
		node.Visited = false
	}
	
	// Calculate levels using topological sort
	var calculateLevel func(*TestDependencyNode) int
	calculateLevel = func(node *TestDependencyNode) int {
		if node.Visited {
			return node.Level
		}
		
		node.Visited = true
		maxDepLevel := -1
		
		for _, dep := range node.Dependencies {
			depLevel := calculateLevel(dep)
			if depLevel > maxDepLevel {
				maxDepLevel = depLevel
			}
		}
		
		node.Level = maxDepLevel + 1
		return node.Level
	}
	
	// Calculate levels for all nodes
	maxLevel := 0
	for _, node := range engine.dependencyGraph {
		level := calculateLevel(node)
		if level > maxLevel {
			maxLevel = level
		}
	}
	
	// Group tests by execution level
	engine.executionLevels = make([][]string, maxLevel+1)
	for i := range engine.executionLevels {
		engine.executionLevels[i] = make([]string, 0)
	}
	
	for name, node := range engine.dependencyGraph {
		engine.executionLevels[node.Level] = append(engine.executionLevels[node.Level], name)
	}
}

// detectResourceConflicts detects resource conflicts between tests
func (engine *TestDiscoveryEngine) detectResourceConflicts() {
	conflicts := make([]string, 0)
	
	// Group tests by execution level and check for resource conflicts
	for level, testNames := range engine.executionLevels {
		resourceUsage := make(map[ResourceType]int)
		exclusiveResources := make(map[ResourceType][]string)
		
		for _, testName := range testNames {
			if test, exists := engine.discoveredTests[testName]; exists {
				for _, resource := range test.Resources {
					resourceUsage[resource.Type] += resource.Count
					
					if resource.Exclusive {
						exclusiveResources[resource.Type] = append(
							exclusiveResources[resource.Type], testName)
					}
				}
			}
		}
		
		// Check for exclusive resource conflicts
		for resourceType, users := range exclusiveResources {
			if len(users) > 1 {
				conflicts = append(conflicts, fmt.Sprintf(
					"Level %d: Exclusive resource %s conflict between tests: %v",
					level, resourceType, users))
			}
		}
		
		// Check for resource capacity conflicts (simplified)
		for resourceType, usage := range resourceUsage {
			if usage > 10 { // Simplified capacity check
				conflicts = append(conflicts, fmt.Sprintf(
					"Level %d: High resource usage for %s: %d units",
					level, resourceType, usage))
			}
		}
	}
	
	engine.discoveryMetrics.ResourceConflicts = conflicts
}

// buildExecutionPlan builds optimized execution plan based on analysis
func (engine *TestDiscoveryEngine) buildExecutionPlan(ctx context.Context) error {
	// Build parallel groups within each execution level
	engine.parallelGroups = make([][]string, 0)
	engine.sequentialChains = make([][]string, 0)
	
	for _, levelTests := range engine.executionLevels {
		if len(levelTests) <= 1 {
			continue
		}
		
		// Group tests by execution mode
		parallelTests := make([]string, 0)
		sequentialTests := make([]string, 0)
		
		for _, testName := range levelTests {
			if test, exists := engine.discoveredTests[testName]; exists {
				switch test.ExecutionMode {
				case ModeParallel:
					parallelTests = append(parallelTests, testName)
				case ModeSequential:
					sequentialTests = append(sequentialTests, testName)
				case ModeConditional:
					// Analyze conditions to determine execution mode
					if len(test.Dependencies) <= 1 && !engine.hasResourceConflicts(testName) {
						parallelTests = append(parallelTests, testName)
					} else {
						sequentialTests = append(sequentialTests, testName)
					}
				}
			}
		}
		
		if len(parallelTests) > 1 {
			engine.parallelGroups = append(engine.parallelGroups, parallelTests)
		}
		
		if len(sequentialTests) > 0 {
			engine.sequentialChains = append(engine.sequentialChains, sequentialTests)
		}
	}
	
	return nil
}

// hasResourceConflicts checks if a test has resource conflicts with other tests
func (engine *TestDiscoveryEngine) hasResourceConflicts(testName string) bool {
	for _, conflict := range engine.discoveryMetrics.ResourceConflicts {
		if strings.Contains(conflict, testName) {
			return true
		}
	}
	return false
}

// updateDiscoveryMetrics updates discovery metrics
func (engine *TestDiscoveryEngine) updateDiscoveryMetrics() {
	engine.discoveryMetrics.TotalTestsDiscovered = len(engine.discoveredTests)
	
	// Reset counters
	for category := range engine.discoveryMetrics.TestsByCategory {
		engine.discoveryMetrics.TestsByCategory[category] = 0
	}
	for priority := range engine.discoveryMetrics.TestsByPriority {
		engine.discoveryMetrics.TestsByPriority[priority] = 0
	}
	for mode := range engine.discoveryMetrics.TestsByExecutionMode {
		engine.discoveryMetrics.TestsByExecutionMode[mode] = 0
	}
	
	// Count tests by category, priority, and execution mode
	for _, test := range engine.discoveredTests {
		engine.discoveryMetrics.TestsByCategory[test.Category]++
		engine.discoveryMetrics.TestsByPriority[test.Priority]++
		engine.discoveryMetrics.TestsByExecutionMode[test.ExecutionMode]++
		
		// Update category and priority mappings
		engine.categoryTests[test.Category] = append(engine.categoryTests[test.Category], test)
		engine.priorityTests[test.Priority] = append(engine.priorityTests[test.Priority], test)
	}
	
	// Calculate dependency complexity
	totalDependencies := 0
	for _, test := range engine.discoveredTests {
		totalDependencies += len(test.Dependencies)
	}
	if len(engine.discoveredTests) > 0 {
		engine.discoveryMetrics.DependencyComplexity = float64(totalDependencies) / float64(len(engine.discoveredTests))
	}
}

// FilterTests filters discovered tests based on criteria
func (engine *TestDiscoveryEngine) FilterTests(filter *TestFilter) ([]*TestDiscoveryMetadata, error) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	
	if !engine.discoveryComplete {
		return nil, fmt.Errorf("test discovery not complete")
	}
	
	filtered := make([]*TestDiscoveryMetadata, 0)
	
	for _, test := range engine.discoveredTests {
		if engine.matchesFilter(test, filter) {
			filtered = append(filtered, test)
		}
	}
	
	// Sort by priority (highest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Priority > filtered[j].Priority
	})
	
	return filtered, nil
}

// matchesFilter checks if a test matches the filter criteria
func (engine *TestDiscoveryEngine) matchesFilter(test *TestDiscoveryMetadata, filter *TestFilter) bool {
	// Check categories
	if len(filter.Categories) > 0 {
		found := false
		for _, category := range filter.Categories {
			if test.Category == category {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check priorities
	if len(filter.Priorities) > 0 {
		found := false
		for _, priority := range filter.Priorities {
			if test.Priority == priority {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check minimum priority
	if test.Priority < filter.MinPriority {
		return false
	}
	
	// Check execution modes
	if len(filter.ExecutionModes) > 0 {
		found := false
		for _, mode := range filter.ExecutionModes {
			if test.ExecutionMode == mode {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check max duration
	if filter.MaxDuration > 0 && test.Timeout > filter.MaxDuration {
		return false
	}
	
	// Check required resources
	if len(filter.RequiredResources) > 0 {
		hasAllResources := true
		for _, requiredType := range filter.RequiredResources {
			found := false
			for _, resource := range test.Resources {
				if resource.Type == requiredType {
					found = true
					break
				}
			}
			if !found {
				hasAllResources = false
				break
			}
		}
		if !hasAllResources {
			return false
		}
	}
	
	// Check tags
	if len(filter.Tags) > 0 {
		hasAllTags := true
		for _, requiredTag := range filter.Tags {
			found := false
			for _, testTag := range test.Tags {
				if testTag == requiredTag {
					found = true
					break
				}
			}
			if !found {
				hasAllTags = false
				break
			}
		}
		if !hasAllTags {
			return false
		}
	}
	
	// Check exclude tags
	if len(filter.ExcludeTags) > 0 {
		for _, excludeTag := range filter.ExcludeTags {
			for _, testTag := range test.Tags {
				if testTag == excludeTag {
					return false
				}
			}
		}
	}
	
	// Check include patterns
	if len(filter.IncludePatterns) > 0 {
		found := false
		for _, pattern := range filter.IncludePatterns {
			if pattern.MatchString(test.Name) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check exclude patterns
	for _, pattern := range filter.ExcludePatterns {
		if pattern.MatchString(test.Name) {
			return false
		}
	}
	
	return true
}

// GetDiscoveredTests returns all discovered tests
func (engine *TestDiscoveryEngine) GetDiscoveredTests() map[string]*TestDiscoveryMetadata {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	tests := make(map[string]*TestDiscoveryMetadata)
	for name, test := range engine.discoveredTests {
		tests[name] = test
	}
	return tests
}

// GetDependencyGraph returns the test dependency graph
func (engine *TestDiscoveryEngine) GetDependencyGraph() map[string]*TestDependencyNode {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	graph := make(map[string]*TestDependencyNode)
	for name, node := range engine.dependencyGraph {
		graph[name] = node
	}
	return graph
}

// GetExecutionLevels returns tests grouped by execution level
func (engine *TestDiscoveryEngine) GetExecutionLevels() [][]string {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	
	// Return a copy
	levels := make([][]string, len(engine.executionLevels))
	for i, level := range engine.executionLevels {
		levels[i] = make([]string, len(level))
		copy(levels[i], level)
	}
	return levels
}

// GetParallelGroups returns tests that can run in parallel
func (engine *TestDiscoveryEngine) GetParallelGroups() [][]string {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	
	// Return a copy
	groups := make([][]string, len(engine.parallelGroups))
	for i, group := range engine.parallelGroups {
		groups[i] = make([]string, len(group))
		copy(groups[i], group)
	}
	return groups
}

// GetDiscoveryMetrics returns discovery and analysis metrics
func (engine *TestDiscoveryEngine) GetDiscoveryMetrics() *TestDiscoveryMetrics {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	
	// Return a copy
	return &TestDiscoveryMetrics{
		TotalTestsDiscovered:     engine.discoveryMetrics.TotalTestsDiscovered,
		TestsByCategory:          copyIntMap(engine.discoveryMetrics.TestsByCategory),
		TestsByPriority:          copyIntMap(engine.discoveryMetrics.TestsByPriority),
		TestsByExecutionMode:     copyIntMap(engine.discoveryMetrics.TestsByExecutionMode),
		DependencyComplexity:     engine.discoveryMetrics.DependencyComplexity,
		DiscoveryDurationMs:      engine.discoveryMetrics.DiscoveryDurationMs,
		AnalysisDurationMs:       engine.discoveryMetrics.AnalysisDurationMs,
		CyclicDependencies:       copyStringSlice(engine.discoveryMetrics.CyclicDependencies),
		OrphanedTests:           copyStringSlice(engine.discoveryMetrics.OrphanedTests),
		ResourceConflicts:       copyStringSlice(engine.discoveryMetrics.ResourceConflicts),
	}
}

// Helper functions for copying data structures

func copyIntMap[K comparable](original map[K]int) map[K]int {
	copy := make(map[K]int)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

func copyStringSlice(original []string) []string {
	if original == nil {
		return nil
	}
	copy := make([]string, len(original))
	copy = append(copy[:0], original...)
	return copy
}

// IsDiscoveryComplete returns whether test discovery is complete
func (engine *TestDiscoveryEngine) IsDiscoveryComplete() bool {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	return engine.discoveryComplete
}

// GetTestByName returns a specific test by name
func (engine *TestDiscoveryEngine) GetTestByName(name string) (*TestDiscoveryMetadata, bool) {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	
	test, exists := engine.discoveredTests[name]
	return test, exists
}

// GetTestsByCategory returns tests in a specific category
func (engine *TestDiscoveryEngine) GetTestsByCategory(category TestCategory) []*TestDiscoveryMetadata {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	
	if tests, exists := engine.categoryTests[category]; exists {
		// Return a copy
		result := make([]*TestDiscoveryMetadata, len(tests))
		copy(result, tests)
		return result
	}
	return []*TestDiscoveryMetadata{}
}

// GetTestsByPriority returns tests with a specific priority
func (engine *TestDiscoveryEngine) GetTestsByPriority(priority TestPriority) []*TestDiscoveryMetadata {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	
	if tests, exists := engine.priorityTests[priority]; exists {
		// Return a copy
		result := make([]*TestDiscoveryMetadata, len(tests))
		copy(result, tests)
		return result
	}
	return []*TestDiscoveryMetadata{}
}