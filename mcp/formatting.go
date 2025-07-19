package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// ResponseFormatter formats LSP responses for optimal AI consumption
type ResponseFormatter struct{}

// FormattedLSPResponse represents a structured LSP response optimized for AI consumption
type FormattedLSPResponse struct {
	Method      string                 `json:"method"`
	Success     bool                   `json:"success"`
	Results     interface{}            `json:"results,omitempty"`
	Summary     string                 `json:"summary"`
	Context     map[string]interface{} `json:"context,omitempty"`
	RawResponse json.RawMessage        `json:"rawResponse,omitempty"`
}

// LSPLocation represents a location in the LSP protocol
type LSPLocation struct {
	URI   string   `json:"uri"`
	Range LSPRange `json:"range"`
}

// LSPRange represents a range in the LSP protocol
type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

// LSPPosition represents a position in the LSP protocol
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// LSPSymbolInformation represents symbol information in the LSP protocol
type LSPSymbolInformation struct {
	Name          string      `json:"name"`
	Kind          int         `json:"kind"`
	Location      LSPLocation `json:"location"`
	ContainerName string      `json:"containerName,omitempty"`
	Deprecated    bool        `json:"deprecated,omitempty"`
}

// LSPDocumentSymbol represents a document symbol in the LSP protocol
type LSPDocumentSymbol struct {
	Name           string              `json:"name"`
	Detail         string              `json:"detail,omitempty"`
	Kind           int                 `json:"kind"`
	Deprecated     bool                `json:"deprecated,omitempty"`
	Range          LSPRange            `json:"range"`
	SelectionRange LSPRange            `json:"selectionRange"`
	Children       []LSPDocumentSymbol `json:"children,omitempty"`
}

// LSPHover represents hover information in the LSP protocol
type LSPHover struct {
	Contents interface{} `json:"contents"`
	Range    *LSPRange   `json:"range,omitempty"`
}

// NewResponseFormatter creates a new response formatter
func NewResponseFormatter() *ResponseFormatter {
	return &ResponseFormatter{}
}

// FormatDefinitionResponse formats textDocument/definition response
func (f *ResponseFormatter) FormatDefinitionResponse(rawResponse json.RawMessage, requestArgs map[string]interface{}) *FormattedLSPResponse {
	var locations []LSPLocation
	var summary string

	// Try to parse as array of locations (most common)
	if err := json.Unmarshal(rawResponse, &locations); err != nil {
		// Try to parse as single location
		var singleLocation LSPLocation
		if err := json.Unmarshal(rawResponse, &singleLocation); err == nil {
			locations = []LSPLocation{singleLocation}
		}
	}

	// Build summary
	if len(locations) == 0 {
		summary = "No definition found for the symbol at the specified position"
	} else if len(locations) == 1 {
		fileName := f.extractFileName(locations[0].URI)
		line := locations[0].Range.Start.Line + 1 // Convert to 1-based for display
		summary = fmt.Sprintf("Definition found in %s at line %d", fileName, line)
	} else {
		summary = fmt.Sprintf("Found %d possible definitions", len(locations))
	}

	// Structure the results for AI consumption
	structuredResults := map[string]interface{}{
		"definitionCount": len(locations),
		"definitions":     f.formatLocationList(locations),
	}

	if len(locations) > 0 {
		structuredResults["primaryDefinition"] = f.formatLocationDetails(locations[0])
	}

	return &FormattedLSPResponse{
		Method:      "textDocument/definition",
		Success:     true,
		Results:     structuredResults,
		Summary:     summary,
		Context:     f.buildDefinitionContext(requestArgs, locations),
		RawResponse: rawResponse,
	}
}

// FormatReferencesResponse formats textDocument/references response
func (f *ResponseFormatter) FormatReferencesResponse(rawResponse json.RawMessage, requestArgs map[string]interface{}) *FormattedLSPResponse {
	var references []LSPLocation
	var summary string

	// Parse as array of locations
	if err := json.Unmarshal(rawResponse, &references); err != nil {
		references = []LSPLocation{}
	}

	// Build summary
	if len(references) == 0 {
		summary = "No references found for the symbol at the specified position"
	} else {
		fileCount := f.countUniqueFiles(references)
		if fileCount == 1 {
			summary = fmt.Sprintf("Found %d references in 1 file", len(references))
		} else {
			summary = fmt.Sprintf("Found %d references across %d files", len(references), fileCount)
		}
	}

	// Group references by file for better organization
	referencesByFile := f.groupReferencesByFile(references)

	// Structure the results for AI consumption
	structuredResults := map[string]interface{}{
		"referenceCount":   len(references),
		"fileCount":        f.countUniqueFiles(references),
		"references":       f.formatLocationList(references),
		"referencesByFile": referencesByFile,
	}

	return &FormattedLSPResponse{
		Method:      "textDocument/references",
		Success:     true,
		Results:     structuredResults,
		Summary:     summary,
		Context:     f.buildReferencesContext(requestArgs, references),
		RawResponse: rawResponse,
	}
}

