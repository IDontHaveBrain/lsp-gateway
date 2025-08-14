package lspconv

import (
	"encoding/json"
	"testing"

	"lsp-gateway/src/internal/types"
)

func TestParseWorkspaceSymbols_VariousShapes(t *testing.T) {
	arr := []interface{}{map[string]interface{}{"name": "Foo", "kind": float64(types.Function), "location": map[string]interface{}{"uri": "file:///x.go", "range": map[string]interface{}{"start": map[string]interface{}{"line": float64(1), "character": float64(2)}, "end": map[string]interface{}{"line": float64(1), "character": float64(3)}}}}}
	got := ParseWorkspaceSymbols(arr)
	if len(got) != 1 || got[0].Name != "Foo" {
		t.Fatalf("unexpected: %+v", got)
	}
	raw, _ := json.Marshal(arr)
	got2 := ParseWorkspaceSymbols(json.RawMessage(raw))
	if len(got2) != 1 || got2[0].Name != "Foo" {
		t.Fatalf("unexpected raw: %+v", got2)
	}
}

func TestParseDocumentSymbols_AndToSymbolInformation(t *testing.T) {
	ds := []interface{}{map[string]interface{}{"name": "Bar", "kind": float64(types.Function), "range": map[string]interface{}{"start": map[string]interface{}{"line": float64(2), "character": float64(0)}, "end": map[string]interface{}{"line": float64(5), "character": float64(0)}}, "selectionRange": map[string]interface{}{"start": map[string]interface{}{"line": float64(2), "character": float64(1)}, "end": map[string]interface{}{"line": float64(2), "character": float64(3)}}}}
	out := ParseDocumentSymbols(ds)
	if len(out) != 1 || out[0] == nil || out[0].Name != "Bar" {
		t.Fatalf("unexpected: %+v", out)
	}
	conv := ParseDocumentSymbolsToSymbolInformation(ds, "file:///y.go")
	if len(conv) != 1 || conv[0].Name != "Bar" || conv[0].Location.URI != "file:///y.go" {
		t.Fatalf("unexpected conv: %+v", conv)
	}
	if conv[0].SelectionRange == nil || conv[0].SelectionRange.Start.Line != 2 {
		t.Fatalf("selection range lost: %+v", conv[0])
	}
	si := []types.SymbolInformation{{Name: "Z", Kind: types.Variable, Location: types.Location{URI: "u"}}}
	conv2 := ParseDocumentSymbolsToSymbolInformation(si, "d")
	if len(conv2) != 1 || conv2[0].Name != "Z" {
		t.Fatalf("unexpected direct: %+v", conv2)
	}
}

func TestParseLocations_Shapes(t *testing.T) {
	l := types.Location{URI: "file:///a", Range: types.Range{Start: types.Position{Line: 1}}}
	if got := ParseLocations(l); len(got) != 1 {
		t.Fatalf("location single failed")
	}
	arr := []interface{}{map[string]interface{}{"uri": "file:///a", "range": map[string]interface{}{"start": map[string]interface{}{"line": float64(1), "character": float64(0)}, "end": map[string]interface{}{"line": float64(1), "character": float64(1)}}}}
	raw, _ := json.Marshal(arr)
	if got := ParseLocations(json.RawMessage(raw)); len(got) != 1 || got[0].URI == "" {
		t.Fatalf("raw array failed: %+v", got)
	}
}

func TestKindStringConverters(t *testing.T) {
	s1 := LSPSymbolKindToString(types.Class, StyleLowercase)
	if s1 != "class" {
		t.Fatalf("got %s", s1)
	}
	s2 := SCIPSymbolKindToString(12, StyleUppercase)
	if s2 != "FUNCTION" {
		t.Fatalf("got %s", s2)
	}
}

func TestParseRangeFromMap(t *testing.T) {
	m := map[string]interface{}{"start": map[string]interface{}{"line": float64(10), "character": float64(2)}, "end": map[string]interface{}{"line": float64(11), "character": float64(3)}}
	r, ok := ParseRangeFromMap(m)
	if !ok || r.Start.Line != 10 || r.End.Line != 11 {
		t.Fatalf("unexpected: %+v %v", r, ok)
	}
}
