package helpers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/e2e/mcp/types"
)

type ErrorSeverity int

const (
	SeverityError ErrorSeverity = iota
	SeverityWarning
	SeverityInfo
)

func (s ErrorSeverity) String() string {
	switch s {
	case SeverityError:
		return "ERROR"
	case SeverityWarning:
		return "WARNING"
	case SeverityInfo:
		return "INFO"
	default:
		return "UNKNOWN"
	}
}

func NewResponseValidationConfig() *types.ResponseValidationConfig {
	return &types.ResponseValidationConfig{
		StrictMode:              true,
		ValidatePerformance:     true,
		MaxResponseTime:         30 * time.Second,
		RequireExactFieldMatch:  false,
		ValidateURIFormat:       true,
		ValidateRangeOrdering:   true,
		ValidateSymbolHierarchy: true,
		MaxItems:                1000,
	}
}

type ValidationError struct {
	Code     string      `json:"code"`
	Message  string      `json:"message"`
	Field    string      `json:"field"`
	Expected interface{} `json:"expected"`
	Actual   interface{} `json:"actual"`
	Severity ErrorSeverity `json:"severity"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s - Field: %s, Expected: %v, Actual: %v", 
		e.Severity.String(), e.Message, e.Field, e.Expected, e.Actual)
}

type ValidationWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field"`
}

func (w *ValidationWarning) String() string {
	return fmt.Sprintf("[WARNING] %s - Field: %s", w.Message, w.Field)
}

type ResponseMetrics struct {
	ResponseSize    int64         `json:"response_size"`
	ValidationTime  time.Duration `json:"validation_time"`
	ItemCount       int           `json:"item_count"`
	ComplexityScore int           `json:"complexity_score"`
}

type ValidationResult struct {
	IsValid     bool                `json:"is_valid"`
	Errors      []ValidationError   `json:"errors"`
	Warnings    []ValidationWarning `json:"warnings"`
	Metrics     *ResponseMetrics    `json:"metrics"`
	ElapsedTime time.Duration       `json:"elapsed_time"`
}

func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		IsValid:  true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Metrics:  &ResponseMetrics{},
	}
}

func (r *ValidationResult) addError(code, message, field string, expected, actual interface{}) {
	r.IsValid = false
	r.Errors = append(r.Errors, ValidationError{
		Code:     code,
		Message:  message,
		Field:    field,
		Expected: expected,
		Actual:   actual,
		Severity: SeverityError,
	})
}

func (r *ValidationResult) addWarning(code, message, field string) {
	r.Warnings = append(r.Warnings, ValidationWarning{
		Code:    code,
		Message: message,
		Field:   field,
	})
}


type MCPResponse struct {
	JSONRPC   string          `json:"jsonrpc"`
	ID        interface{}     `json:"id"`
	Result    interface{}     `json:"result,omitempty"`
	Error     *mcp.JSONRPCError `json:"error,omitempty"`
	Timestamp time.Time       `json:"timestamp,omitempty"`
}

type DetailedAssertions struct {
	validationErrors []ValidationError
}

func NewDetailedAssertions() *DetailedAssertions {
	return &DetailedAssertions{
		validationErrors: []ValidationError{},
	}
}

type ValidationMetrics struct {
	TotalValidations int           `json:"total_validations"`
	SuccessCount     int           `json:"success_count"`
	ErrorCount       int           `json:"error_count"`
	WarningCount     int           `json:"warning_count"`
	AverageTime      time.Duration `json:"average_time"`
}

func NewValidationMetrics() *ValidationMetrics {
	return &ValidationMetrics{}
}

type ResponseValidator struct {
	config     *types.ResponseValidationConfig
	assertions *DetailedAssertions
	metrics    *ValidationMetrics
}

func NewResponseValidator(config *types.ResponseValidationConfig) *ResponseValidator {
	if config == nil {
		config = NewResponseValidationConfig()
	}
	
	return &ResponseValidator{
		config:     config,
		assertions: NewDetailedAssertions(),
		metrics:    NewValidationMetrics(),
	}
}

