package e2e_test

import (
	"testing"

	"lsp-gateway/tests/e2e/base"

	"github.com/stretchr/testify/suite"
)

// PythonRealClientComprehensiveE2ETestSuite tests all 6 supported LSP methods for Python
type PythonRealClientComprehensiveE2ETestSuite struct {
	base.ComprehensiveTestBaseSuite
}

// SetupSuite initializes the test suite for Python
func (suite *PythonRealClientComprehensiveE2ETestSuite) SetupSuite() {
	// Initialize language configuration
	suite.Config = base.LanguageConfig{
		Language:      "python",
		DisplayName:   "Python",
		HasRepoMgmt:   true,
		HasAllLSPTest: true,
	}

	// Call base setup
	suite.ComprehensiveTestBaseSuite.SetupSuite()
}

// Test methods that delegate to base
func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveServerLifecycle() {
	suite.TestComprehensiveServerLifecycle()
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveDefinition() {
	suite.TestDefinitionComprehensive()
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveReferences() {
	suite.TestReferencesComprehensive()
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveHover() {
	suite.TestHoverComprehensive()
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveDocumentSymbol() {
	suite.TestDocumentSymbolComprehensive()
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveWorkspaceSymbol() {
	suite.TestWorkspaceSymbolComprehensive()
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonComprehensiveCompletion() {
	suite.TestCompletionComprehensive()
}

func (suite *PythonRealClientComprehensiveE2ETestSuite) TestPythonAllLSPMethodsSequential() {
	suite.TestAllLSPMethodsSequential()
}

// TestPythonRealClientComprehensiveE2ETestSuite runs the test suite
func TestPythonRealClientComprehensiveE2ETestSuite(t *testing.T) {
	suite.Run(t, new(PythonRealClientComprehensiveE2ETestSuite))
}