// FormatHoverResponse formats textDocument/hover response
func (f *ResponseFormatter) FormatHoverResponse(rawResponse json.RawMessage, requestArgs map[string]interface{}) *FormattedLSPResponse {
	var hover LSPHover
	var summary string

	// Parse hover response
	if err := json.Unmarshal(rawResponse, &hover); err != nil {
		hover = LSPHover{}
	}

	// Extract hover contents
	hoverText := f.extractHoverText(hover.Contents)

	// Build summary
	if hoverText == "" {
		summary = "No hover information available for the symbol at the specified position"
	} else {
		// Create a brief summary of the hover content
		lines := strings.Split(hoverText, "\n")
		if len(lines) > 0 {
			firstLine := strings.TrimSpace(lines[0])
			if len(firstLine) > 100 {
				firstLine = firstLine[:97] + "..."
			}
			summary = fmt.Sprintf("Hover info: %s", firstLine)
		} else {
			summary = "Hover information available"
		}
	}

	// Structure the results for AI consumption
	structuredResults := map[string]interface{}{
		"hasHoverInfo": hoverText != "",
		"content":      hoverText,
		"contentType":  f.detectContentType(hoverText),
	}

	if hover.Range != nil {
		structuredResults["range"] = hover.Range
	}

	return &FormattedLSPResponse{
		Method:      "textDocument/hover",
		Success:     true,
		Results:     structuredResults,
		Summary:     summary,
		Context:     f.buildHoverContext(requestArgs, hover),
		RawResponse: rawResponse,
	}
}

// FormatDocumentSymbolsResponse formats textDocument/documentSymbol response
func (f *ResponseFormatter) FormatDocumentSymbolsResponse(rawResponse json.RawMessage, requestArgs map[string]interface{}) *FormattedLSPResponse {
	var symbols []LSPDocumentSymbol
	var symbolInfo []LSPSymbolInformation
	var summary string

	// Try parsing as DocumentSymbol array first (hierarchical)
	if err := json.Unmarshal(rawResponse, &symbols); err != nil {
		// Try parsing as SymbolInformation array (flat)
		_ = json.Unmarshal(rawResponse, &symbolInfo)
	}

	totalSymbols := len(symbols) + len(symbolInfo)

	// Build summary
	if totalSymbols == 0 {
		summary = "No symbols found in the document"
	} else {
		fileName := f.extractFileName(requestArgs["uri"].(string))
		summary = fmt.Sprintf("Found %d symbols in %s", totalSymbols, fileName)
	}

	// Structure the results for AI consumption
	structuredResults := map[string]interface{}{
		"symbolCount":  totalSymbols,
		"hasHierarchy": len(symbols) > 0,
	}

	if len(symbols) > 0 {
		structuredResults["symbols"] = f.formatDocumentSymbolHierarchy(symbols)
		structuredResults["symbolsByKind"] = f.groupSymbolsByKind(symbols)
	} else if len(symbolInfo) > 0 {
		structuredResults["symbols"] = f.formatSymbolInformationList(symbolInfo)
		structuredResults["symbolsByKind"] = f.groupSymbolInfoByKind(symbolInfo)
	}

	return &FormattedLSPResponse{
		Method:      "textDocument/documentSymbol",
		Success:     true,
		Results:     structuredResults,
		Summary:     summary,
		Context:     f.buildDocumentSymbolsContext(requestArgs, totalSymbols),
		RawResponse: rawResponse,
	}
}

