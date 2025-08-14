package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
	clicommon "lsp-gateway/src/cli/common"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils"
	"lsp-gateway/src/utils/lspconv"
)

type contextNode struct {
	Name      string         `json:"name,omitempty"`
	Signature string         `json:"signature,omitempty"`
	StartL    int            `json:"start_line"`
	StartC    int            `json:"start_char"`
	EndL      int            `json:"end_line"`
	EndC      int            `json:"end_char"`
	Kind      string         `json:"kind"`
	Children  []*contextNode `json:"children,omitempty"`
}

type SymbolInfo struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Signature  string   `json:"signature,omitempty"`
	File       string   `json:"file"`
	Line       int      `json:"line"`
	References []string `json:"references,omitempty"`
}

type FileRelation struct {
	File         string   `json:"file"`
	Imports      []string `json:"imports,omitempty"`
	ImportedBy   []string `json:"imported_by,omitempty"`
	References   []string `json:"references,omitempty"`
	ReferencedBy []string `json:"referenced_by,omitempty"`
	Symbols      int      `json:"symbols_count"`
}

type DependencyInfo struct {
	File        string         `json:"file"`
	Exports     []ExportSymbol `json:"exports"`
	SymbolCount int            `json:"symbol_count"`
}

type ExportSymbol struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// GenerateContextSignatureMap creates a .txt signature map from indexed data
func GenerateContextSignatureMap(configPath, outputPath string) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	if cmdCtx.Cache == nil {
		return fmt.Errorf("cache unavailable")
	}

	if _, err := cmdCtx.CheckCacheHealth(); err != nil {
		return fmt.Errorf("cache health check failed: %w", err)
	}

	storage := cmdCtx.Cache.GetSCIPStorage()
	if storage == nil {
		return fmt.Errorf("scip storage unavailable")
	}

	// Ensure output dir exists
	if outputPath == "" {
		outputPath = "context-signature-map.txt"
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil && filepath.Dir(outputPath) != "." {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Collect entries grouped by file
	grouped := make(map[string][]*contextNode)

	// List and iterate documents
	uris, err := storage.ListDocuments(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	totalSymbols := 0

	for _, uri := range uris {
		doc, err := storage.GetDocument(context.Background(), uri)
		if err != nil || doc == nil {
			continue
		}

		file := utils.URIToFilePathCached(uri)
		if file == "" {
			file = uri
		}

		for _, si := range doc.SymbolInformation {
			name := strings.TrimSpace(si.DisplayName)
			sig := strings.TrimSpace(si.SignatureDocumentation.Text)
			if name == "" && sig == "" {
				continue
			}
			nd := &contextNode{
				Name:      name,
				Signature: sig,
				StartL:    int(si.Range.Start.Line),
				StartC:    int(si.Range.Start.Character),
				EndL:      int(si.Range.End.Line),
				EndC:      int(si.Range.End.Character),
				Kind:      lspconv.SCIPSymbolKindToString(si.Kind, lspconv.StyleLowercase),
				Children:  nil,
			}
			grouped[file] = append(grouped[file], nd)
			totalSymbols++
		}
	}

	if totalSymbols == 0 {
		return fmt.Errorf("no indexed symbols found; run 'lsp-gateway cache index' first")
	}

	// Sort files and entries
	files := make([]string, 0, len(grouped))
	for f := range grouped {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, f := range files {
		list := grouped[f]
		sort.Slice(list, func(i, j int) bool {
			if list[i].StartL == list[j].StartL {
				if list[i].StartC == list[j].StartC {
					if list[i].EndL == list[j].EndL {
						if list[i].EndC == list[j].EndC {
							return list[i].Name < list[j].Name
						}
						return list[i].EndC < list[j].EndC
					}
					return list[i].EndL < list[j].EndL
				}
				return list[i].StartC < list[j].StartC
			}
			return list[i].StartL < list[j].StartL
		})
		roots := buildTree(list)
		grouped[f] = roots
	}

	// Write output
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	w := bufio.NewWriter(out)
	defer w.Flush()

	// Header
	fmt.Fprintf(w, "# context signature map\n")
	fmt.Fprintf(w, "# generated: %s\n\n", time.Now().Format(time.RFC3339))

	for _, f := range files {
		fmt.Fprintf(w, "FILE: %s\n", f)
		roots := grouped[f]
		writeNodes(w, roots, 0)
		fmt.Fprintln(w)
	}

	common.CLILogger.Info("Signature map written: %s (%d symbols)", outputPath, totalSymbols)
	return nil
}

func runContextCmd(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func runContextMapCmd(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("please provide exactly one file path")
	}
	return PrintReferencedFilesCode(configPath, args[0])
}

func PrintReferencedFilesCode(configPath string, inputFile string) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	absPath, err := filepath.Abs(inputFile)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}
	if st, err := os.Stat(absPath); err != nil || st.IsDir() {
		return fmt.Errorf("file not found or is a directory: %s", absPath)
	}

	uri := utils.FilePathToURICached(absPath)
	// Determine project root for filtering (prefer repo root with go.mod/.git)
	workspaceRoot := ""
	fileExists := func(p string) bool {
		if st, err := os.Stat(p); err == nil && st.IsDir() || (err == nil && !st.IsDir()) {
			return true
		}
		return false
	}
	root := filepath.Dir(absPath)
	for {
		if fileExists(filepath.Join(root, ".git")) || fileExists(filepath.Join(root, "go.mod")) {
			workspaceRoot = root
			break
		}
		parent := filepath.Dir(root)
		if parent == root { // reached filesystem root
			break
		}
		root = parent
	}
	if workspaceRoot == "" {
		if wd, err := os.Getwd(); err == nil {
			workspaceRoot = wd
		}
	}
	hostRootToken := string(os.PathSeparator) + "work" + string(os.PathSeparator) + "lsp-gateway" + string(os.PathSeparator)

	var storage scip.SCIPDocumentStorage
	if cmdCtx.Cache != nil {
		storage = cmdCtx.Cache.GetSCIPStorage()
	}
	if storage == nil {
		return fmt.Errorf("index unavailable; run 'lsp-gateway cache index' and try again")
	}

	type snippet struct {
		file   string
		name   string
		startL int
		startC int
		endL   int
		endC   int
	}

	// Collect referenced symbol definitions -> snippets
	doc, err := storage.GetDocument(context.Background(), uri)
	if err != nil || doc == nil {
		return fmt.Errorf("document not indexed: %s", absPath)
	}

	// Map: file -> list of snippets
	snippetMap := make(map[string][]snippet)
	// Dedup by (file, symbol)
	seen := make(map[string]bool)

	// Range helpers and language-agnostic expansion to full symbol range
	rangeContains := func(outer, inner types.Range) bool {
		if inner.Start.Line < outer.Start.Line || (inner.Start.Line == outer.Start.Line && inner.Start.Character < outer.Start.Character) {
			return false
		}
		if inner.End.Line > outer.End.Line || (inner.End.Line == outer.End.Line && inner.End.Character > outer.End.Character) {
			return false
		}
		return true
	}
	rangeSize := func(r types.Range) int64 {
		return int64(r.End.Line-r.Start.Line)*100000 + int64(r.End.Character-r.Start.Character)
	}
	expandRangeToSymbol := func(defPath string, base types.Range, name string) types.Range {
		best := base
		defURI := utils.FilePathToURICached(defPath)
		if storage != nil && defURI != "" {
			if ddoc, err := storage.GetDocument(context.Background(), defURI); err == nil && ddoc != nil {
				for _, si := range ddoc.SymbolInformation {
					if rangeContains(si.Range, best) {
						if name == "" || strings.EqualFold(si.DisplayName, name) {
							if rangeSize(si.Range) >= rangeSize(best) {
								best = si.Range
							}
						} else if rangeSize(si.Range) > rangeSize(best) {
							best = si.Range
						}
					}
				}
			}
		}
		if cmdCtx.Manager != nil && defURI != "" {
			if res, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentDocumentSymbol, &lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: defURI}}); err == nil && res != nil {
				if syms, err2 := parseDocumentSymbolsResult(res); err2 == nil {
					var walk func([]lsp.DocumentSymbol)
					walk = func(list []lsp.DocumentSymbol) {
						for _, ds := range list {
							if rangeContains(ds.Range, best) && rangeSize(ds.Range) >= rangeSize(best) {
								if name == "" || strings.EqualFold(ds.Name, name) {
									best = ds.Range
								} else if rangeSize(ds.Range) > rangeSize(best) {
									best = ds.Range
								}
							}
							if len(ds.Children) > 0 {
								tmp := make([]lsp.DocumentSymbol, 0, len(ds.Children))
								for _, ch := range ds.Children {
									if ch != nil {
										tmp = append(tmp, *ch)
									}
								}
								if len(tmp) > 0 {
									walk(tmp)
								}
							}
						}
					}
					walk(syms)
				}
			}
		}
		return best
	}

	for _, occ := range doc.Occurrences {
		if occ.Symbol == "" {
			continue
		}
		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) || occ.SymbolRoles.HasRole(types.SymbolRoleGenerated) {
			continue
		}

		defs, err := storage.GetDefinitionsWithDocuments(context.Background(), occ.Symbol)
		if err == nil && len(defs) > 0 {
			symInfo, _ := storage.GetSymbolInfo(context.Background(), occ.Symbol)
			for _, d := range defs {
				defPath := utils.URIToFilePathCached(d.DocumentURI)
				if defPath == "" || defPath == absPath {
					continue
				}
				if workspaceRoot != "" {
					rp, _ := filepath.Abs(defPath)
					rr, _ := filepath.Abs(workspaceRoot)
					if (!strings.HasPrefix(rp, rr+string(os.PathSeparator)) && rp != rr) || (hostRootToken != "" && !strings.Contains(rp, hostRootToken)) {
						continue
					}
				}
				key := defPath + "::" + occ.Symbol
				if seen[key] {
					continue
				}
				seen[key] = true

				rng := d.Range
				name := occ.Symbol
				if symInfo != nil {
					rng = symInfo.Range
					if symInfo.DisplayName != "" {
						name = symInfo.DisplayName
					}
				}
				rng = expandRangeToSymbol(defPath, rng, name)

				sn := snippet{
					file:   defPath,
					name:   name,
					startL: int(rng.Start.Line),
					startC: int(rng.Start.Character),
					endL:   int(rng.End.Line),
					endC:   int(rng.End.Character),
				}
				snippetMap[defPath] = append(snippetMap[defPath], sn)
			}
			continue
		}

		if cmdCtx.Manager != nil {
			pos := occ.Range.Start
			defResult, derr := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentDefinition, &lsp.TextDocumentPositionParams{
				TextDocument: lsp.TextDocumentIdentifier{URI: uri},
				Position:     pos,
			})
			if derr == nil && defResult != nil {
				if locs, perr := parseDefinitionResult(defResult); perr == nil {
					for _, loc := range locs {
						defPath := utils.URIToFilePathCached(loc.URI)
						if defPath == "" || defPath == absPath {
							continue
						}
						if workspaceRoot != "" {
							rp, _ := filepath.Abs(defPath)
							rr, _ := filepath.Abs(workspaceRoot)
							if (!strings.HasPrefix(rp, rr+string(os.PathSeparator)) && rp != rr) || (hostRootToken != "" && !strings.Contains(rp, hostRootToken)) {
								continue
							}
						}
						key := defPath + "::" + occ.Symbol
						if seen[key] {
							continue
						}
						seen[key] = true

						name := occ.Symbol
						if si, _ := storage.GetSymbolInfo(context.Background(), occ.Symbol); si != nil && si.DisplayName != "" {
							name = si.DisplayName
						}
						// expand to full symbol range using doc symbols/scip when available
						full := expandRangeToSymbol(defPath, loc.Range, name)
						sn := snippet{
							file:   defPath,
							name:   name,
							startL: int(full.Start.Line),
							startC: int(full.Start.Character),
							endL:   int(full.End.Line),
							endC:   int(full.End.Character),
						}
						snippetMap[defPath] = append(snippetMap[defPath], sn)
					}
				}
			}
		}
	}

	for _, si := range doc.SymbolInformation {
		if si.Symbol == "" {
			continue
		}
		defs, err := storage.GetDefinitionsWithDocuments(context.Background(), si.Symbol)
		if err != nil || len(defs) == 0 {
			continue
		}
		for _, d := range defs {
			defPath := utils.URIToFilePathCached(d.DocumentURI)
			if defPath == "" || defPath == absPath {
				continue
			}
			if workspaceRoot != "" {
				rp, _ := filepath.Abs(defPath)
				rr, _ := filepath.Abs(workspaceRoot)
				if (!strings.HasPrefix(rp, rr+string(os.PathSeparator)) && rp != rr) || (hostRootToken != "" && !strings.Contains(rp, hostRootToken)) {
					continue
				}
			}
			key := defPath + "::" + si.Symbol
			if seen[key] {
				continue
			}
			seen[key] = true
			rng := si.Range
			if rng.Start.Line == 0 && rng.End.Line == 0 {
				rng = d.Range
			}
			name := si.DisplayName
			if name == "" {
				name = si.Symbol
			}
			rng = expandRangeToSymbol(defPath, rng, name)
			sn := snippet{
				file:   defPath,
				name:   name,
				startL: int(rng.Start.Line),
				startC: int(rng.Start.Character),
				endL:   int(rng.End.Line),
				endC:   int(rng.End.Character),
			}
			snippetMap[defPath] = append(snippetMap[defPath], sn)
		}
	}

	// defer empty-check until after LSP fallback scan

	// Defer computing files until after potential LSP fallback augmentation
	var files []string

	// Helper: merge overlapping/adjacent snippets per file
	merge := func(list []snippet) []snippet {
		sort.Slice(list, func(i, j int) bool {
			if list[i].startL == list[j].startL {
				return list[i].startC < list[j].startC
			}
			return list[i].startL < list[j].startL
		})
		var out []snippet
		for _, s := range list {
			if len(out) == 0 {
				out = append(out, s)
				continue
			}
			last := &out[len(out)-1]
			// Overlap check
			if s.startL < last.endL || (s.startL == last.endL && s.startC <= last.endC) {
				// extend end if needed
				if s.endL > last.endL || (s.endL == last.endL && s.endC > last.endC) {
					last.endL = s.endL
					last.endC = s.endC
				}
				continue
			}
			out = append(out, s)
		}
		return out
	}

	// Helper: extract full lines covering the range (print entire code lines)
	extract := func(content []byte, s snippet) string {
		lines := strings.Split(string(content), "\n")
		if s.startL < 0 {
			s.startL = 0
		}
		if s.endL >= len(lines) {
			s.endL = len(lines) - 1
		}
		var b strings.Builder
		for ln := s.startL; ln <= s.endL && ln < len(lines); ln++ {
			b.WriteString(lines[ln])
			if ln < s.endL {
				b.WriteByte('\n')
			}
		}
		return b.String()
	}

	// Fallback LSP scan: walk tokens and resolve definitions when storage misses
    // No token refinement; keep full ranges

	addSnippet := func(path, name string, r types.Range) {
		if path == "" || path == absPath {
			return
		}
		if workspaceRoot != "" {
			rp, _ := filepath.Abs(path)
			rr, _ := filepath.Abs(workspaceRoot)
			if (!strings.HasPrefix(rp, rr+string(os.PathSeparator)) && rp != rr) || (hostRootToken != "" && !strings.Contains(rp, hostRootToken)) {
				return
			}
		}
        // Expand to full symbol range when possible
        r = expandRangeToSymbol(path, r, name)

		key := path + "::" + name + fmt.Sprintf("@%d:%d-%d:%d", r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
		if seen[key] {
			return
		}
		seen[key] = true
		snippetMap[path] = append(snippetMap[path], snippet{
			file:   path,
			name:   name,
			startL: int(r.Start.Line),
			startC: int(r.Start.Character),
			endL:   int(r.End.Line),
			endC:   int(r.End.Character),
		})
	}

	// If we have few or no snippets from storage, probe via LSP
	if cmdCtx.Manager != nil {
		data, _ := os.ReadFile(absPath)
		content := string(data)
		lines := strings.Split(content, "\n")
		for ln, line := range lines {
			// Simple token scan
			i := 0
			for i < len(line) {
				r := rune(line[i])
				if unicode.IsLetter(r) || r == '_' || unicode.IsDigit(r) {
					start := i
					for i < len(line) {
						r2 := rune(line[i])
						if unicode.IsLetter(r2) || unicode.IsDigit(r2) || r2 == '_' {
							i++
							continue
						}
						break
					}
					token := line[start:i]

					// Only consider selector-style references: preceding '.'
					if start == 0 || line[start-1] != '.' {
						continue
					}
					// Query definition at token start
					defRes, derr := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentDefinition, &lsp.DefinitionParams{
						TextDocumentPositionParams: lsp.TextDocumentPositionParams{
							TextDocument: lsp.TextDocumentIdentifier{URI: uri},
							Position:     types.Position{Line: int32(ln), Character: int32(start)},
						},
					})

					if derr == nil && defRes != nil {
						if locs, perr := parseDefinitionResult(defRes); perr == nil {
							for _, loc := range locs {
								defPath := utils.URIToFilePathCached(loc.URI)
								addSnippet(defPath, token, loc.Range)
							}
						} else {
							_ = perr
						}
					} else if derr != nil {
						_ = derr
					}

				} else {
					i++
				}
			}
		}
	}

	// If still empty after LSP fallback, report error
	if len(snippetMap) == 0 {
		return fmt.Errorf("no referenced code found for: %s", absPath)
	}

	// Compute files now (after augmentation) and print snippets grouped by file
	files = make([]string, 0, len(snippetMap))
	for f := range snippetMap {
		files = append(files, f)
	}
	sort.Strings(files)
	// Expand snippet ranges per file using on-demand LSP document symbols
	if cmdCtx.Manager != nil {
		for _, f := range files {
			defURI := utils.FilePathToURICached(f)
			if defURI == "" {
				continue
			}
			if res, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentDocumentSymbol, &lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: defURI}}); err == nil && res != nil {
				if syms, perr := parseDocumentSymbolsResult(res); perr == nil && len(syms) > 0 {
					list := snippetMap[f]
					for i := range list {
						base := types.Range{Start: types.Position{Line: int32(list[i].startL), Character: int32(list[i].startC)}, End: types.Position{Line: int32(list[i].endL), Character: int32(list[i].endC)}}
						best := base
						var walk func([]lsp.DocumentSymbol)
						walk = func(nodes []lsp.DocumentSymbol) {
							for _, ds := range nodes {
								r := ds.Range
								if r.Start.Line <= best.Start.Line && (r.Start.Line < best.Start.Line || r.Start.Character <= best.Start.Character) &&
									(r.End.Line > best.End.Line || (r.End.Line == best.End.Line && r.End.Character >= best.End.Character)) {
									// Prefer larger ranges
									if (r.End.Line-r.Start.Line) > (best.End.Line-best.Start.Line) || ((r.End.Line-r.Start.Line) == (best.End.Line-best.Start.Line) && (r.End.Character-r.Start.Character) > (best.End.Character-best.Start.Character)) {
										best = r
									}
								}
								if len(ds.Children) > 0 {
									tmp := make([]lsp.DocumentSymbol, 0, len(ds.Children))
									for _, ch := range ds.Children {
										if ch != nil {
											tmp = append(tmp, *ch)
										}
									}
									if len(tmp) > 0 {
										walk(tmp)
									}
								}
							}
						}
						// Update expanded range
						list[i].startL = int(best.Start.Line)
						list[i].startC = int(best.Start.Character)
						list[i].endL = int(best.End.Line)
						list[i].endC = int(best.End.Character)
					}
					snippetMap[f] = list
				}
			}
		}
	}

	// Print snippets grouped by file, using indexed content
	for _, f := range files {
		// Skip stdlib dirs
		if strings.Contains(f, string(os.PathSeparator)+".g"+string(os.PathSeparator)+"go"+string(os.PathSeparator)+"src"+string(os.PathSeparator)) {
			continue
		}
		if workspaceRoot != "" {
			rp, _ := filepath.Abs(f)
			rr, _ := filepath.Abs(workspaceRoot)
			if (!strings.HasPrefix(rp, rr+string(os.PathSeparator)) && rp != rr) || (hostRootToken != "" && !strings.Contains(rp, hostRootToken)) {
				continue
			}
		}
		merged := merge(snippetMap[f])
		fmt.Printf("\n# FILE: %s\n", f)
		// Always read from disk to ensure latest content and full lines
		data, derr := os.ReadFile(f)
		if derr != nil {
			fmt.Printf("# (unreadable)\n")
			continue
		}
		for _, sn := range merged {
			fmt.Printf("# SNIPPET %s [%d:%d-%d:%d]\n", sn.name, sn.startL, sn.startC, sn.endL, sn.endC)
            text := extract(data, sn)
            fmt.Print(text)
			fmt.Print("\n")
		}
	}

	return nil
}

