package e2e_test

import (
	"testing"

	"lsp-gateway/tests/e2e/base"

	"github.com/stretchr/testify/suite"
)

// TypeScriptRealClientComprehensiveE2ETestSuite tests all 6 supported LSP methods for TypeScript
type TypeScriptRealClientComprehensiveE2ETestSuite struct {
	base.ComprehensiveTestBaseSuite
}

// SetupSuite initializes the test suite for TypeScript
func (suite *TypeScriptRealClientComprehensiveE2ETestSuite) SetupSuite() {
	// Initialize language configuration
	suite.Config = base.LanguageConfig{
		Language:      "typescript",
		DisplayName:   "TypeScript",
		HasRepoMgmt:   false,
		HasAllLSPTest: true,
	}

	// Call base setup
	suite.ComprehensiveTestBaseSuite.SetupSuite()
}

// Test methods that delegate to base
func (suite *TypeScriptRealClientComprehensiveE2ETestSuite) TestTypeScriptComprehensiveServerLifecycle() {
	suite.TestComprehensiveServerLifecycle()
}

func (suite *TypeScriptRealClientComprehensiveE2ETestSuite) TestTypeScriptDefinitionComprehensive() {
	suite.TestDefinitionComprehensive()
}

func (suite *TypeScriptRealClientComprehensiveE2ETestSuite) TestTypeScriptReferencesComprehensive() {
	suite.TestReferencesComprehensive()
}

func (suite *TypeScriptRealClientComprehensiveE2ETestSuite) TestTypeScriptHoverComprehensive() {
	suite.TestHoverComprehensive()
}

func (suite *TypeScriptRealClientComprehensiveE2ETestSuite) TestTypeScriptDocumentSymbolComprehensive() {
	suite.TestDocumentSymbolComprehensive()
}

func (suite *TypeScriptRealClientComprehensiveE2ETestSuite) TestTypeScriptWorkspaceSymbolComprehensive() {
	suite.TestWorkspaceSymbolComprehensive()
}

func (suite *TypeScriptRealClientComprehensiveE2ETestSuite) TestTypeScriptCompletionComprehensive() {
	suite.TestCompletionComprehensive()
}

func (suite *TypeScriptRealClientComprehensiveE2ETestSuite) TestTypeScriptAllLSPMethodsSequential() {
	suite.TestAllLSPMethodsSequential()
}

// TestTypeScriptRealClientComprehensiveE2ETestSuite runs the test suite
func TestTypeScriptRealClientComprehensiveE2ETestSuite(t *testing.T) {
	suite.Run(t, new(TypeScriptRealClientComprehensiveE2ETestSuite))
}
