package base

import (
	"fmt"
	"reflect"
)

// ResultMerger provides generic result merging strategies for different response types
type ResultMerger[T any] interface {
	Merge(results map[string]T) T
	MergeWithMetadata(results map[string]T) MergeResult[T]
}

// MergeResult contains merged result with metadata about the merge operation
type MergeResult[T any] struct {
	Result      T                      `json:"result"`
	SourceCount int                    `json:"source_count"`
	Languages   []string               `json:"languages"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// MergeConfig provides configuration for merge operations
type MergeConfig struct {
	MaxSize           int               `json:"max_size"`            // Maximum result size (0 = unlimited)
	LanguagePriority  []string          `json:"language_priority"`   // Preferred language order
	DeduplicationKey  string            `json:"deduplication_key"`   // Key field for deduplication
	FailureThreshold  float64           `json:"failure_threshold"`   // Max failure ratio (0.0-1.0)
	CustomMetadata    map[string]string `json:"custom_metadata"`     // Custom metadata to include
	AllowPartialMerge bool              `json:"allow_partial_merge"` // Allow merging with some failures
}

// DefaultMergeConfig returns sensible defaults
func DefaultMergeConfig() *MergeConfig {
	return &MergeConfig{
		MaxSize:           1000,
		LanguagePriority:  []string{"go", "typescript", "javascript", "python", "java", "rust"},
		DeduplicationKey:  "",
		FailureThreshold:  0.5, // Allow up to 50% failures
		CustomMetadata:    make(map[string]string),
		AllowPartialMerge: true,
	}
}

// SliceMerger merges slice results by concatenation with deduplication support
type SliceMerger[T any] struct {
	Config *MergeConfig
}

// NewSliceMerger creates a new slice merger with optional configuration
func NewSliceMerger[T any](config *MergeConfig) *SliceMerger[T] {
	if config == nil {
		config = DefaultMergeConfig()
	}
	return &SliceMerger[T]{Config: config}
}

// Merge concatenates slices from different language servers
func (m *SliceMerger[T]) Merge(results map[string][]T) []T {
	result := m.MergeWithMetadata(results)
	return result.Result
}

// MergeWithMetadata concatenates slices and provides merge metadata
func (m *SliceMerger[T]) MergeWithMetadata(results map[string][]T) MergeResult[[]T] {
	merged := make([]T, 0)
	languages := make([]string, 0, len(results))
	totalSources := 0
	seenItems := make(map[string]bool)

	metadata := map[string]interface{}{
		"total_input_sources":   len(results),
		"deduplication_enabled": m.Config.DeduplicationKey != "",
		"size_limit_applied":    m.Config.MaxSize > 0,
	}

	// Merge in priority order if specified
	orderedLangs := m.getOrderedLanguages(results)

	for _, lang := range orderedLangs {
		items := results[lang]
		if len(items) == 0 {
			continue
		}

		languages = append(languages, lang)
		totalSources++

		for _, item := range items {
			// Apply size limit if configured
			if m.Config.MaxSize > 0 && len(merged) >= m.Config.MaxSize {
				metadata["size_limit_reached"] = true
				break
			}

			// Apply deduplication if configured
			if m.Config.DeduplicationKey != "" {
				key := m.extractDeduplicationKey(item)
				if seenItems[key] {
					continue
				}
				seenItems[key] = true
			}

			merged = append(merged, item)
		}
	}

	// Add custom metadata
	for k, v := range m.Config.CustomMetadata {
		metadata[k] = v
	}

	metadata["items_merged"] = len(merged)
	metadata["duplicates_removed"] = len(seenItems) - len(merged)

	return MergeResult[[]T]{
		Result:      merged,
		SourceCount: totalSources,
		Languages:   languages,
		Metadata:    metadata,
	}
}

// MapMerger merges map results with conflict resolution
type MapMerger[K comparable, V any] struct {
	Config *MergeConfig
}

// NewMapMerger creates a new map merger
func NewMapMerger[K comparable, V any](config *MergeConfig) *MapMerger[K, V] {
	if config == nil {
		config = DefaultMergeConfig()
	}
	return &MapMerger[K, V]{Config: config}
}

// Merge combines maps from different language servers
func (m *MapMerger[K, V]) Merge(results map[string]map[K]V) map[K]V {
	result := m.MergeWithMetadata(results)
	return result.Result
}

// MergeWithMetadata combines maps and provides merge metadata
func (m *MapMerger[K, V]) MergeWithMetadata(results map[string]map[K]V) MergeResult[map[K]V] {
	merged := make(map[K]V)
	languages := make([]string, 0, len(results))
	totalSources := 0
	conflicts := 0

	metadata := map[string]interface{}{
		"total_input_sources": len(results),
		"conflict_resolution": "priority_based",
	}

	// Merge in priority order
	orderedLangs := m.getOrderedLanguages(results)

	for _, lang := range orderedLangs {
		langMap := results[lang]
		if len(langMap) == 0 {
			continue
		}

		languages = append(languages, lang)
		totalSources++

		for key, value := range langMap {
			if _, exists := merged[key]; exists {
				conflicts++
				// Priority-based resolution: first language in priority order wins
				continue
			}

			// Apply size limit if configured
			if m.Config.MaxSize > 0 && len(merged) >= m.Config.MaxSize {
				metadata["size_limit_reached"] = true
				break
			}

			merged[key] = value
		}
	}

	// Add custom metadata
	for k, v := range m.Config.CustomMetadata {
		metadata[k] = v
	}

	metadata["items_merged"] = len(merged)
	metadata["conflicts_resolved"] = conflicts

	return MergeResult[map[K]V]{
		Result:      merged,
		SourceCount: totalSources,
		Languages:   languages,
		Metadata:    metadata,
	}
}

// CountMerger sums numeric results
type CountMerger struct {
	Config *MergeConfig
}

// NewCountMerger creates a new count merger
func NewCountMerger(config *MergeConfig) *CountMerger {
	if config == nil {
		config = DefaultMergeConfig()
	}
	return &CountMerger{Config: config}
}

// Merge sums all numeric values
func (m *CountMerger) Merge(results map[string]int) int {
	result := m.MergeWithMetadata(results)
	return result.Result
}

// MergeWithMetadata sums values and provides merge metadata
func (m *CountMerger) MergeWithMetadata(results map[string]int) MergeResult[int] {
	var sum int
	languages := make([]string, 0, len(results))
	totalSources := 0

	metadata := map[string]interface{}{
		"total_input_sources": len(results),
		"operation":           "sum",
	}

	for lang, count := range results {
		languages = append(languages, lang)
		totalSources++
		sum += count
	}

	// Add custom metadata
	for k, v := range m.Config.CustomMetadata {
		metadata[k] = v
	}

	metadata["sum_total"] = sum

	return MergeResult[int]{
		Result:      sum,
		SourceCount: totalSources,
		Languages:   languages,
		Metadata:    metadata,
	}
}

// FirstSuccessMerger returns the first successful result (for startup/shutdown scenarios)
type FirstSuccessMerger[T any] struct {
	Config *MergeConfig
}

// NewFirstSuccessMerger creates a new first success merger
func NewFirstSuccessMerger[T any](config *MergeConfig) *FirstSuccessMerger[T] {
	if config == nil {
		config = DefaultMergeConfig()
	}
	return &FirstSuccessMerger[T]{Config: config}
}

// Merge returns the first non-zero/non-nil result in priority order
func (m *FirstSuccessMerger[T]) Merge(results map[string]T) T {
	result := m.MergeWithMetadata(results)
	return result.Result
}

// MergeWithMetadata returns first success and provides merge metadata
func (m *FirstSuccessMerger[T]) MergeWithMetadata(results map[string]T) MergeResult[T] {
	var result T
	languages := make([]string, 0, len(results))
	totalSources := 0
	successCount := 0

	metadata := map[string]interface{}{
		"total_input_sources": len(results),
		"strategy":            "first_success",
	}

	// Try in priority order
	orderedLangs := m.getOrderedLanguages(results)

	for _, lang := range orderedLangs {
		value := results[lang]
		totalSources++

		// Check if value is considered "successful" (non-zero/non-nil)
		if !m.isZeroValue(value) {
			if successCount == 0 {
				result = value
				languages = append(languages, lang)
			}
			successCount++
		}
	}

	// Add custom metadata
	for k, v := range m.Config.CustomMetadata {
		metadata[k] = v
	}

	metadata["success_count"] = successCount
	metadata["first_success_language"] = ""
	if len(languages) > 0 {
		metadata["first_success_language"] = languages[0]
	}

	return MergeResult[T]{
		Result:      result,
		SourceCount: totalSources,
		Languages:   languages,
		Metadata:    metadata,
	}
}

// AllOrNothingMerger requires all operations to succeed
type AllOrNothingMerger[T any] struct {
	Config *MergeConfig
}

// NewAllOrNothingMerger creates a new all-or-nothing merger
func NewAllOrNothingMerger[T any](config *MergeConfig) *AllOrNothingMerger[T] {
	if config == nil {
		config = DefaultMergeConfig()
	}
	return &AllOrNothingMerger[T]{Config: config}
}

// Merge returns results only if all sources succeeded
func (m *AllOrNothingMerger[T]) Merge(results map[string][]T) []T {
	result := m.MergeWithMetadata(results)
	return result.Result
}

// MergeWithMetadata validates all succeeded and provides merge metadata
func (m *AllOrNothingMerger[T]) MergeWithMetadata(results map[string][]T) MergeResult[[]T] {
	var merged []T
	languages := make([]string, 0, len(results))
	totalSources := 0
	successCount := 0

	metadata := map[string]interface{}{
		"total_input_sources": len(results),
		"strategy":            "all_or_nothing",
		"validation_passed":   false,
	}

	// First pass: validate all sources succeeded
	for lang, items := range results {
		totalSources++
		if len(items) > 0 {
			successCount++
			languages = append(languages, lang)
		}
	}

	// Check if failure threshold is exceeded
	failureRatio := float64(totalSources-successCount) / float64(totalSources)
	if failureRatio > m.Config.FailureThreshold {
		metadata["failure_ratio"] = failureRatio
		return MergeResult[[]T]{
			Result:      merged,
			SourceCount: totalSources,
			Languages:   []string{},
			Metadata:    metadata,
		}
	}

	// Second pass: merge all results if validation passed
	metadata["validation_passed"] = true
	for _, lang := range languages {
		items := results[lang]
		merged = append(merged, items...)

		// Apply size limit if configured
		if m.Config.MaxSize > 0 && len(merged) >= m.Config.MaxSize {
			merged = merged[:m.Config.MaxSize]
			metadata["size_limit_reached"] = true
			break
		}
	}

	// Add custom metadata
	for k, v := range m.Config.CustomMetadata {
		metadata[k] = v
	}

	metadata["items_merged"] = len(merged)
	metadata["success_count"] = successCount
	metadata["failure_ratio"] = failureRatio

	return MergeResult[[]T]{
		Result:      merged,
		SourceCount: totalSources,
		Languages:   languages,
		Metadata:    metadata,
	}
}

// Helper methods

// getOrderedLanguages returns languages in priority order
func (m *SliceMerger[T]) getOrderedLanguages(results map[string][]T) []string {
	if len(m.Config.LanguagePriority) == 0 {
		// No priority specified, return in map iteration order
		langs := make([]string, 0, len(results))
		for lang := range results {
			langs = append(langs, lang)
		}
		return langs
	}

	ordered := make([]string, 0, len(results))
	seen := make(map[string]bool)

	// Add languages in priority order
	for _, priority := range m.Config.LanguagePriority {
		if _, exists := results[priority]; exists {
			ordered = append(ordered, priority)
			seen[priority] = true
		}
	}

	// Add remaining languages not in priority list
	for lang := range results {
		if !seen[lang] {
			ordered = append(ordered, lang)
		}
	}

	return ordered
}

// getOrderedLanguages for MapMerger
func (m *MapMerger[K, V]) getOrderedLanguages(results map[string]map[K]V) []string {
	if len(m.Config.LanguagePriority) == 0 {
		langs := make([]string, 0, len(results))
		for lang := range results {
			langs = append(langs, lang)
		}
		return langs
	}

	ordered := make([]string, 0, len(results))
	seen := make(map[string]bool)

	for _, priority := range m.Config.LanguagePriority {
		if _, exists := results[priority]; exists {
			ordered = append(ordered, priority)
			seen[priority] = true
		}
	}

	for lang := range results {
		if !seen[lang] {
			ordered = append(ordered, lang)
		}
	}

	return ordered
}

// getOrderedLanguages for FirstSuccessMerger
func (m *FirstSuccessMerger[T]) getOrderedLanguages(results map[string]T) []string {
	if len(m.Config.LanguagePriority) == 0 {
		langs := make([]string, 0, len(results))
		for lang := range results {
			langs = append(langs, lang)
		}
		return langs
	}

	ordered := make([]string, 0, len(results))
	seen := make(map[string]bool)

	for _, priority := range m.Config.LanguagePriority {
		if _, exists := results[priority]; exists {
			ordered = append(ordered, priority)
			seen[priority] = true
		}
	}

	for lang := range results {
		if !seen[lang] {
			ordered = append(ordered, lang)
		}
	}

	return ordered
}

// extractDeduplicationKey extracts a key for deduplication
func (m *SliceMerger[T]) extractDeduplicationKey(item T) string {
	if m.Config.DeduplicationKey == "" {
		// Fallback to string representation of the item
		return fmt.Sprintf("%v", item)
	}

	// Use reflection to extract the specified field
	v := reflect.ValueOf(item)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Map {
		if mapVal := v.MapIndex(reflect.ValueOf(m.Config.DeduplicationKey)); mapVal.IsValid() {
			return fmt.Sprintf("%v", mapVal.Interface())
		}
	} else if v.Kind() == reflect.Struct {
		if field := v.FieldByName(m.Config.DeduplicationKey); field.IsValid() {
			return fmt.Sprintf("%v", field.Interface())
		}
	}

	// Fallback to string representation
	return fmt.Sprintf("%v", item)
}

// isZeroValue checks if a value is considered "zero" (nil, empty, etc.)
func (m *FirstSuccessMerger[T]) isZeroValue(value T) bool {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	case reflect.Array, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return v.Complex() == 0
	default:
		// For structs and other types, check if it's the zero value
		return v.Interface() == reflect.Zero(v.Type()).Interface()
	}
}
