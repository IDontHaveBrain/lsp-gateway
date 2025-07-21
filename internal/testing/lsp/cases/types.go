package cases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"lsp-gateway/internal/testing/lsp/config"
)

// TestCase represents a single LSP test case
type TestCase struct {
	ID          string
	Name        string
	Description string
	Method      string
	Repository  *config.RepositoryConfig
	Config      *config.TestCaseConfig
	
	// Test execution context
	Workspace   string
	FilePath    string
	Position    *config.Position
	Params      map[string]interface{}
	Expected    *config.ExpectedResult
	
	// Metadata
	Tags        []string
	Language    string
	Timeout     time.Duration
	
	// State
	Status      TestStatus
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	
	// Results
	Response    json.RawMessage
	Error       error
	ValidationResults []*ValidationResult
}

// TestStatus represents the status of a test case
type TestStatus int

const (
	TestStatusPending TestStatus = iota
	TestStatusRunning
	TestStatusPassed
	TestStatusFailed
	TestStatusSkipped
	TestStatusError
)

func (s TestStatus) String() string {
	switch s {
	case TestStatusPending:
		return "pending"
	case TestStatusRunning:
		return "running"
	case TestStatusPassed:
		return "passed"
	case TestStatusFailed:
		return "failed"
	case TestStatusSkipped:
		return "skipped"
	case TestStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// ValidationResult represents the result of a single validation check
type ValidationResult struct {
	Name        string
	Description string
	Passed      bool
	Message     string
	Details     map[string]interface{}
}

// TestSuite represents a collection of related test cases
type TestSuite struct {
	Name         string
	Description  string
	Repository   *config.RepositoryConfig
	TestCases    []*TestCase
	
	// Execution context
	WorkspaceDir string
	ServerConfig *config.ServerConfig
	
	// State
	Status       TestStatus
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	
	// Results
	TotalCases   int
	PassedCases  int
	FailedCases  int
	SkippedCases int
	ErrorCases   int
}

// TestRunContext provides context for test execution
type TestRunContext struct {
	ctx context.Context
	
	// Configuration
	Config       *config.LSPTestConfig
	Repository   *config.RepositoryConfig
	ServerConfig *config.ServerConfig
	
	// Runtime state
	WorkspaceDir string
	ServerPID    int
	
	// Test execution tracking
	TestResults  map[string]*TestCase
	
	// Utilities
	Logger       TestLogger
	FileManager  TestFileManager
}

// TestLogger interface for test logging
type TestLogger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	
	WithFields(fields map[string]interface{}) TestLogger
	WithTestCase(testCase *TestCase) TestLogger
}

// TestFileManager interface for managing test files and workspaces
type TestFileManager interface {
	CreateWorkspace(ctx context.Context, repo *config.RepositoryConfig) (string, error)
	CleanupWorkspace(ctx context.Context, workspaceDir string) error
	
	GetFilePath(workspaceDir, relativePath string) string
	FileExists(filePath string) bool
	ReadFile(filePath string) ([]byte, error)
	
	ResolvePosition(filePath string, position *config.Position) (*AbsolutePosition, error)
}

// AbsolutePosition represents an absolute position in a file with byte offset
type AbsolutePosition struct {
	Line      int
	Character int
	Offset    int
}

// LSPMethod constants for the five core methods we're testing
const (
	LSPMethodDefinition      = "textDocument/definition"
	LSPMethodReferences      = "textDocument/references"  
	LSPMethodHover           = "textDocument/hover"
	LSPMethodDocumentSymbol  = "textDocument/documentSymbol"
	LSPMethodWorkspaceSymbol = "workspace/symbol"
)

// SupportedLSPMethods returns the list of LSP methods supported by the test framework
func SupportedLSPMethods() []string {
	return []string{
		LSPMethodDefinition,
		LSPMethodReferences,
		LSPMethodHover,
		LSPMethodDocumentSymbol,
		LSPMethodWorkspaceSymbol,
	}
}

// IsMethodSupported checks if a given LSP method is supported by the test framework
func IsMethodSupported(method string) bool {
	for _, supported := range SupportedLSPMethods() {
		if supported == method {
			return true
		}
	}
	return false
}

// TestCaseFilter provides filtering capabilities for test cases
type TestCaseFilter struct {
	IDs        []string
	Names      []string
	Methods    []string
	Tags       []string
	Languages  []string
	
	IncludeSkipped bool
	Pattern        string
}

// Matches checks if a test case matches the filter criteria
func (f *TestCaseFilter) Matches(testCase *TestCase) bool {
	// Check IDs
	if len(f.IDs) > 0 {
		found := false
		for _, id := range f.IDs {
			if testCase.ID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check names
	if len(f.Names) > 0 {
		found := false
		for _, name := range f.Names {
			if testCase.Name == name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check methods
	if len(f.Methods) > 0 {
		found := false
		for _, method := range f.Methods {
			if testCase.Method == method {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check tags
	if len(f.Tags) > 0 {
		found := false
		for _, filterTag := range f.Tags {
			for _, testTag := range testCase.Tags {
				if filterTag == testTag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check languages
	if len(f.Languages) > 0 {
		found := false
		for _, lang := range f.Languages {
			if testCase.Language == lang {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check if skipped tests should be included
	if testCase.Config.Skip && !f.IncludeSkipped {
		return false
	}
	
	return true
}

// TestResult aggregates results across test cases
type TestResult struct {
	TestSuites   []*TestSuite
	TotalCases   int
	PassedCases  int
	FailedCases  int
	SkippedCases int
	ErrorCases   int
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
}

// Success returns true if all tests passed
func (r *TestResult) Success() bool {
	return r.FailedCases == 0 && r.ErrorCases == 0
}

// PassRate returns the pass rate as a percentage
func (r *TestResult) PassRate() float64 {
	if r.TotalCases == 0 {
		return 0
	}
	return float64(r.PassedCases) / float64(r.TotalCases) * 100
}

// Summary returns a summary string of the test results
func (r *TestResult) Summary() string {
	return fmt.Sprintf("Tests: %d, Passed: %d, Failed: %d, Skipped: %d, Errors: %d, Pass Rate: %.1f%%, Duration: %v",
		r.TotalCases, r.PassedCases, r.FailedCases, r.SkippedCases, r.ErrorCases, r.PassRate(), r.Duration)
}

// NewTestCase creates a new test case from configuration
func NewTestCase(id string, repo *config.RepositoryConfig, testConfig *config.TestCaseConfig) *TestCase {
	return &TestCase{
		ID:          id,
		Name:        testConfig.Name,
		Description: testConfig.Description,
		Method:      testConfig.Method,
		Repository:  repo,
		Config:      testConfig,
		Position:    testConfig.Position,
		Params:      testConfig.Params,
		Expected:    testConfig.Expected,
		Tags:        testConfig.Tags,
		Language:    repo.Language,
		Timeout:     testConfig.Timeout,
		Status:      TestStatusPending,
		ValidationResults: make([]*ValidationResult, 0),
	}
}

// NewTestSuite creates a new test suite
func NewTestSuite(name string, repo *config.RepositoryConfig, serverConfig *config.ServerConfig) *TestSuite {
	return &TestSuite{
		Name:         name,
		Description:  repo.Description,
		Repository:   repo,
		ServerConfig: serverConfig,
		TestCases:    make([]*TestCase, 0),
		Status:       TestStatusPending,
	}
}

// AddTestCase adds a test case to the suite
func (s *TestSuite) AddTestCase(testCase *TestCase) {
	s.TestCases = append(s.TestCases, testCase)
	s.TotalCases = len(s.TestCases)
}

// UpdateStatus updates the test suite status based on test case results
func (s *TestSuite) UpdateStatus() {
	s.PassedCases = 0
	s.FailedCases = 0
	s.SkippedCases = 0
	s.ErrorCases = 0
	
	for _, testCase := range s.TestCases {
		switch testCase.Status {
		case TestStatusPassed:
			s.PassedCases++
		case TestStatusFailed:
			s.FailedCases++
		case TestStatusSkipped:
			s.SkippedCases++
		case TestStatusError:
			s.ErrorCases++
		}
	}
	
	if s.FailedCases > 0 || s.ErrorCases > 0 {
		s.Status = TestStatusFailed
	} else if s.SkippedCases == s.TotalCases {
		s.Status = TestStatusSkipped
	} else {
		s.Status = TestStatusPassed
	}
}

// GetContext returns the test run context
func (c *TestRunContext) GetContext() context.Context {
	return c.ctx
}

// WithTimeout creates a new context with timeout
func (c *TestRunContext) WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.ctx, timeout)
}

// LogTestCase logs information about a test case
func (c *TestRunContext) LogTestCase(testCase *TestCase, message string) {
	if c.Logger != nil {
		logger := c.Logger.WithTestCase(testCase)
		logger.Info(message)
	}
}

// RecordTestResult records a test case result
func (c *TestRunContext) RecordTestResult(testCase *TestCase) {
	if c.TestResults == nil {
		c.TestResults = make(map[string]*TestCase)
	}
	c.TestResults[testCase.ID] = testCase
}