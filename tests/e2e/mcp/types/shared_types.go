package types

import (
	"time"
)

// LSPPosition represents a position in LSP coordinate system (0-based)
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// LSPRange represents a range in LSP coordinate system
type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

// LSPLocation represents a location with URI and range
type LSPLocation struct {
	URI   string   `json:"uri"`
	Range LSPRange `json:"range"`
}

// HoverContent represents hover content with kind information
type HoverContent struct {
	Value string `json:"value"`
	Kind  string `json:"kind,omitempty"`
}

// DocumentSymbol represents a document symbol with hierarchical structure
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Kind           int              `json:"kind"`
	Range          LSPRange         `json:"range"`
	SelectionRange LSPRange         `json:"selectionRange"`
	Detail         string           `json:"detail,omitempty"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// CompletionItem represents a completion item
type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
	InsertText    string `json:"insertText,omitempty"`
}

// ExpectedResult contains expected test results for various LSP operations
type ExpectedResult struct {
	Type            string           `json:"type"`
	Location        *LSPLocation     `json:"location,omitempty"`
	Locations       []LSPLocation    `json:"locations,omitempty"`
	Content         string           `json:"content,omitempty"`
	HoverContent    *HoverContent    `json:"hoverContent,omitempty"`
	Symbols         []DocumentSymbol `json:"symbols,omitempty"`
	Query           string           `json:"query,omitempty"`
	CompletionItems []string         `json:"completionItems,omitempty"`
	Items           []CompletionItem `json:"items,omitempty"`
}

// SymbolInfo contains detailed symbol information
type SymbolInfo struct {
	Name          string        `json:"name"`
	Kind          int           `json:"kind"`
	Location      LSPLocation   `json:"location"`
	Definition    LSPLocation   `json:"definition"`
	References    []LSPLocation `json:"references"`
	Documentation string        `json:"documentation"`
	Type          string        `json:"type"`
	Package       string        `json:"package"`
	IsExported    bool          `json:"isExported"`
}

// WorkspaceFile represents a file in the test workspace
type WorkspaceFile struct {
	Path         string    `json:"path"`
	Content      string    `json:"content"`
	Language     string    `json:"language"`
	LastModified time.Time `json:"lastModified"`
}

// TestWorkspace manages the test workspace and repository
type TestWorkspace struct {
	ID             string                    `json:"id"`
	Language       string                    `json:"language"`
	ProjectType    string                    `json:"projectType"`
	RootPath       string                    `json:"rootPath"`
	RepositoryPath string                    `json:"repositoryPath"`
	RepositoryURL  string                    `json:"repositoryUrl"`
	CommitHash     string                    `json:"commitHash"`
	ConfigPath     string                    `json:"configPath"`
	TempDir        string                    `json:"tempDir"`
	Files          map[string]*WorkspaceFile `json:"files"`
	Symbols        map[string]*SymbolInfo    `json:"symbols"`

	// LSP Configuration
	LSPConfig *LSPServerConfig `json:"lspConfig,omitempty"`

	// Test validation data  
	ExpectedSymbols    []ExpectedSymbol    `json:"expectedSymbols,omitempty"`
	ExpectedReferences []ExpectedReference `json:"expectedReferences,omitempty"`
	ExpectedHovers     []ExpectedHover     `json:"expectedHovers,omitempty"`

	// Workspace state
	CreatedAt     time.Time     `json:"createdAt"`
	IsInitialized bool          `json:"isInitialized"`
	CleanupFunc   func() error  `json:"-"`
}

// LSPServerConfig contains LSP server configuration
type LSPServerConfig struct {
	ServerPath   string                 `json:"serverPath"`
	Args         []string               `json:"args"`
	InitOptions  map[string]interface{} `json:"initOptions"`
	WorkspaceDir string                 `json:"workspaceDir"`
	Language     string                 `json:"language"`
}

// ExpectedSymbol represents expected symbol data for testing
type ExpectedSymbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Location  string `json:"location"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}

// ExpectedReference represents expected reference data for testing
type ExpectedReference struct {
	Symbol    string `json:"symbol"`
	FilePath  string `json:"filePath"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}

// ExpectedHover represents expected hover data for testing
type ExpectedHover struct {
	Position     LSPPosition `json:"position"`
	ExpectedText string      `json:"expectedText"`
	FilePath     string      `json:"filePath"`
}

// ValidationConfig contains validation configuration for test case building
type ValidationConfig struct {
	ValidateSymbols    bool `json:"validateSymbols"`
	ValidateReferences bool `json:"validateReferences"`
	ValidateHover      bool `json:"validateHover"`
}

// ResponseValidationConfig contains validation configuration for response validation
type ResponseValidationConfig struct {
	StrictMode              bool          `json:"strictMode"`
	ValidatePerformance     bool          `json:"validatePerformance"`
	MaxResponseTime         time.Duration `json:"maxResponseTime"`
	RequireExactFieldMatch  bool          `json:"requireExactFieldMatch"`
	ValidateURIFormat       bool          `json:"validateUriFormat"`
	ValidateRangeOrdering   bool          `json:"validateRangeOrdering"`
	ValidateSymbolHierarchy bool          `json:"validateSymbolHierarchy"`
	MaxItems                int           `json:"maxItems"`
}

// MCPTestConfig holds test configuration for MCP test suites
type MCPTestConfig struct {
	GatewayPort     int                `json:"gatewayPort"`
	ServerTimeout   time.Duration      `json:"serverTimeout"`
	RequestTimeout  time.Duration      `json:"requestTimeout"`
	WorkspaceConfig *TestWorkspace     `json:"workspaceConfig,omitempty"`
}

// TestPriority defines the priority level for test cases
type TestPriority int

const (
	HighPriority TestPriority = iota
	MediumPriority
	LowPriority
)

func (p TestPriority) String() string {
	switch p {
	case HighPriority:
		return "high"
	case MediumPriority:
		return "medium"
	case LowPriority:
		return "low"
	default:
		return "unknown"
	}
}

// TestCase represents a single test case for MCP testing
type TestCase struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Tool        string          `json:"tool"`
	Language    string          `json:"language"`
	File        string          `json:"file"`
	Position    LSPPosition     `json:"position"`
	Expected    *ExpectedResult `json:"expected"`
	Tags        []string        `json:"tags"`
	Priority    TestPriority    `json:"priority"`
}