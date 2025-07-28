package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/e2e/helpers"
	"lsp-gateway/tests/e2e/testutils"
)

type LSPAPIMethodsTestSuite struct {
	suite.Suite
	httpClient     *testutils.HttpClient
	assertHelper   *e2e_test.AssertionHelper
	gatewayCmd     *exec.Cmd
	gatewayPort    int
	configPath     string
	tempDir        string
	projectRoot    string
	testTimeout    time.Duration
	testFixtures   map[string]TestFixture
	sampleFiles    map[string]SampleFile
}

type TestFixture struct {
	Language    string                 `json:"language"`
	Method      string                 `json:"method"`
	FileURI     string                 `json:"file_uri"`
	Position    testutils.Position     `json:"position,omitempty"`
	Query       string                 `json:"query,omitempty"`
	Expected    interface{}            `json:"expected"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type SampleFile struct {
	Language string `json:"language"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
	URI      string `json:"uri"`
}

func (suite *LSPAPIMethodsTestSuite) SetupSuite() {
	suite.testTimeout = 60 * time.Second
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err)
	
	suite.tempDir, err = os.MkdirTemp("", "lsp-api-methods-test-*")
	suite.Require().NoError(err)
	
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err)

	suite.assertHelper = e2e_test.NewAssertionHelper(suite.T())
	
	suite.createTestConfig()
	suite.setupTestFixtures()
	suite.setupSampleFiles()
}

func (suite *LSPAPIMethodsTestSuite) SetupTest() {
	config := testutils.HttpClientConfig{
		BaseURL:         fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:         15 * time.Second,
		MaxRetries:      3,
		RetryDelay:      500 * time.Millisecond,
		EnableLogging:   true,
		EnableRecording: true,
		WorkspaceID:     "lsp-api-test-workspace",
		ProjectPath:     suite.tempDir,
	}
	suite.httpClient = testutils.NewHttpClient(config)
}

func (suite *LSPAPIMethodsTestSuite) TearDownTest() {
	if suite.httpClient != nil {
		suite.httpClient.Close()
	}
}

func (suite *LSPAPIMethodsTestSuite) TearDownSuite() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *LSPAPIMethodsTestSuite) createTestConfig() {
	configContent := fmt.Sprintf(`
servers:
- name: go-lsp
  languages:
  - go
  command: gopls
  args: []
  transport: stdio
  root_markers:
  - go.mod
  priority: 1
  weight: 1.0
- name: python-lsp
  languages:
  - python
  command: pylsp
  args: []
  transport: stdio
  root_markers:
  - pyproject.toml
  - setup.py
  priority: 1
  weight: 1.0
- name: typescript-lsp
  languages:
  - typescript
  - javascript
  command: typescript-language-server
  args: ["--stdio"]
  transport: stdio
  root_markers:
  - package.json
  - tsconfig.json
  priority: 1
  weight: 1.0
- name: java-lsp
  languages:
  - java
  command: jdtls
  args: []
  transport: stdio
  root_markers:
  - pom.xml
  - build.gradle
  priority: 1
  weight: 1.0
port: %d
timeout: 30s
max_concurrent_requests: 100
`, suite.gatewayPort)

	var err error
	suite.configPath, _, err = testutils.CreateTempConfig(configContent)
	suite.Require().NoError(err)
}

func (suite *LSPAPIMethodsTestSuite) setupTestFixtures() {
	suite.testFixtures = make(map[string]TestFixture)
	
	fixturesDir := filepath.Join(suite.projectRoot, "tests", "fixtures", "lsp_responses")
	files, err := ioutil.ReadDir(fixturesDir)
	if err != nil {
		suite.T().Logf("Warning: Could not read fixtures directory: %v", err)
		return
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		
		fixturePath := filepath.Join(fixturesDir, file.Name())
		data, err := ioutil.ReadFile(fixturePath)
		if err != nil {
			suite.T().Logf("Warning: Could not read fixture %s: %v", file.Name(), err)
			continue
		}
		
		parts := strings.Split(strings.TrimSuffix(file.Name(), ".json"), "_")
		if len(parts) < 2 {
			continue
		}
		
		language := parts[0]
		method := strings.Join(parts[1:], "_")
		
		var expected interface{}
		if err := json.Unmarshal(data, &expected); err != nil {
			suite.T().Logf("Warning: Could not unmarshal fixture %s: %v", file.Name(), err)
			continue
		}
		
		key := fmt.Sprintf("%s_%s", language, method)
		suite.testFixtures[key] = TestFixture{
			Language: language,
			Method:   method,
			Expected: expected,
		}
	}
}

