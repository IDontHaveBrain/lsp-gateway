package assertions

// ExpectedPosition represents expected position values for validation
type ExpectedPosition struct {
	Line      *int `json:"line,omitempty"`
	Character *int `json:"character,omitempty"`
}

// ExpectedRange represents expected range values for validation
type ExpectedRange struct {
	Start *ExpectedPosition `json:"start,omitempty"`
	End   *ExpectedPosition `json:"end,omitempty"`
}

// ExpectedURI represents expected URI values for validation
type ExpectedURI struct {
	Exact    *string `json:"exact,omitempty"`    // Exact URI match
	Contains *string `json:"contains,omitempty"` // URI must contain this substring
	Pattern  *string `json:"pattern,omitempty"`  // URI must match this regex pattern
}

// ExpectedLocation represents expected location values for validation
type ExpectedLocation struct {
	URI   *ExpectedURI   `json:"uri,omitempty"`
	Range *ExpectedRange `json:"range,omitempty"`
}

// ExpectedArrayLength represents expected array length constraints
type ExpectedArrayLength struct {
	Exact *int `json:"exact,omitempty"` // Exact length
	Min   *int `json:"min,omitempty"`   // Minimum length
	Max   *int `json:"max,omitempty"`   // Maximum length
}

// ExpectedDefinitionResult represents expected definition response
type ExpectedDefinitionResult struct {
	HasResult    bool                  `json:"has_result"`
	IsArray      *bool                 `json:"is_array,omitempty"`
	ArrayLength  *ExpectedArrayLength  `json:"array_length,omitempty"`
	Locations    []*ExpectedLocation   `json:"locations,omitempty"`
	FirstLocation *ExpectedLocation    `json:"first_location,omitempty"`
}

// ExpectedReferencesResult represents expected references response
type ExpectedReferencesResult struct {
	HasResult        bool                 `json:"has_result"`
	ArrayLength      *ExpectedArrayLength `json:"array_length,omitempty"`
	IncludesDeclaration bool              `json:"includes_declaration"`
	Locations        []*ExpectedLocation  `json:"locations,omitempty"`
	ContainsFile     *string              `json:"contains_file,omitempty"`
}

// ExpectedHoverContent represents expected hover content patterns
type ExpectedHoverContent struct {
	HasContent  bool     `json:"has_content"`
	Contains    []string `json:"contains,omitempty"`
	Matches     []string `json:"matches,omitempty"`     // Regex patterns
	Excludes    []string `json:"excludes,omitempty"`
	MarkupKind  *string  `json:"markup_kind,omitempty"` // "plaintext" or "markdown"
	MinLength   *int     `json:"min_length,omitempty"`
	MaxLength   *int     `json:"max_length,omitempty"`
}

// ExpectedHoverResult represents expected hover response
type ExpectedHoverResult struct {
	HasResult bool                  `json:"has_result"`
	Content   *ExpectedHoverContent `json:"content,omitempty"`
	Range     *ExpectedRange        `json:"range,omitempty"`
}

// ExpectedSymbolKind represents expected symbol kind constants
type ExpectedSymbolKind int

// LSP Symbol kinds (from LSP specification)
const (
	SymbolKindFile          ExpectedSymbolKind = 1
	SymbolKindModule        ExpectedSymbolKind = 2
	SymbolKindNamespace     ExpectedSymbolKind = 3
	SymbolKindPackage       ExpectedSymbolKind = 4
	SymbolKindClass         ExpectedSymbolKind = 5
	SymbolKindMethod        ExpectedSymbolKind = 6
	SymbolKindProperty      ExpectedSymbolKind = 7
	SymbolKindField         ExpectedSymbolKind = 8
	SymbolKindConstructor   ExpectedSymbolKind = 9
	SymbolKindEnum          ExpectedSymbolKind = 10
	SymbolKindInterface     ExpectedSymbolKind = 11
	SymbolKindFunction      ExpectedSymbolKind = 12
	SymbolKindVariable      ExpectedSymbolKind = 13
	SymbolKindConstant      ExpectedSymbolKind = 14
	SymbolKindString        ExpectedSymbolKind = 15
	SymbolKindNumber        ExpectedSymbolKind = 16
	SymbolKindBoolean       ExpectedSymbolKind = 17
	SymbolKindArray         ExpectedSymbolKind = 18
	SymbolKindObject        ExpectedSymbolKind = 19
	SymbolKindKey           ExpectedSymbolKind = 20
	SymbolKindNull          ExpectedSymbolKind = 21
	SymbolKindEnumMember    ExpectedSymbolKind = 22
	SymbolKindStruct        ExpectedSymbolKind = 23
	SymbolKindEvent         ExpectedSymbolKind = 24
	SymbolKindOperator      ExpectedSymbolKind = 25
	SymbolKindTypeParameter ExpectedSymbolKind = 26
)

