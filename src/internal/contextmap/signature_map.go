package contextmap

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils/lspconv"
)

// ErrNoSymbols indicates that no symbol data was discovered in the backing store.
var ErrNoSymbols = errors.New("contextmap: no indexed symbols available")

// DocumentStore provides read-only access to indexed SCIP documents.
type DocumentStore interface {
	ListDocuments(ctx context.Context) ([]string, error)
	GetDocument(ctx context.Context, uri string) (*scip.SCIPDocument, error)
}

// Node represents a hierarchical symbol entry.
type Node struct {
	Name           string  `json:"name,omitempty"`
	Signature      string  `json:"signature,omitempty"`
	StartLine      int     `json:"start_line"`
	StartCharacter int     `json:"start_char"`
	EndLine        int     `json:"end_line"`
	EndCharacter   int     `json:"end_char"`
	Kind           string  `json:"kind"`
	Children       []*Node `json:"children,omitempty"`
}

// SignatureMap aggregates symbol hierarchy per file.
type SignatureMap struct {
	Files        map[string][]*Node
	TotalSymbols int
}

// BuildSignatureMap constructs a signature map from the provided document store.
func BuildSignatureMap(ctx context.Context, store DocumentStore, uriToPath func(string) string) (*SignatureMap, error) {
	if store == nil {
		return nil, fmt.Errorf("contextmap: document store is required")
	}
	if uriToPath == nil {
		uriToPath = func(uri string) string { return uri }
	}

	uris, err := store.ListDocuments(ctx)
	if err != nil {
		return nil, fmt.Errorf("contextmap: list documents: %w", err)
	}

	files := make(map[string][]*Node)
	total := 0

	for _, uri := range uris {
		doc, err := store.GetDocument(ctx, uri)
		if err != nil || doc == nil {
			continue
		}

		path := uriToPath(uri)
		if path == "" {
			path = uri
		}

		for _, info := range doc.SymbolInformation {
			name := strings.TrimSpace(info.DisplayName)
			signature := strings.TrimSpace(info.SignatureDocumentation.Text)
			if name == "" && signature == "" {
				continue
			}

			node := &Node{
				Name:           name,
				Signature:      signature,
				StartLine:      int(info.Range.Start.Line),
				StartCharacter: int(info.Range.Start.Character),
				EndLine:        int(info.Range.End.Line),
				EndCharacter:   int(info.Range.End.Character),
				Kind:           lspconv.SCIPSymbolKindToString(info.Kind, lspconv.StyleLowercase),
			}
			files[path] = append(files[path], node)
			total++
		}
	}

	if total == 0 {
		return nil, ErrNoSymbols
	}

	for path, nodes := range files {
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].StartLine != nodes[j].StartLine {
				return nodes[i].StartLine < nodes[j].StartLine
			}
			if nodes[i].StartCharacter != nodes[j].StartCharacter {
				return nodes[i].StartCharacter < nodes[j].StartCharacter
			}
			if nodes[i].EndLine != nodes[j].EndLine {
				return nodes[i].EndLine < nodes[j].EndLine
			}
			if nodes[i].EndCharacter != nodes[j].EndCharacter {
				return nodes[i].EndCharacter < nodes[j].EndCharacter
			}
			return nodes[i].Name < nodes[j].Name
		})
		files[path] = buildTree(nodes)
	}

	return &SignatureMap{
		Files:        files,
		TotalSymbols: total,
	}, nil
}

// WriteText renders the signature map to the given writer.
func (m *SignatureMap) WriteText(w io.Writer, generatedAt time.Time) error {
	if nullSignatureMap(m) {
		return fmt.Errorf("contextmap: signature map is nil")
	}
	if w == nil {
		return fmt.Errorf("contextmap: writer is nil")
	}

	buf := bufio.NewWriter(w)
	_, err := fmt.Fprintf(buf, "# context signature map\n# generated: %s\n\n", generatedAt.Format(time.RFC3339))
	if err != nil {
		return err
	}

	files := make([]string, 0, len(m.Files))
	for path := range m.Files {
		files = append(files, path)
	}
	sort.Strings(files)

	for _, path := range files {
		if _, err := fmt.Fprintf(buf, "FILE: %s\n", path); err != nil {
			return err
		}
		writeNodes(buf, m.Files[path], 0)
		if _, err := fmt.Fprintln(buf); err != nil {
			return err
		}
	}

	return buf.Flush()
}

// ToJSON marshals the signature map to JSON.
func (m *SignatureMap) ToJSON(generatedAt time.Time) ([]byte, error) {
	if nullSignatureMap(m) {
		return nil, fmt.Errorf("contextmap: signature map is nil")
	}

	files := make(map[string][]*Node, len(m.Files))
	for path, nodes := range m.Files {
		files[path] = cloneNodes(nodes)
	}

	payload := struct {
		Generated    string                 `json:"generated"`
		Files        map[string][]*Node     `json:"files"`
		TotalSymbols int                    `json:"total_symbols"`
		Metadata     map[string]interface{} `json:"metadata,omitempty"`
	}{
		Generated:    generatedAt.Format(time.RFC3339),
		Files:        files,
		TotalSymbols: m.TotalSymbols,
	}

	return json.Marshal(payload)
}

func nullSignatureMap(m *SignatureMap) bool {
	return m == nil || m.Files == nil
}

func containsRange(parent, child *Node) bool {
	if child.StartLine < parent.StartLine || (child.StartLine == parent.StartLine && child.StartCharacter < parent.StartCharacter) {
		return false
	}
	if child.EndLine > parent.EndLine || (child.EndLine == parent.EndLine && child.EndCharacter > parent.EndCharacter) {
		return false
	}
	return true
}

func buildTree(nodes []*Node) []*Node {
	var roots []*Node
	var stack []*Node

	for _, node := range nodes {
		for len(stack) > 0 && !containsRange(stack[len(stack)-1], node) {
			stack = stack[:len(stack)-1]
		}

		if len(stack) == 0 {
			roots = append(roots, node)
		} else {
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, node)
		}

		stack = append(stack, node)
	}

	return roots
}

func writeNodes(w *bufio.Writer, nodes []*Node, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, node := range nodes {
		if node.Signature != "" {
			_, _ = fmt.Fprintf(w, "%s- %s: %s\n", indent, node.Name, node.Signature)
		} else if node.Name != "" {
			_, _ = fmt.Fprintf(w, "%s- %s\n", indent, node.Name)
		} else {
			_, _ = fmt.Fprintf(w, "%s- (%s)\n", indent, node.Kind)
		}
		if len(node.Children) > 0 {
			writeNodes(w, node.Children, depth+1)
		}
	}
}

func cloneNodes(nodes []*Node) []*Node {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		cloned := *node
		cloned.Children = cloneNodes(node.Children)
		out = append(out, &cloned)
	}
	return out
}
