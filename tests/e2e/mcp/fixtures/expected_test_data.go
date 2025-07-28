package fixtures

import (
	"encoding/json"
	"fmt"
)

// LSPPosition represents a position in LSP coordinate system (0-based)
type LSPPosition struct {
	Line      int `json:"line"`      // 0-based line number
	Character int `json:"character"` // 0-based character offset
}

// LSPRange represents a range in LSP coordinate system
type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

// ExpectedSymbol represents an expected document or workspace symbol
type ExpectedSymbol struct {
	Name          string      `json:"name"`
	Kind          int         `json:"kind"`       // LSP SymbolKind values
	File          string      `json:"file"`       // Relative path from workspace root
	Line          int         `json:"line"`       // 0-based line number  
	Character     int         `json:"character"`  // 0-based character offset
	Type          string      `json:"type"`       // Go type information
	Documentation string      `json:"documentation,omitempty"`
	Range         *LSPRange   `json:"range,omitempty"`
}

// ExpectedReference represents expected reference locations for a symbol
type ExpectedReference struct {
	Symbol    string        `json:"symbol"`     // Symbol identifier
	File      string        `json:"file"`       // File containing references
	Locations []LSPPosition `json:"locations"`  // All reference positions
	Context   string        `json:"context"`    // Reference context
}

// ExpectedHover represents expected hover information
type ExpectedHover struct {
	Position LSPPosition `json:"position"`   // Hover trigger position
	File     string      `json:"file"`       // File for hover
	Content  string      `json:"content"`    // Expected hover content
	Language string      `json:"language"`   // Content language (go)
}

// ExpectedCompletion represents expected completion items
type ExpectedCompletion struct {
	Position    LSPPosition `json:"position"`     // Completion trigger position
	File        string      `json:"file"`         // File for completion
	Label       string      `json:"label"`        // Completion item label
	Kind        int         `json:"kind"`         // CompletionItemKind
	Detail      string      `json:"detail"`       // Additional details
	InsertText  string      `json:"insert_text"`  // Text to insert
}

// RepositoryMetadata contains information about the test repository
type RepositoryMetadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	GoVersion   string `json:"go_version"`
	Description string `json:"description"`
	Files       []string `json:"files"`
}

// FatihColorTestData contains comprehensive test fixture data for fatih/color repository
type FatihColorTestData struct {
	RepositoryInfo   RepositoryMetadata                    `json:"repository_info"`
	DocumentSymbols  map[string][]ExpectedSymbol           `json:"document_symbols"`     // filename -> symbols
	WorkspaceSymbols []ExpectedSymbol                      `json:"workspace_symbols"`    // all symbols  
	References       map[string][]ExpectedReference        `json:"references"`           // symbol -> references
	Hovers           map[string][]ExpectedHover            `json:"hovers"`               // filename -> hovers
	Completions      map[string][]ExpectedCompletion       `json:"completions"`          // context -> completions
	Definitions      map[string]map[string]LSPPosition     `json:"definitions"`          // file -> symbol -> definition position
}