// String returns the string representation of a symbol kind
func (k ExpectedSymbolKind) String() string {
	kindMap := map[ExpectedSymbolKind]string{
		SymbolKindFile:          "File",
		SymbolKindModule:        "Module",
		SymbolKindNamespace:     "Namespace",
		SymbolKindPackage:       "Package",
		SymbolKindClass:         "Class",
		SymbolKindMethod:        "Method",
		SymbolKindProperty:      "Property",
		SymbolKindField:         "Field",
		SymbolKindConstructor:   "Constructor",
		SymbolKindEnum:          "Enum",
		SymbolKindInterface:     "Interface",
		SymbolKindFunction:      "Function",
		SymbolKindVariable:      "Variable",
		SymbolKindConstant:      "Constant",
		SymbolKindString:        "String",
		SymbolKindNumber:        "Number",
		SymbolKindBoolean:       "Boolean",
		SymbolKindArray:         "Array",
		SymbolKindObject:        "Object",
		SymbolKindKey:           "Key",
		SymbolKindNull:          "Null",
		SymbolKindEnumMember:    "EnumMember",
		SymbolKindStruct:        "Struct",
		SymbolKindEvent:         "Event",
		SymbolKindOperator:      "Operator",
		SymbolKindTypeParameter: "TypeParameter",
	}
	
	if name, exists := kindMap[k]; exists {
		return name
	}
	return "Unknown"
}

// ExpectedSymbol represents expected symbol properties
type ExpectedSymbol struct {
	Name           *string             `json:"name,omitempty"`
	Kind           *ExpectedSymbolKind `json:"kind,omitempty"`
	Location       *ExpectedLocation   `json:"location,omitempty"`
	ContainerName  *string             `json:"container_name,omitempty"`
	NameContains   []string            `json:"name_contains,omitempty"`
	NameMatches    []string            `json:"name_matches,omitempty"`    // Regex patterns
	NameExcludes   []string            `json:"name_excludes,omitempty"`
	Detail         *string             `json:"detail,omitempty"`
	DetailContains []string            `json:"detail_contains,omitempty"`
	DetailMatches  []string            `json:"detail_matches,omitempty"`  // Regex patterns
	Tags           []int               `json:"tags,omitempty"`             // Symbol tags (deprecated, etc.)
	Children       []*ExpectedSymbol   `json:"children,omitempty"`        // For document symbols
}

// ExpectedDocumentSymbolResult represents expected document symbol response
type ExpectedDocumentSymbolResult struct {
	HasResult      bool                 `json:"has_result"`
	ArrayLength    *ExpectedArrayLength `json:"array_length,omitempty"`
	Symbols        []*ExpectedSymbol    `json:"symbols,omitempty"`
	HasHierarchy   *bool                `json:"has_hierarchy,omitempty"`   // True if symbols have children
	TopLevelCount  *int                 `json:"top_level_count,omitempty"` // Count of top-level symbols
	ContainsSymbol *ExpectedSymbol      `json:"contains_symbol,omitempty"` // At least one symbol matching these criteria
}

// ExpectedWorkspaceSymbolResult represents expected workspace symbol response
type ExpectedWorkspaceSymbolResult struct {
	HasResult      bool                 `json:"has_result"`
	ArrayLength    *ExpectedArrayLength `json:"array_length,omitempty"`
	Symbols        []*ExpectedSymbol    `json:"symbols,omitempty"`
	ContainsSymbol *ExpectedSymbol      `json:"contains_symbol,omitempty"` // At least one symbol matching these criteria
	FromFiles      []string             `json:"from_files,omitempty"`      // Symbols should come from these files
}