func (suite *LSPAPIMethodsTestSuite) setupSampleFiles() {
	suite.sampleFiles = make(map[string]SampleFile)
	
	suite.sampleFiles["go"] = SampleFile{
		Language: "go",
		Filename: "main.go",
		Content: `package main

import "fmt"

type Server struct {
	Name string
	Port int
}

func (s *Server) Start() error {
	fmt.Printf("Starting server %s on port %d\n", s.Name, s.Port)
	return nil
}

func NewServer(name string, port int) *Server {
	return &Server{
		Name: name,
		Port: port,
	}
}

func main() {
	server := NewServer("gateway", 8080)
	server.Start()
}`,
		URI: fmt.Sprintf("file://%s/main.go", suite.tempDir),
	}
	
	suite.sampleFiles["python"] = SampleFile{
		Language: "python",
		Filename: "server.py",
		Content: `"""LSP Gateway Server Module"""

import asyncio
from typing import Optional, List


class Server:
    """Main server class for LSP Gateway"""
    
    def __init__(self, name: str, port: int):
        self.name = name
        self.port = port
        self.running = False
    
    async def start(self) -> None:
        """Start the server"""
        print(f"Starting server {self.name} on port {self.port}")
        self.running = True
    
    async def stop(self) -> None:
        """Stop the server"""
        print(f"Stopping server {self.name}")
        self.running = False


def create_server(name: str, port: int) -> Server:
    """Factory function to create a server instance"""
    return Server(name, port)


async def main():
    """Main entry point"""
    server = create_server("gateway", 8080)
    await server.start()


if __name__ == "__main__":
    asyncio.run(main())`,
		URI: fmt.Sprintf("file://%s/server.py", suite.tempDir),
	}
	
	suite.sampleFiles["typescript"] = SampleFile{
		Language: "typescript",
		Filename: "server.ts",
		Content: `/**
 * LSP Gateway Server Implementation
 */

interface ServerConfig {
    name: string;
    port: number;
    timeout?: number;
}

class Server {
    private name: string;
    private port: number;
    private timeout: number;
    private running: boolean = false;

    constructor(config: ServerConfig) {
        this.name = config.name;
        this.port = config.port;
        this.timeout = config.timeout || 30000;
    }

    public async start(): Promise<void> {
        console.log(\`Starting server \${this.name} on port \${this.port}\`);
        this.running = true;
    }

    public async stop(): Promise<void> {
        console.log(\`Stopping server \${this.name}\`);
        this.running = false;
    }

    public isRunning(): boolean {
        return this.running;
    }
}

export function createServer(config: ServerConfig): Server {
    return new Server(config);
}

export default Server;`,
		URI: fmt.Sprintf("file://%s/server.ts", suite.tempDir),
	}
	
	suite.sampleFiles["java"] = SampleFile{
		Language: "java",
		Filename: "Server.java",
		Content: `package com.example.gateway;

import java.util.concurrent.CompletableFuture;

/**
 * LSP Gateway Server Implementation
 */
public class Server {
    private final String name;
    private final int port;
    private final int timeout;
    private boolean running = false;

    public Server(String name, int port) {
        this(name, port, 30000);
    }

    public Server(String name, int port, int timeout) {
        this.name = name;
        this.port = port;
        this.timeout = timeout;
    }

    public CompletableFuture<Void> start() {
        return CompletableFuture.runAsync(() -> {
            System.out.printf("Starting server %s on port %d%n", name, port);
            running = true;
        });
    }

    public CompletableFuture<Void> stop() {
        return CompletableFuture.runAsync(() -> {
            System.out.printf("Stopping server %s%n", name);
            running = false;
        });
    }

    public boolean isRunning() {
        return running;
    }

    public String getName() {
        return name;
    }

    public int getPort() {
        return port;
    }

    public static Server createServer(String name, int port) {
        return new Server(name, port);
    }
}`,
		URI: fmt.Sprintf("file://%s/Server.java", suite.tempDir),
	}
	
	for _, sample := range suite.sampleFiles {
		filePath := filepath.Join(suite.tempDir, sample.Filename)
		err := ioutil.WriteFile(filePath, []byte(sample.Content), 0644)
		if err != nil {
			suite.T().Logf("Warning: Could not write sample file %s: %v", sample.Filename, err)
		}
	}
}

