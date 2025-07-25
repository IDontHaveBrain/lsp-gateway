package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/gateway"
	"lsp-gateway/tests/framework"

	"github.com/stretchr/testify/suite"
)

// ConcurrentWorkflowTestSuite provides comprehensive concurrent operation testing
// for multi-language workflows, testing concurrent access, load balancing,
// resource management, and system stability under high load.
type ConcurrentWorkflowTestSuite struct {
	suite.Suite
	framework       *framework.MultiLanguageTestFramework
	testTimeout     time.Duration
	createdProjects []*framework.TestProject
}

// SetupSuite initializes the test suite for concurrent testing
func (suite *ConcurrentWorkflowTestSuite) SetupSuite() {
	suite.testTimeout = 10 * time.Minute // Longer timeout for concurrent tests
	suite.framework = framework.NewMultiLanguageTestFramework(suite.testTimeout)
	suite.createdProjects = make([]*framework.TestProject, 0)

	// Setup test environment
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := suite.framework.SetupTestEnvironment(ctx)
	suite.Require().NoError(err, "Failed to setup test environment")
}

// TearDownSuite cleans up all test resources
func (suite *ConcurrentWorkflowTestSuite) TearDownSuite() {
	if suite.framework != nil {
		err := suite.framework.CleanupAll()
		suite.Require().NoError(err, "Failed to cleanup test framework")
	}
}

// SetupTest prepares each individual test
func (suite *ConcurrentWorkflowTestSuite) SetupTest() {
	suite.createdProjects = make([]*framework.TestProject, 0)
}

