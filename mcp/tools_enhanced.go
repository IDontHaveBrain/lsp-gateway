package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// LSP Protocol Structs for proper type validation

// LSP Position represents a position in a text document
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// LSPRange represents a range in a text document
type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

// LSPLocation represents a location inside a resource
type LSPLocation struct {
	URI   string   `json:"uri"`
	Range LSPRange `json:"range"`
}

// LSPMarkupContent represents markup content with kind and value
type LSPMarkupContent struct {
	Kind  string `json:"kind"`  // "plaintext" or "markdown"
	Value string `json:"value"`
}

// LSPHover represents hover information
type LSPHover struct {
	Contents interface{} `json:"contents"` // Can be string, MarkupContent, MarkedString, or array
	Range    *LSPRange   `json:"range,omitempty"`
}

// LSPSymbolKind represents the kind of a symbol
type LSPSymbolKind int

const (
	File          LSPSymbolKind = 1
	Module        LSPSymbolKind = 2
	Namespace     LSPSymbolKind = 3
	Package       LSPSymbolKind = 4
	Class         LSPSymbolKind = 5
	Method        LSPSymbolKind = 6
	Property      LSPSymbolKind = 7
	Field         LSPSymbolKind = 8
	Constructor   LSPSymbolKind = 9
	Enum          LSPSymbolKind = 10
	Interface     LSPSymbolKind = 11
	Function      LSPSymbolKind = 12
	Variable      LSPSymbolKind = 13
	Constant      LSPSymbolKind = 14
	String        LSPSymbolKind = 15
	Number        LSPSymbolKind = 16
	Boolean       LSPSymbolKind = 17
	Array         LSPSymbolKind = 18
	Object        LSPSymbolKind = 19
	Key           LSPSymbolKind = 20
	Null          LSPSymbolKind = 21
	EnumMember    LSPSymbolKind = 22
	Struct        LSPSymbolKind = 23
	Event         LSPSymbolKind = 24
	Operator      LSPSymbolKind = 25
	TypeParameter LSPSymbolKind = 26
)

// LSPSymbolInformation represents symbol information
type LSPSymbolInformation struct {
	Name          string        `json:"name"`
	Kind          LSPSymbolKind `json:"kind"`
	Tags          []int         `json:"tags,omitempty"`
	Deprecated    bool          `json:"deprecated,omitempty"`
	Location      LSPLocation   `json:"location"`
	ContainerName string        `json:"containerName,omitempty"`
}

// LSPDocumentSymbol represents a document symbol
type LSPDocumentSymbol struct {
	Name           string               `json:"name"`
	Detail         string               `json:"detail,omitempty"`
	Kind           LSPSymbolKind        `json:"kind"`
	Tags           []int                `json:"tags,omitempty"`
	Deprecated     bool                 `json:"deprecated,omitempty"`
	Range          LSPRange             `json:"range"`
	SelectionRange LSPRange             `json:"selectionRange"`
	Children       []*LSPDocumentSymbol `json:"children,omitempty"`
}

// LSPCompletionItemKind represents the kind of a completion item
type LSPCompletionItemKind int

const (
	Text          LSPCompletionItemKind = 1
	MethodComp    LSPCompletionItemKind = 2
	FunctionComp  LSPCompletionItemKind = 3
	ConstructorComp LSPCompletionItemKind = 4
	FieldComp     LSPCompletionItemKind = 5
	VariableComp  LSPCompletionItemKind = 6
	ClassComp     LSPCompletionItemKind = 7
	InterfaceComp LSPCompletionItemKind = 8
	ModuleComp    LSPCompletionItemKind = 9
	PropertyComp  LSPCompletionItemKind = 10
	UnitComp      LSPCompletionItemKind = 11
	ValueComp     LSPCompletionItemKind = 12
	EnumComp      LSPCompletionItemKind = 13
	KeywordComp   LSPCompletionItemKind = 14
	SnippetComp   LSPCompletionItemKind = 15
	ColorComp     LSPCompletionItemKind = 16
	FileComp      LSPCompletionItemKind = 17
	ReferenceComp LSPCompletionItemKind = 18
	FolderComp    LSPCompletionItemKind = 19
	EnumMemberComp LSPCompletionItemKind = 20
	ConstantComp  LSPCompletionItemKind = 21
	StructComp    LSPCompletionItemKind = 22
	EventComp     LSPCompletionItemKind = 23
	OperatorComp  LSPCompletionItemKind = 24
	TypeParameterComp LSPCompletionItemKind = 25
)