// GetFatihColorTestData returns comprehensive test fixture data for fatih/color repository
func GetFatihColorTestData() *FatihColorTestData {
	return &FatihColorTestData{
		RepositoryInfo: RepositoryMetadata{
			Name:        "github.com/fatih/color",
			Version:     "v1.18.0",
			GoVersion:   "1.11",
			Description: "Color package for Go (golang)",
			Files:       []string{"color.go", "color_test.go", "color_windows.go", "doc.go", "go.mod", "go.sum"},
		},
		
		// Document Symbols - organized by file
		DocumentSymbols: map[string][]ExpectedSymbol{
			"color.go": {
				// Package-level constants
				{Name: "Reset", Kind: 14, File: "color.go", Line: 51, Character: 1, Type: "Attribute", Documentation: "Base attributes"},
				{Name: "Bold", Kind: 14, File: "color.go", Line: 52, Character: 1, Type: "Attribute"},
				{Name: "Faint", Kind: 14, File: "color.go", Line: 53, Character: 1, Type: "Attribute"},
				{Name: "Italic", Kind: 14, File: "color.go", Line: 54, Character: 1, Type: "Attribute"},
				{Name: "Underline", Kind: 14, File: "color.go", Line: 55, Character: 1, Type: "Attribute"},
				{Name: "BlinkSlow", Kind: 14, File: "color.go", Line: 56, Character: 1, Type: "Attribute"},
				{Name: "BlinkRapid", Kind: 14, File: "color.go", Line: 57, Character: 1, Type: "Attribute"},
				{Name: "ReverseVideo", Kind: 14, File: "color.go", Line: 58, Character: 1, Type: "Attribute"},
				{Name: "Concealed", Kind: 14, File: "color.go", Line: 59, Character: 1, Type: "Attribute"},
				{Name: "CrossedOut", Kind: 14, File: "color.go", Line: 60, Character: 1, Type: "Attribute"},
				
				// Foreground colors
				{Name: "FgBlack", Kind: 14, File: "color.go", Line: 69, Character: 1, Type: "Attribute", Documentation: "Foreground text colors"},
				{Name: "FgRed", Kind: 14, File: "color.go", Line: 70, Character: 1, Type: "Attribute"},
				{Name: "FgGreen", Kind: 14, File: "color.go", Line: 71, Character: 1, Type: "Attribute"},
				{Name: "FgYellow", Kind: 14, File: "color.go", Line: 72, Character: 1, Type: "Attribute"},
				{Name: "FgBlue", Kind: 14, File: "color.go", Line: 73, Character: 1, Type: "Attribute"},
				{Name: "FgMagenta", Kind: 14, File: "color.go", Line: 74, Character: 1, Type: "Attribute"},
				{Name: "FgCyan", Kind: 14, File: "color.go", Line: 75, Character: 1, Type: "Attribute"},
				{Name: "FgWhite", Kind: 14, File: "color.go", Line: 76, Character: 1, Type: "Attribute"},
				
				// Hi-intensity foreground colors
				{Name: "FgHiBlack", Kind: 14, File: "color.go", Line: 85, Character: 1, Type: "Attribute", Documentation: "Foreground Hi-Intensity text colors"},
				{Name: "FgHiRed", Kind: 14, File: "color.go", Line: 86, Character: 1, Type: "Attribute"},
				{Name: "FgHiGreen", Kind: 14, File: "color.go", Line: 87, Character: 1, Type: "Attribute"},
				{Name: "FgHiYellow", Kind: 14, File: "color.go", Line: 88, Character: 1, Type: "Attribute"},
				{Name: "FgHiBlue", Kind: 14, File: "color.go", Line: 89, Character: 1, Type: "Attribute"},
				{Name: "FgHiMagenta", Kind: 14, File: "color.go", Line: 90, Character: 1, Type: "Attribute"},
				{Name: "FgHiCyan", Kind: 14, File: "color.go", Line: 91, Character: 1, Type: "Attribute"},
				{Name: "FgHiWhite", Kind: 14, File: "color.go", Line: 92, Character: 1, Type: "Attribute"},
				
				// Background colors
				{Name: "BgBlack", Kind: 14, File: "color.go", Line: 101, Character: 1, Type: "Attribute", Documentation: "Background text colors"},
				{Name: "BgRed", Kind: 14, File: "color.go", Line: 102, Character: 1, Type: "Attribute"},
				{Name: "BgGreen", Kind: 14, File: "color.go", Line: 103, Character: 1, Type: "Attribute"},
				{Name: "BgYellow", Kind: 14, File: "color.go", Line: 104, Character: 1, Type: "Attribute"},
				{Name: "BgBlue", Kind: 14, File: "color.go", Line: 105, Character: 1, Type: "Attribute"},
				{Name: "BgMagenta", Kind: 14, File: "color.go", Line: 106, Character: 1, Type: "Attribute"},
				{Name: "BgCyan", Kind: 14, File: "color.go", Line: 107, Character: 1, Type: "Attribute"},
				{Name: "BgWhite", Kind: 14, File: "color.go", Line: 108, Character: 1, Type: "Attribute"},
				
				// Variables
				{Name: "NoColor", Kind: 13, File: "color.go", Line: 15, Character: 1, Type: "bool", Documentation: "NoColor defines if the output is colorized or not"},
				{Name: "Output", Kind: 13, File: "color.go", Line: 22, Character: 1, Type: "io.Writer", Documentation: "Output defines the standard output of the print functions"},
				{Name: "Error", Kind: 13, File: "color.go", Line: 25, Character: 1, Type: "io.Writer", Documentation: "Error defines a color supporting writer for os.Stderr"},
				
				// Types
				{Name: "Attribute", Kind: 25, File: "color.go", Line: 50, Character: 5, Type: "int", Documentation: "Attribute defines a single SGR Code"},
				{Name: "Color", Kind: 23, File: "color.go", Line: 44, Character: 5, Type: "struct", Documentation: "Color defines a custom color object which is defined by SGR parameters"},
				
				// Constructor functions
				{Name: "New", Kind: 12, File: "color.go", Line: 146, Character: 5, Type: "func", Documentation: "New returns a newly created color object"},
				{Name: "RGB", Kind: 12, File: "color.go", Line: 160, Character: 5, Type: "func", Documentation: "RGB returns a new foreground color in 24-bit RGB"},
				{Name: "BgRGB", Kind: 12, File: "color.go", Line: 165, Character: 5, Type: "func", Documentation: "BgRGB returns a new background color in 24-bit RGB"},
				{Name: "Set", Kind: 12, File: "color.go", Line: 185, Character: 5, Type: "func", Documentation: "Set sets the given parameters immediately"},
				{Name: "Unset", Kind: 12, File: "color.go", Line: 193, Character: 5, Type: "func", Documentation: "Unset resets all escape attributes and clears the output"},
				
				// Color helper functions
				{Name: "Black", Kind: 12, File: "color.go", Line: 544, Character: 5, Type: "func", Documentation: "Black is a convenient helper function to print with black foreground"},
				{Name: "Red", Kind: 12, File: "color.go", Line: 548, Character: 5, Type: "func"},
				{Name: "Green", Kind: 12, File: "color.go", Line: 552, Character: 5, Type: "func"},
				{Name: "Yellow", Kind: 12, File: "color.go", Line: 556, Character: 5, Type: "func"},
				{Name: "Blue", Kind: 12, File: "color.go", Line: 560, Character: 5, Type: "func"},
				{Name: "Magenta", Kind: 12, File: "color.go", Line: 564, Character: 5, Type: "func"},
				{Name: "Cyan", Kind: 12, File: "color.go", Line: 568, Character: 5, Type: "func"},
				{Name: "White", Kind: 12, File: "color.go", Line: 572, Character: 5, Type: "func"},
				
				// String helper functions
				{Name: "BlackString", Kind: 12, File: "color.go", Line: 576, Character: 5, Type: "func"},
				{Name: "RedString", Kind: 12, File: "color.go", Line: 580, Character: 5, Type: "func"},
				{Name: "GreenString", Kind: 12, File: "color.go", Line: 584, Character: 5, Type: "func"},
				{Name: "YellowString", Kind: 12, File: "color.go", Line: 588, Character: 5, Type: "func"},
				{Name: "BlueString", Kind: 12, File: "color.go", Line: 592, Character: 5, Type: "func"},
				{Name: "MagentaString", Kind: 12, File: "color.go", Line: 596, Character: 5, Type: "func"},
				{Name: "CyanString", Kind: 12, File: "color.go", Line: 602, Character: 5, Type: "func"},
				{Name: "WhiteString", Kind: 12, File: "color.go", Line: 606, Character: 5, Type: "func"},
				
				// Hi-intensity helper functions
				{Name: "HiBlack", Kind: 12, File: "color.go", Line: 610, Character: 5, Type: "func"},
				{Name: "HiRed", Kind: 12, File: "color.go", Line: 614, Character: 5, Type: "func"},
				{Name: "HiGreen", Kind: 12, File: "color.go", Line: 618, Character: 5, Type: "func"},
				{Name: "HiYellow", Kind: 12, File: "color.go", Line: 622, Character: 5, Type: "func"},
				{Name: "HiBlue", Kind: 12, File: "color.go", Line: 626, Character: 5, Type: "func"},
				{Name: "HiMagenta", Kind: 12, File: "color.go", Line: 630, Character: 5, Type: "func"},
				{Name: "HiCyan", Kind: 12, File: "color.go", Line: 634, Character: 5, Type: "func"},
				{Name: "HiWhite", Kind: 12, File: "color.go", Line: 638, Character: 5, Type: "func"},
			},
			
			"doc.go": {
				{Name: "color", Kind: 4, File: "doc.go", Line: 0, Character: 8, Type: "package", Documentation: "Package color is an ANSI color package to output colorized or SGR defined output"},
			},
			
			"color_test.go": {
				{Name: "TestColor", Kind: 12, File: "color_test.go", Line: 15, Character: 5, Type: "func"},
				{Name: "TestColorEquals", Kind: 12, File: "color_test.go", Line: 45, Character: 5, Type: "func"},
				{Name: "TestNoColor", Kind: 12, File: "color_test.go", Line: 78, Character: 5, Type: "func"},
				{Name: "ExampleColor", Kind: 12, File: "color_test.go", Line: 120, Character: 5, Type: "func"},
			},
		},
		
		// Workspace Symbols - all symbols across files
		WorkspaceSymbols: []ExpectedSymbol{
			{Name: "Color", Kind: 23, File: "color.go", Line: 44, Character: 5, Type: "struct"},
			{Name: "Attribute", Kind: 25, File: "color.go", Line: 50, Character: 5, Type: "int"},
			{Name: "New", Kind: 12, File: "color.go", Line: 146, Character: 5, Type: "func"},
			{Name: "Red", Kind: 12, File: "color.go", Line: 548, Character: 5, Type: "func"},
			{Name: "Blue", Kind: 12, File: "color.go", Line: 560, Character: 5, Type: "func"},
			{Name: "Green", Kind: 12, File: "color.go", Line: 552, Character: 5, Type: "func"},
			{Name: "Yellow", Kind: 12, File: "color.go", Line: 556, Character: 5, Type: "func"},
			{Name: "FgRed", Kind: 14, File: "color.go", Line: 70, Character: 1, Type: "Attribute"},
			{Name: "FgBlue", Kind: 14, File: "color.go", Line: 73, Character: 1, Type: "Attribute"},
			{Name: "FgGreen", Kind: 14, File: "color.go", Line: 71, Character: 1, Type: "Attribute"},
			{Name: "Bold", Kind: 14, File: "color.go", Line: 52, Character: 1, Type: "Attribute"},
			{Name: "Reset", Kind: 14, File: "color.go", Line: 51, Character: 1, Type: "Attribute"},
		},
		
		// References - symbol usage locations
		References: map[string][]ExpectedReference{
			"Color": {
				{
					Symbol: "Color",
					File: "color.go",
					Locations: []LSPPosition{
						{Line: 44, Character: 5},   // Type definition
						{Line: 146, Character: 25}, // New function return type
						{Line: 160, Character: 25}, // RGB function return type
						{Line: 185, Character: 25}, // Set function return type
					},
					Context: "type usage",
				},
			},
			"Attribute": {
				{
					Symbol: "Attribute",
					File: "color.go",
					Locations: []LSPPosition{
						{Line: 50, Character: 5},   // Type definition
						{Line: 146, Character: 15}, // New function parameter
						{Line: 185, Character: 10}, // Set function parameter
					},
					Context: "type usage",
				},
			},
			"FgRed": {
				{
					Symbol: "FgRed",
					File: "color.go",
					Locations: []LSPPosition{
						{Line: 70, Character: 1},   // Constant definition
						{Line: 548, Character: 20}, // Used in Red function
					},
					Context: "constant usage",
				},
			},
		},
		
		// Hover information
		Hovers: map[string][]ExpectedHover{
			"color.go": {
				{
					Position: LSPPosition{Line: 44, Character: 10},
					File: "color.go",
					Content: "type Color struct\n\nColor defines a custom color object which is defined by SGR parameters.",
					Language: "go",
				},
				{
					Position: LSPPosition{Line: 50, Character: 10},
					File: "color.go",
					Content: "type Attribute int\n\nAttribute defines a single SGR Code",
					Language: "go",
				},
				{
					Position: LSPPosition{Line: 146, Character: 10},
					File: "color.go",
					Content: "func New(value ...Attribute) *Color\n\nNew returns a newly created color object.",
					Language: "go",
				},
				{
					Position: LSPPosition{Line: 548, Character: 10},
					File: "color.go",
					Content: "func Red(format string, a ...interface{})\n\nRed is a convenient helper function to print with red foreground. A newline is appended to format by default.",
					Language: "go",
				},
				{
					Position: LSPPosition{Line: 70, Character: 5},
					File: "color.go",
					Content: "const FgRed Attribute = iota + 30\n\nForeground text colors",
					Language: "go",
				},
			},
		},
		
		// Completion suggestions
		Completions: map[string][]ExpectedCompletion{
			"color_method_calls": {
				{
					Position: LSPPosition{Line: 100, Character: 5},
					File: "color.go",
					Label: "Print",
					Kind: 2, // Method
					Detail: "func (c *Color) Print(a ...interface{}) (n int, err error)",
					InsertText: "Print",
				},
				{
					Position: LSPPosition{Line: 100, Character: 5},
					File: "color.go",
					Label: "Printf",
					Kind: 2, // Method
					Detail: "func (c *Color) Printf(format string, a ...interface{}) (n int, err error)",
					InsertText: "Printf",
				},
				{
					Position: LSPPosition{Line: 100, Character: 5},
					File: "color.go",
					Label: "Println",
					Kind: 2, // Method
					Detail: "func (c *Color) Println(a ...interface{}) (n int, err error)",
					InsertText: "Println",
				},
				{
					Position: LSPPosition{Line: 100, Character: 5},
					File: "color.go",
					Label: "Sprint",
					Kind: 2, // Method
					Detail: "func (c *Color) Sprint(a ...interface{}) string",
					InsertText: "Sprint",
				},
				{
					Position: LSPPosition{Line: 100, Character: 5},
					File: "color.go",
					Label: "Add",
					Kind: 2, // Method
					Detail: "func (c *Color) Add(value ...Attribute) *Color",
					InsertText: "Add",
				},
			},
			"package_level_functions": {
				{
					Position: LSPPosition{Line: 50, Character: 10},
					File: "color.go",
					Label: "Red",
					Kind: 3, // Function
					Detail: "func Red(format string, a ...interface{})",
					InsertText: "Red",
				},
				{
					Position: LSPPosition{Line: 50, Character: 10},
					File: "color.go",
					Label: "Blue",
					Kind: 3, // Function
					Detail: "func Blue(format string, a ...interface{})",
					InsertText: "Blue",
				},
				{
					Position: LSPPosition{Line: 50, Character: 10},
					File: "color.go",
					Label: "Green",
					Kind: 3, // Function
					Detail: "func Green(format string, a ...interface{})",
					InsertText: "Green",
				},
				{
					Position: LSPPosition{Line: 50, Character: 10},
					File: "color.go",
					Label: "New",
					Kind: 3, // Function
					Detail: "func New(value ...Attribute) *Color",
					InsertText: "New",
				},
			},
			"attribute_constants": {
				{
					Position: LSPPosition{Line: 80, Character: 15},
					File: "color.go",
					Label: "FgRed",
					Kind: 21, // Constant
					Detail: "const FgRed Attribute",
					InsertText: "FgRed",
				},
				{
					Position: LSPPosition{Line: 80, Character: 15},
					File: "color.go",
					Label: "FgBlue",
					Kind: 21, // Constant
					Detail: "const FgBlue Attribute",
					InsertText: "FgBlue",
				},
				{
					Position: LSPPosition{Line: 80, Character: 15},
					File: "color.go",
					Label: "Bold",
					Kind: 21, // Constant
					Detail: "const Bold Attribute",
					InsertText: "Bold",
				},
				{
					Position: LSPPosition{Line: 80, Character: 15},
					File: "color.go",
					Label: "Reset",
					Kind: 21, // Constant
					Detail: "const Reset Attribute",
					InsertText: "Reset",
				},
			},
		},
		
		// Definition mappings
		Definitions: map[string]map[string]LSPPosition{
			"color.go": {
				"Color":     {Line: 44, Character: 5},
				"Attribute": {Line: 50, Character: 5},
				"New":       {Line: 146, Character: 5},
				"Red":       {Line: 548, Character: 5},
				"Blue":      {Line: 560, Character: 5},
				"Green":     {Line: 552, Character: 5},
				"FgRed":     {Line: 70, Character: 1},
				"FgBlue":    {Line: 73, Character: 1},
				"FgGreen":   {Line: 71, Character: 1},
				"Bold":      {Line: 52, Character: 1},
				"Reset":     {Line: 51, Character: 1},
			},
		},
	}
}

