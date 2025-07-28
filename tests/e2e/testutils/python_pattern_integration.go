package testutils

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/tests/e2e/fixtures"
)

// TestResult represents the result of an LSP test execution
type TestResult struct {
	Scenario     fixtures.PythonPatternScenario `json:"scenario"`
	Method       string                         `json:"method"`
	Success      bool                           `json:"success"`
	Error        error                          `json:"error,omitempty"`
	Duration     time.Duration                  `json:"duration"`
	ResponseData interface{}                    `json:"response_data,omitempty"`
	Position     Position                       `json:"position"`
	FileURI      string                         `json:"file_uri"`
	Metadata     map[string]interface{}         `json:"metadata,omitempty"`
}

// PythonPatternTestConfig configures the integrated test manager
type PythonPatternTestConfig struct {
	EnableRepoCloning    bool              `json:"enable_repo_cloning"`
	EnableRealFileTests  bool              `json:"enable_real_file_tests"`
	HTTPClientConfig     HttpClientConfig  `json:"http_client_config"`
	RepoConfig          PythonRepoConfig  `json:"repo_config"`
	DefaultTimeout      time.Duration     `json:"default_timeout"`
	EnableLogging       bool              `json:"enable_logging"`
	PreloadScenarios    bool              `json:"preload_scenarios"`
	EnableValidation    bool              `json:"enable_validation"`
	MaxConcurrency      int               `json:"max_concurrency"`
	FailFast            bool              `json:"fail_fast"`
}

// DefaultPythonPatternTestConfig returns a default configuration
func DefaultPythonPatternTestConfig() PythonPatternTestConfig {
	return PythonPatternTestConfig{
		EnableRepoCloning:   true,
		EnableRealFileTests: true,
		HTTPClientConfig:    DefaultHttpClientConfig(),
		RepoConfig:         DefaultPythonRepoConfig(),
		DefaultTimeout:     60 * time.Second,
		EnableLogging:      true,
		PreloadScenarios:   true,
		EnableValidation:   true,
		MaxConcurrency:     4,
		FailFast:           false,
	}
}

// PythonPatternTestManager provides comprehensive integration between repository management and HTTP testing
type PythonPatternTestManager struct {
	repoManager   *PythonRepoManager
	httpClient    *HttpClient
	scenarios     []fixtures.PythonPatternScenario
	config        PythonPatternTestConfig
	mu            sync.RWMutex
	results       []TestResult
	initialized   bool
	errorConfig   ErrorRecoveryConfig
	lastError     *PythonRepoError
	healthChecker *HealthChecker
	retryCount    int
}

// NewPythonPatternTestManager creates a new integrated test manager
func NewPythonPatternTestManager(config PythonPatternTestConfig) *PythonPatternTestManager {
	// Ensure defaults are set
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 60 * time.Second
	}
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = 4
	}

	// Initialize components
	repoManager := NewPythonRepoManager(config.RepoConfig)
	httpClient := NewHttpClient(config.HTTPClientConfig)

	ptm := &PythonPatternTestManager{
		repoManager:   repoManager,
		httpClient:    httpClient,
		config:        config,
		scenarios:     make([]fixtures.PythonPatternScenario, 0),
		results:       make([]TestResult, 0),
		initialized:   false,
		errorConfig:   DefaultErrorRecoveryConfig(),
		retryCount:    0,
	}

	// Initialize health checker after repo manager is set up
	if ptm.repoManager != nil {
		ptm.healthChecker = NewHealthChecker(ptm.repoManager.WorkspaceDir, ptm.repoManager.RepoURL)
	}

	return ptm
}

// SetupPythonPatternRepository initializes the repository and loads scenarios with comprehensive error handling
func (ptm *PythonPatternTestManager) SetupPythonPatternRepository() error {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	if ptm.config.EnableLogging {
		fmt.Printf("[PythonPatternTestManager] Starting repository setup\n")
	}

	// Execute setup with retry and recovery
	return ExecuteWithRetry(func() error {
		// Network connectivity check first
		if ptm.config.EnableRepoCloning {
			if err := ptm.checkNetworkConnectivity(); err != nil {
				ptm.lastError = NewPythonRepoError(ErrorTypeNetwork, "network_check", err)
				LogErrorWithContext(ptm.lastError, CreateErrorContext("network_check", "repo_url", ptm.repoManager.RepoURL))
				return ptm.lastError
			}
		}

		// Clone repository if enabled
		if ptm.config.EnableRepoCloning {
			if err := ptm.setupRepositoryWithRecovery(); err != nil {
				return fmt.Errorf("failed to setup repository: %w", err)
			}
		}

		// Validate repository structure
		if err := ptm.validateRepositoryWithRecovery(); err != nil {
			return fmt.Errorf("repository validation failed: %w", err)
		}

		// Load scenarios if enabled
		if ptm.config.PreloadScenarios {
			if err := ptm.loadScenariosWithValidation(); err != nil {
				return fmt.Errorf("failed to load scenarios: %w", err)
			}
		}

		// Validate HTTP client connection with retry
		if err := ptm.validateHTTPClientWithRetry(); err != nil {
			return fmt.Errorf("HTTP client validation failed: %w", err)
		}

		ptm.initialized = true
		
		if ptm.config.EnableLogging {
			fmt.Printf("[PythonPatternTestManager] Setup completed successfully\n")
		}

		return nil
	}, ptm.errorConfig.MaxRetries, ptm.errorConfig.RetryDelay)
}