// LSPCompletionItem represents a completion item
type LSPCompletionItem struct {
	Label               string                 `json:"label"`
	Kind                LSPCompletionItemKind  `json:"kind,omitempty"`
	Tags                []int                  `json:"tags,omitempty"`
	Detail              string                 `json:"detail,omitempty"`
	Documentation       interface{}            `json:"documentation,omitempty"` // string or MarkupContent
	Deprecated          bool                   `json:"deprecated,omitempty"`
	Preselect           bool                   `json:"preselect,omitempty"`
	SortText            string                 `json:"sortText,omitempty"`
	FilterText          string                 `json:"filterText,omitempty"`
	InsertText          string                 `json:"insertText,omitempty"`
	InsertTextFormat    int                    `json:"insertTextFormat,omitempty"`
	InsertTextMode      int                    `json:"insertTextMode,omitempty"`
	AdditionalTextEdits []interface{}          `json:"additionalTextEdits,omitempty"`
	CommitCharacters    []string               `json:"commitCharacters,omitempty"`
	Command             interface{}            `json:"command,omitempty"`
	Data                interface{}            `json:"data,omitempty"`
}

// LSPCompletionList represents a completion list
type LSPCompletionList struct {
	IsIncomplete bool                 `json:"isIncomplete"`
	Items        []*LSPCompletionItem `json:"items"`
}

// Validation and parsing results
type ValidationResult struct {
	Valid   bool
	Error   error
	Data    interface{}
	Count   int
	Warnings []string
}

// Protocol-compliant parsing methods with comprehensive validation

func (h *ToolHandler) parseDefinitionResult(result []byte) interface{} {
	validation := h.validateDefinitionResponse(result)
	if !validation.Valid {
		return h.createParsingErrorResult("definition", validation.Error, string(result), validation.Warnings)
	}
	return validation.Data
}

func (h *ToolHandler) parseReferencesResult(result []byte) interface{} {
	validation := h.validateReferencesResponse(result)
	if !validation.Valid {
		return h.createParsingErrorResult("references", validation.Error, string(result), validation.Warnings)
	}
	return validation.Data
}

func (h *ToolHandler) parseHoverResult(result []byte) interface{} {
	validation := h.validateHoverResponse(result)
	if !validation.Valid {
		return h.createParsingErrorResult("hover", validation.Error, string(result), validation.Warnings)
	}
	return validation.Data
}

func (h *ToolHandler) parseDocumentSymbolsResult(result []byte) interface{} {
	validation := h.validateDocumentSymbolsResponse(result)
	if !validation.Valid {
		return h.createParsingErrorResult("document_symbols", validation.Error, string(result), validation.Warnings)
	}
	return validation.Data
}

func (h *ToolHandler) parseWorkspaceSymbolsResult(result []byte) interface{} {
	validation := h.validateWorkspaceSymbolsResponse(result)
	if !validation.Valid {
		return h.createParsingErrorResult("workspace_symbols", validation.Error, string(result), validation.Warnings)
	}
	return validation.Data
}

func (h *ToolHandler) parseCompletionResult(result []byte) interface{} {
	validation := h.validateCompletionResponse(result)
	if !validation.Valid {
		return h.createParsingErrorResult("completion", validation.Error, string(result), validation.Warnings)
	}
	return validation.Data
}

// LSP response validation methods

func (h *ToolHandler) validateDefinitionResponse(result []byte) ValidationResult {
	if len(result) == 0 || string(result) == "null" {
		return ValidationResult{
			Valid: true,
			Data:  []LSPLocation{},
			Count: 0,
		}
	}

	// Try to parse as array of locations first
	var locations []LSPLocation
	if err := json.Unmarshal(result, &locations); err == nil {
		// Validate each location
		warnings := []string{}
		validLocations := []LSPLocation{}
		
		for i, loc := range locations {
			if validationErr := h.validateLocation(loc); validationErr != nil {
				warnings = append(warnings, fmt.Sprintf("Location %d invalid: %v", i, validationErr))
				continue
			}
			validLocations = append(validLocations, loc)
		}

		return ValidationResult{
			Valid:    true,
			Data:     validLocations,
			Count:    len(validLocations),
			Warnings: warnings,
		}
	}

	// Try to parse as single location
	var location LSPLocation
	if err := json.Unmarshal(result, &location); err == nil {
		if validationErr := h.validateLocation(location); validationErr != nil {
			return ValidationResult{
				Valid: false,
				Error: fmt.Errorf("invalid location: %v", validationErr),
			}
		}
		return ValidationResult{
			Valid: true,
			Data:  []LSPLocation{location},
			Count: 1,
		}
	}

	return ValidationResult{
		Valid: false,
		Error: fmt.Errorf("invalid definition response format: expected Location or Location[]"),
	}
}

func (h *ToolHandler) validateReferencesResponse(result []byte) ValidationResult {
	if len(result) == 0 || string(result) == "null" {
		return ValidationResult{
			Valid: true,
			Data:  []LSPLocation{},
			Count: 0,
		}
	}

	var locations []LSPLocation
	if err := json.Unmarshal(result, &locations); err != nil {
		return ValidationResult{
			Valid: false,
			Error: fmt.Errorf("invalid references response format: expected Location[], got: %v", err),
		}
	}

	// Validate each location
	warnings := []string{}
	validLocations := []LSPLocation{}
	
	for i, loc := range locations {
		if validationErr := h.validateLocation(loc); validationErr != nil {
			warnings = append(warnings, fmt.Sprintf("Location %d invalid: %v", i, validationErr))
			continue
		}
		validLocations = append(validLocations, loc)
	}

	return ValidationResult{
		Valid:    true,
		Data:     validLocations,
		Count:    len(validLocations),
		Warnings: warnings,
	}
}