func parseDefinitionResult(result interface{}) ([]types.Location, error) {
	if result == nil {
		return nil, nil
	}
	switch v := result.(type) {
	case types.Location:
		return []types.Location{v}, nil
	case *types.Location:
		if v == nil {
			return nil, nil
		}
		return []types.Location{*v}, nil
	case []types.Location:
		return v, nil
	case []*types.Location:
		out := make([]types.Location, 0, len(v))
		for _, p := range v {
			if p != nil {
				out = append(out, *p)
			}
		}
		return out, nil
	case json.RawMessage:
		if len(v) == 0 || string(v) == "null" {
			return nil, nil
		}
		var arr []types.Location
		if err := json.Unmarshal(v, &arr); err == nil {
			return arr, nil
		}
		var single types.Location
		if err := json.Unmarshal(v, &single); err == nil {
			return []types.Location{single}, nil
		}
		// Try LocationLink[]: { targetUri, targetRange, targetSelectionRange }
        type locationLink struct {
            TargetURI            string      `json:"targetUri"`
            TargetRange          types.Range `json:"targetRange"`
            TargetSelectionRange types.Range `json:"targetSelectionRange"`
        }
        var links []locationLink
        if err := json.Unmarshal(v, &links); err == nil && len(links) > 0 {
            out := make([]types.Location, 0, len(links))
            for _, l := range links {
                // Prefer full targetRange; use selection if full is absent
                rng := l.TargetRange
                if rng.Start.Line == 0 && rng.End.Line == 0 && rng.Start.Character == 0 && rng.End.Character == 0 {
                    rng = l.TargetSelectionRange
                }
                out = append(out, types.Location{URI: l.TargetURI, Range: rng})
            }
            return out, nil
        }
        // Try single LocationLink
        var link locationLink
        if err := json.Unmarshal(v, &link); err == nil && link.TargetURI != "" {
            rng := link.TargetRange
            if rng.Start.Line == 0 && rng.End.Line == 0 && rng.Start.Character == 0 && rng.End.Character == 0 {
                rng = link.TargetSelectionRange
            }
            return []types.Location{{URI: link.TargetURI, Range: rng}}, nil
        }
		return nil, fmt.Errorf("unable to parse definition result")
	case []byte:
		return parseDefinitionResult(json.RawMessage(v))
	case string:
		return parseDefinitionResult(json.RawMessage([]byte(v)))
	default:
		return nil, fmt.Errorf("unsupported definition result type: %T", result)
	}
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func isIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func runContextRelatedCmd(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide at least one file path")
	}
	return FindRelatedFiles(configPath, args, formatJSON, maxDepth)
}

