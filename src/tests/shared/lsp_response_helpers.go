package shared

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"
)

// ParseDefinitionResponse parses textDocument/definition response
// Handles: Location | Location[] | LocationLink[] | null | json.RawMessage
func ParseDefinitionResponse(t *testing.T, result interface{}) []protocol.Location {
	if result == nil {
		return []protocol.Location{}
	}

	switch res := result.(type) {
	case protocol.Location:
		return []protocol.Location{res}
	case []protocol.Location:
		return res
	case json.RawMessage:
		// Handle different JSON response formats
		jsonStr := string(res)
		if jsonStr == "null" || jsonStr == "{}" || jsonStr == "[]" {
			return []protocol.Location{}
		}

		// Try to unmarshal as array first
		var locations []protocol.Location
		if err := json.Unmarshal(res, &locations); err == nil {
			return locations
		}

		// Try as single location
		var singleLoc protocol.Location
		if err := json.Unmarshal(res, &singleLoc); err == nil {
			return []protocol.Location{singleLoc}
		}

		// Try as location links (convert to locations)
		var locationLinks []protocol.LocationLink
		if err := json.Unmarshal(res, &locationLinks); err == nil {
			locations := make([]protocol.Location, len(locationLinks))
			for i, link := range locationLinks {
				locations[i] = protocol.Location{
					URI:   link.TargetURI,
					Range: link.TargetRange,
				}
			}
			return locations
		}

		t.Logf("Warning: Failed to unmarshal definition response as any known type: %s", jsonStr)
		return []protocol.Location{}

	case []interface{}:
		// Convert from generic interface array
		var locations []protocol.Location
		for _, item := range res {
			if locationBytes, err := json.Marshal(item); err == nil {
				var loc protocol.Location
				if err := json.Unmarshal(locationBytes, &loc); err == nil {
					locations = append(locations, loc)
				}
			}
		}
		return locations

	default:
		// Try to marshal and unmarshal through JSON
		if resultBytes, err := json.Marshal(result); err == nil {
			return ParseDefinitionResponse(t, json.RawMessage(resultBytes))
		}

		t.Errorf("Unexpected definition result type: %T", result)
		return []protocol.Location{}
	}
}

// ParseReferencesResponse parses textDocument/references response
// Handles: Location[] | null | json.RawMessage
func ParseReferencesResponse(t *testing.T, result interface{}) []protocol.Location {
	if result == nil {
		return []protocol.Location{}
	}

	switch res := result.(type) {
	case []protocol.Location:
		return res
	case json.RawMessage:
		// Handle different JSON response formats
		jsonStr := string(res)
		if jsonStr == "null" || jsonStr == "[]" || jsonStr == "{}" {
			return []protocol.Location{}
		}

		var locations []protocol.Location
		if err := json.Unmarshal(res, &locations); err != nil {
			t.Logf("Warning: Failed to unmarshal references response: %v", err)
			return []protocol.Location{}
		}
		return locations

	case []interface{}:
		// Convert from generic interface array
		var locations []protocol.Location
		for _, item := range res {
			if locationBytes, err := json.Marshal(item); err == nil {
				var loc protocol.Location
				if err := json.Unmarshal(locationBytes, &loc); err == nil {
					locations = append(locations, loc)
				}
			}
		}
		return locations

	default:
		// Try to marshal and unmarshal through JSON
		if resultBytes, err := json.Marshal(result); err == nil {
			return ParseReferencesResponse(t, json.RawMessage(resultBytes))
		}

		t.Errorf("Unexpected references result type: %T", result)
		return []protocol.Location{}
	}
}