func (h *ToolHandler) validateHoverResponse(result []byte) ValidationResult {
	if len(result) == 0 || string(result) == "null" {
		return ValidationResult{
			Valid: true,
			Data:  nil,
			Count: 0,
		}
	}

	var hover LSPHover
	if err := json.Unmarshal(result, &hover); err != nil {
		return ValidationResult{
			Valid: false,
			Error: fmt.Errorf("invalid hover response format: %v", err),
		}
	}

	warnings := []string{}

	// Validate contents field (required)
	if hover.Contents == nil {
		return ValidationResult{
			Valid: false,
			Error: fmt.Errorf("hover contents field is required"),
		}
	}

	// Validate range if present
	if hover.Range != nil {
		if validationErr := h.validateRange(*hover.Range); validationErr != nil {
			warnings = append(warnings, fmt.Sprintf("Invalid hover range: %v", validationErr))
			hover.Range = nil // Remove invalid range
		}
	}

	// Normalize contents to a consistent format
	normalizedHover := h.normalizeHoverContents(hover)

	return ValidationResult{
		Valid:    true,
		Data:     normalizedHover,
		Count:    1,
		Warnings: warnings,
	}
}

func (h *ToolHandler) validateDocumentSymbolsResponse(result []byte) ValidationResult {
	if len(result) == 0 || string(result) == "null" {
		return ValidationResult{
			Valid: true,
			Data:  []interface{}{},
			Count: 0,
		}
	}

	// Try DocumentSymbol[] first (newer format)
	var documentSymbols []LSPDocumentSymbol
	if err := json.Unmarshal(result, &documentSymbols); err == nil {
		warnings := []string{}
		validSymbols := []LSPDocumentSymbol{}
		
		for i, sym := range documentSymbols {
			if validationErr := h.validateDocumentSymbol(sym); validationErr != nil {
				warnings = append(warnings, fmt.Sprintf("DocumentSymbol %d invalid: %v", i, validationErr))
				continue
			}
			validSymbols = append(validSymbols, sym)
		}

		return ValidationResult{
			Valid:    true,
			Data:     validSymbols,
			Count:    len(validSymbols),
			Warnings: warnings,
		}
	}

	// Try SymbolInformation[] (legacy format)
	var symbolInfos []LSPSymbolInformation
	if err := json.Unmarshal(result, &symbolInfos); err == nil {
		warnings := []string{}
		validSymbols := []LSPSymbolInformation{}
		
		for i, sym := range symbolInfos {
			if validationErr := h.validateSymbolInformation(sym); validationErr != nil {
				warnings = append(warnings, fmt.Sprintf("SymbolInformation %d invalid: %v", i, validationErr))
				continue
			}
			validSymbols = append(validSymbols, sym)
		}

		return ValidationResult{
			Valid:    true,
			Data:     validSymbols,
			Count:    len(validSymbols),
			Warnings: warnings,
		}
	}

	return ValidationResult{
		Valid: false,
		Error: fmt.Errorf("invalid document symbols response format: expected DocumentSymbol[] or SymbolInformation[]"),
	}
}

func (h *ToolHandler) validateWorkspaceSymbolsResponse(result []byte) ValidationResult {
	if len(result) == 0 || string(result) == "null" {
		return ValidationResult{
			Valid: true,
			Data:  []LSPSymbolInformation{},
			Count: 0,
		}
	}

	var symbols []LSPSymbolInformation
	if err := json.Unmarshal(result, &symbols); err != nil {
		return ValidationResult{
			Valid: false,
			Error: fmt.Errorf("invalid workspace symbols response format: expected SymbolInformation[], got: %v", err),
		}
	}

	warnings := []string{}
	validSymbols := []LSPSymbolInformation{}
	
	for i, sym := range symbols {
		if validationErr := h.validateSymbolInformation(sym); validationErr != nil {
			warnings = append(warnings, fmt.Sprintf("SymbolInformation %d invalid: %v", i, validationErr))
			continue
		}
		validSymbols = append(validSymbols, sym)
	}

	return ValidationResult{
		Valid:    true,
		Data:     validSymbols,
		Count:    len(validSymbols),
		Warnings: warnings,
	}
}

func (h *ToolHandler) validateCompletionResponse(result []byte) ValidationResult {
	if len(result) == 0 || string(result) == "null" {
		return ValidationResult{
			Valid: true,
			Data: LSPCompletionList{
				IsIncomplete: false,
				Items:        []*LSPCompletionItem{},
			},
			Count: 0,
		}
	}

	// Try CompletionList first
	var completionList LSPCompletionList
	if err := json.Unmarshal(result, &completionList); err == nil {
		return h.validateCompletionList(completionList)
	}

	// Try array of CompletionItem (legacy format)
	var items []*LSPCompletionItem
	if err := json.Unmarshal(result, &items); err != nil {
		return ValidationResult{
			Valid: false,
			Error: fmt.Errorf("invalid completion response format: expected CompletionList or CompletionItem[], got: %v", err),
		}
	}

	// Convert to CompletionList format
	completionList = LSPCompletionList{
		IsIncomplete: false,
		Items:        items,
	}

	return h.validateCompletionList(completionList)
}