func (v *ResponseValidator) ValidateDefinitionResponse(response *MCPResponse, expected *types.ExpectedResult) *ValidationResult {
	start := time.Now()
	result := NewValidationResult()
	
	if err := v.validateMCPResponseStructure(response); err != nil {
		result.addError("MCP_STRUCTURE", err.Error(), "", nil, response)
		result.ElapsedTime = time.Since(start)
		return result
	}

	if err := v.validateLSPDefinitionStructure(response.Result); err != nil {
		result.addError("LSP_DEFINITION_STRUCTURE", err.Error(), "result", expected, response.Result)
		result.ElapsedTime = time.Since(start)
		return result
	}

	if expected != nil && expected.Location != nil {
		if err := v.validateDefinitionLocation(response.Result, expected.Location); err != nil {
			result.addError("DEFINITION_LOCATION", err.Error(), "location", expected.Location, response.Result)
		}
	}

	result.ElapsedTime = time.Since(start)
	return result
}

func (v *ResponseValidator) ValidateReferencesResponse(response *MCPResponse, expected *types.ExpectedResult) *ValidationResult {
	start := time.Now()
	result := NewValidationResult()
	
	if err := v.validateMCPResponseStructure(response); err != nil {
		result.addError("MCP_STRUCTURE", err.Error(), "", nil, response)
		result.ElapsedTime = time.Since(start)
		return result
	}

	references, ok := response.Result.([]interface{})
	if !ok {
		result.addError("REFERENCES_TYPE", "References result must be an array", "result", "[]Location", response.Result)
		result.ElapsedTime = time.Since(start)
		return result
	}

	validatedRefs := []types.LSPLocation{}
	for i, ref := range references {
		location, err := v.validateLocationStructure(ref)
		if err != nil {
			result.addError("REFERENCE_LOCATION", err.Error(), fmt.Sprintf("result[%d]", i), "Location", ref)
			continue
		}
		validatedRefs = append(validatedRefs, *location)
	}

	if expected != nil && expected.Locations != nil {
		if err := v.compareReferenceLocations(validatedRefs, expected.Locations); err != nil {
			result.addWarning("REFERENCE_MISMATCH", err.Error(), "locations")
		}
	}
	
	result.Metrics.ItemCount = len(validatedRefs)
	result.ElapsedTime = time.Since(start)
	return result
}

func (v *ResponseValidator) ValidateHoverResponse(response *MCPResponse, expected *types.ExpectedResult) *ValidationResult {
	start := time.Now()
	result := NewValidationResult()
	
	if err := v.validateMCPResponseStructure(response); err != nil {
		result.addError("MCP_STRUCTURE", err.Error(), "", nil, response)
		result.ElapsedTime = time.Since(start)
		return result
	}

	if response.Result == nil {
		if expected != nil && expected.Content != "" {
			result.addWarning("MISSING_HOVER", "Expected hover content but got null", "result")
		}
		result.ElapsedTime = time.Since(start)
		return result
	}

	hover, ok := response.Result.(map[string]interface{})
	if !ok {
		result.addError("HOVER_TYPE", "Hover result must be an object", "result", "Hover", response.Result)
		result.ElapsedTime = time.Since(start)
		return result
	}

	contents, exists := hover["contents"]
	if !exists {
		result.addError("MISSING_CONTENTS", "Hover must have contents field", "result.contents", "string|MarkupContent", nil)
		result.ElapsedTime = time.Since(start)
		return result
	}

	if err := v.validateHoverContents(contents); err != nil {
		result.addError("HOVER_CONTENTS", err.Error(), "result.contents", "string|MarkupContent", contents)
	}

	if hoverRange, hasRange := hover["range"]; hasRange {
		if _, err := v.validateRangeStructure(hoverRange); err != nil {
			result.addError("HOVER_RANGE", err.Error(), "result.range", "Range", hoverRange)
		}
	}

	result.ElapsedTime = time.Since(start)
	return result
}