// FormatWorkspaceSymbolsResponse formats workspace/symbol response
func (f *ResponseFormatter) FormatWorkspaceSymbolsResponse(rawResponse json.RawMessage, requestArgs map[string]interface{}) *FormattedLSPResponse {
	var symbols []LSPSymbolInformation
	var summary string

	// Parse as SymbolInformation array
	if err := json.Unmarshal(rawResponse, &symbols); err != nil {
		symbols = []LSPSymbolInformation{}
	}

	// Build summary
	query := requestArgs["query"].(string)
	if len(symbols) == 0 {
		summary = fmt.Sprintf("No symbols found matching query: %s", query)
	} else {
		fileCount := f.countUniqueFilesFromSymbols(symbols)
		if fileCount == 1 {
			summary = fmt.Sprintf("Found %d symbols matching '%s' in 1 file", len(symbols), query)
		} else {
			summary = fmt.Sprintf("Found %d symbols matching '%s' across %d files", len(symbols), query, fileCount)
		}
	}

	// Group symbols by file and kind
	symbolsByFile := f.groupSymbolsByFile(symbols)
	symbolsByKind := f.groupSymbolInfoByKind(symbols)

	// Structure the results for AI consumption
	structuredResults := map[string]interface{}{
		"symbolCount":   len(symbols),
		"fileCount":     f.countUniqueFilesFromSymbols(symbols),
		"query":         query,
		"symbols":       f.formatSymbolInformationList(symbols),
		"symbolsByFile": symbolsByFile,
		"symbolsByKind": symbolsByKind,
	}

	return &FormattedLSPResponse{
		Method:      "workspace/symbol",
		Success:     true,
		Results:     structuredResults,
		Summary:     summary,
		Context:     f.buildWorkspaceSymbolsContext(requestArgs, symbols),
		RawResponse: rawResponse,
	}
}

// Helper functions for formatting

func (f *ResponseFormatter) extractFileName(uri string) string {
	if uri == "" {
		return "unknown"
	}
	// Extract file name from URI
	if strings.HasPrefix(uri, "file://") {
		path := uri[7:] // Remove "file://" prefix
		return filepath.Base(path)
	}
	return filepath.Base(uri)
}

func (f *ResponseFormatter) formatLocationList(locations []LSPLocation) []map[string]interface{} {
	result := make([]map[string]interface{}, len(locations))
	for i, loc := range locations {
		result[i] = f.formatLocationDetails(loc)
	}
	return result
}

func (f *ResponseFormatter) formatLocationDetails(location LSPLocation) map[string]interface{} {
	return map[string]interface{}{
		"uri":            location.URI,
		"fileName":       f.extractFileName(location.URI),
		"range":          location.Range,
		"line":           location.Range.Start.Line + 1, // 1-based for display
		"startCharacter": location.Range.Start.Character,
		"endCharacter":   location.Range.End.Character,
	}
}

func (f *ResponseFormatter) countUniqueFiles(locations []LSPLocation) int {
	files := make(map[string]bool)
	for _, loc := range locations {
		files[loc.URI] = true
	}
	return len(files)
}

func (f *ResponseFormatter) groupReferencesByFile(references []LSPLocation) map[string]interface{} {
	fileGroups := make(map[string][]map[string]interface{})

	for _, ref := range references {
		fileName := f.extractFileName(ref.URI)
		if fileGroups[fileName] == nil {
			fileGroups[fileName] = []map[string]interface{}{}
		}
		fileGroups[fileName] = append(fileGroups[fileName], f.formatLocationDetails(ref))
	}

	// Convert to interface{} for JSON serialization
	result := make(map[string]interface{})
	for fileName, refs := range fileGroups {
		result[fileName] = map[string]interface{}{
			"uri":            refs[0]["uri"],
			"referenceCount": len(refs),
			"references":     refs,
		}
	}

	return result
}