// checkNetworkConnectivity validates network connectivity before attempting operations
func (ptm *PythonPatternTestManager) checkNetworkConnectivity() error {
	if ptm.healthChecker == nil {
		return fmt.Errorf("health checker not initialized")
	}

	return WrapWithErrorHandling("network_connectivity_check", func() error {
		return ptm.healthChecker.CheckNetworkConnectivity()
	})
}

// setupRepositoryWithRecovery sets up the repository with error recovery
func (ptm *PythonPatternTestManager) setupRepositoryWithRecovery() error {
	return WrapWithErrorHandling("repository_setup", func() error {
		return ptm.repoManager.CloneRepository()
	})
}

// validateRepositoryWithRecovery validates repository with recovery mechanisms
func (ptm *PythonPatternTestManager) validateRepositoryWithRecovery() error {
	return WrapWithErrorHandling("repository_validation", func() error {
		if err := ptm.ValidatePythonPatternsRepository(ptm.repoManager.WorkspaceDir); err != nil {
			// Attempt repository repair if validation fails
			if repairErr := ptm.repairRepositoryStructure(); repairErr != nil {
				return fmt.Errorf("validation failed and repair unsuccessful: %w, repair error: %v", err, repairErr)
			}
			
			// Re-validate after repair
			if revalidateErr := ptm.ValidatePythonPatternsRepository(ptm.repoManager.WorkspaceDir); revalidateErr != nil {
				return fmt.Errorf("validation failed after repair: %w", revalidateErr)
			}
		}
		return nil
	})
}

// repairRepositoryStructure attempts to repair missing repository structure
func (ptm *PythonPatternTestManager) repairRepositoryStructure() error {
	if ptm.config.EnableLogging {
		fmt.Printf("[PythonPatternTestManager] Attempting repository structure repair\n")
	}

	strategy := &RepositoryRecoveryStrategy{}
	errorCtx := &PythonRepoError{
		Type:    ErrorTypeRepository,
		Context: CreateErrorContext("repair", "workspace_dir", ptm.repoManager.WorkspaceDir),
	}

	return strategy.Recover(errorCtx)
}

// validateHTTPClientWithRetry validates HTTP client connection with retry logic
func (ptm *PythonPatternTestManager) validateHTTPClientWithRetry() error {
	return RetryWithBackoff(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), ptm.config.DefaultTimeout)
		defer cancel()
		
		return ptm.httpClient.ValidateConnection(ctx)
	}, ptm.errorConfig)
}

// loadScenariosWithValidation loads all available Python pattern scenarios with validation
func (ptm *PythonPatternTestManager) loadScenariosWithValidation() error {
	return WrapWithErrorHandling("load_scenarios", func() error {
		var allScenarios []fixtures.PythonPatternScenario

		// Load all scenario types
		allScenarios = append(allScenarios, fixtures.GetCreationalPatternScenarios()...)
		allScenarios = append(allScenarios, fixtures.GetStructuralPatternScenarios()...)
		allScenarios = append(allScenarios, fixtures.GetBehavioralPatternScenarios()...)

		// Validate scenario file paths exist in repository
		repoPath := filepath.Join(ptm.repoManager.WorkspaceDir, "python-patterns")
		validScenarios := make([]fixtures.PythonPatternScenario, 0)
		missingFiles := make([]string, 0)

		for _, scenario := range allScenarios {
			fullPath := filepath.Join(repoPath, scenario.FilePath)
			if _, err := os.Stat(fullPath); err == nil {
				validScenarios = append(validScenarios, scenario)
			} else {
				missingFiles = append(missingFiles, scenario.FilePath)
				if ptm.config.EnableLogging {
					fmt.Printf("[PythonPatternTestManager] Warning: scenario file not found: %s\n", fullPath)
				}
			}
		}

		if len(validScenarios) == 0 {
			return fmt.Errorf("no valid scenarios found - all scenario files missing: %v", missingFiles)
		}

		if len(missingFiles) > len(validScenarios) {
			return fmt.Errorf("too many missing scenario files (%d missing, %d valid) - repository may be corrupted", len(missingFiles), len(validScenarios))
		}

		ptm.scenarios = validScenarios

		if ptm.config.EnableLogging {
			fmt.Printf("[PythonPatternTestManager] Loaded %d valid scenarios (%d missing)\n", len(ptm.scenarios), len(missingFiles))
		}

		return nil
	})
}

