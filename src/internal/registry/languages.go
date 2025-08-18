package registry

import (
	"fmt"
	"time"

	"lsp-gateway/src/internal/types"
)

// ErrorPattern defines an error pattern with associated suggestion message
type ErrorPattern struct {
	Pattern string // Error pattern to match (can be regex)
	Message string // Suggestion message for this error
}

// LanguageInfo contains comprehensive information about a supported language
type LanguageInfo struct {
	Name              string   // Language name (go, python, javascript, typescript, java)
	Extensions        []string // File extensions for this language
	DefaultCommand    string   // Default LSP server command
	DefaultArgs       []string // Default arguments for the LSP server
	InstallerRequired bool     // Whether installer is required for this language

	// Configuration fields
	InitializationOptions map[string]interface{} // LSP initialization options
	RequestTimeout        time.Duration          // Request timeout duration
	InitializeTimeout     time.Duration          // Initialize timeout duration
	EnvironmentVars       map[string]string      // Environment variables to set
	ErrorPatterns         []ErrorPattern         // Error patterns and suggestions
}

// Global language registry containing all supported languages
var languageRegistry = map[string]LanguageInfo{
	"go": {
		Name:              "go",
		Extensions:        []string{".go"},
		DefaultCommand:    "gopls",
		DefaultArgs:       []string{"serve"},
		InstallerRequired: true,
		InitializationOptions: map[string]interface{}{
			"usePlaceholders":    false,
			"completeUnimported": true,
		},
		RequestTimeout:    15 * time.Second,
		InitializeTimeout: 15 * time.Second,
		EnvironmentVars:   map[string]string{},
		ErrorPatterns:     []ErrorPattern{},
	},
	"python": {
		Name:              "python",
		Extensions:        []string{".py", ".pyi"},
		DefaultCommand:    "jedi-language-server",
		DefaultArgs:       []string{},
		InstallerRequired: true,
		InitializationOptions: map[string]interface{}{
			"usePlaceholders":    false,
			"completeUnimported": true,
		},
		RequestTimeout:    30 * time.Second,
		InitializeTimeout: 30 * time.Second,
		EnvironmentVars:   map[string]string{},
		ErrorPatterns: []ErrorPattern{
			{
				Pattern: types.MethodWorkspaceSymbol,
				Message: "Ensure 'jedi-language-server' is installed via 'pip install jedi-language-server'",
			},
			{
				Pattern: "semanticTokens",
				Message: "Ensure 'jedi-language-server' is installed via 'pip install jedi-language-server'",
			},
		},
	},
	"javascript": {
		Name:              "javascript",
		Extensions:        []string{".js", ".jsx", ".mjs"},
		DefaultCommand:    "typescript-language-server",
		DefaultArgs:       []string{"--stdio"},
		InstallerRequired: true,
		InitializationOptions: map[string]interface{}{
			"usePlaceholders":    false,
			"completeUnimported": true,
		},
		RequestTimeout:    15 * time.Second,
		InitializeTimeout: 30 * time.Second,
		EnvironmentVars:   map[string]string{},
		ErrorPatterns:     []ErrorPattern{},
	},
	"typescript": {
		Name:              "typescript",
		Extensions:        []string{".ts", ".tsx", ".d.ts"},
		DefaultCommand:    "typescript-language-server",
		DefaultArgs:       []string{"--stdio"},
		InstallerRequired: true,
		InitializationOptions: map[string]interface{}{
			"usePlaceholders":    false,
			"completeUnimported": true,
		},
		RequestTimeout:    15 * time.Second,
		InitializeTimeout: 30 * time.Second,
		EnvironmentVars:   map[string]string{},
		ErrorPatterns:     []ErrorPattern{},
	},
	"java": {
		Name:              "java",
		Extensions:        []string{".java"},
		DefaultCommand:    "jdtls",
		DefaultArgs:       []string{},
		InstallerRequired: true,
		InitializationOptions: map[string]interface{}{
			"usePlaceholders":    false,
			"completeUnimported": true,
		},
		RequestTimeout:    90 * time.Second,
		InitializeTimeout: 90 * time.Second,
		EnvironmentVars:   map[string]string{},
		ErrorPatterns:     []ErrorPattern{},
	},
	"rust": {
		Name:              "rust",
		Extensions:        []string{".rs"},
		DefaultCommand:    "rust-analyzer",
		DefaultArgs:       []string{},
		InstallerRequired: true,
		InitializationOptions: map[string]interface{}{
			"cargo": map[string]interface{}{
				"features":          []string{},
				"allFeatures":       false,
				"noDefaultFeatures": false,
			},
			"checkOnSave": map[string]interface{}{
				"enable":  true,
				"command": "check",
			},
			"procMacro": map[string]interface{}{
				"enable": true,
			},
			"inlayHints": map[string]interface{}{
				"enable": true,
			},
		},
		RequestTimeout:    15 * time.Second,
		InitializeTimeout: 15 * time.Second,
		EnvironmentVars: map[string]string{
			"CARGO_MANIFEST_DIR": "${workingDir}",
		},
		ErrorPatterns: []ErrorPattern{},
	},
	"csharp": {
		Name:              "csharp",
		Extensions:        []string{".cs"},
		DefaultCommand:    "omnisharp",
		DefaultArgs:       []string{"-lsp"},
		InstallerRequired: true,
		InitializationOptions: map[string]interface{}{
			"usePlaceholders":    false,
			"completeUnimported": true,
		},
		RequestTimeout:    30 * time.Second,
		InitializeTimeout: 45 * time.Second,
		EnvironmentVars: map[string]string{
			"DOTNET_CLI_TELEMETRY_OPTOUT":       "1",
			"DOTNET_SKIP_FIRST_TIME_EXPERIENCE": "1",
			"DOTNET_NOLOGO":                     "1",
		},
		ErrorPatterns: []ErrorPattern{},
	},
}

