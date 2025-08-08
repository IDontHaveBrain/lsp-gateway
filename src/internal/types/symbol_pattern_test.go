package types

import (
	"testing"
)

func TestMatchSymbolPattern(t *testing.T) {
	// Create test symbol
	symbol := SymbolInformation{
		Name: "NewLSPManager",
		Kind: Function,
		Location: Location{
			URI: "file:///home/test/file.go",
			Range: Range{
				Start: Position{Line: 10, Character: 0},
				End:   Position{Line: 20, Character: 0},
			},
		},
	}

	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"Literal match", "NewLSPManager", true},
		{"Prefix regex", "New.*", true},
		{"Suffix regex", ".*Manager", true},
		{"Case insensitive", "(?i)newlspmanager", true},
		{"Case insensitive partial", "(?i)NEW", true},
		{"No match", "OldManager", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := SymbolPatternQuery{
				Pattern:     tt.pattern,
				FilePattern: ".",
			}

			matched, _ := MatchSymbolPattern(symbol, query)
			if matched != tt.expected {
				t.Errorf("Pattern %s: expected %v, got %v", tt.pattern, tt.expected, matched)
			}
		})
	}
}