func (h *ToolHandler) validateCompletionList(list LSPCompletionList) ValidationResult {
	warnings := []string{}
	validItems := []*LSPCompletionItem{}
	
	for i, item := range list.Items {
		if item == nil {
			warnings = append(warnings, fmt.Sprintf("CompletionItem %d is null", i))
			continue
		}

		if validationErr := h.validateCompletionItem(*item); validationErr != nil {
			warnings = append(warnings, fmt.Sprintf("CompletionItem %d invalid: %v", i, validationErr))
			continue
		}
		validItems = append(validItems, item)
	}

	validList := LSPCompletionList{
		IsIncomplete: list.IsIncomplete,
		Items:        validItems,
	}

	return ValidationResult{
		Valid:    true,
		Data:     validList,
		Count:    len(validItems),
		Warnings: warnings,
	}
}

// Individual validation functions

func (h *ToolHandler) validateLocation(loc LSPLocation) error {
	if strings.TrimSpace(loc.URI) == "" {
		return fmt.Errorf("location URI cannot be empty")
	}
	return h.validateRange(loc.Range)
}

func (h *ToolHandler) validateRange(r LSPRange) error {
	if err := h.validatePosition(r.Start); err != nil {
		return fmt.Errorf("invalid start position: %v", err)
	}
	if err := h.validatePosition(r.End); err != nil {
		return fmt.Errorf("invalid end position: %v", err)
	}
	if r.Start.Line > r.End.Line || (r.Start.Line == r.End.Line && r.Start.Character > r.End.Character) {
		return fmt.Errorf("start position must be before or equal to end position")
	}
	return nil
}

func (h *ToolHandler) validatePosition(pos LSPPosition) error {
	if pos.Line < 0 {
		return fmt.Errorf("line number must be non-negative, got %d", pos.Line)
	}
	if pos.Character < 0 {
		return fmt.Errorf("character position must be non-negative, got %d", pos.Character)
	}
	return nil
}

func (h *ToolHandler) validateDocumentSymbol(sym LSPDocumentSymbol) error {
	if strings.TrimSpace(sym.Name) == "" {
		return fmt.Errorf("symbol name cannot be empty")
	}
	if sym.Kind < 1 || sym.Kind > 26 {
		return fmt.Errorf("invalid symbol kind: %d", sym.Kind)
	}
	if err := h.validateRange(sym.Range); err != nil {
		return fmt.Errorf("invalid range: %v", err)
	}
	if err := h.validateRange(sym.SelectionRange); err != nil {
		return fmt.Errorf("invalid selection range: %v", err)
	}
	// Validate children recursively
	for i, child := range sym.Children {
		if child == nil {
			return fmt.Errorf("child symbol %d is null", i)
		}
		if err := h.validateDocumentSymbol(*child); err != nil {
			return fmt.Errorf("child symbol %d invalid: %v", i, err)
		}
	}
	return nil
}

func (h *ToolHandler) validateSymbolInformation(sym LSPSymbolInformation) error {
	if strings.TrimSpace(sym.Name) == "" {
		return fmt.Errorf("symbol name cannot be empty")
	}
	if sym.Kind < 1 || sym.Kind > 26 {
		return fmt.Errorf("invalid symbol kind: %d", sym.Kind)
	}
	return h.validateLocation(sym.Location)
}

func (h *ToolHandler) validateCompletionItem(item LSPCompletionItem) error {
	if strings.TrimSpace(item.Label) == "" {
		return fmt.Errorf("completion item label cannot be empty")
	}
	if item.Kind != 0 && (item.Kind < 1 || item.Kind > 25) {
		return fmt.Errorf("invalid completion item kind: %d", item.Kind)
	}
	return nil
}

// Helper methods

func (h *ToolHandler) normalizeHoverContents(hover LSPHover) LSPHover {
	// Normalize contents to a consistent format for better client handling
	switch contents := hover.Contents.(type) {
	case string:
		// Simple string content
		hover.Contents = LSPMarkupContent{
			Kind:  "plaintext",
			Value: contents,
		}
	case map[string]interface{}:
		// Already MarkupContent or MarkedString, validate structure
		if kind, hasKind := contents["kind"].(string); hasKind {
			if value, hasValue := contents["value"].(string); hasValue {
				hover.Contents = LSPMarkupContent{
					Kind:  kind,
					Value: value,
				}
			}
		}
	case []interface{}:
		// Array of mixed content, convert to markdown
		var builder strings.Builder
		for _, item := range contents {
			switch v := item.(type) {
			case string:
				builder.WriteString(v)
				builder.WriteString("\n\n")
			case map[string]interface{}:
				if value, ok := v["value"].(string); ok {
					builder.WriteString(value)
					builder.WriteString("\n\n")
				}
			}
		}
		hover.Contents = LSPMarkupContent{
			Kind:  "markdown",
			Value: strings.TrimSpace(builder.String()),
		}
	}
	return hover
}

