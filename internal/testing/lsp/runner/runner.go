package runner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/client"
	"lsp-gateway/internal/testing/lsp/config"
)

// TestRunner orchestrates the execution of LSP test suites
type TestRunner struct {
	config        *config.LSPTestConfig
	serverManager *client.LSPServerManager
	communicator  *client.LSPCommunicator
	fileManager   *TestFileManager
	logger        TestLogger

	// Runtime state
	startTime      time.Time
	endTime        time.Time
	testSuites     []*cases.TestSuite
	totalTests     int
	completedTests int

	// Concurrency control
	semaphore chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex

	// Results
	results *cases.TestResult
}

// TestLogger provides logging functionality for the test runner
type TestLogger interface {
	Debug(format string, args ...interface{})
	Info(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})

	WithFields(fields map[string]interface{}) TestLogger
	WithTestCase(testCase *cases.TestCase) TestLogger
	WithTestSuite(testSuite *cases.TestSuite) TestLogger
}

// TestFileManager manages test files and workspaces
type TestFileManager struct {
	workspaces map[string]string
	mu         sync.RWMutex
}

// RunOptions configures test execution
type RunOptions struct {
	Filter         *cases.TestCaseFilter
	DryRun         bool
	Verbose        bool
	FailFast       bool
	MaxConcurrency int
	Timeout        time.Duration
	OutputDir      string

	// File management
	CleanupWorkspaces  bool
	PreserveWorkspaces bool
}

// DefaultRunOptions returns default run options
func DefaultRunOptions() *RunOptions {
	return &RunOptions{
		Filter:             &cases.TestCaseFilter{IncludeSkipped: false},
		DryRun:             false,
		Verbose:            false,
		FailFast:           false,
		MaxConcurrency:     4,
		Timeout:            5 * time.Minute,
		OutputDir:          "test-results",
		CleanupWorkspaces:  true,
		PreserveWorkspaces: false,
	}
}

// NewTestRunner creates a new test runner
func NewTestRunner(config *config.LSPTestConfig, logger TestLogger) (*TestRunner, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	serverManager := client.NewLSPServerManager(config)
	communicator := client.NewLSPCommunicator(serverManager)
	fileManager := NewTestFileManager()

	runner := &TestRunner{
		config:        config,
		serverManager: serverManager,
		communicator:  communicator,
		fileManager:   fileManager,
		logger:        logger,
		testSuites:    make([]*cases.TestSuite, 0),
	}

	return runner, nil
}

// NewTestFileManager creates a new test file manager
func NewTestFileManager() *TestFileManager {
	return &TestFileManager{
		workspaces: make(map[string]string),
	}
}

// LoadTestSuites loads test suites from configuration
func (r *TestRunner) LoadTestSuites() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, repoConfig := range r.config.Repositories {
		// Find appropriate server configuration
		serverConfig, err := r.getServerConfigForLanguage(repoConfig.Language)
		if err != nil {
			r.logger.Warn("No server configuration found for repository %s (language: %s): %v",
				repoConfig.Name, repoConfig.Language, err)
			continue
		}

		testSuite := cases.NewTestSuite(repoConfig.Name, repoConfig, serverConfig)

		// Load test cases
		for _, testCaseConfig := range repoConfig.TestCases {
			testCaseID := fmt.Sprintf("%s:%s", repoConfig.Name, testCaseConfig.ID)
			testCase := cases.NewTestCase(testCaseID, repoConfig, testCaseConfig)

			testSuite.AddTestCase(testCase)
		}

		r.testSuites = append(r.testSuites, testSuite)
		r.totalTests += len(testSuite.TestCases)
	}

	r.logger.Info("Loaded %d test suites with %d total test cases", len(r.testSuites), r.totalTests)
	return nil
}

// getServerConfigForLanguage finds the appropriate server config for a language
func (r *TestRunner) getServerConfigForLanguage(language string) (*config.ServerConfig, error) {
	for _, serverConfig := range r.config.Servers {
		if serverConfig.Language == language {
			return serverConfig, nil
		}
	}
	return nil, fmt.Errorf("no server configuration found for language: %s", language)
}

// Run executes all test suites
func (r *TestRunner) Run(ctx context.Context, options *RunOptions) (*cases.TestResult, error) {
	if options == nil {
		options = DefaultRunOptions()
	}

	r.startTime = time.Now()
	r.logger.Info("Starting LSP test run with %d test suites", len(r.testSuites))

	// Initialize concurrency control
	maxConcurrency := options.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = r.config.Execution.MaxConcurrency
	}
	r.semaphore = make(chan struct{}, maxConcurrency)

	// Setup context with timeout
	runCtx := ctx
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// Initialize results
	r.results = &cases.TestResult{
		TestSuites: r.testSuites,
		StartTime:  r.startTime,
	}

	defer func() {
		r.endTime = time.Now()
		r.results.EndTime = r.endTime
		r.results.Duration = r.endTime.Sub(r.startTime)
		r.updateFinalResults()
	}()

	// Setup workspaces
	if err := r.setupWorkspaces(runCtx); err != nil {
		return nil, fmt.Errorf("failed to setup workspaces: %w", err)
	}

	defer func() {
		if options.CleanupWorkspaces && !options.PreserveWorkspaces {
			r.cleanupWorkspaces(context.Background())
		}
	}()

	// Execute test suites
	if options.DryRun {
		return r.dryRun(runCtx, options)
	}

	return r.executeTestSuites(runCtx, options)
}