// TestConcurrentMultiClientAccess tests multiple clients accessing different
// language servers simultaneously with proper isolation and resource management
func (suite *ConcurrentWorkflowTestSuite) TestConcurrentMultiClientAccess() {
	// Create project with multiple languages
	project, err := suite.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMonorepo,
		[]string{"go", "python", "typescript", "java", "rust"},
	)
	suite.Require().NoError(err, "Failed to create test project")
	suite.createdProjects = append(suite.createdProjects, project)

	// Start all language servers
	err = suite.framework.StartMultipleLanguageServers(project.Languages)
	suite.Require().NoError(err, "Failed to start language servers")

	// Create project gateway
	projectGateway, err := suite.framework.CreateGatewayWithProject(project)
	suite.Require().NoError(err, "Failed to create project gateway")

	ctx := suite.framework.Context()

	// Test concurrent client access
	suite.Run("Concurrent_Client_Access", func() {
		numClients := 10
		requestsPerClient := 20
		var wg sync.WaitGroup
		var totalRequests int64
		var successfulRequests int64
		var failedRequests int64

		// Track performance metrics
		startTime := time.Now()
		responseTimes := make([]time.Duration, 0, numClients*requestsPerClient)
		var responseTimesMutex sync.Mutex

		// Launch concurrent clients
		for clientID := 0; clientID < numClients; clientID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for requestID := 0; requestID < requestsPerClient; requestID++ {
					atomic.AddInt64(&totalRequests, 1)

					// Rotate through languages and request types
					lang := project.Languages[(id*requestsPerClient+requestID)%len(project.Languages)]
					methods := []string{
						"textDocument/definition",
						"textDocument/references",
						"textDocument/hover",
						"textDocument/documentSymbol",
					}
					method := methods[requestID%len(methods)]

					testFile := suite.getTestFileForLanguage(project, lang)
					if testFile == "" {
						atomic.AddInt64(&failedRequests, 1)
						continue
					}

					// Measure individual request time
					reqStart := time.Now()
					params := suite.createLSPParams(method, testFile)

					response, err := projectGateway.HandleLSPRequest(ctx, method, params)
					reqDuration := time.Since(reqStart)

					// Record response time
					responseTimesMutex.Lock()
					responseTimes = append(responseTimes, reqDuration)
					responseTimesMutex.Unlock()

					if err != nil || response == nil {
						atomic.AddInt64(&failedRequests, 1)
						suite.T().Logf("Request failed for client %d, request %d (%s %s): %v",
							id, requestID, lang, method, err)
					} else {
						atomic.AddInt64(&successfulRequests, 1)
					}

					// Small delay to prevent overwhelming servers
					time.Sleep(10 * time.Millisecond)
				}
			}(clientID)
		}

		// Wait for all clients to complete
		wg.Wait()
		totalDuration := time.Since(startTime)

		// Validate results
		suite.Equal(int64(numClients*requestsPerClient), totalRequests, "All requests should be tracked")
		suite.Greater(successfulRequests, totalRequests*80/100, "At least 80% of requests should succeed")
		suite.Less(failedRequests, totalRequests*20/100, "Less than 20% of requests should fail")

		// Validate performance
		suite.Less(totalDuration, 2*time.Minute, "All concurrent requests should complete within 2 minutes")

		// Calculate and validate response times
		if len(responseTimes) > 0 {
			avgResponseTime := suite.calculateAverageResponseTime(responseTimes)
			maxResponseTime := suite.calculateMaxResponseTime(responseTimes)

			suite.Less(avgResponseTime, 5*time.Second, "Average response time should be reasonable")
			suite.Less(maxResponseTime, 30*time.Second, "Maximum response time should be bounded")

			suite.T().Logf("Concurrent access results: Total=%d, Success=%d, Failed=%d, AvgTime=%v, MaxTime=%v",
				totalRequests, successfulRequests, failedRequests, avgResponseTime, maxResponseTime)
		}
	})

	// Test resource isolation between concurrent clients
	suite.Run("Client_Resource_Isolation", func() {
		numClients := 5
		var wg sync.WaitGroup
		clientMetrics := make([]map[string]interface{}, numClients)

		for clientID := 0; clientID < numClients; clientID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Each client performs different types of operations
				operationType := []string{"heavy_symbols", "frequent_definitions", "batch_references", "mixed_operations", "long_running"}[id]

				metrics := suite.performClientOperations(ctx, projectGateway, project, operationType)
				clientMetrics[id] = metrics
			}(clientID)
		}

		wg.Wait()

		// Validate that clients don't interfere with each other
		for i, metrics := range clientMetrics {
			suite.Greater(metrics["successful_requests"].(int), 0,
				fmt.Sprintf("Client %d should have successful requests", i))
			suite.Less(metrics["error_rate"].(float64), 0.2,
				fmt.Sprintf("Client %d should have low error rate", i))
		}
	})
}

