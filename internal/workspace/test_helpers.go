package workspace

// Test helper functions shared across test files
// Note: This file only contains test-specific helpers that don't conflict with production code

// containsString checks if a string slice contains a specific item
// (alias to the production contains function for test compatibility)
func containsString(slice []string, item string) bool {
	return contains(slice, item)
}