func runContextSymbolsCmd(cmd *cobra.Command, args []string) error {
	files := targetFiles
	if len(args) > 0 {
		files = args
	}
	return ExtractSymbols(configPath, files, formatJSON, includeRefs)
}

func runContextDependenciesCmd(cmd *cobra.Command, args []string) error {
	files := targetFiles
	if len(args) > 0 {
		files = args
	}
	return AnalyzeDependencies(configPath, files, formatJSON)
}

func containsRange(p, c *contextNode) bool {
	if c.StartL < p.StartL || (c.StartL == p.StartL && c.StartC < p.StartC) {
		return false
	}
	if c.EndL > p.EndL || (c.EndL == p.EndL && c.EndC > p.EndC) {
		return false
	}
	return true
}

func buildTree(nodes []*contextNode) []*contextNode {
	var roots []*contextNode
	var stack []*contextNode
	for _, n := range nodes {
		for len(stack) > 0 && !containsRange(stack[len(stack)-1], n) {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, n)
		}
		stack = append(stack, n)
	}
	return roots
}

func writeNodes(w *bufio.Writer, nodes []*contextNode, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, n := range nodes {
		label := n.Name
		if n.Signature != "" {
			fmt.Fprintf(w, "%s- %s: %s\n", indent, label, n.Signature)
		} else {
			fmt.Fprintf(w, "%s- %s\n", indent, label)
		}
		if len(n.Children) > 0 {
			writeNodes(w, n.Children, depth+1)
		}
	}
}

