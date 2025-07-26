package config

import "time"

const (
	// Language constants
	LANG_PYTHON     = "python"
	LANG_JAVASCRIPT = "javascript"
	LANG_TYPESCRIPT = "typescript"
	LANG_JAVA       = "java"
	LANG_GO         = "go"

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

	// Multi-server configuration constants
	// Selection strategy constants
	SELECTION_STRATEGY_PERFORMANCE  = "performance"
	SELECTION_STRATEGY_FEATURE      = "feature"
	SELECTION_STRATEGY_LOAD_BALANCE = "load_balance"
	SELECTION_STRATEGY_RANDOM       = "random"

	// Load balancing strategy constants
	LOAD_BALANCE_ROUND_ROBIN       = "round_robin"
	LOAD_BALANCE_LEAST_CONNECTIONS = "least_connections"
	LOAD_BALANCE_RESPONSE_TIME     = "response_time"
	LOAD_BALANCE_RESOURCE_USAGE    = "resource_usage"

	// Default values
	DEFAULT_CONCURRENT_LIMIT                = 3
	DEFAULT_MAX_CONCURRENT_SERVERS_PER_LANG = 3
	DEFAULT_HEALTH_THRESHOLD                = 0.8
	DEFAULT_MAX_RETRIES                     = 3
	DEFAULT_MAX_MEMORY_MB                   = 1024
	DEFAULT_MAX_CONCURRENT_REQUESTS         = 50
	DEFAULT_MAX_PROCESSES                   = 5
	DEFAULT_REQUEST_TIMEOUT_SECONDS         = 30

	// Limits
	MAX_CONCURRENT_SERVERS_LIMIT   = 10
	MAX_CONCURRENT_LIMIT           = 20
	MAX_RETRIES_LIMIT              = 10
	MAX_MEMORY_MB_LIMIT            = 32768 // 32 GB
	MAX_CONCURRENT_REQUESTS_LIMIT  = 1000
	MAX_PROCESSES_LIMIT            = 100
	MAX_REQUEST_TIMEOUT_SECONDS    = 3600 // 1 hour
	MAX_PRIORITY                   = 100
	MAX_WEIGHT                     = 10.0
	MAX_SERVER_CONCURRENT_REQUESTS = 500

	// Minimum values
	MIN_MEMORY_MB               = 64
	MIN_REQUEST_TIMEOUT_SECONDS = 1

	// Timeout string constants
	DEFAULT_TIMEOUT_15S = "15s"
	DEFAULT_TIMEOUT_30S = "30s"
	DEFAULT_TIMEOUT_60S = "60s"
)

// Time-based constants
const (
	DefaultConnectionTimeout = 10 * time.Second
	DefaultGlobalTimeout     = 60 * time.Second
	DefaultMethodTimeout     = 30 * time.Second
)