// setupWorkspaces prepares test workspaces
func (r *TestRunner) setupWorkspaces(ctx context.Context) error {
	r.logger.Info("Setting up test workspaces...")

	for _, testSuite := range r.testSuites {
		workspaceDir, err := r.fileManager.CreateWorkspace(ctx, testSuite.Repository)
		if err != nil {
			return fmt.Errorf("failed to create workspace for %s: %w", testSuite.Name, err)
		}

		testSuite.WorkspaceDir = workspaceDir

		// Update test case workspace paths
		for _, testCase := range testSuite.TestCases {
			testCase.Workspace = workspaceDir
			testCase.FilePath = filepath.Join(workspaceDir, testCase.Config.File)
		}

		r.logger.Debug("Created workspace for %s at %s", testSuite.Name, workspaceDir)
	}

	return nil
}

// cleanupWorkspaces removes temporary workspaces
func (r *TestRunner) cleanupWorkspaces(ctx context.Context) {
	r.logger.Info("Cleaning up test workspaces...")

	for _, testSuite := range r.testSuites {
		if testSuite.WorkspaceDir != "" {
			if err := r.fileManager.CleanupWorkspace(ctx, testSuite.WorkspaceDir); err != nil {
				r.logger.Warn("Failed to cleanup workspace %s: %v", testSuite.WorkspaceDir, err)
			}
		}
	}
}

// dryRun performs a dry run without executing tests
func (r *TestRunner) dryRun(_ context.Context, options *RunOptions) (*cases.TestResult, error) {
	r.logger.Info("Performing dry run...")

	for _, testSuite := range r.testSuites {
		testSuite.Status = cases.TestStatusSkipped

		for _, testCase := range testSuite.TestCases {
			if options.Filter.Matches(testCase) {
				testCase.Status = cases.TestStatusSkipped
				r.logger.Info("Would run: %s", testCase.Name)
			}
		}

		testSuite.UpdateStatus()
	}

	r.updateFinalResults()
	return r.results, nil
}

// executeTestSuites executes all test suites
func (r *TestRunner) executeTestSuites(ctx context.Context, options *RunOptions) (*cases.TestResult, error) {
	r.logger.Info("Executing test suites...")

	for _, testSuite := range r.testSuites {
		if err := r.executeTestSuite(ctx, testSuite, options); err != nil {
			r.logger.Error("Test suite %s failed: %v", testSuite.Name, err)
			if options.FailFast {
				return r.results, err
			}
		}
	}

	// Shutdown all servers
	if err := r.serverManager.ShutdownAll(ctx); err != nil {
		r.logger.Warn("Failed to shutdown servers: %v", err)
	}

	return r.results, nil
}

// executeTestSuite executes a single test suite
func (r *TestRunner) executeTestSuite(ctx context.Context, testSuite *cases.TestSuite, options *RunOptions) error {
	testSuite.StartTime = time.Now()
	testSuite.Status = cases.TestStatusRunning

	r.logger.WithTestSuite(testSuite).Info("Executing test suite: %s", testSuite.Name)

	defer func() {
		testSuite.EndTime = time.Now()
		testSuite.Duration = testSuite.EndTime.Sub(testSuite.StartTime)
		testSuite.UpdateStatus()

		r.logger.WithTestSuite(testSuite).Info("Test suite completed: %s (Duration: %v, Status: %s)",
			testSuite.Name, testSuite.Duration, testSuite.Status)
	}()

	// Filter test cases
	filteredTestCases := make([]*cases.TestCase, 0)
	for _, testCase := range testSuite.TestCases {
		if options.Filter.Matches(testCase) {
			filteredTestCases = append(filteredTestCases, testCase)
		} else {
			testCase.Status = cases.TestStatusSkipped
		}
	}

	if len(filteredTestCases) == 0 {
		r.logger.WithTestSuite(testSuite).Info("No matching test cases found")
		testSuite.Status = cases.TestStatusSkipped
		return nil
	}

	// Execute test cases
	for _, testCase := range filteredTestCases {
		if options.FailFast && r.hasFailures() {
			testCase.Status = cases.TestStatusSkipped
			continue
		}

		r.wg.Add(1)
		go func(tc *cases.TestCase) {
			defer r.wg.Done()
			r.executeTestCase(ctx, tc)
		}(testCase)
	}

	r.wg.Wait()

	return nil
}