// Extension to language mapping for efficient lookups
var extensionToLanguage = map[string]string{
	".go":   "go",
	".py":   "python",
	".pyi":  "python",
	".js":   "javascript",
	".jsx":  "javascript",
	".mjs":  "javascript",
	".ts":   "typescript",
	".tsx":  "typescript",
	".d.ts": "typescript",
	".java": "java",
	".rs":   "rust",
	".cs":   "csharp",
}

// Allowed commands for security validation
var allowedCommands = []string{
	// LSP servers
	"gopls",
	"pyls",
	"typescript-language-server",
	"tsserver",
	"pylsp",
	"jedi-language-server",
	"jdtls",
	"jdtls.bat",
	"jdtls.py",
	"rust-analyzer",
	"rust-analyzer.exe",
	"rust-analyzer.cmd",
	"omnisharp",
	"omnisharp.exe",
	"OmniSharp",
	"OmniSharp.exe",
	// Runtime tools
	"java",
	"java.exe",
	"node",
	"node.exe",
	"python",
	"python.exe",
	"python3",
	"python3.exe",
	"rustup",
	"rustup.exe",
	"cargo",
	"cargo.exe",
	"rustc",
	"rustc.exe",
	"dotnet",
	"dotnet.exe",
	// Installation tools
	"go",
	"go.exe",
	"npm",
	"npm.cmd",
	"npx",
	"npx.cmd",
	"pip",
	"pip.exe",
	"pip3",
	"pip3.exe",
	"pipx",
	"pipx.exe",
	"uv",
	"uv.exe",
	"curl",
	"curl.exe",
	"wget",
	"wget.exe",
	"tar",
	"tar.exe",
	"unzip",
	"unzip.exe",
	"apt-get",
	"brew",
	"echo",
}

// GetSupportedLanguages returns all supported language information
func GetSupportedLanguages() []LanguageInfo {
	languages := make([]LanguageInfo, 0, len(languageRegistry))
	for _, lang := range languageRegistry {
		languages = append(languages, lang)
	}
	return languages
}

// GetLanguageByName returns language information by name
func GetLanguageByName(name string) (*LanguageInfo, bool) {
	lang, exists := languageRegistry[name]
	if !exists {
		return nil, false
	}
	return &lang, true
}