// ExpectedErrorResult represents expected error response
type ExpectedErrorResult struct {
	HasError      bool    `json:"has_error"`
	Code          *int    `json:"code,omitempty"`
	Message       *string `json:"message,omitempty"`
	MessageContains []string `json:"message_contains,omitempty"`
	MessageMatches  []string `json:"message_matches,omitempty"` // Regex patterns
}

// ComprehensiveExpectedResult aggregates all possible expected results
type ComprehensiveExpectedResult struct {
	// General expectations
	Success       bool                   `json:"success"`
	Error         *ExpectedErrorResult   `json:"error,omitempty"`
	ResponseTime  *int                   `json:"max_response_time_ms,omitempty"`
	
	// Content validation
	Contains      []string               `json:"contains,omitempty"`
	Matches       []string               `json:"matches,omitempty"`     // Regex patterns
	Excludes      []string               `json:"excludes,omitempty"`
	
	// Method-specific expectations
	Definition    *ExpectedDefinitionResult      `json:"definition,omitempty"`
	References    *ExpectedReferencesResult      `json:"references,omitempty"`
	Hover         *ExpectedHoverResult           `json:"hover,omitempty"`
	DocumentSymbol *ExpectedDocumentSymbolResult `json:"document_symbol,omitempty"`
	WorkspaceSymbol *ExpectedWorkspaceSymbolResult `json:"workspace_symbol,omitempty"`
	
	// Custom properties for extensibility
	Properties    map[string]interface{} `json:"properties,omitempty"`
}

// ExpectedResultBuilder provides a builder interface for creating expected results
type ExpectedResultBuilder struct {
	result *ComprehensiveExpectedResult
}

// NewExpectedResult creates a new expected result builder
func NewExpectedResult() *ExpectedResultBuilder {
	return &ExpectedResultBuilder{
		result: &ComprehensiveExpectedResult{
			Success: true,
		},
	}
}

// Build returns the built expected result
func (b *ExpectedResultBuilder) Build() *ComprehensiveExpectedResult {
	return b.result
}

// Success sets the success expectation
func (b *ExpectedResultBuilder) Success(success bool) *ExpectedResultBuilder {
	b.result.Success = success
	return b
}

// WithError sets error expectations
func (b *ExpectedResultBuilder) WithError(hasError bool) *ExpectedResultBuilder {
	b.result.Error = &ExpectedErrorResult{
		HasError: hasError,
	}
	return b
}

// WithErrorCode sets expected error code
func (b *ExpectedResultBuilder) WithErrorCode(code int) *ExpectedResultBuilder {
	if b.result.Error == nil {
		b.result.Error = &ExpectedErrorResult{HasError: true}
	}
	b.result.Error.Code = &code
	return b
}

// WithErrorMessage sets expected error message
func (b *ExpectedResultBuilder) WithErrorMessage(message string) *ExpectedResultBuilder {
	if b.result.Error == nil {
		b.result.Error = &ExpectedErrorResult{HasError: true}
	}
	b.result.Error.Message = &message
	return b
}

// WithErrorMessageContains sets expected error message substrings
func (b *ExpectedResultBuilder) WithErrorMessageContains(patterns ...string) *ExpectedResultBuilder {
	if b.result.Error == nil {
		b.result.Error = &ExpectedErrorResult{HasError: true}
	}
	b.result.Error.MessageContains = append(b.result.Error.MessageContains, patterns...)
	return b
}

// WithContains adds content patterns that should be present
func (b *ExpectedResultBuilder) WithContains(patterns ...string) *ExpectedResultBuilder {
	b.result.Contains = append(b.result.Contains, patterns...)
	return b
}

// WithMatches adds regex patterns that should match
func (b *ExpectedResultBuilder) WithMatches(patterns ...string) *ExpectedResultBuilder {
	b.result.Matches = append(b.result.Matches, patterns...)
	return b
}

