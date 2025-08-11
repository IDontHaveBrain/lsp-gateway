package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
)

type mcpReq struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type mcpResp struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      int                    `json:"id"`
	Result  map[string]interface{} `json:"result"`
	Error   interface{}            `json:"error"`
}

func TestMCPFindReferences_UsesSCIPAndPrintsRefs(t *testing.T) {
	wd, _ := os.Getwd()
	tmpDir := filepath.Join(wd, "..", "..", "tmp-mcp-refs")
	_ = os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	mainFile := filepath.Join(tmpDir, "main.go")
	mainContent := `package main

func Foo() {}

func main() {
	Foo()
}
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	goMod := filepath.Join(tmpDir, "go.mod")
	_ = os.WriteFile(goMod, []byte("module m\n\ngo 1.21\n"), 0644)

	orig, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer os.Chdir(orig)

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

	mcp, err := server.NewMCPServer(cfg)
	if err != nil {
		t.Fatalf("new mcp: %v", err)
	}
	t.Logf("Created MCP server with cache enabled, background indexing: %v", cfg.Cache.BackgroundIndex)

	prIn, pwIn := io.Pipe()
	prOut, pwOut := io.Pipe()
	defer pwIn.Close()
	defer prOut.Close()

	lines := make(chan []byte, 10)
	go func() {
		_ = mcp.Run(prIn, pwOut)
	}()
	go func() {
		s := bufio.NewScanner(prOut)
		for s.Scan() {
			b := make([]byte, len(s.Bytes()))
			copy(b, s.Bytes())
			lines <- b
		}
		close(lines)
	}()

	initReq := mcpReq{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: map[string]interface{}{"capabilities": map[string]interface{}{}}}
	b, _ := json.Marshal(initReq)
	pwIn.Write(append(b, '\n'))
	t.Logf("Sent initialize request")

	time.Sleep(1500 * time.Millisecond)
	t.Logf("Waited 1.5s for initialization, now sending findSymbols")

	symCall := mcpReq{JSONRPC: "2.0", ID: 2, Method: "tools/call", Params: map[string]interface{}{
		"name": "findSymbols",
		"arguments": map[string]interface{}{
			"pattern":  "Foo",
			"filePath": "*.go",
		},
	}}
	b, _ = json.Marshal(symCall)
	pwIn.Write(append(b, '\n'))

	// Wait for findSymbols response and verify indexing worked
	if !waitForFindSymbolsResponse(t, lines, 15*time.Second) {
		t.Fatalf("findSymbols did not return expected results within timeout")
	}
	t.Logf("findSymbols succeeded, waiting for reference indexing to complete")

	// Wait additional time for enhanced reference indexing to complete
	// Background indexing includes both symbols and references
	time.Sleep(10 * time.Second)
	t.Logf("Completed wait for reference indexing, now sending findReferences")

	refCall := mcpReq{JSONRPC: "2.0", ID: 3, Method: "tools/call", Params: map[string]interface{}{
		"name": "findReferences",
		"arguments": map[string]interface{}{
			"pattern":    "Foo",
			"filePath":   "*.go",
			"maxResults": 20,
		},
	}}
	b, _ = json.Marshal(refCall)
	pwIn.Write(append(b, '\n'))

	deadline := time.After(8 * time.Second)
	var got []byte
Loop:
	for {
		select {
		case <-deadline:
			break Loop
		case line, ok := <-lines:
			if !ok {
				break Loop
			}
			if bytes.Contains(line, []byte("\"id\":3")) {
				got = line
				break Loop
			}
		}
	}
	if len(got) == 0 {
		t.Fatalf("no response for findReferences")
	}

	var resp mcpResp
	if err := json.Unmarshal(got, &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	content, ok := resp.Result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("no content")
	}
	text := content[0].(map[string]interface{})["text"].(string)
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("response text is not JSON: %v\n%s", err, text)
	}
	refsAny, ok := payload["references"].([]interface{})
	if !ok || len(refsAny) == 0 {
		t.Fatalf("missing references in payload: %s", text)
	}
	foundLineOnly := false
	verifiedTextOnly := false
	for _, r := range refsAny {
		m := r.(map[string]interface{})
		loc := m["location"].(string)
		if !bytes.Contains([]byte(loc), []byte("main.go")) {
			continue
		}
		if regexp.MustCompile(`:(\d+):(\d+)`).MatchString(loc) {
			t.Fatalf("location contains line:col but should be file:line only: %s", loc)
		}
		if regexp.MustCompile(`:(\d+)$`).MatchString(loc) {
			foundLineOnly = true
		}
		if txt, ok := m["text"].(string); ok && txt != "" {
			if _, exists := m["code"]; exists {
				t.Fatalf("code field should not be present: %v", m)
			}
			verifiedTextOnly = true
		}
	}
	if !foundLineOnly {
		t.Fatalf("no location with file:line found in %s", text)
	}
	if !verifiedTextOnly {
		t.Fatalf("text for the line not present in %s", text)
	}
}

// waitForFindSymbolsResponse waits for findSymbols to return results with the expected symbol
func waitForFindSymbolsResponse(t *testing.T, lines <-chan []byte, timeout time.Duration) bool {
	deadline := time.After(timeout)

	for {
		select {
		case <-deadline:
			t.Logf("Timeout waiting for findSymbols response")
			return false
		case line, ok := <-lines:
			if !ok {
				t.Logf("Channel closed while waiting for findSymbols response")
				return false
			}
			if bytes.Contains(line, []byte("\"id\":2")) {
				var resp mcpResp
				if err := json.Unmarshal(line, &resp); err != nil {
					t.Logf("Failed to decode findSymbols response: %v", err)
					return false
				}

				// Check for errors first
				if resp.Error != nil {
					t.Logf("findSymbols returned error: %v", resp.Error)
					return false
				}

				// Check if we got symbols
				content, ok := resp.Result["content"].([]interface{})
				if !ok || len(content) == 0 {
					t.Logf("No content in findSymbols response")
					return false
				}

				text := content[0].(map[string]interface{})["text"].(string)
				var payload map[string]interface{}
				if err := json.Unmarshal([]byte(text), &payload); err != nil {
					t.Logf("Response text is not JSON: %v\nRaw text: %s", err, text)
					return false
				}

				symbols, ok := payload["symbols"].([]interface{})
				if !ok || len(symbols) == 0 {
					t.Logf("findSymbols returned empty symbols, cache may not be ready yet: %s", text)
					return false
				}

				// Verify we found the Foo function
				for _, sym := range symbols {
					if symMap, ok := sym.(map[string]interface{}); ok {
						if name, ok := symMap["name"].(string); ok && name == "Foo" {
							t.Logf("Successfully found Foo symbol via findSymbols")
							return true
						}
					}
				}

				t.Logf("findSymbols returned %d symbols but no 'Foo' found: %s", len(symbols), text)
				return false
			}
		}
	}
}
