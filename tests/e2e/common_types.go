package e2e_test

import (
	"time"

	lsp "lsp-gateway/internal/models/lsp"
)

// TypeScriptRefactoringResult captures results from TypeScript refactoring operations
type TypeScriptRefactoringResult struct {
	RenameSuccess             bool
	ExtractInterfaceSuccess   bool
	MoveSymbolSuccess         bool
	ImportOrganizationSuccess bool
	CrossFileChangesCount     int
	ValidationSuccess         bool
	ErrorCount                int
	RefactoringLatency        time.Duration
}

// TypeScriptAdvancedResult captures results from advanced TypeScript operations
type TypeScriptAdvancedResult struct {
	DefinitionAccuracy      float64
	ReferenceCount          int
	CrossModuleReferences   int
	TypeInferenceAccuracy   float64
	CompletionQuality       float64
	SymbolNavigationSuccess bool
	ErrorCount              int
	PerformanceMetrics      map[string]interface{}
}

// TypeScriptAdvancedFeaturesResult captures modern TypeScript 5.x+ feature results
type TypeScriptAdvancedFeaturesResult struct {
	DecoratorSupport         bool
	SatisfiesOperatorSupport bool
	TemplateLiteralSupport   bool
	ConditionalTypesSupport  bool
	TupleTypesSupport        bool
	BrandedTypesSupport      bool
	ModernFeatureAccuracy    float64
	ErrorCount               int
}

// TypeScriptBuildConfigResult captures results from build configuration hot reload testing
type TypeScriptBuildConfigResult struct {
	ConfigChangeDetected      bool
	TsconfigReloadSuccess     bool
	PathMappingUpdateSuccess  bool
	IncrementalCompileSuccess bool
	FrameworkConfigSuccess    bool
	ErrorCount                int
	RevalidationLatency       time.Duration
}

// TestResult represents test results for E2E tests
// Shared across multiple test suites to avoid redeclaration
type TestResult struct {
	Method   string
	File     string
	Success  bool
	Duration time.Duration
	Error    error
	Response interface{}
}

// MCPToolCallParams represents parameters for MCP tool calls
// Shared across MCP test suites to avoid redeclaration
type MCPToolCallParams struct {
	URI       string `json:"uri,omitempty"`
	Line      int    `json:"line,omitempty"`
	Character int    `json:"character,omitempty"`
	Query     string `json:"query,omitempty"`
}

// PythonSymbol represents a Python symbol
// Shared across Python test suites to avoid redeclaration
type PythonSymbol struct {
	Name        string
	Kind        string
	Position    lsp.Position
	Type        string
	Description string
	Line        int
	Character   int
}

// PythonIntegrationResult is defined in python_e2e_comprehensive_test.go to avoid redeclaration
