package contextmap

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

type fakeStore struct {
	docs map[string]*scip.SCIPDocument
	err  error
}

func (f *fakeStore) ListDocuments(ctx context.Context) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	uris := make([]string, 0, len(f.docs))
	for uri := range f.docs {
		uris = append(uris, uri)
	}
	return uris, nil
}

func (f *fakeStore) GetDocument(ctx context.Context, uri string) (*scip.SCIPDocument, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.docs[uri], nil
}

func TestBuildSignatureMap(t *testing.T) {
	store := &fakeStore{
		docs: map[string]*scip.SCIPDocument{
			"file:///foo.go": {
				URI: "file:///foo.go",
				SymbolInformation: []scip.SCIPSymbolInformation{
					{
						DisplayName: "main",
						Kind:        scip.SCIPSymbolKindFunction,
						Range: types.Range{
							Start: types.Position{Line: 0, Character: 0},
							End:   types.Position{Line: 10, Character: 1},
						},
						SignatureDocumentation: scip.SCIPSignatureDocumentation{Text: "func main()"},
					},
					{
						DisplayName: "helper",
						Kind:        scip.SCIPSymbolKindFunction,
						Range: types.Range{
							Start: types.Position{Line: 2, Character: 1},
							End:   types.Position{Line: 5, Character: 2},
						},
						SignatureDocumentation: scip.SCIPSignatureDocumentation{Text: "func helper() string"},
					},
				},
			},
			"file:///bar.go": {
				URI: "file:///bar.go",
				SymbolInformation: []scip.SCIPSymbolInformation{
					{
						DisplayName: "Widget",
						Kind:        scip.SCIPSymbolKindStruct,
						Range: types.Range{
							Start: types.Position{Line: 0, Character: 0},
							End:   types.Position{Line: 20, Character: 0},
						},
						SignatureDocumentation: scip.SCIPSignatureDocumentation{Text: "type Widget struct"},
					},
				},
			},
		},
	}

	convert := func(uri string) string {
		return strings.TrimPrefix(uri, "file://")
	}

	m, err := BuildSignatureMap(context.Background(), store, convert)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, 3, m.TotalSymbols)

	require.ElementsMatch(t, []string{"/bar.go", "/foo.go"}, keys(m.Files))

	fooNodes := m.Files["/foo.go"]
	require.Len(t, fooNodes, 1)
	require.Equal(t, "main", fooNodes[0].Name)
	require.Len(t, fooNodes[0].Children, 1)
	require.Equal(t, "helper", fooNodes[0].Children[0].Name)

	barNodes := m.Files["/bar.go"]
	require.Len(t, barNodes, 1)
	require.Equal(t, "Widget", barNodes[0].Name)
}

func TestWriteText(t *testing.T) {
	m := &SignatureMap{
		Files: map[string][]*Node{
			"/foo.go": {
				{
					Name:           "main",
					Signature:      "func main()",
					StartLine:      0,
					StartCharacter: 0,
					EndLine:        10,
					EndCharacter:   1,
					Kind:           "function",
					Children: []*Node{
						{
							Name:           "helper",
							Signature:      "func helper() string",
							StartLine:      2,
							StartCharacter: 1,
							EndLine:        5,
							EndCharacter:   2,
							Kind:           "function",
						},
					},
				},
			},
		},
		TotalSymbols: 2,
	}

	var buf bytes.Buffer
	ts := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	err := m.WriteText(&buf, ts)
	require.NoError(t, err)

	expected := "# context signature map\n" +
		"# generated: 2024-05-01T12:00:00Z\n\n" +
		"FILE: /foo.go\n" +
		"- main: func main()\n" +
		"  - helper: func helper() string\n\n"

	require.Equal(t, expected, buf.String())
}

func TestToJSON(t *testing.T) {
	m := &SignatureMap{
		Files:        map[string][]*Node{},
		TotalSymbols: 0,
	}

	ts := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	m.Files["/foo.go"] = []*Node{
		{
			Name:      "main",
			Kind:      "function",
			StartLine: 0,
			EndLine:   1,
		},
	}
	m.TotalSymbols = 1

	data, err := m.ToJSON(ts)
	require.NoError(t, err)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &payload))
	require.Equal(t, "2024-05-01T12:00:00Z", payload["generated"])
	require.Equal(t, float64(1), payload["total_symbols"])

	filesRaw := payload["files"].(map[string]interface{})
	require.Contains(t, filesRaw, "/foo.go")
}

func TestBuildSignatureMapNoSymbols(t *testing.T) {
	store := &fakeStore{
		docs: map[string]*scip.SCIPDocument{
			"file:///empty.go": {URI: "file:///empty.go"},
		},
	}

	_, err := BuildSignatureMap(context.Background(), store, func(uri string) string { return uri })
	require.ErrorIs(t, err, ErrNoSymbols)
}

func keys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
