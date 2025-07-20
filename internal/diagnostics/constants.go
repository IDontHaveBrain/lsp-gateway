package diagnostics

const (
	CONFIG_FILE = "config.yaml"

	TROUBLESHOOT_SYNTAX            = "syntax"
	TROUBLESHOOT_CONNECTION        = "connection"
	TROUBLESHOOT_INCOMPATIBLE      = "incompatible"
	TROUBLESHOOT_ACCESS_DENIED     = "access denied"
	TROUBLESHOOT_COMMAND_NOT_FOUND = "command not found"
	TROUBLESHOOT_WRITE             = "write"

	ERROR_RUNTIME_DETECTION   = "runtime detection failed: %w"
	ERROR_VERIFICATION_FAILED = "Verification failed: %v"

	STATUS_ID_FIELD = "ID="

	SYSTEM_SERVERS = "servers"
)
