package cache

import "lsp-gateway/src/utils/filepattern"

// Common search utilities - shared helper functions for search operations

// matchFilePattern checks if a file URI matches a pattern
func (m *SCIPCacheManager) matchFilePattern(uri, pattern string) bool {
	return filepattern.Match(uri, pattern)
}

// sortEnhancedResults sorts enhanced symbol results by the specified criteria
// Note: sorting helpers are centralized in search.Sort* functions
