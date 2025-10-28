package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"lsp-gateway/tests/e2e/base"
	"lsp-gateway/tests/e2e/testutils"

	"github.com/stretchr/testify/suite"
)

type LanguageTestConfig struct {
	name        string
	displayName string
}

func getLanguageConfigs() []LanguageTestConfig {
	all := []LanguageTestConfig{
		{name: langGo, displayName: "Go"},
		{name: langPython, displayName: "Python"},
		{name: langJavaScript, displayName: "JavaScript"},
		{name: langTypeScript, displayName: "TypeScript"},
		{name: langJava, displayName: "Java"},
		{name: langRust, displayName: "Rust"},
		{name: langCSharp, displayName: "CSharp"}, // Always include C# in tests
		{name: langKotlin, displayName: "Kotlin"},
	}
	langs := os.Getenv("E2E_LANGS")
	if strings.TrimSpace(langs) == "" {
		// When no explicit filter is provided, only include languages with available servers
		filtered := make([]LanguageTestConfig, 0, len(all))
		for _, c := range all {
			if isLanguageAvailable(c.name) {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			return []LanguageTestConfig{}
		}
		return filtered
	}
	allow := map[string]bool{}
	for _, raw := range strings.Split(langs, ",") {
		k := strings.TrimSpace(strings.ToLower(raw))
		switch k {
		case "js":
			k = langJavaScript
		case "ts":
			k = langTypeScript
		}
		if k != "" {
			allow[k] = true
		}
	}
	var filtered []LanguageTestConfig
	for _, c := range all {
		if allow[strings.ToLower(c.name)] && isLanguageAvailable(c.name) {
			filtered = append(filtered, c)
		}
	}
	// If filter produced empty, fall back to all
	if len(filtered) == 0 {
		return all
	}
	return filtered
}

// isLanguageAvailable performs a fast PATH check for the required language server
func isLanguageAvailable(lang string) bool {
	has := func(names ...string) bool {
		for _, n := range names {
			if p, err := exec.LookPath(n); err == nil && p != "" {
				return true
			}
		}
		return false
	}
	hasTool := func(language string, tools ...string) bool {
		for _, tool := range tools {
			home, _ := os.UserHomeDir()
			p := filepath.Join(home, ".lsp-gateway", "tools", language, "bin", tool)
			// On Windows, allow .exe/.bat/.cmd variants
			if runtime.GOOS == osWindows {
				for _, ext := range []string{"", ".exe", ".bat", ".cmd"} {
					if _, err := os.Stat(p + ext); err == nil {
						return true
					}
				}
			} else {
				if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
					return true
				}
			}
		}
		return false
	}
	switch lang {
	case langGo:
		return has("gopls") || hasTool(langGo, "gopls")
	case langPython:
		return testutils.PythonLSPAvailable()
	case langJavaScript, langTypeScript:
		return has("typescript-language-server") || hasTool(langTypeScript, "typescript-language-server")
	case langJava:
		return has("jdtls") || hasTool(langJava, "jdtls")
	case langRust:
		return testutils.RustAnalyzerAvailable()
	case langCSharp:
		return has("omnisharp", "OmniSharp") || hasTool(langCSharp, "omnisharp", "OmniSharp")
	case langKotlin:
		return has("kotlin-lsp", "kotlin-language-server") || hasTool(langKotlin, "kotlin-lsp", "kotlin-language-server")
	default:
		return false
	}
}

// ComprehensiveE2ETestSuite tests all supported LSP methods for all languages
type ComprehensiveE2ETestSuite struct {
	base.ComprehensiveTestBaseSuite
}

// TestAllLanguagesComprehensive runs comprehensive tests for all supported languages
func TestAllLanguagesComprehensive(t *testing.T) {
	for _, lang := range getLanguageConfigs() {
		lang := lang // capture range variable
		t.Run(lang.displayName, func(t *testing.T) {
			if v := strings.ToLower(strings.TrimSpace(os.Getenv("E2E_PARALLEL"))); v == "1" || v == valueTrue || v == valueYes {
				t.Parallel()
			}
			suite.Run(t, &LanguageSpecificSuite{
				language:    lang.name,
				displayName: lang.displayName,
			})
		})
	}
}

// LanguageSpecificSuite is a test suite for a specific language
type LanguageSpecificSuite struct {
	base.ComprehensiveTestBaseSuite
	language    string
	displayName string
}

// SetupSuite initializes the test suite for the specific language
func (suite *LanguageSpecificSuite) SetupSuite() {
	suite.Config = base.LanguageConfig{
		Language:      suite.language,
		DisplayName:   suite.displayName,
		HasRepoMgmt:   true,
		HasAllLSPTest: true,
	}
	suite.ComprehensiveTestBaseSuite.SetupSuite()
}

// TestComprehensiveServerLifecycle tests server lifecycle
func (suite *LanguageSpecificSuite) TestComprehensiveServerLifecycle() {
	suite.ComprehensiveTestBaseSuite.TestComprehensiveServerLifecycle()
}

// TestDefinitionComprehensive tests textDocument/definition
func (suite *LanguageSpecificSuite) TestDefinitionComprehensive() {
	suite.ComprehensiveTestBaseSuite.TestDefinitionComprehensive()
}

// TestReferencesComprehensive tests textDocument/references
func (suite *LanguageSpecificSuite) TestReferencesComprehensive() {
	suite.ComprehensiveTestBaseSuite.TestReferencesComprehensive()
}

// TestHoverComprehensive tests textDocument/hover
func (suite *LanguageSpecificSuite) TestHoverComprehensive() {
	suite.ComprehensiveTestBaseSuite.TestHoverComprehensive()
}

// TestDocumentSymbolComprehensive tests textDocument/documentSymbol
func (suite *LanguageSpecificSuite) TestDocumentSymbolComprehensive() {
	suite.ComprehensiveTestBaseSuite.TestDocumentSymbolComprehensive()
}

// TestWorkspaceSymbolComprehensive tests workspace/symbol
func (suite *LanguageSpecificSuite) TestWorkspaceSymbolComprehensive() {
	suite.ComprehensiveTestBaseSuite.TestWorkspaceSymbolComprehensive()
}

// TestCompletionComprehensive tests textDocument/completion
func (suite *LanguageSpecificSuite) TestCompletionComprehensive() {
	suite.ComprehensiveTestBaseSuite.TestCompletionComprehensive()
}

// TestAllLSPMethodsSequential tests all LSP methods sequentially
func (suite *LanguageSpecificSuite) TestAllLSPMethodsSequential() {
	suite.ComprehensiveTestBaseSuite.TestAllLSPMethodsSequential()
}