// parseDocumentSymbolsResult normalizes an LSP documentSymbol result into []lsp.DocumentSymbol
func parseDocumentSymbolsResult(result interface{}) ([]lsp.DocumentSymbol, error) {
	if result == nil {
		return nil, nil
	}
	switch v := result.(type) {
	case []lsp.DocumentSymbol:
		return v, nil
	case []*lsp.DocumentSymbol:
		out := make([]lsp.DocumentSymbol, 0, len(v))
		for _, p := range v {
			if p != nil {
				out = append(out, *p)
			}
		}
		return out, nil
	case json.RawMessage:
		if len(v) == 0 || string(v) == "null" {
			return nil, nil
		}
		var ds []lsp.DocumentSymbol
		if err := json.Unmarshal(v, &ds); err == nil {
			return ds, nil
		}
		var si []types.SymbolInformation
		if err := json.Unmarshal(v, &si); err == nil {
			out := make([]lsp.DocumentSymbol, 0, len(si))
			for _, s := range si {
				out = append(out, lsp.DocumentSymbol{
					Name:           s.Name,
					Kind:           s.Kind,
					Range:          s.Location.Range,
					SelectionRange: s.Location.Range,
				})
			}
			return out, nil
		}
		return nil, fmt.Errorf("unable to parse document symbols result")
	case []byte:
		return parseDocumentSymbolsResult(json.RawMessage(v))
	case string:
		return parseDocumentSymbolsResult(json.RawMessage([]byte(v)))
	default:
		return nil, fmt.Errorf("unsupported document symbols result type: %T", result)
	}
}

