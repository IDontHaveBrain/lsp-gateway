package registry

import (
	"fmt"
)

// LanguageInfo contains comprehensive information about a supported language
type LanguageInfo struct {
	Name              string   // Language name (go, python, javascript, typescript, java)
	Extensions        []string // File extensions for this language
	DefaultCommand    string   // Default LSP server command
	DefaultArgs       []string // Default arguments for the LSP server
	InstallerRequired bool     // Whether installer is required for this language
}

// Global language registry containing all supported languages
var languageRegistry = map[string]LanguageInfo{
	"go": {
		Name:              "go",
		Extensions:        []string{".go"},
		DefaultCommand:    "gopls",
		DefaultArgs:       []string{"serve"},
		InstallerRequired: true,
	},
	"python": {
		Name:              "python",
		Extensions:        []string{".py", ".pyi"},
		DefaultCommand:    "pylsp",
		DefaultArgs:       []string{},
		InstallerRequired: true,
	},
	"javascript": {
		Name:              "javascript",
		Extensions:        []string{".js", ".jsx", ".mjs"},
		DefaultCommand:    "typescript-language-server",
		DefaultArgs:       []string{"--stdio"},
		InstallerRequired: true,
	},
	"typescript": {
		Name:              "typescript",
		Extensions:        []string{".ts", ".tsx", ".d.ts"},
		DefaultCommand:    "typescript-language-server",
		DefaultArgs:       []string{"--stdio"},
		InstallerRequired: true,
	},
	"java": {
		Name:              "java",
		Extensions:        []string{".java"},
		DefaultCommand:    "jdtls",
		DefaultArgs:       []string{},
		InstallerRequired: true,
	},
	"rust": {
		Name:              "rust",
		Extensions:        []string{".rs"},
		DefaultCommand:    "rust-analyzer",
		DefaultArgs:       []string{},
		InstallerRequired: true,
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
}

// Allowed commands for security validation
var allowedCommands = []string{
	// LSP servers
	"gopls",
	"pyls",
	"typescript-language-server",
	"tsserver",
	"pylsp",
	"jdtls",
	"jdtls.bat",
	"jdtls.py",
	"rust-analyzer",
	"rust-analyzer.exe",
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