func (h *ToolHandler) createParsingErrorResult(method string, err error, rawData string, warnings []string) interface{} {
	return map[string]interface{}{
		"error": map[string]interface{}{
			"method":   method,
			"message":  fmt.Sprintf("LSP protocol parsing failed: %v", err),
			"code":     "PROTOCOL_VIOLATION",
			"raw_data": rawData,
			"warnings": warnings,
		},
		"fallback_data": rawData,
	}
}

// Enhanced project context methods for completion

func (h *ToolHandler) enhanceCompletionWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	// Add file context
	if uri, ok := args["uri"].(string); ok {
		result.Meta.RequestInfo["file_context"] = map[string]interface{}{
			"in_project":    workspaceCtx.IsFileInProject(uri),
			"language":      workspaceCtx.GetLanguageForFile(uri),
			"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
		}
	}

	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type":     projectCtx.ProjectType,
		"primary_language": projectCtx.PrimaryLanguage,
		"workspace_root":   projectCtx.WorkspaceRoot,
	}

	// Enhance completion items with project context
	if len(result.Content) > 0 && result.Content[0].Data != nil {
		result.Content[0] = h.enhanceCompletionContent(result.Content[0], workspaceCtx)
	}

	return result
}

func (h *ToolHandler) enhanceCompletionContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content

	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}

	enhanced.Annotations["project_enhanced"] = true
	enhanced.Annotations["completion_filtered"] = true

	// Filter and enhance completion items based on project context
	if completionList, ok := content.Data.(LSPCompletionList); ok {
		enhanced.Data = h.filterCompletionItemsByProject(completionList, workspaceCtx)
		enhanced.Text = h.formatEnhancedCompletions(enhanced.Data.(LSPCompletionList))
	}

	return enhanced
}

func (h *ToolHandler) filterCompletionItemsByProject(list LSPCompletionList, workspaceCtx *WorkspaceContext) LSPCompletionList {
	projectLang := workspaceCtx.GetProjectContext().PrimaryLanguage
	
	// Apply project-specific filtering and prioritization
	prioritizedItems := []*LSPCompletionItem{}
	regularItems := []*LSPCompletionItem{}

	for _, item := range list.Items {
		// Prioritize items based on project context
		if h.isProjectPriorityItem(item, projectLang) {
			prioritizedItems = append(prioritizedItems, item)
		} else {
			regularItems = append(regularItems, item)
		}
	}

	// Combine prioritized items first, then regular items
	allItems := append(prioritizedItems, regularItems...)

	return LSPCompletionList{
		IsIncomplete: list.IsIncomplete,
		Items:        allItems,
	}
}

func (h *ToolHandler) isProjectPriorityItem(item *LSPCompletionItem, projectLang string) bool {
	// Simple heuristics for project-relevant completion items
	label := strings.ToLower(item.Label)
	
	switch projectLang {
	case "go":
		return strings.Contains(label, "context") || strings.Contains(label, "error") || 
			   item.Kind == MethodComp || item.Kind == FunctionComp
	case "python":
		return strings.Contains(label, "self") || strings.Contains(label, "__") ||
			   item.Kind == MethodComp || item.Kind == ClassComp
	case "javascript", "typescript":
		return strings.Contains(label, "this") || strings.Contains(label, "async") ||
			   item.Kind == MethodComp || item.Kind == FunctionComp
	case "java":
		return strings.Contains(label, "public") || strings.Contains(label, "private") ||
			   item.Kind == MethodComp || item.Kind == ClassComp
	}
	
	return false
}

func (h *ToolHandler) formatEnhancedCompletions(list LSPCompletionList) string {
	if len(list.Items) == 0 {
		return "No completion items available"
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d completion item(s):\n", len(list.Items)))
	
	if list.IsIncomplete {
		result.WriteString("(Note: List is incomplete - more items available)\n")
	}

	for i, item := range list.Items {
		if i >= 10 { // Limit display to first 10 items
			result.WriteString("... and more items available\n")
			break
		}
		
		result.WriteString(fmt.Sprintf("%d. %s", i+1, item.Label))
		if item.Kind != 0 {
			result.WriteString(fmt.Sprintf(" (%s)", h.completionKindToString(item.Kind)))
		}
		if item.Detail != "" {
			result.WriteString(fmt.Sprintf(" - %s", item.Detail))
		}
		result.WriteString("\n")
	}

	return result.String()
}

func (h *ToolHandler) completionKindToString(kind LSPCompletionItemKind) string {
	switch kind {
	case Text:
		return "text"
	case MethodComp:
		return "method"
	case FunctionComp:
		return "function"
	case ConstructorComp:
		return "constructor"
	case FieldComp:
		return "field"
	case VariableComp:
		return "variable"
	case ClassComp:
		return "class"
	case InterfaceComp:
		return "interface"
	case ModuleComp:
		return "module"
	case PropertyComp:
		return "property"
	case KeywordComp:
		return "keyword"
	case SnippetComp:
		return "snippet"
	default:
		return "unknown"
	}
}

// Helper methods for project-aware processing (existing methods remain the same)

func (h *ToolHandler) enhanceDefinitionWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	// Add file context
	if uri, ok := args["uri"].(string); ok {
		result.Meta.RequestInfo["file_context"] = map[string]interface{}{
			"in_project":    workspaceCtx.IsFileInProject(uri),
			"language":      workspaceCtx.GetLanguageForFile(uri),
			"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
		}
	}

	// Add project metadata
	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type":     projectCtx.ProjectType,
		"primary_language": projectCtx.PrimaryLanguage,
		"confidence":       projectCtx.Confidence,
		"workspace_root":   projectCtx.WorkspaceRoot,
	}

	// Enhance content with project context
	if len(result.Content) > 0 && result.Content[0].Data != nil {
		result.Content[0] = h.enhanceDefinitionContent(result.Content[0], workspaceCtx)
	}

	return result
}

