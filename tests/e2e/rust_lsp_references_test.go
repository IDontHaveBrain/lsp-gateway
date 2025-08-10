package e2e_test

import (
	"testing"

	"lsp-gateway/tests/e2e/base"

	"github.com/stretchr/testify/suite"
)

type RustReferencesSuite struct {
	base.ComprehensiveTestBaseSuite
}

func (suite *RustReferencesSuite) SetupSuite() {
	suite.Config = base.LanguageConfig{
		Language:      "rust",
		DisplayName:   "Rust",
		HasRepoMgmt:   true,
		HasAllLSPTest: false,
	}
	suite.ComprehensiveTestBaseSuite.SetupSuite()
}

func (suite *RustReferencesSuite) TestRustLSPReferences() {
	suite.ComprehensiveTestBaseSuite.TestReferencesComprehensive()
}

func TestRustReferencesSuite(t *testing.T) {
	suite.Run(t, new(RustReferencesSuite))
}
