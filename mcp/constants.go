package mcp

const (
	LOG_FIELD_OPERATION  = "operation"
	LOG_FIELD_REQUEST_ID = "request_id"

	LOG_FORMAT_PARENTHESES = "(%s)"

	ERROR_CALL_INITIALIZE_FIRST = "Call initialize first"
	ERROR_INVALID_JSON_RPC      = "invalid JSON-RPC version: %s"

	HTTP_HEADER_CONTENT_TYPE = "Content-Type"

	LSP_METHOD_WORKSPACE_SYMBOL         = "workspace/symbol"
	LSP_METHOD_TEXT_DOCUMENT_DEFINITION = "textDocument/definition"
	LSP_METHOD_TEXT_DOCUMENT_REFERENCES = "textDocument/references"
	LSP_METHOD_TEXT_DOCUMENT_HOVER      = "textDocument/hover"
	LSP_METHOD_TEXT_DOCUMENT_SYMBOLS    = "textDocument/documentSymbol"

	JSON_RPC_ENDPOINT = "/jsonrpc"

	// Language constants
	LANG_PYTHON     = "python"
	LANG_JAVASCRIPT = "javascript"
	LANG_TYPESCRIPT = "typescript"
	LANG_JAVA       = "java"
)
