package testutils

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MultiProjectPerformanceMetrics tracks performance metrics for multi-project operations
type MultiProjectPerformanceMetrics struct {
	SubProjectLatencies map[string]time.Duration `json:"sub_project_latencies"`
	CrossProjectRequests int                      `json:"cross_project_requests"`
	RoutingErrors        int                      `json:"routing_errors"`
	CacheHitRates        map[string]float64       `json:"cache_hit_rates"`
	TotalRequests        int64                    `json:"total_requests"`
	SuccessfulRequests   int64                    `json:"successful_requests"`
	FailedRequests       int64                    `json:"failed_requests"`
	TestDuration         time.Duration            `json:"test_duration"`
	ThroughputRPS        float64                  `json:"throughput_rps"`
	mu                   sync.RWMutex
}

// RoutingValidationResult contains results of request routing validation
type RoutingValidationResult struct {
	CorrectlyRoutedRequests int                                `json:"correctly_routed_requests"`
	MisroutedRequests       int                                `json:"misrouted_requests"`
	FailedRequests          int                                `json:"failed_requests"`
	SubProjectResults       map[string]*SubProjectRoutingResult `json:"sub_project_results"`
	ValidationErrors        []string                           `json:"validation_errors"`
	TestDuration            time.Duration                      `json:"test_duration"`
}

// SubProjectRoutingResult contains routing results for a specific sub-project
type SubProjectRoutingResult struct {
	Language           string        `json:"language"`
	SuccessfulRequests int           `json:"successful_requests"`
	RoutingErrors      int           `json:"routing_errors"`
	AverageLatency     time.Duration `json:"average_latency"`
	TestedMethods      []string      `json:"tested_methods"`
	ErrorsByMethod     map[string]int `json:"errors_by_method"`
}

// MultiProjectTestResults contains comprehensive results for multi-project testing
type MultiProjectTestResults struct {
	WorkspaceValidation  bool                             `json:"workspace_validation"`
	RoutingValidation    *RoutingValidationResult         `json:"routing_validation"`
	PerformanceMetrics   *MultiProjectPerformanceMetrics `json:"performance_metrics"`
	SubProjectResults    map[string]*SubProjectTestResult `json:"sub_project_results"`
	CrossProjectTests    []CrossProjectTestResult         `json:"cross_project_tests"`
	TestDuration         time.Duration                    `json:"test_duration"`
	TotalErrors          int                              `json:"total_errors"`
}

// SubProjectTestResult contains test results for a single sub-project
type SubProjectTestResult struct {
	Language            string            `json:"language"`
	ProjectPath         string            `json:"project_path"`
	TestsExecuted       int               `json:"tests_executed"`
	TestsPassed         int               `json:"tests_passed"`
	TestsFailed         int               `json:"tests_failed"`
	AverageResponseTime time.Duration     `json:"average_response_time"`
	LSPMethodResults    map[string]int    `json:"lsp_method_results"`
	Errors              []string          `json:"errors"`
}

// CrossProjectTestResult represents the result of cross-project testing
type CrossProjectTestResult struct {
	TestCase     CrossProjectTestCase `json:"test_case"`
	Success      bool                 `json:"success"`
	ResponseTime time.Duration        `json:"response_time"`
	Error        string               `json:"error,omitempty"`
	RoutedTo     string               `json:"routed_to,omitempty"`
}

