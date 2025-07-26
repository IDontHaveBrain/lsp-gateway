package e2e_test

import "time"

// LSPPosition represents a position in a text document
// Shared across all language-specific E2E tests
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// TypeScriptRefactoringResult captures results from TypeScript refactoring operations
type TypeScriptRefactoringResult struct {
	RenameSuccess            bool
	ExtractInterfaceSuccess  bool
	MoveSymbolSuccess        bool
	ImportOrganizationSuccess bool
	CrossFileChangesCount    int
	ValidationSuccess        bool
	ErrorCount               int
	RefactoringLatency       time.Duration
}

// TypeScriptAdvancedResult captures results from advanced TypeScript operations
type TypeScriptAdvancedResult struct {
	DefinitionAccuracy       float64
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
	DecoratorSupport        bool
	SatisfiesOperatorSupport bool
	TemplateLiteralSupport  bool
	ConditionalTypesSupport bool
	TupleTypesSupport       bool
	BrandedTypesSupport     bool
	ModernFeatureAccuracy   float64
	ErrorCount              int
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

// PythonIntegrationResult is defined in python_real_pylsp_integration_test.go to avoid redeclaration