// GetLanguageByExtension returns language information by file extension
func GetLanguageByExtension(ext string) (*LanguageInfo, bool) {
	langName, exists := extensionToLanguage[ext]
	if !exists {
		return nil, false
	}

	lang, exists := languageRegistry[langName]
	if !exists {
		return nil, false
	}
	return &lang, true
}

// GetSupportedExtensions returns file extensions map for backwards compatibility
func GetSupportedExtensions() map[string][]string {
	extensions := make(map[string][]string)
	for name, lang := range languageRegistry {
		extensions[name] = make([]string, len(lang.Extensions))
		copy(extensions[name], lang.Extensions)
	}
	return extensions
}

// GetAllowedCommands returns all allowed commands for security validation
func GetAllowedCommands() []string {
	commands := make([]string, len(allowedCommands))
	copy(commands, allowedCommands)
	return commands
}

// GetLanguageNames returns list of supported language names for backwards compatibility
func GetLanguageNames() []string {
	names := make([]string, 0, len(languageRegistry))
	for name := range languageRegistry {
		names = append(names, name)
	}
	return names
}

// IsLanguageSupported checks if a language is supported
func IsLanguageSupported(name string) bool {
	_, exists := languageRegistry[name]
	return exists
}

// IsExtensionSupported checks if a file extension is supported
func IsExtensionSupported(ext string) bool {
	_, exists := extensionToLanguage[ext]
	return exists
}

// GetAllExtensions returns all supported file extensions
func GetAllExtensions() []string {
	extensions := make([]string, 0, len(extensionToLanguage))
	seen := make(map[string]bool)

	for ext := range extensionToLanguage {
		if !seen[ext] {
			extensions = append(extensions, ext)
			seen[ext] = true
		}
	}
	return extensions
}

// ValidateLanguage validates if the language is supported and returns error if not
func ValidateLanguage(name string) error {
	if !IsLanguageSupported(name) {
		return fmt.Errorf("unsupported language: %s (supported: %v)", name, GetLanguageNames())
	}
	return nil
}

// ValidateExtension validates if the extension is supported and returns error if not
func ValidateExtension(ext string) error {
	if !IsExtensionSupported(ext) {
		return fmt.Errorf("unsupported extension: %s (supported: %v)", ext, GetAllExtensions())
	}
	return nil
}

// Helper methods for accessing language configurations

// GetInitOptions returns the initialization options for this language
func (l *LanguageInfo) GetInitOptions() map[string]interface{} {
	if l.InitializationOptions == nil {
		return map[string]interface{}{}
	}
	// Return a copy to prevent modification
	result := make(map[string]interface{})
	for k, v := range l.InitializationOptions {
		result[k] = v
	}
	return result
}

// GetTimeouts returns the request and initialize timeout durations for this language
func (l *LanguageInfo) GetTimeouts() (requestTimeout time.Duration, initializeTimeout time.Duration) {
	return l.RequestTimeout, l.InitializeTimeout
}

// GetEnvironment returns the environment variables for this language
func (l *LanguageInfo) GetEnvironment() map[string]string {
	if l.EnvironmentVars == nil {
		return map[string]string{}
	}
	// Return a copy to prevent modification
	result := make(map[string]string)
	for k, v := range l.EnvironmentVars {
		result[k] = v
	}
	return result
}

// GetEnvironmentWithWorkingDir returns environment variables with workingDir substituted
func (l *LanguageInfo) GetEnvironmentWithWorkingDir(workingDir string) map[string]string {
	envVars := l.GetEnvironment()
	if workingDir == "" {
		return envVars
	}

	// Substitute ${workingDir} template
	result := make(map[string]string)
	for k, v := range envVars {
		if v == "${workingDir}" {
			result[k] = workingDir
		} else {
			result[k] = v
		}
	}
	return result
}

// GetErrorPatterns returns the error patterns for this language
func (l *LanguageInfo) GetErrorPatterns() []ErrorPattern {
	if l.ErrorPatterns == nil {
		return []ErrorPattern{}
	}
	// Return a copy to prevent modification
	result := make([]ErrorPattern, len(l.ErrorPatterns))
	copy(result, l.ErrorPatterns)
	return result
}
