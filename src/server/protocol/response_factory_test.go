package protocol

import (
    "encoding/json"
    "fmt"
    "testing"
)

func TestResponseFactory_CreateSuccess_WithResult(t *testing.T) {
    rf := NewResponseFactory()
    resp := rf.CreateSuccess(1, map[string]interface{}{"ok": true})
    if resp.JSONRPC != JSONRPCVersion {
        t.Fatalf("unexpected jsonrpc: %s", resp.JSONRPC)
    }
    if resp.ID != 1 {
        t.Fatalf("unexpected id: %v", resp.ID)
    }
    if resp.Error != nil {
        t.Fatalf("expected no error, got: %+v", resp.Error)
    }
    b, err := json.Marshal(resp)
    if err != nil {
        t.Fatalf("marshal failed: %v", err)
    }
    if string(b) == "" {
        t.Fatal("empty json output")
    }
}

func TestResponseFactory_CreateSuccess_WithNilResult_EmitsNull(t *testing.T) {
    rf := NewResponseFactory()
    resp := rf.CreateSuccess(2, nil)
    b, err := json.Marshal(resp)
    if err != nil {
        t.Fatalf("marshal failed: %v", err)
    }
    if want, got := `"result":null`, string(b); !contains(got, want) {
        t.Fatalf("want %s in %s", want, got)
    }
}

func TestResponseFactory_CreateError(t *testing.T) {
    rf := NewResponseFactory()
    resp := rf.CreateError("abc", -32000, "Server error")
    if resp.Error == nil || resp.Error.Code != -32000 || resp.Error.Message != "Server error" {
        t.Fatalf("unexpected error payload: %+v", resp.Error)
    }
}

func TestResponseFactory_ErrorHelpers(t *testing.T) {
    rf := NewResponseFactory()
    cases := []struct {
        resp JSONRPCResponse
        code int
    }{
        {rf.CreateInternalError(1, fmt.Errorf("boom")), InternalError},
        {rf.CreateParseError(2), ParseError},
        {rf.CreateInvalidRequest(3, "bad"), InvalidRequest},
        {rf.CreateMethodNotFound(4, "nope"), MethodNotFound},
        {rf.CreateInvalidParams(5, "bad"), InvalidParams},
    }
    for _, tc := range cases {
        if tc.resp.Error == nil || tc.resp.Error.Code != tc.code {
            t.Fatalf("expected code %d, got %+v", tc.code, tc.resp.Error)
        }
    }
}

func TestResponseFactory_CreateCustomError(t *testing.T) {
    rf := NewResponseFactory()
    custom := NewRPCError(-32123, "custom", map[string]string{"k": "v"})
    resp := rf.CreateCustomError(7, custom)
    if resp.Error == nil || resp.Error.Code != -32123 || resp.Error.Message != "custom" {
        t.Fatalf("unexpected custom error: %+v", resp.Error)
    }
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
    for i := 0; i+len(sub) <= len(s); i++ {
        if s[i:i+len(sub)] == sub {
            return i
        }
    }
    return -1
}
