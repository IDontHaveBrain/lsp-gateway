package e2e_test

import (
	"testing"

	"lsp-gateway/tests/e2e/base"

	"github.com/stretchr/testify/suite"
)

type LanguageTestConfig struct {
	name        string
	displayName string
}

var languageConfigs = []LanguageTestConfig{
	{name: "go", displayName: "Go"},
	{name: "python", displayName: "Python"},
	{name: "javascript", displayName: "JavaScript"},
	{name: "typescript", displayName: "TypeScript"},
	{name: "java", displayName: "Java"},
}

// ComprehensiveE2ETestSuite tests all 6 supported LSP methods for all languages
type ComprehensiveE2ETestSuite struct {
	base.ComprehensiveTestBaseSuite
}

// TestAllLanguagesComprehensive runs comprehensive tests for all supported languages
func TestAllLanguagesComprehensive(t *testing.T) {
	for _, lang := range languageConfigs {
		lang := lang // capture range variable
		t.Run(lang.displayName, func(t *testing.T) {
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