func (v *ResponseValidator) ValidateDocumentSymbolsResponse(response *MCPResponse, expected *types.ExpectedResult) *ValidationResult {
	start := time.Now()
	result := NewValidationResult()
	
	if err := v.validateMCPResponseStructure(response); err != nil {
		result.addError("MCP_STRUCTURE", err.Error(), "", nil, response)
		result.ElapsedTime = time.Since(start)
		return result
	}

	symbols, ok := response.Result.([]interface{})
	if !ok {
		result.addError("SYMBOLS_TYPE", "Document symbols result must be an array", "result", "[]types.DocumentSymbol", response.Result)
		result.ElapsedTime = time.Since(start)
		return result
	}

	validatedSymbols := []types.DocumentSymbol{}
	for i, sym := range symbols {
		symbol, err := v.validateDocumentSymbolStructure(sym)
		if err != nil {
			result.addError("SYMBOL_STRUCTURE", err.Error(), fmt.Sprintf("result[%d]", i), "types.DocumentSymbol", sym)
			continue
		}
		validatedSymbols = append(validatedSymbols, *symbol)
	}

	if v.config.ValidateSymbolHierarchy {
		if err := v.validateSymbolHierarchy(validatedSymbols); err != nil {
			result.addWarning("SYMBOL_HIERARCHY", err.Error(), "result")
		}
	}

	result.Metrics.ItemCount = len(validatedSymbols)
	result.ElapsedTime = time.Since(start)
	return result
}

func (v *ResponseValidator) ValidateWorkspaceSymbolsResponse(response *MCPResponse, expected *types.ExpectedResult) *ValidationResult {
	start := time.Now()
	result := NewValidationResult()
	
	if err := v.validateMCPResponseStructure(response); err != nil {
		result.addError("MCP_STRUCTURE", err.Error(), "", nil, response)
		result.ElapsedTime = time.Since(start)
		return result
	}

	symbols, ok := response.Result.([]interface{})
	if !ok {
		result.addError("WORKSPACE_SYMBOLS_TYPE", "Workspace symbols result must be an array", "result", "[]SymbolInformation", response.Result)
		result.ElapsedTime = time.Since(start)
		return result
	}

	for i, sym := range symbols {
		if err := v.validateWorkspaceSymbolStructure(sym); err != nil {
			result.addError("WORKSPACE_SYMBOL", err.Error(), fmt.Sprintf("result[%d]", i), "SymbolInformation", sym)
		}
	}

	if expected != nil && expected.Query != "" {
		if err := v.validateSearchRelevance(symbols, expected.Query); err != nil {
			result.addWarning("SEARCH_RELEVANCE", err.Error(), "result")
		}
	}

	result.Metrics.ItemCount = len(symbols)
	result.ElapsedTime = time.Since(start)
	return result
}

func (v *ResponseValidator) ValidateCompletionResponse(response *MCPResponse, expected *types.ExpectedResult) *ValidationResult {
	start := time.Now()
	result := NewValidationResult()
	
	if err := v.validateMCPResponseStructure(response); err != nil {
		result.addError("MCP_STRUCTURE", err.Error(), "", nil, response)
		result.ElapsedTime = time.Since(start)
		return result
	}

	var items []interface{}
	
	if itemsArray, ok := response.Result.([]interface{}); ok {
		items = itemsArray
	} else if completionList, ok := response.Result.(map[string]interface{}); ok {
		if listItems, hasItems := completionList["items"]; hasItems {
			if itemsArray, ok := listItems.([]interface{}); ok {
				items = itemsArray
			}
		}
	}
	
	if items == nil {
		result.addError("COMPLETION_FORMAT", "Completion result must be types.CompletionItem[] or CompletionList", "result", "types.CompletionItem[]|CompletionList", response.Result)
		result.ElapsedTime = time.Since(start)
		return result
	}

	for i, item := range items {
		if err := v.validateCompletionItemStructure(item); err != nil {
			result.addError("COMPLETION_ITEM", err.Error(), fmt.Sprintf("result[%d]", i), "types.CompletionItem", item)
		}
	}

	result.Metrics.ItemCount = len(items)
	result.ElapsedTime = time.Since(start)
	return result
}

func (v *ResponseValidator) validateMCPResponseStructure(response *MCPResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}
	
	if response.JSONRPC != "2.0" {
		return fmt.Errorf("invalid JSON-RPC version: expected '2.0', got '%s'", response.JSONRPC)
	}
	
	if response.ID == "" {
		return fmt.Errorf("response ID cannot be empty")
	}
	
	if response.Result == nil && response.Error == nil {
		return fmt.Errorf("response must have either result or error")
	}
	
	if response.Result != nil && response.Error != nil {
		return fmt.Errorf("response cannot have both result and error")
	}
	
	return nil
}

