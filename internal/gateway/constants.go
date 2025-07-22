package gateway

const (
	ERROR_INVALID_REQUEST   = "Invalid request"
	ERROR_INTERNAL          = "Internal error"
	ERROR_SERVER_NOT_FOUND  = "server not found: %s"
	FORMAT_INVALID_JSON_RPC = "invalid JSON-RPC version: %s"

	HEADER_CONTENT_TYPE = "Content-Type"

	LSP_METHOD_DEFINITION       = "textDocument/definition"
	LSP_METHOD_REFERENCES       = "textDocument/references"
	LSP_METHOD_DOCUMENT_SYMBOL  = "textDocument/documentSymbol"
	LSP_METHOD_WORKSPACE_SYMBOL = "workspace/symbol"
	LSP_METHOD_HOVER            = "textDocument/hover"

	// Error types
	APPLICATION_ERROR_TYPE = "application_error"
)
