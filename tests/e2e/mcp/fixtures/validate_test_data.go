package fixtures

import (
	"fmt"
	"testing"
)

// ValidateTestDataIntegrity performs comprehensive validation of the test fixture data
func ValidateTestDataIntegrity() error {
	testData := GetFatihColorTestData()
	
	// Run basic validation
	errors := testData.ValidateTestData()
	if len(errors) > 0 {
		return fmt.Errorf("validation errors found: %v", errors)
	}
	
	// Test JSON serialization
	jsonData, err := testData.ToJSON()
	if err != nil {
		return fmt.Errorf("JSON serialization failed: %v", err)
	}
	
	if len(jsonData) == 0 {
		return fmt.Errorf("JSON data is empty")
	}
	
	// Validate core data structure integrity
	if err := validateDataStructure(testData); err != nil {
		return fmt.Errorf("data structure validation failed: %v", err)
	}
	
	fmt.Printf("✅ Test fixture validation passed successfully\n")
	fmt.Printf("📊 Repository: %s (%s)\n", testData.RepositoryInfo.Name, testData.RepositoryInfo.Version)
	fmt.Printf("📁 Files covered: %d\n", len(testData.DocumentSymbols))
	fmt.Printf("🔍 Total workspace symbols: %d\n", len(testData.WorkspaceSymbols))
	fmt.Printf("🔗 Reference mappings: %d\n", len(testData.References))
	fmt.Printf("💡 Hover contexts: %d\n", len(testData.Hovers))
	fmt.Printf("⚡ Completion contexts: %d\n", len(testData.Completions))
	
	return nil
}

// validateDataStructure checks the internal consistency of test data
func validateDataStructure(data *FatihColorTestData) error {
	// Check that all workspace symbols exist in document symbols
	documentSymbolNames := make(map[string]bool)
	for _, symbols := range data.DocumentSymbols {
		for _, symbol := range symbols {
			documentSymbolNames[symbol.Name] = true
		}
	}
	
	for _, wsSymbol := range data.WorkspaceSymbols {
		if !documentSymbolNames[wsSymbol.Name] {
			return fmt.Errorf("workspace symbol '%s' not found in document symbols", wsSymbol.Name)
		}
	}
	
	// Check that referenced symbols exist
	for symbolName := range data.References {
		if !documentSymbolNames[symbolName] {
			return fmt.Errorf("referenced symbol '%s' not found in document symbols", symbolName)
		}
	}
	
	// Validate definition mappings
	for filename, defMap := range data.Definitions {
		if _, exists := data.DocumentSymbols[filename]; !exists {
			return fmt.Errorf("definition file '%s' not found in document symbols", filename)
		}
		
		for symbolName := range defMap {
			if !documentSymbolNames[symbolName] {
				return fmt.Errorf("definition symbol '%s' not found in document symbols", symbolName)
			}
		}
	}
	
	// Validate hover data files exist
	for filename := range data.Hovers {
		if _, exists := data.DocumentSymbols[filename]; !exists {
			return fmt.Errorf("hover file '%s' not found in document symbols", filename)
		}
	}
	
	return nil
}

// GetTestDataSummary returns a summary of the test fixture data for debugging
func GetTestDataSummary() string {
	testData := GetFatihColorTestData()
	
	summary := fmt.Sprintf("Fatih/Color Test Data Summary:\n")
	summary += fmt.Sprintf("Repository: %s (%s)\n", testData.RepositoryInfo.Name, testData.RepositoryInfo.Version)
	summary += fmt.Sprintf("Go Version: %s\n", testData.RepositoryInfo.GoVersion)
	summary += fmt.Sprintf("Files: %v\n", testData.RepositoryInfo.Files)
	summary += fmt.Sprintf("\nDocument Symbols by File:\n")
	
	for filename, symbols := range testData.DocumentSymbols {
		summary += fmt.Sprintf("  %s: %d symbols\n", filename, len(symbols))
		
		// Show first few symbols as examples
		for i, symbol := range symbols {
			if i >= 3 { // Show only first 3 symbols per file
				summary += fmt.Sprintf("    ... and %d more\n", len(symbols)-3)
				break
			}
			summary += fmt.Sprintf("    - %s (%s) at %d:%d\n", symbol.Name, symbol.Type, symbol.Line, symbol.Character)
		}
	}
	
	summary += fmt.Sprintf("\nWorkspace Symbols: %d total\n", len(testData.WorkspaceSymbols))
	summary += fmt.Sprintf("References: %d symbols with reference data\n", len(testData.References))
	summary += fmt.Sprintf("Hover Data: %d files with hover information\n", len(testData.Hovers))
	summary += fmt.Sprintf("Completions: %d completion contexts\n", len(testData.Completions))
	summary += fmt.Sprintf("Definitions: %d files with definition mappings\n", len(testData.Definitions))
	
	return summary
}