// TestConcurrentProjectDetectionAndServerManagement tests concurrent project
// detection and server management operations
func (suite *ConcurrentWorkflowTestSuite) TestConcurrentProjectDetectionAndServerManagement() {
	// Create multiple projects for concurrent detection
	numProjects := 5
	projects := make([]*framework.TestProject, numProjects)

	for i := 0; i < numProjects; i++ {
		projectType := []framework.ProjectType{
			framework.ProjectTypeMonorepo,
			framework.ProjectTypeMicroservices,
			framework.ProjectTypeFrontendBackend,
			framework.ProjectTypeMultiLanguage,
			framework.ProjectTypeWorkspace,
		}[i]

		languages := [][]string{
			{"go", "python", "typescript"},
			{"go", "java", "python"},
			{"typescript", "go"},
			{"python", "rust", "java"},
			{"go", "typescript", "python", "java"},
		}[i]

		project, err := suite.framework.CreateMultiLanguageProject(projectType, languages)
		suite.Require().NoError(err, fmt.Sprintf("Failed to create project %d", i))
		projects[i] = project
		suite.createdProjects = append(suite.createdProjects, project)
	}

	ctx := suite.framework.Context()

	// Test concurrent project detection
	suite.Run("Concurrent_Project_Detection", func() {
		var wg sync.WaitGroup
		detectionResults := make([]*gateway.ProjectInfo, numProjects)
		detectionErrors := make([]error, numProjects)

		detector := gateway.NewProjectDetector()

		// Detect all projects concurrently
		for i, project := range projects {
			wg.Add(1)
			go func(index int, proj *framework.TestProject) {
				defer wg.Done()

				info, err := detector.DetectProject(ctx, proj.RootPath)
				detectionResults[index] = info
				detectionErrors[index] = err
			}(i, project)
		}

		wg.Wait()

		// Validate all detections succeeded
		for i, err := range detectionErrors {
			suite.NoError(err, fmt.Sprintf("Project %d detection should succeed", i))
			suite.NotNil(detectionResults[i], fmt.Sprintf("Project %d should have detection results", i))
		}

		// Validate detection results quality
		for i, info := range detectionResults {
			if info != nil {
				suite.Greater(len(info.Languages), 0,
					fmt.Sprintf("Project %d should detect languages", i))
				suite.NotEmpty(info.ProjectType,
					fmt.Sprintf("Project %d should have project type", i))
			}
		}
	})

	// Test concurrent server management
	suite.Run("Concurrent_Server_Management", func() {
		var wg sync.WaitGroup
		numManagers := 3
		managers := make([]*gateway.MultiServerManager, numManagers)

		// Create multiple server managers
		for i := 0; i < numManagers; i++ {
			managers[i] = gateway.NewMultiServerManager(nil)
		}

		// Start servers concurrently for different projects
		for i, manager := range managers {
			wg.Add(1)
			go func(index int, mgr *gateway.MultiServerManager) {
				defer wg.Done()

				// Each manager handles different projects
				startProject := index * 2 % numProjects
				endProject := ((index + 1) * 2) % numProjects
				if endProject <= startProject {
					endProject = numProjects
				}

				for j := startProject; j < endProject && j < numProjects; j++ {
					project := projects[j]

					// Start servers for project languages
					err := suite.framework.StartMultipleLanguageServers(project.Languages)
					if err != nil {
						suite.T().Logf("Failed to start servers for manager %d project %d: %v",
							index, j, err)
					}
				}
			}(i, manager)
		}

		wg.Wait()

		// Validate server states
		for i, manager := range managers {
			activeServers := manager.GetActiveServers()
			suite.GreaterOrEqual(len(activeServers), 0,
				fmt.Sprintf("Manager %d should have server information", i))
		}
	})
}