// ParseHoverResponse parses textDocument/hover response
// Handles: Hover | null | json.RawMessage
func ParseHoverResponse(t *testing.T, result interface{}) *protocol.Hover {
	if result == nil {
		return nil
	}

	switch res := result.(type) {
	case *protocol.Hover:
		return res
	case protocol.Hover:
		return &res
	case json.RawMessage:
		jsonStr := string(res)
		if jsonStr == "null" {
			return nil
		}

		var hover protocol.Hover
		if err := json.Unmarshal(res, &hover); err != nil {
			t.Logf("Warning: Failed to unmarshal hover response: %v", err)
			return nil
		}
		return &hover

	case map[string]interface{}:
		// Convert from generic map
		if hoverBytes, err := json.Marshal(res); err == nil {
			var hover protocol.Hover
			if err := json.Unmarshal(hoverBytes, &hover); err == nil {
				return &hover
			}
		}
		return nil

	default:
		// Try to marshal and unmarshal through JSON
		if resultBytes, err := json.Marshal(result); err == nil {
			return ParseHoverResponse(t, json.RawMessage(resultBytes))
		}

		t.Errorf("Unexpected hover result type: %T", result)
		return nil
	}
}

// ParseWorkspaceSymbolResponse parses workspace/symbol response
// Handles: SymbolInformation[] | WorkspaceSymbol[] | null | json.RawMessage
func ParseWorkspaceSymbolResponse(t *testing.T, result interface{}) []protocol.SymbolInformation {
	if result == nil {
		return []protocol.SymbolInformation{}
	}

	switch res := result.(type) {
	case []protocol.SymbolInformation:
		return res
	case json.RawMessage:
		jsonStr := string(res)
		if jsonStr == "null" || jsonStr == "[]" {
			return []protocol.SymbolInformation{}
		}

		var symbols []protocol.SymbolInformation
		if err := json.Unmarshal(res, &symbols); err != nil {
			// Try as generic array with similar structure
			var genericSymbols []map[string]interface{}
			if err2 := json.Unmarshal(res, &genericSymbols); err2 == nil {
				symbols = make([]protocol.SymbolInformation, 0, len(genericSymbols))
				for _, gs := range genericSymbols {
					if name, ok := gs["name"].(string); ok {
						sym := protocol.SymbolInformation{
							Name: name,
						}
						if kindFloat, ok := gs["kind"].(float64); ok {
							sym.Kind = protocol.SymbolKind(kindFloat)
						}
						symbols = append(symbols, sym)
					}
				}
				return symbols
			}

			t.Logf("Warning: Failed to unmarshal workspace symbol response: %v", err)
			return []protocol.SymbolInformation{}
		}
		return symbols

	case []interface{}:
		// Convert from generic interface array
		var symbols []protocol.SymbolInformation
		for _, item := range res {
			if symbolBytes, err := json.Marshal(item); err == nil {
				var symbol protocol.SymbolInformation
				if err := json.Unmarshal(symbolBytes, &symbol); err == nil {
					symbols = append(symbols, symbol)
				}
			}
		}
		return symbols

	default:
		// Try to marshal and unmarshal through JSON
		if resultBytes, err := json.Marshal(result); err == nil {
			return ParseWorkspaceSymbolResponse(t, json.RawMessage(resultBytes))
		}

		t.Errorf("Unexpected workspace symbol result type: %T", result)
		return []protocol.SymbolInformation{}
	}
}

// ParseDocumentSymbolResponse parses textDocument/documentSymbol response
// Handles: DocumentSymbol[] | SymbolInformation[] | null | json.RawMessage
func ParseDocumentSymbolResponse(t *testing.T, result interface{}) []protocol.DocumentSymbol {
	if result == nil {
		return []protocol.DocumentSymbol{}
	}

	switch res := result.(type) {
	case []protocol.DocumentSymbol:
		return res
	case json.RawMessage:
		jsonStr := string(res)
		if jsonStr == "null" || jsonStr == "[]" {
			return []protocol.DocumentSymbol{}
		}

		// Try as DocumentSymbol array first
		var docSymbols []protocol.DocumentSymbol
		if err := json.Unmarshal(res, &docSymbols); err == nil {
			return docSymbols
		}

		// Try as SymbolInformation array (convert to DocumentSymbol)
		var symbolInfos []protocol.SymbolInformation
		if err := json.Unmarshal(res, &symbolInfos); err == nil {
			docSymbols := make([]protocol.DocumentSymbol, len(symbolInfos))
			for i, si := range symbolInfos {
				docSymbols[i] = protocol.DocumentSymbol{
					Name:           si.Name,
					Kind:           si.Kind,
					Range:          si.Location.Range,
					SelectionRange: si.Location.Range,
				}
			}
			return docSymbols
		}

		t.Logf("Warning: Failed to unmarshal document symbol response")
		return []protocol.DocumentSymbol{}

	case []interface{}:
		// Convert from generic interface array
		var symbols []protocol.DocumentSymbol
		for _, item := range res {
			if symbolBytes, err := json.Marshal(item); err == nil {
				var symbol protocol.DocumentSymbol
				if err := json.Unmarshal(symbolBytes, &symbol); err == nil {
					symbols = append(symbols, symbol)
				}
			}
		}
		return symbols

	default:
		// Try to marshal and unmarshal through JSON
		if resultBytes, err := json.Marshal(result); err == nil {
			return ParseDocumentSymbolResponse(t, json.RawMessage(resultBytes))
		}

		t.Errorf("Unexpected document symbol result type: %T", result)
		return []protocol.DocumentSymbol{}
	}
}

