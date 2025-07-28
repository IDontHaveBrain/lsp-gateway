package common

const (
	// Configuration errors
	ERROR_CONFIG_NOT_FOUND   = "Configuration file not found: %s"
	ERROR_CONFIG_LOAD_FAILED = "Failed to load configuration: %v"
	SUGGESTION_CREATE_CONFIG = "Create config file: lspg config generate --output %s"

	// Format strings
	FORMAT_PORT_NUMBER = ":%d"

	// Common test values
	TEST_VALUE     = "value"
	TEST_UNKNOWN   = "unknown"
	TEST_NULL      = "null"
	TEST_TEXT_TYPE = "text"

	// Common paths
	TEMP_DIR_UNIX    = "/tmp"
	TEMP_DIR_WINDOWS = "C:\\Windows\\Temp"
	USER_DIR_WINDOWS = "C:\\Users\\testuser"

	// Common status values
	STATUS_NOT_IMPLEMENTED = "not implemented"
)
