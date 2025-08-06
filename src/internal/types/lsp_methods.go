package types

// LSP protocol lifecycle methods
const (
	// MethodInitialize is sent as the first request from client to server
	MethodInitialize = "initialize"
	// MethodInitialized is sent from client to server after the initialize response
	MethodInitialized = "initialized"
	// MethodShutdown is sent from client to server to shutdown the server
	MethodShutdown = "shutdown"
	// MethodExit is sent from client to server to exit the server process
	MethodExit = "exit"
)

// LSP document synchronization methods
const (
	// MethodTextDocumentDidOpen is sent when a document is opened
	MethodTextDocumentDidOpen = "textDocument/didOpen"
)

// LSP language feature methods
const (
	// MethodTextDocumentDefinition provides go-to-definition functionality
	MethodTextDocumentDefinition = "textDocument/definition"
	// MethodTextDocumentReferences finds all references to a symbol
	MethodTextDocumentReferences = "textDocument/references"
	// MethodTextDocumentHover provides hover information for symbols
	MethodTextDocumentHover = "textDocument/hover"
	// MethodTextDocumentDocumentSymbol returns document symbols outline
	MethodTextDocumentDocumentSymbol = "textDocument/documentSymbol"
	// MethodTextDocumentCompletion provides auto-completion suggestions
	MethodTextDocumentCompletion = "textDocument/completion"
	// MethodWorkspaceSymbol provides workspace-wide symbol search
	MethodWorkspaceSymbol = "workspace/symbol"
)