// WithExcludes adds patterns that should not be present
func (b *ExpectedResultBuilder) WithExcludes(patterns ...string) *ExpectedResultBuilder {
	b.result.Excludes = append(b.result.Excludes, patterns...)
	return b
}

// WithMaxResponseTime sets maximum expected response time in milliseconds
func (b *ExpectedResultBuilder) WithMaxResponseTime(ms int) *ExpectedResultBuilder {
	b.result.ResponseTime = &ms
	return b
}

// WithProperty adds a custom property for validation
func (b *ExpectedResultBuilder) WithProperty(key string, value interface{}) *ExpectedResultBuilder {
	if b.result.Properties == nil {
		b.result.Properties = make(map[string]interface{})
	}
	b.result.Properties[key] = value
	return b
}

// Definition method builders

// WithDefinition sets definition-specific expectations
func (b *ExpectedResultBuilder) WithDefinition(hasResult bool) *ExpectedResultBuilder {
	b.result.Definition = &ExpectedDefinitionResult{
		HasResult: hasResult,
	}
	return b
}

// WithDefinitionLocation adds an expected definition location
func (b *ExpectedResultBuilder) WithDefinitionLocation(uri string, startLine, startChar, endLine, endChar int) *ExpectedResultBuilder {
	if b.result.Definition == nil {
		b.result.Definition = &ExpectedDefinitionResult{HasResult: true}
	}
	
	location := &ExpectedLocation{
		URI: &ExpectedURI{Exact: &uri},
		Range: &ExpectedRange{
			Start: &ExpectedPosition{Line: &startLine, Character: &startChar},
			End:   &ExpectedPosition{Line: &endLine, Character: &endChar},
		},
	}
	
	if b.result.Definition.FirstLocation == nil {
		b.result.Definition.FirstLocation = location
	}
	b.result.Definition.Locations = append(b.result.Definition.Locations, location)
	return b
}

// WithDefinitionArray sets expectation for definition array response
func (b *ExpectedResultBuilder) WithDefinitionArray(minCount, maxCount int) *ExpectedResultBuilder {
	if b.result.Definition == nil {
		b.result.Definition = &ExpectedDefinitionResult{HasResult: true}
	}
	
	isArray := true
	b.result.Definition.IsArray = &isArray
	b.result.Definition.ArrayLength = &ExpectedArrayLength{
		Min: &minCount,
		Max: &maxCount,
	}
	return b
}

// References method builders

// WithReferences sets references-specific expectations
func (b *ExpectedResultBuilder) WithReferences(hasResult bool) *ExpectedResultBuilder {
	b.result.References = &ExpectedReferencesResult{
		HasResult: hasResult,
	}
	return b
}

// WithReferencesCount sets expected references count bounds
func (b *ExpectedResultBuilder) WithReferencesCount(minCount, maxCount int) *ExpectedResultBuilder {
	if b.result.References == nil {
		b.result.References = &ExpectedReferencesResult{HasResult: true}
	}
	
	b.result.References.ArrayLength = &ExpectedArrayLength{
		Min: &minCount,
		Max: &maxCount,
	}
	return b
}

// WithReferencesIncludeDeclaration sets expectation for declaration inclusion
func (b *ExpectedResultBuilder) WithReferencesIncludeDeclaration(includeDecl bool) *ExpectedResultBuilder {
	if b.result.References == nil {
		b.result.References = &ExpectedReferencesResult{HasResult: true}
	}
	
	b.result.References.IncludesDeclaration = includeDecl
	return b
}

// WithReferencesFromFile sets expectation for references from specific file
func (b *ExpectedResultBuilder) WithReferencesFromFile(filename string) *ExpectedResultBuilder {
	if b.result.References == nil {
		b.result.References = &ExpectedReferencesResult{HasResult: true}
	}
	
	b.result.References.ContainsFile = &filename
	return b
}

// Hover method builders

// WithHover sets hover-specific expectations
func (b *ExpectedResultBuilder) WithHover(hasResult bool) *ExpectedResultBuilder {
	b.result.Hover = &ExpectedHoverResult{
		HasResult: hasResult,
	}
	return b
}

