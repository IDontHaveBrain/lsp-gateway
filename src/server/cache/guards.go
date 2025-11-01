package cache

import "fmt"

// MustBeEnabled returns an error if the cache is disabled.
// Use this for operations that require the cache to be enabled.
func (m *SCIPCacheManager) MustBeEnabled() error {
	if m.isDisabled() {
		return fmt.Errorf("cache disabled or SCIP storage unavailable")
	}
	return nil
}
