package constants

import "time"

// Timeout constants for LSP operations
const (
	// Request timeouts by language
	DefaultRequestTimeout = 30 * time.Second
	JavaRequestTimeout    = 60 * time.Second
	PythonRequestTimeout  = 30 * time.Second
	GoTSRequestTimeout    = 15 * time.Second
	WriteTimeout          = 10 * time.Second

	// Initialize timeouts by language
	DefaultInitializeTimeout = 15 * time.Second
	JavaInitializeTimeout    = 60 * time.Second
	PythonInitializeTimeout  = 30 * time.Second

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
)

// File and directory constants
const (
	// Maximum directory walk depth
	MaxDirectoryWalkDepth = 3

	// Debounce delay for file watching
	FileWatchDebounceDelay = 500 * time.Millisecond
)

// Supported file extensions by language
var SupportedExtensions = map[string][]string{
	"go":         {".go"},
	"python":     {".py", ".pyi"},
	"javascript": {".js", ".jsx", ".mjs"},
	"typescript": {".ts", ".tsx", ".d.ts"},
	"java":       {".java"},
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
	extensions := make([]string, 0)
	seen := make(map[string]bool)
	for _, exts := range SupportedExtensions {
		for _, ext := range exts {
			if !seen[ext] {
				extensions = append(extensions, ext)
				seen[ext] = true
			}
		}
	}
	return extensions
}

// GetRequestTimeout returns language-specific timeout for LSP requests
func GetRequestTimeout(language string) time.Duration {
	switch language {
	case "java":
		return JavaRequestTimeout
	case "python":
		return PythonRequestTimeout
	case "go", "javascript", "typescript":
		return GoTSRequestTimeout
	default:
		return DefaultRequestTimeout
	}
}

// GetInitializeTimeout returns language-specific timeout for initialize requests
func GetInitializeTimeout(language string) time.Duration {
	switch language {
	case "java":
		return JavaInitializeTimeout
	case "python":
		return PythonInitializeTimeout
	default:
		return DefaultInitializeTimeout
	}
}