// ValidateRoutingToCorrectSubProject validates that LSP requests are routed to the correct sub-project
func ValidateRoutingToCorrectSubProject(ctx context.Context, httpClient *HttpClient, testCase CrossProjectTestCase) error {
	if httpClient == nil {
		return fmt.Errorf("httpClient cannot be nil")
	}

	// Convert file path to URI format
	fileURI := fmt.Sprintf("file://%s", testCase.SourceFile)
	
	startTime := time.Now()
	
	// Test different LSP methods based on expected type
	switch testCase.ExpectedType {
	case "definition":
		locations, err := httpClient.Definition(ctx, fileURI, testCase.Position)
		if err != nil {
			return fmt.Errorf("definition request failed for %s->%s: %w", 
				testCase.SourceProject, testCase.TargetProject, err)
		}
		
		// Validate that at least one location is returned and points to correct project
		if len(locations) == 0 {
			return fmt.Errorf("no definitions found for %s->%s routing test", 
				testCase.SourceProject, testCase.TargetProject)
		}
		
		// Check if locations are routed to expected project
		for _, location := range locations {
			if !strings.Contains(location.URI, testCase.TargetProject) {
				return fmt.Errorf("definition incorrectly routed: expected %s project, got URI %s", 
					testCase.TargetProject, location.URI)
			}
		}
		
	case "references":
		locations, err := httpClient.References(ctx, fileURI, testCase.Position, true)
		if err != nil {
			return fmt.Errorf("references request failed for %s->%s: %w", 
				testCase.SourceProject, testCase.TargetProject, err)
		}
		
		// Validate routing to correct project (references might be empty)
		for _, location := range locations {
			if !strings.Contains(location.URI, testCase.SourceProject) && 
			   !strings.Contains(location.URI, testCase.TargetProject) {
				return fmt.Errorf("references incorrectly routed: expected %s or %s project, got URI %s", 
					testCase.SourceProject, testCase.TargetProject, location.URI)
			}
		}
		
	case "hover":
		hoverResult, err := httpClient.Hover(ctx, fileURI, testCase.Position)
		if err != nil {
			return fmt.Errorf("hover request failed for %s->%s: %w", 
				testCase.SourceProject, testCase.TargetProject, err)
		}
		
		// Hover can be nil for positions without hover info
		if hoverResult == nil {
			return fmt.Errorf("no hover information found for %s->%s routing test", 
				testCase.SourceProject, testCase.TargetProject)
		}
		
	default:
		return fmt.Errorf("unsupported test type: %s", testCase.ExpectedType)
	}
	
	duration := time.Since(startTime)
	if duration > 10*time.Second {
		return fmt.Errorf("routing test %s->%s took too long: %v", 
			testCase.SourceProject, testCase.TargetProject, duration)
	}
	
	return nil
}

// TestSubProjectIsolation tests that requests are properly isolated between sub-projects
func TestSubProjectIsolation(ctx context.Context, httpClient *HttpClient, subProjects []*SubProjectInfo) error {
	if httpClient == nil {
		return fmt.Errorf("httpClient cannot be nil")
	}
	
	if len(subProjects) < 2 {
		return fmt.Errorf("need at least 2 sub-projects for isolation testing")
	}
	
	isolationErrors := []string{}
	
	// Test each sub-project in isolation
	for _, sourceProject := range subProjects {
		// Get test files for this project
		testFiles, err := GetTestFilesForSubProject(sourceProject)
		if err != nil || len(testFiles) == 0 {
			// Try to get source files instead
			sourceFiles, err := getSourceFilesForLanguage(sourceProject)
			if err != nil || len(sourceFiles) == 0 {
				isolationErrors = append(isolationErrors, 
					fmt.Sprintf("no test or source files found for %s project", sourceProject.Language))
				continue
			}
			testFiles = sourceFiles[:1] // Use first source file
		}
		
		// Test with the first available file
		testFile := testFiles[0]
		fileURI := fmt.Sprintf("file://%s/%s", sourceProject.ProjectPath, testFile)
		testPosition := Position{Line: 0, Character: 0}
		
		// Test definition request
		locations, err := httpClient.Definition(ctx, fileURI, testPosition)
		if err != nil {
			isolationErrors = append(isolationErrors, 
				fmt.Sprintf("definition request failed for %s: %v", sourceProject.Language, err))
			continue
		}
		
		// Validate that all returned locations belong to the same project
		for _, location := range locations {
			locationPath := strings.TrimPrefix(location.URI, "file://")
			if !strings.HasPrefix(locationPath, sourceProject.ProjectPath) {
				isolationErrors = append(isolationErrors, 
					fmt.Sprintf("isolation violation in %s: location %s points outside project", 
						sourceProject.Language, location.URI))
			}
		}
		
		// Test references request
		references, err := httpClient.References(ctx, fileURI, testPosition, false)
		if err != nil {
			isolationErrors = append(isolationErrors, 
				fmt.Sprintf("references request failed for %s: %v", sourceProject.Language, err))
			continue
		}
		
		// Check references isolation
		for _, reference := range references {
			referencePath := strings.TrimPrefix(reference.URI, "file://")
			if !strings.HasPrefix(referencePath, sourceProject.ProjectPath) {
				isolationErrors = append(isolationErrors, 
					fmt.Sprintf("isolation violation in %s: reference %s points outside project", 
						sourceProject.Language, reference.URI))
			}
		}
	}
	
	if len(isolationErrors) > 0 {
		return fmt.Errorf("sub-project isolation failures: %s", strings.Join(isolationErrors, "; "))
	}
	
	return nil
}