func (v *ResponseValidator) validateLSPDefinitionStructure(result interface{}) error {
	if result == nil {
		return nil
	}

	if defArray, ok := result.([]interface{}); ok {
		if len(defArray) == 0 {
			return nil
		}
		
		for i, def := range defArray {
			if _, err := v.validateLocationStructure(def); err != nil {
				return fmt.Errorf("invalid definition at index %d: %w", i, err)
			}
		}
		return nil
	}

	if _, err := v.validateLocationStructure(result); err != nil {
		return fmt.Errorf("invalid definition location: %w", err)
	}
	
	return nil
}

func (v *ResponseValidator) validateLocationStructure(location interface{}) (*types.LSPLocation, error) {
	loc, ok := location.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("location must be an object")
	}
	
	uri, hasURI := loc["uri"]
	if !hasURI {
		return nil, fmt.Errorf("location must have uri field")
	}
	
	uriStr, ok := uri.(string)
	if !ok {
		return nil, fmt.Errorf("uri must be a string")
	}
	
	if v.config.ValidateURIFormat && !strings.HasPrefix(uriStr, "file://") {
		return nil, fmt.Errorf("uri must be a file:// URI")
	}
	
	rangeObj, hasRange := loc["range"]
	if !hasRange {
		return nil, fmt.Errorf("location must have range field")
	}
	
	lspRange, err := v.validateRangeStructure(rangeObj)
	if err != nil {
		return nil, fmt.Errorf("invalid range: %w", err)
	}
	
	return &types.LSPLocation{
		URI:   uriStr,
		Range: *lspRange,
	}, nil
}

func (v *ResponseValidator) validateRangeStructure(rangeObj interface{}) (*types.LSPRange, error) {
	rng, ok := rangeObj.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("range must be an object")
	}
	
	start, hasStart := rng["start"]
	if !hasStart {
		return nil, fmt.Errorf("range must have start field")
	}
	
	startPos, err := v.validatePositionStructure(start)
	if err != nil {
		return nil, fmt.Errorf("invalid start position: %w", err)
	}
	
	end, hasEnd := rng["end"]
	if !hasEnd {
		return nil, fmt.Errorf("range must have end field")
	}
	
	endPos, err := v.validatePositionStructure(end)
	if err != nil {
		return nil, fmt.Errorf("invalid end position: %w", err)
	}
	
	if v.config.ValidateRangeOrdering {
		if startPos.Line > endPos.Line || (startPos.Line == endPos.Line && startPos.Character > endPos.Character) {
			return nil, fmt.Errorf("start position must be before or equal to end position")
		}
	}
	
	return &types.LSPRange{
		Start: *startPos,
		End:   *endPos,
	}, nil
}

func (v *ResponseValidator) validatePositionStructure(position interface{}) (*types.LSPPosition, error) {
	pos, ok := position.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("position must be an object")
	}
	
	line, hasLine := pos["line"]
	if !hasLine {
		return nil, fmt.Errorf("position must have line field")
	}
	
	lineNum, ok := line.(float64)
	if !ok {
		return nil, fmt.Errorf("line must be a number")
	}
	
	if lineNum < 0 || lineNum != float64(int(lineNum)) {
		return nil, fmt.Errorf("line must be a non-negative integer")
	}
	
	character, hasChar := pos["character"]
	if !hasChar {
		return nil, fmt.Errorf("position must have character field")
	}
	
	charNum, ok := character.(float64)
	if !ok {
		return nil, fmt.Errorf("character must be a number")
	}
	
	if charNum < 0 || charNum != float64(int(charNum)) {
		return nil, fmt.Errorf("character must be a non-negative integer")
	}
	
	return &types.LSPPosition{
		Line:      int(lineNum),
		Character: int(charNum),
	}, nil
}

func (v *ResponseValidator) validateHoverContents(contents interface{}) error {
	if contents == nil {
		return fmt.Errorf("hover contents cannot be null")
	}

	if contentsStr, ok := contents.(string); ok {
		if contentsStr == "" && v.config.StrictMode {
			return fmt.Errorf("hover contents cannot be empty string")
		}
		return nil
	}

	if markupContent, ok := contents.(map[string]interface{}); ok {
		kind, hasKind := markupContent["kind"]
		if !hasKind {
			return fmt.Errorf("MarkupContent must have kind field")
		}
		
		kindStr, ok := kind.(string)
		if !ok {
			return fmt.Errorf("MarkupContent kind must be string")
		}
		
		if kindStr != "plaintext" && kindStr != "markdown" {
			return fmt.Errorf("MarkupContent kind must be 'plaintext' or 'markdown'")
		}
		
		value, hasValue := markupContent["value"]
		if !hasValue {
			return fmt.Errorf("MarkupContent must have value field")
		}
		
		if _, ok := value.(string); !ok {
			return fmt.Errorf("MarkupContent value must be string")
		}
		
		return nil
	}

	return fmt.Errorf("hover contents must be string or MarkupContent")
}

