package types

import (
	"lsp-gateway/src/utils"
	"lsp-gateway/src/utils/filepattern"
	"regexp"
	"strings"
)

type SymbolPatternQuery struct {
	Pattern     string       `json:"pattern"`
	SymbolKinds []SymbolKind `json:"symbolKinds,omitempty"`
	FilePattern string       `json:"filePattern,omitempty"`
	MaxResults  int          `json:"maxResults,omitempty"`
	IncludeCode bool         `json:"includeCode,omitempty"`
}

type SymbolPatternResult struct {
	Symbols    []EnhancedSymbolInfo `json:"symbols"`
	TotalCount int                  `json:"totalCount"`
	Truncated  bool                 `json:"truncated"`
}

type EnhancedSymbolInfo struct {
	SymbolInformation
	Signature     string  `json:"signature,omitempty"`
	Documentation string  `json:"documentation,omitempty"`
	FilePath      string  `json:"filePath"`
	LineNumber    int     `json:"lineNumber"`
	EndLine       int     `json:"endLine,omitempty"`
	Container     string  `json:"container,omitempty"`
	Code          string  `json:"code,omitempty"`
	Score         float64 `json:"-"` // Internal use only for sorting
}

// MatchSymbolPattern matches a symbol against a pattern query and returns match status and score for sorting
func MatchSymbolPattern(symbol SymbolInformation, query SymbolPatternQuery) (bool, float64) {
	pattern := query.Pattern

	// Check for case-insensitive flag (?i)
	caseInsensitive := false
	if strings.HasPrefix(pattern, "(?i)") {
		caseInsensitive = true
		pattern = strings.TrimPrefix(pattern, "(?i)")
	}

	// Apply case insensitive flag if needed
	finalPattern := pattern
	if caseInsensitive {
		finalPattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(finalPattern)
	if err != nil {
		// If pattern is not valid regex, escape it and treat as literal
		escapedPattern := regexp.QuoteMeta(pattern)
		if caseInsensitive {
			escapedPattern = "(?i)" + escapedPattern
		}
		re, err = regexp.Compile(escapedPattern)
		if err != nil {
			return false, 0
		}
	}

	symbolName := symbol.Name

	if !re.MatchString(symbolName) {
		return false, 0
	}

	if len(query.SymbolKinds) > 0 {
		kindMatches := false
		for _, kind := range query.SymbolKinds {
			if symbol.Kind == kind {
				kindMatches = true
				break
			}
		}
		if !kindMatches {
			return false, 0
		}
	}

	if query.FilePattern != "" {
		filePath := utils.URIToFilePath(symbol.Location.URI)
		if !matchFilePattern(filePath, query.FilePattern) {
			return false, 0
		}
	}

	score := calculateMatchScore(symbolName, pattern, symbol.Kind)
	return true, score
}

func matchFilePattern(filePath, pattern string) bool {
	// Use the unified glob pattern matching from filepattern package
	return filepattern.Match(filePath, pattern)
}

func calculateMatchScore(symbolName, pattern string, kind SymbolKind) float64 {
	baseScore := 1.0

	if strings.EqualFold(symbolName, pattern) {
		baseScore += 2.0
	} else if strings.HasPrefix(strings.ToLower(symbolName), strings.ToLower(pattern)) {
		baseScore += 1.0
	}

	switch kind {
	case Class, Interface, Function:
		baseScore += 0.5
	case Method, Constructor:
		baseScore += 0.3
	}

	return baseScore
}
