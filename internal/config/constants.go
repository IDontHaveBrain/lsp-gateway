package config

const (
	// Test configuration constants
	TEST_BASIC_CONFIG = `port: 8080
servers:
  - name: go-lsp
    command: gopls
    languages: [go]
    transport: stdio`

	TEST_EMPTY_SERVERS_CONFIG = `port: 8080
servers: []`

	TEST_BASIC_GO_CONFIG = `port: 8080
servers:
  - name: go-lsp
    languages: [go]
    command: gopls
    transport: stdio`

	// Error type constants
	APPLICATION_ERROR_TYPE = "application_error"

	// Platform constants
	PLATFORM_WINDOWS = "windows"
)