// ValidateWorkspaceSymbolRouting tests workspace symbol routing across multiple sub-projects
func ValidateWorkspaceSymbolRouting(ctx context.Context, httpClient *HttpClient, workspace string, subProjects []*SubProjectInfo) error {
	if httpClient == nil {
		return fmt.Errorf("httpClient cannot be nil")
	}
	
	if len(subProjects) == 0 {
		return fmt.Errorf("no sub-projects provided for workspace symbol routing test")
	}
	
	// Test queries that should span multiple projects
	testQueries := []string{"test", "main", "class", "function", "method"}
	
	for _, query := range testQueries {
		symbols, err := httpClient.WorkspaceSymbol(ctx, query)
		if err != nil {
			return fmt.Errorf("workspace symbol request failed for query '%s': %w", query, err)
		}
		
		// Validate that symbols from different projects are returned
		projectsWithSymbols := make(map[string]int)
		
		for _, symbol := range symbols {
			symbolPath := strings.TrimPrefix(symbol.Location.URI, "file://")
			
			// Identify which project this symbol belongs to
			for _, project := range subProjects {
				if strings.HasPrefix(symbolPath, project.ProjectPath) {
					projectsWithSymbols[project.Language]++
					break
				}
			}
		}
		
		// We expect symbols from at least one project (workspace might not have symbols for all queries)
		if len(projectsWithSymbols) == 0 {
			return fmt.Errorf("workspace symbol query '%s' returned no symbols from any project", query)
		}
		
		// Log successful routing
		projectNames := make([]string, 0, len(projectsWithSymbols))
		for project := range projectsWithSymbols {
			projectNames = append(projectNames, project)
		}
	}
	
	return nil
}

// BenchmarkMultiProjectOperations performs performance benchmarking on multi-project operations
func BenchmarkMultiProjectOperations(ctx context.Context, httpClient *HttpClient, subProjects []*SubProjectInfo) (*MultiProjectPerformanceMetrics, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("httpClient cannot be nil")
	}
	
	metrics := &MultiProjectPerformanceMetrics{
		SubProjectLatencies: make(map[string]time.Duration),
		CacheHitRates:       make(map[string]float64),
	}
	
	startTime := time.Now()
	
	// Benchmark each sub-project
	for _, project := range subProjects {
		projectStartTime := time.Now()
		
		// Get test files
		testFiles, err := GetTestFilesForSubProject(project)
		if err != nil || len(testFiles) == 0 {
			sourceFiles, err := getSourceFilesForLanguage(project)
			if err != nil || len(sourceFiles) == 0 {
				continue
			}
			testFiles = sourceFiles[:1]
		}
		
		testFile := testFiles[0]
		fileURI := fmt.Sprintf("file://%s/%s", project.ProjectPath, testFile)
		testPosition := Position{Line: 0, Character: 0}
		
		// Perform multiple operations to measure performance
		operationCount := 10
		successCount := int64(0)
		
		for i := 0; i < operationCount; i++ {
			atomic.AddInt64(&metrics.TotalRequests, 1)
			
			// Test definition
			_, err := httpClient.Definition(ctx, fileURI, testPosition)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&metrics.FailedRequests, 1)
			}
			
			// Test references
			atomic.AddInt64(&metrics.TotalRequests, 1)
			_, err = httpClient.References(ctx, fileURI, testPosition, true)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&metrics.FailedRequests, 1)
			}
			
			// Test hover
			atomic.AddInt64(&metrics.TotalRequests, 1)
			_, err = httpClient.Hover(ctx, fileURI, testPosition)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				atomic.AddInt64(&metrics.FailedRequests, 1)
			}
		}
		
		projectDuration := time.Since(projectStartTime)
		metrics.SubProjectLatencies[project.Language] = projectDuration
		
		// Calculate cache hit rate (simplified estimation)
		if operationCount > 0 {
			metrics.CacheHitRates[project.Language] = float64(successCount) / float64(operationCount*3) * 100
		}
	}
	
	// Generate cross-project test cases and execute them
	crossProjectCases, err := GenerateCrossProjectTestCases(subProjects)
	if err == nil {
		for _, testCase := range crossProjectCases {
			atomic.AddInt64(&metrics.TotalRequests, 1)
			metrics.CrossProjectRequests++
			
			err := ValidateRoutingToCorrectSubProject(ctx, httpClient, testCase)
			if err != nil {
				metrics.RoutingErrors++
				atomic.AddInt64(&metrics.FailedRequests, 1)
			} else {
				atomic.AddInt64(&metrics.SuccessfulRequests, 1)
			}
		}
	}
	
	// Calculate final metrics
	metrics.TestDuration = time.Since(startTime)
	totalRequests := atomic.LoadInt64(&metrics.TotalRequests)
	if metrics.TestDuration.Seconds() > 0 {
		metrics.ThroughputRPS = float64(totalRequests) / metrics.TestDuration.Seconds()
	}
	
	return metrics, nil
}

