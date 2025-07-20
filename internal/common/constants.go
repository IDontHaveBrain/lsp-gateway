package common

const (
	FORMAT_LIST_ITEM    = "    - %s\n"
	FORMAT_KEY_VALUE    = "%s: %s\n"
	FORMAT_DETAILS      = "  Details:\n"
	FORMAT_DETAIL_VALUE = "    %s: %v\n"

	ERROR_CONFIG_VALIDATION = "Configuration validation failed: %v"
	ERROR_FILE_PERMISSIONS  = "Check file permissions: ls -la %s"
	ERROR_TABLE_ROW_WRITE   = "failed to write table row: %w"

	LABEL_POST_INSTALL = "post-install"
	LABEL_CRITICAL     = "critical"

	VERIFICATION_INSTALL_RUNTIME = "Install %s runtime"

	// Configuration errors
	ERROR_CONFIG_NOT_FOUND     = "Configuration file not found: %s"
	ERROR_CONFIG_LOAD_FAILED   = "Failed to load configuration: %v"
	SUGGESTION_CREATE_CONFIG   = "Create config file: lsp-gateway config generate --output %s"
	SUGGESTION_FIX_PERMISSIONS = "Fix permissions: chmod 644 %s"

	// Format strings
	FORMAT_PORT        = ":%d"
	FORMAT_LIST_INDENT = "  %s: %v\n"

	// Platform identifiers
	PLATFORM_DARWIN  = "darwin"
	PLATFORM_WINDOWS = "windows"
	PLATFORM_LINUX   = "linux"

	// Common format patterns
	FORMAT_DOUBLE_NEWLINE = "%s\n\n"
	FORMAT_PORT_NUMBER    = ":%d"

	// Server error patterns
	ERROR_SERVER_NOT_INSTALLED = "%s server not installed"
	TRY_INSTALL_SERVER_COMMAND = "Try: ./lsp-gateway install server %s"
)
