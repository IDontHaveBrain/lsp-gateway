package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
)

func TestSCIPCacheOperations(t *testing.T) {
	_, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("Go LSP server (gopls) not installed, skipping test")
	}

	testDir := t.TempDir()
	cacheDir := filepath.Join(testDir, "cache")

	testFile1 := filepath.Join(testDir, "main.go")
	err = os.WriteFile(testFile1, []byte(`package main

import "fmt"

type Server struct {
	name string
	port int
}

func (s *Server) Start() {
	fmt.Printf("Starting server %s on port %d\n", s.name, s.port)
}

func NewServer(name string, port int) *Server {
	return &Server{
		name: name,
		port: port,
	}
}

func main() {
	server := NewServer("api", 8080)
	server.Start()
}
`), 0644)
	require.NoError(t, err)

	testFile2 := filepath.Join(testDir, "handler.go")
	err = os.WriteFile(testFile2, []byte(`package main

import "net/http"

type Handler struct {
	server *Server
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func NewHandler(s *Server) *Handler {
	return &Handler{server: s}
}
`), 0644)
	require.NoError(t, err)

	cacheConfig := &config.CacheConfig{
		Enabled:         true,
		StoragePath:     cacheDir,
		MaxMemoryMB:     128,
		TTLHours:        2,
		Languages:       []string{"go"},
		BackgroundIndex: false,
		DiskCache:       true,
		EvictionPolicy:  "lru",
	}

	cfg := &config.Config{
		Cache: cacheConfig,
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
			},
		},
	}

	lspManager, err := server.NewLSPManager(cfg)
	require.NoError(t, err)

	cacheInterface := lspManager.GetCache()
	require.NotNil(t, cacheInterface)
	scipCache, ok := cacheInterface.(*cache.SimpleCacheManager)
	require.True(t, ok)

	ctx := context.Background()
	err = lspManager.Start(ctx)
	require.NoError(t, err)
	defer lspManager.Stop()

	t.Run("SCIPDefinitionStorage", func(t *testing.T) {
		testURI := uri.File(testFile1)

		params := &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: testURI,
				},
				Position: protocol.Position{
					Line:      21,
					Character: 12,
				},
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/definition", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		_, ok := result.(protocol.Location)
		if !ok {
			locations, ok := result.([]protocol.Location)
			require.True(t, ok)
			require.NotEmpty(t, locations)
		}

		cachedLocations, found := scipCache.GetCachedDefinition("NewServer")
		require.True(t, found)
		require.NotEmpty(t, cachedLocations)

		for _, loc := range cachedLocations {
			require.NotEmpty(t, loc.URI)
			require.GreaterOrEqual(t, loc.Range.Start.Line, 0)
			require.GreaterOrEqual(t, loc.Range.Start.Character, 0)
		}
	})

	t.Run("SCIPReferencesStorage", func(t *testing.T) {
		testURI := uri.File(testFile1)

		params := &protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: testURI,
				},
				Position: protocol.Position{
					Line:      4,
					Character: 6,
				},
			},
			Context: protocol.ReferenceContext{
				IncludeDeclaration: true,
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/references", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		locations, ok := result.([]protocol.Location)
		require.True(t, ok)
		require.NotEmpty(t, locations)

		cachedLocations, found := scipCache.GetCachedReferences("Server")
		require.True(t, found)
		require.NotEmpty(t, cachedLocations)

		for _, loc := range cachedLocations {
			require.NotEmpty(t, loc.URI)
			require.GreaterOrEqual(t, loc.Range.Start.Line, 0)
			require.GreaterOrEqual(t, loc.Range.Start.Character, 0)
		}
	})

	t.Run("SCIPMethodResultStorage", func(t *testing.T) {
		testURI := uri.File(testFile1)

		hoverParams := &protocol.HoverParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: testURI,
				},
				Position: protocol.Position{
					Line:      9,
					Character: 14,
				},
			},
		}

		hoverResult, err := lspManager.ProcessRequest(ctx, "textDocument/hover", hoverParams)
		require.NoError(t, err)
		require.NotNil(t, hoverResult)

		hover, ok := hoverResult.(*protocol.Hover)
		require.True(t, ok)
		require.NotNil(t, hover.Contents)

		err = scipCache.StoreMethodResult(
			"textDocument/hover",
			hoverParams,
			hoverResult,
		)
		require.NoError(t, err)

		cachedHover, found := scipCache.GetCachedHover("Start")
		require.True(t, found)
		require.NotNil(t, cachedHover)
	})

	t.Run("SCIPCrossFilerferences", func(t *testing.T) {
		testURI2 := uri.File(testFile2)
		err = scipCache.IndexDocument(ctx, string(testURI2), "go", nil)
		require.NoError(t, err)

		params := &protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: testURI2,
				},
				Position: protocol.Position{
					Line:      5,
					Character: 10,
				},
			},
			Context: protocol.ReferenceContext{
				IncludeDeclaration: true,
			},
		}

		result, err := lspManager.ProcessRequest(ctx, "textDocument/references", params)
		require.NoError(t, err)
		require.NotNil(t, result)

		locations, ok := result.([]protocol.Location)
		require.True(t, ok)
		require.NotEmpty(t, locations)

		var mainFileRef bool
		var handlerFileRef bool
		for _, loc := range locations {
			if filepath.Base(string(loc.URI)) == "main.go" {
				mainFileRef = true
			}
			if filepath.Base(string(loc.URI)) == "handler.go" {
				handlerFileRef = true
			}
		}
		require.True(t, mainFileRef)
		require.True(t, handlerFileRef)
	})

	t.Run("SCIPSymbolQuery", func(t *testing.T) {
		testURI1 := uri.File(testFile1)
		err = scipCache.IndexDocument(ctx, string(testURI1), "go", nil)
		require.NoError(t, err)

		query := &cache.IndexQuery{
			Type:     "symbol",
			Symbol:   "Server",
			Language: "go",
		}

		result, err := scipCache.QueryIndex(ctx, query)
		require.NoError(t, err)
		require.NotNil(t, result)

		require.NotEmpty(t, result.Results)
		require.Equal(t, "symbol", result.Type)
	})

	t.Run("SCIPCacheInvalidation", func(t *testing.T) {
		testURI := uri.File(testFile1)

		hoverParams := &protocol.HoverParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{
				TextDocument: protocol.TextDocumentIdentifier{
					URI: testURI,
				},
				Position: protocol.Position{
					Line:      4,
					Character: 6,
				},
			},
		}

		result1, err := lspManager.ProcessRequest(ctx, "textDocument/hover", hoverParams)
		require.NoError(t, err)
		require.NotNil(t, result1)

		err = scipCache.InvalidateDocument(string(testURI))
		require.NoError(t, err)

		cachedHover2, found := scipCache.GetCachedHover("Server")
		require.False(t, found)
		require.Nil(t, cachedHover2)

		result2, err := lspManager.ProcessRequest(ctx, "textDocument/hover", hoverParams)
		require.NoError(t, err)
		require.NotNil(t, result2)
	})

	t.Run("SCIPConcurrentOperations", func(t *testing.T) {
		type testCase struct {
			name   string
			line   int
			char   int
			method string
		}

		testCases := []testCase{
			{"definition1", 21, 12, "textDocument/definition"},
			{"hover1", 4, 6, "textDocument/hover"},
			{"references1", 13, 10, "textDocument/references"},
			{"definition2", 13, 10, "textDocument/definition"},
			{"hover2", 21, 12, "textDocument/hover"},
		}

		resultChan := make(chan error, len(testCases))

		for _, tc := range testCases {
			go func(tc testCase) {
				testURI := uri.File(testFile1)

				var params interface{}
				switch tc.method {
				case "textDocument/definition":
					params = &protocol.DefinitionParams{
						TextDocumentPositionParams: protocol.TextDocumentPositionParams{
							TextDocument: protocol.TextDocumentIdentifier{URI: testURI},
							Position:     protocol.Position{Line: uint32(tc.line), Character: uint32(tc.char)},
						},
					}
				case "textDocument/hover":
					params = &protocol.HoverParams{
						TextDocumentPositionParams: protocol.TextDocumentPositionParams{
							TextDocument: protocol.TextDocumentIdentifier{URI: testURI},
							Position:     protocol.Position{Line: uint32(tc.line), Character: uint32(tc.char)},
						},
					}
				case "textDocument/references":
					params = &protocol.ReferenceParams{
						TextDocumentPositionParams: protocol.TextDocumentPositionParams{
							TextDocument: protocol.TextDocumentIdentifier{URI: testURI},
							Position:     protocol.Position{Line: uint32(tc.line), Character: uint32(tc.char)},
						},
						Context: protocol.ReferenceContext{IncludeDeclaration: true},
					}
				}

				_, err := lspManager.ProcessRequest(ctx, tc.method, params)
				resultChan <- err
			}(tc)
		}

		timeout := time.After(10 * time.Second)
		for i := 0; i < len(testCases); i++ {
			select {
			case err := <-resultChan:
				require.NoError(t, err)
			case <-timeout:
				t.Fatal("Concurrent operations timed out")
			}
		}
	})

	t.Run("SCIPEvictionPolicy", func(t *testing.T) {
		smallCacheConfig := &config.CacheConfig{
			Enabled:         true,
			StoragePath:     filepath.Join(testDir, "small_cache"),
			MaxMemoryMB:     1,
			TTLHours:        1,
			Languages:       []string{"go"},
			BackgroundIndex: false,
			DiskCache:       false,
			EvictionPolicy:  "lru",
		}

		smallCache, err := cache.NewSCIPCacheManager(smallCacheConfig)
		require.NoError(t, err)
		defer smallCache.Stop()

		for i := 0; i < 100; i++ {
			tempFile := filepath.Join(testDir, fmt.Sprintf("temp%d.go", i))
			content := fmt.Sprintf(`package main
func Temp%d() {}
`, i)
			err = os.WriteFile(tempFile, []byte(content), 0644)
			require.NoError(t, err)

			tempURI := uri.File(tempFile)
			err = smallCache.IndexDocument(ctx, string(tempURI), "go", nil)
			require.NoError(t, err)
		}

		metrics := smallCache.GetMetrics()
		require.NotNil(t, metrics)
		require.Greater(t, metrics.EvictionCount, int64(0))
	})
}