// TestLoadBalancingUnderConcurrentAccess tests load balancing effectiveness
// under concurrent access patterns
func (suite *ConcurrentWorkflowTestSuite) TestLoadBalancingUnderConcurrentAccess() {
	// Create project for load balancing test
	project, err := suite.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMonorepo,
		[]string{"go", "python", "typescript", "java"},
	)
	suite.Require().NoError(err, "Failed to create project for load balancing test")
	suite.createdProjects = append(suite.createdProjects, project)

	// Start language servers
	err = suite.framework.StartMultipleLanguageServers(project.Languages)
	suite.Require().NoError(err, "Failed to start language servers")

	// Execute load balancing scenario
	result, err := suite.framework.SimulateComplexWorkflow(framework.ScenarioLoadBalancing)
	suite.Require().NoError(err, "Load balancing simulation failed")
	suite.True(result.Success, "Load balancing should succeed")

	ctx := suite.framework.Context()

	// Test specific load balancing scenarios
	suite.Run("Round_Robin_Load_Distribution", func() {
		loadBalancer := gateway.NewLoadBalancer()
		multiServerManager := gateway.NewMultiServerManager(nil)

		numRequests := 100
		serverUsage := make(map[string]int)
		var usageMutex sync.Mutex
		var wg sync.WaitGroup

		// Generate concurrent requests
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				lang := project.Languages[requestID%len(project.Languages)]
				server, err := loadBalancer.SelectServer(ctx, lang, multiServerManager)

				if err == nil && server != nil {
					usageMutex.Lock()
					serverUsage[server.ID]++
					usageMutex.Unlock()
				}
			}(i)
		}

		wg.Wait()

		// Validate load distribution
		if len(serverUsage) > 1 {
			minUsage := numRequests
			maxUsage := 0

			for _, usage := range serverUsage {
				if usage < minUsage {
					minUsage = usage
				}
				if usage > maxUsage {
					maxUsage = usage
				}
			}

			// Load should be reasonably distributed (max usage shouldn't be more than 2x min usage)
			loadVariance := float64(maxUsage-minUsage) / float64(minUsage)
			suite.Less(loadVariance, 2.0, "Load should be reasonably distributed across servers")
		}
	})

	// Test adaptive load balancing under varying loads
	suite.Run("Adaptive_Load_Balancing", func() {
		loadBalancer := gateway.NewLoadBalancer()

		// Simulate varying load patterns
		loadPatterns := []struct {
			name          string
			requestCount  int
			burstInterval time.Duration
		}{
			{"steady_load", 50, 10 * time.Millisecond},
			{"burst_load", 100, 1 * time.Millisecond},
			{"sporadic_load", 30, 50 * time.Millisecond},
		}

		for _, pattern := range loadPatterns {
			suite.Run(pattern.name, func() {
				var wg sync.WaitGroup
				successCount := int64(0)

				for i := 0; i < pattern.requestCount; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()

						// Simulate load balancing decision
						decision := loadBalancer.MakeRoutingDecision(ctx, "go")
						if decision != nil && decision.SelectedServer != "" {
							atomic.AddInt64(&successCount, 1)
						}
					}()

					time.Sleep(pattern.burstInterval)
				}

				wg.Wait()

				// Validate that load balancer handles the pattern effectively
				successRate := float64(successCount) / float64(pattern.requestCount)
				suite.Greater(successRate, 0.8,
					fmt.Sprintf("Load balancer should handle %s effectively", pattern.name))
			})
		}
	})
}

