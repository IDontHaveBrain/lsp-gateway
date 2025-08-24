package e2e_test

import (
	"os"
	"testing"

	"lsp-gateway/tests/e2e/testutils"
)

// TestMain boots a single, multi-language shared server and clones all repos once.
// All E2E tests use this global server automatically.
func TestMain(m *testing.M) {
	_ = testutils.InitGlobalServer()
	code := m.Run()
	testutils.ShutdownGlobalServer()
	os.Exit(code)
}
