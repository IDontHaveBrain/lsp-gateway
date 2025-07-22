package transport

const (
	PROTOCOL_CONTENT_LENGTH        = "Content-Length:"
	PROTOCOL_CONTENT_LENGTH_PREFIX = "Content-Length: "
	PROTOCOL_HEADER_FORMAT         = "Content-Length: %d\r\n\r\n%s"
	HTTP_CONTENT_TYPE_JSON         = "application/json"

	ERROR_CLIENT_STOPPED                 = "client stopped"
	ERROR_CLIENT_NOT_ACTIVE              = "client not active"
	ERROR_TCP_CLIENT_NOT_ACTIVE          = "TCP client not active"
	ERROR_TCP_CONNECTION_NOT_ESTABLISHED = "TCP connection not established"
	ERROR_SEND_NOTIFICATION              = "failed to send notification: %w"
	ERROR_SEND_REQUEST                   = "failed to send request: %w"
	ERROR_WRITE_MESSAGE                  = "failed to write message: %w"
	ERROR_MARSHAL_MESSAGE                = "failed to marshal message: %w"
	ERROR_READ_MESSAGE_BODY              = "failed to read message body: %w"
	ERROR_INVALID_CONTENT_LENGTH         = "invalid Content-Length: %s"

	// LSP method constants
	LSP_METHOD_TEXT_DOCUMENT_DEFINITION = "textDocument/definition"
)