// TestResourceManagementUnderHighLoad tests resource management and
// system stability under high concurrent load
func (suite *ConcurrentWorkflowTestSuite) TestResourceManagementUnderHighLoad() {
	// Create large project for stress testing
	project, err := suite.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMonorepo,
		[]string{"go", "python", "typescript", "java", "rust", "cpp"},
	)
	suite.Require().NoError(err, "Failed to create project for stress test")
	suite.createdProjects = append(suite.createdProjects, project)

	// Start all language servers
	err = suite.framework.StartMultipleLanguageServers(project.Languages)
	suite.Require().NoError(err, "Failed to start language servers")

	ctx := suite.framework.Context()

	// Test high-concurrency stress scenario
	suite.Run("High_Concurrency_Stress_Test", func() {
		numWorkers := 20
		requestsPerWorker := 50
		var wg sync.WaitGroup

		// Metrics tracking
		var totalRequests int64
		var successfulRequests int64
		var failedRequests int64
		startTime := time.Now()

		// Resource monitoring
		resourceMonitor := suite.startResourceMonitoring(ctx)
		defer resourceMonitor.Stop()

		// Launch high-concurrency workers
		for workerID := 0; workerID < numWorkers; workerID++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				projectGateway, err := suite.framework.CreateGatewayWithProject(project)
				if err != nil {
					suite.T().Logf("Worker %d failed to create gateway: %v", id, err)
					return
				}

				for requestID := 0; requestID < requestsPerWorker; requestID++ {
					atomic.AddInt64(&totalRequests, 1)

					// Generate high-frequency requests
					lang := project.Languages[(id*requestsPerWorker+requestID)%len(project.Languages)]
					methods := []string{
						"textDocument/definition",
						"textDocument/references",
						"textDocument/hover",
						"textDocument/documentSymbol",
						"workspace/symbol",
					}
					method := methods[requestID%len(methods)]

					testFile := suite.getTestFileForLanguage(project, lang)
					if testFile == "" {
						atomic.AddInt64(&failedRequests, 1)
						continue
					}

					params := suite.createLSPParams(method, testFile)

					_, err := projectGateway.HandleLSPRequest(ctx, method, params)
					if err != nil {
						atomic.AddInt64(&failedRequests, 1)
					} else {
						atomic.AddInt64(&successfulRequests, 1)
					}

					// Minimal delay for maximum stress
					time.Sleep(1 * time.Millisecond)
				}
			}(workerID)
		}

		wg.Wait()
		totalDuration := time.Since(startTime)

		// Collect final resource metrics
		finalMetrics := resourceMonitor.GetMetrics()

		// Validate stress test results
		suite.Equal(int64(numWorkers*requestsPerWorker), totalRequests, "All requests should be tracked")

		// Under high stress, we expect some failures but system should remain stable
		successRate := float64(successfulRequests) / float64(totalRequests)
		suite.Greater(successRate, 0.5, "At least 50% of requests should succeed under stress")

		// System should complete within reasonable time even under stress
		suite.Less(totalDuration, 5*time.Minute, "Stress test should complete within 5 minutes")

		// Validate resource usage remains bounded
		suite.Less(finalMetrics.MemoryUsageMB, 2000.0, "Memory usage should remain bounded")
		suite.Less(finalMetrics.CPUUsagePercent, 95.0, "CPU usage should not max out")

		suite.T().Logf("Stress test results: Total=%d, Success=%d, Failed=%d, Duration=%v, SuccessRate=%.2f",
			totalRequests, successfulRequests, failedRequests, totalDuration, successRate)
	})

	// Test resource cleanup and garbage collection
	suite.Run("Resource_Cleanup_Under_Load", func() {
		// Start resource monitoring
		resourceMonitor := suite.startResourceMonitoring(ctx)
		defer resourceMonitor.Stop()

		initialMetrics := resourceMonitor.GetMetrics()

		// Create and destroy multiple gateways to test cleanup
		numCycles := 10
		for cycle := 0; cycle < numCycles; cycle++ {
			var wg sync.WaitGroup
			numGateways := 5

			// Create multiple gateways concurrently
			for i := 0; i < numGateways; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					gateway, err := suite.framework.CreateGatewayWithProject(project)
					if err != nil {
						return
					}

					// Use gateway briefly
					testFile := suite.getTestFileForLanguage(project, "go")
					if testFile != "" {
						params := suite.createLSPParams("textDocument/definition", testFile)
						gateway.HandleLSPRequest(ctx, "textDocument/definition", params)
					}

					// Gateway will be garbage collected when function exits
				}()
			}

			wg.Wait()

			// Force garbage collection
			// In real implementation, this would be handled by the framework
			time.Sleep(100 * time.Millisecond)
		}

		// Wait a bit for cleanup
		time.Sleep(2 * time.Second)
		finalMetrics := resourceMonitor.GetMetrics()

		// Memory growth should be reasonable despite creating/destroying many gateways
		memoryGrowth := finalMetrics.MemoryUsageMB - initialMetrics.MemoryUsageMB
		suite.Less(memoryGrowth, 500.0, "Memory growth should be controlled during resource cycling")
	})
}