// loadScenarios loads all available Python pattern scenarios (backward compatibility)
func (ptm *PythonPatternTestManager) loadScenarios() error {
	return ptm.loadScenariosWithValidation()
}

// GetRepositoryFiles returns all Python files in the repository
func (ptm *PythonPatternTestManager) GetRepositoryFiles() ([]string, error) {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	if !ptm.initialized {
		return nil, fmt.Errorf("manager not initialized, call SetupPythonPatternRepository first")
	}

	repoPath := filepath.Join(ptm.repoManager.WorkspaceDir, "python-patterns")
	var files []string

	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".py") {
			relPath, err := filepath.Rel(repoPath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	return files, err
}

// GetFileContent reads content from a file in the repository
func (ptm *PythonPatternTestManager) GetFileContent(filePath string) (string, error) {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	if !ptm.initialized {
		return "", fmt.Errorf("manager not initialized, call SetupPythonPatternRepository first")
	}

	fullPath := filepath.Join(ptm.repoManager.WorkspaceDir, "python-patterns", filePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return string(content), nil
}

// ExecuteScenarioTest runs a complete test scenario against the HTTP client with comprehensive error handling
func (ptm *PythonPatternTestManager) ExecuteScenarioTest(scenario fixtures.PythonPatternScenario) error {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	if !ptm.initialized {
		ptm.lastError = NewPythonRepoError(ErrorTypeValidation, "scenario_execution", fmt.Errorf("manager not initialized"))
		return ptm.lastError
	}

	// Pre-execution health check
	if err := ptm.preExecutionHealthCheck(scenario); err != nil {
		ptm.lastError = NewPythonRepoError(ErrorTypeValidation, "pre_execution_check", err)
		LogErrorWithContext(ptm.lastError, CreateErrorContext("pre_execution_check", "scenario", scenario.Name))
		return ptm.lastError
	}

	return ExecuteWithRetry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), ptm.config.DefaultTimeout)
		defer cancel()

		// Validate file exists with recovery
		fileURI, err := ptm.GetPatternFileURIWithValidation(scenario.FilePath)
		if err != nil {
			return fmt.Errorf("failed to get file URI for scenario %s: %w", scenario.Name, err)
		}

		// Execute all specified LSP methods for each test position
		for _, position := range scenario.TestPositions {
			for _, method := range scenario.LSPMethods {
				if err := ptm.executeSingleLSPMethod(ctx, scenario, position, method, fileURI); err != nil {
					if ptm.config.FailFast {
						return fmt.Errorf("scenario %s failed on method %s: %w", scenario.Name, method, err)
					}
					// Continue with other methods if not fail-fast
				}
			}
		}

		return nil
	}, ptm.errorConfig.MaxRetries, ptm.errorConfig.RetryDelay)
}

// preExecutionHealthCheck performs health checks before scenario execution
func (ptm *PythonPatternTestManager) preExecutionHealthCheck(scenario fixtures.PythonPatternScenario) error {
	// Check repository health
	if ptm.healthChecker != nil {
		if err := ptm.healthChecker.CheckRepositoryHealth(); err != nil {
			return fmt.Errorf("repository health check failed: %w", err)
		}
	}

	// Validate HTTP client is responsive
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := ptm.httpClient.ValidateConnection(ctx); err != nil {
		return fmt.Errorf("HTTP client health check failed: %w", err)
	}

	// Basic scenario validation
	if scenario.Name == "" {
		return fmt.Errorf("scenario name is empty")
	}

	if scenario.FilePath == "" {
		return fmt.Errorf("scenario file path is empty")
	}

	if len(scenario.LSPMethods) == 0 {
		return fmt.Errorf("no LSP methods specified for scenario")
	}

	return nil
}

