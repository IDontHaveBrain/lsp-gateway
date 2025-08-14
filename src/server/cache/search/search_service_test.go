package search

import (
	"context"
	"sync"
	"testing"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

type fakeStorage struct {
	sym  scip.SCIPSymbolInformation
	def  scip.OccurrenceWithDocument
	refs []scip.OccurrenceWithDocument
}

func (f *fakeStorage) SearchSymbols(ctx context.Context, pattern string, maxResults int) ([]scip.SCIPSymbolInformation, error) {
	if pattern == f.sym.DisplayName {
		return []scip.SCIPSymbolInformation{f.sym}, nil
	}
	return []scip.SCIPSymbolInformation{}, nil
}
func (f *fakeStorage) GetDefinitions(ctx context.Context, symbolID string) ([]scip.SCIPOccurrence, error) {
	return nil, nil
}
func (f *fakeStorage) GetDefinitionsWithDocuments(ctx context.Context, symbolID string) ([]scip.OccurrenceWithDocument, error) {
	if symbolID == f.sym.Symbol {
		return []scip.OccurrenceWithDocument{f.def}, nil
	}
	return []scip.OccurrenceWithDocument{}, nil
}
func (f *fakeStorage) GetReferences(ctx context.Context, symbolID string) ([]scip.SCIPOccurrence, error) {
	return nil, nil
}
func (f *fakeStorage) GetReferencesWithDocuments(ctx context.Context, symbolID string) ([]scip.OccurrenceWithDocument, error) {
	if symbolID == f.sym.Symbol {
		return f.refs, nil
	}
	return []scip.OccurrenceWithDocument{}, nil
}
func (f *fakeStorage) GetOccurrences(ctx context.Context, symbolID string) ([]scip.SCIPOccurrence, error) {
	return nil, nil
}
func (f *fakeStorage) GetOccurrencesWithDocuments(ctx context.Context, symbolID string) ([]scip.OccurrenceWithDocument, error) {
	return nil, nil
}
func (f *fakeStorage) GetIndexStats() *scip.IndexStats                     { return &scip.IndexStats{} }
func (f *fakeStorage) ListDocuments(ctx context.Context) ([]string, error) { return nil, nil }
func (f *fakeStorage) GetDocument(ctx context.Context, uri string) (*scip.SCIPDocument, error) {
	return nil, nil
}

func TestSearchService_DefRefSymWorkspace(t *testing.T) {
	fs := &fakeStorage{sym: scip.SCIPSymbolInformation{Symbol: "sym1", DisplayName: "Foo", Kind: scip.SCIPSymbolKindFunction}, def: scip.OccurrenceWithDocument{SCIPOccurrence: scip.SCIPOccurrence{Range: types.Range{Start: types.Position{Line: 1}}}, DocumentURI: "file:///p.go"}, refs: []scip.OccurrenceWithDocument{{SCIPOccurrence: scip.SCIPOccurrence{Range: types.Range{Start: types.Position{Line: 2}}}, DocumentURI: "file:///p.go"}, {SCIPOccurrence: scip.SCIPOccurrence{Range: types.Range{Start: types.Position{Line: 3}}}, DocumentURI: "file:///q.go"}}}
	svc := NewSearchService(&SearchServiceConfig{Storage: fs, Enabled: true, IndexMutex: &sync.RWMutex{}, MatchFilePatternFn: func(uri, pattern string) bool { return true }, BuildOccurrenceInfoFn: func(occ *scip.SCIPOccurrence, docURI string) interface{} {
		return map[string]interface{}{"uri": docURI, "line": occ.Range.Start.Line}
	}, FormatSymbolDetailFn: func(si *scip.SCIPSymbolInformation) string { return si.DisplayName }})
	ctx := context.Background()
	defRes, err := svc.ExecuteDefinitionSearch(&SearchRequest{Type: SearchTypeDefinition, SymbolName: "Foo", Context: ctx})
	if err != nil || defRes.Total != 1 {
		t.Fatalf("definition failed: %+v %v", defRes, err)
	}
	refRes, err := svc.ExecuteReferenceSearch(&SearchRequest{Type: SearchTypeReference, SymbolName: "Foo", Context: ctx, MaxResults: 10})
	if err != nil || refRes.Total != 2 {
		t.Fatalf("reference failed: %+v %v", refRes, err)
	}
	symRes, err := svc.ExecuteSymbolSearch(&SearchRequest{Type: SearchTypeSymbol, SymbolName: "Foo", Context: ctx, MaxResults: 1})
	if err != nil || symRes.Total != 1 {
		t.Fatalf("symbol failed: %+v %v", symRes, err)
	}
	wsRes, err := svc.ExecuteWorkspaceSearch(&SearchRequest{Type: SearchTypeWorkspace, SymbolName: "Foo", Context: ctx, MaxResults: 5})
	if err != nil || wsRes.Total != 1 {
		t.Fatalf("workspace failed: %+v %v", wsRes, err)
	}
}