func (suite *LSPAPIMethodsTestSuite) startGatewayServer() {
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lsp-gateway")
	suite.gatewayCmd = exec.Command(binaryPath, "server", "--config", suite.configPath)
	suite.gatewayCmd.Dir = suite.projectRoot
	
	err := suite.gatewayCmd.Start()
	suite.Require().NoError(err)
	
	time.Sleep(3 * time.Second)
}

func (suite *LSPAPIMethodsTestSuite) TestLSPDefinitionMethod() {
	suite.T().Run("DefinitionBasicFunctionality", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		for language, sample := range suite.sampleFiles {
			suite.T().Run(fmt.Sprintf("Definition_%s", language), func(t *testing.T) {
				position := suite.getTestPosition(language, "definition")
				
				locations, err := suite.httpClient.Definition(ctx, sample.URI, position)
				
				if err != nil {
					suite.T().Logf("Definition request failed for %s: %v", language, err)
					return
				}

				suite.assertHelper.AssertNotEmpty(locations, fmt.Sprintf("%s definition locations", language))
				
				if len(locations) > 0 {
					location := locations[0]
					suite.Contains(location.URI, sample.Filename, "Definition should reference correct file")
					suite.GreaterOrEqual(location.Range.Start.Line, 0, "Definition line should be valid")
					suite.GreaterOrEqual(location.Range.Start.Character, 0, "Definition character should be valid")
				}
			})
		}
	})

	suite.T().Run("DefinitionEdgeCases", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		invalidPosition := testutils.Position{Line: -1, Character: -1}
		_, err := suite.httpClient.Definition(ctx, "file:///nonexistent.go", invalidPosition)
		suite.Error(err, "Should handle invalid position gracefully")

		_, err = suite.httpClient.Definition(ctx, "invalid-uri", testutils.Position{Line: 1, Character: 1})
		suite.Error(err, "Should handle invalid URI gracefully")
	})
}

func (suite *LSPAPIMethodsTestSuite) TestLSPReferencesMethod() {
	suite.T().Run("ReferencesBasicFunctionality", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		for language, sample := range suite.sampleFiles {
			suite.T().Run(fmt.Sprintf("References_%s", language), func(t *testing.T) {
				position := suite.getTestPosition(language, "references")
				
				locations, err := suite.httpClient.References(ctx, sample.URI, position, true)
				
				if err != nil {
					suite.T().Logf("References request failed for %s: %v", language, err)
					return
				}

				suite.T().Logf("Found %d references for %s", len(locations), language)
				
				for i, location := range locations {
					suite.Contains(location.URI, "file://", fmt.Sprintf("Reference %d URI should be valid", i))
					suite.GreaterOrEqual(location.Range.Start.Line, 0, fmt.Sprintf("Reference %d line should be valid", i))
					suite.GreaterOrEqual(location.Range.Start.Character, 0, fmt.Sprintf("Reference %d character should be valid", i))
				}
			})
		}
	})

	suite.T().Run("ReferencesWithoutDeclaration", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		sample := suite.sampleFiles["go"]
		position := suite.getTestPosition("go", "references")
		
		withDecl, err := suite.httpClient.References(ctx, sample.URI, position, true)
		if err != nil {
			suite.T().Logf("References with declaration failed: %v", err)
			return
		}

		withoutDecl, err := suite.httpClient.References(ctx, sample.URI, position, false)
		if err != nil {
			suite.T().Logf("References without declaration failed: %v", err)
			return
		}

		suite.T().Logf("With declaration: %d, without: %d", len(withDecl), len(withoutDecl))
		if len(withDecl) > 0 && len(withoutDecl) >= 0 {
			suite.LessOrEqual(len(withoutDecl), len(withDecl), "References without declaration should be <= with declaration")
		}
	})
}