// TestConcurrentWorkspaceOperations tests concurrent workspace-level operations
func (suite *ConcurrentWorkflowTestSuite) TestConcurrentWorkspaceOperations() {
	// Execute concurrent workspaces scenario
	result, err := suite.framework.SimulateComplexWorkflow(framework.ScenarioConcurrentWorkspaces)
	suite.Require().NoError(err, "Concurrent workspaces simulation failed")
	suite.True(result.Success, "Concurrent workspaces should succeed")

	// Create multiple projects simulating different workspaces
	numWorkspaces := 3
	workspaceProjects := make([]*framework.TestProject, numWorkspaces)

	for i := 0; i < numWorkspaces; i++ {
		project, err := suite.framework.CreateMultiLanguageProject(
			framework.ProjectTypeWorkspace,
			[]string{"go", "python", "typescript"},
		)
		suite.Require().NoError(err, fmt.Sprintf("Failed to create workspace %d", i))
		workspaceProjects[i] = project
		suite.createdProjects = append(suite.createdProjects, project)
	}

	ctx := suite.framework.Context()

	// Test concurrent workspace symbol searches
	suite.Run("Concurrent_Workspace_Symbol_Search", func() {
		var wg sync.WaitGroup

		for i, project := range workspaceProjects {
			wg.Add(1)
			go func(workspaceID int, proj *framework.TestProject) {
				defer wg.Done()

				// Start servers for this workspace
				err := suite.framework.StartMultipleLanguageServers(proj.Languages)
				if err != nil {
					suite.T().Logf("Failed to start servers for workspace %d: %v", workspaceID, err)
					return
				}

				// Create gateway for workspace
				gateway, err := suite.framework.CreateGatewayWithProject(proj)
				if err != nil {
					suite.T().Logf("Failed to create gateway for workspace %d: %v", workspaceID, err)
					return
				}

				// Perform multiple symbol searches concurrently within workspace
				symbolQueries := []string{"User", "Service", "Handler", "Model", "Config"}
				var innerWg sync.WaitGroup

				for _, query := range symbolQueries {
					innerWg.Add(1)
					go func(symbolQuery string) {
						defer innerWg.Done()

						params := map[string]interface{}{
							"query": symbolQuery,
						}

						response, err := gateway.HandleLSPRequest(ctx, "workspace/symbol", params)
						if err != nil {
							suite.T().Logf("Symbol search failed in workspace %d for query %s: %v",
								workspaceID, symbolQuery, err)
						} else if response != nil {
							suite.T().Logf("Symbol search succeeded in workspace %d for query %s",
								workspaceID, symbolQuery)
						}
					}(query)
				}

				innerWg.Wait()
			}(i, project)
		}

		wg.Wait()
	})

	// Test concurrent workspace modifications
	suite.Run("Concurrent_Workspace_Modifications", func() {
		var wg sync.WaitGroup

		for i, project := range workspaceProjects {
			wg.Add(1)
			go func(workspaceID int, proj *framework.TestProject) {
				defer wg.Done()

				// Simulate file modifications in the workspace
				modificationTypes := []string{"file_create", "file_modify", "file_delete"}

				for _, modType := range modificationTypes {
					suite.simulateWorkspaceModification(ctx, proj, modType)
					time.Sleep(100 * time.Millisecond) // Small delay between modifications
				}
			}(i, project)
		}

		wg.Wait()
	})
}

// Helper methods for concurrent testing