// executeSingleLSPMethod executes a single LSP method with error handling
func (ptm *PythonPatternTestManager) executeSingleLSPMethod(ctx context.Context, scenario fixtures.PythonPatternScenario, position fixtures.PythonTestPosition, method, fileURI string) error {
	result := TestResult{
		Scenario: scenario,
		Method:   method,
		Position: ptm.ConvertPythonTestPositionToPosition(position),
		FileURI:  fileURI,
		Metadata: make(map[string]interface{}),
	}

	startTime := time.Now()
	var err error

	// Execute with retry for transient errors
	err = ExecuteWithRetry(func() error {
		switch method {
		case "textDocument/definition":
			return ptm.TestDefinitionForScenario(ctx, scenario, position)
		case "textDocument/references":
			return ptm.TestReferencesForScenario(ctx, scenario, position)
		case "textDocument/hover":
			return ptm.TestHoverForScenario(ctx, scenario, position)
		case "textDocument/documentSymbol":
			return ptm.TestDocumentSymbolsForScenario(ctx, scenario)
		case "workspace/symbol":
			return ptm.TestWorkspaceSymbolsForScenario(ctx, scenario)
		case "textDocument/completion":
			return ptm.TestCompletionForScenario(ctx, scenario, position)
		default:
			return fmt.Errorf("unsupported LSP method: %s", method)
		}
	}, 2, 1*time.Second) // Limited retries for individual methods

	result.Duration = time.Since(startTime)
	result.Success = err == nil
	result.Error = err

	// Add error context to metadata
	if err != nil {
		result.Metadata["error_type"] = string(ClassifyError(err))
		result.Metadata["recoverable"] = IsRecoverableError(err)
		result.Metadata["retry_count"] = ptm.retryCount
	}

	ptm.results = append(ptm.results, result)
	return err
}

// TestDefinitionForScenario executes textDocument/definition test
func (ptm *PythonPatternTestManager) TestDefinitionForScenario(ctx context.Context, scenario fixtures.PythonPatternScenario, position fixtures.PythonTestPosition) error {
	fileURI, err := ptm.GetPatternFileURI(scenario.FilePath)
	if err != nil {
		return err
	}

	lspPosition := ptm.ConvertPythonTestPositionToPosition(position)
	locations, err := ptm.httpClient.Definition(ctx, fileURI, lspPosition)
	if err != nil {
		return fmt.Errorf("definition request failed: %w", err)
	}

	if ptm.config.EnableValidation {
		return ptm.validateDefinitionResults(scenario, position, locations)
	}

	return nil
}

// TestReferencesForScenario executes textDocument/references test
func (ptm *PythonPatternTestManager) TestReferencesForScenario(ctx context.Context, scenario fixtures.PythonPatternScenario, position fixtures.PythonTestPosition) error {
	fileURI, err := ptm.GetPatternFileURI(scenario.FilePath)
	if err != nil {
		return err
	}

	lspPosition := ptm.ConvertPythonTestPositionToPosition(position)
	locations, err := ptm.httpClient.References(ctx, fileURI, lspPosition, true)
	if err != nil {
		return fmt.Errorf("references request failed: %w", err)
	}

	if ptm.config.EnableValidation {
		return ptm.validateReferencesResults(scenario, position, locations)
	}

	return nil
}

// TestHoverForScenario executes textDocument/hover test
func (ptm *PythonPatternTestManager) TestHoverForScenario(ctx context.Context, scenario fixtures.PythonPatternScenario, position fixtures.PythonTestPosition) error {
	fileURI, err := ptm.GetPatternFileURI(scenario.FilePath)
	if err != nil {
		return err
	}

	lspPosition := ptm.ConvertPythonTestPositionToPosition(position)
	hoverResult, err := ptm.httpClient.Hover(ctx, fileURI, lspPosition)
	if err != nil {
		return fmt.Errorf("hover request failed: %w", err)
	}

	if ptm.config.EnableValidation {
		return ptm.validateHoverResults(scenario, position, hoverResult)
	}

	return nil
}

// TestDocumentSymbolsForScenario executes textDocument/documentSymbol test
func (ptm *PythonPatternTestManager) TestDocumentSymbolsForScenario(ctx context.Context, scenario fixtures.PythonPatternScenario) error {
	fileURI, err := ptm.GetPatternFileURI(scenario.FilePath)
	if err != nil {
		return err
	}

	symbols, err := ptm.httpClient.DocumentSymbol(ctx, fileURI)
	if err != nil {
		return fmt.Errorf("document symbol request failed: %w", err)
	}

	if ptm.config.EnableValidation {
		return ptm.validateDocumentSymbolResults(scenario, symbols)
	}

	return nil
}

// TestWorkspaceSymbolsForScenario executes workspace/symbol test
func (ptm *PythonPatternTestManager) TestWorkspaceSymbolsForScenario(ctx context.Context, scenario fixtures.PythonPatternScenario) error {
	// Use first expected symbol as query if available
	query := scenario.Name
	if len(scenario.ExpectedSymbols) > 0 {
		query = scenario.ExpectedSymbols[0].Name
	}

	symbols, err := ptm.httpClient.WorkspaceSymbol(ctx, query)
	if err != nil {
		return fmt.Errorf("workspace symbol request failed: %w", err)
	}

	if ptm.config.EnableValidation {
		return ptm.validateWorkspaceSymbolResults(scenario, symbols)
	}

	return nil
}

