package gateway

const (
	HEADER_CONTENT_TYPE = "Content-Type"

	LSP_METHOD_DEFINITION       = "textDocument/definition"
	LSP_METHOD_REFERENCES       = "textDocument/references"
	LSP_METHOD_DOCUMENT_SYMBOL  = "textDocument/documentSymbol"
	LSP_METHOD_WORKSPACE_SYMBOL = "workspace/symbol"
	LSP_METHOD_HOVER            = "textDocument/hover"

	// Server status constants
	STATUS_FAILED = "failed"

	// Error types
	APPLICATION_ERROR_TYPE = "application_error"

	// Language constants
	LANG_JAVASCRIPT = "javascript"
	LANG_PYTHON     = "python"
	LANG_JAVA       = "java"
	LANG_RUST       = "rust"
	LANG_TYPESCRIPT = "typescript"

	// Project type constants
	PROJECT_TYPE_SINGLE_LANGUAGE = "single-language"
	PROJECT_TYPE_GENERIC         = "generic"

	// Routing strategy constants
	ROUTING_STRATEGY_BROADCAST_AGGREGATE = "broadcast_aggregate"
)
