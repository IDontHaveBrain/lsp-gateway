package testutils

import ()

// SubProjectInfo represents information about a sub-project in a multi-project workspace
type SubProjectInfo struct {
	Language    string
	ProjectPath string
	RootMarkers []string
	TestFiles   []string
	LSPConfig   map[string]string
}

// Note: LSP type aliases removed to avoid internal package dependencies
// Use map[string]interface{} for raw JSON-RPC testing instead
