package e2e_test

import (
	"testing"

	"lsp-gateway/tests/e2e/base"

	"github.com/stretchr/testify/suite"
)

// JavaRealClientComprehensiveE2ETestSuite tests all 6 supported LSP methods for Java
type JavaRealClientComprehensiveE2ETestSuite struct {
	base.ComprehensiveTestBaseSuite
}

// SetupSuite initializes the test suite for Java
func (suite *JavaRealClientComprehensiveE2ETestSuite) SetupSuite() {
	// Initialize language configuration
	suite.Config = base.LanguageConfig{
		Language:      "java",
		DisplayName:   "Java",
		HasRepoMgmt:   true,
		HasAllLSPTest: true,
	}

	// Call base setup
	suite.ComprehensiveTestBaseSuite.SetupSuite()
}

// Test methods that delegate to base
func (suite *JavaRealClientComprehensiveE2ETestSuite) TestJavaComprehensiveServerLifecycle() {
	suite.TestComprehensiveServerLifecycle()
}

func (suite *JavaRealClientComprehensiveE2ETestSuite) TestJavaDefinitionComprehensive() {
	suite.TestDefinitionComprehensive()
}

func (suite *JavaRealClientComprehensiveE2ETestSuite) TestJavaReferencesComprehensive() {
	suite.TestReferencesComprehensive()
}

func (suite *JavaRealClientComprehensiveE2ETestSuite) TestJavaHoverComprehensive() {
	suite.TestHoverComprehensive()
}

func (suite *JavaRealClientComprehensiveE2ETestSuite) TestJavaDocumentSymbolComprehensive() {
	suite.TestDocumentSymbolComprehensive()
}

func (suite *JavaRealClientComprehensiveE2ETestSuite) TestJavaWorkspaceSymbolComprehensive() {
	suite.TestWorkspaceSymbolComprehensive()
}

func (suite *JavaRealClientComprehensiveE2ETestSuite) TestJavaCompletionComprehensive() {
	suite.TestCompletionComprehensive()
}

func (suite *JavaRealClientComprehensiveE2ETestSuite) TestJavaAllLSPMethodsSequential() {
	suite.TestAllLSPMethodsSequential()
}

// TestJavaRealClientComprehensiveE2ETestSuite runs the test suite
func TestJavaRealClientComprehensiveE2ETestSuite(t *testing.T) {
	suite.Run(t, new(JavaRealClientComprehensiveE2ETestSuite))
}