// parseReferencesResult normalizes an LSP references result into []types.Location
func parseReferencesResult(result interface{}) ([]types.Location, error) {
	if result == nil {
		return nil, nil
	}
	switch v := result.(type) {
	case []types.Location:
		return v, nil
	case []*types.Location:
		out := make([]types.Location, 0, len(v))
		for _, p := range v {
			if p != nil {
				out = append(out, *p)
			}
		}
		return out, nil
	case json.RawMessage:
		if len(v) == 0 || string(v) == "null" {
			return nil, nil
		}
		var locs []types.Location
		if err := json.Unmarshal(v, &locs); err == nil {
			return locs, nil
		}
		return nil, fmt.Errorf("unable to parse references result")
	case []byte:
		return parseReferencesResult(json.RawMessage(v))
	case string:
		return parseReferencesResult(json.RawMessage([]byte(v)))
	default:
		return nil, fmt.Errorf("unsupported references result type: %T", result)
	}
}

// GenerateContextSignatureMapJSON creates a JSON signature map from indexed data
func GenerateContextSignatureMapJSON(configPath, outputPath string) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	if cmdCtx.Cache == nil {
		return fmt.Errorf("cache unavailable")
	}

	storage := cmdCtx.Cache.GetSCIPStorage()
	if storage == nil {
		return fmt.Errorf("scip storage unavailable")
	}

	type JSONOutput struct {
		Generated string                    `json:"generated"`
		Files     map[string][]*contextNode `json:"files"`
		Total     int                       `json:"total_symbols"`
	}

	output := JSONOutput{
		Generated: time.Now().Format(time.RFC3339),
		Files:     make(map[string][]*contextNode),
	}

	uris, err := storage.ListDocuments(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	for _, uri := range uris {
		doc, err := storage.GetDocument(context.Background(), uri)
		if err != nil || doc == nil {
			continue
		}

		file := utils.URIToFilePathCached(uri)
		var nodes []*contextNode

		for _, si := range doc.SymbolInformation {
			name := strings.TrimSpace(si.DisplayName)
			sig := strings.TrimSpace(si.SignatureDocumentation.Text)
			if name == "" && sig == "" {
				continue
			}
			nd := &contextNode{
				Name:      name,
				Signature: sig,
				StartL:    int(si.Range.Start.Line),
				StartC:    int(si.Range.Start.Character),
				EndL:      int(si.Range.End.Line),
				EndC:      int(si.Range.End.Character),
				Kind:      lspconv.SCIPSymbolKindToString(si.Kind, lspconv.StyleLowercase),
			}
			nodes = append(nodes, nd)
			output.Total++
		}

		if len(nodes) > 0 {
			sort.Slice(nodes, func(i, j int) bool {
				if nodes[i].StartL != nodes[j].StartL {
					return nodes[i].StartL < nodes[j].StartL
				}
				return nodes[i].StartC < nodes[j].StartC
			})
			output.Files[file] = buildTree(nodes)
		}
	}

	if output.Total == 0 {
		return fmt.Errorf("no indexed symbols found; run 'lsp-gateway cache index' first")
	}

	if outputPath == "" {
		outputPath = "context-signature-map.json"
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	common.CLILogger.Info("JSON signature map written: %s (%d symbols)", outputPath, output.Total)
	return nil
}

// FindRelatedFiles finds files related to the given input files
func FindRelatedFiles(configPath string, inputFiles []string, formatJSON bool, maxDepth int) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	var storage scip.SCIPDocumentStorage
	if cmdCtx.Cache != nil {
		storage = cmdCtx.Cache.GetSCIPStorage()
	}

	relatedFiles := make(map[string]*FileRelation)
	visited := make(map[string]bool)

	var targetFiles []string
	for _, f := range inputFiles {
		absPath, err := filepath.Abs(f)
		if err == nil {
			targetFiles = append(targetFiles, absPath)
			visited[absPath] = true
		}
	}

	for depth := 0; depth < maxDepth; depth++ {
		var newFiles []string
		for _, file := range targetFiles {
			uri := utils.FilePathToURICached(file)

			symbolCount := 0
			refSet := make(map[string]bool)
			refBySet := make(map[string]bool)

			if storage != nil {
				if doc, err := storage.GetDocument(context.Background(), uri); err == nil && doc != nil {
					symbolCount = len(doc.SymbolInformation)

					defSymbols := make(map[string]bool)
					for _, occ := range doc.Occurrences {
						if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
							defSymbols[occ.Symbol] = true
						}
						if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) || occ.SymbolRoles.HasRole(types.SymbolRoleImport) || occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
							if defs, err := storage.GetDefinitionsWithDocuments(context.Background(), occ.Symbol); err == nil {
								for _, d := range defs {
									defURI := utils.URIToFilePathCached(d.DocumentURI)
									if defURI != "" && defURI != file {
										refSet[defURI] = true
									}
								}
							}
						}
					}

					for sym := range defSymbols {
						if refs, err := storage.GetReferencesWithDocuments(context.Background(), sym); err == nil {
							for _, r := range refs {
								refURI := utils.URIToFilePathCached(r.DocumentURI)
								if refURI != "" && refURI != file {
									refBySet[refURI] = true
									if !visited[refURI] {
										newFiles = append(newFiles, refURI)
										visited[refURI] = true
									}
								}
							}
						}
					}
				}
			} else {
				// Fallback to LSP if no storage available
				result, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentDocumentSymbol, &lsp.DocumentSymbolParams{
					TextDocument: lsp.TextDocumentIdentifier{URI: uri},
				})
				if err == nil && result != nil {
					if symbols, err := parseDocumentSymbolsResult(result); err == nil && len(symbols) > 0 {
						symbolCount = countSymbols(symbols)
					}
				}

				if content, err := os.ReadFile(file); err == nil {
					lines := strings.Split(string(content), "\n")
					for lineNum := range lines {
						refsResult, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentReferences, &lsp.ReferenceParams{
							TextDocumentPositionParams: lsp.TextDocumentPositionParams{
								TextDocument: lsp.TextDocumentIdentifier{URI: uri},
								Position:     types.Position{Line: int32(lineNum), Character: 0},
							},
							Context: lsp.ReferenceContext{IncludeDeclaration: true},
						})
						if err == nil && refsResult != nil {
							if refs, err := parseReferencesResult(refsResult); err == nil {
								for _, ref := range refs {
									refFile := utils.URIToFilePathCached(ref.URI)
									if refFile != file && !visited[refFile] {
										newFiles = append(newFiles, refFile)
										visited[refFile] = true
										refSet[refFile] = true
										refBySet[refFile] = true
									}
								}
							}
						}
					}
				}
			}

			if relation, ok := relatedFiles[file]; ok {
				relation.Symbols += symbolCount
			} else {
				relatedFiles[file] = &FileRelation{File: file, Symbols: symbolCount}
			}

			if rel := relatedFiles[file]; rel != nil {
				for f := range refSet {
					rel.References = append(rel.References, f)
				}
				for f := range refBySet {
					rel.ReferencedBy = append(rel.ReferencedBy, f)
				}
			}
		}

		if len(newFiles) == 0 {
			break
		}
		targetFiles = newFiles
	}

	if formatJSON {
		data, err := json.MarshalIndent(relatedFiles, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Println("# Related Files Analysis")
		fmt.Printf("# Found %d related files\n\n", len(relatedFiles))
		for _, rel := range relatedFiles {
			fmt.Printf("File: %s\n", rel.File)
			if rel.Symbols > 0 {
				fmt.Printf("  Symbols: %d\n", rel.Symbols)
			}
			if len(rel.References) > 0 {
				fmt.Printf("  References: %s\n", strings.Join(rel.References, ", "))
			}
			if len(rel.ReferencedBy) > 0 {
				fmt.Printf("  Referenced by: %s\n", strings.Join(rel.ReferencedBy, ", "))
			}
			fmt.Println()
		}
	}

	return nil
}