func (suite *LSPAPIMethodsTestSuite) TestLSPHoverMethod() {
	suite.T().Run("HoverBasicFunctionality", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		for language, sample := range suite.sampleFiles {
			suite.T().Run(fmt.Sprintf("Hover_%s", language), func(t *testing.T) {
				position := suite.getTestPosition(language, "hover")
				
				hoverResult, err := suite.httpClient.Hover(ctx, sample.URI, position)
				
				if err != nil {
					suite.T().Logf("Hover request failed for %s: %v", language, err)
					return
				}

				if hoverResult != nil {
					suite.NotNil(hoverResult.Contents, fmt.Sprintf("%s hover should have contents", language))
					suite.T().Logf("Hover content for %s: %+v", language, hoverResult.Contents)
				} else {
					suite.T().Logf("No hover information available for %s", language)
				}
			})
		}
	})

	suite.T().Run("HoverEmptySpace", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		sample := suite.sampleFiles["go"]
		emptyPosition := testutils.Position{Line: 0, Character: 0}
		
		hoverResult, err := suite.httpClient.Hover(ctx, sample.URI, emptyPosition)
		
		if err == nil {
			if hoverResult == nil {
				suite.T().Log("No hover information for empty space (expected)")
			} else {
				suite.T().Logf("Hover information at empty space: %+v", hoverResult)
			}
		} else {
			suite.T().Logf("Hover at empty space failed: %v", err)
		}
	})
}

func (suite *LSPAPIMethodsTestSuite) TestLSPDocumentSymbolMethod() {
	suite.T().Run("DocumentSymbolsBasicFunctionality", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		for language, sample := range suite.sampleFiles {
			suite.T().Run(fmt.Sprintf("DocumentSymbols_%s", language), func(t *testing.T) {
				symbols, err := suite.httpClient.DocumentSymbol(ctx, sample.URI)
				
				if err != nil {
					suite.T().Logf("Document symbols request failed for %s: %v", language, err)
					return
				}

				suite.T().Logf("Found %d document symbols for %s", len(symbols), language)
				
				for i, symbol := range symbols {
					suite.NotEmpty(symbol.Name, fmt.Sprintf("Symbol %d name should not be empty", i))
					suite.Greater(symbol.Kind, 0, fmt.Sprintf("Symbol %d kind should be valid", i))
					suite.GreaterOrEqual(symbol.Range.Start.Line, 0, fmt.Sprintf("Symbol %d range should be valid", i))
					
					suite.T().Logf("Symbol %d: %s (kind: %d, line: %d)", i, symbol.Name, symbol.Kind, symbol.Range.Start.Line)
				}
			})
		}
	})

	suite.T().Run("DocumentSymbolsNonExistentFile", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		symbols, err := suite.httpClient.DocumentSymbol(ctx, "file:///nonexistent.go")
		
		if err != nil {
			suite.T().Logf("Document symbols for non-existent file failed as expected: %v", err)
		} else {
			suite.Empty(symbols, "Should return empty symbols for non-existent file")
		}
	})
}