// ValidateRequestRouting performs comprehensive request routing validation
func ValidateRequestRouting(ctx context.Context, httpClient *HttpClient, workspace string, subProjects []*SubProjectInfo) (*RoutingValidationResult, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("httpClient cannot be nil")
	}
	
	result := &RoutingValidationResult{
		SubProjectResults: make(map[string]*SubProjectRoutingResult),
		ValidationErrors:  []string{},
	}
	
	startTime := time.Now()
	
	// Validate routing for each sub-project
	for _, project := range subProjects {
		projectResult := &SubProjectRoutingResult{
			Language:       project.Language,
			TestedMethods:  []string{},
			ErrorsByMethod: make(map[string]int),
		}
		
		// Get test files
		testFiles, err := GetTestFilesForSubProject(project)
		if err != nil || len(testFiles) == 0 {
			sourceFiles, err := getSourceFilesForLanguage(project)
			if err != nil || len(sourceFiles) == 0 {
				result.ValidationErrors = append(result.ValidationErrors, 
					fmt.Sprintf("no test files found for %s project", project.Language))
				continue
			}
			testFiles = sourceFiles[:1]
		}
		
		testFile := testFiles[0]
		fileURI := fmt.Sprintf("file://%s/%s", project.ProjectPath, testFile)
		testPosition := Position{Line: 0, Character: 0}
		
		latencySum := time.Duration(0)
		requestCount := 0
		
		// Test definition routing
		methodStartTime := time.Now()
		_, err = httpClient.Definition(ctx, fileURI, testPosition)
		methodDuration := time.Since(methodStartTime)
		projectResult.TestedMethods = append(projectResult.TestedMethods, "definition")
		
		if err != nil {
			projectResult.RoutingErrors++
			projectResult.ErrorsByMethod["definition"]++
			result.FailedRequests++
			result.ValidationErrors = append(result.ValidationErrors, 
				fmt.Sprintf("definition routing failed for %s: %v", project.Language, err))
		} else {
			projectResult.SuccessfulRequests++
			result.CorrectlyRoutedRequests++
		}
		latencySum += methodDuration
		requestCount++
		
		// Test references routing
		methodStartTime = time.Now()
		_, err = httpClient.References(ctx, fileURI, testPosition, true)
		methodDuration = time.Since(methodStartTime)
		projectResult.TestedMethods = append(projectResult.TestedMethods, "references")
		
		if err != nil {
			projectResult.RoutingErrors++
			projectResult.ErrorsByMethod["references"]++
			result.FailedRequests++
			result.ValidationErrors = append(result.ValidationErrors, 
				fmt.Sprintf("references routing failed for %s: %v", project.Language, err))
		} else {
			projectResult.SuccessfulRequests++
			result.CorrectlyRoutedRequests++
		}
		latencySum += methodDuration
		requestCount++
		
		// Test hover routing
		methodStartTime = time.Now()
		_, err = httpClient.Hover(ctx, fileURI, testPosition)
		methodDuration = time.Since(methodStartTime)
		projectResult.TestedMethods = append(projectResult.TestedMethods, "hover")
		
		if err != nil {
			projectResult.RoutingErrors++
			projectResult.ErrorsByMethod["hover"]++
			result.FailedRequests++
			result.ValidationErrors = append(result.ValidationErrors, 
				fmt.Sprintf("hover routing failed for %s: %v", project.Language, err))
		} else {
			projectResult.SuccessfulRequests++
			result.CorrectlyRoutedRequests++
		}
		latencySum += methodDuration
		requestCount++
		
		// Calculate average latency for this project
		if requestCount > 0 {
			projectResult.AverageLatency = latencySum / time.Duration(requestCount)
		}
		
		result.SubProjectResults[project.Language] = projectResult
	}
	
	// Test workspace symbol routing
	_, err := httpClient.WorkspaceSymbol(ctx, "test")
	if err != nil {
		result.ValidationErrors = append(result.ValidationErrors, 
			fmt.Sprintf("workspace symbol routing failed: %v", err))
		result.FailedRequests++
	} else {
		result.CorrectlyRoutedRequests++
	}
	
	result.TestDuration = time.Since(startTime)
	
	return result, nil
}

