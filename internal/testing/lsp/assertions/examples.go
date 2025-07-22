package assertions

// Helper functions for creating pointers to basic types

// Helper function for creating int pointers
func intPtr(i int) *int {
	return &i
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