// GetExpectedDocumentSymbols returns expected document symbols for a specific file
func (data *FatihColorTestData) GetExpectedDocumentSymbols(filename string) []ExpectedSymbol {
	if symbols, exists := data.DocumentSymbols[filename]; exists {
		return symbols
	}
	return []ExpectedSymbol{}
}

// GetExpectedWorkspaceSymbols returns all expected workspace symbols
func (data *FatihColorTestData) GetExpectedWorkspaceSymbols() []ExpectedSymbol {
	return data.WorkspaceSymbols
}

// GetExpectedReferences returns expected references for a symbol
func (data *FatihColorTestData) GetExpectedReferences(symbol string) []ExpectedReference {
	if refs, exists := data.References[symbol]; exists {
		return refs
	}
	return []ExpectedReference{}
}

// GetExpectedHovers returns expected hover information for a file
func (data *FatihColorTestData) GetExpectedHovers(filename string) []ExpectedHover {
	if hovers, exists := data.Hovers[filename]; exists {
		return hovers
	}
	return []ExpectedHover{}
}

// GetExpectedCompletions returns expected completion items for a context
func (data *FatihColorTestData) GetExpectedCompletions(context string) []ExpectedCompletion {
	if completions, exists := data.Completions[context]; exists {
		return completions
	}
	return []ExpectedCompletion{}
}

