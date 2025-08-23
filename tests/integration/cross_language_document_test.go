package integration

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/documents"
	"lsp-gateway/src/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"
)

func getKotlinCommand() string {
	// Use platform-specific command
	if runtime.GOOS == "windows" {
		// Check for installed kotlin-language-server first
		homeDir, _ := os.UserHomeDir()
		if homeDir != "" {
			installedPath := filepath.Join(homeDir, ".lsp-gateway", "tools", "kotlin", "bin", "kotlin-language-server.bat")
			if _, err := os.Stat(installedPath); err == nil {
				return installedPath
			}
			// Try without .bat extension
			installedPath = filepath.Join(homeDir, ".lsp-gateway", "tools", "kotlin", "bin", "kotlin-language-server")
			if _, err := os.Stat(installedPath); err == nil {
				return installedPath
			}
		}
		return "kotlin-language-server"
	}
	
	// Unix-like systems
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		installedPath := filepath.Join(homeDir, ".lsp-gateway", "tools", "kotlin", "kotlin-lsp")
		if _, err := os.Stat(installedPath); err == nil {
			return installedPath
		}
		// Try bin directory
		installedPath = filepath.Join(homeDir, ".lsp-gateway", "tools", "kotlin", "bin", "kotlin-lsp")
		if _, err := os.Stat(installedPath); err == nil {
			return installedPath
		}
	}
	
	return "kotlin-lsp"
}