func (f *ResponseFormatter) extractHoverText(contents interface{}) string {
	switch c := contents.(type) {
	case string:
		return c
	case []interface{}:
		var parts []string
		for _, part := range c {
			if str, ok := part.(string); ok {
				parts = append(parts, str)
			} else if obj, ok := part.(map[string]interface{}); ok {
				if value, exists := obj["value"]; exists {
					if str, ok := value.(string); ok {
						parts = append(parts, str)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	case map[string]interface{}:
		if value, exists := c["value"]; exists {
			if str, ok := value.(string); ok {
				return str
			}
		}
	}
	return ""
}

func (f *ResponseFormatter) detectContentType(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		return "markdown"
	}
	if strings.Contains(content, "function") || strings.Contains(content, "class") || strings.Contains(content, "var ") {
		return "code"
	}
	return "text"
}

func (f *ResponseFormatter) formatDocumentSymbolHierarchy(symbols []LSPDocumentSymbol) []map[string]interface{} {
	result := make([]map[string]interface{}, len(symbols))
	for i, symbol := range symbols {
		result[i] = map[string]interface{}{
			"name":           symbol.Name,
			"detail":         symbol.Detail,
			"kind":           symbol.Kind,
			"kindName":       f.getSymbolKindName(symbol.Kind),
			"range":          symbol.Range,
			"selectionRange": symbol.SelectionRange,
			"deprecated":     symbol.Deprecated,
			"childCount":     len(symbol.Children),
		}
		if len(symbol.Children) > 0 {
			result[i]["children"] = f.formatDocumentSymbolHierarchy(symbol.Children)
		}
	}
	return result
}

func (f *ResponseFormatter) formatSymbolInformationList(symbols []LSPSymbolInformation) []map[string]interface{} {
	result := make([]map[string]interface{}, len(symbols))
	for i, symbol := range symbols {
		result[i] = map[string]interface{}{
			"name":          symbol.Name,
			"kind":          symbol.Kind,
			"kindName":      f.getSymbolKindName(symbol.Kind),
			"location":      f.formatLocationDetails(symbol.Location),
			"containerName": symbol.ContainerName,
			"deprecated":    symbol.Deprecated,
		}
	}
	return result
}

func (f *ResponseFormatter) groupSymbolsByKind(symbols []LSPDocumentSymbol) map[string]interface{} {
	groups := make(map[string][]map[string]interface{})

	var processSymbols func([]LSPDocumentSymbol)
	processSymbols = func(syms []LSPDocumentSymbol) {
		for _, symbol := range syms {
			kindName := f.getSymbolKindName(symbol.Kind)
			if groups[kindName] == nil {
				groups[kindName] = []map[string]interface{}{}
			}
			groups[kindName] = append(groups[kindName], map[string]interface{}{
				"name":   symbol.Name,
				"detail": symbol.Detail,
				"range":  symbol.Range,
			})

			// Process children recursively
			if len(symbol.Children) > 0 {
				processSymbols(symbol.Children)
			}
		}
	}

	processSymbols(symbols)

	// Convert to interface{} and add counts
	result := make(map[string]interface{})
	for kindName, symbolList := range groups {
		result[kindName] = map[string]interface{}{
			"count":   len(symbolList),
			"symbols": symbolList,
		}
	}

	return result
}

func (f *ResponseFormatter) groupSymbolInfoByKind(symbols []LSPSymbolInformation) map[string]interface{} {
	groups := make(map[string][]map[string]interface{})

	for _, symbol := range symbols {
		kindName := f.getSymbolKindName(symbol.Kind)
		if groups[kindName] == nil {
			groups[kindName] = []map[string]interface{}{}
		}
		groups[kindName] = append(groups[kindName], map[string]interface{}{
			"name":          symbol.Name,
			"location":      f.formatLocationDetails(symbol.Location),
			"containerName": symbol.ContainerName,
		})
	}

	// Convert to interface{} and add counts
	result := make(map[string]interface{})
	for kindName, symbolList := range groups {
		result[kindName] = map[string]interface{}{
			"count":   len(symbolList),
			"symbols": symbolList,
		}
	}

	return result
}

func (f *ResponseFormatter) countUniqueFilesFromSymbols(symbols []LSPSymbolInformation) int {
	files := make(map[string]bool)
	for _, symbol := range symbols {
		files[symbol.Location.URI] = true
	}
	return len(files)
}

func (f *ResponseFormatter) groupSymbolsByFile(symbols []LSPSymbolInformation) map[string]interface{} {
	fileGroups := make(map[string][]map[string]interface{})

	for _, symbol := range symbols {
		fileName := f.extractFileName(symbol.Location.URI)
		if fileGroups[fileName] == nil {
			fileGroups[fileName] = []map[string]interface{}{}
		}
		fileGroups[fileName] = append(fileGroups[fileName], map[string]interface{}{
			"name":          symbol.Name,
			"kind":          symbol.Kind,
			"kindName":      f.getSymbolKindName(symbol.Kind),
			"location":      f.formatLocationDetails(symbol.Location),
			"containerName": symbol.ContainerName,
		})
	}

	// Convert to interface{} and add metadata
	result := make(map[string]interface{})
	for fileName, symbolList := range fileGroups {
		result[fileName] = map[string]interface{}{
			"uri":         symbolList[0]["location"].(map[string]interface{})["uri"],
			"symbolCount": len(symbolList),
			"symbols":     symbolList,
		}
	}

	return result
}

func (f *ResponseFormatter) getSymbolKindName(kind int) string {
	kindNames := map[int]string{
		1:  "File",
		2:  "Module",
		3:  "Namespace",
		4:  "Package",
		5:  "Class",
		6:  "Method",
		7:  "Property",
		8:  "Field",
		9:  "Constructor",
		10: "Enum",
		11: "Interface",
		12: "Function",
		13: "Variable",
		14: "Constant",
		15: "String",
		16: "Number",
		17: "Boolean",
		18: "Array",
		19: "Object",
		20: "Key",
		21: "Null",
		22: "EnumMember",
		23: "Struct",
		24: "Event",
		25: "Operator",
		26: "TypeParameter",
	}

	if name, exists := kindNames[kind]; exists {
		return name
	}
	return fmt.Sprintf("Unknown(%d)", kind)
}

// Context building functions

func (f *ResponseFormatter) buildDefinitionContext(requestArgs map[string]interface{}, locations []LSPLocation) map[string]interface{} {
	context := map[string]interface{}{
		"requestedPosition": map[string]interface{}{
			"uri":       requestArgs["uri"],
			"line":      requestArgs["line"],
			"character": requestArgs["character"],
		},
		"hasDefinitions": len(locations) > 0,
	}

	if len(locations) > 0 {
		context["definitionFiles"] = f.getUniqueFileNames(locations)
	}

	return context
}

func (f *ResponseFormatter) buildReferencesContext(requestArgs map[string]interface{}, references []LSPLocation) map[string]interface{} {
	context := map[string]interface{}{
		"requestedPosition": map[string]interface{}{
			"uri":       requestArgs["uri"],
			"line":      requestArgs["line"],
			"character": requestArgs["character"],
		},
		"includeDeclaration": requestArgs["includeDeclaration"],
		"hasReferences":      len(references) > 0,
	}

	if len(references) > 0 {
		context["referenceFiles"] = f.getUniqueFileNames(references)
	}

	return context
}

func (f *ResponseFormatter) buildHoverContext(requestArgs map[string]interface{}, hover LSPHover) map[string]interface{} {
	return map[string]interface{}{
		"requestedPosition": map[string]interface{}{
			"uri":       requestArgs["uri"],
			"line":      requestArgs["line"],
			"character": requestArgs["character"],
		},
		"hasRange": hover.Range != nil,
	}
}

func (f *ResponseFormatter) buildDocumentSymbolsContext(requestArgs map[string]interface{}, symbolCount int) map[string]interface{} {
	return map[string]interface{}{
		"documentUri": requestArgs["uri"],
		"fileName":    f.extractFileName(requestArgs["uri"].(string)),
		"hasSymbols":  symbolCount > 0,
	}
}

func (f *ResponseFormatter) buildWorkspaceSymbolsContext(requestArgs map[string]interface{}, symbols []LSPSymbolInformation) map[string]interface{} {
	context := map[string]interface{}{
		"query":      requestArgs["query"],
		"hasResults": len(symbols) > 0,
	}

	if len(symbols) > 0 {
		context["resultFiles"] = f.getUniqueFileNamesFromSymbols(symbols)
	}

	return context
}

func (f *ResponseFormatter) getUniqueFileNames(locations []LSPLocation) []string {
	fileNames := make(map[string]bool)
	for _, loc := range locations {
		fileName := f.extractFileName(loc.URI)
		fileNames[fileName] = true
	}

	result := make([]string, 0, len(fileNames))
	for fileName := range fileNames {
		result = append(result, fileName)
	}
	return result
}

func (f *ResponseFormatter) getUniqueFileNamesFromSymbols(symbols []LSPSymbolInformation) []string {
	fileNames := make(map[string]bool)
	for _, symbol := range symbols {
		fileName := f.extractFileName(symbol.Location.URI)
		fileNames[fileName] = true
	}

	result := make([]string, 0, len(fileNames))
	for fileName := range fileNames {
		result = append(result, fileName)
	}
	return result
}