// TestCompletionForScenario executes textDocument/completion test
func (ptm *PythonPatternTestManager) TestCompletionForScenario(ctx context.Context, scenario fixtures.PythonPatternScenario, position fixtures.PythonTestPosition) error {
	fileURI, err := ptm.GetPatternFileURI(scenario.FilePath)
	if err != nil {
		return err
	}

	lspPosition := ptm.ConvertPythonTestPositionToPosition(position)
	completions, err := ptm.httpClient.Completion(ctx, fileURI, lspPosition)
	if err != nil {
		return fmt.Errorf("completion request failed: %w", err)
	}

	if ptm.config.EnableValidation {
		return ptm.validateCompletionResults(scenario, position, completions)
	}

	return nil
}

// ConvertToFileURI converts a repository-relative path to a file:// URI
func (ptm *PythonPatternTestManager) ConvertToFileURI(repoPath string) string {
	fullPath := filepath.Join(ptm.repoManager.WorkspaceDir, "python-patterns", repoPath)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		// Fallback to the original path if absolute path fails
		absPath = fullPath
	}

	// Convert to proper file URI
	u := &url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(absPath),
	}

	return u.String()
}

// GetPatternFileURIWithValidation returns the file URI for a pattern file path with validation
func (ptm *PythonPatternTestManager) GetPatternFileURIWithValidation(patternPath string) (string, error) {
	if patternPath == "" {
		return "", fmt.Errorf("pattern path cannot be empty")
	}

	// Validate file exists before creating URI
	if err := ptm.ValidateFileExists(patternPath); err != nil {
		return "", fmt.Errorf("file validation failed for %s: %w", patternPath, err)
	}

	return ptm.ConvertToFileURI(patternPath), nil
}

// GetPatternFileURI returns the file URI for a pattern file path (backward compatibility)
func (ptm *PythonPatternTestManager) GetPatternFileURI(patternPath string) (string, error) {
	return ptm.GetPatternFileURIWithValidation(patternPath)
}

// ValidateFileExists validates that a file exists in the repository
func (ptm *PythonPatternTestManager) ValidateFileExists(filePath string) error {
	fullPath := filepath.Join(ptm.repoManager.WorkspaceDir, "python-patterns", filePath)
	
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", filePath)
		}
		return fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	if info.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	return nil
}

// ConvertPythonTestPositionToPosition converts PythonTestPosition to Position
func (ptm *PythonPatternTestManager) ConvertPythonTestPositionToPosition(position fixtures.PythonTestPosition) Position {
	return Position{
		Line:      position.Line,
		Character: position.Character,
	}
}

// ConvertScenarioToLSPParams converts scenario data to LSP request parameters
func (ptm *PythonPatternTestManager) ConvertScenarioToLSPParams(scenario fixtures.PythonPatternScenario, position fixtures.PythonTestPosition) map[string]interface{} {
	fileURI, _ := ptm.GetPatternFileURI(scenario.FilePath)
	
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": map[string]interface{}{
			"line":      position.Line,
			"character": position.Character,
		},
	}
}

// CreateTextDocumentPositionParams creates LSP textDocument/position parameters
func (ptm *PythonPatternTestManager) CreateTextDocumentPositionParams(fileURI string, position fixtures.PythonTestPosition) map[string]interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": map[string]interface{}{
			"line":      position.Line,
			"character": position.Character,
		},
	}
}

// CreateReferenceParams creates LSP textDocument/references parameters
func (ptm *PythonPatternTestManager) CreateReferenceParams(fileURI string, position fixtures.PythonTestPosition) map[string]interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": map[string]interface{}{
			"line":      position.Line,
			"character": position.Character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	}
}

// CreateWorkspaceSymbolParams creates LSP workspace/symbol parameters
func (ptm *PythonPatternTestManager) CreateWorkspaceSymbolParams(query string) map[string]interface{} {
	return map[string]interface{}{
		"query": query,
	}
}

// PrepareTestSuite prepares the manager for testify/suite integration
func (ptm *PythonPatternTestManager) PrepareTestSuite() error {
	if ptm.initialized {
		return nil
	}

	return ptm.SetupPythonPatternRepository()
}