// ParseCompletionResponse parses textDocument/completion response
// Handles: CompletionItem[] | CompletionList | null | json.RawMessage
func ParseCompletionResponse(t *testing.T, result interface{}) *protocol.CompletionList {
	if result == nil {
		return &protocol.CompletionList{
			IsIncomplete: false,
			Items:        []protocol.CompletionItem{},
		}
	}

	switch res := result.(type) {
	case *protocol.CompletionList:
		return res
	case protocol.CompletionList:
		return &res
	case []protocol.CompletionItem:
		return &protocol.CompletionList{
			IsIncomplete: false,
			Items:        res,
		}
	case json.RawMessage:
		jsonStr := string(res)
		if jsonStr == "null" || jsonStr == "[]" {
			return &protocol.CompletionList{
				IsIncomplete: false,
				Items:        []protocol.CompletionItem{},
			}
		}

		// Try as CompletionList first
		var completionList protocol.CompletionList
		if err := json.Unmarshal(res, &completionList); err == nil {
			return &completionList
		}

		// Try as CompletionItem array
		var items []protocol.CompletionItem
		if err := json.Unmarshal(res, &items); err == nil {
			return &protocol.CompletionList{
				IsIncomplete: false,
				Items:        items,
			}
		}

		t.Logf("Warning: Failed to unmarshal completion response")
		return &protocol.CompletionList{
			IsIncomplete: false,
			Items:        []protocol.CompletionItem{},
		}

	case []interface{}:
		// Convert from generic interface array (CompletionItem[])
		var items []protocol.CompletionItem
		for _, item := range res {
			if itemBytes, err := json.Marshal(item); err == nil {
				var completionItem protocol.CompletionItem
				if err := json.Unmarshal(itemBytes, &completionItem); err == nil {
					items = append(items, completionItem)
				}
			}
		}
		return &protocol.CompletionList{
			IsIncomplete: false,
			Items:        items,
		}

	case map[string]interface{}:
		// Convert from generic map (CompletionList)
		if listBytes, err := json.Marshal(res); err == nil {
			var completionList protocol.CompletionList
			if err := json.Unmarshal(listBytes, &completionList); err == nil {
				return &completionList
			}
		}
		return &protocol.CompletionList{
			IsIncomplete: false,
			Items:        []protocol.CompletionItem{},
		}

	default:
		// Try to marshal and unmarshal through JSON
		if resultBytes, err := json.Marshal(result); err == nil {
			return ParseCompletionResponse(t, json.RawMessage(resultBytes))
		}

		t.Errorf("Unexpected completion result type: %T", result)
		return &protocol.CompletionList{
			IsIncomplete: false,
			Items:        []protocol.CompletionItem{},
		}
	}
}

// Helper functions for testing parsed responses

// RequireValidLocations validates that locations are properly formed
func RequireValidLocations(t *testing.T, locations []protocol.Location, minCount int, context string) {
	require.GreaterOrEqual(t, len(locations), minCount,
		"%s should return at least %d location(s), got %d", context, minCount, len(locations))

	for i, loc := range locations {
		require.NotEmpty(t, loc.URI, "%s location %d should have valid URI", context, i)
		require.NotNil(t, loc.Range, "%s location %d should have valid range", context, i)
	}
}

