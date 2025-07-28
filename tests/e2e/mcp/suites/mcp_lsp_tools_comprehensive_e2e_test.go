package suites

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	e2e_test "lsp-gateway/tests/e2e/helpers"
	"lsp-gateway/tests/e2e/testutils"
	"lsp-gateway/tests/e2e/mcp/types"
)

// MCPLSPToolsE2ETestSuite provides comprehensive E2E tests for all 6 supported LSP features
// via MCP protocol using fatih/color repository as test workspace
type MCPLSPToolsE2ETestSuite struct {
	suite.Suite
	client          *TestMCPClient
	workspace       *TestWorkspace
	server          *exec.Cmd
	config          *types.MCPTestConfig
	metrics         *TestMetricsCollector
	projectRoot     string
	binaryPath      string
	testTimeout     time.Duration
	assertionHelper *e2e_test.AssertionHelper
	serverMutex     sync.Mutex
}

// TestWorkspace manages the fatih/color repository workspace for testing
type TestWorkspace struct {
	RootPath        string
	RepositoryURL   string
	CommitHash      string
	Files           map[string]*WorkspaceFile
	LSPServerConfig *LSPServerConfig
	setupComplete   bool
}

// WorkspaceFile represents a file in the test workspace with known symbols
type WorkspaceFile struct {
	Path         string
	RelativePath string
	Content      string
	Language     string
	KnownSymbols []KnownSymbol
}

// KnownSymbol represents a symbol with known position in fatih/color repository
type KnownSymbol struct {
	Name      string
	Kind      string
	Line      int
	Character int
	Type      string
	URI       string
}

// LSPServerConfig holds LSP server configuration for the workspace
type LSPServerConfig struct {
	Command     string
	Args        []string
	RootMarkers []string
	InitOptions map[string]interface{}
}

// TestMCPClient represents a simple MCP client for testing
type TestMCPClient struct {
	stdin     *os.File
	stdout    *os.File
	reader    *bufio.Reader
	messageID int
}

// TestMetricsCollector collects test execution metrics
type TestMetricsCollector struct {
	mutex         sync.Mutex
	ResponseTimes map[string][]time.Duration
	SuccessCount  map[string]int
	ErrorCount    map[string]int
	TotalTests    int
}

