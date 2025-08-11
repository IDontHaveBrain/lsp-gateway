package base

// This file contains usage examples for the ResultMerger system.
// These examples demonstrate how to use different merger types for various LSP operations.

// Example: Using SliceMerger for workspace symbols
func ExampleWorkspaceSymbolMerging() {
	// Configure merger for workspace symbols
	config := DefaultMergeConfig()
	config.MaxSize = 500 // Limit to 500 symbols
	config.LanguagePriority = []string{"go", "typescript", "python"}
	config.DeduplicationKey = "name" // Deduplicate by symbol name

	merger := NewSliceMerger[map[string]interface{}](config)

	// Simulate results from different language servers
	results := map[string][]map[string]interface{}{
		"go": {
			{"name": "func1", "kind": "function", "uri": "file:///main.go"},
			{"name": "struct1", "kind": "class", "uri": "file:///types.go"},
		},
		"typescript": {
			{"name": "method1", "kind": "method", "uri": "file:///app.ts"},
			{"name": "interface1", "kind": "interface", "uri": "file:///types.ts"},
		},
		"python": {
			{"name": "func1", "kind": "function", "uri": "file:///main.py"}, // Duplicate name
			{"name": "class1", "kind": "class", "uri": "file:///models.py"},
		},
	}

	// Merge with metadata
	result := merger.MergeWithMetadata(results)

	// result.Result contains merged symbols (deduplicated)
	// result.Languages contains ["go", "typescript", "python"] in priority order
	// result.Metadata contains merge statistics
	_ = result
}

// Example: Using MapMerger for completion items by prefix
func ExampleCompletionMerging() {
	config := DefaultMergeConfig()
	config.LanguagePriority = []string{"typescript", "javascript"}

	merger := NewMapMerger[string, map[string]interface{}](config)

	results := map[string]map[string]map[string]interface{}{
		"typescript": {
			"func":  {"label": "function", "kind": "keyword", "source": "typescript"},
			"const": {"label": "const", "kind": "keyword", "source": "typescript"},
		},
		"javascript": {
			"func": {"label": "function", "kind": "keyword", "source": "javascript"}, // Conflict - typescript wins
			"var":  {"label": "var", "kind": "keyword", "source": "javascript"},
		},
	}

	merged := merger.Merge(results)
	// typescript "func" definition wins due to priority
	_ = merged
}

// Example: Using CountMerger for reference counts
func ExampleReferenceCounting() {
	merger := NewCountMerger(nil)

	results := map[string]int{
		"go":         15, // 15 references found
		"typescript": 8,  // 8 references found
		"python":     3,  // 3 references found
	}

	totalRefs := merger.Merge(results) // Returns 26
	_ = totalRefs
}

// Example: Using FirstSuccessMerger for server initialization
func ExampleServerInitialization() {
	config := DefaultMergeConfig()
	config.LanguagePriority = []string{"go", "python", "java"}

	merger := NewFirstSuccessMerger[bool](config)

	results := map[string]bool{
		"java":   false, // Failed to initialize
		"go":     true,  // Successfully initialized
		"python": true,  // Also successful, but go has priority
	}

	result := merger.MergeWithMetadata(results)
	// result.Result is true (from "go")
	// result.Metadata["first_success_language"] is "go"
	_ = result
}

// Example: Using AllOrNothingMerger for critical operations
func ExampleCriticalOperationMerging() {
	config := DefaultMergeConfig()
	config.FailureThreshold = 0.2 // Allow max 20% failures

	merger := NewAllOrNothingMerger[map[string]interface{}](config)

	// All servers must succeed for this to work
	results := map[string][]map[string]interface{}{
		"go": {
			{"type": "definition", "uri": "file:///main.go"},
		},
		"typescript": {
			{"type": "definition", "uri": "file:///app.ts"},
		},
		"python": {}, // Failed - empty result
	}

	result := merger.MergeWithMetadata(results)
	// result.Result will be empty if validation fails
	// result.Metadata["validation_passed"] indicates success/failure
	_ = result
}

// Example: Custom merger configuration
func ExampleCustomConfiguration() {
	config := &MergeConfig{
		MaxSize:          100,
		LanguagePriority: []string{"rust", "go", "python"},
		DeduplicationKey: "uri",
		FailureThreshold: 0.3,
		CustomMetadata: map[string]string{
			"operation":  "workspace_symbol_search",
			"request_id": "req_12345",
		},
		AllowPartialMerge: true,
	}

	merger := NewSliceMerger[map[string]interface{}](config)

	results := map[string][]map[string]interface{}{
		"rust": {
			{"name": "fn main", "uri": "file:///main.rs"},
		},
	}

	result := merger.MergeWithMetadata(results)
	// result.Metadata will include custom metadata
	_ = result
}