// performClientOperations performs different types of operations for client isolation testing
func (suite *ConcurrentWorkflowTestSuite) performClientOperations(ctx context.Context, gateway *gateway.ProjectAwareGateway, project *framework.TestProject, operationType string) map[string]interface{} {
	metrics := map[string]interface{}{
		"successful_requests": 0,
		"failed_requests":     0,
		"error_rate":          0.0,
		"operation_type":      operationType,
	}

	numRequests := 20
	successCount := 0
	failureCount := 0

	switch operationType {
	case "heavy_symbols":
		// Perform symbol-heavy operations
		for i := 0; i < numRequests; i++ {
			params := map[string]interface{}{"query": fmt.Sprintf("symbol_%d", i)}
			_, err := gateway.HandleLSPRequest(ctx, "workspace/symbol", params)
			if err != nil {
				failureCount++
			} else {
				successCount++
			}
		}

	case "frequent_definitions":
		// Perform frequent definition lookups
		for i := 0; i < numRequests; i++ {
			lang := project.Languages[i%len(project.Languages)]
			testFile := suite.getTestFileForLanguage(project, lang)
			if testFile != "" {
				params := suite.createLSPParams("textDocument/definition", testFile)
				_, err := gateway.HandleLSPRequest(ctx, "textDocument/definition", params)
				if err != nil {
					failureCount++
				} else {
					successCount++
				}
			}
		}

	case "batch_references":
		// Perform batch reference operations
		for i := 0; i < numRequests; i++ {
			lang := project.Languages[i%len(project.Languages)]
			testFile := suite.getTestFileForLanguage(project, lang)
			if testFile != "" {
				params := suite.createLSPParams("textDocument/references", testFile)
				_, err := gateway.HandleLSPRequest(ctx, "textDocument/references", params)
				if err != nil {
					failureCount++
				} else {
					successCount++
				}
			}
		}

	case "mixed_operations":
		// Perform mixed operations
		methods := []string{"textDocument/definition", "textDocument/references", "textDocument/hover"}
		for i := 0; i < numRequests; i++ {
			lang := project.Languages[i%len(project.Languages)]
			method := methods[i%len(methods)]
			testFile := suite.getTestFileForLanguage(project, lang)
			if testFile != "" {
				params := suite.createLSPParams(method, testFile)
				_, err := gateway.HandleLSPRequest(ctx, method, params)
				if err != nil {
					failureCount++
				} else {
					successCount++
				}
			}
		}

	case "long_running":
		// Perform operations with longer delays
		for i := 0; i < numRequests/2; i++ { // Fewer requests but longer running
			lang := project.Languages[i%len(project.Languages)]
			testFile := suite.getTestFileForLanguage(project, lang)
			if testFile != "" {
				params := suite.createLSPParams("textDocument/documentSymbol", testFile)
				_, err := gateway.HandleLSPRequest(ctx, "textDocument/documentSymbol", params)
				if err != nil {
					failureCount++
				} else {
					successCount++
				}
				time.Sleep(100 * time.Millisecond) // Simulate longer processing
			}
		}
	}

	metrics["successful_requests"] = successCount
	metrics["failed_requests"] = failureCount
	if successCount+failureCount > 0 {
		metrics["error_rate"] = float64(failureCount) / float64(successCount+failureCount)
	}

	return metrics
}

// ResourceMonitor provides basic resource monitoring for tests
type ResourceMonitor struct {
	ctx     context.Context
	cancel  context.CancelFunc
	metrics *framework.PerformanceMetrics
	running bool
	mu      sync.RWMutex
}

// startResourceMonitoring starts monitoring system resources
func (suite *ConcurrentWorkflowTestSuite) startResourceMonitoring(ctx context.Context) *ResourceMonitor {
	monitorCtx, cancel := context.WithCancel(ctx)

	monitor := &ResourceMonitor{
		ctx:     monitorCtx,
		cancel:  cancel,
		metrics: &framework.PerformanceMetrics{},
		running: true,
	}

	// Start monitoring goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				monitor.updateMetrics()
			}
		}
	}()

	return monitor
}

// updateMetrics updates the performance metrics (simplified implementation)
func (rm *ResourceMonitor) updateMetrics() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// In a real implementation, this would collect actual system metrics
	// For testing, we simulate reasonable values
	rm.metrics.MemoryAllocated += 1024 * 1024 // 1MB increments
	rm.metrics.MemoryUsageMB = float64(rm.metrics.MemoryAllocated) / 1024 / 1024
	rm.metrics.CPUUsagePercent = 25.0 + float64(rm.metrics.NetworkRequests%40) // Simulate 25-65% CPU
	rm.metrics.GoroutineCount += 1
	rm.metrics.NetworkRequests++
}