// ValidateScenarioResults validates the results of scenario execution
func (ptm *PythonPatternTestManager) ValidateScenarioResults(results []TestResult) error {
	if len(results) == 0 {
		return fmt.Errorf("no test results to validate")
	}

	failedCount := 0
	for _, result := range results {
		if !result.Success {
			failedCount++
			if ptm.config.EnableLogging {
				fmt.Printf("[PythonPatternTestManager] Failed test: %s.%s - %v\n", 
					result.Scenario.Name, result.Method, result.Error)
			}
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("%d out of %d tests failed", failedCount, len(results))
	}

	return nil
}

// CleanupTestSuite performs cleanup after test execution
func (ptm *PythonPatternTestManager) CleanupTestSuite() error {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	var errs []error

	// Close HTTP client
	if ptm.httpClient != nil {
		if err := ptm.httpClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close HTTP client: %w", err))
		}
	}

	// Cleanup repository
	if ptm.repoManager != nil && ptm.repoManager.CleanupFunc != nil {
		if err := ptm.repoManager.CleanupFunc(); err != nil {
			errs = append(errs, fmt.Errorf("failed to cleanup repository: %w", err))
		}
	}

	// Clear results
	ptm.results = nil
	ptm.scenarios = nil
	ptm.initialized = false

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}

// GetScenarios returns loaded scenarios
func (ptm *PythonPatternTestManager) GetScenarios() []fixtures.PythonPatternScenario {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()
	
	return ptm.scenarios
}

// GetResults returns test execution results
func (ptm *PythonPatternTestManager) GetResults() []TestResult {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()
	
	return ptm.results
}

// GetHttpClient returns the underlying HTTP client for advanced usage
func (ptm *PythonPatternTestManager) GetHttpClient() *HttpClient {
	return ptm.httpClient
}

// GetRepoManager returns the underlying repository manager for advanced usage
func (ptm *PythonPatternTestManager) GetRepoManager() *PythonRepoManager {
	return ptm.repoManager
}

// GetLastError returns the last error encountered during operations
func (ptm *PythonPatternTestManager) GetLastError() *PythonRepoError {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()
	return ptm.lastError
}

// RecoverFromError attempts to recover from the last encountered error
func (ptm *PythonPatternTestManager) RecoverFromError() error {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	if ptm.lastError == nil {
		return fmt.Errorf("no error to recover from")
	}

	config := DefaultErrorRecoveryConfig()
	strategy, exists := config.FallbackStrategies[ptm.lastError.Type]
	if !exists {
		return fmt.Errorf("no recovery strategy available for error type: %s", ptm.lastError.Type)
	}

	if !strategy.CanRecover(ptm.lastError) {
		return fmt.Errorf("error is not recoverable: %s", ptm.lastError.Message)
	}

	if ptm.config.EnableLogging {
		fmt.Printf("[PythonPatternTestManager] Attempting recovery from error: %s\n", ptm.lastError.Message)
	}
	
	if err := strategy.Recover(ptm.lastError); err != nil {
		return fmt.Errorf("recovery failed: %w", err)
	}

	if ptm.config.EnableLogging {
		fmt.Printf("[PythonPatternTestManager] Recovery completed successfully\n")
	}
	
	ptm.lastError = nil
	return nil
}

// ValidateSystemHealth performs comprehensive system health checks
func (ptm *PythonPatternTestManager) ValidateSystemHealth() error {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	if !ptm.initialized {
		return fmt.Errorf("manager not initialized")
	}

	var healthErrors []error

	// Check repository health
	if ptm.healthChecker != nil {
		if err := ptm.healthChecker.CheckRepositoryHealth(); err != nil {
			healthErrors = append(healthErrors, fmt.Errorf("repository health: %w", err))
		}

		if err := ptm.healthChecker.CheckNetworkConnectivity(); err != nil {
			healthErrors = append(healthErrors, fmt.Errorf("network connectivity: %w", err))
		}
	}

	// Check HTTP client health
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	if err := ptm.httpClient.ValidateConnection(ctx); err != nil {
		healthErrors = append(healthErrors, fmt.Errorf("HTTP client: %w", err))
	}

	// Check scenarios availability
	if len(ptm.scenarios) == 0 {
		healthErrors = append(healthErrors, fmt.Errorf("no scenarios loaded"))
	}

	// Check workspace accessibility
	if _, err := os.Stat(ptm.repoManager.WorkspaceDir); err != nil {
		healthErrors = append(healthErrors, fmt.Errorf("workspace inaccessible: %w", err))
	}

	if len(healthErrors) > 0 {
		return fmt.Errorf("system health check failed: %v", healthErrors)
	}

	if ptm.config.EnableLogging {
		fmt.Printf("[PythonPatternTestManager] System health check passed\n")
	}

	return nil
}