// WithHoverContent sets expected hover content patterns
func (b *ExpectedResultBuilder) WithHoverContent(hasContent bool, contains ...string) *ExpectedResultBuilder {
	if b.result.Hover == nil {
		b.result.Hover = &ExpectedHoverResult{HasResult: true}
	}
	
	b.result.Hover.Content = &ExpectedHoverContent{
		HasContent: hasContent,
		Contains:   contains,
	}
	return b
}

// WithHoverMarkupKind sets expected hover markup kind
func (b *ExpectedResultBuilder) WithHoverMarkupKind(kind string) *ExpectedResultBuilder {
	if b.result.Hover == nil {
		b.result.Hover = &ExpectedHoverResult{HasResult: true}
	}
	if b.result.Hover.Content == nil {
		b.result.Hover.Content = &ExpectedHoverContent{HasContent: true}
	}
	
	b.result.Hover.Content.MarkupKind = &kind
	return b
}

// Document Symbol method builders

// WithDocumentSymbol sets document symbol expectations
func (b *ExpectedResultBuilder) WithDocumentSymbol(hasResult bool) *ExpectedResultBuilder {
	b.result.DocumentSymbol = &ExpectedDocumentSymbolResult{
		HasResult: hasResult,
	}
	return b
}

// WithDocumentSymbolCount sets expected document symbol count bounds
func (b *ExpectedResultBuilder) WithDocumentSymbolCount(minCount, maxCount int) *ExpectedResultBuilder {
	if b.result.DocumentSymbol == nil {
		b.result.DocumentSymbol = &ExpectedDocumentSymbolResult{HasResult: true}
	}
	
	b.result.DocumentSymbol.ArrayLength = &ExpectedArrayLength{
		Min: &minCount,
		Max: &maxCount,
	}
	return b
}

// WithDocumentSymbolOfKind adds expectation for symbol of specific kind
func (b *ExpectedResultBuilder) WithDocumentSymbolOfKind(name string, kind ExpectedSymbolKind) *ExpectedResultBuilder {
	if b.result.DocumentSymbol == nil {
		b.result.DocumentSymbol = &ExpectedDocumentSymbolResult{HasResult: true}
	}
	
	symbol := &ExpectedSymbol{
		Name: &name,
		Kind: &kind,
	}
	
	if b.result.DocumentSymbol.ContainsSymbol == nil {
		b.result.DocumentSymbol.ContainsSymbol = symbol
	}
	b.result.DocumentSymbol.Symbols = append(b.result.DocumentSymbol.Symbols, symbol)
	return b
}

// Workspace Symbol method builders

// WithWorkspaceSymbol sets workspace symbol expectations
func (b *ExpectedResultBuilder) WithWorkspaceSymbol(hasResult bool) *ExpectedResultBuilder {
	b.result.WorkspaceSymbol = &ExpectedWorkspaceSymbolResult{
		HasResult: hasResult,
	}
	return b
}

// WithWorkspaceSymbolCount sets expected workspace symbol count bounds
func (b *ExpectedResultBuilder) WithWorkspaceSymbolCount(minCount, maxCount int) *ExpectedResultBuilder {
	if b.result.WorkspaceSymbol == nil {
		b.result.WorkspaceSymbol = &ExpectedWorkspaceSymbolResult{HasResult: true}
	}
	
	b.result.WorkspaceSymbol.ArrayLength = &ExpectedArrayLength{
		Min: &minCount,
		Max: &maxCount,
	}
	return b
}

// WithWorkspaceSymbolOfKind adds expectation for workspace symbol of specific kind
func (b *ExpectedResultBuilder) WithWorkspaceSymbolOfKind(name string, kind ExpectedSymbolKind) *ExpectedResultBuilder {
	if b.result.WorkspaceSymbol == nil {
		b.result.WorkspaceSymbol = &ExpectedWorkspaceSymbolResult{HasResult: true}
	}
	
	symbol := &ExpectedSymbol{
		Name: &name,
		Kind: &kind,
	}
	
	if b.result.WorkspaceSymbol.ContainsSymbol == nil {
		b.result.WorkspaceSymbol.ContainsSymbol = symbol
	}
	b.result.WorkspaceSymbol.Symbols = append(b.result.WorkspaceSymbol.Symbols, symbol)
	return b
}