func (v *ResponseValidator) validateDocumentSymbolStructure(symbol interface{}) (*types.DocumentSymbol, error) {
	sym, ok := symbol.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("document symbol must be an object")
	}
	
	name, hasName := sym["name"]
	if !hasName {
		return nil, fmt.Errorf("document symbol must have name field")
	}
	
	nameStr, ok := name.(string)
	if !ok {
		return nil, fmt.Errorf("document symbol name must be string")
	}
	
	kind, hasKind := sym["kind"]
	if !hasKind {
		return nil, fmt.Errorf("document symbol must have kind field")
	}
	
	kindNum, ok := kind.(float64)
	if !ok {
		return nil, fmt.Errorf("document symbol kind must be number")
	}
	
	rangeObj, hasRange := sym["range"]
	if !hasRange {
		return nil, fmt.Errorf("document symbol must have range field")
	}
	
	symbolRange, err := v.validateRangeStructure(rangeObj)
	if err != nil {
		return nil, fmt.Errorf("invalid symbol range: %w", err)
	}
	
	selectionRangeObj := sym["selectionRange"]
	var selectionRange types.LSPRange
	if selectionRangeObj != nil {
		selRange, err := v.validateRangeStructure(selectionRangeObj)
		if err != nil {
			return nil, fmt.Errorf("invalid selection range: %w", err)
		}
		selectionRange = *selRange
	} else {
		selectionRange = *symbolRange
	}
	
	docSymbol := &types.DocumentSymbol{
		Name:           nameStr,
		Kind:           int(kindNum),
		Range:          *symbolRange,
		SelectionRange: selectionRange,
	}
	
	if detail, hasDetail := sym["detail"]; hasDetail {
		if detailStr, ok := detail.(string); ok {
			docSymbol.Detail = detailStr
		}
	}
	
	if children, hasChildren := sym["children"]; hasChildren {
		if childrenArray, ok := children.([]interface{}); ok {
			for _, child := range childrenArray {
				childSymbol, err := v.validateDocumentSymbolStructure(child)
				if err != nil {
					return nil, fmt.Errorf("invalid child symbol: %w", err)
				}
				docSymbol.Children = append(docSymbol.Children, *childSymbol)
			}
		}
	}
	
	return docSymbol, nil
}

func (v *ResponseValidator) validateWorkspaceSymbolStructure(symbol interface{}) error {
	sym, ok := symbol.(map[string]interface{})
	if !ok {
		return fmt.Errorf("workspace symbol must be an object")
	}
	
	requiredFields := []string{"name", "kind", "location"}
	for _, field := range requiredFields {
		if _, exists := sym[field]; !exists {
			return fmt.Errorf("workspace symbol missing required field: %s", field)
		}
	}
	
	if _, err := v.validateLocationStructure(sym["location"]); err != nil {
		return fmt.Errorf("invalid workspace symbol location: %w", err)
	}
	
	return nil
}

func (v *ResponseValidator) validateCompletionItemStructure(item interface{}) error {
	comp, ok := item.(map[string]interface{})
	if !ok {
		return fmt.Errorf("completion item must be an object")
	}
	
	label, hasLabel := comp["label"]
	if !hasLabel {
		return fmt.Errorf("completion item must have label field")
	}
	
	if _, ok := label.(string); !ok {
		return fmt.Errorf("completion item label must be string")
	}
	
	if kind, hasKind := comp["kind"]; hasKind {
		if _, ok := kind.(float64); !ok {
			return fmt.Errorf("completion item kind must be number")
		}
	}
	
	return nil
}