func (h *ToolHandler) enhanceReferencesWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	// Add file and project context
	if uri, ok := args["uri"].(string); ok {
		result.Meta.RequestInfo["file_context"] = map[string]interface{}{
			"in_project":    workspaceCtx.IsFileInProject(uri),
			"language":      workspaceCtx.GetLanguageForFile(uri),
			"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
		}
	}

	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type":     projectCtx.ProjectType,
		"primary_language": projectCtx.PrimaryLanguage,
		"workspace_root":   projectCtx.WorkspaceRoot,
	}

	// Enhance content by filtering and categorizing references
	if len(result.Content) > 0 && result.Content[0].Data != nil {
		result.Content[0] = h.enhanceReferencesContent(result.Content[0], workspaceCtx)
	}

	return result
}

func (h *ToolHandler) enhanceHoverWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	// Add file context
	if uri, ok := args["uri"].(string); ok {
		result.Meta.RequestInfo["file_context"] = map[string]interface{}{
			"in_project":    workspaceCtx.IsFileInProject(uri),
			"language":      workspaceCtx.GetLanguageForFile(uri),
			"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
		}
	}

	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type":     projectCtx.ProjectType,
		"primary_language": projectCtx.PrimaryLanguage,
	}

	// Enhance hover content with project-specific documentation
	if len(result.Content) > 0 {
		result.Content[0] = h.enhanceHoverContent(result.Content[0], workspaceCtx)
	}

	return result
}

func (h *ToolHandler) enhanceDocumentSymbolsWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	// Add file context
	if uri, ok := args["uri"].(string); ok {
		result.Meta.RequestInfo["file_context"] = map[string]interface{}{
			"in_project":    workspaceCtx.IsFileInProject(uri),
			"language":      workspaceCtx.GetLanguageForFile(uri),
			"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
		}
	}

	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type": projectCtx.ProjectType,
		"file_type":    h.categorizeFileType(args, workspaceCtx),
	}

	// Enhance symbols with project categorization
	if len(result.Content) > 0 && result.Content[0].Data != nil {
		result.Content[0] = h.enhanceDocumentSymbolsContent(result.Content[0], workspaceCtx)
	}

	return result
}

func (h *ToolHandler) enhanceWorkspaceSymbolsWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type":     projectCtx.ProjectType,
		"primary_language": projectCtx.PrimaryLanguage,
		"workspace_roots":  workspaceCtx.GetWorkspaceRoots(),
		"active_languages": workspaceCtx.GetActiveLanguages(),
	}

	if query, ok := args["query"].(string); ok {
		result.Meta.RequestInfo["search_context"] = map[string]interface{}{
			"query":                 query,
			"project_scoped":        true,
			"multi_language_search": len(projectCtx.Languages) > 1,
		}
	}

	// Filter and enhance symbols with project context
	if len(result.Content) > 0 && result.Content[0].Data != nil {
		result.Content[0] = h.enhanceWorkspaceSymbolsContent(result.Content[0], workspaceCtx)
	}

	return result
}

// Content enhancement methods

func (h *ToolHandler) enhanceDefinitionContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content

	// Add project-aware annotations
	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}

	enhanced.Annotations["project_enhanced"] = true
	enhanced.Annotations["workspace_aware"] = workspaceCtx.IsProjectAware()

	// Try to parse and enhance definition locations
	if definitions, ok := content.Data.([]LSPLocation); ok {
		enhancedDefs := h.filterAndEnhanceDefinitions(definitions, workspaceCtx)
		enhanced.Data = enhancedDefs
		enhanced.Text = h.formatEnhancedDefinitions(enhancedDefs)
	}

	return enhanced
}

func (h *ToolHandler) enhanceReferencesContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content

	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}

	enhanced.Annotations["project_enhanced"] = true

	// Parse and categorize references by project structure
	if references, ok := content.Data.([]LSPLocation); ok {
		categorized := h.categorizeReferencesByProject(references, workspaceCtx)
		enhanced.Data = categorized
		enhanced.Text = h.formatCategorizedReferences(categorized)
	}

	return enhanced
}

