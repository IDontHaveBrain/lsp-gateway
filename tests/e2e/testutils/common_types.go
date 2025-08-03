package testutils

import (
	"lsp-gateway/internal/models/lsp"
)

// SubProjectInfo represents information about a sub-project in a multi-project workspace
type SubProjectInfo struct {
	Language    string
	ProjectPath string
	RootMarkers []string
	TestFiles   []string
	LSPConfig   map[string]string
}

// LSP Protocol type aliases for test compatibility
type Position = lsp.Position
type Range = lsp.Range
type Location = lsp.Location
type HoverResult = lsp.Hover
type DocumentSymbol = lsp.DocumentSymbol
type SymbolInformation = lsp.SymbolInformation
type SymbolKind = lsp.SymbolKind
type CompletionList = lsp.CompletionList
type CompletionItemKind = lsp.CompletionItemKind
