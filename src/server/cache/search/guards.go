package search

import "fmt"

// MustBeEnabled returns an error if the cache/search is disabled.
// Use this for operations that require the search to be enabled.
func MustBeEnabled(enabled bool) error {
	if !enabled {
		return fmt.Errorf("cache disabled or SCIP storage unavailable")
	}
	return nil
}