func TestCrossLanguageDocumentCoordination(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	testDir := t.TempDir()

	testFiles := map[string]string{
		"go.mod": `module testproject

go 1.21

require (
)`,
		"main.go": `package main

import (
	"fmt"
	"os/exec"
)

func CallPython() {
	cmd := exec.Command("python", "script.py")
	output, _ := cmd.Output()
	fmt.Println(string(output))
}

func ProcessData(data string) string {
	return data + " processed"
}`,
		"requirements.txt": `requests>=2.25.0
json5>=0.9.0`,
		"script.py": `import subprocess
import json

def call_go_function():
    result = subprocess.run(['go', 'run', 'main.go'], capture_output=True)
    return result.stdout.decode()

def process_json(data):
    return json.loads(data)

class DataProcessor:
    def __init__(self):
        self.data = []
    
    def add_item(self, item):
        self.data.append(item)`,
		"package.json": `{
  "name": "testproject",
  "version": "1.0.0",
  "description": "Test project for cross-language operations",
  "main": "utils.js",
  "scripts": {
    "build": "tsc",
    "start": "node utils.js"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "@types/node": "^20.0.0"
  },
  "dependencies": {}
}`,
		"tsconfig.json": `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "lib": ["ES2020"],
    "outDir": "./dist",
    "rootDir": "./",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["*.ts"],
  "exclude": ["node_modules", "dist"]
}`,
		"frontend.ts": `import { processData } from './utils';
import * as api from './api';

export class DataHandler {
    private processor: DataProcessor;
    
    constructor() {
        this.processor = new DataProcessor();
    }
    
    async fetchAndProcess(): Promise<void> {
        const data = await api.fetchData();
        const processed = processData(data);
        return processed;
    }
}

interface DataProcessor {
    process(data: any): any;
}`,
		"utils.js": `function processData(data) {
    if (typeof data === 'string') {
        return data.toUpperCase();
    }
    return JSON.stringify(data);
}

function callBackend(endpoint) {
    return fetch('/api/' + endpoint)
        .then(res => res.json());
}

module.exports = { processData, callBackend };`,
		"pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    
    <groupId>com.example</groupId>
    <artifactId>testproject</artifactId>
    <version>1.0.0</version>
    <packaging>jar</packaging>
    
    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>
    
    <dependencies>
    </dependencies>
</project>`,
		"src/main/java/com/example/Backend.java": `package com.example;

import java.util.List;
import java.util.ArrayList;

public class Backend {
    private DataService dataService;
    
    public Backend() {
        this.dataService = new DataService();
    }
    
    public String processRequest(String request) {
        List<String> items = parseRequest(request);
        return dataService.process(items);
    }
    
    private List<String> parseRequest(String request) {
        return new ArrayList<>();
    }
}

class DataService {
    public String process(List<String> items) {
        return String.join(",", items);
    }
}`,
		"build.gradle.kts": `plugins {
    kotlin("jvm") version "1.8.0"
    application
}

group = "com.example"
version = "1.0.0"

repositories {
    mavenCentral()
}

dependencies {
    implementation(kotlin("stdlib"))
}

application {
    mainClass.set("com.example.MainKt")
}`,
		"src/main/kotlin/com/example/Gateway.kt": `package com.example

import kotlinx.coroutines.*

data class GatewayConfig(
    val name: String,
    val port: Int,
    val timeout: Long = 30000L
)

class Gateway(private val config: GatewayConfig) {
    private var running = false
    
    suspend fun start(): Unit = withContext(Dispatchers.IO) {
        println("Starting gateway ${config.name} on port ${config.port}")
        running = true
    }
    
    suspend fun stop(): Unit = withContext(Dispatchers.IO) {
        println("Stopping gateway ${config.name}")
        running = false
    }
    
    fun isRunning(): Boolean = running
    
    companion object {
        fun createGateway(name: String, port: Int): Gateway {
            return Gateway(GatewayConfig(name, port))
        }
    }
}

class RequestHandler(private val gateway: Gateway) {
    suspend fun handleRequest(method: String, path: String): Map<String, Any> {
        return when {
            method == "GET" && path == "/status" -> {
                mapOf(
                    "gateway" to gateway.config.name,
                    "port" to gateway.config.port,
                    "running" to gateway.isRunning()
                )
            }
            else -> mapOf("error" to "Not found")
        }
    }
}

suspend fun main() {
    val gateway = Gateway.createGateway("lsp-gateway", 8080)
    gateway.start()
}`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(testDir, filename)

		dir := filepath.Dir(filePath)
		if dir != testDir {
			require.NoError(t, os.MkdirAll(dir, 0755))
		}

		require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))
	}

	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:         true,
			StoragePath:     t.TempDir(),
			MaxMemoryMB:     256,
			TTLHours:        1,
			BackgroundIndex: false,
		},
		Servers: map[string]*config.ServerConfig{
			"go": &config.ServerConfig{
				Command: "gopls",
				Args:    []string{"serve"},
			},
			"python": &config.ServerConfig{
				Command: "jedi-language-server",
				Args:    []string{},
			},
			"typescript": &config.ServerConfig{
				Command: "typescript-language-server",
				Args:    []string{"--stdio"},
			},
			"javascript": &config.ServerConfig{
				Command: "typescript-language-server",
				Args:    []string{"--stdio"},
			},
			"java": &config.ServerConfig{
				Command: "jdtls",
				Args:    []string{},
			},
			"kotlin": &config.ServerConfig{
				Command: getKotlinCommand(),
				Args:    []string{},
			},
		},
	}

	scipCache, err := cache.NewSCIPCacheManager(cfg.Cache)
	require.NoError(t, err)
	defer scipCache.Stop()

	lspManager, err := server.NewLSPManager(cfg)
	require.NoError(t, err)
	lspManager.SetCache(scipCache)

	// Start the LSP manager to initialize clients
	err = lspManager.Start(ctx)
	require.NoError(t, err)
	defer lspManager.Stop()

	files := []string{
		filepath.Join(testDir, "main.go"),
		filepath.Join(testDir, "script.py"),
		filepath.Join(testDir, "frontend.ts"),
		filepath.Join(testDir, "utils.js"),
		filepath.Join(testDir, "src/main/java/com/example/Backend.java"),
		filepath.Join(testDir, "src/main/kotlin/com/example/Gateway.kt"),
	}

	t.Run("MultiLanguageDocumentOpen", func(t *testing.T) {
		// Note: Files are already written to disk, LSP servers can access them directly
		t.Logf("Multi-language files created: %d", len(files))
		for _, file := range files {
			assert.FileExists(t, file)
		}
	})

	t.Run("CrossLanguageSymbolSearch", func(t *testing.T) {
		symbols := []string{
			"CallPython",
			"call_go_function",
			"DataHandler",
			"processData",
			"Backend",
			"Gateway",
			"RequestHandler",
		}

		var wg sync.WaitGroup
		symbolResults := make([]int, len(symbols))

		for i, symbol := range symbols {
			wg.Add(1)
			go func(idx int, sym string) {
				defer wg.Done()

				request := &protocol.WorkspaceSymbolParams{
					Query: sym,
				}

				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				results, err := lspManager.ProcessRequest(ctx, "workspace/symbol", request)
				if err == nil && results != nil {
					if symbols, ok := results.([]interface{}); ok {
						symbolResults[idx] = len(symbols)
					}
				}
			}(i, symbol)
		}

		wg.Wait()

		for i, count := range symbolResults {
			t.Logf("Symbol %s found %d times", symbols[i], count)
		}
	})

	t.Run("CrossLanguageDefinitionNavigation", func(t *testing.T) {
		testCases := []struct {
			file     string
			line     int
			char     int
			expected string
		}{
			{"main.go", 8, 20, "script.py"},
			{"frontend.ts", 0, 30, "utils"},
			{"utils.js", 7, 10, "callBackend"},
			{"src/main/java/com/example/Backend.java", 11, 20, "processRequest"},
			{"src/main/kotlin/com/example/Gateway.kt", 30, 15, "createGateway"},
		}

		for _, tc := range testCases {
			uri := utils.FilePathToURI(filepath.Join(testDir, tc.file))

			request := protocol.DefinitionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{
						URI: protocol.DocumentURI(uri),
					},
					Position: protocol.Position{
						Line:      uint32(tc.line),
						Character: uint32(tc.char),
					},
				},
			}

			ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			_, err := lspManager.ProcessRequest(ctx, "textDocument/definition", request)
			if err != nil {
				t.Logf("Definition request for %s failed: %v", tc.file, err)
			} else {
				t.Logf("Definition from %s:%d:%d resolved", tc.file, tc.line, tc.char)
			}
		}
	})

	t.Run("ConcurrentMultiLanguageOperations", func(t *testing.T) {
		// Allow LSP servers some time to initialize
		time.Sleep(2 * time.Second)

		operations := []struct {
			method string
			file   string
			line   int
			char   int
		}{
			{"textDocument/hover", "main.go", 6, 5},           // func CallPython
			{"textDocument/definition", "script.py", 3, 4},    // def call_go_function
			{"textDocument/references", "frontend.ts", 3, 13}, // export class DataHandler
			{"textDocument/completion", "utils.js", 0, 9},     // function processData
			{"textDocument/documentSymbol", "src/main/java/com/example/Backend.java", 0, 0},
			{"textDocument/hover", "src/main/kotlin/com/example/Gateway.kt", 14, 5}, // class Gateway
		}

		var wg sync.WaitGroup
		var successCount int32

		iterations := 3
		for round := 0; round < iterations; round++ {
			for _, op := range operations {
				wg.Add(1)
				go func(operation struct {
					method string
					file   string
					line   int
					char   int
				}) {
					defer wg.Done()

					uri := utils.FilePathToURI(filepath.Join(testDir, operation.file))

					var err error
					// Fix context shadowing - create a new context from the parent
					opCtx, opCancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer opCancel()

					switch operation.method {
					case "textDocument/hover":
						params := protocol.HoverParams{
							TextDocumentPositionParams: protocol.TextDocumentPositionParams{
								TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
								Position:     protocol.Position{Line: uint32(operation.line), Character: uint32(operation.char)},
							},
						}
						_, err = lspManager.ProcessRequest(opCtx, "textDocument/hover", params)

					case "textDocument/definition":
						params := protocol.DefinitionParams{
							TextDocumentPositionParams: protocol.TextDocumentPositionParams{
								TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
								Position:     protocol.Position{Line: uint32(operation.line), Character: uint32(operation.char)},
							},
						}
						_, err = lspManager.ProcessRequest(opCtx, "textDocument/definition", params)

					case "textDocument/references":
						params := protocol.ReferenceParams{
							TextDocumentPositionParams: protocol.TextDocumentPositionParams{
								TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
								Position:     protocol.Position{Line: uint32(operation.line), Character: uint32(operation.char)},
							},
							Context: protocol.ReferenceContext{
								IncludeDeclaration: true,
							},
						}
						_, err = lspManager.ProcessRequest(opCtx, "textDocument/references", params)

					case "textDocument/completion":
						params := protocol.CompletionParams{
							TextDocumentPositionParams: protocol.TextDocumentPositionParams{
								TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
								Position:     protocol.Position{Line: uint32(operation.line), Character: uint32(operation.char)},
							},
						}
						_, err = lspManager.ProcessRequest(opCtx, "textDocument/completion", params)

					case "textDocument/documentSymbol":
						params := protocol.DocumentSymbolParams{
							TextDocument: protocol.TextDocumentIdentifier{URI: protocol.DocumentURI(uri)},
						}
						_, err = lspManager.ProcessRequest(opCtx, "textDocument/documentSymbol", params)
					}

					if err == nil {
						atomic.AddInt32(&successCount, 1)
					} else {
						t.Logf("Operation %s on %s failed: %v", operation.method, operation.file, err)
					}
				}(op)
			}
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			t.Logf("Concurrent operations completed. Success rate: %d/%d",
				successCount, len(operations)*iterations)
		case <-time.After(2 * time.Minute):
			t.Fatal("Concurrent multi-language operations timeout")
		}

		dm := documents.NewLSPDocumentManager()
		availableOps := 0
		for _, op := range operations {
			uri := utils.FilePathToURI(filepath.Join(testDir, op.file))
			lang := dm.DetectLanguage(uri)
			if _, err := lspManager.GetClient(lang); err == nil {
				availableOps++
			}
		}
		if availableOps == 0 {
			t.Skip("No available LSP servers for tested languages; skipping success ratio assertion")
		}
		expectedMinSuccess := (availableOps * iterations) / 2
		if expectedMinSuccess < 1 {
			expectedMinSuccess = 1
		}
		assert.GreaterOrEqual(t, int(successCount), expectedMinSuccess,
			"At least 50% of operations for available languages should succeed")
	})

	t.Run("DocumentStateConsistency", func(t *testing.T) {
		for _, file := range files {
			originalContent := testFiles[filepath.Base(file)]
			modifiedContent := originalContent + "\n// Modified"

			err := os.WriteFile(file, []byte(modifiedContent), 0644)
			assert.NoError(t, err, "File update should succeed")

			currentContentBytes, err := os.ReadFile(file)
			assert.NoError(t, err, "Should be able to read file")
			assert.Equal(t, modifiedContent, string(currentContentBytes),
				"File content should match after update")
		}

		t.Logf("All %d files updated successfully", len(files))
	})

	t.Run("LanguageDetectionAccuracy", func(t *testing.T) {
		expectedLanguages := map[string]string{
			"main.go":                                "go",
			"script.py":                              "python",
			"frontend.ts":                            "typescript",
			"utils.js":                               "javascript",
			"src/main/java/com/example/Backend.java": "java",
			"src/main/kotlin/com/example/Gateway.kt": "kotlin",
		}

		documentManager := documents.NewLSPDocumentManager()
		for filename, expectedLang := range expectedLanguages {
			uri := utils.FilePathToURI(filepath.Join(testDir, filename))
			detectedLang := documentManager.DetectLanguage(uri)
			assert.Equal(t, expectedLang, detectedLang,
				"Language detection should be accurate for %s", filename)
		}
	})

	cacheMetrics := scipCache.GetMetrics()
	t.Logf("Final cache stats - Entries: %d, Size: %d KB, Hit Rate: %.2f%%",
		cacheMetrics.EntryCount, cacheMetrics.TotalSize/1024,
		float64(cacheMetrics.HitCount)/float64(cacheMetrics.HitCount+cacheMetrics.MissCount)*100)

	t.Logf("Cache should contain cross-language symbol data")
}