// ExecuteScenarioTestWithRecovery executes a scenario test with automatic error recovery
func (ptm *PythonPatternTestManager) ExecuteScenarioTestWithRecovery(scenario fixtures.PythonPatternScenario) error {
	maxRecoveryAttempts := 2
	
	for attempt := 0; attempt <= maxRecoveryAttempts; attempt++ {
		err := ptm.ExecuteScenarioTest(scenario)
		if err == nil {
			return nil
		}

		if attempt < maxRecoveryAttempts {
			if ptm.config.EnableLogging {
				fmt.Printf("[PythonPatternTestManager] Scenario execution failed (attempt %d), attempting recovery: %v\n", attempt+1, err)
			}

			// Attempt recovery
			if recoveryErr := ptm.RecoverFromError(); recoveryErr != nil {
				if ptm.config.EnableLogging {
					fmt.Printf("[PythonPatternTestManager] Recovery failed: %v\n", recoveryErr)
				}
				continue
			}

			// Wait before retry
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
		}
	}

	return fmt.Errorf("scenario execution failed after %d recovery attempts", maxRecoveryAttempts+1)
}

// CleanupTestSuiteWithRecovery performs cleanup with enhanced error recovery
func (ptm *PythonPatternTestManager) CleanupTestSuiteWithRecovery() error {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()

	var cleanupErrors []error

	// Close HTTP client with retry
	if ptm.httpClient != nil {
		err := ExecuteWithRetry(func() error {
			return ptm.httpClient.Close()
		}, 3, 1*time.Second)
		
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to close HTTP client: %w", err))
		}
	}

	// Cleanup repository with force cleanup fallback
	if ptm.repoManager != nil {
		if ptm.repoManager.CleanupFunc != nil {
			if err := ptm.repoManager.CleanupFunc(); err != nil {
				// Try force cleanup as fallback
				if forceErr := ptm.repoManager.ForceCleanup(); forceErr != nil {
					cleanupErrors = append(cleanupErrors, fmt.Errorf("failed to cleanup repository (normal: %w, force: %v)", err, forceErr))
				} else if ptm.config.EnableLogging {
					fmt.Printf("[PythonPatternTestManager] Force cleanup succeeded after normal cleanup failed\n")
				}
			}
		}
	}

	// Clear state
	ptm.results = nil
	ptm.scenarios = nil
	ptm.initialized = false
	ptm.lastError = nil
	ptm.retryCount = 0

	if len(cleanupErrors) > 0 {
		return fmt.Errorf("cleanup completed with errors: %v", cleanupErrors)
	}

	if ptm.config.EnableLogging {
		fmt.Printf("[PythonPatternTestManager] Cleanup completed successfully\n")
	}

	return nil
}

// GetErrorStatistics returns statistics about errors encountered during testing
func (ptm *PythonPatternTestManager) GetErrorStatistics() map[string]interface{} {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_tests": len(ptm.results),
		"successful_tests": 0,
		"failed_tests": 0,
		"error_types": make(map[string]int),
		"recoverable_errors": 0,
		"retry_count": ptm.retryCount,
		"last_error": nil,
	}

	for _, result := range ptm.results {
		if result.Success {
			stats["successful_tests"] = stats["successful_tests"].(int) + 1
		} else {
			stats["failed_tests"] = stats["failed_tests"].(int) + 1
			
			if result.Error != nil {
				errorType := string(ClassifyError(result.Error))
				if count, exists := stats["error_types"].(map[string]int)[errorType]; exists {
					stats["error_types"].(map[string]int)[errorType] = count + 1
				} else {
					stats["error_types"].(map[string]int)[errorType] = 1
				}

				if IsRecoverableError(result.Error) {
					stats["recoverable_errors"] = stats["recoverable_errors"].(int) + 1
				}
			}
		}
	}

	if ptm.lastError != nil {
		stats["last_error"] = map[string]interface{}{
			"type": string(ptm.lastError.Type),
			"operation": ptm.lastError.Operation,
			"message": ptm.lastError.Message,
			"recoverable": ptm.lastError.Recoverable,
			"retry_count": ptm.lastError.RetryCount,
			"timestamp": ptm.lastError.Timestamp,
		}
	}

	return stats
}

// SetErrorRecoveryConfig allows customization of error recovery behavior
func (ptm *PythonPatternTestManager) SetErrorRecoveryConfig(config ErrorRecoveryConfig) {
	ptm.mu.Lock()
	defer ptm.mu.Unlock()
	ptm.errorConfig = config
}

// GetErrorRecoveryConfig returns the current error recovery configuration
func (ptm *PythonPatternTestManager) GetErrorRecoveryConfig() ErrorRecoveryConfig {
	ptm.mu.RLock()
	defer ptm.mu.RUnlock()
	return ptm.errorConfig
}

// Helper utility functions