func (h *ToolHandler) enhanceHoverContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content

	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}

	enhanced.Annotations["project_enhanced"] = true
	enhanced.Annotations["language_context"] = workspaceCtx.GetProjectContext().PrimaryLanguage

	return enhanced
}

func (h *ToolHandler) enhanceDocumentSymbolsContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content

	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}

	enhanced.Annotations["project_enhanced"] = true

	// Categorize symbols by type and importance within project context
	switch symbols := content.Data.(type) {
	case []LSPDocumentSymbol:
		categorized := h.categorizeDocumentSymbolsByProject(symbols, workspaceCtx)
		enhanced.Data = categorized
		enhanced.Text = h.formatCategorizedDocumentSymbols(categorized)
	case []LSPSymbolInformation:
		categorized := h.categorizeSymbolInformationByProject(symbols, workspaceCtx)
		enhanced.Data = categorized
		enhanced.Text = h.formatCategorizedSymbolInformation(categorized)
	}

	return enhanced
}

func (h *ToolHandler) enhanceWorkspaceSymbolsContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content

	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}

	enhanced.Annotations["project_enhanced"] = true
	enhanced.Annotations["project_filtered"] = true

	// Filter symbols to project scope and enhance with context
	if symbols, ok := content.Data.([]LSPSymbolInformation); ok {
		filtered := h.filterSymbolsToProject(symbols, workspaceCtx)
		enhanced.Data = filtered
		enhanced.Text = h.formatProjectScopedSymbols(filtered, workspaceCtx)
	}

	return enhanced
}

// Utility methods for project-aware processing

func (h *ToolHandler) getRelativePathFromContext(uri string, workspaceCtx *WorkspaceContext) string {
	if workspaceCtx == nil {
		return ""
	}

	// Convert URI to file path
	filePath := strings.TrimPrefix(uri, "file://")
	projectRoot := workspaceCtx.GetProjectRoot()

	if relPath, err := filepath.Rel(projectRoot, filePath); err == nil {
		return relPath
	}

	return filepath.Base(filePath)
}

func (h *ToolHandler) categorizeFileType(args map[string]interface{}, workspaceCtx *WorkspaceContext) string {
	if uri, ok := args["uri"].(string); ok {
		language := workspaceCtx.GetLanguageForFile(uri)
		relPath := h.getRelativePathFromContext(uri, workspaceCtx)

		// Categorize based on path patterns
		if strings.Contains(relPath, "test") || strings.Contains(relPath, "spec") {
			return "test_file"
		} else if strings.Contains(relPath, "config") || strings.HasSuffix(relPath, ".config") {
			return "config_file"
		} else if language != "" {
			return "source_file"
		}
	}

	return "unknown_file"
}

func (h *ToolHandler) filterAndEnhanceDefinitions(definitions []LSPLocation, workspaceCtx *WorkspaceContext) []interface{} {
	enhanced := make([]interface{}, 0, len(definitions))

	for _, def := range definitions {
		// Create enhanced definition with project context
		enhancedDef := map[string]interface{}{
			"uri":   def.URI,
			"range": def.Range,
			"project_info": map[string]interface{}{
				"in_project":    workspaceCtx.IsFileInProject(def.URI),
				"relative_path": h.getRelativePathFromContext(def.URI, workspaceCtx),
				"language":      workspaceCtx.GetLanguageForFile(def.URI),
			},
		}
		enhanced = append(enhanced, enhancedDef)
	}

	return enhanced
}

func (h *ToolHandler) categorizeReferencesByProject(references []LSPLocation, workspaceCtx *WorkspaceContext) map[string]interface{} {
	projectReferences := make([]interface{}, 0)
	externalReferences := make([]interface{}, 0)

	for _, ref := range references {
		if workspaceCtx.IsFileInProject(ref.URI) {
			// Enhance project references
			enhanced := map[string]interface{}{
				"uri":   ref.URI,
				"range": ref.Range,
				"project_info": map[string]interface{}{
					"relative_path": h.getRelativePathFromContext(ref.URI, workspaceCtx),
					"language":      workspaceCtx.GetLanguageForFile(ref.URI),
				},
			}
			projectReferences = append(projectReferences, enhanced)
		} else {
			externalReferences = append(externalReferences, map[string]interface{}{
				"uri":   ref.URI,
				"range": ref.Range,
			})
		}
	}

	return map[string]interface{}{
		"project_references":  projectReferences,
		"external_references": externalReferences,
		"total_project":       len(projectReferences),
		"total_external":      len(externalReferences),
	}
}

