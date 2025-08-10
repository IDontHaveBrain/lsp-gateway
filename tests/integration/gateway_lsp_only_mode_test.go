package integration

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "testing"
    "time"

    "github.com/stretchr/testify/require"

    "lsp-gateway/src/config"
    "lsp-gateway/src/server"
    "lsp-gateway/src/utils"
)

func TestGatewayLSPOneAndCacheHeaders(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    if _, err := exec.LookPath("gopls"); err != nil {
        t.Skip("Go LSP server (gopls) not installed, skipping test")
    }

    testDir := t.TempDir()
    goMod := `module gateway-lsp-only

go 1.21
`
    require.NoError(t, os.WriteFile(filepath.Join(testDir, "go.mod"), []byte(goMod), 0644))
    filePath := filepath.Join(testDir, "main.go")
    content := `package main

func Foo() int { return 42 }
`
    require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

    origWD, _ := os.Getwd()
    require.NoError(t, os.Chdir(testDir))
    t.Cleanup(func() { _ = os.Chdir(origWD) })

    cfg := &config.Config{
        Cache: &config.CacheConfig{Enabled: true, StoragePath: t.TempDir(), MaxMemoryMB: 64, TTLHours: 1, Languages: []string{"go"}},
        Servers: map[string]*config.ServerConfig{
            "go": {Command: "gopls", Args: []string{"serve"}},
        },
    }

    gw, err := server.NewHTTPGateway(":18090", cfg, true)
    require.NoError(t, err)

    ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
    defer cancel()
    require.NoError(t, gw.Start(ctx))
    defer gw.Stop()

    // Wait for /health
    client := &http.Client{Timeout: 5 * time.Second}
    ready := false
    for i := 0; i < 50; i++ {
        resp, err := client.Get("http://localhost:18090/health")
        if err == nil && resp.StatusCode == http.StatusOK {
            resp.Body.Close()
            ready = true
            break
        }
        if resp != nil { resp.Body.Close() }
        time.Sleep(100 * time.Millisecond)
    }
    require.True(t, ready, "gateway not ready")

    // Allowed method: hover
    uri := utils.FilePathToURI(filePath)
    req1 := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "textDocument/hover",
        "id":      1,
        "params": map[string]interface{}{
            "textDocument": map[string]interface{}{"uri": uri},
            "position":     map[string]interface{}{"line": 1, "character": 5},
        },
    }
    body1, _ := json.Marshal(req1)
    httpReq1, _ := http.NewRequest(http.MethodPost, "http://localhost:18090/jsonrpc", bytes.NewReader(body1))
    httpReq1.Header.Set("Content-Type", "application/json")
    resp1, err := client.Do(httpReq1)
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, resp1.StatusCode)

    // Check cache headers present on first request
    status1 := resp1.Header.Get("X-LSP-Cache-Status")
    rtime1 := resp1.Header.Get("X-LSP-Response-Time")
    require.NotEmpty(t, status1)
    if _, err := strconv.ParseInt(rtime1, 10, 64); err != nil {
        t.Fatalf("invalid X-LSP-Response-Time: %s", rtime1)
    }
    resp1.Body.Close()

    // Second identical request should be a cache hit
    httpReq2, _ := http.NewRequest(http.MethodPost, "http://localhost:18090/jsonrpc", bytes.NewReader(body1))
    httpReq2.Header.Set("Content-Type", "application/json")
    resp2, err := client.Do(httpReq2)
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, resp2.StatusCode)
    status2 := resp2.Header.Get("X-LSP-Cache-Status")
    require.Equal(t, "hit", status2)
    resp2.Body.Close()

    // Disallowed method in lspOnly mode
    disallowed := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "workspace/executeCommand",
        "id":      99,
        "params":  map[string]interface{}{"command": "noop"},
    }
    body3, _ := json.Marshal(disallowed)
    httpReq3, _ := http.NewRequest(http.MethodPost, "http://localhost:18090/jsonrpc", bytes.NewReader(body3))
    httpReq3.Header.Set("Content-Type", "application/json")
    resp3, err := client.Do(httpReq3)
    require.NoError(t, err)
    require.Equal(t, http.StatusOK, resp3.StatusCode)

    var payload map[string]interface{}
    require.NoError(t, json.NewDecoder(resp3.Body).Decode(&payload))
    resp3.Body.Close()

    errObj, ok := payload["error"].(map[string]interface{})
    require.True(t, ok, "expected JSON-RPC error response")
    code, ok := errObj["code"].(float64)
    require.True(t, ok)
    require.Equal(t, float64(-32601), code)
}
