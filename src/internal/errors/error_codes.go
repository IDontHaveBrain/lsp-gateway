package errors

// Standard JSON-RPC error codes as defined in RFC 7309
const (
	// Pre-defined JSON-RPC errors
	ParseError     = -32700 // Invalid JSON was received by the server
	InvalidRequest = -32600 // The JSON sent is not a valid Request object
	MethodNotFound = -32601 // The method does not exist / is not available
	InvalidParams  = -32602 // Invalid method parameter(s)
	InternalError  = -32603 // Internal JSON-RPC error
)

// LSP-specific error codes as defined in the LSP specification
const (
	// LSP error codes (range: -32000 to -32099)
	ServerNotInitialized = -32002 // Server not initialized
	UnknownErrorCode     = -32001 // Unknown error code
	RequestCancelled     = -32800 // Request was cancelled
	ContentModified      = -32801 // Content was modified
	RequestFailed        = -32803 // Request failed with unrecoverable error
)

// LSP Gateway custom error codes (range: -33000 to -33099)
const (
	// Connection and process errors
	ConnectionFailure   = -33001 // Failed to connect to LSP server
	ProcessStartFailure = -33002 // Failed to start LSP server process
	ProcessStopFailure  = -33003 // Failed to stop LSP server process
	CommunicationError  = -33004 // Communication error with LSP server

	// Timeout errors
	InitializationTimeout = -33010 // LSP server initialization timeout
	OperationTimeout      = -33011 // LSP operation timeout
	ShutdownTimeout       = -33012 // LSP server shutdown timeout

	// Validation errors
	InvalidURI           = -33020 // Invalid URI format
	InvalidPosition      = -33021 // Invalid position (line/character)
	InvalidTextDocument  = -33022 // Invalid text document identifier
	MissingParameter     = -33023 // Required parameter missing
	InvalidParameterType = -33024 // Parameter has invalid type

	// Feature and capability errors
	UnsupportedMethod   = -33030 // Method not supported by server
	UnsupportedLanguage = -33031 // Language not supported
	CapabilityNotFound  = -33032 // Required capability not available
	FeatureDisabled     = -33033 // Feature is disabled

	// Cache and indexing errors
	CacheError    = -33040 // Cache operation error
	IndexingError = -33041 // Indexing operation error
	SCIPError     = -33042 // SCIP protocol error

	// Aggregation errors
	PartialFailure   = -33050 // Partial failure in multi-server operation
	AggregationError = -33051 // Error during result aggregation
	NoValidResults   = -33052 // No valid results from any server

	// Configuration errors
	ConfigurationError = -33060 // Configuration error
	InstallationError  = -33061 // LSP server installation error
	ServerNotFound     = -33062 // LSP server executable not found
)

// Error code categories for classification and handling
const (
	CategoryJSONRPC     = "jsonrpc"     // Standard JSON-RPC errors
	CategoryLSP         = "lsp"         // LSP specification errors
	CategoryConnection  = "connection"  // Connection and process errors
	CategoryTimeout     = "timeout"     // Timeout-related errors
	CategoryValidation  = "validation"  // Parameter validation errors
	CategoryFeature     = "feature"     // Feature and capability errors
	CategoryCache       = "cache"       // Cache and indexing errors
	CategoryAggregation = "aggregation" // Multi-server operation errors
	CategoryConfig      = "config"      // Configuration errors
)

// GetErrorCodeCategory returns the category for a given error code
func GetErrorCodeCategory(code int) string {
	switch {
	case code >= -32700 && code <= -32600:
		// Standard JSON-RPC errors including ParseError (-32700)
		return CategoryJSONRPC
	case code >= -32099 && code <= -32000:
		// JSON-RPC reserved for server-defined errors
		return CategoryJSONRPC
	case code >= -32899 && code <= -32800:
		// LSP specification errors
		return CategoryLSP
	case code >= -33009 && code <= -33001:
		return CategoryConnection
	case code >= -33019 && code <= -33010:
		return CategoryTimeout
	case code >= -33029 && code <= -33020:
		return CategoryValidation
	case code >= -33039 && code <= -33030:
		return CategoryFeature
	case code >= -33049 && code <= -33040:
		return CategoryCache
	case code >= -33059 && code <= -33050:
		return CategoryAggregation
	case code >= -33069 && code <= -33060:
		return CategoryConfig
	default:
		return "unknown"
	}
}

// GetErrorCodeMessage returns the standard message for a given error code
func GetErrorCodeMessage(code int) string {
	switch code {
	// Standard JSON-RPC errors
	case ParseError:
		return "Parse error"
	case InvalidRequest:
		return "Invalid Request"
	case MethodNotFound:
		return "Method not found"
	case InvalidParams:
		return "Invalid params"
	case InternalError:
		return "Internal error"

	// LSP errors
	case ServerNotInitialized:
		return "Server not initialized"
	case UnknownErrorCode:
		return "Unknown error code"
	case RequestCancelled:
		return "Request cancelled"
	case ContentModified:
		return "Content modified"
	case RequestFailed:
		return "Request failed"

	// Connection errors
	case ConnectionFailure:
		return "Connection failure"
	case ProcessStartFailure:
		return "Process start failure"
	case ProcessStopFailure:
		return "Process stop failure"
	case CommunicationError:
		return "Communication error"

	// Timeout errors
	case InitializationTimeout:
		return "Initialization timeout"
	case OperationTimeout:
		return "Operation timeout"
	case ShutdownTimeout:
		return "Shutdown timeout"

	// Validation errors
	case InvalidURI:
		return "Invalid URI"
	case InvalidPosition:
		return "Invalid position"
	case InvalidTextDocument:
		return "Invalid text document"
	case MissingParameter:
		return "Missing parameter"
	case InvalidParameterType:
		return "Invalid parameter type"

	// Feature errors
	case UnsupportedMethod:
		return "Unsupported method"
	case UnsupportedLanguage:
		return "Unsupported language"
	case CapabilityNotFound:
		return "Capability not found"
	case FeatureDisabled:
		return "Feature disabled"

	// Cache errors
	case CacheError:
		return "Cache error"
	case IndexingError:
		return "Indexing error"
	case SCIPError:
		return "SCIP error"

	// Aggregation errors
	case PartialFailure:
		return "Partial failure"
	case AggregationError:
		return "Aggregation error"
	case NoValidResults:
		return "No valid results"

	// Configuration errors
	case ConfigurationError:
		return "Configuration error"
	case InstallationError:
		return "Installation error"
	case ServerNotFound:
		return "Server not found"

	default:
		return "Unknown error"
	}
}

// IsRetryableError determines if an error code represents a retryable condition
func IsRetryableError(code int) bool {
	switch code {
	case ConnectionFailure, CommunicationError, OperationTimeout:
		return true
	case InitializationTimeout, ProcessStartFailure:
		return true // Can retry with different parameters
	case CacheError, IndexingError:
		return true // Cache operations can be retried
	default:
		return false
	}
}

// IsCriticalError determines if an error code represents a critical system error
func IsCriticalError(code int) bool {
	switch code {
	case ProcessStartFailure, ProcessStopFailure:
		return true
	case ConfigurationError, InstallationError:
		return true
	case ServerNotFound:
		return true
	default:
		return false
	}
}