// ExtractSymbols extracts symbols from specified files
func ExtractSymbols(configPath string, files []string, formatJSON bool, includeRefs bool) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	var symbols []SymbolInfo

	var storage scip.SCIPDocumentStorage
	if cmdCtx.Cache != nil {
		storage = cmdCtx.Cache.GetSCIPStorage()
	}

	if len(files) == 0 && storage != nil {
		if uris, _ := storage.ListDocuments(context.Background()); len(uris) > 0 {
			for _, uri := range uris {
				files = append(files, utils.URIToFilePathCached(uri))
			}
		}
	}

	for _, file := range files {
		absPath, _ := filepath.Abs(file)
		uri := utils.FilePathToURICached(absPath)

		if storage != nil {
			if doc, err := storage.GetDocument(context.Background(), uri); err == nil && doc != nil {
				for _, si := range doc.SymbolInformation {
					info := SymbolInfo{
						Name:      si.DisplayName,
						Kind:      lspconv.SCIPSymbolKindToString(si.Kind, lspconv.StyleLowercase),
						File:      absPath,
						Line:      int(si.Range.Start.Line),
						Signature: si.SignatureDocumentation.Text,
					}
					if includeRefs {
						if refs, err := storage.GetReferencesWithDocuments(context.Background(), si.Symbol); err == nil {
							for _, r := range refs {
								refFile := utils.URIToFilePathCached(r.DocumentURI)
								info.References = append(info.References, fmt.Sprintf("%s:%d", refFile, r.Range.Start.Line))
							}
						}
					}
					symbols = append(symbols, info)
				}
				continue
			}
		}

		// Fallback to LSP
		result, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentDocumentSymbol, &lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
		if err != nil {
			continue
		}
		var extractSymbols func([]lsp.DocumentSymbol)
		extractSymbols = func(syms []lsp.DocumentSymbol) {
			for _, sym := range syms {
				info := SymbolInfo{
					Name:      sym.Name,
					Kind:      lspconv.LSPSymbolKindToString(sym.Kind, lspconv.StyleLowercase),
					File:      absPath,
					Line:      int(sym.Range.Start.Line),
					Signature: sym.Detail,
				}
				if includeRefs {
					refsResult, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentReferences, &lsp.ReferenceParams{
						TextDocumentPositionParams: lsp.TextDocumentPositionParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: sym.Range.Start},
						Context:                    lsp.ReferenceContext{IncludeDeclaration: false},
					})
					if err == nil && refsResult != nil {
						if refs, err := parseReferencesResult(refsResult); err == nil {
							for _, ref := range refs {
								refFile := utils.URIToFilePathCached(ref.URI)
								info.References = append(info.References, fmt.Sprintf("%s:%d", refFile, ref.Range.Start.Line))
							}
						}
					}
				}
				symbols = append(symbols, info)
				if len(sym.Children) > 0 {
					children := make([]lsp.DocumentSymbol, 0, len(sym.Children))
					for _, child := range sym.Children {
						if child != nil {
							children = append(children, *child)
						}
					}
					extractSymbols(children)
				}
			}
		}
		if result != nil {
			if docSymbols, err := parseDocumentSymbolsResult(result); err == nil && len(docSymbols) > 0 {
				extractSymbols(docSymbols)
			}
		}
	}

	// Output results
	if formatJSON {
		data, err := json.MarshalIndent(symbols, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("# Extracted Symbols (%d total)\n\n", len(symbols))

		currentFile := ""
		for _, sym := range symbols {
			if sym.File != currentFile {
				currentFile = sym.File
				fmt.Printf("\nFile: %s\n", currentFile)
			}
			fmt.Printf("  %s %s (line %d)", sym.Kind, sym.Name, sym.Line)
			if sym.Signature != "" {
				fmt.Printf(": %s", sym.Signature)
			}
			if len(sym.References) > 0 {
				fmt.Printf(" [%d refs]", len(sym.References))
			}
			fmt.Println()
		}
	}

	return nil
}