// GetDefinitionPosition returns the expected definition position for a symbol
func (data *FatihColorTestData) GetDefinitionPosition(filename, symbol string) *LSPPosition {
	if fileMap, exists := data.Definitions[filename]; exists {
		if pos, symbolExists := fileMap[symbol]; symbolExists {
			return &pos
		}
	}
	return nil
}

// ToJSON serializes the test data to JSON format
func (data *FatihColorTestData) ToJSON() ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}

// ValidateTestData performs basic validation on the test fixture data
func (data *FatihColorTestData) ValidateTestData() []string {
	var errors []string
	
	// Validate repository info
	if data.RepositoryInfo.Name == "" {
		errors = append(errors, "repository name is empty")
	}
	
	// Validate document symbols have valid positions
	for filename, symbols := range data.DocumentSymbols {
		for _, symbol := range symbols {
			if symbol.Line < 0 || symbol.Character < 0 {
				errors = append(errors, fmt.Sprintf("invalid position for symbol %s in %s", symbol.Name, filename))
			}
		}
	}
	
	// Validate references point to valid symbols
	for symbolName, refs := range data.References {
		for _, ref := range refs {
			if ref.Symbol != symbolName {
				errors = append(errors, fmt.Sprintf("reference symbol mismatch: expected %s, got %s", symbolName, ref.Symbol))
			}
		}
	}
	
	return errors
}