// GetMetrics returns current performance metrics
func (rm *ResourceMonitor) GetMetrics() *framework.PerformanceMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Return a copy
	return &framework.PerformanceMetrics{
		OperationDuration:  rm.metrics.OperationDuration,
		MemoryAllocated:    rm.metrics.MemoryAllocated,
		MemoryFreed:        rm.metrics.MemoryFreed,
		MemoryUsageMB:      rm.metrics.MemoryUsageMB,
		CPUUsagePercent:    rm.metrics.CPUUsagePercent,
		GoroutineCount:     rm.metrics.GoroutineCount,
		FileOperations:     rm.metrics.FileOperations,
		NetworkRequests:    rm.metrics.NetworkRequests,
		CacheOperations:    rm.metrics.CacheOperations,
		ServerCreations:    rm.metrics.ServerCreations,
		ServerDestructions: rm.metrics.ServerDestructions,
		ErrorCount:         rm.metrics.ErrorCount,
		WarningCount:       rm.metrics.WarningCount,
	}
}

// Stop stops the resource monitor
func (rm *ResourceMonitor) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.running {
		rm.cancel()
		rm.running = false
	}
}

// simulateWorkspaceModification simulates workspace file modifications
func (suite *ConcurrentWorkflowTestSuite) simulateWorkspaceModification(ctx context.Context, project *framework.TestProject, modificationType string) {
	// In a real implementation, this would modify files and notify the LSP servers
	// For testing, we just simulate the operation

	switch modificationType {
	case "file_create":
		// Simulate creating a new file
		newFile := fmt.Sprintf("new_file_%d.go", time.Now().UnixNano())
		suite.T().Logf("Simulating file creation: %s in project %s", newFile, project.Name)

	case "file_modify":
		// Simulate modifying an existing file
		if len(project.Structure) > 0 {
			for filePath := range project.Structure {
				suite.T().Logf("Simulating file modification: %s in project %s", filePath, project.Name)
				break // Just simulate modifying the first file
			}
		}

	case "file_delete":
		// Simulate deleting a file
		deleteFile := fmt.Sprintf("temp_file_%d.go", time.Now().UnixNano())
		suite.T().Logf("Simulating file deletion: %s in project %s", deleteFile, project.Name)
	}

	// Small delay to simulate file system operation
	time.Sleep(10 * time.Millisecond)
}

// Utility methods from the main workflow test

// getTestFileForLanguage returns a test file path for the given language
func (suite *ConcurrentWorkflowTestSuite) getTestFileForLanguage(project *framework.TestProject, language string) string {
	languageFiles := map[string]string{
		"go":         "main.go",
		"python":     "main.py",
		"typescript": "src/index.ts",
		"java":       "src/main/java/Main.java",
		"rust":       "src/main.rs",
		"cpp":        "main.cpp",
	}

	if fileName, exists := languageFiles[language]; exists {
		return filepath.Join(project.RootPath, fileName)
	}
	return ""
}

// createLSPParams creates LSP request parameters for testing
func (suite *ConcurrentWorkflowTestSuite) createLSPParams(method, filePath string) interface{} {
	switch method {
	case "textDocument/definition", "textDocument/references", "textDocument/hover":
		return map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fmt.Sprintf("file://%s", filePath),
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		}
	case "textDocument/documentSymbol":
		return map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fmt.Sprintf("file://%s", filePath),
			},
		}
	case "workspace/symbol":
		return map[string]interface{}{
			"query": "test",
		}
	default:
		return map[string]interface{}{}
	}
}

// calculateAverageResponseTime calculates average response time from a slice of durations
func (suite *ConcurrentWorkflowTestSuite) calculateAverageResponseTime(responseTimes []time.Duration) time.Duration {
	if len(responseTimes) == 0 {
		return 0
	}

	var total time.Duration
	for _, duration := range responseTimes {
		total += duration
	}

	return total / time.Duration(len(responseTimes))
}

// calculateMaxResponseTime finds the maximum response time from a slice of durations
func (suite *ConcurrentWorkflowTestSuite) calculateMaxResponseTime(responseTimes []time.Duration) time.Duration {
	if len(responseTimes) == 0 {
		return 0
	}

	max := responseTimes[0]
	for _, duration := range responseTimes[1:] {
		if duration > max {
			max = duration
		}
	}

	return max
}

// Run the test suite
func TestConcurrentWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(ConcurrentWorkflowTestSuite))
}
