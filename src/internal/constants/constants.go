package constants

import (
	"os"
	"runtime"
	"time"

	"lsp-gateway/src/internal/registry"
)

// Timeout constants for LSP operations
const (
	// Request timeouts by language
	DefaultRequestTimeout = 30 * time.Second
	JavaRequestTimeout    = 90 * time.Second
	PythonRequestTimeout  = 30 * time.Second
	GoTSRequestTimeout    = 15 * time.Second
	WriteTimeout          = 10 * time.Second

	// Initialize timeouts by language
	DefaultInitializeTimeout    = 30 * time.Second
	JavaInitializeTimeout       = 90 * time.Second
	PythonInitializeTimeout     = 30 * time.Second
	GoInitializeTimeout         = 15 * time.Second
	TypeScriptInitializeTimeout = 30 * time.Second

	// Process management timeouts
	ProcessShutdownTimeout = 5 * time.Second
	ProcessStartTimeout    = 30 * time.Second
)

// Cache configuration constants
const (
	// Default cache settings
	DefaultCacheMemoryMB      = 512
	DefaultCacheTTLHours      = 24
	DefaultHealthCheckMinutes = 5
	DefaultEvictionPolicy     = "lru"

	// MCP server optimized cache settings
	MCPCacheMemoryMB        = 512
	MCPCacheTTLHours        = 1
	MCPHealthCheckMinutes   = 2
	MCPInitialIndexingDelay = 2 * time.Second
	MCPIndexingTimeout      = 60 * time.Second
	MCPMaxIndexFiles        = 50
	MCPMaxSymbolResults     = 5000

	// MaxResults constants for queries
	DefaultMaxResults            = 100
	DefaultMaxResultsSymbols     = 100
	DefaultMaxResultsReferences  = 100
	DefaultMaxResultsDefinitions = 100
)

// File and directory constants
const (
	// Maximum directory walk depth
	MaxDirectoryWalkDepth = 3

	// Debounce delay for file watching
	FileWatchDebounceDelay = 500 * time.Millisecond
)

// Protocol constants
const (
	// LSPResponseBufferSize is the buffer size for reading LSP responses
	// Use 1MB to handle large responses like workspace/symbol results
	LSPResponseBufferSize = 1024 * 1024 // 1MB
)

// Supported file extensions by language
var SupportedExtensions map[string][]string

// init initializes SupportedExtensions from registry on package load
func init() {
	SupportedExtensions = registry.GetSupportedExtensions()
}

// getSupportedExtensions returns the SupportedExtensions map
func getSupportedExtensions() map[string][]string {
	return SupportedExtensions
}

// Directories to skip during file scanning
var SkipDirectories = map[string]bool{
	".":            true,
	"node_modules": true,
	"vendor":       true,
	"build":        true,
	"dist":         true,
	"target":       true,
	"__pycache__":  true,
	".git":         true,
	".svn":         true,
	".hg":          true,
	".idea":        true,
	".vscode":      true,
}

// GetAllSupportedExtensions returns all supported file extensions
func GetAllSupportedExtensions() []string {
	supportedExts := getSupportedExtensions()
	extensions := make([]string, 0)
	seen := make(map[string]bool)
	for _, exts := range supportedExts {
		for _, ext := range exts {
			if !seen[ext] {
				extensions = append(extensions, ext)
				seen[ext] = true
			}
		}
	}
	return extensions
}

// isCI detects if running in CI environment
func isCI() bool {
	return os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true"
}

// isWindowsCI detects if running in Windows CI environment
func isWindowsCI() bool {
	return runtime.GOOS == "windows" && isCI()
}

// GetRequestTimeout returns language-specific timeout for LSP requests
func GetRequestTimeout(language string) time.Duration {
	langInfo, exists := registry.GetLanguageByName(language)
	var baseTimeout time.Duration
	if exists {
		requestTimeout, _ := langInfo.GetTimeouts()
		baseTimeout = requestTimeout
	} else {
		baseTimeout = DefaultRequestTimeout
	}

	// Apply CI environment multipliers
	if isWindowsCI() {
		// Windows CI is significantly slower, especially for Java
		if language == "java" {
			return baseTimeout * 3 // Triple timeout for Java on Windows CI (270s)
		}
		return time.Duration(float64(baseTimeout) * 1.5) // 50% more for other languages
	} else if isCI() {
		// Non-Windows CI environments also need more time
		return time.Duration(float64(baseTimeout) * 1.2) // 20% more time
	}

	return baseTimeout
}

// GetInitializeTimeout returns language-specific timeout for initialize requests
func GetInitializeTimeout(language string) time.Duration {
	langInfo, exists := registry.GetLanguageByName(language)
	var baseTimeout time.Duration
	if exists {
		_, initializeTimeout := langInfo.GetTimeouts()
		baseTimeout = initializeTimeout
	} else {
		baseTimeout = DefaultInitializeTimeout
	}

	// Apply CI environment multipliers
	if isWindowsCI() {
		// Windows CI is significantly slower
		if language == "java" {
			return baseTimeout * 3 // Triple timeout for Java on Windows CI (270s)
		}
		return time.Duration(float64(baseTimeout) * 1.5) // 50% more for other languages
	} else if isCI() {
		// Non-Windows CI environments also need more time
		return time.Duration(float64(baseTimeout) * 1.2) // 20% more time
	}

	return baseTimeout
}
