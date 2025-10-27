package integration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
)

func TestMCPFindReferences_UsesSCIPAndReturnsReferences(t *testing.T) {
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("Go LSP server (gopls) not installed, skipping test")
	}

	wd, _ := os.Getwd()
	tmpDir := filepath.Join(wd, "..", "..", "tmp-mcp-refs")
	require.NoError(t, os.MkdirAll(tmpDir, 0o755))
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	mainFile := filepath.Join(tmpDir, "main.go")
	mainContent := `package main

func Foo() {}

func main() {
	Foo()
}
`
	require.NoError(t, os.WriteFile(mainFile, []byte(mainContent), 0o644))
	goMod := filepath.Join(tmpDir, "go.mod")
	require.NoError(t, os.WriteFile(goMod, []byte("module m\n\ngo 1.21\n"), 0o644))

	origWd, _ := os.Getwd()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:         true,
			MaxMemoryMB:     64,
			TTLHours:        1,
			BackgroundIndex: true,
			StoragePath:     filepath.Join(tmpDir, "cache"),
		},
		Servers: map[string]*config.ServerConfig{"go": {Command: "gopls", Args: []string{"serve"}}},
	}

	mcpServer, err := server.NewMCPServer(cfg)
	require.NoError(t, err)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverErrCh := make(chan error, 1)
	go func() {
		if err := mcpServer.Run(ctx, serverTransport); err != nil && !errors.Is(err, context.Canceled) {
			serverErrCh <- err
		}
		close(serverErrCh)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1.0.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	// Wait for the symbol to appear in the MCP tool output.
	require.Eventually(t, func() bool {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "findSymbols",
			Arguments: map[string]any{
				"pattern":  "Foo",
				"filePath": "*.go",
			},
		})
		if err != nil {
			t.Logf("findSymbols call failed: %v", err)
			return false
		}

		payload := extractStructuredPayload(t, res)
		symbols, ok := payload["symbols"].([]interface{})
		if !ok || len(symbols) == 0 {
			return false
		}
		for _, sym := range symbols {
			if symMap, ok := sym.(map[string]interface{}); ok {
				if name, ok := symMap["name"].(string); ok && name == "Foo" {
					return true
				}
			}
		}
		return false
	}, 15*time.Second, 500*time.Millisecond, "findSymbols never returned Foo")

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "findReferences",
		Arguments: map[string]any{
			"pattern":    "Foo",
			"filePath":   "*.go",
			"maxResults": 20,
		},
	})
	require.NoError(t, err)

	payload := extractStructuredPayload(t, res)
	refsAny, ok := payload["references"].([]interface{})
	require.True(t, ok, "references payload missing")
	require.NotEmpty(t, refsAny, "no references returned")

	foundLineOnly := false
	verifiedText := false
	rangePattern := regexp.MustCompile(`:(\d+)(-\d+)?$`)
	columnPattern := regexp.MustCompile(`:(\d+):(\d+)`)

	for _, r := range refsAny {
		refMap, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		loc, _ := refMap["location"].(string)
		if strings.Contains(loc, "main.go") {
			if columnPattern.MatchString(loc) {
				t.Fatalf("location contains line:col but should be file:line or file:line-line: %s", loc)
			}
			if rangePattern.MatchString(loc) {
				foundLineOnly = true
			}
		}
		if text, ok := refMap["text"].(string); ok && text != "" {
			if _, exists := refMap["code"]; !exists {
				verifiedText = true
			}
		}
	}

	require.True(t, foundLineOnly, "expected reference locations to be file:line format")
	require.True(t, verifiedText, "expected references to include surrounding source text")

	cancel()
	select {
	case err, ok := <-serverErrCh:
		if ok && err != nil {
			t.Fatalf("mcp server exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("server did not terminate")
	}
}

func extractStructuredPayload(t *testing.T, res *mcp.CallToolResult) map[string]interface{} {
	t.Helper()
	if res.StructuredContent != nil {
		if payload, ok := res.StructuredContent.(map[string]interface{}); ok {
			return payload
		}
	}

	if len(res.Content) == 0 {
		t.Fatalf("call tool result missing content")
	}

	switch c := res.Content[0].(type) {
	case *mcp.TextContent:
		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(c.Text), &payload))
		return payload
	default:
		t.Fatalf("unsupported content type %T", c)
	}
	return nil
}