func (h *ToolHandler) categorizeDocumentSymbolsByProject(symbols []LSPDocumentSymbol, workspaceCtx *WorkspaceContext) map[string]interface{} {
	publicSymbols := make([]LSPDocumentSymbol, 0)
	privateSymbols := make([]LSPDocumentSymbol, 0)
	testSymbols := make([]LSPDocumentSymbol, 0)

	for _, sym := range symbols {
		// Categorize based on symbol name and context
		if strings.HasPrefix(sym.Name, "test") || strings.HasPrefix(sym.Name, "Test") {
			testSymbols = append(testSymbols, sym)
		} else if strings.HasPrefix(sym.Name, "_") || strings.ToLower(sym.Name) == sym.Name {
			privateSymbols = append(privateSymbols, sym)
		} else {
			publicSymbols = append(publicSymbols, sym)
		}
	}

	return map[string]interface{}{
		"public_symbols":  publicSymbols,
		"private_symbols": privateSymbols,
		"test_symbols":    testSymbols,
		"total_public":    len(publicSymbols),
		"total_private":   len(privateSymbols),
		"total_test":      len(testSymbols),
	}
}

func (h *ToolHandler) categorizeSymbolInformationByProject(symbols []LSPSymbolInformation, workspaceCtx *WorkspaceContext) map[string]interface{} {
	publicSymbols := make([]LSPSymbolInformation, 0)
	privateSymbols := make([]LSPSymbolInformation, 0)
	testSymbols := make([]LSPSymbolInformation, 0)

	for _, sym := range symbols {
		// Categorize based on symbol name and context
		if strings.HasPrefix(sym.Name, "test") || strings.HasPrefix(sym.Name, "Test") {
			testSymbols = append(testSymbols, sym)
		} else if strings.HasPrefix(sym.Name, "_") || strings.ToLower(sym.Name) == sym.Name {
			privateSymbols = append(privateSymbols, sym)
		} else {
			publicSymbols = append(publicSymbols, sym)
		}
	}

	return map[string]interface{}{
		"public_symbols":  publicSymbols,
		"private_symbols": privateSymbols,
		"test_symbols":    testSymbols,
		"total_public":    len(publicSymbols),
		"total_private":   len(privateSymbols),
		"total_test":      len(testSymbols),
	}
}

func (h *ToolHandler) filterSymbolsToProject(symbols []LSPSymbolInformation, workspaceCtx *WorkspaceContext) []LSPSymbolInformation {
	filtered := make([]LSPSymbolInformation, 0)

	for _, sym := range symbols {
		if workspaceCtx.IsFileInProject(sym.Location.URI) {
			filtered = append(filtered, sym)
		}
	}

	return filtered
}

// Formatting methods for enhanced results

func (h *ToolHandler) formatEnhancedDefinitions(definitions []interface{}) string {
	if len(definitions) == 0 {
		return "No definitions found"
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d definition(s):\n", len(definitions)))

	for i, def := range definitions {
		if defMap, ok := def.(map[string]interface{}); ok {
			result.WriteString(fmt.Sprintf("%d. ", i+1))

			if projectInfo, exists := defMap["project_info"].(map[string]interface{}); exists {
				if relPath, exists := projectInfo["relative_path"].(string); exists {
					result.WriteString(relPath)
				}
				if lang, exists := projectInfo["language"].(string); exists && lang != "" {
					result.WriteString(fmt.Sprintf(" (%s)", lang))
				}
			}

			result.WriteString("\n")
		}
	}

	return result.String()
}

func (h *ToolHandler) formatCategorizedReferences(categorized map[string]interface{}) string {
	var result strings.Builder

	if totalProject, ok := categorized["total_project"].(int); ok {
		result.WriteString(fmt.Sprintf("Project references: %d\n", totalProject))
	}

	if totalExternal, ok := categorized["total_external"].(int); ok {
		result.WriteString(fmt.Sprintf("External references: %d\n", totalExternal))
	}

	return result.String()
}

func (h *ToolHandler) formatCategorizedDocumentSymbols(categorized map[string]interface{}) string {
	var result strings.Builder

	if totalPublic, ok := categorized["total_public"].(int); ok {
		result.WriteString(fmt.Sprintf("Public symbols: %d\n", totalPublic))
	}

	if totalPrivate, ok := categorized["total_private"].(int); ok {
		result.WriteString(fmt.Sprintf("Private symbols: %d\n", totalPrivate))
	}

	if totalTest, ok := categorized["total_test"].(int); ok {
		result.WriteString(fmt.Sprintf("Test symbols: %d\n", totalTest))
	}

	return result.String()
}

func (h *ToolHandler) formatCategorizedSymbolInformation(categorized map[string]interface{}) string {
	return h.formatCategorizedDocumentSymbols(categorized) // Same format
}

func (h *ToolHandler) formatProjectScopedSymbols(symbols []LSPSymbolInformation, workspaceCtx *WorkspaceContext) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("Found %d symbol(s) in project:\n", len(symbols)))

	for i, sym := range symbols {
		result.WriteString(fmt.Sprintf("%d. %s", i+1, sym.Name))

		relPath := h.getRelativePathFromContext(sym.Location.URI, workspaceCtx)
		if relPath != "" {
			result.WriteString(fmt.Sprintf(" - %s", relPath))
		}

		result.WriteString("\n")
	}

	return result.String()
}