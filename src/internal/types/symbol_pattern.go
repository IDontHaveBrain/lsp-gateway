package types

import (
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"regexp"
	"strings"
)

type SymbolPatternQuery struct {
	Pattern     string           `json:"pattern"`
	SymbolKinds []lsp.SymbolKind `json:"symbolKinds,omitempty"`
	FilePattern string           `json:"filePattern,omitempty"`
	MaxResults  int              `json:"maxResults,omitempty"`
	IncludeCode bool             `json:"includeCode,omitempty"`
}

type SymbolPatternResult struct {
	Symbols    []EnhancedSymbolInfo `json:"symbols"`
	TotalCount int                  `json:"totalCount"`
	Truncated  bool                 `json:"truncated"`
}

type EnhancedSymbolInfo struct {
	lsp.SymbolInformation
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
func MatchSymbolPattern(symbol lsp.SymbolInformation, query SymbolPatternQuery) (bool, float64) {
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
		filePath := common.URIToFilePath(symbol.Location.URI)
		if !matchFilePattern(filePath, query.FilePattern) {
			return false, 0
		}
	}

	score := calculateMatchScore(symbolName, pattern, symbol.Kind)
	return true, score
}

func matchFilePattern(filePath, pattern string) bool {
	// Handle special case for current directory
	if pattern == "." || pattern == "./" {
		// Match all files (current directory)
		return true
	}

	// Handle directory patterns (e.g., "src/", "tests/internal/", "src")
	if strings.HasSuffix(pattern, "/") {
		// Explicit directory pattern with trailing slash
		return strings.Contains(filePath, pattern)
	}

	// If pattern looks like a directory path (contains slash but no glob/regex patterns)
	if strings.Contains(pattern, "/") && !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") && !strings.Contains(pattern, "[") && !strings.Contains(pattern, "(") {
		// Check if it's meant to be a directory (no file extension)
		lastPart := pattern[strings.LastIndex(pattern, "/")+1:]
		if !strings.Contains(lastPart, ".") {
			// Treat as directory prefix - add trailing slash
			pattern = pattern + "/"
			return strings.Contains(filePath, pattern)
		}
	}

	// Check for case-insensitive flag (?i)
	caseInsensitive := false
	if strings.HasPrefix(pattern, "(?i)") {
		caseInsensitive = true
		pattern = strings.TrimPrefix(pattern, "(?i)")
	}

	// Convert common glob patterns to regex
	regexPattern := pattern

	// Handle common glob patterns
	if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") {
		// Convert glob to regex (order matters!)
		// First escape dots
		regexPattern = strings.ReplaceAll(regexPattern, ".", "\\.")

		// Then handle ** patterns specially
		if strings.Contains(regexPattern, "**") {
			// Replace **/ with a pattern that matches any number of directories
			regexPattern = strings.ReplaceAll(regexPattern, "**/", "@@RECURSE@@")
			// Replace /** with a pattern that matches any number of directories
			regexPattern = strings.ReplaceAll(regexPattern, "/**", "/@@FULLRECURSE@@")
		}

		// Now handle remaining single * and ? patterns
		regexPattern = strings.ReplaceAll(regexPattern, "*", "[^/]*")
		regexPattern = strings.ReplaceAll(regexPattern, "?", ".")

		// Finally replace the recursive markers with the actual regex
		regexPattern = strings.ReplaceAll(regexPattern, "@@RECURSE@@", "(?:.*/)?")
		regexPattern = strings.ReplaceAll(regexPattern, "@@FULLRECURSE@@", ".*")

		// Add anchors for better matching
		if !strings.HasPrefix(regexPattern, "^") {
			if !strings.Contains(pattern, "/") {
				// For patterns like "*.go", match at any depth
				regexPattern = "(^|.*/)" + regexPattern + "$"
			} else if strings.HasPrefix(pattern, "**/") {
				// For patterns like "**/*.go", already has (?:.*/)?  prefix
				regexPattern = "^" + regexPattern + "$"
			} else {
				// For patterns with path like "src/*.go" or "src/**/*.go"
				// Allow matching at any directory level
				regexPattern = "(^|.*/)" + regexPattern + "$"
			}
		}
	}

	// Apply case insensitive flag if needed
	if caseInsensitive {
		regexPattern = "(?i)" + regexPattern
	}

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		// If pattern is not valid regex, fall back to simple contains
		return strings.Contains(filePath, pattern)
	}

	return re.MatchString(filePath)
}

func calculateMatchScore(symbolName, pattern string, kind lsp.SymbolKind) float64 {
	baseScore := 1.0

	if strings.EqualFold(symbolName, pattern) {
		baseScore += 2.0
	} else if strings.HasPrefix(strings.ToLower(symbolName), strings.ToLower(pattern)) {
		baseScore += 1.0
	}

	switch kind {
	case lsp.Class, lsp.Interface, lsp.Function:
		baseScore += 0.5
	case lsp.Method, lsp.Constructor:
		baseScore += 0.3
	}

	return baseScore
}