// TestFatihColorFixtureData can be used in Go tests to validate the fixture data
func TestFatihColorFixtureData(t *testing.T) {
	err := ValidateTestDataIntegrity()
	if err != nil {
		t.Fatalf("Test fixture validation failed: %v", err)
	}
	
	// Test specific data access methods
	testData := GetFatihColorTestData()
	
	// Test document symbols access
	colorSymbols := testData.GetExpectedDocumentSymbols("color.go")
	if len(colorSymbols) == 0 {
		t.Error("No document symbols found for color.go")
	}
	
	// Test workspace symbols access
	wsSymbols := testData.GetExpectedWorkspaceSymbols()
	if len(wsSymbols) == 0 {
		t.Error("No workspace symbols found")
	}
	
	// Test references access
	colorRefs := testData.GetExpectedReferences("Color")
	if len(colorRefs) == 0 {
		t.Error("No references found for Color type")
	}
	
	// Test hover data access
	hovers := testData.GetExpectedHovers("color.go")
	if len(hovers) == 0 {
		t.Error("No hover data found for color.go")
	}
	
	// Test completions access
	completions := testData.GetExpectedCompletions("color_method_calls")
	if len(completions) == 0 {
		t.Error("No completions found for color_method_calls context")
	}
	
	// Test definition position access
	colorPos := testData.GetDefinitionPosition("color.go", "Color")
	if colorPos == nil {
		t.Error("No definition position found for Color type")
	}
	
	t.Logf("✅ All test fixture access methods working correctly")
}

// GetExpectedSymbolsForLSPFeature returns expected symbols organized by LSP feature type
func GetExpectedSymbolsForLSPFeature(feature string) map[string]interface{} {
	testData := GetFatihColorTestData()
	
	switch feature {
	case "textDocument/documentSymbol":
		return map[string]interface{}{
			"color.go":      testData.GetExpectedDocumentSymbols("color.go"),
			"doc.go":        testData.GetExpectedDocumentSymbols("doc.go"),
			"color_test.go": testData.GetExpectedDocumentSymbols("color_test.go"),
		}
		
	case "workspace/symbol":
		return map[string]interface{}{
			"symbols": testData.GetExpectedWorkspaceSymbols(),
		}
		
	case "textDocument/references":
		return map[string]interface{}{
			"Color":     testData.GetExpectedReferences("Color"),
			"Attribute": testData.GetExpectedReferences("Attribute"),
			"FgRed":     testData.GetExpectedReferences("FgRed"),
		}
		
	case "textDocument/hover":
		return map[string]interface{}{
			"color.go": testData.GetExpectedHovers("color.go"),
		}
		
	case "textDocument/completion":
		return map[string]interface{}{
			"method_calls":         testData.GetExpectedCompletions("color_method_calls"),
			"package_functions":    testData.GetExpectedCompletions("package_level_functions"),
			"attribute_constants":  testData.GetExpectedCompletions("attribute_constants"),
		}
		
	case "textDocument/definition":
		return map[string]interface{}{
			"definitions": testData.Definitions,
		}
		
	default:
		return map[string]interface{}{
			"error": fmt.Sprintf("Unknown LSP feature: %s", feature),
		}
	}
}