func (v *ResponseValidator) validateDefinitionLocation(result interface{}, expected *types.LSPLocation) error {
	if result == nil {
		return fmt.Errorf("expected definition but got null")
	}

	if defArray, ok := result.([]interface{}); ok {
		if len(defArray) == 0 {
			return fmt.Errorf("expected definition but got empty array")
		}
		
		location, err := v.validateLocationStructure(defArray[0])
		if err != nil {
			return err
		}
		
		if location.URI != expected.URI {
			return fmt.Errorf("definition URI mismatch: expected %s, got %s", expected.URI, location.URI)
		}
		
		return nil
	}

	location, err := v.validateLocationStructure(result)
	if err != nil {
		return err
	}
	
	if location.URI != expected.URI {
		return fmt.Errorf("definition URI mismatch: expected %s, got %s", expected.URI, location.URI)
	}
	
	return nil
}

func (v *ResponseValidator) compareReferenceLocations(actual, expected []types.LSPLocation) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("reference count mismatch: expected %d, got %d", len(expected), len(actual))
	}
	
	for i, expectedRef := range expected {
		if i >= len(actual) {
			return fmt.Errorf("missing reference at index %d", i)
		}
		
		actualRef := actual[i]
		if actualRef.URI != expectedRef.URI {
			return fmt.Errorf("reference URI mismatch at index %d: expected %s, got %s", i, expectedRef.URI, actualRef.URI)
		}
	}
	
	return nil
}

func (v *ResponseValidator) validateSymbolHierarchy(symbols []types.DocumentSymbol) error {
	for _, symbol := range symbols {
		for _, child := range symbol.Children {
			if !v.isRangeContained(child.Range, symbol.Range) {
				return fmt.Errorf("child symbol range is not contained within parent range")
			}
		}
	}
	return nil
}

func (v *ResponseValidator) validateSearchRelevance(symbols []interface{}, query string) error {
	if query == "" {
		return nil
	}
	
	queryLower := strings.ToLower(query)
	for i, sym := range symbols {
		symbol, ok := sym.(map[string]interface{})
		if !ok {
			continue
		}
		
		name, hasName := symbol["name"]
		if !hasName {
			continue
		}
		
		nameStr, ok := name.(string)
		if !ok {
			continue
		}
		
		nameLower := strings.ToLower(nameStr)
		if !strings.Contains(nameLower, queryLower) {
			return fmt.Errorf("symbol at index %d ('%s') does not match query '%s'", i, nameStr, query)
		}
	}
	
	return nil
}

func (v *ResponseValidator) isRangeContained(inner, outer types.LSPRange) bool {
	if inner.Start.Line < outer.Start.Line || inner.End.Line > outer.End.Line {
		return false
	}
	
	if inner.Start.Line == outer.Start.Line && inner.Start.Character < outer.Start.Character {
		return false
	}
	
	if inner.End.Line == outer.End.Line && inner.End.Character > outer.End.Character {
		return false
	}
	
	return true
}

func (v *ResponseValidator) ValidatePerformanceMetrics(response *MCPResponse, thresholds *types.ResponseValidationConfig) *ValidationResult {
	result := NewValidationResult()
	
	if v.config.ValidatePerformance {
		elapsed := time.Since(response.Timestamp)
		if elapsed > thresholds.MaxResponseTime {
			result.addError("PERFORMANCE_TIMEOUT", 
				fmt.Sprintf("Response time exceeded threshold: %v > %v", elapsed, thresholds.MaxResponseTime),
				"response_time", thresholds.MaxResponseTime, elapsed)
		}
	}
	
	return result
}

func (v *ResponseValidator) ValidateResponseFormat(response json.RawMessage, expectedFormat string) *ValidationResult {
	result := NewValidationResult()
	
	var data interface{}
	if err := json.Unmarshal(response, &data); err != nil {
		result.addError("JSON_PARSE", fmt.Sprintf("Failed to parse JSON: %v", err), "response", "valid JSON", string(response))
		return result
	}
	
	switch expectedFormat {
	case "array":
		if _, ok := data.([]interface{}); !ok {
			result.addError("FORMAT_MISMATCH", "Expected array format", "response", "array", fmt.Sprintf("%T", data))
		}
	case "object":
		if _, ok := data.(map[string]interface{}); !ok {
			result.addError("FORMAT_MISMATCH", "Expected object format", "response", "object", fmt.Sprintf("%T", data))
		}
	}
	
	return result
}