// LSP SymbolKind constants for reference
const (
	SymbolKindFile          = 1
	SymbolKindModule        = 2
	SymbolKindNamespace     = 3
	SymbolKindPackage       = 4
	SymbolKindClass         = 5
	SymbolKindMethod        = 6
	SymbolKindProperty      = 7
	SymbolKindField         = 8
	SymbolKindConstructor   = 9
	SymbolKindEnum          = 10
	SymbolKindInterface     = 11
	SymbolKindFunction      = 12
	SymbolKindVariable      = 13
	SymbolKindConstant      = 14
	SymbolKindString        = 15
	SymbolKindNumber        = 16
	SymbolKindBoolean       = 17
	SymbolKindArray         = 18
	SymbolKindObject        = 19
	SymbolKindKey           = 20
	SymbolKindNull          = 21
	SymbolKindEnumMember    = 22
	SymbolKindStruct        = 23
	SymbolKindEvent         = 24
	SymbolKindOperator      = 25
	SymbolKindTypeParameter = 26
)

// LSP CompletionItemKind constants for reference
const (
	CompletionItemKindText          = 1
	CompletionItemKindMethod        = 2
	CompletionItemKindFunction      = 3
	CompletionItemKindConstructor   = 4
	CompletionItemKindField         = 5
	CompletionItemKindVariable      = 6
	CompletionItemKindClass         = 7
	CompletionItemKindInterface     = 8
	CompletionItemKindModule        = 9
	CompletionItemKindProperty      = 10
	CompletionItemKindUnit          = 11
	CompletionItemKindValue         = 12
	CompletionItemKindEnum          = 13
	CompletionItemKindKeyword       = 14
	CompletionItemKindSnippet       = 15
	CompletionItemKindColor         = 16
	CompletionItemKindFile          = 17
	CompletionItemKindReference     = 18
	CompletionItemKindFolder        = 19
	CompletionItemKindEnumMember    = 20
	CompletionItemKindConstant      = 21
	CompletionItemKindStruct        = 22
	CompletionItemKindEvent         = 23
	CompletionItemKindOperator      = 24
	CompletionItemKindTypeParameter = 25
)