// Helper function to count symbols recursively
func countSymbols(symbols []lsp.DocumentSymbol) int {
	count := len(symbols)
	for _, sym := range symbols {
		if len(sym.Children) > 0 {
			// Convert pointers to values
			children := make([]lsp.DocumentSymbol, 0, len(sym.Children))
			for _, child := range sym.Children {
				if child != nil {
					children = append(children, *child)
				}
			}
			count += countSymbols(children)
		}
	}
	return count
}

// AnalyzeDependencies analyzes dependencies for specified files using only LSP data
func AnalyzeDependencies(configPath string, files []string, formatJSON bool) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 3*time.Minute)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	dependencies := make(map[string]*DependencyInfo)

	var storage scip.SCIPDocumentStorage
	if cmdCtx.Cache != nil {
		storage = cmdCtx.Cache.GetSCIPStorage()
	}

	if len(files) == 0 && storage != nil {
		if uris, _ := storage.ListDocuments(context.Background()); len(uris) > 0 {
			for _, uri := range uris {
				files = append(files, utils.URIToFilePathCached(uri))
			}
		}
	}

	for _, file := range files {
		absPath, _ := filepath.Abs(file)
		uri := utils.FilePathToURICached(absPath)

		dep := &DependencyInfo{File: absPath, Exports: []ExportSymbol{}, SymbolCount: 0}

		if storage != nil {
			if doc, err := storage.GetDocument(context.Background(), uri); err == nil && doc != nil {
				dep.SymbolCount = len(doc.SymbolInformation)
				// Determine exported symbols by definition in this file and uppercase heuristic
				defSymbols := make(map[string]bool)
				for _, occ := range doc.Occurrences {
					if occ.SymbolRoles&types.SymbolRoleDefinition != 0 {
						defSymbols[occ.Symbol] = true
					}
				}
				for _, si := range doc.SymbolInformation {
					if !defSymbols[si.Symbol] {
						continue
					}
					name := si.DisplayName
					if name != "" && unicode.IsUpper(rune(name[0])) {
						dep.Exports = append(dep.Exports, ExportSymbol{Name: name, Kind: lspconv.SCIPSymbolKindToString(si.Kind, lspconv.StyleLowercase)})
					}
				}
				dependencies[absPath] = dep
				continue
			}
		}

		// Fallback to LSP
		result, err := cmdCtx.Manager.ProcessRequest(cmdCtx.Context, types.MethodTextDocumentDocumentSymbol, &lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
		if err == nil && result != nil {
			var processSymbols func([]lsp.DocumentSymbol)
			processSymbols = func(syms []lsp.DocumentSymbol) {
				for _, sym := range syms {
					dep.SymbolCount++
					if sym.Name != "" && unicode.IsUpper(rune(sym.Name[0])) {
						dep.Exports = append(dep.Exports, ExportSymbol{Name: sym.Name, Kind: lspconv.LSPSymbolKindToString(sym.Kind, lspconv.StyleLowercase)})
					}
					if len(sym.Children) > 0 {
						children := make([]lsp.DocumentSymbol, 0, len(sym.Children))
						for _, child := range sym.Children {
							if child != nil {
								children = append(children, *child)
							}
						}
						processSymbols(children)
					}
				}
			}
			if docSymbols, err := parseDocumentSymbolsResult(result); err == nil && len(docSymbols) > 0 {
				processSymbols(docSymbols)
			}
		}
		dependencies[absPath] = dep
	}

	if formatJSON {
		data, err := json.MarshalIndent(dependencies, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	} else {
		fmt.Printf("# Dependencies Analysis (%d files)\n\n", len(dependencies))
		for file, dep := range dependencies {
			relPath, _ := filepath.Rel(workingDir, file)
			fmt.Printf("File: %s\n", relPath)
			fmt.Printf("  Total Symbols: %d\n", dep.SymbolCount)
			if len(dep.Exports) > 0 {
				fmt.Printf("  Exported Symbols (%d):\n", len(dep.Exports))
				for _, exp := range dep.Exports {
					fmt.Printf("    - %s %s\n", exp.Kind, exp.Name)
				}
			}
			fmt.Println()
		}
	}

	return nil
}