// RequireValidHover validates that hover response is properly formed
func RequireValidHover(t *testing.T, hover *protocol.Hover, context string) {
	require.NotNil(t, hover, "%s should return valid hover information", context)
	require.NotNil(t, hover.Contents, "%s hover should have contents", context)
}

// RequireValidSymbols validates that symbols are properly formed
func RequireValidSymbols(t *testing.T, symbols []protocol.SymbolInformation, minCount int, context string) {
	require.GreaterOrEqual(t, len(symbols), minCount,
		"%s should return at least %d symbol(s), got %d", context, minCount, len(symbols))

	for i, symbol := range symbols {
		require.NotEmpty(t, symbol.Name, "%s symbol %d should have valid name", context, i)
		require.NotZero(t, symbol.Kind, "%s symbol %d should have valid kind", context, i)
	}
}

// RequireValidDocumentSymbols validates that document symbols are properly formed
func RequireValidDocumentSymbols(t *testing.T, symbols []protocol.DocumentSymbol, minCount int, context string) {
	require.GreaterOrEqual(t, len(symbols), minCount,
		"%s should return at least %d symbol(s), got %d", context, minCount, len(symbols))

	for i, symbol := range symbols {
		require.NotEmpty(t, symbol.Name, "%s symbol %d should have valid name", context, i)
		require.NotZero(t, symbol.Kind, "%s symbol %d should have valid kind", context, i)
		require.NotNil(t, symbol.Range, "%s symbol %d should have valid range", context, i)
		require.NotNil(t, symbol.SelectionRange, "%s symbol %d should have valid selection range", context, i)
	}
}

// RequireValidCompletion validates that completion response is properly formed
func RequireValidCompletion(t *testing.T, completion *protocol.CompletionList, minCount int, context string) {
	require.NotNil(t, completion, "%s should return valid completion list", context)
	require.GreaterOrEqual(t, len(completion.Items), minCount,
		"%s should return at least %d completion item(s), got %d", context, minCount, len(completion.Items))

	for i, item := range completion.Items {
		require.NotEmpty(t, item.Label, "%s completion item %d should have valid label", context, i)
	}
}

// LogResponseSummary logs a summary of the parsed response for debugging
func LogResponseSummary(t *testing.T, methodName string, result interface{}) {
	switch methodName {
	case "textDocument/definition":
		if locations := ParseDefinitionResponse(t, result); len(locations) > 0 {
			t.Logf("%s: Found %d definition location(s)", methodName, len(locations))
		} else {
			t.Logf("%s: No definitions found", methodName)
		}
	case "textDocument/references":
		if locations := ParseReferencesResponse(t, result); len(locations) > 0 {
			t.Logf("%s: Found %d reference location(s)", methodName, len(locations))
		} else {
			t.Logf("%s: No references found", methodName)
		}
	case "textDocument/hover":
		if hover := ParseHoverResponse(t, result); hover != nil {
			t.Logf("%s: Found hover information", methodName)
		} else {
			t.Logf("%s: No hover information", methodName)
		}
	case "workspace/symbol":
		if symbols := ParseWorkspaceSymbolResponse(t, result); len(symbols) > 0 {
			t.Logf("%s: Found %d workspace symbol(s)", methodName, len(symbols))
		} else {
			t.Logf("%s: No workspace symbols found", methodName)
		}
	case "textDocument/documentSymbol":
		if symbols := ParseDocumentSymbolResponse(t, result); len(symbols) > 0 {
			t.Logf("%s: Found %d document symbol(s)", methodName, len(symbols))
		} else {
			t.Logf("%s: No document symbols found", methodName)
		}
	case "textDocument/completion":
		if completion := ParseCompletionResponse(t, result); len(completion.Items) > 0 {
			t.Logf("%s: Found %d completion item(s)", methodName, len(completion.Items))
		} else {
			t.Logf("%s: No completion items found", methodName)
		}
	default:
		t.Logf("%s: Response received (type: %T)", methodName, result)
	}
}
