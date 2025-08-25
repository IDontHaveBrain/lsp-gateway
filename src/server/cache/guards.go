package cache

import (
    "fmt"
    "time"
)

// WithEnabledGuard executes the provided function only if the cache is enabled.
// Returns the function result if cache is enabled, otherwise returns nil and no error.
func (m *SCIPCacheManager) WithEnabledGuard(fn func() (interface{}, error)) (interface{}, error) {
	if !m.enabled {
		return nil, nil
	}
	return fn()
}

// NOTE: Typed guard helpers are centralized in internal/common.WithEnabledGuard.
// Local typed variants removed to avoid duplication.

// MustBeEnabled returns an error if the cache is disabled.
// Use this for operations that require the cache to be enabled.
func (m *SCIPCacheManager) MustBeEnabled() error {
	if !m.enabled {
		return fmt.Errorf("cache disabled or SCIP storage unavailable")
	}
	return nil
}

// WithIndexResult executes the function if cache is enabled, otherwise returns a default IndexResult
// with cache_disabled metadata.
func (m *SCIPCacheManager) WithIndexResult(queryType string, fn func() (*IndexResult, error)) (*IndexResult, error) {
	if !m.enabled {
		return &IndexResult{
			Type:      queryType,
			Results:   []interface{}{},
			Metadata:  map[string]interface{}{"cache_disabled": true},
			Timestamp: time.Now(),
		}, nil
	}
	return fn()
}

// Removed: Enhanced/Symbol/Reference typed guard wrappers; search service guard handles disabled cases.

// WithSliceResult executes the function if cache is enabled, otherwise returns an empty slice.
// Useful for methods that return []interface{} when cache is disabled.
func (m *SCIPCacheManager) WithSliceResult(fn func() ([]interface{}, error)) ([]interface{}, error) {
	if !m.enabled {
		return []interface{}{}, nil
	}
	return fn()
}

// isEnabledAndStarted checks if cache is enabled and properly initialized
func (m *SCIPCacheManager) isEnabledAndStarted() bool {
	return m.enabled && m.started
}

// WithManagerGuard executes the function only if the cache manager is enabled and started.
// Returns nil values if the cache is not ready.
func (m *SCIPCacheManager) WithManagerGuard(fn func() (interface{}, bool, error)) (interface{}, bool, error) {
	if !m.isEnabledAndStarted() {
		return nil, false, nil
	}
	return fn()
}