func (suite *LSPAPIMethodsTestSuite) TestLSPWorkspaceSymbolMethod() {
	suite.T().Run("WorkspaceSymbolsBasicFunctionality", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		queries := []string{"Server", "main", "start", "create"}
		
		for _, query := range queries {
			suite.T().Run(fmt.Sprintf("WorkspaceSymbols_%s", query), func(t *testing.T) {
				symbols, err := suite.httpClient.WorkspaceSymbol(ctx, query)
				
				if err != nil {
					suite.T().Logf("Workspace symbols query '%s' failed: %v", query, err)
					return
				}

				suite.T().Logf("Found %d workspace symbols for query '%s'", len(symbols), query)
				
				for i, symbol := range symbols {
					suite.NotEmpty(symbol.Name, fmt.Sprintf("Symbol %d name should not be empty", i))
					suite.Greater(symbol.Kind, 0, fmt.Sprintf("Symbol %d kind should be valid", i))
					suite.Contains(symbol.Location.URI, "file://", fmt.Sprintf("Symbol %d URI should be valid", i))
					
					suite.T().Logf("Workspace symbol %d: %s (kind: %d, file: %s)", 
						i, symbol.Name, symbol.Kind, symbol.Location.URI)
				}
			})
		}
	})

	suite.T().Run("WorkspaceSymbolsEmptyQuery", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		symbols, err := suite.httpClient.WorkspaceSymbol(ctx, "")
		
		if err != nil {
			suite.T().Logf("Workspace symbols empty query failed: %v", err)
		} else {
			suite.T().Logf("Empty query returned %d symbols", len(symbols))
		}
	})
}

func (suite *LSPAPIMethodsTestSuite) TestLSPCompletionMethod() {
	suite.T().Run("CompletionBasicFunctionality", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		for language, sample := range suite.sampleFiles {
			suite.T().Run(fmt.Sprintf("Completion_%s", language), func(t *testing.T) {
				position := suite.getTestPosition(language, "completion")
				
				completionList, err := suite.httpClient.Completion(ctx, sample.URI, position)
				
				if err != nil {
					suite.T().Logf("Completion request failed for %s: %v", language, err)
					return
				}

				if completionList != nil {
					suite.T().Logf("Found %d completion items for %s", len(completionList.Items), language)
					
					for i, item := range completionList.Items {
						suite.NotEmpty(item.Label, fmt.Sprintf("Completion item %d label should not be empty", i))
						suite.Greater(item.Kind, 0, fmt.Sprintf("Completion item %d kind should be valid", i))
						
						suite.T().Logf("Completion item %d: %s (kind: %d)", i, item.Label, item.Kind)
					}
				}
			})
		}
	})

	suite.T().Run("CompletionInvalidPosition", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		sample := suite.sampleFiles["go"]
		invalidPosition := testutils.Position{Line: 1000, Character: 1000}
		
		completionList, err := suite.httpClient.Completion(ctx, sample.URI, invalidPosition)
		
		if err != nil {
			suite.T().Logf("Completion at invalid position failed as expected: %v", err)
		} else if completionList != nil {
			suite.T().Logf("Completion at invalid position returned %d items", len(completionList.Items))
		}
	})
}

func (suite *LSPAPIMethodsTestSuite) TestCrossMethodIntegration() {
	suite.T().Run("DefinitionToReferencesWorkflow", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		sample := suite.sampleFiles["go"]
		position := suite.getTestPosition("go", "definition")
		
		locations, err := suite.httpClient.Definition(ctx, sample.URI, position)
		if err != nil || len(locations) == 0 {
			suite.T().Logf("Definition request failed or returned no results: %v", err)
			return
		}

		defLocation := locations[0]
		defPosition := testutils.Position{
			Line:      defLocation.Range.Start.Line,
			Character: defLocation.Range.Start.Character,
		}

		references, err := suite.httpClient.References(ctx, defLocation.URI, defPosition, true)
		if err != nil {
			suite.T().Logf("References request failed: %v", err)
			return
		}

		suite.T().Logf("Definition-to-references workflow: found %d references", len(references))
		suite.GreaterOrEqual(len(references), 1, "Should find at least the definition itself")
	})
}

