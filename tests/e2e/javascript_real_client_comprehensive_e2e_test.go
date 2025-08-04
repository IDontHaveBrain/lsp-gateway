package e2e_test

import (
	"testing"

	"lsp-gateway/tests/e2e/base"

	"github.com/stretchr/testify/suite"
)

// JavaScriptRealClientComprehensiveE2ETestSuite tests all 6 supported LSP methods for JavaScript
type JavaScriptRealClientComprehensiveE2ETestSuite struct {
	base.ComprehensiveTestBaseSuite
}

// SetupSuite initializes the test suite for JavaScript
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) SetupSuite() {
	// Initialize language configuration
	suite.Config = base.LanguageConfig{
		Language:      "javascript",
		DisplayName:   "JavaScript",
		HasRepoMgmt:   true,
		HasAllLSPTest: true,
	}

	// Call base setup
	suite.ComprehensiveTestBaseSuite.SetupSuite()
}

// Test methods that delegate to base
func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptComprehensiveServerLifecycle() {
	suite.TestComprehensiveServerLifecycle()
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptDefinitionComprehensive() {
	suite.TestDefinitionComprehensive()
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptReferencesComprehensive() {
	suite.TestReferencesComprehensive()
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptHoverComprehensive() {
	suite.TestHoverComprehensive()
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptDocumentSymbolComprehensive() {
	suite.TestDocumentSymbolComprehensive()
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptWorkspaceSymbolComprehensive() {
	suite.TestWorkspaceSymbolComprehensive()
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptCompletionComprehensive() {
	suite.TestCompletionComprehensive()
}

func (suite *JavaScriptRealClientComprehensiveE2ETestSuite) TestJavaScriptAllLSPMethodsSequential() {
	suite.TestAllLSPMethodsSequential()
}

// TestJavaScriptRealClientComprehensiveE2ETestSuite runs the test suite
func TestJavaScriptRealClientComprehensiveE2ETestSuite(t *testing.T) {
	suite.Run(t, new(JavaScriptRealClientComprehensiveE2ETestSuite))
}