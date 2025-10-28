package e2e_test

import (
	"testing"

	"lsp-gateway/tests/e2e/base"
	"lsp-gateway/tests/e2e/testutils"

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
	suite.TestReferencesComprehensive()
}

func TestRustReferencesSuite(t *testing.T) {
	if !testutils.RustAnalyzerAvailable() {
		t.Skip("rust-analyzer not available; skipping Rust E2E suite")
	}
	suite.Run(t, new(RustReferencesSuite))
}
