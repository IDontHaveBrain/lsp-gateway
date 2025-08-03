package e2e_test

import (
	"testing"

	"lsp-gateway/tests/e2e/base"

	"github.com/stretchr/testify/suite"
)

// GoRealClientComprehensiveE2ETestSuite tests all 6 supported LSP methods for Go
type GoRealClientComprehensiveE2ETestSuite struct {
	base.ComprehensiveTestBaseSuite
}

// SetupSuite initializes the test suite for Go
func (suite *GoRealClientComprehensiveE2ETestSuite) SetupSuite() {
	// Initialize with Go configuration
	suite.ComprehensiveTestBaseSuite = *base.NewComprehensiveTestSuite(base.LanguageConfig{
		Language:      "go",
		DisplayName:   "Go",
		HasRepoMgmt:   true,
		HasAllLSPTest: false,
	})

	// Call base setup
	suite.ComprehensiveTestBaseSuite.SetupSuite()
}

// Test methods that delegate to base
func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoComprehensiveServerLifecycle() {
	suite.TestComprehensiveServerLifecycle()
}

func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoDefinitionComprehensive() {
	suite.TestDefinitionComprehensive()
}

func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoReferencesComprehensive() {
	suite.TestReferencesComprehensive()
}

func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoHoverComprehensive() {
	suite.TestHoverComprehensive()
}

func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoDocumentSymbolComprehensive() {
	suite.TestDocumentSymbolComprehensive()
}

func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoWorkspaceSymbolComprehensive() {
	suite.TestWorkspaceSymbolComprehensive()
}

func (suite *GoRealClientComprehensiveE2ETestSuite) TestGoCompletionComprehensive() {
	suite.TestCompletionComprehensive()
}

// TestGoRealClientComprehensiveE2ETestSuite runs the test suite
func TestGoRealClientComprehensiveE2ETestSuite(t *testing.T) {
	suite.Run(t, new(GoRealClientComprehensiveE2ETestSuite))
}
