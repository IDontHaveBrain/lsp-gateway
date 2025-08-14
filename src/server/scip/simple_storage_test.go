package scip

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"lsp-gateway/src/internal/types"
)

func TestSimpleSCIPStorage_StoreQueryPersist(t *testing.T) {
	dir := t.TempDir()
	cfg := SCIPStorageConfig{DiskCacheDir: dir, MemoryLimit: 8 * 1024 * 1024}
	s, err := NewSimpleSCIPStorage(cfg)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	uri := "file:///" + filepath.ToSlash(filepath.Join(dir, "a.go"))
	doc := &SCIPDocument{URI: uri, Language: "go", Content: []byte("package a\nfunc Foo(){}\n"), Size: 32, Occurrences: []SCIPOccurrence{{Range: types.Range{Start: types.Position{Line: 1}}, Symbol: "sym:foo", SymbolRoles: types.SymbolRoleDefinition}}, SymbolInformation: []SCIPSymbolInformation{{Symbol: "sym:foo", DisplayName: "Foo", Kind: SCIPSymbolKindFunction, Range: types.Range{Start: types.Position{Line: 1}}}}}
	if err := s.StoreDocument(context.Background(), doc); err != nil {
		t.Fatalf("store: %v", err)
	}
	defs, err := s.GetDefinitionsWithDocuments(context.Background(), "sym:foo")
	if err != nil || len(defs) != 1 {
		t.Fatalf("defs: %v %+v", err, defs)
	}
	syms, err := s.SearchSymbols(context.Background(), "Foo", 10)
	if err != nil || len(syms) == 0 {
		t.Fatalf("search: %v %+v", err, syms)
	}
	_ = s.GetIndexStats()
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "simple_cache.json")); err != nil {
		t.Fatalf("no disk file: %v", err)
	}
	s2, _ := NewSimpleSCIPStorage(cfg)
	if err := s2.Start(context.Background()); err != nil {
		t.Fatalf("start2: %v", err)
	}
	defer s2.Stop(context.Background())
	defs2, _ := s2.GetDefinitionsWithDocuments(context.Background(), "sym:foo")
	if len(defs2) != 1 {
		t.Fatalf("persisted defs not found")
	}
}