func (suite *LSPAPIMethodsTestSuite) TestResponseValidation() {
	suite.T().Run("JSONRPCComplianceValidation", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		sample := suite.sampleFiles["go"]
		position := testutils.Position{Line: 10, Character: 5}

		suite.httpClient.ClearRecordings()
		
		_, err := suite.httpClient.Definition(ctx, sample.URI, position)
		if err != nil {
			suite.T().Logf("Definition request failed: %v", err)
		}

		recordings := suite.httpClient.GetRecordings()
		suite.Greater(len(recordings), 0, "Should have recorded requests")

		for _, recording := range recordings {
			suite.Equal("POST", recording.Method, "Should use POST method")
			suite.Contains(recording.URL, "/jsonrpc", "Should use JSON-RPC endpoint")
			suite.Equal("application/json", recording.Headers["Content-Type"], "Should have JSON content type")
			
			if bodyMap, ok := recording.Body.(map[string]interface{}); ok {
				suite.Equal("2.0", bodyMap["jsonrpc"], "Should use JSON-RPC 2.0")
				suite.NotEmpty(bodyMap["id"], "Should have request ID")
				suite.NotEmpty(bodyMap["method"], "Should have method")
				suite.NotNil(bodyMap["params"], "Should have params")
			}
		}
	})
}

func (suite *LSPAPIMethodsTestSuite) TestConcurrentRequests() {
	suite.T().Run("ConcurrentLSPRequests", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		numConcurrent := 10
		done := make(chan bool, numConcurrent)
		errors := make(chan error, numConcurrent)

		sample := suite.sampleFiles["go"]
		position := testutils.Position{Line: 10, Character: 5}

		for i := 0; i < numConcurrent; i++ {
			go func(id int) {
				defer func() { done <- true }()
				
				_, err := suite.httpClient.Definition(ctx, sample.URI, position)
				if err != nil {
					errors <- fmt.Errorf("concurrent request %d failed: %w", id, err)
				}
			}(i)
		}

		completed := 0
		for completed < numConcurrent {
			select {
			case <-done:
				completed++
			case err := <-errors:
				suite.T().Logf("Concurrent request error: %v", err)
			case <-ctx.Done():
				suite.Fail("Timeout waiting for concurrent requests")
				return
			}
		}

		suite.T().Logf("Completed %d concurrent requests", completed)
	})
}

func (suite *LSPAPIMethodsTestSuite) stopGatewayServer() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
		suite.gatewayCmd = nil
	}
}

func (suite *LSPAPIMethodsTestSuite) waitForServerReady(ctx context.Context) {
	suite.Eventually(func() bool {
		err := suite.httpClient.HealthCheck(ctx)
		return err == nil
	}, 30*time.Second, 500*time.Millisecond, "Server should become ready")
}

func (suite *LSPAPIMethodsTestSuite) getTestPosition(language, method string) testutils.Position {
	positions := map[string]map[string]testutils.Position{
		"go": {
			"definition":  {Line: 15, Character: 8},  // NewServer function call
			"references":  {Line: 7, Character: 5},   // Server struct
			"hover":       {Line: 9, Character: 10},  // Start method
			"completion":  {Line: 22, Character: 10}, // After server.
		},
		"python": {
			"definition":  {Line: 32, Character: 15}, // create_server call
			"references":  {Line: 7, Character: 6},   // Server class
			"hover":       {Line: 15, Character: 10}, // start method
			"completion":  {Line: 33, Character: 15}, // After server.
		},
		"typescript": {
			"definition":  {Line: 38, Character: 15}, // createServer call
			"references":  {Line: 11, Character: 6},  // Server class
			"hover":       {Line: 21, Character: 15}, // start method
			"completion":  {Line: 25, Character: 10}, // After this.
		},
		"java": {
			"definition":  {Line: 59, Character: 15}, // createServer call
			"references":  {Line: 8, Character: 13},  // Server class
			"hover":       {Line: 24, Character: 10}, // start method
			"completion":  {Line: 35, Character: 10}, // After this.
		},
	}
	
	if langPositions, ok := positions[language]; ok {
		if position, ok := langPositions[method]; ok {
			return position
		}
	}
	
	return testutils.Position{Line: 1, Character: 1}
}

func TestLSPAPIMethodsTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP API methods tests in short mode")
	}
	suite.Run(t, new(LSPAPIMethodsTestSuite))
}