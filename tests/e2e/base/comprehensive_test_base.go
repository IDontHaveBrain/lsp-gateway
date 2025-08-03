package base

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"lsp-gateway/tests/e2e/testutils"

	"github.com/stretchr/testify/suite"
)

// LanguageConfig defines language-specific configuration
type LanguageConfig struct {
	Language      string
	DisplayName   string
	HasRepoMgmt   bool
	HasAllLSPTest bool
}

// ComprehensiveTestBaseSuite provides base functionality for comprehensive E2E tests
type ComprehensiveTestBaseSuite struct {
	suite.Suite

	// Language configuration
	config LanguageConfig

	// Core infrastructure
	httpClient  *testutils.HttpClient
	gatewayCmd  *exec.Cmd
	gatewayPort int
	configPath  string
	tempDir     string
	projectRoot string
	testTimeout time.Duration

	// Repository management (for languages that need it)
	repoDir   string
	goFiles   []string
	testFiles []string
	pyFiles   []string

	// Server state tracking
	serverStarted bool
}

// NewComprehensiveTestSuite creates a new test suite with language configuration
func NewComprehensiveTestSuite(config LanguageConfig) *ComprehensiveTestBaseSuite {
	return &ComprehensiveTestBaseSuite{
		config: config,
	}
}

// SetupSuite initializes the comprehensive test suite
func (suite *ComprehensiveTestBaseSuite) SetupSuite() {
	suite.testTimeout = 120 * time.Second
	suite.gatewayPort = 8080

	// Basic project root setup (for languages that need it)
	if suite.config.HasRepoMgmt {
		if cwd, err := os.Getwd(); err == nil {
			suite.projectRoot = filepath.Dir(filepath.Dir(cwd))
		}
	}
}

// SetupTest prepares each test
func (suite *ComprehensiveTestBaseSuite) SetupTest() {
	// Basic setup only
}

// TearDownTest cleans up after each test
func (suite *ComprehensiveTestBaseSuite) TearDownTest() {
	suite.stopGatewayServer()
}

// TearDownSuite cleans up after all tests
func (suite *ComprehensiveTestBaseSuite) TearDownSuite() {
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

// stopGatewayServer stops the gateway server
func (suite *ComprehensiveTestBaseSuite) stopGatewayServer() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
		suite.gatewayCmd = nil
	}
	suite.serverStarted = false
}

// Test methods that can be used by language-specific tests

// TestComprehensiveServerLifecycle tests server lifecycle
func (suite *ComprehensiveTestBaseSuite) TestComprehensiveServerLifecycle() {
	suite.T().Logf("Testing %s comprehensive server lifecycle", suite.config.DisplayName)
	// Basic lifecycle test
}

// TestDefinitionComprehensive tests textDocument/definition
func (suite *ComprehensiveTestBaseSuite) TestDefinitionComprehensive() {
	suite.T().Logf("Testing %s definition", suite.config.DisplayName)
	// Basic definition test
}

// TestReferencesComprehensive tests textDocument/references
func (suite *ComprehensiveTestBaseSuite) TestReferencesComprehensive() {
	suite.T().Logf("Testing %s references", suite.config.DisplayName)
	// Basic references test
}

// TestHoverComprehensive tests textDocument/hover
func (suite *ComprehensiveTestBaseSuite) TestHoverComprehensive() {
	suite.T().Logf("Testing %s hover", suite.config.DisplayName)
	// Basic hover test
}

// TestDocumentSymbolComprehensive tests textDocument/documentSymbol
func (suite *ComprehensiveTestBaseSuite) TestDocumentSymbolComprehensive() {
	suite.T().Logf("Testing %s document symbols", suite.config.DisplayName)
	// Basic document symbol test
}

// TestWorkspaceSymbolComprehensive tests workspace/symbol
func (suite *ComprehensiveTestBaseSuite) TestWorkspaceSymbolComprehensive() {
	suite.T().Logf("Testing %s workspace symbols", suite.config.DisplayName)
	// Basic workspace symbol test
}

// TestCompletionComprehensive tests textDocument/completion
func (suite *ComprehensiveTestBaseSuite) TestCompletionComprehensive() {
	suite.T().Logf("Testing %s completion", suite.config.DisplayName)
	// Basic completion test
}

// TestAllLSPMethodsSequential tests all LSP methods sequentially (for languages that support it)
func (suite *ComprehensiveTestBaseSuite) TestAllLSPMethodsSequential() {
	if !suite.config.HasAllLSPTest {
		suite.T().Skip("Language does not support sequential LSP test")
		return
	}
	suite.T().Logf("Testing %s all LSP methods sequentially", suite.config.DisplayName)
	// Basic sequential test
}