// executeTestCase executes a single test case
func (r *TestRunner) executeTestCase(ctx context.Context, testCase *cases.TestCase) {
	// Acquire semaphore for concurrency control
	r.semaphore <- struct{}{}
	defer func() { <-r.semaphore }()

	testCase.StartTime = time.Now()
	testCase.Status = cases.TestStatusRunning

	logger := r.logger.WithTestCase(testCase)
	logger.Debug("Executing test case: %s", testCase.Name)

	defer func() {
		testCase.EndTime = time.Now()
		testCase.Duration = testCase.EndTime.Sub(testCase.StartTime)

		r.mu.Lock()
		r.completedTests++
		r.mu.Unlock()

		logger.Info("Test case completed: %s (Duration: %v, Status: %s)",
			testCase.Name, testCase.Duration, testCase.Status)
	}()

	// Apply test case timeout
	testCtx := ctx
	if testCase.Timeout > 0 {
		var cancel context.CancelFunc
		testCtx, cancel = context.WithTimeout(ctx, testCase.Timeout)
		defer cancel()
	}

	// Skip test if configured
	if testCase.Config.Skip {
		testCase.Status = cases.TestStatusSkipped
		testCase.Error = fmt.Errorf("%s", testCase.Config.SkipReason)
		return
	}

	// Read file content if needed
	var fileContent string
	if r.needsFileContent(testCase.Method) {
		content, err := r.fileManager.ReadFile(testCase.FilePath)
		if err != nil {
			testCase.Status = cases.TestStatusError
			testCase.Error = fmt.Errorf("failed to read file %s: %w", testCase.FilePath, err)
			return
		}
		fileContent = string(content)
	}

	// Execute the test case
	if fileContent != "" {
		err := r.communicator.ExecuteTestCaseWithDocumentLifecycle(testCtx, testCase, fileContent)
		if err != nil {
			testCase.Status = cases.TestStatusError
			testCase.Error = err
		}
	} else {
		err := r.communicator.ExecuteTestCase(testCtx, testCase)
		if err != nil {
			testCase.Status = cases.TestStatusError
			testCase.Error = err
		}
	}
}

// needsFileContent determines if the method requires file content
func (r *TestRunner) needsFileContent(method string) bool {
	switch method {
	case cases.LSPMethodDefinition, cases.LSPMethodReferences, cases.LSPMethodHover, cases.LSPMethodDocumentSymbol:
		return true
	case cases.LSPMethodWorkspaceSymbol:
		return false
	default:
		return true // Default to requiring file content
	}
}

// hasFailures checks if there are any test failures
func (r *TestRunner) hasFailures() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, testSuite := range r.testSuites {
		for _, testCase := range testSuite.TestCases {
			if testCase.Status == cases.TestStatusFailed || testCase.Status == cases.TestStatusError {
				return true
			}
		}
	}
	return false
}

// updateFinalResults updates the final test results
func (r *TestRunner) updateFinalResults() {
	r.results.TotalCases = 0
	r.results.PassedCases = 0
	r.results.FailedCases = 0
	r.results.SkippedCases = 0
	r.results.ErrorCases = 0

	for _, testSuite := range r.testSuites {
		testSuite.UpdateStatus()

		r.results.TotalCases += testSuite.TotalCases
		r.results.PassedCases += testSuite.PassedCases
		r.results.FailedCases += testSuite.FailedCases
		r.results.SkippedCases += testSuite.SkippedCases
		r.results.ErrorCases += testSuite.ErrorCases
	}
}

// CreateWorkspace creates a workspace directory for a repository
func (fm *TestFileManager) CreateWorkspace(_ context.Context, repo *config.RepositoryConfig) (string, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if existing, exists := fm.workspaces[repo.Name]; exists {
		return existing, nil
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("lsp-test-%s-*", repo.Name))
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// If repository path exists, copy it
	if repo.Path != "" {
		if err := fm.copyRepository(repo.Path, tmpDir); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("failed to copy repository: %w", err)
		}
	}

	fm.workspaces[repo.Name] = tmpDir
	return tmpDir, nil
}

// copyRepository copies repository files to workspace
func (fm *TestFileManager) copyRepository(srcPath, destPath string) error {
	return filepath.WalkDir(srcPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return err
		}

		destFile := filepath.Join(destPath, relPath)

		if d.IsDir() {
			return os.MkdirAll(destFile, 0755)
		}

		// Copy file
		srcData, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destFile, srcData, 0644)
	})
}

// CleanupWorkspace removes a workspace directory
func (fm *TestFileManager) CleanupWorkspace(_ context.Context, workspaceDir string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	// Remove from tracking
	for name, dir := range fm.workspaces {
		if dir == workspaceDir {
			delete(fm.workspaces, name)
			break
		}
	}

	return os.RemoveAll(workspaceDir)
}

// ReadFile reads a file from the workspace
func (fm *TestFileManager) ReadFile(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}