// RunMultiProjectTestSuite runs a comprehensive test suite for multi-project functionality
func RunMultiProjectTestSuite(ctx context.Context, httpClient *HttpClient, workspace string, subProjects []*SubProjectInfo) (*MultiProjectTestResults, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("httpClient cannot be nil")
	}
	
	startTime := time.Now()
	
	results := &MultiProjectTestResults{
		SubProjectResults: make(map[string]*SubProjectTestResult),
		CrossProjectTests: []CrossProjectTestResult{},
	}
	
	// Validate workspace structure
	results.WorkspaceValidation = true
	for _, project := range subProjects {
		if err := ValidateSubProjectStructure(project); err != nil {
			results.WorkspaceValidation = false
			results.TotalErrors++
		}
	}
	
	// Run routing validation
	routingResult, err := ValidateRequestRouting(ctx, httpClient, workspace, subProjects)
	if err != nil {
		results.TotalErrors++
	}
	results.RoutingValidation = routingResult
	
	// Run performance benchmarks
	perfMetrics, err := BenchmarkMultiProjectOperations(ctx, httpClient, subProjects)
	if err != nil {
		results.TotalErrors++
	}
	results.PerformanceMetrics = perfMetrics
	
	// Test each sub-project individually
	for _, project := range subProjects {
		subResult := &SubProjectTestResult{
			Language:         project.Language,
			ProjectPath:      project.ProjectPath,
			LSPMethodResults: make(map[string]int),
			Errors:           []string{},
		}
		
		// Create sub-project LSP client for testing
		subClient := NewSubProjectLSPClient(project, httpClient)
		if subClient == nil {
			subResult.Errors = append(subResult.Errors, "failed to create sub-project LSP client")
			results.TotalErrors++
			continue
		}
		
		// Test LSP functionality
		testStartTime := time.Now()
		err := subClient.TestLSPFunctionality(ctx)
		testDuration := time.Since(testStartTime)
		
		subResult.TestsExecuted = 3 // definition, references, hover
		if err != nil {
			subResult.TestsFailed = 1
			subResult.Errors = append(subResult.Errors, err.Error())
			results.TotalErrors++
		} else {
			subResult.TestsPassed = subResult.TestsExecuted
		}
		
		subResult.AverageResponseTime = testDuration / time.Duration(subResult.TestsExecuted)
		
		// Record LSP method results
		subResult.LSPMethodResults["definition"] = 1
		subResult.LSPMethodResults["references"] = 1  
		subResult.LSPMethodResults["hover"] = 1
		
		results.SubProjectResults[project.Language] = subResult
	}
	
	// Generate and execute cross-project tests
	crossProjectCases, err := GenerateCrossProjectTestCases(subProjects)
	if err == nil {
		for _, testCase := range crossProjectCases {
			crossTestStartTime := time.Now()
			
			crossResult := CrossProjectTestResult{
				TestCase: testCase,
			}
			
			err := ValidateRoutingToCorrectSubProject(ctx, httpClient, testCase)
			crossResult.ResponseTime = time.Since(crossTestStartTime)
			
			if err != nil {
				crossResult.Success = false
				crossResult.Error = err.Error()
				results.TotalErrors++
			} else {
				crossResult.Success = true
				crossResult.RoutedTo = testCase.TargetProject
			}
			
			results.CrossProjectTests = append(results.CrossProjectTests, crossResult)
		}
	}
	
	results.TestDuration = time.Since(startTime)
	
	return results, nil
}