// MCPResponse represents a simplified MCP response
type MCPResponse struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents content in MCP response
type ContentBlock struct {
	Type string      `json:"type"`
	Text string      `json:"text,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// SetupSuite initializes the test suite with fatih/color workspace
func (s *MCPLSPToolsE2ETestSuite) SetupSuite() {
	s.testTimeout = 300 * time.Second
	var err error
	s.projectRoot, err = testutils.GetProjectRoot()
	if err != nil {
		s.T().Fatalf("Failed to get project root: %v", err)
	}
	s.binaryPath = filepath.Join(s.projectRoot, "bin", "lspg")
	s.assertionHelper = e2e_test.NewAssertionHelper(s.T())
	s.metrics = &TestMetricsCollector{
		ResponseTimes: make(map[string][]time.Duration),
		SuccessCount:  make(map[string]int),
		ErrorCount:    make(map[string]int),
	}

	s.setupFatihColorWorkspace()
}

// SetupTest prepares each individual test
func (s *MCPLSPToolsE2ETestSuite) SetupTest() {
	if _, err := os.Stat(s.binaryPath); os.IsNotExist(err) {
		s.T().Skipf("Binary not found at %s, run 'make local' first", s.binaryPath)
	}
}

// TearDownSuite cleans up test resources
func (s *MCPLSPToolsE2ETestSuite) TearDownSuite() {
	if s.workspace != nil && s.workspace.RootPath != "" {
		os.RemoveAll(s.workspace.RootPath)
	}
	if s.server != nil && s.server.Process != nil {
		s.server.Process.Signal(syscall.SIGTERM)
		s.server.Wait()
	}
}

// setupFatihColorWorkspace clones and prepares the fatih/color repository
func (s *MCPLSPToolsE2ETestSuite) setupFatihColorWorkspace() {
	tempDir, err := os.MkdirTemp("", "mcp-fatih-color-test-*")
	if err != nil {
		s.T().Fatalf("Failed to create temp directory: %v", err)
	}

	s.workspace = &TestWorkspace{
		RootPath:      tempDir,
		RepositoryURL: "https://github.com/fatih/color.git",
		CommitHash:    "4c05561a8fbfd21922e4908479e63b48b677a61f",
		Files:         make(map[string]*WorkspaceFile),
		LSPServerConfig: &LSPServerConfig{
			Command:     "gopls",
			Args:        []string{},
			RootMarkers: []string{"go.mod", "go.sum"},
			InitOptions: map[string]interface{}{},
		},
	}

	s.cloneFatihColorRepository()
	s.analyzeFatihColorFiles()
}

// cloneFatihColorRepository clones the fatih/color repository
func (s *MCPLSPToolsE2ETestSuite) cloneFatihColorRepository() {
	cloneCmd := exec.Command("git", "clone", s.workspace.RepositoryURL, s.workspace.RootPath)
	if err := cloneCmd.Run(); err != nil {
		s.T().Fatalf("Failed to clone fatih/color repository: %v", err)
	}

	checkoutCmd := exec.Command("git", "checkout", s.workspace.CommitHash)
	checkoutCmd.Dir = s.workspace.RootPath
	if err := checkoutCmd.Run(); err != nil {
		s.T().Fatalf("Failed to checkout commit %s: %v", s.workspace.CommitHash, err)
	}

	s.workspace.setupComplete = true
	s.T().Logf("Successfully cloned fatih/color repository to %s", s.workspace.RootPath)
}

// analyzeFatihColorFiles analyzes the repository files and extracts known symbols
func (s *MCPLSPToolsE2ETestSuite) analyzeFatihColorFiles() {
	// Main color.go file with core functionality
	colorGoPath := filepath.Join(s.workspace.RootPath, "color.go")
	if content, err := os.ReadFile(colorGoPath); err == nil {
		s.workspace.Files["color.go"] = &WorkspaceFile{
			Path:         colorGoPath,
			RelativePath: "color.go",
			Content:      string(content),
			Language:     "go",
			KnownSymbols: []KnownSymbol{
				{Name: "Red", Kind: "function", Line: 110, Character: 5, Type: "func(*Color) string", URI: fmt.Sprintf("file://%s", colorGoPath)},
				{Name: "Green", Kind: "function", Line: 115, Character: 5, Type: "func(*Color) string", URI: fmt.Sprintf("file://%s", colorGoPath)},
				{Name: "Blue", Kind: "function", Line: 120, Character: 5, Type: "func(*Color) string", URI: fmt.Sprintf("file://%s", colorGoPath)},
				{Name: "Color", Kind: "struct", Line: 51, Character: 5, Type: "struct", URI: fmt.Sprintf("file://%s", colorGoPath)},
				{Name: "Print", Kind: "method", Line: 169, Character: 17, Type: "func(*Color, ...interface{})", URI: fmt.Sprintf("file://%s", colorGoPath)},
				{Name: "Printf", Kind: "method", Line: 175, Character: 17, Type: "func(*Color, string, ...interface{})", URI: fmt.Sprintf("file://%s", colorGoPath)},
				{Name: "Println", Kind: "method", Line: 181, Character: 17, Type: "func(*Color, ...interface{})", URI: fmt.Sprintf("file://%s", colorGoPath)},
				{Name: "Sprint", Kind: "method", Line: 187, Character: 17, Type: "func(*Color, ...interface{}) string", URI: fmt.Sprintf("file://%s", colorGoPath)},
				{Name: "Sprintf", Kind: "method", Line: 193, Character: 17, Type: "func(*Color, string, ...interface{}) string", URI: fmt.Sprintf("file://%s", colorGoPath)},
				{Name: "Sprintln", Kind: "method", Line: 199, Character: 17, Type: "func(*Color, ...interface{}) string", URI: fmt.Sprintf("file://%s", colorGoPath)},
			},
		}
	}

	// color_test.go file for testing scenarios
	testGoPath := filepath.Join(s.workspace.RootPath, "color_test.go")
	if content, err := os.ReadFile(testGoPath); err == nil {
		s.workspace.Files["color_test.go"] = &WorkspaceFile{
			Path:         testGoPath,
			RelativePath: "color_test.go",
			Content:      string(content),
			Language:     "go",
			KnownSymbols: []KnownSymbol{
				{Name: "TestColor", Kind: "function", Line: 10, Character: 5, Type: "func(*testing.T)", URI: fmt.Sprintf("file://%s", testGoPath)},
				{Name: "TestColorEquals", Kind: "function", Line: 50, Character: 5, Type: "func(*testing.T)", URI: fmt.Sprintf("file://%s", testGoPath)},
				{Name: "BenchmarkPrintln", Kind: "function", Line: 80, Character: 5, Type: "func(*testing.B)", URI: fmt.Sprintf("file://%s", testGoPath)},
			},
		}
	}

	// doc.go for documentation testing
	docGoPath := filepath.Join(s.workspace.RootPath, "doc.go")
	if content, err := os.ReadFile(docGoPath); err == nil {
		s.workspace.Files["doc.go"] = &WorkspaceFile{
			Path:         docGoPath,
			RelativePath: "doc.go",
			Content:      string(content),
			Language:     "go",
			KnownSymbols: []KnownSymbol{
				{Name: "color", Kind: "package", Line: 1, Character: 8, Type: "package", URI: fmt.Sprintf("file://%s", docGoPath)},
			},
		}
	}

	s.T().Logf("Analyzed %d files in fatih/color repository", len(s.workspace.Files))
}

// TestGotoDefinitionComprehensive tests textDocument/definition via MCP
func (s *MCPLSPToolsE2ETestSuite) TestGotoDefinitionComprehensive() {
	if testing.Short() {
		s.T().Skip("Skipping comprehensive MCP LSP test in short mode")
	}

	testCases := []struct {
		name             string
		file             string
		line             int
		character        int
		expectedLocation *ExpectedLocation
		description      string
	}{
		{
			name:      "Go to Red function definition",
			file:      "color.go",
			line:      110,
			character: 8,
			expectedLocation: &ExpectedLocation{
				URI:   s.getFileURI("color.go"),
				Range: LSPRange{Start: Position{Line: 110, Character: 0}, End: Position{Line: 110, Character: 15}},
			},
			description: "Navigate to Red function definition in color.go",
		},
		{
			name:      "Go to Color struct definition",
			file:      "color.go",
			line:      51,
			character: 8,
			expectedLocation: &ExpectedLocation{
				URI:   s.getFileURI("color.go"),
				Range: LSPRange{Start: Position{Line: 51, Character: 0}, End: Position{Line: 51, Character: 11}},
			},
			description: "Navigate to Color struct definition",
		},
		{
			name:      "Go to Print method definition",
			file:      "color.go",
			line:      169,
			character: 20,
			expectedLocation: &ExpectedLocation{
				URI:   s.getFileURI("color.go"),
				Range: LSPRange{Start: Position{Line: 169, Character: 0}, End: Position{Line: 169, Character: 30}},
			},
			description: "Navigate to Print method definition on Color struct",
		},
		{
			name:      "Go to imported package definition",
			file:      "color.go",
			line:      6,
			character: 2,
			expectedLocation: &ExpectedLocation{
				URI:   "file:///go/src/fmt",
				Range: LSPRange{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 0}},
			},
			description: "Navigate to imported fmt package definition",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			response := s.executeMCPToolCall("goto_definition", map[string]interface{}{
				"uri":       s.getFileURI(tc.file),
				"line":      tc.line,
				"character": tc.character,
			})

			s.assertMCPResponse(response, "goto_definition")
			s.validateDefinitionResponse(response, tc.expectedLocation)
			s.T().Logf("✓ %s: %s", tc.name, tc.description)
		})
	}
}

// TestFindReferencesComprehensive tests textDocument/references via MCP
func (s *MCPLSPToolsE2ETestSuite) TestFindReferencesComprehensive() {
	if testing.Short() {
		s.T().Skip("Skipping comprehensive MCP LSP test in short mode")
	}

	testCases := []struct {
		name               string
		file               string
		line               int
		character          int
		includeDeclaration bool
		minExpectedRefs    int
		description        string
	}{
		{
			name:               "Find all references to Color struct",
			file:               "color.go",
			line:               51,
			character:          8,
			includeDeclaration: true,
			minExpectedRefs:    10,
			description:        "Find all references to Color struct throughout the codebase",
		},
		{
			name:               "Find all references to Red function",
			file:               "color.go",
			line:               110,
			character:          8,
			includeDeclaration: false,
			minExpectedRefs:    2,
			description:        "Find usage references to Red function (excluding declaration)",
		},
		{
			name:               "Find all references to Print method",
			file:               "color.go",
			line:               169,
			character:          20,
			includeDeclaration: true,
			minExpectedRefs:    3,
			description:        "Find all references to Print method including declaration",
		},
		{
			name:               "Find cross-file references",
			file:               "color_test.go",
			line:               15,
			character:          10,
			includeDeclaration: true,
			minExpectedRefs:    1,
			description:        "Find references that span across multiple files",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			response := s.executeMCPToolCall("find_references", map[string]interface{}{
				"uri":                s.getFileURI(tc.file),
				"line":               tc.line,
				"character":          tc.character,
				"includeDeclaration": tc.includeDeclaration,
			})

			s.assertMCPResponse(response, "find_references")
			references := s.validateReferencesResponse(response)
			s.GreaterOrEqual(len(references), tc.minExpectedRefs, 
				"Expected at least %d references for %s", tc.minExpectedRefs, tc.name)
			s.T().Logf("✓ %s: Found %d references - %s", tc.name, len(references), tc.description)
		})
	}
}

// TestGetHoverInfoComprehensive tests textDocument/hover via MCP
func (s *MCPLSPToolsE2ETestSuite) TestGetHoverInfoComprehensive() {
	if testing.Short() {
		s.T().Skip("Skipping comprehensive MCP LSP test in short mode")
	}

	testCases := []struct {
		name         string
		file         string
		line         int
		character    int
		expectHover  bool
		expectedInfo []string
		description  string
	}{
		{
			name:         "Hover over Red function",
			file:         "color.go",
			line:         110,
			character:    8,
			expectHover:  true,
			expectedInfo: []string{"func", "Red", "string"},
			description:  "Get hover information for Red function with signature",
		},
		{
			name:         "Hover over Color struct",
			file:         "color.go",
			line:         51,
			character:    8,
			expectHover:  true,
			expectedInfo: []string{"type", "Color", "struct"},
			description:  "Get hover information for Color struct definition",
		},
		{
			name:         "Hover over Print method",
			file:         "color.go",
			line:         169,
			character:    20,
			expectHover:  true,
			expectedInfo: []string{"func", "Print", "Color"},
			description:  "Get hover information for Print method with receiver info",
		},
		{
			name:         "Hover over import statement",
			file:         "color.go",
			line:         6,
			character:    2,
			expectHover:  true,
			expectedInfo: []string{"package", "fmt"},
			description:  "Get hover information for imported package",
		},
		{
			name:        "Hover over empty space",
			file:        "color.go",
			line:        1,
			character:   1,
			expectHover: false,
			description: "No hover information expected for empty space",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			response := s.executeMCPToolCall("get_hover_info", map[string]interface{}{
				"uri":       s.getFileURI(tc.file),
				"line":      tc.line,
				"character": tc.character,
			})

			s.assertMCPResponse(response, "get_hover_info")
			
			if tc.expectHover {
				hover := s.validateHoverResponse(response)
				for _, expected := range tc.expectedInfo {
					s.Contains(strings.ToLower(hover), strings.ToLower(expected),
						"Hover info should contain '%s'", expected)
				}
				s.T().Logf("✓ %s: %s", tc.name, tc.description)
			} else {
				s.validateEmptyHoverResponse(response)
				s.T().Logf("✓ %s: %s", tc.name, tc.description)
			}
		})
	}
}

// TestGetDocumentSymbolsComprehensive tests textDocument/documentSymbol via MCP
func (s *MCPLSPToolsE2ETestSuite) TestGetDocumentSymbolsComprehensive() {
	if testing.Short() {
		s.T().Skip("Skipping comprehensive MCP LSP test in short mode")
	}

	testCases := []struct {
		name            string
		file            string
		minExpectedSymbols int
		expectedSymbols []string
		description     string
	}{
		{
			name:            "Get document symbols from color.go",
			file:            "color.go",
			minExpectedSymbols: 15,
			expectedSymbols: []string{"Color", "Red", "Green", "Blue", "Print", "Printf", "Println"},
			description:     "Extract all document symbols from main color.go file",
		},
		{
			name:            "Get document symbols from color_test.go",
			file:            "color_test.go",
			minExpectedSymbols: 5,
			expectedSymbols: []string{"TestColor", "TestColorEquals", "BenchmarkPrintln"},
			description:     "Extract test functions and benchmarks from test file",
		},
		{
			name:            "Get document symbols from doc.go",
			file:            "doc.go",
			minExpectedSymbols: 1,
			expectedSymbols: []string{"color"},
			description:     "Extract package documentation symbols",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			response := s.executeMCPToolCall("get_document_symbols", map[string]interface{}{
				"uri": s.getFileURI(tc.file),
			})

			s.assertMCPResponse(response, "get_document_symbols")
			symbols := s.validateDocumentSymbolsResponse(response)
			
			s.GreaterOrEqual(len(symbols), tc.minExpectedSymbols,
				"Expected at least %d symbols in %s", tc.minExpectedSymbols, tc.file)

			for _, expectedSymbol := range tc.expectedSymbols {
				found := false
				for _, symbol := range symbols {
					if symbolMap, ok := symbol.(map[string]interface{}); ok {
						if symbolName, ok := symbolMap["name"].(string); ok && symbolName == expectedSymbol {
							found = true
							break
						}
					}
				}
				s.True(found, "Expected symbol '%s' not found in %s", expectedSymbol, tc.file)
			}

			s.T().Logf("✓ %s: Found %d symbols - %s", tc.name, len(symbols), tc.description)
		})
	}
}

// TestSearchWorkspaceSymbolsComprehensive tests workspace/symbol via MCP
func (s *MCPLSPToolsE2ETestSuite) TestSearchWorkspaceSymbolsComprehensive() {
	if testing.Short() {
		s.T().Skip("Skipping comprehensive MCP LSP test in short mode")
	}

	testCases := []struct {
		name           string
		query          string
		minExpectedResults int
		expectedSymbols []string
		description    string
	}{
		{
			name:           "Search for 'Color' symbols",
			query:          "Color",
			minExpectedResults: 3,
			expectedSymbols: []string{"Color", "TestColor", "TestColorEquals"},
			description:    "Find all symbols containing 'Color' across workspace",
		},
		{
			name:           "Search for function symbols",
			query:          "Red",
			minExpectedResults: 1,
			expectedSymbols: []string{"Red"},
			description:    "Find Red function symbol in workspace",
		},
		{
			name:           "Search for method symbols",
			query:          "Print",
			minExpectedResults: 3,
			expectedSymbols: []string{"Print", "Printf", "Println"},
			description:    "Find all Print-related methods in workspace",
		},
		{
			name:           "Search with partial match",
			query:          "Test",
			minExpectedResults: 2,
			expectedSymbols: []string{"TestColor", "TestColorEquals"},
			description:    "Find symbols using partial matching",
		},
		{
			name:           "Search for nonexistent symbol",
			query:          "NonExistentSymbol123",
			minExpectedResults: 0,
			expectedSymbols: []string{},
			description:    "Gracefully handle search for nonexistent symbols",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			response := s.executeMCPToolCall("search_workspace_symbols", map[string]interface{}{
				"query": tc.query,
			})

			s.assertMCPResponse(response, "search_workspace_symbols")
			symbols := s.validateWorkspaceSymbolsResponse(response)

			if tc.minExpectedResults > 0 {
				s.GreaterOrEqual(len(symbols), tc.minExpectedResults,
					"Expected at least %d results for query '%s'", tc.minExpectedResults, tc.query)

				for _, expectedSymbol := range tc.expectedSymbols {
					found := false
					for _, symbol := range symbols {
						if symbolMap, ok := symbol.(map[string]interface{}); ok {
							if symbolName, ok := symbolMap["name"].(string); ok && symbolName == expectedSymbol {
								found = true
								break
							}
						}
					}
					s.True(found, "Expected symbol '%s' not found in search results", expectedSymbol)
				}
			} else {
				s.Equal(0, len(symbols), "Expected no results for nonexistent symbol search")
			}

			s.T().Logf("✓ %s: Found %d symbols - %s", tc.name, len(symbols), tc.description)
		})
	}
}

// TestGetCompletionsComprehensive tests textDocument/completion via MCP
func (s *MCPLSPToolsE2ETestSuite) TestGetCompletionsComprehensive() {
	if testing.Short() {
		s.T().Skip("Skipping comprehensive MCP LSP test in short mode")
	}

	testCases := []struct {
		name                string
		file                string
		line                int
		character           int
		minExpectedItems    int
		expectedCompletions []string
		description         string
	}{
		{
			name:                "Method completion on Color struct",
			file:                "color.go",
			line:                200,
			character:           10,
			minExpectedItems:    5,
			expectedCompletions: []string{"Print", "Printf", "Println", "Sprint", "Sprintf"},
			description:         "Get method completions for Color struct instance",
		},
		{
			name:                "Import completion",
			file:                "color.go",
			line:                5,
			character:           8,
			minExpectedItems:    1,
			expectedCompletions: []string{"fmt"},
			description:         "Get import completions for standard library",
		},
		{
			name:                "Variable completion",
			file:                "color.go",
			line:                170,
			character:           15,
			minExpectedItems:    2,
			expectedCompletions: []string{"c", "a"},
			description:         "Get variable completions in function scope",
		},
		{
			name:                "Type completion",
			file:                "color_test.go",
			line:                15,
			character:           20,
			minExpectedItems:    1,
			expectedCompletions: []string{"Color"},
			description:         "Get type completions for struct instantiation",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			response := s.executeMCPToolCall("get_completions", map[string]interface{}{
				"uri":       s.getFileURI(tc.file),
				"line":      tc.line,
				"character": tc.character,
			})

			s.assertMCPResponse(response, "get_completions")
			completions := s.validateCompletionsResponse(response)

			s.GreaterOrEqual(len(completions), tc.minExpectedItems,
				"Expected at least %d completion items", tc.minExpectedItems)

			for _, expectedCompletion := range tc.expectedCompletions {
				found := false
				for _, completion := range completions {
					if completionMap, ok := completion.(map[string]interface{}); ok {
						if label, ok := completionMap["label"].(string); ok && 
						   strings.Contains(label, expectedCompletion) {
							found = true
							break
						}
					}
				}
				s.True(found, "Expected completion '%s' not found", expectedCompletion)
			}

			s.T().Logf("✓ %s: Found %d completions - %s", tc.name, len(completions), tc.description)
		})
	}
}

// TestMCPLSPToolsErrorHandling tests error handling across all MCP LSP tools
func (s *MCPLSPToolsE2ETestSuite) TestMCPLSPToolsErrorHandling() {
	if testing.Short() {
		s.T().Skip("Skipping comprehensive MCP LSP test in short mode")
	}

	errorTestCases := []struct {
		toolName        string
		params          map[string]interface{}
		expectSuccess   bool
		description     string
	}{
		{
			toolName: "goto_definition",
			params: map[string]interface{}{
				"uri": "file:///nonexistent.go",
				"line": 999,
				"character": 999,
			},
			expectSuccess: true,
			description: "Invalid position should return empty results gracefully",
		},
		{
			toolName: "find_references",
			params: map[string]interface{}{
				"uri": "invalid://uri",
				"line": 0,
				"character": 0,
			},
			expectSuccess: true,
			description: "Invalid URI should return empty results gracefully",
		},
		{
			toolName: "get_hover_info",
			params: map[string]interface{}{
				"uri": s.getFileURI("color.go"),
				"line": -1,
				"character": -1,
			},
			expectSuccess: true,
			description: "Negative position should return empty results gracefully",
		},
		{
			toolName: "get_document_symbols",
			params: map[string]interface{}{
				"uri": "file:///empty.go",
			},
			expectSuccess: true,
			description: "Nonexistent file should return empty results gracefully",
		},
		{
			toolName: "search_workspace_symbols",
			params: map[string]interface{}{
				"query": "",
			},
			expectSuccess: true,
			description: "Empty query should return empty results gracefully",
		},
		{
			toolName: "get_completions",
			params: map[string]interface{}{
				"uri": s.getFileURI("color.go"),
				"line": 10000,
				"character": 10000,
			},
			expectSuccess: true,
			description: "Out-of-bounds position should return empty results gracefully",
		},
	}

	for _, tc := range errorTestCases {
		s.Run(fmt.Sprintf("%s_error_handling", tc.toolName), func() {
			response := s.executeMCPToolCall(tc.toolName, tc.params)

			if tc.expectSuccess {
				s.assertMCPResponse(response, tc.toolName)
				s.T().Logf("✓ %s error handling: %s", tc.toolName, tc.description)
			} else {
				s.True(response.IsError, "Expected error for %s", tc.description)
				s.T().Logf("✓ %s error handling: %s", tc.toolName, tc.description)
			}
		})
	}
}

// TestMCPLSPToolsPerformance tests performance characteristics of all MCP LSP tools
func (s *MCPLSPToolsE2ETestSuite) TestMCPLSPToolsPerformance() {
	if testing.Short() {
		s.T().Skip("Skipping comprehensive MCP LSP test in short mode")
	}

	performanceThresholds := map[string]time.Duration{
		"goto_definition":         6 * time.Second,
		"find_references":         8 * time.Second,
		"get_hover_info":          5 * time.Second,
		"get_document_symbols":    6 * time.Second,
		"search_workspace_symbols": 7 * time.Second,
		"get_completions":         5 * time.Second,
	}

	for toolName, threshold := range performanceThresholds {
		s.Run(fmt.Sprintf("%s_performance", toolName), func() {
			var params map[string]interface{}

			switch toolName {
			case "search_workspace_symbols":
				params = map[string]interface{}{"query": "Color"}
			case "get_document_symbols":
				params = map[string]interface{}{"uri": s.getFileURI("color.go")}
			default:
				params = map[string]interface{}{
					"uri":       s.getFileURI("color.go"),
					"line":      50,
					"character": 5,
				}
			}

			startTime := time.Now()
			response := s.executeMCPToolCall(toolName, params)
			responseTime := time.Since(startTime)

			s.assertMCPResponse(response, toolName)
			s.Less(responseTime, threshold,
				"%s should respond within %v (actual: %v)", toolName, threshold, responseTime)

			s.recordMetric(toolName, responseTime, true)
			s.T().Logf("✓ %s performance: %v (threshold: %v)", toolName, responseTime, threshold)
		})
	}
}

// Helper methods

// executeMCPToolCall executes an MCP tool call and returns the response
func (s *MCPLSPToolsE2ETestSuite) executeMCPToolCall(toolName string, params map[string]interface{}) *MCPResponse {
	s.serverMutex.Lock()
	defer s.serverMutex.Unlock()

	if s.client == nil {
		s.startMCPServer()
	}

	// Convert params to JSON
	toolCall := map[string]interface{}{
		"name":      toolName,
		"arguments": params,
	}

	// Create MCP message
	message := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      s.client.messageID,
		"method":  "tools/call",
		"params":  toolCall,
	}
	s.client.messageID++

	// Send message
	msgBytes, _ := json.Marshal(message)
	msgLine := string(msgBytes) + "\n"
	s.client.stdin.WriteString(msgLine)

	// Read response
	responseLine, err := s.client.reader.ReadString('\n')
	if err != nil {
		return &MCPResponse{IsError: true}
	}

	var response map[string]interface{}
	if err := json.Unmarshal([]byte(responseLine), &response); err != nil {
		return &MCPResponse{IsError: true}
	}

	// Extract result
	if result, ok := response["result"].(map[string]interface{}); ok {
		if content, ok := result["content"].([]interface{}); ok {
			var contentBlocks []ContentBlock
			for _, c := range content {
				if block, ok := c.(map[string]interface{}); ok {
					contentBlock := ContentBlock{
						Type: block["type"].(string),
					}
					if text, exists := block["text"]; exists {
						contentBlock.Text = text.(string)
					}
					if data, exists := block["data"]; exists {
						contentBlock.Data = data
					}
					contentBlocks = append(contentBlocks, contentBlock)
				}
			}
			return &MCPResponse{Content: contentBlocks}
		}
	}

	return &MCPResponse{IsError: true}
}

// startMCPServer starts the MCP server for testing
func (s *MCPLSPToolsE2ETestSuite) startMCPServer() {
	gatewayPort, err := testutils.FindAvailablePort()
	if err != nil {
		s.T().Fatalf("Failed to find available port: %v", err)
	}

	configPath := s.createTempConfig(gatewayPort)
	defer os.Remove(configPath)

	// Start gateway
	gatewayCmd := exec.Command(s.binaryPath, "server", "--config", configPath)
	gatewayCmd.Dir = s.workspace.RootPath
	err = gatewayCmd.Start()
	if err != nil {
		s.T().Fatalf("Failed to start gateway: %v", err)
	}

	time.Sleep(3 * time.Second)

	// Start MCP server
	mcpCmd := exec.Command(s.binaryPath, "mcp", "--gateway", fmt.Sprintf("http://localhost:%d", gatewayPort))
	mcpCmd.Dir = s.workspace.RootPath

	stdin, _ := mcpCmd.StdinPipe()
	stdout, _ := mcpCmd.StdoutPipe()
	err = mcpCmd.Start()
	if err != nil {
		gatewayCmd.Process.Kill()
		s.T().Fatalf("Failed to start MCP server: %v", err)
	}

	s.server = mcpCmd
	s.client = &TestMCPClient{
		stdin:     stdin.(*os.File),
		reader:    bufio.NewReader(stdout),
		messageID: 1,
	}

	// Initialize MCP
	initMsg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      0,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]interface{}{},
		},
	}

	initBytes, _ := json.Marshal(initMsg)
	initLine := string(initBytes) + "\n"
	s.client.stdin.WriteString(initLine)

	// Read initialization response
	s.client.reader.ReadString('\n')
}

// createTempConfig creates a temporary configuration file for Go LSP
func (s *MCPLSPToolsE2ETestSuite) createTempConfig(port int) string {
	configContent := fmt.Sprintf(`
servers:
- name: go-lsp
  languages:
  - go
  command: gopls
  args: []
  transport: stdio
  root_markers:
  - go.mod
  - go.sum
  priority: 1
  weight: 1.0
port: %d
timeout: 45s
max_concurrent_requests: 150
`, port)

	tempFile, _, err := testutils.CreateTempConfig(configContent)
	if err != nil {
		s.T().Fatalf("Failed to create temp config: %v", err)
	}
	return tempFile
}

// getFileURI returns the file URI for a given file in the workspace
func (s *MCPLSPToolsE2ETestSuite) getFileURI(filename string) string {
	if file, exists := s.workspace.Files[filename]; exists {
		return fmt.Sprintf("file://%s", file.Path)
	}
	return fmt.Sprintf("file://%s/%s", s.workspace.RootPath, filename)
}

// recordMetric records a test metric
func (s *MCPLSPToolsE2ETestSuite) recordMetric(toolName string, responseTime time.Duration, success bool) {
	s.metrics.mutex.Lock()
	defer s.metrics.mutex.Unlock()

	if s.metrics.ResponseTimes[toolName] == nil {
		s.metrics.ResponseTimes[toolName] = make([]time.Duration, 0)
	}
	s.metrics.ResponseTimes[toolName] = append(s.metrics.ResponseTimes[toolName], responseTime)

	if success {
		s.metrics.SuccessCount[toolName]++
	} else {
		s.metrics.ErrorCount[toolName]++
	}
	s.metrics.TotalTests++
}

// Validation helper methods

// assertMCPResponse asserts that an MCP response is valid
func (s *MCPLSPToolsE2ETestSuite) assertMCPResponse(response *MCPResponse, toolName string) {
	s.NotNil(response, "MCP response should not be nil for %s", toolName)
	s.False(response.IsError, "MCP response should not be an error for %s", toolName)
	s.NotEmpty(response.Content, "MCP response content should not be empty for %s", toolName)
}

// Type definitions for expected responses

type ExpectedLocation struct {
	URI   string   `json:"uri"`
	Range LSPRange `json:"range"`
}

type LSPRange struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// validateDefinitionResponse validates a goto definition response
func (s *MCPLSPToolsE2ETestSuite) validateDefinitionResponse(response *MCPResponse, expected *ExpectedLocation) {
	s.assertMCPResponse(response, "goto_definition")
	// Additional validation logic for definition responses would go here
	// For now, we just ensure the response is not empty and properly formatted
}

// validateReferencesResponse validates a find references response
func (s *MCPLSPToolsE2ETestSuite) validateReferencesResponse(response *MCPResponse) []interface{} {
	s.assertMCPResponse(response, "find_references")
	
	// Extract references from response data
	references := make([]interface{}, 0)
	if len(response.Content) > 0 && response.Content[0].Data != nil {
		if data, ok := response.Content[0].Data.([]interface{}); ok {
			references = data
		}
	}
	return references
}

// validateHoverResponse validates a hover response
func (s *MCPLSPToolsE2ETestSuite) validateHoverResponse(response *MCPResponse) string {
	s.assertMCPResponse(response, "get_hover_info")
	
	// Extract hover text from response
	if len(response.Content) > 0 {
		return response.Content[0].Text
	}
	return ""
}

// validateEmptyHoverResponse validates that hover response is empty
func (s *MCPLSPToolsE2ETestSuite) validateEmptyHoverResponse(response *MCPResponse) {
	s.NotNil(response, "Response should not be nil")
	// Empty hover responses are still valid, just with no meaningful content
}

// validateDocumentSymbolsResponse validates a document symbols response
func (s *MCPLSPToolsE2ETestSuite) validateDocumentSymbolsResponse(response *MCPResponse) []interface{} {
	s.assertMCPResponse(response, "get_document_symbols")
	
	// Extract symbols from response data
	symbols := make([]interface{}, 0)
	if len(response.Content) > 0 && response.Content[0].Data != nil {
		if data, ok := response.Content[0].Data.([]interface{}); ok {
			symbols = data
		}
	}
	return symbols
}

// validateWorkspaceSymbolsResponse validates a workspace symbols response
func (s *MCPLSPToolsE2ETestSuite) validateWorkspaceSymbolsResponse(response *MCPResponse) []interface{} {
	s.assertMCPResponse(response, "search_workspace_symbols")
	
	// Extract symbols from response data
	symbols := make([]interface{}, 0)
	if len(response.Content) > 0 && response.Content[0].Data != nil {
		if data, ok := response.Content[0].Data.([]interface{}); ok {
			symbols = data
		}
	}
	return symbols
}

// validateCompletionsResponse validates a completions response
func (s *MCPLSPToolsE2ETestSuite) validateCompletionsResponse(response *MCPResponse) []interface{} {
	s.assertMCPResponse(response, "get_completions")
	
	// Extract completions from response data
	completions := make([]interface{}, 0)
	if len(response.Content) > 0 && response.Content[0].Data != nil {
		if data, ok := response.Content[0].Data.([]interface{}); ok {
			completions = data
		}
	}
	return completions
}

// Test suite runner
func TestMCPLSPToolsE2ETestSuite(t *testing.T) {
	suite.Run(t, new(MCPLSPToolsE2ETestSuite))
}