// GetProjectRootForPythonPatterns returns the project root directory for Python pattern tests
func GetProjectRootForPythonPatterns() (string, error) {
	return GetProjectRoot()
}

// CreateTempPythonConfig creates a temporary configuration for testing
func CreateTempPythonConfig(repoManager *PythonRepoManager) (string, error) {
	tempDir := repoManager.WorkspaceDir
	configPath := filepath.Join(tempDir, "python-test-config.yaml")
	
	configContent := fmt.Sprintf(`
workspace_root: %s
python:
  enabled: true
  server_command: ["pylsp"]
  initialization_options: {}
`, tempDir)

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create temp config: %w", err)
	}

	return configPath, nil
}

// SetupPythonEnvironmentForTesting sets up Python environment variables
func SetupPythonEnvironmentForTesting() error {
	// Set PYTHONPATH to include the patterns directory
	currentPath := os.Getenv("PYTHONPATH")
	if currentPath != "" {
		return nil // Already configured
	}

	// This is a placeholder - in real scenarios, this would set up the Python environment
	return nil
}

// ValidatePythonPatternsRepository validates the repository structure
func (ptm *PythonPatternTestManager) ValidatePythonPatternsRepository(repoPath string) error {
	pythonPatternsDir := filepath.Join(repoPath, "python-patterns")
	
	// Check if directory exists
	info, err := os.Stat(pythonPatternsDir)
	if err != nil {
		return fmt.Errorf("python-patterns directory not found at %s: %w", pythonPatternsDir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("python-patterns is not a directory: %s", pythonPatternsDir)
	}

	// Check for expected pattern directories
	expectedDirs := []string{"patterns", "patterns/creational", "patterns/structural", "patterns/behavioral"}
	for _, expectedDir := range expectedDirs {
		dirPath := filepath.Join(pythonPatternsDir, expectedDir)
		if _, err := os.Stat(dirPath); err != nil {
			return fmt.Errorf("expected directory not found: %s", expectedDir)
		}
	}

	return nil
}

// Validation helper methods

func (ptm *PythonPatternTestManager) validateDefinitionResults(scenario fixtures.PythonPatternScenario, position fixtures.PythonTestPosition, locations []Location) error {
	if len(locations) == 0 {
		return fmt.Errorf("no definitions found for symbol %s", position.Symbol)
	}

	// Basic validation - ensure we got some results
	for _, location := range locations {
		if location.URI == "" {
			return fmt.Errorf("empty URI in definition result")
		}
	}

	return nil
}

func (ptm *PythonPatternTestManager) validateReferencesResults(scenario fixtures.PythonPatternScenario, position fixtures.PythonTestPosition, locations []Location) error {
	if len(locations) == 0 {
		return fmt.Errorf("no references found for symbol %s", position.Symbol)
	}

	return nil
}

func (ptm *PythonPatternTestManager) validateHoverResults(scenario fixtures.PythonPatternScenario, position fixtures.PythonTestPosition, hover *HoverResult) error {
	if hover == nil {
		return fmt.Errorf("no hover information found for symbol %s", position.Symbol)
	}

	if hover.Contents == nil {
		return fmt.Errorf("empty hover contents for symbol %s", position.Symbol)
	}

	return nil
}

func (ptm *PythonPatternTestManager) validateDocumentSymbolResults(scenario fixtures.PythonPatternScenario, symbols []DocumentSymbol) error {
	if len(symbols) == 0 {
		return fmt.Errorf("no document symbols found")
	}

	// Check if expected symbols are present
	expectedSymbolNames := make(map[string]bool)
	for _, expectedSymbol := range scenario.ExpectedSymbols {
		expectedSymbolNames[expectedSymbol.Name] = false
	}

	// Mark found symbols
	for _, symbol := range symbols {
		if _, exists := expectedSymbolNames[symbol.Name]; exists {
			expectedSymbolNames[symbol.Name] = true
		}
	}

	// Check for missing symbols
	for name, found := range expectedSymbolNames {
		if !found {
			return fmt.Errorf("expected symbol not found: %s", name)
		}
	}

	return nil
}

func (ptm *PythonPatternTestManager) validateWorkspaceSymbolResults(scenario fixtures.PythonPatternScenario, symbols []SymbolInformation) error {
	if len(symbols) == 0 {
		return fmt.Errorf("no workspace symbols found")
	}

	return nil
}

func (ptm *PythonPatternTestManager) validateCompletionResults(scenario fixtures.PythonPatternScenario, position fixtures.PythonTestPosition, completions *CompletionList) error {
	if completions == nil {
		return fmt.Errorf("no completion results")
	}

	if len(completions.Items) == 0 {
		return fmt.Errorf("no completion items found")
	}

	return nil
}